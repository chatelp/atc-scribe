"""Local speech-to-text sidecar for atc-scribe.

Implements the HTTP contract described in upstream's `docs/LOCAL-STT.md`, which
was designed but never built: the Go side posts raw PCM and gets a transcript
back, so the audio pipeline stays unchanged and no audio leaves the machine.

Two things are deliberately added to that design, and both come from measurement
on a real station rather than from taste:

  1. A voice-activity gate in front of the model. Up to 98% of night-time squelch
     openings carry no speech, and Whisper fed near-silence does not return an
     empty string — it invents a plausible one.

  2. Language routing with a refusal path. An English ATC model fed French audio
     produces fluent, confident, wrong English: "cleared to land runway zero five"
     over a French flying club. Passed downstream, that fabricates a landing
     clearance for a real aircraft on the map. When the expected language has no
     model configured, this server returns an empty transcript and says why,
     which is the only safe answer.

Run:
    python sidecar/whisper_server.py --model-en <mlx-model> [--model-fr <mlx-model>]
"""
from __future__ import annotations

import io
import logging
import sys
import time
import wave
from dataclasses import dataclass

import numpy as np
from fastapi import FastAPI, Header, Request
from fastapi.responses import JSONResponse

sys.path.insert(0, __file__.rsplit("/", 1)[0])
from config import Config, from_args  # noqa: E402

WHISPER_RATE = 16000

log = logging.getLogger("stt")
cfg = Config()
app = FastAPI(title="atc-scribe STT sidecar")

_models: dict[str, object] = {}
_vad = None


# --------------------------------------------------------------------------- audio

