# ADS-B `external-opensky` Source Plan (Review First)

## Objective

Add a new ADS-B source type: `external-opensky` using OpenSky REST state vectors, while preserving the current clean-slate source architecture.
This mode targets public OpenSky network data for users without local ADS-B receivers.

This is a **plan only** document (no code changes yet).

Primary requirement:
- Prefer OpenSky `on_ground` signal over local heuristic ground detection when available.

---

## Scope

### In scope
- New `adsb.source_type = "external-opensky"`.
- Fetch aircraft via OpenSky public `/states/all` endpoint.
- Convert OpenSky state-vector array format into internal `RawAircraftData` / `ADSBTarget`.
- Use OpenSky `on_ground` as authoritative when present.
- Startup probe + fail-fast consistent with current source validation behavior.
- Config/docs updates for new mode.

### Out of scope (MVP)
- Historical OpenSky endpoints (`/flights/*`, `/tracks`).
- OpenSky `/states/own` receiver-specific endpoint.
- Receiver/stats metadata (OpenSky does not provide tar1090-style `receiver.json`/`stats.json`).
- Multi-endpoint blending across OpenSky + other sources.

---

## OpenSky API Shape (Key Details)

Endpoint base: `https://opensky-network.org/api`

Primary call:
- `GET /states/all` (optionally with bbox query params `lamin`, `lomin`, `lamax`, `lomax`).

Response shape:
- Top-level object with:
  - `time` (unix seconds)
  - `states` (2D array)
- Live sample check (bbox Switzerland) returns default rows with 17 fields (`0..16`) when `extended` is not set.

Each `states[i]` is an indexed array:
- `0` `icao24` (hex)
- `1` `callsign`
- `5` `longitude`
- `6` `latitude`
- `7` `baro_altitude` (meters)
- `8` `on_ground` (boolean)
- `9` `velocity` (m/s)
- `10` `true_track` (degrees)
- `11` `vertical_rate` (m/s)
- `13` `geo_altitude` (meters)
- `14` `squawk`
- `15` `spi`
- `16` `position_source` (0=ADS-B, 1=ASTERIX, 2=MLAT, 3=FLARM)
- `17` `category` (**only present when** `extended=1`)

Important behavior notes:
- Some fields are null.
- Callsign is often padded/trailing-whitespace and should be trimmed.
- Anonymous access is rate/time-resolution limited.
- Authenticated access has different limits; new accounts must use OAuth2 client credentials flow.

---

## Proposed Config Additions

Add OpenSky mode and settings to `[adsb]`:

```toml
[adsb]
source_type = "external-opensky"

# Existing shared fields reused for area selection
search_radius_nm = 50

# OpenSky configuration
opensky_base_url = "https://opensky-network.org/api"
opensky_token_url = "https://auth.opensky-network.org/auth/realms/opensky-network/protocol/openid-connect/token"

# Auth mode: anonymous | oauth2
opensky_auth_mode = "anonymous"
opensky_oauth2_credentials_path = "configs/opensky_credentials.json"

fetch_interval_seconds = 1
signal_lost_timeout_seconds = 60
```

Credentials file format (`opensky_oauth2_credentials_path`):

```json
{"clientId":"your-api-client-id","clientSecret":"your-api-client-secret"}
```

Validation rules (planned):
- `source_type` accepts new value `external-opensky`.
- `opensky_base_url` required for this mode.
- `opensky_auth_mode` must be one of `anonymous|oauth2`.
- `oauth2` requires `opensky_oauth2_credentials_path`.
- `opensky_token_url` required when `opensky_auth_mode=oauth2` (default shown above).
- `search_radius_nm > 0` required (used to build bbox from station coordinates).

---

## Data Mapping Plan (OpenSky -> ADSBTarget)

Create an OpenSky parser that safely handles null/missing indexes and converts units:

- `hex` <= `state[0]`
- `flight` <= trimmed `state[1]`
- `lon` <= `state[5]`
- `lat` <= `state[6]`
- `alt_baro` <= meters-to-feet(`state[7]`)
- `alt_geom` <= meters-to-feet(`state[13]`)
- `gs` <= m/s-to-knots(`state[9]`)
- `track` <= `state[10]`
- `baro_rate` <= m/s-to-ft/min(`state[11]`)
- `squawk` <= `state[14]`
- `spi` <= bool->int or dedicated bool handling consistent with existing model
- `category` <= stringified `state[17]` when `extended=1`, else empty/unknown
- `position_source` <= optional diagnostic field from `state[16]` (not required for MVP logic)
- `source_type` <= `external-opensky`

