# Local STT via faster-whisper

## Overview

Add local speech-to-text as an alternative to the cloud-based OpenAI Realtime Transcription API (`gpt-4o-transcribe`). Uses **faster-whisper** — a CTranslate2-based Whisper implementation that's 4x faster than OpenAI's open-source whisper with comparable accuracy.

**This supersedes the previous Moonshine proposal.** faster-whisper provides significantly better accuracy (Whisper large-v3 level), built-in Silero VAD, and a mature ecosystem with GPU acceleration.

### Why

- **Cost**: OpenAI API charges ~$0.006/min of audio — local is free
- **Latency**: Eliminates ~200-500ms network round-trip
- **Privacy**: Audio never leaves the device
- **Offline**: Works without internet connectivity
- **Flexibility**: Choose model size to match your hardware

---

## Architecture

### Approach: Python HTTP Sidecar

faster-whisper is Python/CTranslate2 — it can't run natively in Go. A lightweight **FastAPI server** runs alongside co-atc and accepts PCM audio over HTTP.

```
Audio Stream → CentralAudioProcessor → MultiReader → io.Reader
                                                        │
                                          ┌─────────────┴─────────────┐
                                          │ backend = "openai"         │ backend = "local"
                                          ↓                            ↓
                                    Processor                    LocalProcessor
                                    (WebSocket to OpenAI)        (HTTP to sidecar)
                                          │                            │
                                          ↓                            ↓
                                    ┌─────────────┐             ┌─────────────┐
                                    │  SQLite     │             │  SQLite     │
                                    │  WS Bcast   │             │  WS Bcast   │
                                    │  FileLog    │             │  FileLog    │
                                    └─────────────┘             └─────────────┘
                                          │                            │
                                          └──────────┬─────────────────┘
                                                     ↓
                                          PostProcessor (GPT-4o)
                                          Works with either backend
```

### Key Design Decisions

