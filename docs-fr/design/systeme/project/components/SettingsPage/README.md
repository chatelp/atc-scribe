# SettingsPage

The settings page opens over the screen: a navigation on the left, sections of fields on the right.

**Markup.** `cl-page` (on `color-scrim`) › `cl-page__inner` › `cl-page__nav` (links, `aria-current` on the section in view) and `cl-page__body`: `cl-page__head` (`cl-page__title`, a `StatusLight` "Saved", "Reload from the server", close), then `cl-page__section`s with an `<h3>` and `cl-field`s. A field is `cl-field__label`, its control (`Switch`, `Checkbox`, `cl-select`, `SegmentedControl`, `SearchField`) and an optional `cl-field__help` sentence.

**Sections.** Listening (on air, reference airport, also follow), Callsign matching (three presets; the eleven rules folded behind a `Disclosure`), Display (this browser: speech bubbles, smoothing, times in UTC or local), Account, System (log level, database size, the read-only reports, folded).

**Phone.** The nav becomes a scrolling row above the body. A field whose value is a sentence (On air) takes `cl-field--text` and goes under its label; a switch stays on the label's line.

**Names from the AIP** (Q55, 15/10), in Callsign matching, after the eleven rules:
- a field "Names from the AIP" with a `StatusLight` and its help ("The official names around the station, from the French eAIP: reporting points, control positions, procedures. The matcher reads them so that "ELCOB 7 Golf" is a procedure, not a callsign."). The light in five states: ● "Up to date · AIRAC 2026-10-01"; ▲ "AIRAC 2026-10-29 in force; this list is of 2026-10-01"; – (`--off`) "Not made yet: the matcher works without it"; ◐ (`--switching`) "Updating from the SIA…"; ■ "Last update failed at 14:20Z: <why> (the list of 2026-10-01 stays in use)";
- `KeyValues`, read only: reporting points, control positions, procedures (counts), the area ("120 NM around the station") and the aerodromes ("LFPG · LFPO · LFPZ (the ones followed)");
- a `cl-row`: the button "Update from the SIA", `cl-btn--primary` when the list is out of date or absent, plain `cl-btn` when it is up to date, disabled while updating; beside it, `cl-plain` "Five public pages of the eAIP, about 9 MB, a few seconds.";
- the source line the SIA asks to cite, always shown when there is a list, `cl-plain`: "Source: SIA (DGAC), eAIP France, AIRAC cycle of 2026-10-01: ENR 4.4, ENR 2.1, AD 2 LFPG, LFPO, LFPZ. Made on 15/10 at 14:20Z. Next cycle: 2026-11-26.";
- a field with a `Switch`, "Update at each new AIRAC cycle", **off by default** (a request to an outside site: the server asks nothing of the SIA until the button is pressed or this is turned on), help "Every 28 days, when a new cycle comes into force, the server downloads the pages again."
