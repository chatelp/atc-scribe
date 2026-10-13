# SignIn

The sign-in page is the first thing seen when the station requires a password; the first run asks who may open Clairance.

**Markup.** `cl-signin` › `cl-signin__card`: `cl-signin__mark` (the sign at 40 px and the name, mono 600, 26 px), the promise in one `cl-plain` sentence, `cl-signin__field`s (a `cl-label` and a `SearchField`-styled input), a primary `Button`, and the limit ("After 8 failed tries, wait 15 minutes"). The first run swaps the fields for two radio choices (`Checkbox`, `cl-check--radio`): this machine only, or the network with a password.

**Rules.** The same surfaces and the same green as the tool: the old page's own palette goes.
