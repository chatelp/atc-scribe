# TopBar

The top bar holds the sign and the name, what the station has on air, the chain's lights, the time, and the tools.

**Markup.** `<header class="cl-topbar">`: `cl-topbar__brand` (the sign at 20 px, "Clairance" in mono 600), `StatusLight`s, `cl-topbar__utc` (accent, "13:13:20Z") and `cl-topbar__local`, icon buttons (event log with its `Counter`, settings).

**Rules.** The sign is the listening state (lit scope) while a frequency plays in this browser, the running state otherwise. What is on air is chosen at the station: say so, offer no switch. `size-bar` high. When following a flight, a callsign chip with ✕ sits after the brand.
