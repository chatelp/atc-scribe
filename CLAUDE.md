# CLAUDE.md - Co-ATC Project Guide

## Project Overview

Co-ATC is an AI-enhanced aircraft monitoring system for air traffic control operations. It provides real-time airspace monitoring, AI-powered voice assistant, transcription of ATC communications, and intelligent alert systems. **Local/network use only - not internet-facing.**

## Quick Reference

### Build & Run

```powershell
# Build (Windows - always use this, not 'go run')
.\build.ps1

# Run
.\bin\co-atc.exe

# With custom config
.\bin\co-atc.exe -config configs/config.toml
```

```bash
# Build (macOS/Linux)
./build.sh
./bin/co-atc
```

### Key URLs (default)

- Web UI: `http://localhost:8080`
- API: `http://localhost:8080/api/v1/`
- WebSocket: `ws://localhost:8080/ws`

## Tech Stack

**Backend:** Go 1.23.2, chi/v5 router, SQLite (pure Go), Gorilla WebSocket, zap logging, TOML config

**Frontend:** Alpine.js, Leaflet.js, Tailwind CSS, Web Audio API

**AI:** OpenAI Whisper (transcription), GPT-4 (post-processing), Realtime API (voice assistant)

## Project Structure

```
cmd/server/main.go          # Entry point
internal/
  adsb/                     # Aircraft tracking (ADS-B data)
  api/                      # HTTP routes and handlers
  audio/                    # Audio processing (SRT, FFmpeg)
  atcchat/                  # Voice assistant (OpenAI Realtime)
  config/                   # TOML configuration
  frequencies/              # Radio frequency management
  simulation/               # Simulated aircraft
  storage/sqlite/           # Database layer
  templating/               # AI context aggregation
  transcription/            # Whisper transcription
  weather/                  # METAR/TAF/NOTAM
  websocket/                # Real-time broadcasting
www/                        # Frontend (Alpine.js SPA)
assets/                     # Static data (airlines, airports, runways)
prompts/                    # AI system prompts
configs/config.toml         # Configuration file
data/                       # SQLite databases (auto-created)
docs/                       # Documentation
```

## Key Patterns

### Service Architecture
- Each domain is a self-contained service with dependency injection
- Context-aware for graceful shutdown
- Concurrent goroutines for real-time operations

### Real-time Data Flow
1. ADS-B Client → Service → Change Detector → WebSocket → Frontend
2. Audio Stream → Transcription → OpenAI Whisper → SQLite → WebSocket

### Flight Phases
8-phase system: NEW, TAX, T/O, DEP, CRZ, ARR, APP, T/D

### Database
- Daily rotation: `data/co-atc-YYYY-MM-DD.db`
- Pure Go SQLite (no CGO)

## Development Guidelines

### Backend
- Use PowerShell terminal commands (Windows dev environment)
- Build via `build.ps1`, never `go run`
- Update `docs/api_spec.md` when changing APIs
- Clean up duplicate/dead code

### Frontend
- Use Tailwind CSS classes over custom CSS
- Follow Alpine.js patterns
- Dark theme with green highlights
- No rebuild needed for frontend-only changes

### Documentation
- Keep `docs/project_progress.md` as working memory
- Update README.md after major features

## Important Files

- `configs/config.toml.example` - Config template with documentation
- `docs/api_spec.md` - API endpoint documentation
- `docs/technical_docs.md` - Architecture details
- `.roorules` - Project conventions
- `prompts/*.txt` - AI system prompts

## Testing

No automated test suite. Test manually:
- API: curl/Postman for endpoints
- Frontend: Browser testing
- Config validation runs on startup

## Notes

- **No authentication** - development/hobby project
- AI features require OpenAI API key in config
- Frontend served from `www/` directory
- WebSocket change detection reduces bandwidth ~95%
