# Fiche K — rapport (13/10)

Branche **`claude/ecstatic-shannon-snaiwt`** (et non `cloud/k-then-sans-verdict` : la session cloud est tenue de pousser sur la branche que lui assigne le harnais, comme pour L), partie de `main` (c783e88). Rien dans `www/` hors de `www/story/`, rien dans `docs/`, aucun fichier Go.

| Commit | Ce qu'il fait | Lignes |
|---|---|---|
| 76605e6 | `www/story/then.js` (logique pure, `window.StoryThen` / `require`) et `then.test.js`, 43 cas | +447 / +383 |
| 334af83 | `tools/www-check/check.mjs` lance aussi `www/story/` | 3 modifiées |

## Écarts à la fiche, à regarder

1. **La fonction s'appelle `StoryThen.describe(instruction, track, options)`, pas `then`** : un objet qui porte une méthode `then` est un *thenable* ; `await import(...)`, ou une fonction `async` qui le rend, l'appellerait. Signature et retour `{ text, at, kind }` inchangés.
2. **La route des traces sert `altitude` et `speed_gs`, pas `alt_baro` et `gs`** (`adsb.Position`, `models.go:274` ; `altitude` y est bien `alt_baro`, 0 au sol). Le module lit les deux jeux de noms, et `mag_heading` quand il existe (le cap magnétique que donne le contrôleur), sinon `track` (route sol).
3. **`at` n'est pas dans `plain`** : le store devra le poser depuis l'heure de la transcription. Les heures sont lues en RFC 3339, `Date` ou ms ; `at` rendu est en ms (`null` pour « no later transmission heard here »).
4. **Sans `frames`, la lettre de piste tombe** : « Cleared to land, runway 27R » suivi d'un atterrissage au cap ≈ 270° rend « landed on 27 » — un cap ne distingue pas deux parallèles. Avec `options.frames` (`ParGeometry.buildFrames`, ce que la PAR calcule déjà), la piste est nommée là où l'avion a touché : « landed on 27R », ou « 27L » si c'est là qu'il a posé (un fait, pas un écart).
5. **La vitesse comparée est la vitesse sol** (la seule servie) ; la consigne est en vitesse indiquée. À FL 100, 220 kt IAS ≈ 255 kt sol sans vent : le module dira « 255 kt, 6 min later », exact mais loin du chiffre entendu. Demander `ias` dans les traces si cela gêne à l'écoute.
6. **`www-check` est rouge sur `main` avant moi** : l'empreinte de `www/vendor/tailwind.css` est périmée. Mesuré : la CSS compilée (tailwindcss 3.4.17) est identique à l'octet avec et sans `www/story/`, et identique à celle de `main`. `tools/tailwind/build.sh` réécrit la ligne ; je ne l'ai pas lancé (`www/vendor` hors fiche).

## Les phrases possibles (pour `AircraftStory`)

`L` = « 4 000 ft » (U+202F) sous l'altitude de transition, « FL 110 » au-dessus, via `AltitudeScale.reading/readingText` ; `D` = « , 2 min later » ou « , 40 s later » (pas de 10 s, minutes dès 55 s).

- **level** : `level at L, D` (atteint et tenu 30 s) · `at L, D` (déjà là, ou aucun mouvement) · `climbing|descending through L, D` (passé sans s'arrêter : niveau demandé, instant interpolé) · `climbing|descending through L` (en route à la fin de la fenêtre ; `, D` si la trace s'arrête avant : signal perdu, ou maintenant) · `on the ground, D`
- **heading** : `turned to 270°` · `turning through 270°, D` · `heading 254°, still turning` · `heading 254°, D`
- **speed** : `225 kt, D` · `slowing|accelerating through 250 kt, D` · `slowing|accelerating, 241 kt` (`, D` si la trace s'arrête)
- **frequency** : `next heard on 119.255, D` · `next heard, D` (fréquence inconnue) · `no later transmission heard here`
- **runway**, **clearance** : `landed on 27R` · `took off from 09`
- **null** : pas de valeur, aucun point dans la fenêtre, une QNH, un squawk, « Passing »/« Leaving » (un compte rendu, pas une demande), une fréquence sans la liste `transmissions`, une piste sans transition sol/air visible.

Règles : **tenu** = tous les points des 30 s suivantes dans la tolérance et leur écart max-min ≤ tolérance (un virage lent à travers ± 10° n'est pas un cap tenu) ; **en mouvement** = ≥ 300 ft/min, ≥ 5° en 30 s, ≥ 5 kt en 60 s, comparés au point le plus proche de cette durée en arrière (la route dédoublonne les positions) ; **atterri** = un point au sol après un point en vol, **décollé** l'inverse ; la piste sans `frames` = route à ± 20° de son numéro × 10, avec `frames` = point au sol à moins de 0,05 NM de l'axe et l'extrémité dont l'avion suivait la direction. « Cleared for takeoff » cherche un décollage, les autres clairances un atterrissage, « Runway 27 » seul les deux. Le niveau est comparé sur 1013 hPa pour un FL, sur le QNH pour des pieds (le standard si `qnh` manque).

## Extraction de la valeur quand `value` manque (`StoryThen.valueOf`)

`value`/`unit` du serveur priment. Sinon, dans `text`, par rôle : `flight_level` « FL100 », « FL 100 », « flight level 100 » ; `altitude` « 4000 ft », « 4 000 ft », « 4,000 feet », « 500 ft » ; `heading` « heading 270 », « heading 024 » (0–360) ; `speed` « 220 kt », « 220 knots » ; `frequency` 118.000 à 136.999, écrite à trois décimales ; `runway`/`clearance` « runway 27R », « runway 09 » (une clairance sans piste reste possible, nommée par les `frames`). Sinon `null`.

## Vérifié, et comment

- À chaque commit : `node --check`, `node --test www/par/ www/story/` (68 verts) ; `go build`, `go vet`, `go test -race ./...` verts sauf `TestAChoiceThatCannotBeWrittenChangesNothing` (`internal/auth`), rouge en root **déjà sur `main`** (relevé par G).
- Les 43 cas : chaque cas nommé par la fiche (niveau atteint, non atteint, aucun point après → null, signal perdu, sans valeur → null, QNH donné et non donné) et chaque branche. Deux tests sur **toutes** les sorties : chaque texte produit est une des 16 formes listées et chaque forme est produite ; aucun mot de verdict, ni sur les textes ni sur les formes, « still » seulement dans « still turning » ; format des nombres (U+202F, jamais « 4,000 » ni « FL110 », « , N min later »).
- Mutations : « still not there » dans le module, « FL110 » dans `altitude-scale.js` → les tests tombent ; `www-check` signale l'échec de `www/story/`.

## Pas vérifié

Rien sur de vraies traces : seulement des trajectoires synthétiques. Ni navigateur, ni station, ni production. Les seuils (300 ft/min, 5°, 5 kt, 30 s, 0,05 NM, ± 20°) sont des choix, pas des mesures.

## À regarder en priorité sur le Mac

1. Rejouer `describe` sur des consignes réelles et leurs traces (base de la station, sur le Mac) : la part de `null`, et si « level at » tombe au bon moment malgré la densité réelle des points.
2. La vitesse sol face à la vitesse indiquée (écart 5) : la phrase peut sembler fausse à l'écoute.
3. Le chargement dans la page : `then.js` après `altitude-scale.js` et `par-geometry.js` ; passer les `frames` de la PAR et la liste des transmissions de l'avion.
4. `tools/tailwind/build.sh` à l'intégration (écart 6).
