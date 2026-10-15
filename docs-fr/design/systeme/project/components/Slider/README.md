# Slider

Sliders set a frequency's volume or the altitude range of the Traffic tab.

**Volume.** `cl-slider` with `cl-slider__rail`, `cl-slider__fill` and `cl-slider__thumb`: a 2 px `color-border-control` rail, the filled part `color-accent-default`, a 12 px thumb; the hit area is 24 px high whatever the drawing.

**Altitude range** (`cl-altrange`): the value (mono) and the count shown, a histogram of the traffic (each bar in its altitude band's colour, see below), the two-thumb rail, and marks SFC · TA · FL 100 · FL 200 in `mono-caption` `color-text-tertiary`. ← and → move the focused thumb.

**Data.** The histogram counts the store's aircraft by level band; the range filters the list and the map together.

**The altitude slider** (Traffic, owner 15/10): its histogram's bars take the colour the aircraft of their altitude wear on the map (`data-band` very-low to cruise, the dark palette always, the column being dark), at 85 % inside the chosen band and 30 % outside; the track stays the accent of the selection. The transition altitude is a dashed tertiary mark (`cl-altrange__ta`) labelled "TA" alone; marks that would touch their neighbour are dropped ("FL 300" went: at 347 px it overlapped "FL 200").

**What the altitude range writes** (owner, 15/10). The value says the range that filters, not the stops the thumbs sit on: a range set by "Show only" ("FL 430 ± 500 ft") falls between stops (1 000 ft apart above FL 195), the thumbs go to the nearest stops, the text keeps the range. The count says what it counts: "8 of 86 airborne" (the slider counts aircraft in the air; the sky's note counts every active aircraft, as the Traffic tab, ground included, which the altitude filter leaves alone).
