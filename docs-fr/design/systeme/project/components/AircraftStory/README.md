# AircraftStory

The aircraft story is what the context column shows while a flight is followed: who it is, where it is, what happened to it.

**Markup.** A `‹ Back to Traffic` header (`cl-tabs--column` height), then `cl-story__head` (`PhaseBadge`, `cl-story__callsign`, operator and type in `cl-plain`, a `cl-story__grid` of `cl-label` / `cl-story__val` pairs), then `<ol class="cl-story__events">` with `<time>`, a dot (`is-radio` for what was heard, `is-now`) and a sentence; an estimate is `is-next`.

**Rules.** The column's content is replaced, not overlaid; ‹ returns to the tab it came from and ✕ in the top bar stops following. Each line says a fact and its source: "Seen by the receiver", "Handed to Approche De Gaulle", "Expected touchdown, from its distance and speed".

**Data.** The aircraft's `created_at` and first track point; matched transmissions whose `plain` has a frequency role (hand-offs); `store.arrivals` (touchdown estimate).
