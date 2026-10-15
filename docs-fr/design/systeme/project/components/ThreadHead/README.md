# ThreadHead

The thread head holds what governs the thread: its title and pace, search, mute all, the channel buttons, and which frequencies it shows.

**Markup.** `<div class="cl-threadhead">` with `cl-threadhead__row`s: the title (`cl-threadhead__title`) and "5 exchanges in the last 2 min"; `cl-threadhead__end` with search (icon button) and "Mute all" / "Listen to all"; the `ChannelButton`s (split); "Show" with a `SegmentedControl` All · one per frequency.

**Rules.** It replaces the bottom radio bar: per-frequency transcript popovers become this one thread, filtered here; their search becomes the thread's search. When the reader has scrolled up, new exchanges gather in one `NewBelow` pill, not in per-frequency counters.

**Classes.** The pace is `cl-threadhead__pace`, the "Show" row `cl-threadhead__row--show`. **On the phone** the title and the pace go (the tab bar says Radio) and so does "Show" (each frequency's menu has "Only this frequency in the thread"): two rows, the tools and the channels.
