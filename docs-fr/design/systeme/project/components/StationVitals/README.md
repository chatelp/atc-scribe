# StationVitals

The station's vital signs: a few aggregated figures in a console, shown on the sign-in page before signing in, to say the station is alive and what it does.

**Markup.** `cl-vitals` › `cl-vitals__head` ("This station, live" · "refreshed every 10 s") and `<dl class="cl-vitals__list">`: `<dt>` a lowercase key in `color-text-tertiary`, `<dd>` the figures in mono, a `cl-vitals__ok` dot when the part is working (the shape of `StatusLight` ok; a failing part shows the light's ■ and its word instead), separators in `<span>`.

**What it may show** (validated 13/10): aircraft tracked, ADS-B messages per second, the farthest aircraft's distance; channels on air and transmissions today; the transcription model, its device and its time per transmission; the share of transmissions matched today; airports followed and runways in use; uptime, version, commit. **Never** a position, a callsign, a named frequency or anything that locates the station.

**Data.** A public route serving these aggregates only, frozen by a 10 s server cache so that no visit queries the database (≈ 2 h 30 server, migration step 9): counts from the live state; transmissions and matched share from one query on the day's database; the farthest distance computed from the station's configured position (a distance locates nothing); the model as a readable family ("Whisper large-v3 ATCO2", from the sidecar's `/health`); the device from a `backend` field to add there; the mean time per transmission from a rolling average to add on the Go side; version and commit set at build time. A test checks that no position, callsign or named frequency leaves the route. A setting in Settings › Account turns the console off.
