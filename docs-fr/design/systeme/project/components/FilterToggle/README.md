# FilterToggle

A filter toggle narrows what the list and the map show: Air, Ground, Heard, a phase family, an altitude preset.

**Markup.** `<button class="cl-toggle" aria-pressed>` with an optional `cl-toggle__count` (mono). A phase filter starts with its `cl-dot--<family>`: the colour never stands alone, the code or name is written.

**States.** Off: `color-border-control` outline, `color-text-secondary`. On (`is-on`): `color-surface-selected`, `color-accent-default` outline, `color-accent-strong` text. One way to say "on", no inner shadow, no transparency.

**Mixed** (`is-mixed`, `aria-pressed="mixed"`): a family filter whose codes are partly on (the digit keys 1 to 0 still act per code): dashed `color-accent-default` outline, no fill, `color-text-primary`. A click turns the whole family on.

**Labels.** Short enough for one row of five in the 380 px column: "Gnd", "Dep", "Crz", "Arr", "?"; the full name ("Departure: take-off, climb, departure") goes in the `title` and the help bubble.

**Data.** Counts come from the store's filters (`on_ground`, `voiceAircraftCount`, phase families).
