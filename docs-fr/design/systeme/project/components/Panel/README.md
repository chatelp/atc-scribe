# Panel

Surfaces hold the components: a panel for the thread and the column, a card for what floats over the map, a page for the settings.

**Panel** `cl-panel`: `color-surface-panel`, `color-border-default`, a `cl-panel__head` (`size-panel-head`, `cl-panel__title` in `text-title`, quiet actions).

**Card** `cl-card`: `color-surface-raised`, `radius-lg`, `shadow-overlay` (the only shadow, because the map can be light): the folded Right now, popovers, menus.

**Page** (settings): `color-surface-page` on `color-scrim`, one 640 px column, closed by Esc or ✕.
