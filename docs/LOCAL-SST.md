# Proposal: Local Real-Time STT with Moonshine

## Overview

Replace the cloud-based OpenAI transcription (`gpt-4o-transcribe`) with local on-device transcription using **Moonshine**, an open-source ASR model optimized for real-time edge inference.

## Motivation

- **Latency**: Cloud transcription adds ~200-500ms network latency per request
- **Cost**: OpenAI API charges per-minute of audio transcribed
- **Privacy**: Audio data currently leaves the device
- **Reliability**: Dependent on internet connectivity and OpenAI API availability

## Current Architecture

```
┌─────────────┐     SRT/WAV      ┌─────────────┐    WebSocket    ┌─────────────┐
│ rtl_airband │ ───────────────► │   co-atc    │ ──────────────► │   OpenAI    │
│  (HackRF)   │   16kHz mono     │  Go server  │   Audio chunks  │ gpt-4o-xscr │
└─────────────┘    pcm_s16le     └─────────────┘                 └─────────────┘
                                        │
                                        ▼
                                 ┌─────────────┐
                                 │   SQLite    │
                                 │ transcripts │
                                 └─────────────┘
```

### Current Audio Pipeline (in `internal/transcription/`)

1. **SRT Reader** (`internal/audio/srt_reader.go`): Connects to rtl_airband SRT streams
2. **Central Processor** (`internal/audio/central_processor.go`): Manages audio buffering
3. **Transcription Manager** (`internal/transcription/manager.go`): Coordinates transcription sessions
4. **Processor** (`internal/transcription/processor.go`): Handles OpenAI WebSocket connection, VAD, and streaming

### Current Audio Format

The SRT stream from rtl_airband outputs:
- **Format**: `pcm_s16le` (signed 16-bit little-endian PCM)
- **Sample Rate**: 16000 Hz (16kHz)
- **Channels**: 1 (mono)
- **Bit Rate**: 256 kb/s

This is already the optimal format for Moonshine — no transcoding required.

## Proposed Architecture

```
┌─────────────┐     SRT/WAV      ┌─────────────┐     numpy      ┌─────────────┐
│ rtl_airband │ ───────────────► │   co-atc    │ ─────────────► │  Moonshine  │
│  (HackRF)   │   16kHz mono     │  Go server  │   float32[]    │  ONNX/Tiny  │
└─────────────┘    pcm_s16le     └─────────────┘                └─────────────┘
                                        │                              │
                                        │◄─────────────────────────────┘
                                        ▼                         text
                                 ┌─────────────┐
                                 │   SQLite    │
                                 │ transcripts │
                                 └─────────────┘
```

## Moonshine Model Details

### Model Options

| Model | Parameters | Size | Speed | WER (LibriSpeech) |
|-------|------------|------|-------|-------------------|
| `moonshine/tiny` | 27M | ~190MB | Fastest | 12.66% |
| `moonshine/base` | 62M | ~400MB | Fast | 10.07% |

**Recommendation**: Start with `moonshine/tiny` for lowest latency. Upgrade to `base` if accuracy is insufficient for ATC terminology.

### Input Requirements

- **Sample Rate**: 16000 Hz (matches SRT output ✓)
- **Format**: Float32 normalized (-1.0 to 1.0)
- **Shape**: `[batch, samples]` or `[samples]`
- **Segment Length**: 0.1s to 64s per transcribe call

### Conversion (Zero-Transcode)

```python
import numpy as np

def pcm_s16le_to_float32(pcm_bytes: bytes) -> np.ndarray:
    """Convert raw PCM bytes to Moonshine-compatible float32 array."""
    pcm_int16 = np.frombuffer(pcm_bytes, dtype=np.int16)
    return pcm_int16.astype(np.float32) / 32768.0
```

This is a single memory operation — no resampling, no codec conversion.

## Implementation Plan

### Phase 1: Moonshine Python Service

