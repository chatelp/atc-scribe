# ADS-B Source Refactor Plan (Review First)

## Objective

Refactor ADS-B source configuration and startup behavior to support four explicit source types, smarter source handling, readsb auto-detection, and source metadata exposure to the frontend.

This document is planning-only and does **not** implement code changes yet.

## Requested Modes (normalized)

The implementation will support these `adsb.source_type` values:

1. `external_api` (rename of current `external`)
   - Existing external API mode with URL template + headers.
   - Aircraft data only.
   - No receiver/stats metadata.

2. `tar1090`
   - Configured by base URL only.
   - Must read and validate all three files:
     - `aircraft.json`
     - `receiver.json`
     - `stats.json`
   - Aircraft ingestion same as current local JSON path flow.
   - Receiver/stats exposed by new API.

3. `readsb_api`
   - Configured by exact URL for aircraft endpoint (example: `http://192.168.1.60:30152/?all`).
   - Aircraft data only.
   - No receiver/stats metadata.

4. `readsb_file`
   - No per-source config required except `source_type`.
   - Auto-detect and read local files from standard readsb runtime directories.
   - Must consume:
     - `aircraft.json`
     - `receiver.json`
     - `stats.json`
   - Behaves like tar1090 triple-file mode for metadata exposure.

---

## Proposed Config Schema

### Clean-slate strategy (no legacy support)

- Remove legacy ADS-B keys and aliases from code and docs.
- Accept only the new explicit `source_type` values and mode-specific fields.
- Fail startup on unknown/legacy keys in `[adsb]` by strict schema validation.
- Update `configs/config.toml.example` to only show the new schema.

### Proposed ADS-B config (target)

```toml
[adsb]
# Required: external_api | tar1090 | readsb_api | readsb_file
source_type = "tar1090"

# Common polling settings
fetch_interval_seconds = 1
signal_lost_timeout_seconds = 60

# Used only for external_api
external_source_url = "https://.../lat/%f/lon/%f/dist/%.0f/"
api_host = "adsbexchange-com1.p.rapidapi.com"
api_key = ""
search_radius_nm = 50

# Used only for tar1090
# Base URL ending with /data/ recommended, but code should normalize with/without trailing slash
tar1090_base_url = "http://192.168.1.60/tar1090/data/"

# Used only for readsb_api
readsb_api_url = "http://192.168.1.60:30152/?all"

# Optional: custom readsb filesystem paths for readsb_file mode
# If empty, auto-detect from defaults.
readsb_data_dir = ""
```

### Source-type strictness

- Accepted values are only:
  - `external_api`
  - `tar1090`
  - `readsb_api`
  - `readsb_file`

Any other value fails validation.

---

## Auto-detection: `readsb_file`

For `readsb_file`, startup probes directories in order (first valid wins):

1. `adsb.readsb_data_dir` (if explicitly set)
2. `/run/readsb`
3. `/var/run/readsb`
4. `/run/dump1090-fa`
5. `/run/dump1090-mutability`

Validation criteria for a candidate directory:

- Files exist and are readable:
  - `aircraft.json`
  - `receiver.json`
  - `stats.json`
- `aircraft.json` parses into current `RawAircraftData` model.
- `receiver.json` and `stats.json` parse as generic JSON objects (strict schema not required initially).

If no valid directory is found, process startup fails.

---

## Data Access Design

## 1) Aircraft ingestion abstraction

Introduce a source reader abstraction (names tentative):

- `AircraftProvider` (returns parsed aircraft payload)
- `SourceMetaProvider` (returns receiver/stats payload where available)

Concrete implementations:

- `ExternalAPIProvider` (`external_api`)
- `Tar1090Provider` (`tar1090`)
- `ReadsbAPIProvider` (`readsb_api`)
- `ReadsbFileProvider` (`readsb_file`)

`adsb.Client` becomes source-agnostic and delegates to selected provider.

## 2) Metadata capture and API exposure

Metadata model (tentative):

```json
{
  "source_type": "tar1090",
  "source_label": "Tar1090",
  "receiver": { "...": "raw receiver.json" },
  "stats": { "...": "raw stats.json" },
  "updated_at": "2026-02-18T22:00:00Z",
  "available": true,
  "errors": []
}
```

Service behavior:

- Cache last successful receiver/stats for file/API sources that provide them.
- For modes without metadata (`external_api`, `readsb_api`), return `receiver=null`, `stats=null`, and `available=true` if aircraft stream is healthy.

---

## New API Endpoint

Add a new endpoint for Settings sidebar display:

- `GET /api/v1/adsb/source`

Response (shape):

