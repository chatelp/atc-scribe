# PhoneShell

The phone shell shows one pane at a time, with a tab bar at the bottom: Radio (home), Sky, Traffic, Right now.

**Markup.** `cl-phone` › `cl-phone__bar` (the sign, what is on air, UTC), `cl-phone__pane`, `cl-tabbar` of `cl-tabbar__tab`s (icon and word, `aria-selected`). Inside a phone frame, and on any screen under 768 px, controls take the touch sizes: `size-control-touch`, `size-target-touch`, `size-row-touch`.

**Rules.** The thread is home. Selecting an aircraft on the map moves to Traffic with its story. Settings hide the tab bar.
