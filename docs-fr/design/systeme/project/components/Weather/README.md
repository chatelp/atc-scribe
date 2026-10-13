# Weather

The weather block gives the reference airport's METAR, decoded in one sentence, with the TAF and the NOTAMs behind buttons.

**Markup.** In a `cl-section`: a `cl-label` ("Weather at LFPG"), the raw METAR in `cl-metar`, its plain-English reading in `cl-metar__plain`, then a `cl-row` of buttons "TAF" and "NOTAM" (with their count) that open popovers (`cl-card`).

**Rules.** It moves from the bottom bar to Right now. The plain reading says wind, visibility, clouds and QNH in words a newcomer understands.
