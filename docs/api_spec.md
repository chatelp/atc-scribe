# Co-ATC API Specification

This document provides detailed information about the Co-ATC API endpoints, request parameters, and response formats.

## Aircraft Data Endpoints

### GET /api/v1/aircraft

Retrieves the current list of all tracked aircraft.

**Response Format:**
```json
{
  "timestamp": "2025-05-19T01:02:03.456Z",
  "count": 2,
  "counts": {
    "ground_active": 5,
    "ground_total": 12,
    "air_active": 15,
    "air_total": 20
  },
  "aircraft": [
    {
      "hex": "a1b2c3",
      "flight": "SWA1234",
      "airline": "Southwest Airlines",
      "status": "active",
      "lat": 43.7,
      "lon": -79.5,
      "altitude": 35000,
      "heading": 90,
      "speed_gs": 450,
      "speed_true": 496,
      "vert_rate": -64,
      "category": "A5",
      "last_seen": "2025-05-19T01:02:03.456Z",
      "on_ground": false,
      "date_landed": null,
      "date_tookoff": "2025-05-19T00:45:12.123Z",
      "distance": 30.8,
      "is_simulated": false,
      "phase_data": {
        "current": {
          "phase": "CRZ",
          "changed_at": "2025-05-19T01:00:00.000Z"
        }
      },
      "clearances": [
        {
          "id": 1,
          "type": "takeoff",
          "text": "Cleared for takeoff runway 24R",
          "runway": "24R",
          "issued_at": "2025-05-19T01:00:00.000Z",
          "status": "active",
          "age": "2m"
        }
      ],
      "adsb": {
        "hex": "a1b2c3",
        "type": "adsb_icao",
        "flight": "SWA1234",
        "lat": 43.7,
        "lon": -79.5,
        "alt_baro": 35000,
        "alt_geom": 35250,
        "gs": 450,
        "ias": 292,
        "tas": 496,
        "mach": 0.852,
        "wd": 305,
        "ws": 89,
        "oat": -49,
        "tat": -17,
        "track": 90,
        "track_rate": 0,
        "roll": 0,
        "mag_heading": 86.48,
        "true_heading": 76.33,
        "baro_rate": -64,
        "geom_rate": 0,
        "squawk": "3151",
        "category": "A5",
        "nav_qnh": 1013.6,
        "nav_altitude_mcp": 35008,
        "nav_altitude_fms": 35008,
        "nav_heading": 85.08,
        "nic": 8,
        "rc": 186,
        "seen_pos": 6.431,
        "r_dst": 30.769,
        "r_dir": 141,
        "version": 2,
        "nic_baro": 1,
        "nac_p": 9,
        "nac_v": 1,
        "sil": 3,
        "sil_type": "perhour",
        "gva": 2,
        "sda": 2,
        "alert": 0,
        "spi": 0,
        "messages": 514,
        "seen": 5.9,
        "rssi": -18.6
      },
      "future": [
        {
          "lat": 43.72567,
          "lon": -79.658339,
          "altitude": 3400,
          "speed_gs": 208.3,
          "speed_true": 220.5,
          "heading": 327.15,
          "mag_heading": 325.8,
          "vertical_speed": 128,
          "timestamp": "2025-05-19T03:54:52-04:00",
          "distance": 5.4
        }
      ]
    }
  ]
}
```

**Enhanced Response Structure:**
- `counts`: Detailed aircraft counts by ground/air and active/total status
- `distance`: Distance from station in nautical miles
- `is_simulated`: Boolean indicating if aircraft is simulated
- `phase_data`: Current flight phase information
- `clearances`: Recent ATC clearances issued to the aircraft
- `future`: Future trajectory predictions (up to 5 positions)

The response includes detailed counts of aircraft by status:
- `counts.ground_active`: Number of grounded aircraft currently transmitting
- `counts.ground_total`: Total number of grounded aircraft being tracked
- `counts.air_active`: Number of airborne aircraft currently transmitting
- `counts.air_total`: Total number of airborne aircraft being tracked

The `status` field for each aircraft indicates its current status:
- `active`: Aircraft is currently transmitting ADS-B data
- `stale`: Aircraft has not transmitted data recently but is still within the history window
- `signal_lost`: Aircraft has disappeared from ADS-B coverage but is still being tracked

