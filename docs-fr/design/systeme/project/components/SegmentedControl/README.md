# SegmentedControl

A segmented control picks what the sky shows: the map, or the PAR of one followed airport.

**Markup.** `<div class="cl-seg">` holding `<button class="cl-seg__item" aria-pressed>`. On the phone it switches panes (Thread · Sky · Traffic).

**States.** The current segment takes `color-surface-selected` and `color-accent-strong`; the rail is `color-surface-panel` with a `color-border-default` outline. "PAR" is part of each segment's label, never a button-looking label of its own.
