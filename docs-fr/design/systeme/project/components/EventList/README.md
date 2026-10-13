# EventList

The event list ("Worth knowing") gives the last half hour's notable events: go-arounds, runway changes, emergencies, the antenna switching.

**Markup.** `<ul class="cl-events">` of `<li><time>13:09</time><span>…</span></li>`; a warning word in `is-warn` with its ▲.

**Rules.** Newest first, a short sentence each, at most five; the full log is the bell. Kept across a reload; a line from another day shows its date.

**Data.** `events.recent(30)` (the page's event log, 100 events kept in the browser).
