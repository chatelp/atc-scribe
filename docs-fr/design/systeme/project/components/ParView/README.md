# ParView

The PAR view takes the sky's place for one followed airport: its header, then a `ParFrame` per runway, each with its arrival sequence.

**Markup.** `cl-parview` over the sky; `cl-parhead` (`cl-parhead__airport` code in accent, the name, a `cl-link` "VAC" to the SIA chart in a new tab, runways in use with their share, QNH and its source, TA, TL "(computed)", separated by `cl-parhead__sep`); frames in `cl-parframe` (`is-in-use` adds the accent border), folded frames show only their head.

**Rules.** par-view.js draws the frames; this system styles what surrounds them. Frames take the column's width on show and on resize.
