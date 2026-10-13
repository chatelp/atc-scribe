# KeyValues

Key values list an object's facts as label / value pairs; a disclosure folds a group of them.

**Markup.** `<dl class="cl-kv">` with `<dt>` (label, `color-text-secondary`) and `<dd>` (value, mono, `color-text-primary`; `is-none` writes "—" once in `color-text-tertiary`). A group is folded behind `<button class="cl-disclosure" aria-expanded>` (▸ / ▾).

**Used by.** The aircraft's Details tab (Basic info, Position, Speed and direction, Navigation and weather, ADS-B and signal), an airport or navaid card on the map.
