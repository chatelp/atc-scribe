# AircraftStory

The aircraft story is what the context column shows while a flight is followed: who it is, where it is, what happened to it.

**Markup.** A `‹ Back to Traffic` header (`cl-tabs--column` height); `cl-story__head` (`PhaseBadge`, `cl-story__callsign`, operator, type and registration in `cl-plain`, a `cl-story__grid` of `cl-label` / `cl-story__val` pairs); then the tabs Story · Details · Track · Nearby (`cl-tabs--inset`). Story is `<ol class="cl-story__events">`: `<time>`, a dot (`is-radio` for what was heard, `is-now`) and a sentence; an estimate is `is-next`. Details is `KeyValues` behind `Disclosure`s, with the phase history and the instructions heard as tables (the "Then" column from `www/story/then.js`, card K, fed by each instruction's `value` and `unit`: FL, ft, deg, kt, MHz, hPa, code, runway); Track is the mini-map and its table; Nearby the proximity list.

**Rules.** The column's content is replaced, not overlaid; ‹ returns to the tab it came from and ✕ in the top bar stops following. Each line says a fact and its source: "Seen by the receiver", "Handed to Approche De Gaulle", "Expected touchdown, from its distance and speed".

**Data.** The aircraft's `created_at` and first track point; matched transmissions whose `plain` has a frequency role (hand-offs); `store.arrivals` (touchdown estimate).
