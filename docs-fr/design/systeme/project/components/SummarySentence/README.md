# SummarySentence

The summary sentence says in one line what the sky is doing now, and where that comes from.

**Markup.** `<p class="cl-summary">` with runways and values in `<b>` (mono 600), followed by `<p class="cl-summary__source">`.

**Rules.** Built from templates, never generated: the runway in use and since when, the arrivals on final, the next touchdown, the other followed airports. "Since" takes the present perfect: "De Gaulle has been using runway 26L since 11:40Z", and without a time "Orly uses runway 06". Runways of the same direction in use together are named together ("runways 26L and 27R": numbers one apart at most, as De Gaulle's doublets), never as "perhaps". A change of runway is a change of direction (west to east), not the lead passing from one runway of a doublet to the other: only the former resets "since". Folded, the short sentence speaks of one airport, the one of the next touchdown, with the count on final there: "De Gaulle lands on 26L, three on final, next in about 2 min." The source line is required when the facts are inferred: "Read from the ADS-B tracks, not announced by the controllers."

**Data.** `/api/v1/station` `runway_in_use` (arrival, departure, go-around; airport, runway, `since` in RFC 3339), `store.arrivals`, `store.nextTouchdown`, `/airports` for names. `since` is null until the server has seen a change after it started: then write "since at least 09:12", the time the server started (uptime in `/api/v1/status`), never a bare "since" without a time.
