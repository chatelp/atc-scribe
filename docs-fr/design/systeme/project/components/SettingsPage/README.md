# SettingsPage

The settings page opens over the screen: a navigation on the left, sections of fields on the right.

**Markup.** `cl-page` (on `color-scrim`) › `cl-page__inner` › `cl-page__nav` (links, `aria-current` on the section in view) and `cl-page__body`: `cl-page__head` (`cl-page__title`, a `StatusLight` "Saved", "Reload from the server", close), then `cl-page__section`s with an `<h3>` and `cl-field`s. A field is `cl-field__label`, its control (`Switch`, `Checkbox`, `cl-select`, `SegmentedControl`, `SearchField`) and an optional `cl-field__help` sentence.

**Sections.** Listening (on air, reference airport, also follow), Callsign matching (three presets; the eleven rules folded behind a `Disclosure`), Display (this browser: speech bubbles, smoothing, times in UTC or local), Account, System (log level, database size, the read-only reports, folded).
