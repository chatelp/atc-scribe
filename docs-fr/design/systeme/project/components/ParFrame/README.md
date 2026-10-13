# ParFrame

The PAR frame is the sky in profile for one runway, as drawn by the PAR view; it also shows a followed aircraft's approach.

**Not a CSS component.** `www/par/par-view.js` draws it (SVG): elevation band on a square-root altitude scale, the transition layer (TA to TL), the 7° and 15° coverage cones, the 3° glide path with its 0.7° margins, the azimuth band, a filled triangle when the runway judge assigned the aircraft to this runway, the followed aircraft's track in white. Reuse it; never redraw it. This card is a static rendition made with the same formulas.

**Around it.** The head (`cl-parframe__head`): "26L → 08R", an `IN USE` mark, the aircraft count, "Open the PAR view". Under the PAR view, its `ArrivalSequence`.

**Migration.** Its phase colours move to the five families; its own palette (grid, cone, runway, transition amber) stays its own. It sizes to its column on show and on resize (`ResizeObserver`, 13/10).
