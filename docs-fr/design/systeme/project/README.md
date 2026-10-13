Clairance lets a listener hear the airband their station receives, read it in English and French, and see which aircraft is speaking. It runs on the listener's own machine and judges no one. This system is how its screens are built: one dark theme, one green, two type families, and components written as plain CSS classes (prefix `cl-`) that the interface binds with Alpine.

## Content

- **Interface text is English; transcripts keep their language.** A French transmission stays French, marked `lang="fr"`, and gets its plain-French explanation under it. Instructions read in sentence case, verbs first: "Descend to FL 070", "Contact 120.650", "✓ Cleared ILS runway 26L".
- **Say what happened, never whether it was right.** Write "↩ Pilot replied, 4 s later", never "readback correct". Nothing in the interface grades a controller or a pilot.
- **Say where a fact comes from when it is inferred.** The runway in use and the arrival sequence are read from ADS-B tracks: the summary carries "Read from the ADS-B tracks, not announced by the controllers." An uncertain match says so: "probably TVF61WB", "AFR47KX or AFR52PB: left open".
- **Write for a newcomer, explain on demand.** Every term a newcomer would stumble on (TA, QNH, FL, "26L") carries a help bubble (`HelpBubble`); the screen itself stays terse.
- **Numbers.** Levels as flight levels from the transition level up ("FL 070"), in feet on the QNH below it ("2 400 ft"). Frequencies "124.350" (three decimals), speeds "158 kt", distances "6.2 NM", headings "270°", times "13:12:31" in UTC with a trailing "Z" where UTC and local time sit together. A thin space separates thousands; the unit follows the value in `text-label` size and `color-text-tertiary`.
- **No emoji, no exclamation marks.** Glyphs that carry meaning are few and fixed: ✓ (a clearance), ↩ (a reply), ↑ ↓ ↔ (climbing, descending, level), ▲ ■ ● ○ (status shapes), → (from speaker to addressee).

## Colour

- **One theme, dark.** The page is `color-surface-page`, panels `color-surface-panel`, floating things `color-surface-raised`, the map and the PAR `color-surface-map`. Hover is `color-surface-hover`; selection is the opaque `color-surface-selected`, never a transparency of green.
- **Text has three levels**: `color-text-primary` for values, callsigns and transcripts; `color-text-secondary` for labels and secondary values; `color-text-tertiary` for units, times and hints. All three hold 4.5:1 on every surface up to `color-surface-hover` (tertiary is the floor, 4.7:1). `color-text-disabled` is for disabled controls only.
- **One green, and it means the tool's voice**: `color-accent-default` is the brand, what is live (a frequency being heard, a chain that works), what is chosen, and the focus ring. `color-accent-strong` is accent text on `color-surface-selected`. Never use green for "pilot" or "clearance".
- **Statuses always carry a shape and a word**: ● ok (`color-status-ok`), ▲ attention (`color-status-warn`), ■ failure (`color-status-urgent`), ○ unknown (`color-status-unknown`). An urgent fill is the dark `color-status-urgent-fill`, outlined with `color-status-urgent`, white text: a red and the green differ by lightness, not hue alone.
- **Five phase families**, the same on the map, the list and the PAR: ground `color-phase-ground` (TAX), departure `color-phase-departure` (T/O, CLB, DEP), cruise `color-phase-cruise` (CRZ), arrival `color-phase-arrival` (ARR, APP, T/D), unknown `color-phase-unknown` (NEW, UNK). The code is always written; T/O and T/D get a filled badge.
- **Aircraft icons on the map take their altitude as lightness, not hue**: `color-altitude-dark-very-low` (brightest) to `color-altitude-dark-cruise` on dark base maps, the `color-altitude-light-*` ramp on light ones. The label says the phase; the icon says the height.

## Type

- **Two families, no font shipped.** `family-text` (the system font) for what is read: transcripts, explanations, help, titles. `family-mono` for what is compared: callsigns, levels, frequencies, speeds, distances, times, codes.
- **Five sizes, floor 11 px**: `text-label` / `mono-caption` 11/14, `text-small` / `mono-small` 12/16, `text-body` / `mono-body` 14/20, `text-title` 16/22, `text-display` / `mono-display` 20/26. Weights are 400 and 600 only: 600 says "this is the value" or "this is a clearance", never "this title matters".
- Field labels are `text-label`: capitals, `font-tracking-label`, `color-text-secondary`.

