# Button

Buttons start an action: primary for the one action of a screen, secondary for the others, quiet inside panels, icon for tools.

**Markup.** `<button class="cl-btn">`, plus `cl-btn--primary`, `cl-btn--quiet` or `cl-btn--icon`. An icon button holds a 14 px inline SVG and an `aria-label`.

**States.** Hover `color-surface-hover`; focus ring `color-border-focus`; `disabled` turns to `color-text-disabled`. Height `size-control-desk`, `size-control-touch` under 768 px.

- Do: one primary per screen at most (sign in, save).
- Do: put a close button (icon, ✕) on every card and page.
- Don't: use a primary button for a filter or a tab; those are `FilterToggle` and `Tabs`.
