# SummarySentence

The summary sentence says in one line what the sky is doing now, and where that comes from.

**Markup.** `<p class="cl-summary">` with runways and values in `<b>` (mono 600), followed by `<p class="cl-summary__source">`.

**Rules.** Built from templates, never generated: the runway in use and since when, the arrivals on final, the next touchdown, the other followed airports. The source line is required when the facts are inferred: "Read from the ADS-B tracks, not announced by the controllers."

**Data.** `/api/v1/station` `runway_in_use` (arrival, departure, go-around; airport, runway, since), `store.arrivals`, `store.nextTouchdown`, `/airports` for names.
