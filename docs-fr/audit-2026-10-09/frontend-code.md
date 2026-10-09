# Audit du code de l'interface (`www/`), 09/10/2026

Mesures par analyse statique : parseur acorn de Node 25.5, `git diff upstream/main...HEAD`, grep. Le serveur local
était arrêté (`curl localhost:8000` : 000) : **rien n'a été mesuré dans un navigateur** (temps de chargement, rendu,
CSS réellement utilisé). Scripts d'analyse dans le scratchpad, hors dépôt. Lignes « ≈ » : estimées par plages.

## 1. Carte du code

| fichier | lignes | fonctions | console.log | origine |
|---|---:|---:|---:|---|
| app.js | 6 040 | 395 (store : 226) | 72 | amont, +534/−6 |
| index.html | 2 656 | 6 `x-data` | 0 | amont, +407/−19 |
| map/ (11 fichiers) | 4 304 | 326 | 0 | amont, +16/−4 |
| atc-chat.js | 1 106 | 56 | 47 | amont, intact |
| par/par-view.js, par-geometry.js (+ test 188) | 745, 298 | 89, 18 | 0 | **à nous** |
| aircraft-animation.js | 695 | 33 | 0 | amont, intact |
| audio-client.js | 542 | 32 | 2 | amont, +50/−22 |
| websocket-client.js | 448 | 29 | 5 | amont, +3 |
| login.js | 219 | 10 | 0 | **à nous** |
| style.css / sw.js | 935 / 99 | — / 10 | 0 | amont, +68/−1 / intact |

Total 18 275 lignes (JS+HTML+CSS) ; 890 Ko chargés à chaque ouverture, servis sans compression et en `no-store`
(`internal/api/static.go:107`) ; mesuré : gzip -6 ramènerait app.js de 278 à 56 Ko, index.html de 236 à 29 Ko.
Le « 13 036 lignes » de docs-fr/32 = tout sauf `map/` et `style.css`.

**Store Alpine** (`Alpine.store('atc', {…})`, app.js:162–5747 = 5 586 lignes, 92 % du fichier) :
- 360 propriétés : 226 fonctions (dont 8 accesseurs `get`, 18 `async`), **134 champs d'état** de premier niveau (dont
  `settings`, ≈ 39 clés, l.378–455) et 3 champs jamais déclarés (`collapsibleSections`, `_staleCleanupCounter`,
  `_trailUpdatePending`). Médiane 12 lignes par fonction, 48 fonctions de 30 lignes et plus, 4 888 lignes de fonctions.
- Surface publique : index.html lit 169 noms via `$store.atc` (714 occurrences, **0 non résolu**) ; map-manager 31
  accès, par-view 16, audio-client 14 ; 191 propriétés sur 360 n'apparaissent jamais dans index.html.
- 44 clés localStorage (42 `getItem` + 42 `setItem`), 18 `fetch`, 9 `setInterval`, 33 accès directs au DOM.

**Dépendances** (index.html:9–54), toutes en CDN, aucune copie locale :

| ressource | version | taille | usage |
|---|---|---|---|
| Tailwind play CDN | non épinglé (302 → 3.4.17) | non mesurée | oui : 987 attributs `class` (≈ 52 Ko), compilés dans le navigateur |
| Alpine, unpkg `3.x.x` | non épinglé (302 → 3.17.4) | 55,9 Ko* | oui |
| OpenLayers `ol.js` + `ol.css` | 10.6.1 | 234,8 Ko* + 1,5 Ko* | oui |
| Font Awesome `all.min.css` | 6.5.1 | non mesurée | 41 icônes distinctes utilisées (80 occurrences) |
| tuiles : cartocdn, arcgis (FAA), mesonet, NOAA, adsbexchange | | | seul cartocdn sert ici |

*`Content-Range` d'une requête d'un octet, non vérifié par téléchargement complet (peut être la taille compressée).
Sans Internet la page ne se charge pas (Tailwind, Alpine, OL), ce qui contredit « station autonome » (00-mission).
`console.log` : 126 (122 actifs ; plus 19 `warn` et 32 `error` dans app.js), dont 21 actifs dans les gestionnaires du flux
d'avions (app.js:3181–4299), sans drapeau de débogage.

