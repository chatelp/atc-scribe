# Fiche M — rapport (session cloud du 13/10)

**Branche : `claude/loving-ride-qvtxnh`**, pas `cloud/m-mouvement-ecran` : comme pour L, la session cloud pousse sur la branche qui lui est attribuée. Partie de `main` (37a88b3), 13 commits, rien sur `main`, pas de pull request.

## Commits (chiffres du banc navigateur, sauf mention)
| Commit | Ce qu'il fait | Chiffres |
|---|---|---|
| c1e3307 | **1** Générateur sorti dans `internal/adsb/simtraffic` (12 avions de L inchangés, flotte réglable, trou de 18 s à 45 NM) | enregistrement de L régénéré **identique à l'octet** |
| 3e85c75 | **1** Banc `tools/motion-bench` + référence `A-before` | +1 366 lignes, rien hors du dossier |
| f90aec9 | Un avion apparu page ouverte n'était **jamais animé** (absent de `visibleAircraftOnMap` jusqu'au prochain déplacement de carte). Corrigé en premier : masquait tout le reste | b : erreur 1 479 → 29 m, p95 14 km → 22 m |
| 5603e44 | **2** Première minute : cause **côté page** (redémarrage serveur seul : 40 m 3 s ; rechargement seul : tout le défaut). Chargement initial daté à l'arrivée + pas de vitesse avant le 2ᵉ relevé | rechargement : 1ʳᵉ s 310 → 98 m, stable à 3 s au lieu de 8 |
| 327ddd6 | **2** Onglet montré : plus de balayage, la pose repart de l'état courant (`resumeAfterMs`) | 3 s après : sauts 80 → 1,6 %, erreur 1 875 → 12 m |
| 6e41fe1 | **2** Long trou : au-delà de **1,67 s de vol en arrière** ou **3,33 s** dans tout sens (déduits du plafond de 2,5 s), plus de glissade : fondu 150 + 150 ms | avion du trou : sauts 29 → 0,5 %, corr. max 4 158 → 393 m |
| 3badbfd, 56279bb | **3** Stratégies B et C en option (défaut A), références A/B/C | voir plus bas |
| 835b32c | **4** Le moteur tournait à **20 i/s** au lieu de 30 (seuil `>=` sur des images de 16,7 ms) | une pose toutes les 49,7 → 33,3 ms |
| 7fbde62 | **4** Pas d'image périmée à la création d'un symbole (450 m d'un coup) ni au retour d'onglet (3,7 km) | é.-t. écran a 245 → 31 %, e 5 505 → 35 % |
| 2f24342 | **4** `sent_at` dans la réponse groupée (clé de plus, compatible), horloge de la page prise dessus — contribuable | a, 1ʳᵉ s : 232 → 20 m ; p95 111 → 26 m |
| 25f2de2 | **4** `--profile` : profil CPU de la page | c : voir « liste » |

## Métriques navigateur par scénario : avant (A-before) → après l'étape 2 (A-after) → final (A-final)
| | reculs % | é.-t. vitesse moteur % | é.-t. écran % | err. moy. m | err. p95 m | corr. max m | pose méd. ms |
|---|---|---|---|---|---|---|---|
| a redémarrage | 0,11→0,10→0,02 | 52,9→57,9→12,5 | 56→245→28 | 39→38→28 | 66→138→29 | 4 338→395→113 | 49,7→33,3 |
| b 60, Right now | 0,09→0,13→0,01 | 31,5→12,1→11,5 | 38→32→32 | 1 479→25→25 | 13 993→23→20 | 28 248→122→102 | 49,7→33,3 |
| c 150, Traffic | 4,08→3,67→2,18 | 38,3→27,9→20,7 | 74→73→79 | 1 807→35→27 | 15 726→79→54 | 29 304→209→143 | 124→99 |
| d zoom, glissés | 0,11→0,10→0,01 | 28,5→13,0→11,9 | 39→28→40 | 1 486→26→25 | 14 033→27→21 | 28 343→349→106 | 49,7→33,3 |
| e onglet caché | 0,13→0,14→0,01 | 237→10,8→10,2 | 245→5 505→33 | 1 671→21→14 | 12 424→22→20 | 28 279→114→106 | 49,5→33,3 |
| f trou de 18 s | 0,04→0,10→0,03 | 28,1→12,4→13,2 | 39→34→33 | 1 473→27→28 | 13 919→23→29 | 28 147→147→115 | 49,7→33,3 |
Images (rAF) : 16,7 ms médianes partout sauf c (104 → 30 ms, p95 ~150 ms). Glyphe > 3° : ≤ 0,03 %. Sauts : 1 à 2,7 % (à l'écran, voir doutes). Le f « avant » est faussé (l'avion du trou était l'un des figés). **Rejeu L** : 38,3 → 32,2 m, p95 34,4 → 30,8 m, sauts 0,41 → 0,05 %, é.-t. 29,6 → 17,7 %, corr. max 3 254 → 185 m (10 relocalisations comptées à part) ; `reference.json` réécrite.

## A / B / C (moteur après l'étape 2) — é.-t. vitesse moteur % / erreur moyenne m
| | A (défaut) | B d = 5,6 s (p95) | B d = 2,8 s | C d = 1 s |
|---|---|---|---|---|
| rejeu, croisière | 7,7 / 9 | 3,5 / 1 370 | 7,4 / 744 | 15,1 / 346 |
| rejeu, virage | 14,9 / 16,5 | 5,9 / 911 | 13,8 / 495 | 27,1 / 231 |
| navigateur b | 12,1 / 25 | 9,8 / 1 115 | 11,9 / 580 | 15,5 / 344 |
| navigateur c / d / e / f | 27,9 / 12,9 / 10,8 / 12,4 (21–35 m) | 21,8 / 10,0 / 9,1 / 10,5 (≈ 1 050 m) | 23,3 / 11,3 / 9,3 / 11,9 (≈ 600 m) | 24,5 / 15,3 / 13,5 / 15,4 (≈ 320 m) |
B dessine l'avion là où il était il y a d ; C (1 s, un peu plus que l'âge d'une position à l'arrivée, 0,9 s) alterne interpolation et extrapolation et paie une correction à chaque bascule.

**Recommandation : garder A.** B à 5,6 s ne gagne que 2 à 6 points de régularité (et c'est au banc de rejeu, sans navigateur, qu'il gagne le plus) pour **1,1 km de retard** à 230 m/s ; B à 2,8 s égale A pour 600 m ; C est moins bon des deux côtés. Prix de A : ~12 % d'écart-type de vitesse dans le moteur, 25 m d'erreur moyenne. **Le choix revient au propriétaire.**

**Outils publics** (lus par un agent dans leur code, sans rien copier) :
- **tar1090** (`html/planeObject.js`, `setProjection()`, `script.js`) : saute à chaque position ; gris au-delà de 15 s sans position, retiré à 58 s ; onglet caché : les minuteries s'arrêtent, l'historique est rechargé au retour si l'absence dépasse 5 s. github.com/wiedehopf/tar1090
- **dump1090-fa / SkyAware** (`public_html/planeObject.js`, `updateMarker()`) : saute ; « stale » au-delà de 15 s, retiré à 58 s ; rien pour l'onglet caché. github.com/flightaware/dump1090
- **Virtual Radar Server** (`aircraftPlotter.ts`, `refreshAircraftMarker()`) : saute ; le serveur décide de la position périmée. github.com/vradarserver/vrs
- **FlightAirMap** (`js/map-aircraft.2d.js.php`) : seul à extrapoler (option) : point futur calculé par le serveur, animé jusqu'à la mise à jour suivante. Aucun ne dessine dans le passé en direct.

## Rejouer le banc (sur le Mac, avec le vrai écran)
`node tools/motion-bench/bench.mjs --headed --label mac` (Chrome de macOS par défaut ; `--scenario b`, `--save-reference`, `--engine '{"strategy":"B","renderDelayMs":5600}'`, `--profile 20` ; ports 8790 et 8791 libres ; ~15 min). Rejeu L : `node --test www/aircraft-replay.test.js`, avec `REPLAY_ENGINE='{"strategy":"B"}'`.

## Vérifié, et comment
À chaque commit : `node tools/www-check/check.mjs` (dont `www/*.test.js`, 14 tests du moteur dont 6 nouveaux), rejeu L non dégradé, empreinte Tailwind régénérée par `tools/tailwind/build.sh` ; `go build`, `go vet` et les tests des paquets touchés aux commits Go ; à la fin, `go build`, `go vet ./...` et `go test -race ./...` vert sauf `internal/auth`, qui échoue ici comme sur `main` (session root). Chaque correction mesurée avant/après sur le banc navigateur, parfois avec l’ancien comportement remis par configuration (`resumeAfterMs`, `relocateMinM`).

## Pas vérifié, et ce que le Chrome du cloud ne représente pas
Chromium 141 sans interface, 4 cœurs Xeon, **WebGL logiciel (SwiftShader), pas de GPU, écran 60 Hz simulé** (le Mac est à 120 Hz : la tolérance de cadence y est calculée, pas mesurée). Le rendu des avions est la **couche vectorielle canvas 2D** (`mapAircraftWebGL: false`), pas WebGL. Aucun flux réel, ni station, ni production ; Go 1.24 amd64. Le générateur suppose des décodages au hasard (Poisson) : le p95 de 5,6 s qui règle B n'est pas mesuré sur la station.

## Doutes
- **L'écart-type « à l'écran »** (≈ 30 %) reste bien au-dessus de celui du moteur (≈ 12 %) : une pose calculée pour t est dessinée tantôt à t, tantôt une image après. `map.renderSync()` après chaque image du moteur l'a **aggravé** (31 → 40 %) : essayé, retiré. Cause non trouvée.
- Le trou de 18 s : l'avion **s'arrête encore** ~5 s avant sa reprise (les états prédits du serveur cessent ~13 s après la dernière position, le moteur prolonge 7 s). Le recul d'AIC143 ne se reproduit pas tel quel : sur le banc, l'avion freiné et en virage restait derrière et rattrapait à 3× sa vitesse ; la règle couvre les deux sens.
- Relocalisation : trois lignes dans `map/renderers/aircraft-webgl.js` (clé de cache des styles + `fade`, `textOpacity × fade`, `markerAlpha × fade`), à fusionner avec la session design.
- `sent_at` est un changement du serveur de production ; `visibilitychange` et `currentPose` passent par la carte : à relire.

## Ce qui relève de la liste et de la colonne (noté, pas corrigé)
Scénario c, profil de 20 s : **Alpine 7,3 s, `store/sidebar.js` 3,8 s** (`get counts` 1,7 s, `getSecondsSinceLastSeen` 1,3 s, `activeAircraftCount` 0,25 s), rendu natif 6,4 s, OpenLayers 1,5 s, moteur 0,2 s : 15 tâches longues / 10 s, image médiane 30 ms mais p95 150 ms, 2 % de reculs à l'écran. Avec Right now (b) : 0,2.

## À regarder en premier sur le Mac
1. `bench.mjs --headed` sur la vraie machine, puis la sonde de D79 à l'œil : un avion qui apparaît page ouverte doit bouger tout de suite (f90aec9).
2. Recharger la page en plein trafic : plus de flotte « en retard » les premières secondes (5603e44, 2f24342).
3. Onglet caché une minute puis montré ; un avion qui perd sa position (fondu au lieu d'une glissade).
4. Trancher A / B / C.
