# Fiche L — Le bloc position, trajectoire, prédiction : un seul modèle, l'observé séparé du déduit

*Vague 3, ajoutée le 13/10 au soir, à la demande du propriétaire après la journée D79.
Branche : `cloud/l-bloc-position`. Lis d'abord `README.md` de ce dossier : ses règles
s'appliquent. Lis ensuite la décision **D79** dans `docs-fr/05-decisions.md` : elle raconte
les quatre défauts trouvés et corrigés ce jour-là, et c'est le cahier des charges de cette
fiche.*

## Le constat, vérifié le 13/10

Le mouvement des avions sur la carte est produit par neuf fichiers, 8 379 lignes, répartis
entre le serveur et la page :

| Côté | Fichier | Lignes | Rôle |
|---|---|---|---|
| serveur | `internal/adsb/service.go` | 1 927 | sondage de la station, détection des changements, **boucle de diffusion des états prédits** (`predictionBroadcastLoop`), diffusion WebSocket, phases, et le reste |
| serveur | `internal/adsb/trajectory.go` | 1 028 | mémoire des positions (anneau), état dérivé, **prédiction en direct** (`GetLivePredictionsAt`, `interpolateLivePredictionFromForecast`) |
| serveur | `internal/adsb/trajectory_prediction.go` | 328 | prévision et rétro-prévision (`computeForecast`, `computeHindcast`) |
| serveur | `internal/adsb/trajectory_phase.go` | 558 | règles de phase, qui lisent l'état dérivé |
| serveur | `internal/adsb/atc_utils.go` | 822 | **un second prédicteur**, `PredictFuturePositions`, encore appelé en repli (`service.go` vers la ligne 1912, `a.Future`) |
| serveur | `internal/adsb/change_detector.go` | 228 | le delta envoyé à la page |
| page | `www/store/feed.js` | 763 | réception WebSocket : relevés réels, deltas, **états prédits écrits dans `aircraft.adsb.lat/lon`** avec `adsb.source = 'predicted'` et des champs privés `_lastRealObservedAt`, `_lastPredictedAppliedAt` |
| page | `www/aircraft-animation.js` | 772 | le moteur : échantillons, vitesse, prédiction, corrections, poses |
| page | `www/map/openlayers-map-manager.js` | 1 953 | applique les poses aux entités (`updateAnimatedAircraftPose`), parmi tout le reste de la carte |

