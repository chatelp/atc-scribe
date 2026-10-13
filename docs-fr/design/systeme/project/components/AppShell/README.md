# AppShell

The app shell lays out the screen: the top bar, then the thread, the sky and the context column; folded, the column goes and the thread widens.

**Markup.** `<div class="cl-app">` (add `is-folded` for layout B) holding `TopBar` and `<div class="cl-app__main">` with `<section class="cl-app__thread">`, `<section class="cl-app__sky">`, `<aside class="cl-app__column">`. Inside them `cl-app__bottom` (content that grows from the bottom: the thread) and `cl-app__scroll` (content that scrolls from the top). Over the map: `cl-sky__top` (the view switch, a `cl-sky__note` such as "Showing 64 of 107", `cl-sky__end` for cards) and `cl-sky__corner` (layers).

**Sizes.** Thread ≈ 440 px (640 folded), column ≈ 380 px, the sky takes the rest; under 768 px one pane at a time (`PhoneShell`).
