# FilterToggle

A filter toggle narrows what the list and the map show: Air, Ground, Heard, a phase family, an altitude preset.

**Markup.** `<button class="cl-toggle" aria-pressed>` with an optional `cl-toggle__count` (mono). A phase filter starts with its `cl-dot--<family>`: the colour never stands alone, the code or name is written.

**States.** Off: `color-border-control` outline, `color-text-secondary`. On (`is-on`): `color-surface-selected`, `color-accent-default` outline, `color-accent-strong` text. One way to say "on", no inner shadow, no transparency.

**Data.** Counts come from the store's filters (`on_ground`, `voiceAircraftCount`, phase families).