## 2. À nous contre amont

`git diff --stat upstream/main...HEAD -- www/` : 11 fichiers, **+2 528 / −52**. Fichiers nouveaux : par/par-view.js 745,
par-geometry.js 298, par-geometry.test.js 188, login.js 219 (1 450 lignes). Modifiés : index.html +407/−19 (15,3 % du
fichier), app.js +534/−6 (8,8 %), style.css +68/−1 (7,3 %), audio-client.js +50/−22, visibility-rules.js +9/−2,
openlayers-map-manager.js +7/−2, websocket-client.js +3. Intacts : atc-chat.js, aircraft-animation.js, sw.js, 9 fichiers de `map/`.
Dans app.js, **228 de nos 534 lignes sont `serverSettings()`** (l.5813–6040, déjà autonome) ; le reste est dispersé sur ~20
fonctions (loadAircraftRadio 38, handleTranscriptionUpdateMessage 27, transcriptHtml 25, refreshAudioFrequencies 24…).
**D19 est périmé** (05-decisions.md:869) : « 1,7 % de app.js » valait 93 lignes au 16/09 (commit fdd8f2f), c'est 534
aujourd'hui (9,7 % de la taille amont, ×5,7 en 23 jours) ; index.html passe de 3,1 % à 17,9 %. La règle « modifier
l'amont quand c'est le plus simple » tient ; l'argument « empreinte faible » ne tient plus pour l'interface.

## 3. Code mort et fonctionnalités amont