Create a lightweight Python microservice that:
1. Accepts audio chunks via Unix socket or HTTP
2. Runs Moonshine ONNX inference
3. Returns transcription text

```
internal/transcription/
├── processor.go          # Existing OpenAI processor
├── processor_local.go    # NEW: Local Moonshine processor
└── moonshine/
    ├── service.py        # Python inference service
    ├── vad.py            # Voice Activity Detection
    └── requirements.txt
```

#### Python Service Interface

```python
# moonshine/service.py
import moonshine_onnx
import numpy as np
from flask import Flask, request, jsonify

app = Flask(__name__)
model = moonshine_onnx.MoonshineOnnxModel("moonshine/tiny")

@app.route('/transcribe', methods=['POST'])
def transcribe():
    # Receive raw PCM s16le bytes
    pcm_bytes = request.data

    # Convert to float32
    audio = np.frombuffer(pcm_bytes, dtype=np.int16).astype(np.float32) / 32768.0
    audio = audio[np.newaxis, :]  # Add batch dimension

    # Transcribe
    tokens = model.generate(audio)
    text = moonshine_onnx.load_tokenizer().decode_batch(tokens)[0]

    return jsonify({"text": text})

if __name__ == '__main__':
    app.run(host='127.0.0.1', port=5050, threaded=True)
```

### Phase 2: Go Integration

Add a new processor that calls the local Moonshine service instead of OpenAI.

```go
// internal/transcription/processor_local.go

type LocalProcessor struct {
    serviceURL string
    client     *http.Client
    logger     *logger.Logger
}

func (p *LocalProcessor) Transcribe(audioChunk []byte) (string, error) {
    resp, err := p.client.Post(
        p.serviceURL + "/transcribe",
        "application/octet-stream",
        bytes.NewReader(audioChunk),
    )
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    var result struct {
        Text string `json:"text"`
    }
    json.NewDecoder(resp.Body).Decode(&result)
    return result.Text, nil
}
```

### Phase 3: Voice Activity Detection (VAD)

For streaming, we need to detect speech boundaries. Options:

1. **Silero VAD** (recommended): Fast, accurate, Python-native
2. **WebRTC VAD**: C library, very fast but less accurate
3. **Energy-based**: Simple threshold on RMS, least accurate

```python
# moonshine/vad.py
import torch
torch.set_num_threads(1)

model, utils = torch.hub.load(
    repo_or_dir='snakers4/silero-vad',
    model='silero_vad',
    force_reload=False
)

def detect_speech(audio_chunk: np.ndarray, sample_rate: int = 16000) -> bool:
    """Returns True if speech is detected in the audio chunk."""
    tensor = torch.from_numpy(audio_chunk)
    speech_prob = model(tensor, sample_rate).item()
    return speech_prob > 0.5
```

### Phase 4: Streaming Pipeline

```
Audio Stream → VAD → Buffer → Moonshine → Post-Process → DB
     │           │       │         │            │
     │           │       │         │            └─ Callsign extraction, cleanup
     │           │       │         └─ ~50-100ms inference
     │           │       └─ Accumulate 2-5s of speech
     │           └─ Detect speech start/end
     └─ 16kHz PCM from SRT
```

#### Chunking Strategy

```python
class StreamingTranscriber:
    def __init__(self):
        self.buffer = []
        self.min_chunk_ms = 500   # Minimum chunk size
        self.max_chunk_ms = 5000  # Maximum chunk size
        self.silence_ms = 300     # Silence to trigger transcription

    def process_audio(self, pcm_chunk: bytes):
        self.buffer.append(pcm_chunk)

        if self.detect_end_of_utterance():
            audio = self.flush_buffer()
            text = self.transcribe(audio)
            return text
        return None
```

## Configuration Changes

Add to `configs/config.toml`:

