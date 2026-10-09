# Fiche B — rapport (session cloud du 10/10)

Branche `cloud/b-retraits-interface`, partie de `main` (`b561478`) ; la fiche a grandi pendant la
session (étapes 6 à 8, défaut `osm` à l'étape 4) : `main` (`86659f0`) y est fusionné, sans conflit.
`www/` contre `main` : **−2 613 / +403**. `app.js` 6 040 → 5 301, `index.html` 2 675 → 2 314,
`style.css` 935 → 898, `atc-chat.js` supprimé, `sw.js` 99 → 32. Aucun fichier Go touché.

| Commit | Étape | Lignes | Contenu |
|---|---|---:|---|
| `6408d91`, `d2f1617` | 0 | +298 | Le filet `tools/www-check/check.mjs`, puis l'équilibre `/* */` et `{}` des CSS |
| `24f8e42` | 1 | −1 232 | Chat « AI Advisory » : fichier, tuile, `<script>`, CSS |
| `e34b4ca` | 2 | −420 | Simulation : store, 3 blocs HTML, carte, CSS `.slider-blue` |
| `51fb776` | 3 | −391 / +3 | 21 fonctions et 3 champs morts ; `cleanup()` corrigé (`this.animationEngine`) |
| `c045868`, `b089fe9` | 4 | −261 / +23 | FAA, NEXRAD, NOAA ; défaut sombre (1ʳᵉ fiche) puis `osm` (fiche révisée) |
| `d6ee78d` | 5 | −210 / +54 | Clones : `applyLayerVisibility`, `applyLayerOpacity`, `toggleWeatherDetails`, `cycleAircraft` |
| `ee27809` | 6 | −119 / +112 | `ign`, `ign-dark`, `osm`, `osm-dark` ; défaut selon la station ; Carto retiré ; `sw.js` |
| `199120b` | 7 | +462 | `tools/airport-layout/fetch.py` (189 l., le message dit 179) et 11 tests ; couche « Airport layout » |
| `12732b1` | 8 | +109 | `www/par/par-vac.js` et 5 tests ; lien « VAC » dans l'en-tête de la PAR |

**Le filet** : `node tools/www-check/check.mjs` depuis la racine (Node ≥ 22, rien à installer ;
`--quick` saute `node --check` et les tests PAR). Il capture le vrai store en exécutant `app.js`
dans un bac à sable `vm`, et échoue si un `$store.atc.X`, un `store.X` d'un autre JS ou un `this.X`
d'une méthode n'existe pas, si un appel nu d'un attribut Alpine n'est défini nulle part, si un
`<script src>` manque, si un CSS est déséquilibré, si `node --check` ou les tests PAR échouent.
`main` : 169 noms lus par `index.html`, 0 non résolu ; fin : 156, 0 non résolu.

## Vérifié, et comment

- **Chaque commit** : le filet, puis `index.html` dans Chromium sans tête, **0 erreur JavaScript**.
  CDN et hôtes de tuiles sont refusés ici : Tailwind 3.4.17 **compilé localement**, Font Awesome,
  OpenLayers 10.6.1 et Alpine 3.15.0 pris sur npm, **tuiles synthétiques**. Le filet sait échouer
  (14 problèmes sur cassures volontaires ; mon `/*` emporté à l'étape 2, vu par le contrôle CSS).
- **3** : propriétés comptées par nom dans `www/` jusqu'au point fixe. **5** : test différentiel
  ancien/nouvel `app.js`, 47 cas identiques, et qui échoue sur code cassé exprès.
- **4 et 6** : station à Fontenay → `ign-dark` ; à JFK ou absente → `osm-dark` ; préférence `dark`,
  `vfr-sectional` → défaut de la station ; `ign`, `osm` gardés. Un défaut n'est **pas enregistré**
  (il suit la station) ; un clic l'est, clé `mapStyle` inchangée. Le filtre est sur le seul canvas
  de fond (`.atc-basemap`), pas sur les autres couches, carte et mini-carte. `sw.js` : ancien worker
  et cache `co-atc-tiles-v2` rempli, puis mise à jour → plus aucun cache.
- **7** : sans fichier, couche vide, pas d'erreur ; fichier d'essai de 5 objets (non commité) :
  45 m à z15 = 14,3 px, désignations dès z13, taxiways dès z12, rien sous z9 sauf pistes ; dessinée
  sans filtre ; l'interrupteur la masque et enregistre `showAirportLayout`. **8** : lien, `target`,
  `rel`, aide ; un clic garde l'avion sélectionné. Cycles AIRAC recoupés par un calcul indépendant.
- **Go** : `build`, `vet` verts ; `test -race` vert sauf `auth/TestAChoiceThatCannotBeWritten…`, rouge
  aussi sur `main` : la session tourne en root, que `chmod 0500` n'arrête pas.

**Pas vérifié** : la politique réseau refuse `data.geopf.fr`, `tile.openstreetmap.org`, les deux
Overpass, le SIA et les CDN (403 du proxy). Donc **ni vraie tuile, ni capture IGN/OSM sur CDG, ni
taille du GeoJSON, ni temps d'Overpass**, ni le `maxZoom: 19` d'IGN ; ni station, ni son, ni téléphone.

## Doutes

1. Clairances cassées **sur `main`** : `this.addAlert` et `this.selectAircraft` n'existent pas ; un
   `clearance_issued` lèverait une `TypeError`. Laissé aux alertes (2.2), « connu » dans le filet.
2. Retirés car listés à l'étape 3, jamais appelés : `getTAF`, `getNOTAMs`, `getNOTAMCount`,
   `getTAFCount`, `toggleDateFormat`, `toggleStatusFilter`, et `highlightSearchTerm` (mort sur
   `main`, cru vivant par l'audit à cause d'un homonyme du chat).
3. **Attributions invisibles** : le contrôle d'attribution d'OpenLayers est désactivé depuis
   l'amont ; OSM et IGN demandent qu'elles se voient. Posées sur les sources, pas affichées.
4. Au premier chargement sans préférence, `osm-dark` s'affiche puis bascule sur `ign-dark` quand
   `/api/v1/station` répond. Repli de `normalizeBaseMapStyle` sans position : `osm-dark`.
5. L'infobulle VAC est en anglais (interface en anglais, arbitrage 3), date au format JJ/MM/AAAA.
6. La PAR se redessine chaque seconde : un clic à cheval peut se perdre (comme ses replis).
   Hélistations notées en point dans OSM non prises. Laissés : clés NEXRAD/NOAA, `setOverlayOpacity`
   sans appelant, cache à zéro dans le panneau Debug. Branche `claude/inspiring-cannon-5v61k9` identique.

## À refaire sur le Mac, navigateur réel, vrais flux

1. Console propre ; barre du haut sans AI Advisory. 2. **Écoute** : lancer, couper, relancer.
3. **Avion** : clic sur la carte, fiche sans simulation ; Tab et Maj+Tab dans les deux sens.
4. **PAR** : bascule, lien **VAC** de LFPG (le PDF s'ouvre), d'un LFPV (404 accepté).
5. **Réglages** : plus de « Simulated Aircraft » ; **Serveur** chargé. 6. **Fonds** : `ign-dark` par
défaut à Fontenay, les quatre boutons, rechargement ; **juger le filtre à l'œil**, réglable dans
`style.css`, `.basemap-dark .atc-basemap`. 7. **Couches** : afficher, masquer, opacité.
8. **Plan des aéroports** : `python3 tools/airport-layout/fetch.py --config configs/config.toml`,
noter taille et temps, puis CDG à z13 et z15 sur `ign-dark` et `osm-dark`, avec et sans la couche ;
couleurs dans `AIRPORT_LAYOUT_COLORS`, en tête de `openlayers-map-manager.js`. 9. **METAR, TAF,
NOTAM**. 10. Une journée sur l'instance d'essai avant la production.

**En priorité** : `ee27809` (fonds, préférence enregistrée du propriétaire), `d6ee78d` (fusion).
