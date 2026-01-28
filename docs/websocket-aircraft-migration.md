# Plan: Migrate Aircraft Updates from HTTP Polling to WebSocket

## Summary

Replace HTTP polling (every 2 seconds) with WebSocket-only aircraft updates. Send delta updates (only changed fields) instead of full aircraft objects to minimize bandwidth and improve real-time responsiveness.

## Current Problems

1. **HTTP polling inefficiency**: Full aircraft objects (~2.5KB each) sent every 2 seconds
2. **Redundant code paths**: Dual HTTP/WebSocket architecture controlled by `websocket_aircraft_updates` config
3. **LastSeen triggers every update**: Change detector fires on every cycle because `LastSeen` always changes (line 149-152 in change_detector.go)
4. **Over-engineered toggle**: The `aircraftStreamingDisabled` flag adds complexity

## Architecture After Change

```
Backend:  ADSB Fetch -> Change Detector -> Compute Delta -> WebSocket Broadcast
Frontend: WebSocket  -> Apply Delta     -> Aircraft Store -> Animation Engine -> Map
```

---

## Files to Modify

### Backend (Go)

#### 1. `internal/adsb/change_detector.go`
- Add `Delta map[string]interface{}` field to `AircraftChange` struct
- Add `computeDelta()` method that returns only changed fields
- **Remove** the `LastSeen` comparison (lines 149-152) - it triggers updates every cycle
- Modify `DetectChanges()` to populate the Delta field for "updated" changes

**Delta fields to track:**
- `lat`, `lon` (position)
- `alt_baro` (altitude)
- `track` (heading)
- `gs` (ground speed)
- `baro_rate` (vertical rate)
- `on_ground` (ground state)
- `status` (if changed)
- `phase` (if changed)

#### 2. `internal/adsb/service.go`
- **Remove** conditional change detector initialization (lines 198-210)
- Always initialize `changeDetector` and `broadcastChan`
- Modify `broadcastAircraftChange()` (lines 252-279):
  - For `"added"`: Send full aircraft object (unchanged)
  - For `"updated"`: Send `delta` field instead of full `aircraft`
  - For `"removed"`: Send hex only (unchanged)

#### 3. `internal/config/config.go`
- **Remove** `WebSocketAircraftUpdates bool` field from ADSB config struct

#### 4. `configs/config.toml`
- **Remove** `websocket_aircraft_updates = false` line

#### 5. `internal/api/handlers.go`
- **Remove** `websocket_aircraft_updates` from `/api/v1/config` response
- Keep `/api/v1/aircraft` endpoint for debugging/third-party use

### Frontend (JavaScript)

#### 6. `www/app.js`
- **Remove** `initHTTPPolling()` method (lines 2361-2375)
- **Remove** `fetchAircraftData()` method (lines 2378-2505)
- **Remove** `disableAircraftStreamingInWebSocket()` method (lines 2354-2358)
- **Remove** `aircraftPollingInterval` state variable (line 3020)
- **Remove** `aircraftStreamingDisabled` state variable and all checks
- Simplify `initAircraftDataSource()` to just call `initWebSocket()`
- Modify `handleAircraftUpdate()` to apply delta updates
- Add `applyDelta(aircraft, delta)` helper method

#### 7. `www/aircraft-animation.js`
- Add `updateAircraftDelta(hex, delta)` method for efficient delta updates

---

## Message Format Change

### Before (full object on every update):
```json
{
  "type": "aircraft_update",
  "data": {
    "type": "updated",
    "hex": "A1B2C3",
    "aircraft": { /* 50+ fields, ~2.5KB */ }
  }
}
```

### After (delta only):
```json
{
  "type": "aircraft_update",
  "data": {
    "type": "updated",
    "hex": "A1B2C3",
    "delta": {
      "lat": 43.6789,
      "lon": -79.3456,
      "alt_baro": 12500,
      "track": 245
    }
  }
}
```

**Bandwidth reduction**: ~90% (4-5 fields vs 50+ fields per update)

---

## Code to Remove (Cleanup)

| Location | What to Remove |
|----------|----------------|
| `app.js:2361-2505` | `initHTTPPolling()` and `fetchAircraftData()` |
| `app.js:2354-2358` | `disableAircraftStreamingInWebSocket()` |
| `app.js:3020-3021` | `aircraftPollingInterval` and `aircraftStreamingDisabled` state |
| `app.js` (multiple) | All `if (this.aircraftStreamingDisabled)` checks |
| `service.go:198-210` | Conditional change detector initialization |
| `config.go` | `WebSocketAircraftUpdates` field |
| `config.toml` | `websocket_aircraft_updates` line |
| `handlers.go` | `websocket_aircraft_updates` in config response |

---

## Implementation Order

1. **Backend first** (Go):
   - Modify `change_detector.go` to compute and include deltas
   - Modify `service.go` to always enable WebSocket and send deltas
   - Remove config toggle from `config.go` and `handlers.go`
   - Update `config.toml`

2. **Frontend second** (JavaScript):
   - Add `applyDelta()` helper to `app.js`
   - Modify `handleAircraftUpdate()` to handle deltas
   - Add `updateAircraftDelta()` to animation engine
   - Remove HTTP polling code and `aircraftStreamingDisabled` checks

---

## Verification Plan

1. **Start the server** and open the web UI
2. **Check browser console** - should see WebSocket messages, no HTTP polling logs
3. **Check Network tab** - no repeated `/api/v1/aircraft` requests
4. **Monitor WebSocket frames** - should see small delta messages
5. **Verify map updates** - aircraft should move smoothly
6. **Test reconnection** - disconnect/reconnect should reload aircraft via bulk response
7. **Test simulated aircraft** - create via API, verify they update via WebSocket

---

## Server-Side Filtering Removed

Server-side filtering has been removed for simplicity. All filtering is now done client-side.

### What was removed:
- `ClientFilters` struct from `server.go`
- `shouldSendToClient()` method - all messages now broadcast to all clients
- `UpdateFilters()`, `GetFilters()`, `MatchesFilters()` methods from Client
- `sendFilterUpdate()` calls from frontend
- `updateFilters()` method from WebSocket client
- `buildCurrentFilters()` helper from app.js

### Why:
- Server broadcast is simpler and more predictable
- Client-side filtering via `filteredAircraft` getter already handles display filtering
- Reduces server complexity and WebSocket message overhead
- Delta updates are small enough that filtering on server provides minimal bandwidth savings