```toml
#######################################################
# Transcription Configuration
#######################################################
[transcription]
# Backend: "openai" or "local"
backend = "local"

# OpenAI settings (used when backend = "openai")
openai_api_key = "sk-..."
model = "gpt-4o-transcribe"

# Local Moonshine settings (used when backend = "local")
[transcription.local]
enabled = true
service_url = "http://127.0.0.1:5050"
model = "moonshine/tiny"  # or "moonshine/base"

# VAD settings
vad_enabled = true
vad_threshold = 0.5
min_speech_ms = 500
max_speech_ms = 10000
silence_trigger_ms = 300

# Prompt for ATC domain (used in post-processing)
prompt = "ATC radio communication. NATO phonetic alphabet. Callsigns, altitudes, headings."
```

## ATC-Specific Optimizations

### Custom Vocabulary Boosting

Moonshine supports prompting via tokenizer hints. Create an ATC-specific prompt:

```python
ATC_PROMPT = """
Toronto Tower, Toronto Ground, Toronto Departure, Toronto Arrival.
Callsigns: Air Canada, WestJet, United, Delta, American, Jazz, Porter.
Phonetic: Alpha Bravo Charlie Delta Echo Foxtrot Golf Hotel India Juliet
Kilo Lima Mike November Oscar Papa Quebec Romeo Sierra Tango Uniform
Victor Whiskey X-ray Yankee Zulu.
Clearance, taxi, takeoff, landing, approach, departure, hold short,
runway, altitude, heading, squawk, ident, contact, frequency.
"""
```

### Post-Processing Pipeline

Keep the existing GPT-4o post-processing for:
- Callsign extraction and normalization
- Speaker identification (ATC vs Pilot)
- Instruction parsing

This hybrid approach gives fast local transcription + intelligent post-processing.

## Performance Expectations

| Metric | OpenAI Cloud | Moonshine Local |
|--------|--------------|-----------------|
| Latency (2s audio) | 300-800ms | 50-150ms |
| Latency (5s audio) | 500-1200ms | 100-300ms |
| Cost | ~$0.006/min | $0 |
| Accuracy (WER) | ~5-8% | ~10-13% |
| Offline capable | No | Yes |

## Dependencies

### Python
```
useful-moonshine-onnx @ git+https://github.com/moonshine-ai/moonshine.git#subdirectory=moonshine-onnx
flask>=2.0
numpy>=1.20
torch>=2.0  # For Silero VAD
```

### System
- ONNX Runtime (CPU or CUDA)
- ~200MB disk for tiny model, ~400MB for base

## Rollout Plan

1. **Week 1**: Install Moonshine, benchmark on recorded ATC audio
2. **Week 2**: Implement Python microservice with VAD
3. **Week 3**: Integrate with co-atc Go codebase
4. **Week 4**: A/B test against OpenAI, tune parameters
5. **Week 5**: Full cutover, monitor quality

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Lower accuracy than OpenAI | Keep OpenAI as fallback, use for post-processing |
| Jetson CPU too slow | Use ONNX optimizations, consider base only for quiet periods |
| ATC jargon misrecognized | Fine-tune on ATC audio corpus (future work) |
| Memory pressure | Monitor, tune batch sizes, use tiny model |

## Success Criteria

- [ ] Transcription latency < 200ms for 3s audio segments
- [ ] Word Error Rate < 15% on ATC communications
- [ ] Zero cloud API calls for basic transcription
- [ ] Seamless fallback to OpenAI if local fails

## Files to Modify

1. `internal/transcription/manager.go` - Add backend selection logic
2. `internal/transcription/processor.go` - Refactor to interface
3. `internal/transcription/processor_local.go` - NEW: Local processor
4. `internal/config/config.go` - Add local transcription config
5. `configs/config.toml` - Add local transcription settings
6. `moonshine/` - NEW: Python service directory

## References

- [Moonshine GitHub](https://github.com/moonshine-ai/moonshine)
- [Moonshine Paper](https://arxiv.org/abs/2410.15608)
- [Silero VAD](https://github.com/snakers4/silero-vad)
- [ONNX Runtime](https://onnxruntime.ai/)

---

*Proposal prepared by Watt, 2026-01-28*