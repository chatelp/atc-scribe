# StatusLight

A status light says the state of one part of the chain: antenna, transcription, ADS-B.

**Markup.** `<span class="cl-light cl-light--ok">Transcription</span>`; variants `--ok` ● `--warn` ▲ `--bad` ■ `--unknown` ○ `--off` – `--switching` ◐. The shape and the word are required; the colour (`color-status-*`) is added to them.

**Off is not unknown.** `--off` is a state known and chosen: the part does not run, by configuration (a server started without transcription, a frequency off air); it is neutral, never a fault, and its word says so ("Off"). `--unknown` is a state not known: before the first status, the status route unreachable, a value the page does not recognise ("?"). A part that should run and does not is `--bad`. A stand-in that runs but is not the real thing is `--warn` with its word: the test instance's fake transcription sidecar reads "▲ Transcription Simulated" (owner, 15/10): what it writes must not be read as what the radio said.

**Data.** `/api/v1/status`: `antenna` (state, title, since: "On air: … chosen at the station 1 h 38 ago"), the transcription sidecar, ADS-B (`aircraft`, `messages_per_second`: "ADS-B 107 · 700/s").

- Don't: show a light without its word, or blink it.
