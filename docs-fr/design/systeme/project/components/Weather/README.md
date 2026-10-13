# Weather

The weather block gives the reference airport's METAR, decoded in one sentence, with the TAF and the NOTAMs behind buttons.

**Markup.** In a `cl-section`: a `cl-label` ("Weather at LFPG"), the raw METAR in `cl-metar`, its plain-English reading in `cl-metar__plain`, then a `cl-row` of buttons "TAF", "NOTAM" (with their count) and "Earlier" (the previous METARs, when there are some). Each opens a `cl-card` below the buttons, in the column, one at a time: a NOTAM runs long, and nothing floats over the map. The button says it is open with `aria-expanded="true"` (and `aria-controls` naming the card), which the system draws as selected.

**Rules.** It moves from the bottom bar to Right now. The plain reading says wind, visibility, clouds and QNH in words a newcomer understands.
