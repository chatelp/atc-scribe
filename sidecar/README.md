# Local STT sidecar

Implements the HTTP contract described in upstream's `docs/LOCAL-STT.md` — designed
there, never built. The Go side posts raw PCM, gets a transcript back, and no audio
leaves the machine.

```bash
python3 -m venv .venv && .venv/bin/pip install -r sidecar/requirements.txt
.venv/bin/python sidecar/whisper_server.py \
    --model-en <mlx-model-or-hf-repo> \
    [--model-fr <mlx-model-or-hf-repo>]
```

`GET /health` reports which languages are served. `POST /transcribe` takes raw
`s16le` PCM (or a WAV container) with `X-Sample-Rate`, `X-Channels`, `X-Language`
and `X-Frequency-Id`, and returns the transcript plus `duration`,
`speech_seconds`, `realtime_factor`, and `rejected` when nothing was transcribed.

## Two departures from the upstream design, both measured

**A voice-activity gate in front of the model.** On this station up to 98% of
night-time squelch openings carry no speech. Whisper fed near-silence does not
return an empty string, it invents a plausible one — `"Thank you."` over 2.2s of
noise. Silero costs ~10ms and answers `rejected: "no_speech"` instead.

**Language routing with a refusal path.** An English ATC model fed French audio
produces fluent, confident, wrong English. Same 7.5s clip from a French flying
club tower, same server, only the routing differs:

```
X-Language: fr  →  "Fox Alpha Charlie, 28 au tourisme d'atterrissage, vent 282,
                    5 kt. On atterrit, piste 28, Fox Alpha Charlie."
X-Language: en  →  "lufthansa seven eight two so request heading five one two seven
                    climbing ten eight five to lufthansa seven eight two lufthansa…"
```

The second is not noise — it is well-formed ATC English that a downstream
post-processor will happily turn into a clearance for an aircraft that exists.
When the expected language has no model configured the server returns
`rejected: "no_model_for_fr"` and an empty transcript, which is the only safe
answer.

`X-Language` is the caller's expectation, taken from the frequency catalogue, and
it is honoured rather than second-guessed: measured language detection is wrong on
16% of French and 28% of English transmissions here, and its failures cluster on
clips that hold no usable speech anyway.

## No initial prompt by default

Upstream's transcription prompt is 150 words written for a continuous realtime
stream. On isolated 3-second transmissions it measurably hurts: sevenfold more
degeneration loops on French, decode time doubled, and the prompt text leaking
into the transcript (`"Aircraft are at cruise level and use ICA-7, Yankee Papa…"`).
See `docs-fr/10-mesure-amorces.md`.