**21 fonctions du store jamais appelées** (ni app.js, ni index.html, ni les autres JS ; commentaires exclus ; fermeture
transitive) : **346 lignes**. createLabelContent app.js:893 (109), processAircraftData 2561 (53), aircraftPassesFilters 4249
(49), getStatusColor 628 (17), formatLastSeenAgo 1623 (16), onMapClickForSimulation 1545 (14), showPhaseHistory 2150 (13),
toggleSort 1323 (12), toggleStatusFilter 1337, getAnimationStats 3070 (8), getNOTAMCount 5201, getTAFCount 5209 (6),
clearAllPendingRequests 216, closePhaseHistory 2254, toggleDateFormat 2622 (5), isFrequencyConnected 5403,
getFrequencyStatus 5409 (4), getGroundedAircraftCount 1601, processSampleData 2652, getTAF 5191, getNOTAMs 5196 (3).
Champs jamais lus : wsConnection:249, showLostAircraftOnly:254, _filteringScheduled:3820. Seul accès dynamique au store :
`this[id]` (app.js:2536, 9 noms d'intervalles). Code commenté : 30 lignes en tout, négligeable. Bug latent : `cleanup()`
lit `window.animationEngine` (app.js:2543), jamais assigné (`let` de script).

| bloc de l'amont | emplacement | lignes | état chez nous |
|---|---|---:|---|
| Chat IA, OpenAI realtime | atc-chat.js ; index.html:2256–2371 ; style.css:282 ; `<script>` index.html:2655 | ≈ 1 225 | `[atc_chat] enabled = true`, clé vide : le bloc s'affiche, inutilisable. 0 référence depuis app.js |
| Simulation d'avions | app.js:465–474, 1416–1558 ; index.html:493–541, 1593–1657, 2552–2649 ; map-manager:71, 1671–1672 | ≈ 360 | bouton toujours visible ; Toronto (CYYZ) en dur app.js:467, 1525 ; routes Go sans condition (routes.go:140–143) |
| Couches américaines (VFR/IFR FAA, NEXRAD, NOAA) | index.html:859–866, 933–968 ; app.js:415–430, 513–519, 2732–2772 ; map-engine.js:62–105 ; map-manager:170–230 | ≈ 210 | mesuré : tuile z8 sur la station = 404 pour les 3 sources FAA (200, 51 Ko sur New York). **Fond par défaut `vfr-sectional` (app.js:380)** : un navigateur neuf affiche une carte vide |
| Position de la station par géolocalisation | app.js:2848–3068 (11 fonctions) ; index.html:381–492 | ≈ 330 | station fixe, réglée côté serveur ; à trancher |
| Diagnostic de performance | index.html:268–380 ; app.js:64–148, 334–366, 3080–3173 ; map/perf/telemetry.js | ≈ 410 | utile (audit produit, point 2) ; à isoler plutôt qu'à retirer |
| Météo METAR/TAF/NOTAM | app.js:279–283, 5039–5214 ; index.html:1109–1120, 2373–2473 | ≈ 300 | **active** (Windy, cloud, « gardée » Q36) : pas du code mort |
| CSS | style.css | 48 classes sur 105 | sans occurrence littérale dans HTML/JS (hors OL et noms construits), sans doute les libellés DOM d'avant le rendu WebGL ; non vérifié |

Recoupements : onMapClickForSimulation (simulation) et getTAF, getNOTAMs… (météo) figurent aussi parmi les 21 mortes.

## 4. Grosses fonctions et duplications

Plus de 100 lignes : app.js `serverSettings` 228 (l.5813, à nous, fabrique de composant) ; `formatAircraftDetails` 183
(1138, construit du HTML en chaîne) ; `init` 163 (2354, câble tout, appelé depuis `alpine:init` l.5765) ; `initWebSocket`
141 (3193) ; `createLabelContent` 109 (893, **morte**) ; `getATCDerivedMetrics` 104 (1032) ; `handleAircraftUpdate` 102 (3866,
chemin chaud). Juste sous 100 : handleTranscriptionUpdateMessage 99, fetchStationData 98, updateMapPerformanceStats 94.
Hors app.js : par-view.js `frameSvg` 213 (447–659) et `draw` 121, à nous ; openlayers-map-manager.js `updateFlightPaths`
187 (1316–1502) ; aircraft-webgl.js `getVectorStyle` 149 ; websocket-client.js `connect` 127 ; atc-chat.js 116 et 115.

Duplications :
- Copies exactes de 6 lignes et plus : 145 lignes seulement. **En ignorant les noms : 11 groupes, 201 lignes redondantes**
  dans le store : 9 `set*Opacity` identiques (2753–2815), 8 `toggle*` de couches (2697–2751), 3 × 29 lignes
  METAR/TAF/NOTAM (toggle*Details, 5098–5189), 3 `stop*Refresh`, 3 prédicats d'état de fréquence.
- Quasi-jumeaux non comptés : cycleToNext/PreviousAircraft (1658–1719, seul le sens change) ; les 3 constructeurs
  d'alertes (4392, 4499, 4575 : 57 + 74 + 72 lignes, même squelette). **Pas avant la décision UX** sur les alertes.
- Utilitaires : haversine dans app.js:4234 et aircraft-animation.js:646 ; degrés → radians réécrit dans cinq fichiers.

## 5. Plan de découpage de app.js

**Principe.** Scripts classiques comme `map/` (IIFE + `window.X`), sans bundler ni `type="module"` : Alpine est en `defer`
dans le `<head>`, l'ordre d'écoute d'`alpine:init` serait un risque avec des modules (non essayé). Chaque domaine va dans
`www/store/<domaine>.js` et s'enregistre dans `window.AtcStore` ; app.js assemble `Alpine.store('atc', …)`. Les 8 accesseurs
`get` imposent `Object.defineProperties(cible, Object.getOwnPropertyDescriptors(partie))`, pas `Object.assign` (qui figerait
leur valeur). Aucun nom ne change : index.html reste intact. Les fichiers sont lus sur disque : chaque étape se déploie par
copie, sans compilation, et se défait par `git revert`. **Ne jamais renommer les 44 clés localStorage** (préférences du
propriétaire) ; `serverSettings` reste une fonction globale.

| # | étape | lignes retirées ou déplacées | effort |
|---|---|---:|---:|
| 0 | Filet : script `node` qui vérifie que chaque `$store.atc.X` d'index.html et chaque `store.X` des autres JS existe, `node --check` de chaque fichier, `node --test www/par`, plus 10 gestes à faire au navigateur (écoute, sélection, PAR, panneau Serveur) | 0 | 2 h |
| 1 | **Retirer le chat IA** (fichier, bloc HTML, `<script>`, règle CSS) ; le propriétaire passe `[atc_chat] enabled = false` | −1 225 | 1,5 h |
| 2 | Retirer la simulation (store, 3 blocs HTML, 3 lignes de carte) | −360 | 2 h |
| 3 | Retirer les 20 autres fonctions mortes, 3 champs, le code commenté | −330 | 1,5 h |
| 4 | Retirer les couches américaines ; fond par défaut `dark` (décision du propriétaire) | −210 | 2 h |
| 5 | Fusionner les clones : `setLayerVisible(nom)`, `setLayerOpacity(nom)`, `togglePopup(type)`, `cycleAircraft(sens)` | −230 | 2 h |
| 6 | Déplacer sans modifier : `serverSettings()` → `server-settings.js` ; tuiles et service worker (app.js:64–148) → `tile-cache.js` | −310 | 1,5 h |
| 7 | Introduire `AtcStore` ; extraire audio et fréquences (≈ 300 lignes, 21 fonctions) → `store/audio.js` | −300 | 3 h |
| 8 | Station, source ADS-B, référence, météo (≈ 490) → `store/station.js` | −490 | 3 h |
| 9 | Panneau de détails d'un avion (≈ 1 130) → `store/aircraft-details.js` ; `formatAircraftDetails` devient un gabarit Alpine | −1 130 | 5 h |
| 10 | Flux d'avions et WebSocket (≈ 1 050) → `store/feed.js`, `ws-handlers.js` ; couper `init` et `initWebSocket`. **Chemin chaud** : relever les compteurs d'animation avant/après, avec 80 à 110 avions | −1 050 | 6 h |
| 11 | Liste latérale, filtres, alertes, réglages (≈ 1 300) | −1 300 | 6 h |
| 12 | Option : héberger Alpine (épinglé), OL, Tailwind compilé en CSS statique, les 41 icônes en SVG | | 5 h |

Effort : étapes 0–6 = 12,5 h ; 0–11 = 35,5 h ; avec l'option 12 = 40,5 h. **L'étape 11 se fait avec la refonte UX**
(réglages en page à part, alertes) : on ne découpe pas ce qui va être réécrit. app.js estimé : 6 040 → ≈ 5 000 après
l'étape 6 → ≈ 2 000 après la 10 → moins de 1 000 après la 11. **Supprimer d'abord** (étapes 1–5, ≈ 2 355 lignes, 13,6 %
du JS+HTML, sans déplacer de code donc sans risque sur l'ordre d'initialisation), puis déplacer du moins couplé au plus
couplé : audio, station, détails, flux.

## Résumé

1. `app.js` fait 6 040 lignes, dont 5 586 dans un seul store de 360 propriétés (226 fonctions, 134 champs) lié à
   index.html par 169 noms ; le fork y a ajouté 534 lignes (8,8 %, dont 228 dans `serverSettings`), contre 93 au 16/09.
2. Cinq ressources CDN dans le `<head>`, dont Alpine et Tailwind non épinglés : sans Internet la page ne charge pas.
3. Retirables sans toucher à la station : chat IA (≈ 1 225 lignes), simulation (≈ 360), fonctions mortes (≈ 330),
   couches américaines (≈ 210), clones (≈ 230) : ≈ 2 355 lignes, soit 13,6 % du JS+HTML.
4. À corriger : le fond de carte par défaut `vfr-sectional` renvoie 404 sur la station (mesuré), carte vide sur un navigateur neuf.
5. Plan en 13 étapes (0 à 12), interface fonctionnelle à chaque étape : 12,5 h pour supprimer et isoler, 35,5 h pour tout découper.