## Space, shape, depth

- Spacing steps `space-0-5` to `space-8` (2, 4, 6, 8, 12, 16, 24, 32 px). Half steps exist up to 8 px because the screen is dense.
- Radii: `radius-sm` for badges and bars, `radius-md` by default, `radius-lg` for what floats over the map, `radius-full` for counters and slider thumbs.
- Depth is said by surface and border, not shadow: a shadow is invisible on black. Only what floats over the map (whose base can be light) takes `shadow-overlay`.
- Borders: `border-width-default` everywhere; `border-width-strong` for focus, the selected aircraft and the current tab.

## Layout

- **The thread in the middle of attention.** Desk layout A: the radio thread (≈ 440 px), the sky (map or PAR), and a context column (≈ 380 px) with two tabs, "Right now" (default) and "Traffic". The map follows the Traffic filters.
- **Fold, don't switch.** » folds the column (layout B): the thread widens into a transcript, Right now becomes a `cl-card` over the map with "Traffic" and « to unfold. Following a flight replaces the column's content with its story; ‹ returns.
- **No bottom bar.** Its parts move: the frequencies to the `ThreadHead` (listen or mute, a `ChannelMenu` with the volume, one thread for all of them with a frequency filter and a search), the weather to Right now (`Weather`), the runways in use to the summary and the `ParView`, the lights and the clocks to the `TopBar`, the choice of UTC or local time to Settings › Display.
- **Phone**: one pane at a time (`PhoneShell`): Radio, Sky, Traffic, Right now; the thread is home.
- Desk targets `size-target-desk` (24 px), phone `size-target-touch` (44 px); rows `size-row-desk` / `size-row-touch`. The breakpoint is 768 px.

## States and motion

- Hover `color-surface-hover` in `motion-duration-fast`; focus a solid 2 px `color-border-focus` ring, offset 2 px, on `:focus-visible` only; disabled `color-text-disabled`.
- What moves is information: aircraft, a frequency's level meter. A change of state takes `motion-duration-fast` with `motion-easing-standard`; a popover opens in `motion-duration-base`. An emergency pulses three times and stops. Nothing loops. Under `prefers-reduced-motion` durations drop to 0.
- A new line in the thread or the list appears; it does not slide or blink.

## Iconography

- **The sign** (assets › Logos): the radar scope. Three states that differ by shape: stopped (empty scope), running (`signe`, the sweep and one blip), listening (`signe-actif`, the lit scope) when a frequency plays in this browser. Use `signe-16` below 24 px. Never animate it in a loop, never add blips.
- **Aircraft** (assets › Aircraft): our own family of nine silhouettes drawn from real dimensions, single black ink, tinted by the map renderer. Use the class silhouette, never a picture of a real type.
- **Interface glyphs** are inline SVG on a 16 grid, 1.5 px strokes, `currentColor`. Font Awesome 6 is what `www/` uses today (54 icons): keep its icons until each is replaced, never mix FA5 names.

## Accessibility

- Text 4.5:1 on its surfaces, control outlines and focus 3:1: checked by `tools/design/tokens.mjs` for every pair, failing the build if one regresses.
- Colour is never alone: phase codes are written, statuses have shapes and words.
- What is clicked is a `<button>`; tabs have `role="tab"`; icon buttons an `aria-label`; transcripts in French `lang="fr"`.
- Tab moves between controls; J / K (or ↓ / ↑ in the list) move between aircraft.

## Using it

Load the tokens (`tokens.css`, generated from `tokens.json`) then `components/bundle.css`, and wrap the interface in `.cl-root`. Components are classes with BEM-style parts (`cl-exchange__head`) and state classes (`is-selected`, `is-on`) to bind with Alpine's `:class`. Values come from `docs-fr/design/jetons.json` in the repository: change them there, run `tools/design/tokens.mjs` (contrast checks) then `tools/design/ds-export.mjs`, and republish; an edit made only here is overwritten by the next export.
