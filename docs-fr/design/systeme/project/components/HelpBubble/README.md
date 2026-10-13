# HelpBubble

A help bubble explains a term to a newcomer, on demand: TA, QNH, a runway name, a phase code.

**Markup.** The term carries `cl-help` (a dotted `color-text-tertiary` underline) and `data-help-title` / `data-help` (help-tips.js); the bubble is `cl-bubble` with a `cl-label` title and a `text-small` paragraph in the system font.

**Rules.** Opens after 400 ms of hover or on keyboard focus on the desk, a 500 ms long press on the phone. At most 280 px wide. Every confusing element gets one; none is shown permanently.
