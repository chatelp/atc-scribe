# PhaseBadge

A phase badge writes an aircraft's flight phase in the colour of its family.

**Markup.** `<span class="cl-phase cl-phase--arr">APP</span>`; families `--gnd` (TAX), `--dep` (T/O, CLB, DEP), `--crz` (CRZ), `--arr` (ARR, APP, T/D), `--unk` (NEW, UNK). T/O and T/D add `cl-phase--fill`. A dot `cl-dot cl-dot--<family>` stands for the family in a filter.

**Rules.** Five colours for ten codes: the code is always written, so the colour is never alone. The same badge on the map label, the list, the chip, the PAR.