**Query Parameters:**
- `min_altitude` (optional): Minimum altitude in feet
- `max_altitude` (optional): Maximum altitude in feet  
- `status` (optional): Comma-separated list of statuses to include (active, stale, signal_lost)
- `callsign` (optional): Filter by callsign (partial match)
- `last_seen_minutes` (optional): Only include aircraft seen within the last N minutes
- `took_off_after` (optional): Only include aircraft that took off after this time (RFC3339 format)
- `took_off_before` (optional): Only include aircraft that took off before this time (RFC3339 format)
- `landed_after` (optional): Only include aircraft that landed after this time (RFC3339 format)
- `landed_before` (optional): Only include aircraft that landed before this time (RFC3339 format)
- `distance_nm` (optional): Only include aircraft within this distance (in nautical miles) from the reference
- `ref_lat` and `ref_lon` (optional): Reference coordinates for distance filtering
- `ref_hex` (optional): Reference aircraft hex code for distance filtering
- `ref_flight` (optional): Reference flight number for distance filtering
- `exclude_other_airports_grounded` (optional): Exclude grounded aircraft outside the airport range (1 = true, 0 = false)

### GET /api/v1/aircraft/{hex}

Retrieves data for a specific aircraft by its ICAO hex code.

**Response Format:**
Same as individual aircraft object in the `/aircraft` endpoint.

### GET /api/v1/aircraft/{hex}/tracks

Retrieves both position history and future predictions for a specific aircraft.

**Query Parameters:**
- `limit` (optional): Maximum number of historical positions to return (default: 1000, range: 100-3600)

**Response Format:**
```json
{
  "hex": "a1b2c3",
  "flight": "SWA1234",
  "distance": 30.8,
  "history": [
    {
      "id": 12345,
      "lat": 43.71567,
      "lon": -79.668339,
      "altitude": 3300,
      "speed_gs": 208.3,
      "speed_true": 220.5,
      "heading": 327.15,
      "mag_heading": 325.8,
      "vertical_speed": 128,
      "timestamp": "2025-05-19T03:53:52-04:00",
      "distance": 5.2
    }
  ],
  "future": [
    {
      "lat": 43.72567,
      "lon": -79.658339,
      "altitude": 3400,
      "speed_gs": 208.3,
      "speed_true": 220.5,
      "heading": 327.15,
      "mag_heading": 325.8,
      "vertical_speed": 128,
      "timestamp": "2025-05-19T03:54:52-04:00",
      "distance": 5.4
    }
  ]
}
```

## Health and Status Endpoints

### GET /api/v1/health

Returns the health status of the server.

**Response Format:**
```json
{
  "status": "active",
  "last_fetch": "2025-05-19T01:02:03.456Z",
  "aircraft_count": 25
}
```

### GET /api/v1/config

Returns the public configuration settings.

**Response Format:**
```json
{
  "adsb": {
    "fetch_interval_seconds": 1,
    "websocket_aircraft_updates": true
  },
  "storage": {
    "sqlite_base_path": "data/",
    "max_positions_in_api": 60
  },
  "frequencies": {
    "buffer_size_kb": 16,
    "stream_timeout_secs": 30,
    "reconnect_interval_secs": 5
  },
  "atc_chat": {
    "enabled": true
  }
}
```

### GET /api/v1/station

Returns the station's configured location and weather data.

**Response Format:**
```json
{
  "latitude": 43.6777,
  "longitude": -79.6248,
  "elevation_feet": 569,
  "airport_code": "CYYZ",
  "fetch_metar": true,
  "fetch_taf": true,
  "fetch_notams": true,
  "metar": {
    "note": "Free from https://www.aviationweather.gov/dataserver",
    "source": "Internal",
    "trend": [
      {
        "metar": "CYYZ 210600Z 07007KT 15SM FEW220 BKN260 09/03 A2994 RMK CC2CI4 SLP144",
        "ux": 29130120,
        "type": "V",
        "txt": [
          "Wind 070° 7kt. Visibility 15sm. Clouds few 22000ft, broken 26000ft. Temperature 9°C, dew point 3°C. Altimeter 29.94inHg."
        ],
        "rmk": "CC2CI4 SLP144",
        "wind": {
          "dir": "070",
          "speedMPS": 4,
          "speed": 7,
          "measure": "KT"
        },
        "decoded": {
          "wind_direction": "070",
          "wind_speed": "7",
          "wind_unit": "KT",
          "visibility": "15SM",
          "temperature": "9",
          "dew_point": "3",
          "altimeter": "29.94"
        }
      }
    ]
  },
  "taf": {
    "raw": "CYYZ 210541Z 2106/2212 07008KT P6SM FEW220 SCT260 TX15/2112Z TN07/2110Z...",
    "decoded": []
  },
  "notams": []
}
```

