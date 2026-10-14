# EventList

The event list ("Worth knowing") gives the last half hour's notable events: go-arounds, runway changes, emergencies, the antenna switching.

**Markup.** `<ul class="cl-events">` of `<li><time>13:09</time><span>…</span></li>`; a warning word in `is-warn` with its ▲.

**Rules.** Newest first, a short sentence each, at most five here. One sentence per kind, the same in the column and in the bell's log (one function writes them): emergency "■ 7700 AFR6TZ, emergency squawk, FL 110, 18 NM north-east."; go-around "▲ Go-around EZY14LM on 27R at LFPG, now climbing back." (the airport, since several are followed); runway "LFPG switched to landing west (26L, was 08R)."; of interest "● AFR18GH, an A388, is in range, 34 NM west."; station "The station switched to Approche De Gaulle." Times in the page's clock ("13:12Z" in UTC), without seconds, in both. A runway change is a change of direction ("LFPG switched to landing west (26L, was 08R)"), never the lead passing between two runways of the same direction. Empty, the list says what it has watched: "Nothing notable in the last half hour" only when the page has been open that long, otherwise "Nothing notable since 16:06Z, when this page opened"; the full log is the bell's popover (`cl-menu`): six kinds (emergency squawk ■ in `is-urgent`, go-around ▲, runway in use changed, aircraft of interest ● in `is-info`, station channels changed, audio lost or back), rows that select the aircraft (`is-clickable`), "Ring a bell for" checkboxes per kind (emergencies by default), Clear. Kept across a reload; a line from another day shows its date.

**Data.** `events.recent(30)` (the page's event log, 100 events kept in the browser).
