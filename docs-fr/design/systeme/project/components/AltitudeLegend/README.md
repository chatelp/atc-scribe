# AltitudeLegend

The altitude legend says which colour an aircraft wears on the map, from which altitude, and how busy each band and its levels are (owner, 15/10: an altitude scale matching the aircraft's colours, then the levels' occupation, "A, by band, more informative").

**Markup.** `<div class="cl-altlegend">` over the sky, bottom left (the mirror of the layers corner):
- `cl-altlegend__head`: `cl-altlegend__title` "Altitude" and `cl-altlegend__total` ("108 airborne");
- `cl-altlegend__bands`: five `cl-altlegend__band cl-altlegend__band--very-low|low|mid|high|cruise`, each a `cl-altlegend__histo` of `<i>` bars (height in % of the busiest bar of the whole legend, so that bands compare; `is-empty` for none) over a 4 px strip of the band's colour, then `cl-altlegend__count`, its number of aircraft;
- `cl-altlegend__bounds`: four `<span>` under the boundaries, "1 000 ft", "3 000 ft", "FL 125" and the station's cruise level ("FL 180" by default, `stationCruiseAltitudeFt`).
`is-light` over a light base map (IGN, OSM): the light palette on a light chip, so that it always describes what is drawn.

**Bars inside each band** (equal boxes, the scale differs from one to the next): very low, 4 bars of 250 ft; low, 4 of 500 ft; mid, 8 from 3 000 ft to FL 125; high, 5 up to the cruise level; cruise, 9 of 3 000 ft from the cruise level to FL 450 (FL 180-210 … FL 420-450). The population is the slider's: aircraft airborne now.

**Rules.** Refreshed with the counters, every 5 s, not at each position. It rises above the PAR frame when a flight is followed, as the layers corner does; it is not shown in the PAR view, which has its own colours. A switch "Altitude legend" in the layers menu, under Aircraft, on by default. A picture, not a control: `pointer-events: none`, `aria-hidden`.

**Data.** The bands of `getAltitudeColorBand` (`map/renderers/aircraft-webgl.js`): under 1 000 ft, 1 000 to 3 000, 3 000 to 12 500, above 12 500, and from the cruise level up.
