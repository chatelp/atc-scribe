# Tabs

Tabs switch between views of one place: the context column (Right now, Traffic), the list (Active, Signal lost), the settings sections.

**Markup.** `<div class="cl-tabs" role="tablist">` holding `<button class="cl-tabs__tab" role="tab" aria-selected>`; an optional `cl-tabs__count`. The context column uses `cl-tabs--column`: `size-tabs-head` high, `text-body` labels, and a fold button (`cl-btn cl-btn--icon`, »/«) in `cl-tabs__end`.

**States.** The current tab is `color-text-primary` underlined with 2 px `color-accent-default`; the others `color-text-secondary`. No background.

- Do: keep two to four tabs.
- Don't: use tabs to filter; that is `FilterToggle`.
