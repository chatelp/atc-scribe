# Fiche L — rapport (session cloud du 13/10)

**Branche : `claude/blissful-babbage-oiwjxm`**, pas `cloud/l-bloc-position` : la session cloud est tenue de pousser sur la branche qui lui est attribuée. Partie de `main` (e72db46), 15 commits, aucun push sur `main`, pas de pull request. Le message de e9df4ef cite `a31c032` : c'est 06abaf2 (message amendé avant le push).

## Commits, dans l'ordre (chiffres du banc : erreur moyenne contre la vraie position)
| Commit | Ce qu'il fait | Chiffres |
|---|---|---|
| 83f68f8 | **Étape 1**, banc de rejeu : générateur Go (12 avions, 0,6 position/s, `seen_pos` qui grimpe, trois pertes de 30 s), la vraie chaîne serveur sur horloge simulée, flux enregistré ; rejeu node de `feed.js` + moteur à 30 i/s. Deux coutures neutres dans `service.go` (`s.now()`, `broadcastLivePredictions`) | +924 −49 ; référence 181 m |
| af30fee, 93e9294 | **Étape 2** : forme de `/tracks` figée, puis un seul prédicteur. `PredictFuturePositions` et son singleton de station retirés ; la prévision du suivi répond aussi depuis un seul relevé (`ForecastFromTarget`) | −168 dans `atc_utils.go`, −50 dans SQLite |
| 06abaf2, e9df4ef | **Étape 3** : formes de `aircraft_added/update/predicted_state` figées clé par clé ; prédiction en direct dans `live_prediction.go` (270 lignes), `service.go` la branche | flux identique à l'octet |
| 423cab8 | La route prédite part de la route du relevé, plus de son cap vrai (reste de D79) — contribuable | +5 −5 |
| e6332b3 | Plafond d'extrapolation 3 → 6 s (un intervalle sur six dépassait 3 s : l'avion s'arrêtait) | 181 → 170 m |
| 19c9d9d | La correction part d'où l'avion est dessiné **maintenant** (image lente à chaque échantillon) | 170 → 161 m, reculs 1,30 → 0,83 % |
| 749bb22 | Les seuils de correction (30 m, 7 m) retirés : l'avion sautait sous ces seuils | reculs 0,83 → 0,03 %, écart-type 60 → 40 % |
| 6a77be0 | **Étape 4** : `aircraft.adsb` = la station seule ; l'estimation va au moteur (`estimate()`) | 159 → 150 m |
| f82a208 | **Étape 5** : interface écrite (`observe`, `estimate`), échantillons datés par leur source, décalage d'horloge serveur/page estimé, réglages documentés, deux morts retirés | 150 → 108 m |
| a403aeb | En virage, l'état prédit suit le virage (route du relevé, pas la moyenne sur 10 s ; l'arc, pas la corde) — contribuable | reculs → 0 % |
| 6b780fd | `seen_pos` compté depuis le `now` d'`aircraft.json` (tar1090, readsb), pas depuis la lecture du serveur — contribuable | **108 → 38 m** |
| 9541098 | **Étape 6** : tests Go (expiration, plancher, message valeur par valeur) et node (datation, horloge, filtres) ; rejeu comparé sans tolérance aux mesures de l'étape 1 | |

## Le rejeu, avant (étape 1) et après (étape 6)
| | pas à reculons | images lentes (< 30 % méd.) | sauts (> 2×) | écart-type vitesse / vraie | plus grande correction | erreur moyenne | p95 |
|---|---|---|---|---|---|---|---|
| avant | 1,28 % | 1,99 % | 2,03 % | 63,4 % | 3 758 m | 181,4 m | 236,4 m |
| après | **0 %** | 1,36 % | 0,41 % | 29,6 % | 3 254 m | **38,3 m** | 34,4 m |

