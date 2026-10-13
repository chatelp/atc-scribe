# Exchange

An exchange is one transmission in the thread: who spoke to whom, what was said, what it means.

**Markup.** `<article class="cl-exchange">`: `cl-exchange__head` (time · frequency in `cl-exchange__when`, the speaker chip, →, the addressee's callsign chip, `MatchStrength`, the play button `cl-exchange__play`), `cl-exchange__text` (the transcript, `lang="fr"` when French), `cl-exchange__instr` (instruction chips), `cl-exchange__explain` (the plain-language explanation; `--boxed` when it is a sentence), `cl-exchange__reply` ("↩ Pilot replied, 4 s later", optionally the reply in a `<q>`).

**Variants.** `is-selected` (the exchange of the followed or selected aircraft), `is-unmatched` (no aircraft named: the chip says why), `is-urgent` (MAYDAY, 7700), `is-live` (someone is speaking: `LevelMeter` only). `cl-exchange--wide` is the transcript layout of the folded column: `cl-exchange__who` (time, frequency) left, `cl-exchange__what` right.

**Rules.** State what happened, never whether it was right: "Pilot replied", "No reply heard: the pilot may be out of the station's reach". A guess says it is one. The thread grows at the bottom; a new exchange appears, it does not slide.

**Data.** `/api/v1/transcriptions` (`speaker_type`, `callsign`, `match_score`, `match_reason`, `plain[]` with `text` and `text_fr`), `store.plainFrench(t)`, `/transcriptions/{id}/audio`.