### POST /api/v1/station

Sets or clears station coordinate override.

**Request Body:**
```json
{
  "latitude": 43.6777,
  "longitude": -79.6248
}
```

**Response Format:**
```json
{
  "success": true,
  "message": "Station override coordinates set successfully",
  "latitude": 43.6777,
  "longitude": -79.6248
}
```

### GET /api/v1/wx

Returns cached weather data (METAR, TAF, NOTAMs).

**Response Format:**
```json
{
  "timestamp": "2025-05-19T01:02:03.456Z",
  "airport_code": "CYYZ",
  "metar": {
    "raw": "CYYZ 210600Z 07007KT 15SM FEW220 BKN260 09/03 A2994 RMK CC2CI4 SLP144",
    "decoded": {
      "wind_direction": "070",
      "wind_speed": "7",
      "wind_unit": "KT",
      "visibility": "15SM",
      "temperature": "9",
      "dew_point": "3",
      "altimeter": "29.94"
    }
  },
  "taf": {
    "raw": "CYYZ 210541Z 2106/2212 07008KT P6SM FEW220 SCT260...",
    "decoded": []
  },
  "notams": []
}
```

## Frequency Data Endpoints

### GET /api/v1/frequencies

Retrieves the list of all monitored ATC frequencies.

**Response Format:**
```json
{
  "timestamp": "2025-05-19T01:02:03.456Z",
  "count": 2,
  "frequencies": [
    {
      "id": "cyyz_dep",
      "airport": "CYYZ",
      "name": "Toronto Departures",
      "frequency_mhz": 127.575,
      "url": "https://s1-bos.liveatc.net/cyyz8",
      "status": "active",
      "bitrate": 128,
      "format": "mp3",
      "stream_url": "http://127.0.0.1:8080/api/v1/stream/cyyz_dep",
      "last_active": "2025-05-19T01:02:03.456Z",
      "order": 1
    }
  ]
}
```

### GET /api/v1/frequencies/{id}

Retrieves data for a specific frequency by its ID.

### GET /api/v1/stream/{id}

Streams audio for a specific frequency.

**Response Headers:**
```
Content-Type: audio/mpeg
Transfer-Encoding: chunked
Cache-Control: no-cache, no-store
X-Bitrate: 128
```

## WebSocket Endpoints

### GET /api/v1/ws

WebSocket endpoint for real-time aircraft updates and transcriptions.

**Message Types:**
- `aircraft_added`: New aircraft detected
- `aircraft_update`: Aircraft data updated
- `aircraft_removed`: Aircraft no longer tracked
- `aircraft_bulk_request`: Client requests bulk aircraft data
- `aircraft_bulk_response`: Server sends bulk aircraft data
- `filter_update`: Client updates filter preferences
- `transcription`: Real-time transcription updates
- `phase_change`: Aircraft phase changes
- `clearance_issued`: ATC clearance issued
- `alert`: System alerts

**Client-to-Server Messages:**
```json
{
  "type": "aircraft_bulk_request",
  "data": {
    "filters": {
      "show_air": true,
      "show_ground": true,
      "phases": {"CRZ": true, "APP": true}
    }
  }
}
```

**Server-to-Client Messages:**
```json
{
  "type": "aircraft_update",
  "data": {
    "aircraft": {
      "hex": "a1b2c3",
      "flight": "SWA1234",
      "status": "active"
    }
  }
}
```

## ATC Chat Endpoints

### POST /api/v1/atc-chat/session

Creates a new ATC chat session.

**Request Body:**
```json
{
  "instructions": "Custom AI instructions",
  "speed": 1.5
}
```

**Response Format:**
```json
{
  "session_id": "12345",
  "status": "created",
  "expires_at": "2025-05-19T02:02:03.456Z"
}
```

### DELETE /api/v1/atc-chat/session/{sessionId}

Ends an ATC chat session.

**Response Format:**
```json
{
  "status": "success",
  "session_id": "12345",
  "message": "Session ended successfully"
}
```

### GET /api/v1/atc-chat/session/{sessionId}/status

Gets the status of an ATC chat session.

**Response Format:**
```json
{
  "id": "12345",
  "active": true,
  "connected": true,
  "last_activity": "2025-05-19T01:02:03.456Z",
  "expires_at": "2025-05-19T02:02:03.456Z"
}
```

### POST /api/v1/atc-chat/session/{sessionId}/update-context

Updates the session context with fresh airspace data.

**Response Format:**
```json
{
  "status": "success",
  "message": "Session context updated with fresh airspace data"
}
```

