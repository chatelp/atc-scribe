# LevelMeter

The level meter shows that someone is speaking on a frequency now.

**Markup.** `<span class="cl-level"><i style="height:6px"></i>…</span>`, five 2 px bars in `color-accent-default`, heights driven by the frequency's audio level (0 to 1).

**Rules.** It moves because the sound moves; it is the only thing that does. Say "someone is speaking", never when the text will arrive: the server cuts a transmission when it ends and knows no more.
