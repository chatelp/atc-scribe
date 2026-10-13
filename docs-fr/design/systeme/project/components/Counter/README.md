# Counter

A counter counts what is unread (event log, a frequency); an urgent badge names an emergency.

**Markup.** `<span class="cl-counter">3</span>`, `cl-counter--urgent` as soon as one counted item is urgent. Emergencies: `<span class="cl-urgent-badge">MAYDAY</span>` or a squawk `<span class="cl-squawk">7700</span>`.

**Rules.** Ordinary unread counts are green (`color-accent-fill`, `color-text-on-accent`); red only when an emergency is counted. The urgent badge is the dark `color-status-urgent-fill` outlined with `color-status-urgent`, with its ■.
