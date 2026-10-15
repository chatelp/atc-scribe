# AltitudeLegend

The altitude legend says which colour an aircraft wears on the map, and from which altitude: the same five bands as the icons and as the Traffic slider's histogram (owner, 15/10).

**Markup.** `<div class="cl-altlegend">` over the sky, bottom left (the mirror of the layers corner): `cl-altlegend__title` "Altitude", five `cl-altlegend__band cl-altlegend__band--very-low|low|mid|high|cruise`, then four `cl-altlegend__bound` under the boundaries: "1 000 ft", "3 000 ft", "FL 125", and the station's cruise level ("FL 180" by default, `stationCruiseAltitudeFt`). `is-light` over a light base map (IGN, OSM): the light palette on a light chip, so that it always describes what is drawn.

**Rules.** It rises above the PAR frame when a flight is followed, as the layers corner does; it is not shown in the PAR view, which has its own colours. A switch "Altitude legend" in the layers menu, under Aircraft, on by default. It is a picture, not a control: `pointer-events: none`, `aria-hidden` (the bands are said in the slider's help).

**Data.** The bands of `getAltitudeColorBand` (`map/renderers/aircraft-webgl.js`): under 1 000 ft, 1 000 to 3 000, 3 000 to 12 500, above 12 500, and from the cruise level up.