def decode_pcm(raw: bytes, sample_rate: int, channels: int) -> np.ndarray:
    """Raw s16le (or a WAV container) to mono float32 at Whisper's 16 kHz."""
    if raw[:4] == b"RIFF":
        with wave.open(io.BytesIO(raw)) as w:
            sample_rate, channels = w.getframerate(), w.getnchannels()
            raw = w.readframes(w.getnframes())

    audio = np.frombuffer(raw, dtype=np.int16).astype(np.float32) / 32768.0
    if channels > 1:
        audio = audio.reshape(-1, channels).mean(axis=1)

    if sample_rate != WHISPER_RATE:
        from math import gcd
        from scipy.signal import resample_poly
        g = gcd(int(sample_rate), WHISPER_RATE)
        audio = resample_poly(audio, WHISPER_RATE // g, int(sample_rate) // g)
    return np.ascontiguousarray(audio, dtype=np.float32)


@dataclass
class Speech:
    seconds: float
    fraction: float


def measure_speech(audio: np.ndarray) -> Speech:
    """Seconds of actual speech, via Silero. Cheap next to the model itself."""
    global _vad
    if _vad is None:
        from silero_vad import load_silero_vad
        _vad = load_silero_vad()
    import torch
    from silero_vad import get_speech_timestamps

    spans = get_speech_timestamps(
        torch.from_numpy(audio), _vad, sampling_rate=WHISPER_RATE,
        threshold=cfg.vad_threshold,
        min_speech_duration_ms=cfg.vad_min_speech_ms,
        min_silence_duration_ms=cfg.vad_min_silence_ms,
    )
    speech = sum(s["end"] - s["start"] for s in spans) / WHISPER_RATE
    total = len(audio) / WHISPER_RATE
    return Speech(speech, speech / total if total else 0.0)


# --------------------------------------------------------------------------- models

def model_for(language: str) -> tuple[str | None, str]:
    """Return (model, resolved language), or (None, language) if we must refuse."""
    if language == "fr":
        return (cfg.model_fr or None), "fr"
    return (cfg.model_en or None), "en"


def load(path: str):
    if path not in _models:
        t0 = time.time()
        import mlx_whisper
        # mlx-whisper resolves the model lazily on first transcribe; touching it
        # here just makes the cost visible in the logs instead of in the first request.
        _models[path] = mlx_whisper
        log.info("model registered: %s (%.2fs)", path, time.time() - t0)
    return _models[path]


# --------------------------------------------------------------------------- routes

@app.get("/health")
def health():
    return {
        "status": "ok",
        "model_en": cfg.model_en,
        "model_fr": cfg.model_fr or None,
        "vad": cfg.vad_enabled,
        "min_speech_seconds": cfg.min_speech_seconds,
        "languages": ["en"] + (["fr"] if cfg.model_fr else []),
    }


@app.post("/transcribe")
async def transcribe(
    request: Request,
    x_sample_rate: int = Header(default=24000),
    x_channels: int = Header(default=1),
    x_language: str = Header(default="en"),
    x_frequency_id: str = Header(default=""),
):
    """Transcribe raw PCM.

    `X-Language` is the caller's expectation, taken from the frequency catalogue.
    It is honoured, not second-guessed: measured language detection is wrong on
    16% of French and 28% of English transmissions here, and its failures are
    concentrated on clips that hold no usable speech anyway.
    """
    started = time.time()
    raw = await request.body()
    if not raw:
        return JSONResponse({"error": "empty body"}, status_code=400)

    audio = decode_pcm(raw, x_sample_rate, x_channels)
    duration = len(audio) / WHISPER_RATE

    speech = Speech(duration, 1.0)
    if cfg.vad_enabled:
        speech = measure_speech(audio)
        if speech.seconds < cfg.min_speech_seconds:
            return {
                "text": "", "segments": [], "language": None,
                "duration": round(duration, 3),
                "speech_seconds": round(speech.seconds, 3),
                "rejected": "no_speech",
                "frequency_id": x_frequency_id,
                "elapsed": round(time.time() - started, 3),
            }

    language = (x_language or "en").lower()[:2]
    model, language = model_for(language)
    if model is None:
        # Refusing is the safe answer: transcribing French with an English model
        # yields confident, well-formed, wrong clearances.
        return {
            "text": "", "segments": [], "language": language,
            "duration": round(duration, 3),
            "speech_seconds": round(speech.seconds, 3),
            "rejected": f"no_model_for_{language}",
            "frequency_id": x_frequency_id,
            "elapsed": round(time.time() - started, 3),
        }

    mlx_whisper = load(model)
    prompt = cfg.initial_prompt_fr if language == "fr" else cfg.initial_prompt_en
    kwargs = {"path_or_hf_repo": model, "language": language}
    if prompt:
        kwargs["initial_prompt"] = prompt

    result = mlx_whisper.transcribe(audio, **kwargs)
    text = " ".join(result["text"].split())
    segments = [
        {"text": " ".join(s["text"].split()),
         "start": round(s["start"], 2), "end": round(s["end"], 2),
         "no_speech_prob": round(s.get("no_speech_prob", 0.0), 4)}
        for s in result.get("segments", [])
    ]
    elapsed = time.time() - started
    log.info("%s %.1fs speech=%.1fs %s %.2fs %r",
             x_frequency_id or "-", duration, speech.seconds, language, elapsed, text[:80])
    return {
        "text": text,
        "segments": segments,
        "language": language,
        "duration": round(duration, 3),
        "speech_seconds": round(speech.seconds, 3),
        "speech_fraction": round(speech.fraction, 3),
        "model": model,
        "frequency_id": x_frequency_id,
        "elapsed": round(elapsed, 3),
        "realtime_factor": round(duration / elapsed, 2) if elapsed else None,
    }


def main() -> None:
    global cfg
    cfg = from_args()
    logging.basicConfig(level=cfg.log_level.upper(),
                        format="%(asctime)s %(levelname)s %(name)s %(message)s")
    log.info("English model: %s", cfg.model_en)
    log.info("French model: %s", cfg.model_fr or "(none — French will be refused)")
    if cfg.preload:
        load(cfg.model_en)
        if cfg.model_fr:
            load(cfg.model_fr)
    import uvicorn
    uvicorn.run(app, host=cfg.host, port=cfg.port, log_level=cfg.log_level)


if __name__ == "__main__":
    main()
