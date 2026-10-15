# AppShell

The app shell lays out the screen: the top bar, then the thread, the sky and the context column; folded, the column goes and the thread widens.

**Markup.** `<div class="cl-app">` (add `is-folded` for layout B) holding `TopBar` and `<div class="cl-app__main">` with `<section class="cl-app__thread">`, `<section class="cl-app__sky">`, `<aside class="cl-app__column">`. Inside them `cl-app__bottom` (content that grows from the bottom: the thread) and `cl-app__scroll` (content that scrolls from the top). Over the map: `cl-sky__top` (the view switch, a `cl-sky__note` such as "Showing 64 of 107", `cl-sky__end` for cards) and `cl-sky__corner` (layers).

**Sizes.** Thread ≈ 440 px (640 folded), column ≈ 380 px, the sky takes the rest; under 768 px one pane at a time (`PhoneShell`).

**Keyboard.** The first stops are three skip links in `cl-skip` ("Skip to the map", "Skip to the column", "Skip to the radio thread"), off screen until focused: the thread holds up to 200 exchanges, each with its play button. Whatever opens over the screen takes the focus and gives it back: the settings page is a modal dialog (`aria-modal="true"`, the rest `inert`, Tab stays inside, Esc or × closes and returns the focus to the gear); a popover (event log, frequency menu, layers, an airport's card) puts the focus on its first control, Esc closes it and returns the focus to the button that opened it.
