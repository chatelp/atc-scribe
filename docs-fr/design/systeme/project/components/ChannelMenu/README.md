# ChannelMenu

The channel menu is one frequency's popover: its state, its volume, and whether the thread shows only it.

**Markup.** Opened by the caret of a split `ChannelButton` (`cl-channel--split`, `cl-channel__main` toggles listening, `cl-channel__caret` opens the menu, `is-menu-open` while open). The menu is `cl-menu`: `cl-menu__head` (frequency in mono, name), a `StatusLight`, a `cl-menu__row` with the volume `Slider` and its percentage, `cl-menu__sep`, a `Checkbox` "Only this frequency in the thread", and a `cl-plain` line (last transmission, count today, who chose it).

**Rules.** The menu opens under its own button, its left edge on the button's (inline position, as every popover). Volume is kept per frequency by the browser, as today. A failed frequency shows "■ failed" here; the page retries on its own, as the bottom bar did, so there is no Retry button. Closes on Esc or a click outside.
