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
import json
import logging
import sys
import os
import threading
import time
import unicodedata
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
_counters = {"second_opinion_ok": 0, "second_opinion_failed": 0,
             "transcribed": 0, "rejected_no_speech": 0, "audio_save_failed": 0,
             "archiving_stopped": 0}


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


# The French model on this station is a symlink to an external drive. Unplug it
# and the second opinion stops working, the primary transcript still arrives, and
# nothing anywhere says the station just lost 8.5% of its callsign matches. That
# is the failure this reports: not a crash, a silence.
def model_status(model: str) -> dict:
    if not model:
        return {"id": "", "kind": "none", "available": False, "detail": "not configured"}
    # A local path is a directory we can look at; anything else is a hub id we
    # cannot check without reaching the network, so we do not claim to know.
    if model.startswith("/") or model.startswith("./") or model.startswith("~"):
        path = os.path.expanduser(model)
        if os.path.isdir(path):
            return {"id": model, "kind": "path", "available": True}
        return {"id": model, "kind": "path", "available": False,
                "detail": "path not found -- an external drive may be unplugged"}
    return {"id": model, "kind": "hub", "available": True, "detail": "not verified"}


_disk = {"checked": 0.0, "ok": True}


def _disk_has_room() -> bool:
    """Free space, rechecked at most once a minute.

    The archive is the expendable half: transcription and the database must
    survive a full disk, so archiving is what stops. It says so once rather than
    on every transmission -- a log line per clip would itself be a problem.
    """
    now = time.time()
    if now - _disk["checked"] < 60:
        return _disk["ok"]
    _disk["checked"] = now
    try:
        # The archive directory does not exist until the first clip is written,
        # and statvfs on a missing path raises. Walking up to an existing parent
        # is the difference between a guard and a guard that fails open -- which
        # is what this did on its first test.
        probe = os.path.abspath(cfg.save_audio or "/")
        while probe != "/" and not os.path.isdir(probe):
            probe = os.path.dirname(probe)
        st = os.statvfs(probe)
        free_gb = st.f_bavail * st.f_frsize / 1073741824
    except Exception as e:
        if _disk["ok"]:
            log.warning("cannot read free space, archiving continues: %s", e)
        return _disk["ok"]
    ok = free_gb >= cfg.save_audio_min_free_gb
    if ok != _disk["ok"]:
        if ok:
            log.info("audio archiving resumed, %.1f GB free", free_gb)
        else:
            log.warning("audio archiving STOPPED: %.1f GB free, floor is %.1f -- "
                        "transcription continues", free_gb, cfg.save_audio_min_free_gb)
        _counters["archiving_stopped"] = 0 if ok else 1
    _disk["ok"] = ok
    return ok


def keep_audio(raw: bytes, rate: int, channels: int, record: dict) -> str:
    """Write one transmission's audio and append its manifest line.

    The audio is written exactly as it arrived, before resampling: a transcript
    can always be recomputed from the audio, never the reverse, so the archive
    has to hold the thing that cannot be rebuilt.
    """
    if not cfg.save_audio:
        return ""
    if not _disk_has_room():
        return ""
    day = time.strftime("%Y%m%d")
    folder = os.path.join(cfg.save_audio, day, record.get("frequency_id") or "unknown")
    os.makedirs(folder, exist_ok=True)
    name = time.strftime("t_%Y%m%d_%H%M%S") + f"_{int(time.time()*1000) % 1000:03d}.wav"
    path = os.path.join(folder, name)
    try:
        with wave.open(path, "wb") as w:
            w.setnchannels(channels)
            w.setsampwidth(2)
            w.setframerate(rate)
            w.writeframes(raw)
        record["fichier"] = os.path.relpath(path, cfg.save_audio)
        with open(os.path.join(cfg.save_audio, "manifeste.jsonl"), "a") as m:
            m.write(json.dumps(record, ensure_ascii=False) + "\n")
        return path
    except Exception as e:
        # Losing the archive must never lose the transmission.
        _counters["audio_save_failed"] += 1
        log.warning("could not keep audio: %s", e)
        return ""


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
    models = {"en": model_status(cfg.model_en), "fr": model_status(cfg.model_fr)}
    # degraded, not unhealthy: the primary model answers, so transcription works;
    # what is lost is the second opinion, and losing it quietly is the problem.
    degraded = [lang for lang, m in models.items()
                if m["kind"] == "path" and not m["available"]]
    return {
        "status": "degraded" if degraded else "ok",
        # So a caller that started this process can tell it apart from one that
        # was already holding the port. A health probe answering says something
        # is there, not that it is yours.
        "pid": os.getpid(),
        "degraded": degraded,
        "models": models,
        "second_opinion": cfg.second_opinion,
        "second_opinion_ok": _counters["second_opinion_ok"],
        "second_opinion_failed": _counters["second_opinion_failed"],
        # Q27 asked for this and nothing answered it: the voice gate's rejection
        # rate in production was logged only at Debug, so "is the night-time
        # parasite absent in daylight?" had no instrument.
        "transcribed": _counters["transcribed"],
        "rejected_no_speech": _counters["rejected_no_speech"],
        "archiving": bool(cfg.save_audio) and _disk["ok"],
        "archiving_stopped_low_disk": bool(_counters["archiving_stopped"]),
        "audio_save_failed": _counters["audio_save_failed"],
        # kept for callers written against the earlier shape
        "model_en": cfg.model_en,
        "model_fr": cfg.model_fr or None,
        "vad": cfg.vad_enabled,
        "min_speech_seconds": cfg.min_speech_seconds,
        "languages": ["en"] + (["fr"] if cfg.model_fr else []),
    }