Ce que la journée du 13/10 a montré (D79) : quatre défauts de conception de l'amont, chacun
trouvé en lisant le flux à la main parce que **rien ne distingue, dans le store de la page, ce
que la station a vu de ce que le logiciel en déduit** (`adsb.lat` porte tantôt un relevé,
tantôt une prédiction ; quatorze endroits de `www/` lisent `source === 'predicted'`,
`_lastPredicted*` ou `stale_position` pour s'y retrouver) ; le moteur date chaque échantillon
de `performance.now()` à sa réception, pas de l'heure de la position ; il recalcule une
vitesse que le serveur a déjà ; et deux prédicteurs coexistent côté serveur. Les tests qui
couvrent ce bloc tiennent en deux fichiers écrits ce jour-là (`internal/adsb/live_prediction_test.go`,
`www/aircraft-animation.test.js`, quatre cas) : rien ne mesure le mouvement rendu de bout en bout.

**But** : que le prochain défaut de trajectoire ou de prédiction se trouve en lisant un module
de 300 lignes et un test, pas en sondant une page en direct pendant deux heures. **Contrainte** :
le mouvement rendu ne doit pas se dégrader d'un pas — c'est la règle du chantier (« ne rien
casser », sept garde-fous, doc 32, garde-fou 4 sur les compteurs d'animation), et c'est
mesurable, donc tu le mesures.

## Étapes, dans cet ordre, un commit chacune

1. **Le banc de rejeu, avant toute modification.** Un générateur de sondages ADS-B
   synthétiques (Go, dans `internal/adsb`, test seulement) : des avions à vitesse constante,
   en virage, en montée, en décélération, chacun sondé chaque seconde avec **une position
   neuve une fois sur deux en moyenne** (`seen_pos` qui grimpe de +1 s sur les relevés figés,
   c'est le régime mesuré de la station : 0,6 position par seconde et par avion) ; quelques
   avions qui perdent leur position trente secondes puis la retrouvent. Ce générateur alimente
   la chaîne du serveur telle qu'elle est (détection des changements, suivi de trajectoire,
   boucle de prédiction) et **enregistre le flux de messages WebSocket** qu'elle produit
   (`aircraft_added`, `aircraft_update`, `aircraft_predicted_state`), en JSON, dans
   `testdata/`. Côté page, un test node rejoue ce flux dans `feed.js` et le moteur avec une
   horloge simulée à 30 images par seconde et un faux `mapManager` qui note chaque pose. Les
   **métriques** du rejeu, imprimées et gardées dans le rapport comme référence : part des pas
   à reculons, part des images presque arrêtées (< 30 % de la vitesse médiane), part des sauts
   (> 2×), écart-type de la vitesse rendue, plus grande correction en mètres, et l'erreur
   moyenne entre la pose rendue et la **vraie** position du générateur (lui seul la connaît :
   c'est la métrique qui manque à tout ce qui précède). **Aucune capture réelle** : ni
   position de la station, ni flux enregistré sur elle ; le générateur suffit, et il
   contient les cas que le réel ne donne pas sur commande.
2. **Un seul prédicteur côté serveur.** `PredictFuturePositions` (`atc_utils.go`) et son repli
   dans `service.go` disparaissent au profit de la prévision du suivi de trajectoire, ou tu
   écris dans le rapport pourquoi il faut les deux (avec un cas où la prévision ne sait pas
   répondre). `GET /api/v1/aircraft/{hex}/future` et les champs `future`/`hindcast` des
   réponses gardent leur forme : un test la fige avant que tu touches à quoi que ce soit.
3. **La prédiction en direct sort de `service.go`.** Un type à part (`internal/adsb/live_prediction.go`
   ou un paquet `internal/adsb/predict`, à toi de voir ce qui reste lisible) porte la boucle,
   l'intervalle, le plancher de confiance, l'ancrage sur l'heure de la position (D79) et la
   forme du message émis ; `service.go` ne fait que le brancher. Le message `aircraft_predicted_state`
   garde sa forme exacte (clé par clé, un test la fige : la page d'aujourd'hui doit continuer
   de marcher avec un serveur d'hier et inversement).
4. **La page sépare l'observé du déduit.** `aircraft.adsb` redevient **ce que la station a
   dit** et rien d'autre : plus aucune prédiction n'y écrit, `adsb.source` et les champs
   `_lastPredicted*` disparaissent. Ce que le logiciel déduit vit à côté, sous un nom clair
   (`aircraft.estimate` : `lat`, `lon`, `alt`, `heading`, `at`, `based_on`, `confidence`, ou
   le nom que tu justifies), ou va **directement au moteur** sans passer par le store si rien
   d'autre n'en a besoin. Avant de toucher, **dresse la carte de qui lit quoi** : les quatorze
   endroits (`git grep -n "source === 'predicted'\|_lastPredicted\|stale_position" www/`),
   la distance dans la liste, la fiche de vol, la PAR, le rendu WebGL. Chacun choisit
   explicitement l'observé ou l'estimé, et le rapport le dit. `stale_position` (position
   gardée quand la station la perd) reste : c'est de l'observé daté, pas du déduit.
5. **Le moteur a une interface écrite.** `aircraft-animation.js` reçoit des échantillons
   **datés par leur source** (l'heure de la position pour un relevé, `predicted_at` pour une
   estimation) et non plus par `performance.now()` à la réception ; il expose deux entrées
   (`observe(hex, sample)`, `estimate(hex, sample)`) ou une seule avec un champ, à toi de
   voir, et une sortie (la pose par image, contrat inchangé avec `updateAnimatedAircraftPose`).
   Chaque réglage de `config` est documenté en une ligne avec son unité et sa raison ; ceux
   que rien ne lit plus disparaissent (vérifie `predictedJitterDeadbandNm`,
   `predictedHeadingDeadbandDeg`, `mergeDurationMs`, `maxFrameBudgetMs`, la portée exacte de
   `qualityLevel`). Les quatre corrections de D79 (point répété, écart qui s'efface, gain 1,
   échantillon prédit prolongé) sont conservées telles quelles et leurs quatre tests passent.
6. **Les tests tiennent le bloc.** Go : le prédicteur (ancrage, expiration à 60 s de la
   position, plancher de confiance, forme du message) ; node : le moteur (les quatre cas de
   D79 plus le rejeu de l'étape 1) ; et le rejeu complet comme test, avec ses métriques
   comparées à la référence de l'étape 1 : **égales ou meilleures, sinon le commit ne passe
   pas**. L'erreur moyenne contre la vraie position doit baisser ou rester ; dis de combien.

## Vérifications à chaque commit

`go build ./...`, `go vet ./...`, `go test -race ./...` ; `node tools/www-check/check.mjs`
(il exécute `www/*.test.js`) ; la liste figée des routes (`internal/api/testdata/routes.txt`)
ne change pas ; les formes de `aircraft_update` et `aircraft_predicted_state` figées à
l'étape 3 ne changent pas ; les métriques du rejeu ne reculent pas. **Ne touche pas** à
`www/index.html`, `www/store/sidebar.js`, `www/store/station.js`, `www/map/renderers/`,
`www/tokens.css`, `www/components.css` : la migration du design y travaille en parallèle sur la
branche `migration` ; si l'étape 4 t'oblige à y changer une lecture (la distance d'une bande,
la fiche de vol), fais-le minimal, sur une ligne, et signale-la dans le rapport pour que la
fusion se fasse en connaissance de cause.

## Rapport

`docs-fr/fiches-cloud/L-rapport.md` (voir README). En plus : le tableau des métriques du rejeu
**avant** (étape 1) et **après** (étape 6), la carte « qui lit l'observé, qui lit l'estimé »
de l'étape 4, la liste des réglages du moteur avec leur unité et leur raison, et ce que tu
n'as pas osé toucher, avec pourquoi.