```json
{
  "source_type": "tar1090",
  "mode": "tar1090",
  "status": "ok",
  "aircraft": {
    "available": true,
    "last_success_at": "2026-02-18T22:00:00Z",
    "last_error": ""
  },
  "receiver": {
    "available": true,
    "data": {}
  },
  "stats": {
    "available": true,
    "data": {}
  }
}
```

Notes:

- In `external_api` and `readsb_api`, receiver/stats availability is `false` with `data=null`.
- Endpoint is read-only and does not alter existing aircraft API.

---

## Startup Validation and Fail-fast Rules

System must terminate on startup if configured source is inaccessible or invalid.

Validation per mode:

1. `external_api`
   - Validate required config fields.
   - Perform a startup probe request with timeout.
   - Require HTTP 200 and JSON parse success.

2. `tar1090`
   - Validate `tar1090_base_url`.
   - Probe all required files (`aircraft.json`, `receiver.json`, `stats.json`).
   - Require HTTP 200 for all and JSON parse success.

3. `readsb_api`
   - Validate `readsb_api_url`.
   - Probe URL; require HTTP 200 and aircraft parse success.

4. `readsb_file`
   - Auto-detect directory.
   - Require all three files present/readable and parseable.

Failure behavior:

- Return explicit error from config/source validation path.
- `main.go` exits non-zero before services start.

---

## Frontend Plan (Settings Sidebar)

Placement:

- Add a new section in left Settings panel above current Debug section.
- Suggested title: `ADS-B Source`.

Displayed fields (minimal):

- Source type/mode
- Aircraft feed status
- Receiver metadata status/value preview (if available)
- Stats metadata status/value preview (if available)
- Last update timestamp
- Last error text (if any)

Frontend tasks:

- Add store state in `www/app.js` for source metadata.
- Poll `GET /api/v1/adsb/source` on interval (e.g., 5s) or piggyback existing periodic refresh.
- Render section in `www/index.html` directly above Debug Settings.

---

## Backend Change Map (planned files)

- `internal/config/config.go`
  - Expand ADS-B enum + strict validation.
  - Add mode-specific required-field logic.

- `configs/config.toml.example`
  - Replace old ADS-B section with new 4-mode schema and comments.

- `internal/adsb/client.go`
  - Refactor into provider-based fetching.

- `internal/adsb/*` (new files likely)
  - Add provider implementations for `external_api`, `tar1090`, `readsb_api`, `readsb_file`.
  - Add startup probe/validation helper(s).
  - Add metadata cache structs.

- `internal/adsb/service.go`
  - Surface source metadata accessor for API handler.

- `internal/api/handlers.go`
  - Add `GetADSBSourceStatus` handler.

- `internal/api/routes.go`
  - Register `GET /api/v1/adsb/source`.

- `www/app.js`
  - Fetch/store ADS-B source metadata.

- `www/index.html`
  - New Settings section above Debug.

- `docs/api_spec.md`
  - Document new endpoint.

---

## Implementation Phases

### Phase A - Config + Validation Foundation

1. Extend config schema and enum handling.
2. Remove old ADS-B config fields from struct/validation.
3. Add strict mode-specific config validation.

### Phase B - Source Providers + Startup Probing

1. Introduce provider abstraction.
2. Implement 4 source providers.
3. Add fail-fast startup probe for selected mode.

### Phase C - Source Metadata API

1. Capture receiver/stats for tar1090/readsb_file.
2. Add ADS-B service accessor.
3. Add `/api/v1/adsb/source` route/handler.

### Phase D - Frontend Settings Panel

1. Add ADS-B Source section above Debug.
2. Bind UI to new endpoint data.
3. Handle no-metadata modes cleanly.

### Phase E - Docs + Manual Validation

1. Update config example and API docs.
2. Build with `./build_windows.ps1`.
3. Manual verification across all 4 modes.

---

## Manual Test Matrix

1. `external_api` valid config -> app starts; source endpoint shows aircraft available, receiver/stats unavailable.
2. `external_api` invalid key/url -> startup fails.
3. `tar1090` all files present -> app starts; source endpoint returns receiver/stats payloads.
4. `tar1090` missing one file/404 -> startup fails.
5. `readsb_api` valid URL -> app starts; aircraft available only.
6. `readsb_api` 404/invalid JSON -> startup fails.
7. `readsb_file` with `/run/readsb` present -> auto-detect works, metadata available.
8. `readsb_file` no candidate directories -> startup fails.

---

## Risks / Notes

- `stats.json` and `receiver.json` schema variations across deployments: store and expose raw JSON initially.
- Windows development environment may not host `/run/readsb`; `readsb_file` validation remains Linux-targeted by design.

---

## Approval Gate

After your review/approval of this plan, implementation will proceed in this exact phase order with minimal unrelated changes.
