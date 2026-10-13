# MapLabel

A map label names an aircraft on the map: phase code, callsign, and when selected its level and speed.

**Markup.** `<span class="cl-maplabel">` with `<span class="cl-maplabel__ph--<family>">APP</span> <b>AFR47KX</b>`, a second line for the selected one; `is-selected` adds the accent outline. The icon is drawn by the map renderer from our aircraft family (assets › Aircraft), its fill on the altitude ramp (`color-altitude-dark-*`).

**Rules.** The label carries the phase colour, the icon the altitude lightness: two codes, two objects. The aircraft that just spoke wears a green ring that fades over 10 s.
