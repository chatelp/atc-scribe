"""Configuration for the local STT sidecar."""
from __future__ import annotations

import argparse
from dataclasses import dataclass, field


@dataclass
class Config:
    host: str = "127.0.0.1"
    port: int = 8178

    # Models are chosen per language, not per frequency: the caller passes the
    # language it expects and the sidecar honours it. See docs-fr/13-strategie-bilingue.md.
    model_en: str = "sfabriece/whisper-large-v3-atco2-asr-mlx"
    model_fr: str = ""          # e.g. a converted bofenghuang/whisper-large-v3-french
    preload: bool = False

    # Voice activity detection. Measured on this station: up to 98% of night-time
    # squelch openings carry no speech at all, and a model fed silence hallucinates
    # plausible text. The gate is a correctness requirement, not an optimisation.
    vad_enabled: bool = True
    vad_threshold: float = 0.5
    vad_min_speech_ms: int = 250
    vad_min_silence_ms: int = 300
    min_speech_seconds: float = 0.25

    # Decoding. The upstream transcription prompt measurably hurts on short
    # transmissions: it multiplies degeneration loops sevenfold and leaks into the
    # output. Empty by default. See docs-fr/10-mesure-amorces.md.
    initial_prompt_en: str = ""
    initial_prompt_fr: str = ""

    log_level: str = "info"


def from_args(argv: list[str] | None = None) -> Config:
    d = Config()
    p = argparse.ArgumentParser(description="Local STT sidecar for atc-scribe")
    p.add_argument("--host", default=d.host)
    p.add_argument("--port", type=int, default=d.port)
    p.add_argument("--model-en", default=d.model_en,
                   help="MLX model (path or HF repo) for English audio")
    p.add_argument("--model-fr", default=d.model_fr,
                   help="MLX model for French audio; leave empty to refuse French")
    p.add_argument("--preload", action="store_true",
                   help="load models at startup rather than on first use")
    p.add_argument("--no-vad", dest="vad", action="store_false")
    p.add_argument("--min-speech-seconds", type=float, default=d.min_speech_seconds)
    p.add_argument("--log-level", default=d.log_level)
    a = p.parse_args(argv)
    return Config(host=a.host, port=a.port, model_en=a.model_en, model_fr=a.model_fr,
                  preload=a.preload, vad_enabled=a.vad,
                  min_speech_seconds=a.min_speech_seconds, log_level=a.log_level)
