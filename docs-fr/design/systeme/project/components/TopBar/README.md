# TopBar

The top bar holds the sign and the name, what the station has on air, the chain's lights, the time, and the tools.

**Markup.** `<header class="cl-topbar">`: `cl-topbar__brand` (the sign at 20 px, `cl-topbar__name` "Clairance" in mono 600), `StatusLight`s, `cl-topbar__utc` (accent, "13:13:20Z") and `cl-topbar__local`, icon buttons (event log with its `Counter`, settings).

**Rules.** The sign is the listening state (lit scope) while a frequency plays in this browser, the running state otherwise. The clock that times the interface is in `color-accent-default` (UTC by default); which one is chosen in Settings › Display. What is on air is chosen at the station: say so, offer no switch. `size-bar` high. When following a flight, a callsign chip with ✕ sits after the brand. **On the phone** (under 768 px) the bar keeps the sign, one light (the antenna's, cut with an ellipsis), the UTC clock, the bell and the settings, at 44 px: the name and the local time go, and the other lights show only when they warn or fail. It is never wider than the screen: the bell and the settings are the only way to the event log and the settings there.
