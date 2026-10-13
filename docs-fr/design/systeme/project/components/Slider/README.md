# Slider

Sliders set a frequency's volume or the altitude range of the Traffic tab.

**Volume.** `cl-slider` with `cl-slider__rail`, `cl-slider__fill` and `cl-slider__thumb`: a 2 px `color-border-control` rail, the filled part `color-accent-default`, a 12 px thumb; the hit area is 24 px high whatever the drawing.

**Altitude range** (`cl-altrange`): the value (mono) and the count shown, a histogram of the traffic (`color-surface-hover` outside the range, `color-accent-default` at 40 % inside, class `in`), the two-thumb rail, and marks SFC · TA · FL 100 · FL 195 in `mono-caption` `color-text-tertiary`. ← and → move the focused thumb.

**Data.** The histogram counts the store's aircraft by level band; the range filters the list and the map together.