1. **FastAPI over Flask** — async support, better performance, auto OpenAPI docs
2. **HTTP over gRPC** — project uses HTTP everywhere; gRPC adds protobuf complexity for minimal benefit at ~1 req/5s per frequency
3. **Sidecar resamples, not Go** — keeps the Go audio pipeline unchanged (24kHz); sidecar converts to 16kHz (Whisper's native rate) internally
4. **VAD on sidecar side** — faster-whisper has built-in Silero VAD; no need for Go-side VAD or torch dependency
5. **Buffer-and-flush** — Go accumulates audio for N seconds, sends to sidecar, sidecar's VAD filters silence

---

## Model Comparison

| Model | Params | Disk Size | VRAM (float16) | VRAM (int8) | Speed vs Realtime | Accuracy |
|-------|--------|-----------|----------------|-------------|-------------------|----------|
| `tiny` | 39M | 75 MB | ~1 GB | ~0.5 GB | ~30x | Fair |
| `base` | 74M | 142 MB | ~1 GB | ~0.5 GB | ~20x | Good |
| `small` | 244M | 466 MB | ~2 GB | ~1 GB | ~10x | Better |
| `medium` | 769M | 1.5 GB | ~4 GB | ~2 GB | ~5x | Great |
| `large-v2` | 1550M | 3.1 GB | ~6 GB | ~3 GB | ~3x | Best |
| `large-v3` | 1550M | 3.1 GB | ~6 GB | ~3 GB | ~3x | Best |

**Speed notes**: "vs Realtime" means how many times faster than real-time on a mid-range NVIDIA GPU (RTX 3060/4060). CPU is roughly 5-10x slower.

### Recommendations by Hardware

| Hardware | Recommended Model | Compute Type | Notes |
|----------|------------------|--------------|-------|
| NVIDIA GPU (6GB+ VRAM) | `medium` or `large-v3` | `float16` | Best accuracy, fast inference |
| NVIDIA GPU (4GB VRAM) | `small` or `medium` | `int8` | Good balance |
| NVIDIA GPU (2GB VRAM) | `base` or `small` | `int8` | Usable |
| CPU only (modern x86) | `small` | `int8` | Acceptable latency |
| CPU only (older/ARM) | `tiny` or `base` | `int8` | Fastest, lower accuracy |
| Apple Silicon (M1/M2/M3) | `small` | `int8` | CTranslate2 has no MPS support — CPU only |

---

## Quick Start

### 1. Run the setup script

**Windows:**
```powershell
.\scripts\setup_local_stt_windows.ps1
```

**macOS:**
```bash
./scripts/setup_local_stt_mac.sh
```

**Linux:**
```bash
./scripts/setup_local_stt_linux.sh
```

### 2. Start the whisper sidecar

**Windows:**
```powershell
.\scripts\start_whisper_server.ps1
```

**macOS/Linux:**
```bash
./scripts/start_whisper_server.sh
```

Or manually:
```bash
cd sidecar
.venv/bin/python whisper_server.py --model medium --device auto --compute-type float16 --port 8178
```

### 3. Configure co-atc

In `configs/config.toml`, set:
```toml
[transcription]
backend = "local"

[transcription.local]
server_url = "http://localhost:8178"
model_size = "medium"
```

Then rebuild and run co-atc as normal.

### 4. Download different models for testing

```bash
python scripts/download_whisper_model.py --model tiny
python scripts/download_whisper_model.py --model base
python scripts/download_whisper_model.py --model small
python scripts/download_whisper_model.py --model medium
python scripts/download_whisper_model.py --model large-v3
```

Models are cached in `~/.cache/huggingface/hub/` — download once, use forever.

---

## Python Sidecar Design

### File Structure

```
sidecar/
├── whisper_server.py      # FastAPI server (main entry point)
├── config.py              # Configuration dataclass + CLI arg parsing
└── requirements.txt       # Python dependencies
```

### API Endpoints

#### `POST /transcribe`

Accepts raw PCM audio, returns transcription.

**Request:**
- Body: raw PCM bytes (s16le format)
- Headers:
  - `Content-Type: application/octet-stream`
  - `X-Sample-Rate: 24000` (or 16000 — sidecar resamples internally)
  - `X-Channels: 1`

**Response:**
```json
{
  "text": "Air Canada 123 contact Toronto Tower 118.7",
  "segments": [
    {
      "text": " Air Canada 123 contact Toronto Tower 118.7",
      "start": 0.0,
      "end": 3.2,
      "no_speech_prob": 0.05
    }
  ],
  "language": "en",
  "duration": 5.0
}
```

#### `GET /health`

**Response:**
```json
{
  "status": "ok",
  "model": "medium",
  "device": "cuda",
  "compute_type": "float16"
}
```

### Key Implementation Details

```python
from faster_whisper import WhisperModel
from fastapi import FastAPI, Request, Header
import numpy as np
import threading

app = FastAPI()
model: WhisperModel = None
model_lock = threading.Lock()  # WhisperModel.transcribe() is NOT thread-safe

@app.on_event("startup")
def load_model():
    global model
    model = WhisperModel(
        config.model_size,
        device=config.device,
        compute_type=config.compute_type
    )

@app.post("/transcribe")
async def transcribe(request: Request, x_sample_rate: int = Header(24000)):
    pcm_bytes = await request.body()
    audio = np.frombuffer(pcm_bytes, dtype=np.int16).astype(np.float32) / 32768.0

    # Resample to 16kHz if needed (Whisper's native rate)
    if x_sample_rate != 16000:
        from scipy.signal import resample
        target_len = int(len(audio) * 16000 / x_sample_rate)
        audio = resample(audio, target_len)

    with model_lock:
        segments, info = model.transcribe(
            audio,
            language=config.language,
            beam_size=config.beam_size,
            vad_filter=config.vad_filter,
            vad_parameters=dict(
                threshold=config.vad_threshold,
                min_silence_duration_ms=config.min_silence_duration_ms,
            ),
        )
        result = list(segments)  # Consume the generator inside the lock

    return {
        "segments": [{"text": s.text, "start": s.start, "end": s.end, "no_speech_prob": s.no_speech_prob} for s in result],
        "text": " ".join(s.text for s in result).strip(),
        "language": info.language,
        "duration": info.duration,
    }
```

**Dependencies** (`sidecar/requirements.txt`):
```
faster-whisper>=1.1.0
fastapi>=0.104.0
uvicorn[standard]>=0.24.0
numpy>=1.24.0
scipy>=1.11.0
```

Note: faster-whisper bundles CTranslate2 and Silero VAD — no separate torch install needed.

---

## Go Integration Design

### New Config Fields

**`internal/config/config.go`** — Add to `TranscriptionConfig`:
```go
Backend string             `toml:"backend"` // "openai" (default) or "local"
Local   LocalWhisperConfig `toml:"local"`
```

New struct:
```go
type LocalWhisperConfig struct {
    ServerURL            string  `toml:"server_url"`             // Sidecar URL (default: http://localhost:8178)
    ModelSize            string  `toml:"model_size"`             // tiny, base, small, medium, large-v2, large-v3
    Device               string  `toml:"device"`                 // auto, cpu, cuda
    ComputeType          string  `toml:"compute_type"`           // float16, int8, int8_float16, float32
    Language             string  `toml:"language"`               // Language code (default: en)
    BeamSize             int     `toml:"beam_size"`              // Beam search width (default: 5)
    VADFilter            bool    `toml:"vad_filter"`             // Enable Silero VAD (default: true)
    VADThreshold         float64 `toml:"vad_threshold"`          // VAD sensitivity (default: 0.5)
    MinSilenceDurationMs int     `toml:"min_silence_duration_ms"` // Silence to split speech (default: 500)
    BufferSeconds        int     `toml:"buffer_seconds"`         // Audio accumulation window (default: 5)
    MaxBufferSeconds     int     `toml:"max_buffer_seconds"`     // Force flush threshold (default: 30)
    TimeoutSeconds       int     `toml:"timeout_seconds"`        // HTTP timeout for sidecar (default: 30)
}
```

**Mirror** these fields in `internal/transcription/models.go` (the `transcription.Config` struct).

### TOML Config Example

```toml
[transcription]
# ... existing OpenAI fields ...

# Backend: "openai" (cloud, default) or "local" (faster-whisper sidecar)
backend = "openai"

# Local faster-whisper settings (used when backend = "local")
# Requires the whisper sidecar server running. See docs/LOCAL-STT.md
[transcription.local]
server_url = "http://localhost:8178"
model_size = "medium"              # tiny, base, small, medium, large-v2, large-v3
device = "auto"                    # auto, cpu, cuda
compute_type = "float16"           # float16, int8, int8_float16, float32
language = "en"
beam_size = 5
vad_filter = true
vad_threshold = 0.5
min_silence_duration_ms = 500
buffer_seconds = 5                 # Accumulate N seconds before sending to sidecar
max_buffer_seconds = 30            # Force flush if buffer exceeds this
timeout_seconds = 30               # HTTP timeout for sidecar requests
```

### LocalProcessor (`internal/transcription/local_processor.go`)

Implements `ProcessorInterface` (same as the OpenAI `Processor`).

```go
type LocalProcessor struct {
    frequencyID    string
    audioReader    io.ReadCloser
    sidecarURL     string
    httpClient     *http.Client
    wsServer       *websocket.Server
    storage        *sqlite.TranscriptionStorage
    ctx            context.Context
    cancel         context.CancelFunc
    logger         *logger.Logger
    config         Config
    fileLogger     *FileLogger
    // Audio accumulation
    audioBuffer    []byte
    audioBufferMu  sync.Mutex
}
```

**Audio strategy — fixed-window with server-side VAD:**

1. `processAudio()` goroutine reads from `audioReader` continuously, appends to `audioBuffer`
2. `transcriptionLoop()` goroutine runs every `buffer_seconds`:
   - Takes a snapshot of the buffer and clears it
   - HTTP POSTs raw PCM to `sidecar_url/transcribe`
   - Sidecar runs Silero VAD + faster-whisper
   - If no speech detected → empty response → silently discard
   - If speech found → store in SQLite, broadcast via WebSocket, write to FileLogger
3. `max_buffer_seconds` prevents unbounded accumulation if sidecar is slow/down

**Shared logic extraction:**

The DB storage + WebSocket broadcast + FileLogger logic from `Processor.processTranscriptionEvent()` should be extracted into a shared helper so both processors use the same code path. This ensures identical behavior for:
- SQLite `StoreTranscription()` calls
- WebSocket `transcription` event broadcasts
- File logging (raw transcription logs)

### Manager Branching (`internal/transcription/manager.go`)

Two insertion points where `NewProcessor()` is called (lines 155 and 236):

```go
var processor ProcessorInterface
var err error
if m.transcriptionConfig.Backend == "local" {
    processor, err = NewLocalProcessor(ctx, frequencyID, reader, m.transcriptionConfig, m.wsServer, m.transcriptionStorage, m.logger, m.fileLogger)
} else {
    processor, err = NewProcessor(ctx, frequencyID, reader, m.transcriptionConfig, m.wsServer, m.transcriptionStorage, m.logger, m.fileLogger)
}
```

**API key guard update** (manager.go:197): The check `if m.openAIAPIKey == ""` must be updated to also allow `backend == "local"` without an API key.

### Config Mapping (`internal/frequencies/service.go`)

Map `Backend` and `Local` fields through at ~line 511 where `transcription.Config` is constructed. Update the skip guard at ~line 198 for the local backend.

---

## Setup Scripts Design

### `scripts/setup_local_stt_windows.ps1`

```powershell
# 1. Check Python 3.10+ is installed
# 2. Create venv at sidecar/.venv
# 3. Activate venv, pip install -r sidecar/requirements.txt
# 4. Detect NVIDIA GPU via nvidia-smi
# 5. If CUDA available: pip install ctranslate2 with CUDA wheels
# 6. Download default model: python -c "from faster_whisper import WhisperModel; WhisperModel('medium')"
# 7. Print success + start instructions
```

### `scripts/setup_local_stt_linux.sh`

Same logic, bash. Additional: detect CUDA via `nvidia-smi`, handle apt dependencies if needed (`python3-venv`).

### `scripts/setup_local_stt_mac.sh`

Same logic, bash. **Important notes:**
- CTranslate2 does NOT support MPS (Apple Silicon GPU) — CPU only
- Recommend `compute_type = "int8"` for CPU performance
- Recommend `model_size = "small"` for reasonable CPU latency

### `scripts/start_whisper_server.ps1` / `scripts/start_whisper_server.sh`

Activate the venv, launch `sidecar/whisper_server.py` with configured arguments. Pass through CLI args.

### `scripts/download_whisper_model.py`

Download a specific model to the HuggingFace cache:
```
Usage: python scripts/download_whisper_model.py --model <size>
Sizes: tiny, base, small, medium, large-v2, large-v3
```

---

## Performance Expectations

| Metric | OpenAI Cloud | Local (GPU, medium) | Local (CPU, small) |
|--------|--------------|--------------------|--------------------|
| Latency (5s audio) | 500-1200ms | 200-500ms | 1-3s |
| Cost | ~$0.006/min | $0 | $0 |
| Accuracy (WER) | ~5-8% | ~7-10% | ~10-15% |
| Offline capable | No | Yes | Yes |
| GPU required | No | Recommended | No |

**Latency note**: The buffer window adds fixed latency (default 5s). Total latency = buffer_seconds + inference_time. For ATC monitoring this is acceptable. Reduce `buffer_seconds` to 3 for faster response at the cost of more sidecar calls.

---

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Sidecar not running at startup | `LocalProcessor.Start()` health-checks sidecar; returns error, frequency skipped |
| Sidecar crashes mid-operation | Log error, keep accumulating audio, retry on next flush cycle |
| Empty transcription (silence) | Sidecar returns `{"text": ""}`, silently discarded |
| Very long buffer (sidecar slow) | `max_buffer_seconds` forces flush, old audio discarded with warning |
| Config `backend` empty/missing | Defaults to `"openai"` — fully backwards compatible |

---

## Implementation Phases

### Phase 1: Python Sidecar
Create `sidecar/` directory with `whisper_server.py`, `config.py`, `requirements.txt`. Test standalone with curl.

### Phase 2: Go Config
Add `Backend`, `LocalWhisperConfig` to config structs. Update `config.toml.example`. Map through frequencies service.

### Phase 3: Go LocalProcessor
Extract shared transcription event handler. Create `local_processor.go`. Add backend branching in manager.

### Phase 4: Setup Scripts & Docs
Write setup scripts for all 3 platforms + model download script + start scripts. Update this doc with final details.

### Phase 5: Integration Testing
End-to-end: SRT/HTTP stream → Go → sidecar → SQLite → WebSocket → UI. Verify post-processing still works. Test backend switching.

---

## Files Summary

### To Create
| File | Purpose |
|------|---------|
| `sidecar/whisper_server.py` | FastAPI transcription server |
| `sidecar/config.py` | Server configuration |
| `sidecar/requirements.txt` | Python dependencies |
| `internal/transcription/local_processor.go` | Go local processor |
| `scripts/setup_local_stt_windows.ps1` | Windows setup |
| `scripts/setup_local_stt_linux.sh` | Linux setup |
| `scripts/setup_local_stt_mac.sh` | macOS setup |
| `scripts/start_whisper_server.ps1` | Windows start script |
| `scripts/start_whisper_server.sh` | Unix start script |
| `scripts/download_whisper_model.py` | Model downloader |

### To Modify
| File | Change |
|------|--------|
| `internal/config/config.go` | Add `Backend`, `LocalWhisperConfig` |
| `internal/transcription/models.go` | Mirror config fields |
| `internal/transcription/interface.go` | Add `LocalProcessor` compile check |
| `internal/transcription/processor.go` | Extract shared store/broadcast helper |
| `internal/transcription/manager.go` | Backend branching logic |
| `internal/frequencies/service.go` | Config mapping + API key guard |
| `configs/config.toml.example` | New `[transcription.local]` section |

---

## References

- [faster-whisper](https://github.com/SYSTRAN/faster-whisper) — CTranslate2 Whisper implementation
- [CTranslate2](https://github.com/OpenNMT/CTranslate2) — Fast inference engine for Transformer models
- [Silero VAD](https://github.com/snakers4/silero-vad) — Voice Activity Detection (bundled in faster-whisper)
- [FastAPI](https://fastapi.tiangolo.com/) — Python async web framework
- [OpenAI Whisper](https://github.com/openai/whisper) — Original model architecture

---

## Troubleshooting

### Sidecar won't start
- Ensure Python 3.10+ is installed: `python --version`
- Ensure venv is activated: `sidecar/.venv/Scripts/activate` (Windows) or `source sidecar/.venv/bin/activate`
- Check port 8178 is available: `netstat -an | findstr 8178`

### CUDA not detected
- Verify NVIDIA drivers: `nvidia-smi`
- Verify CUDA toolkit: `nvcc --version`
- faster-whisper needs CTranslate2 with CUDA — reinstall: `pip install ctranslate2 --force-reinstall`
- Fallback: set `device = "cpu"` and `compute_type = "int8"`

### Slow transcription
- Use a smaller model (`small` instead of `medium`)
- Use `int8` compute type (2x faster on CPU, slightly less accurate)
- Reduce `beam_size` from 5 to 1 (faster, slightly less accurate)
- Ensure you're using GPU if available (`device = "auto"`)

### Empty transcriptions
- Check `vad_threshold` — lower it (e.g., 0.3) if speech is being missed
- Check audio is reaching the sidecar — look at Go logs for HTTP POST activity
- Test sidecar directly: `curl -X POST http://localhost:8178/transcribe -H "Content-Type: application/octet-stream" --data-binary @test.pcm`

### Model download issues
- Models are downloaded from HuggingFace — ensure internet access during setup
- Cache location: `~/.cache/huggingface/hub/`
- Manual download: `python -c "from faster_whisper import WhisperModel; WhisperModel('medium')"`