### GET /api/v1/atc-chat/sessions

Lists all active ATC chat sessions.

**Response Format:**
```json
{
  "sessions": [
    {
      "id": "12345",
      "active": true,
      "connected": true,
      "last_activity": "2025-05-19T01:02:03.456Z",
      "expires_at": "2025-05-19T02:02:03.456Z"
    }
  ]
}
```

### GET /api/v1/atc-chat/airspace-status

Gets current airspace status for ATC chat.

**Response Format:**
```json
{
  "aircraft_count": 25,
  "active_count": 20,
  "frequencies_active": 3,
  "last_updated": "2025-05-19T01:02:03.456Z"
}
```

### GET /api/v1/atc-chat/ws/{sessionId}

WebSocket endpoint for ATC chat audio streaming.

**WebSocket Message Types:**
- `connection_ready`: Client connection established
- `openai_ready`: OpenAI connection established
- `connection_error`: Connection error occurred
- `session.update`: Session context updated
- `response.audio.delta`: Audio response chunk
- `response.audio.done`: Audio response complete

## Simulation Endpoints

### POST /api/v1/simulation/aircraft

Creates a simulated aircraft.

**Request Body:**
```json
{
  "hex": "123456",
  "flight": "SIM001",
  "lat": 43.6777,
  "lon": -79.6248,
  "altitude": 5000,
  "heading": 90,
  "speed": 200,
  "vertical_rate": 0
}
```

**Response Format:**
```json
{
  "success": true,
  "message": "Simulated aircraft created successfully",
  "hex": "123456"
}
```

### PUT /api/v1/simulation/aircraft/{hex}/controls

Updates simulation controls for an aircraft.

**Request Body:**
```json
{
  "heading": 180,
  "speed": 250,
  "vertical_rate": 500
}
```

**Response Format:**
```json
{
  "success": true,
  "message": "Simulation controls updated successfully"
}
```

### DELETE /api/v1/simulation/aircraft/{hex}

Removes a simulated aircraft.

**Response Format:**
```json
{
  "success": true,
  "message": "Simulated aircraft removed successfully"
}
```

### GET /api/v1/simulation/aircraft

Lists all simulated aircraft.

**Response Format:**
```json
{
  "timestamp": "2025-05-19T01:02:03.456Z",
  "count": 1,
  "aircraft": [
    {
      "hex": "123456",
      "flight": "SIM001",
      "lat": 43.6777,
      "lon": -79.6248,
      "altitude": 5000,
      "heading": 90,
      "speed": 200,
      "vertical_rate": 0,
      "is_simulated": true,
      "created_at": "2025-05-19T01:00:00.000Z"
    }
  ]
}
```

## Transcription Endpoints

### GET /api/v1/transcriptions

Returns a paginated list of all transcriptions.

**Query Parameters:**
- `limit` (optional): Maximum number of transcriptions to return (default: 100)
- `offset` (optional): Offset for pagination (default: 0)

**Response Format:**
```json
{
  "timestamp": "2025-05-20T20:15:35Z",
  "count": 2,
  "transcriptions": [
    {
      "id": 123,
      "frequency_id": "cyyz_grd",
      "created_at": "2025-05-20T20:15:35Z",
      "content": "Delta 123, cleared to land runway 24R",
      "is_complete": true,
      "is_processed": true,
      "content_processed": "Clearance: Landing clearance issued",
      "speaker_type": "ATC",
      "callsign": ""
    }
  ]
}
```

### GET /api/v1/transcriptions/frequency/{id}

Returns transcriptions for a specific frequency.

### GET /api/v1/transcriptions/time-range

Returns transcriptions within a specified time range.

**Query Parameters:**
- `start_time` (required): Start time in RFC3339 format
- `end_time` (optional): End time in RFC3339 format
- `limit` (optional): Maximum number of transcriptions to return (default: 100)
- `offset` (optional): Offset for pagination (default: 0)

### GET /api/v1/transcriptions/speaker/{type}

Returns transcriptions by speaker type (ATC or PILOT).

### GET /api/v1/transcriptions/callsign/{callsign}

Returns transcriptions for a specific aircraft callsign.

## Error Responses

All endpoints return appropriate HTTP status codes:
- `200 OK`: Success
- `400 Bad Request`: Invalid request parameters
- `404 Not Found`: Resource not found
- `500 Internal Server Error`: Server error

Error responses include a JSON object with error details:
```json
{
  "error": "Invalid parameter",
  "message": "Aircraft not found"
}
```