# MapControls

Map controls choose the base map and the layers; they sit behind the layers button in the sky's corner.

**Markup.** `cl-menu cl-menu--map`: "Base map" with a `SegmentedControl` (IGN dark, IGN, OSM dark, OSM, None), then groups (`cl-menu__group` with a `cl-label`): Aircraft (labels, tracks and their length) and Layers (distance rings, airspace, airports, heliports, navaids, other runways, airport layout). Each row is `cl-menu__row`: a `Switch`, `cl-menu__label`, and when the layer has one an opacity `Slider` (`cl-slider--short`).

**Rules.** The same choices as today's Map Controls, saved in this browser. Hidden while the sky shows the PAR. Closes on Esc or a click outside.

**Procedures (SID, STAR)** (Q56): a layer row off by default, with "Arrivals · Departures" and the configuration it draws, said from the runways in use (see `ProcedureRoute`).

**Probable runway and exit** (Q57, proposed): a row of the Aircraft group, on by default, after "Altitude legend"; its help: "Read from each aircraft's ADS-B track, not announced by the controllers. An arrival: its pair of runways from 20 NM out (→26L/R), the runway itself from about 15 NM (→26L). A departure, from 15 NM: the exit it is heading for, and how many others stay possible (→OKASI +2). Nothing for an aircraft guided on a heading." (see `MapLabel`).
