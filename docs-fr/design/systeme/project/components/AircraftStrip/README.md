# AircraftStrip

An aircraft strip is one aircraft in the Traffic list: phase, callsign, level, speed, distance, and whether it was just heard.

**Markup.** `<div class="cl-strip cl-strip--<family>">`: a 3 px phase bar (from the family modifier), `PhaseBadge`, a `cl-strip__col` with the callsign (`cl-strip__v--callsign`) over the type and "heard 8 s" (`cl-strip__heard`), then level and vertical rate, speed, distance (`cl-strip__col--end`). Units in `cl-strip__s`.

**States.** Hover `color-surface-hover`, and the aircraft is highlighted on the map; `is-selected` (`color-surface-selected`, accent outline, same ring on the map and in the PAR); `is-lost` (values `color-text-tertiary`, no transparency); `is-urgent` (`color-surface-urgent` and an urgent badge).

**Rules.** `size-row-desk` high, `size-row-touch` on the phone where the whole row is the target. "Heard" disappears after 60 s and never blinks. Levels follow the transition level: "FL 070" above, "2 400 ft" below.

**Data.** The store's aircraft (ADS-B), filtered by the Traffic tab; "heard" from the last matched transmission.
