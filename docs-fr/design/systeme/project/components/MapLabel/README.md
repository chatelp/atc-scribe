# MapLabel

A map label names an aircraft on the map: phase, callsign and type; its level and speed; and, for ten minutes after it was heard, what it last said.

**Markup.** `<span class="cl-maplabel">` with up to three lines: `<span class="cl-maplabel__ph--<family>">APP</span> <b>AFR47KX</b> A320`; the level, its trend and the ground speed ("2 400 ft ↓ 158 kt"); optionally `<span class="cl-maplabel__said">` with the last transmission in quotes, cut at 40 characters. `is-selected` adds the accent outline. The icon is drawn by the map renderer from our aircraft family (assets › Aircraft), its fill on the altitude ramp (`color-altitude-dark-*`).

**Rules.** The label carries the phase colour, the icon the altitude lightness: two codes, two objects. The third line is part of the label, drawn with the other two, never a separate bubble that would cover them; it shows for ten minutes after the transmission, and the setting "Last transmission on aircraft labels" (key `voiceBubbles`) turns it off. A click on the aircraft opens its story. The aircraft that just spoke wears a green ring that fades over 10 s.
