# EventList

The event list ("Worth knowing") gives the last half hour's notable events: go-arounds, runway changes, emergencies, the antenna switching.

**Markup.** `<ul class="cl-events">` of `<li><time>13:09</time><span>…</span></li>`; a warning word in `is-warn` with its ▲.

**Rules.** Newest first, a short sentence each, at most five here. A runway change is a change of direction ("LFPG switched to landing west (26L, was 08R)"), never the lead passing between two runways of the same direction. Empty, the list says what it has watched: "Nothing notable in the last half hour" only when the page has been open that long, otherwise "Nothing notable since 16:06Z, when this page opened"; the full log is the bell's popover (`cl-menu`): six kinds (emergency squawk ■ in `is-urgent`, go-around ▲, runway in use changed, aircraft of interest ● in `is-info`, station channels changed, audio lost or back), rows that select the aircraft (`is-clickable`), "Ring a bell for" checkboxes per kind (emergencies by default), Clear. Kept across a reload; a line from another day shows its date.

**Data.** `events.recent(30)` (the page's event log, 100 events kept in the browser).
