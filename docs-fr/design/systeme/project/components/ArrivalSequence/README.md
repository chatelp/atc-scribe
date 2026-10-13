# ArrivalSequence

The arrival sequence lists who lands next on a runway: distance to the threshold, time, gap to the one ahead.

**Markup.** `<table class="cl-table">` with the PAR's columns: rank, Aircraft (callsign and type), To threshold, Time, Behind the one ahead. Numbers `is-num`, secondary text `is-dim`; a gap under 3 NM (the usual radar separation on final) takes `is-tight`, amber.

**Rules.** It is the PAR's own table, shown outside the PAR: same rows, same words, same amber. A title says the airport and runway ("Arrivals to LFPG 26L"). One table per runway end in use with aircraft coming in, the nearest touchdown first: five rows when it is alone, three when there are several, then "And N more". Clicking a row selects the aircraft.

**Data.** `store.arrivals[code]` (each followed airport, recomputed every 5 s, PAR open or not) and `store.nextTouchdown`.
