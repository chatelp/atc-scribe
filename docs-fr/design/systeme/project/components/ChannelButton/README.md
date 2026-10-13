# ChannelButton

A channel button plays or mutes one frequency in this browser, at the head of the thread.

**Markup.** `<button class="cl-channel" aria-pressed>`: the speaker icon, `cl-channel__freq` (mono, three decimals), the frequency's name, `cl-channel__age` (last transmission). States: `is-on` (heard here: accent outline and icon), muted (default), `is-connecting` and `is-failed` with a `cl-channel__status` word, `is-off-air` (a known frequency the station does not receive now: dashed, not a button, "not on air").

**Split variant** (`cl-channel--split`, in `ThreadHead`): `cl-channel__main` toggles listening, `cl-channel__caret` (▾) opens the `ChannelMenu` with the volume; `is-menu-open` while it is open. In the narrow thread the age is left to the menu.

**Rules.** The station chooses what is on air; the page only plays or mutes what it receives. No antenna switch here. A failure stays at full opacity: a failure does not fade. A mixed stream with no single frequency shows its name only ("mixed").

**Data.** `/api/v1/status` `antenna` (what is on air), `/frequencies` (names), the page's audio clients (level, last audio).
