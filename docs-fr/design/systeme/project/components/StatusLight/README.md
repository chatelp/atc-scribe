# StatusLight

A status light says the state of one part of the chain: antenna, transcription, ADS-B.

**Markup.** `<span class="cl-light cl-light--ok">Transcription</span>`; variants `--ok` ● `--warn` ▲ `--bad` ■ `--unknown` ○ `--off` – `--switching` ◐. The shape and the word are required; the colour (`color-status-*`) is added to them.

**Data.** `/api/v1/status`: `antenna` (state, title, since: "On air: … chosen at the station 1 h 38 ago"), the transcription sidecar, ADS-B (`aircraft`, `messages_per_second`: "ADS-B 107 · 700/s").

- Don't: show a light without its word, or blink it.
