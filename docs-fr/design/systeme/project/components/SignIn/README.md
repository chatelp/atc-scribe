# SignIn

The sign-in page is the sky: the whole page is the sign's scope, the name stands large, the station's console shows it is alive, and the form sits at the centre. The first run asks, in the same frame, who may open Clairance.

**Markup.** `cl-signin cl-signin--sky` › the background `<svg class="cl-signin__scope">` (the sign's scope at the size of the page: range rings every 150 px around the centre, the sweep still at 45° with its afterglow at 10 %, one blip, a few silhouettes of our family on the altitude ramp; drawn by `tools/design/connexion.py`, `scope_svg`), `cl-signin__brand` (the sign at 64 px and the name, mono 600, 56 px), `cl-signin__console` with `StationVitals`, `cl-signin__card` (the promise, `cl-signin__title`, `cl-signin__field`s, a primary `Button`, the limit "After 8 failed tries, wait 15 minutes"), `cl-signin__foot` (the second promise). The first run swaps the fields for two `cl-check--radio` choices: this machine only, or the network with a password.

**Rules.** Chosen by the owner on 13/10 among three directions (`docs-fr/design/connexion/`). Nothing moves: the sweep is still (principle 5). Nothing real is drawn in the sky: the silhouettes are decoration, never positions. Under 768 px the brand shrinks, the card takes the width, the console follows it. The plain `cl-signin` (a card on the page) stays for a page without the sky.