# Words that do not exist in air-traffic English, so an English model only writes
# them when it actually heard French. Deliberately short and common: this opens a
# gate, it does not measure a proportion. The same list produced the Q29 figures;
# keep it in step with whisper-lab/scripts/q1-francais.py or the measurement stops
# describing what runs.
FRENCH_MARKERS = (
    "bonjour", "bonsoir", "au revoir", "aurevoir", "merci", "monsieur",
    "autoris", "niveau", "descendez", "montez", "contactez", "rappelez",
    "piste", "vent", "quittez", "maintenez", "approche", "tour de",
    "s'il vous", "bien recu", "bien recu", "roger merci",
)


def _fold(s: str) -> str:
    return "".join(c for c in unicodedata.normalize("NFD", s.lower())
                   if unicodedata.category(c) != "Mn")


def looks_french(text: str) -> bool:
    folded = _fold(text)
    return any(_fold(m) in folded for m in FRENCH_MARKERS)


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
            _counters["rejected_no_speech"] += 1
            keep_audio(raw, x_sample_rate, x_channels, {
                "at": time.strftime("%Y-%m-%dT%H:%M:%S"),
                "frequency_id": x_frequency_id,
                "duree": round(duration, 3),
                "speech_seconds": round(speech.seconds, 3),
                "rejected": "no_speech",
                "texte": "",
            })
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
    # Second opinion. The two models fail on different recordings -- on French
    # transmissions they agreed on only 8 of the 34 they matched -- so a second
    # reading of the same audio reaches transmissions the first one misses. The
    # gate matters: asked on every transmission the French model also answers
    # where it has nothing to say, and invents callsigns on English audio.
    second = None
    if (cfg.second_opinion and language == "en" and cfg.model_fr
            and text and looks_french(text)):
        try:
            t1 = time.time()
            r2 = mlx_whisper.transcribe(audio, path_or_hf_repo=cfg.model_fr, language="fr")
            second = {
                "text": " ".join(r2["text"].split()),
                "language": "fr",
                "model": cfg.model_fr,
                "elapsed": round(time.time() - t1, 3),
            }
            _counters["second_opinion_ok"] += 1
            log.info("%s second opinion (fr) %r", x_frequency_id or "-", second["text"][:80])
        except Exception as e:
            # A failed second opinion is not a failed transcription: the primary
            # text stands and the caller is told nothing was added. But it is
            # counted, so /health can say it is happening.
            _counters["second_opinion_failed"] += 1
            log.warning("second opinion failed (%d so far): %s",
                        _counters["second_opinion_failed"], e)

    _counters["transcribed"] += 1
    keep_audio(raw, x_sample_rate, x_channels, {
        "at": time.strftime("%Y-%m-%dT%H:%M:%S"),
        "frequency_id": x_frequency_id,
        "duree": round(duration, 3),
        "speech_seconds": round(speech.seconds, 3),
        "langue": language,
        "modele": model,
        "texte": text,
        "texte_second": (second or {}).get("text", ""),
        "modele_second": (second or {}).get("model", ""),
    })

    elapsed = time.time() - started
    log.info("%s %.1fs speech=%.1fs %s %.2fs %r",
             x_frequency_id or "-", duration, speech.seconds, language, elapsed, text[:80])
    return {
        "text": text,
        "segments": segments,
        "language": language,
        "second_text": (second or {}).get("text", ""),
        "second_language": (second or {}).get("language", ""),
        "second_model": (second or {}).get("model", ""),
        "duration": round(duration, 3),
        "speech_seconds": round(speech.seconds, 3),
        "speech_fraction": round(speech.fraction, 3),
        "model": model,
        "frequency_id": x_frequency_id,
        "elapsed": round(elapsed, 3),
        "realtime_factor": round(duration / elapsed, 2) if elapsed else None,
    }


def watch_parent(interval: float = 5.0) -> None:
    """Stop when the process that started this one goes away.

    co-atc stops its sidecar on the way out, but it cannot do so when it is
    killed outright, and a sidecar that outlives it keeps port 8178 -- which is
    how seventeen servers came to share one in a night. The parent cannot
    prevent that; the child can notice.

    A process that is already orphaned at startup is left alone, so running this
    under `nohup ... &` and closing the terminal still works.
    """
    # co-atc sets this when it spawns the sidecar. Guessing instead -- watching
    # whenever the parent looks alive -- would kill a sidecar started by hand
    # with `nohup ... &` the moment its terminal closes, because its parent
    # becomes init too. The caller knows; it should say so rather than be
    # inferred.
    if os.environ.get("COATC_SPAWNED") != "1":
        log.info("not spawned by co-atc; not watching for a parent")
        return

    original = os.getppid()
    if original == 1:
        log.info("already orphaned at startup; not watching for a parent")
        return

    def loop() -> None:
        while True:
            time.sleep(interval)
            if os.getppid() != original:
                log.warning("parent %d is gone, stopping", original)
                # The hard exit is deliberate: uvicorn's graceful shutdown waits
                # on in-flight requests, and the client that would have finished
                # them is the process that just died.
                os._exit(0)

    threading.Thread(target=loop, daemon=True, name="parent-watch").start()
    log.info("watching parent %d", original)


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
    if cfg.exit_with_parent:
        watch_parent()

    import uvicorn
    uvicorn.run(app, host=cfg.host, port=cfg.port, log_level=cfg.log_level)


if __name__ == "__main__":
    main()