Raw envelope mapping:
- `RawAircraftData.Now` <= response `time` (fallback to current unix time).
- `RawAircraftData.Messages` <= 0 (OpenSky response doesn’t provide messages count).

Also carry raw OpenSky ground flag into target-level metadata so service can use it authoritatively.

---

## `on_ground` Precedence Rule (Critical)

Current service derives ground/air state via local heuristic (`IsFlying(...)`) with no-source-data fallback logic.

Planned precedence per aircraft:
1. If OpenSky source and OpenSky `on_ground` is present: **use it directly**.
2. Else: use existing heuristic path (`IsFlying`, no-usable-data safeguards, previous state preservation).

Implementation note:
- Extend `ADSBTarget` with optional source-ground field (e.g., `OnGroundReported *bool` or equivalent) to avoid overloading unrelated numeric fields.
- Keep existing heuristic code intact for all non-OpenSky sources.

---

## Request Construction Plan

For OpenSky requests:
- Build URL as `opensky_base_url + /states/all`.
- Add bbox query from station lat/lon + `search_radius_nm`.
  - Use conservative geodesic approximation for MVP:
    - `deltaLat = radiusNm / 60`
    - `deltaLon = radiusNm / (60 * cos(lat))` (guard high latitudes)
- For `anonymous`, send no auth header.
- For `oauth2`, obtain token from `opensky_token_url` using `grant_type=client_credentials`, `client_id`, `client_secret`, then send `Authorization: Bearer <token>`.
- Cache token in-memory and refresh proactively (or on first `401`) because token lifetime is ~30 minutes.

Rationale:
- Bounding box reduces payload size and API credit usage compared to global `/states/all`.

---

## Startup Validation / Fail-Fast

For `external-opensky` startup probe:
- If `oauth2` is enabled, validate credentials file load + token acquisition first.
- Perform a single timed `/states/all` request with configured auth + bbox.
- Require HTTP 200 and parseable OpenSky response object.
- Require `states` to be present (can be empty array).
- On failure: return explicit validation error and abort startup (existing fatal startup pattern).

---

## API/Frontend Impact

`GET /api/v1/adsb/source` remains same schema:
- `source_type = external-opensky`
- `aircraft.available` reflects fetch health
- `receiver.available = false`
- `stats.available = false`

No new UI section needed; existing ADS-B source panel should display mode/status as today.

---

## Planned Code Touchpoints

- `internal/config/config.go`
  - Add OpenSky fields to `ADSBConfig`.
  - Update validation switch + error messages.

- `internal/adsb/source_status.go`
  - Add `SourceTypeExternalOpenSky` constant.

- `internal/adsb/client.go`
  - Add `external-opensky` fetch branch.
  - Add OpenSky request builder, OAuth2 token client (credentials-file based), parser.

- `internal/adsb/models.go`
  - Add optional field for source-reported on-ground state.
  - Update source_type comment enum list.

- `internal/adsb/service.go`
  - Apply `on_ground` precedence rule in processing pipeline.

- `configs/config.toml.example`
  - Document new source type and OpenSky fields.

- `docs/api_spec.md`
  - Update source_type enumeration where relevant.

---

## Risks & Mitigations

1. OAuth2 token lifecycle and credentials handling
- Mitigation: credentials loaded from file, in-memory token cache, refresh on expiry/401, clear startup errors for invalid creds.

2. Array-index payload fragility
- Mitigation: robust index guards and null-safe typed extraction helpers.

3. Rate limits / credit usage
- Mitigation: always query bounded area; keep startup probe single-shot.

4. Unit-conversion mistakes
- Mitigation: central conversion helpers and logging sample converted values in debug mode.

---

## Validation Matrix (Post-Implementation)

1. Anonymous mode
- Valid config boots; `/states/all` parsed; aircraft ingested.

2. OAuth2 mode
- Credentials file loads; token acquisition succeeds; authenticated `/states/all` fetch works.
- Invalid `clientId/clientSecret` fails startup cleanly.
- Expired token path refreshes and resumes (or fails with explicit auth error).

3. Ground precedence
- For an OpenSky target with `on_ground=true`, service stores `OnGround=true` even if local heuristic suggests airborne.
- For `on_ground=false`, service stores airborne unless no-data fallback applies.
- Non-OpenSky sources unchanged.

4. API/UI status
- `/api/v1/adsb/source` returns `external-opensky` mode with receiver/stats unavailable.

---

## Rollout Steps

1. Add config schema + validation.
2. Add source constant + client fetch/parser.
3. Add `on_ground` source-field support + service precedence.
4. Update config/docs.
5. Build + manual smoke test (`build_windows.ps1`, startup probe, UI status panel).

If this plan looks good, next step is implementation in that exact order.