Par type, après : croisière 9 m, montée 8 m, virage 16,5 m, perte de position 122 m (l'avion s'arrête 7 s après sa dernière position, c'est voulu) ; écart-type de la vitesse 7,7 % en croisière (40 % avant ; à 231 m/s, la vitesse dessinée oscillait chaque seconde entre 90 et 347 m/s).

## Qui lit l'observé, qui lit l'estimé (étape 4)
**L'observé, tous** : distance (`feed.js`) et tri de la liste (`sidebar.js`), bande et badges LP/S (`index.html`), fiche de vol et centrage (`aircraft-details.js`, `transcripts.js`), traînée en direct, création et visibilité du symbole (`openlayers-map-manager.js`), propriétés WebGL, minicarte, trace et arrivées de la PAR. **L'estimé, un seul** : la pose du symbole (le moteur, qui mêle les deux). Aucun fichier hors `feed.js` et du moteur n'a été modifié. Changement visible attendu : la distance, la traînée et la trace PAR ne bougent plus qu'aux relevés réels.

## Réglages du moteur (unité, raison : commentés dans le code)
`enabled`, `interpolationFps` (i/s), `viewportCulling` : panneau, enregistrés. `maxPredictionSeconds` 7 s depuis l'heure de la source (≈ 6 s après réception). `predictionGain` 1 (D79). `mergeDurationMs` 450 ms (correction d'un avion à l'arrêt), `min/maxCorrectionDurationMs` 180 / 2 500 ms. `headingSmoothingAlpha` 0,22 par échantillon. `adaptivePerformance`, `maxFrameBudgetMs` 14 ms : `qualityLevel` ne limite plus que le nombre d'avions par image (240 × q, au moins 30). `cleanupIntervalMs`, `maxStates`. `minEstimateConfidence` 0,65, `minEstimateIntervalMs` 1 000 (repris de `feed.js`). `clockWindowSec` 30 (décalage d'horloge). **Retirés** : `predictedJitterDeadbandNm`, `predictedHeadingDeadbandDeg`, `realCorrectionThresholdNm`, `predictedCorrectionThresholdNm`. Mesurés et gardés : lissage de vitesse 0,35, vitesse tirée des seuls relevés. Essayé et abandonné : extrapolation en arc côté page (aucun gain).

## Vérifié, et comment
À chaque commit : `go build`, `go vet`, `go test -race` (adsb, api, cmd ; `./...` aux étapes serveur), `node tools/www-check/check.mjs` (exécute `www/*.test.js`), routes figées inchangées, formes des messages figées tenues, métriques du rejeu qui ne reculent pas (tolérance d'arrondi : deux reculs de 0,01 et 0,04 point, écrits dans les commits concernés). `internal/auth` échoue ici comme sur `main` : la session tourne en root et un dossier en lecture seule reste inscriptible pour elle.

## Pas vérifié
Rien dans un navigateur, rien sur la station ni en production, aucun flux réel. Go 1.24 sur amd64, pas l'arm64 du Mac (la référence se compare à ±0,05 point près, le flux n'est pas comparé à l'octet). Hypothèses du générateur **non mesurées** : décodages de position aléatoires (Poisson), fichier lu 0,5 s après écriture, bruit de 5 m, horloges synchrones.

## Doutes
- **6b780fd suppose la station et le Mac à l'heure (NTP)**, comme le MLAT. Une horloge de station en avance est ramenée à celle du serveur ; en retard, elle vieillirait les positions d'autant.
- Sans états prédits (phases de vol coupées), le décalage d'horloge de la page reste biaisé de l'âge du fichier : l'avion est dessiné court d'autant, comme avant.
- Un virage à plus de 3°/s ne reçoit aucun état prédit (plancher 0,65) : je n'ai pas touché au plancher.
- La fiche cite `GET /api/v1/aircraft/{hex}/future` : cette route n'existe pas, c'est `/tracks` qui porte `future` et `hindcast`.
- Chaque `aircraft_update` transporte l'avion entier, `future` compris : environ 5,8 Ko par avion et par seconde. Hors sujet ici ; à mesurer sur le réseau.
- `aircraft-animation.js` fait 898 lignes, dont beaucoup de commentaires : le module de 300 lignes visé n'est pas atteint côté page. `live_prediction.go` (270) l'est côté serveur.

## Pas osé toucher
`index.html`, `sidebar.js`, `station.js`, `map/renderers/`, les CSS (migration) ; le nom interne `lastReal` (les tests D79 le lisent) ; le pas de 5 s et les 12 points de la prévision (forme de `/tracks`) ; l'altitude prévue, ancrée sur la moyenne de 30 s ; le plancher de confiance. `www/vendor/tailwind.css` ne change que par sa ligne d'empreinte (`tools/tailwind/build.sh`) : **à régénérer après la fusion avec `migration`**, pas à fusionner à la main.

## À regarder en premier sur le Mac
1. L'instance d'essai dans un onglet visible, 80 à 110 avions : la sonde de D79 (reculs, images lentes, sauts) avant et après, et l'œil sur un virage serré.
2. La distance de la liste et la trace PAR, qui ne se mettent plus à jour qu'aux relevés réels.
3. Sur le téléphone par Tailscale (horloge différente) : les avions ne doivent ni se figer ni prendre d'avance.
4. 6b780fd et a403aeb : relire avant tout, ce sont deux changements du serveur de production.
