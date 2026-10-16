# ParView

The PAR view takes the sky's place for one followed airport: its header, then a `ParFrame` per runway, each with its arrival sequence.

**Markup.** `cl-parview` over the sky; `cl-parhead` (`cl-parhead__airport` code in accent, the name, a `cl-link` "VAC" to the SIA chart in a new tab, runways in use with their share, QNH and its source, TA, TL "(computed)", separated by `cl-parhead__sep`); frames in `cl-parframe` (`is-in-use` adds the accent border), folded frames show only their head.

**Rules.** par-view.js draws the frames; this system styles what surrounds them. Frames take the column's width on show and on resize. **Their height follows the screen** (owner, 14/10): after each render the view measures what the header, the frame heads, the arrival tables and the folded runways leave, and sets the elevation band of the open frames between 80 px and its full 230 px; the azimuth band follows in proportion, the 20 px bottom margin only at full height. One runway on a tall enough screen keeps the full drawing; two shrink just enough to fit (De Gaulle at 1440 × 900: frames of 329 and 279 px, tables included, no scroll); under 80 px the view scrolls. A runway not in use stays folded on its head unless opened, and that choice is kept in the browser.
