# Fonds de carte et couches aéronautiques : audit du 09/10/2026

Mesuré par `curl` le 09/10/2026 (date système et en-têtes serveurs ; la demande disait 10/10). Tuile de référence : CDG z14 = `14/8307/5625` (z13 `13/4153/2812`, z15 `15/16615/11251`). Tuiles dans le scratchpad, rien dans le dépôt. « o » = octets. Non mesuré : tout ce qui exige une clé que je n'ai pas (openAIP, CARTO avec clé, SCAN OACI). Les conditions de licence (CARTO, Stadia, OSMF, SIA, OFM) ont été lues sur les pages officielles via un résumé automatique : à relire dans le texte avant toute publication.

## Constat urgent : le fond CARTO de l'app est cassé
`dark_all` répond **HTTP 200, 2 513 o, toujours le même MD5** : c'est la tuile « API KEY REQUIRED ». Mesuré sur a/b/c/d, z3, z14, z15, z16, `@2x`, `dark_nolabels`, avec ou sans Referer ; `light_all` et `voyager` idem (2 049 o). Cause : clé gratuite exigée par CARTO. `www/sw.js` est en cache d'abord, sans expiration, 1 500 entrées : les anciennes bonnes tuiles masquent la panne sur les zones déjà vues, et chaque nouvelle tuile y fige le filigrane.

## 1. Fonds sombres
| Source | URL de tuile | Clé | Licence / conditions | Vérifié sur CDG z14 |
|---|---|---|---|---|
| CARTO dark_all (raster, actuel) | `https://basemaps.cartocdn.com/rastertiles/dark_all/{z}/{x}/{y}.png?key=…` | **oui**, formulaire e-mail, sans compte | Non commercial gratuit jusqu'à 5 M req/mois ; attribution « © OpenStreetMap contributors, © CARTO » ; pas de proxy ni cache serveur, cache navigateur 30 j max (carto.com/legal/basemap-terms, v. 29/09/2026) | sans clé : 200, 2 513 o (filigrane) ; `?key=bidon` : idem ; avec clé : non testé |
| CARTO Dark Matter (vectoriel) | `https://basemaps.cartocdn.com/gl/dark-matter-gl-style/style.json` | annoncée aussi (« vector suivra ») ; encore libre | idem | style 200 70 431 o ; tuile mvt 200 35 313 o. **Pistes `#111` sur fond `#0e0e0e` (contraste 1,02:1)** : capture z14 CDG, routes nettes, pistes et taxiways invisibles (11 pistes et 218 taxiways pourtant dans les données) |
| Stadia `alidade_smooth_dark` | `https://tiles.stadiamaps.com/tiles/alidade_smooth_dark/{z}/{x}/{y}.png` | sans Referer : 401 ; Referer localhost ou `192.168.1.10` : 200 (mesuré, la doc dit clé pour un intranet) | Plan gratuit : 200 000 crédits/mois, « commercial use not allowed » ; attribution © Stadia Maps © OpenMapTiles © OpenStreetMap | 401 14 885 o sans Referer ; 200 22 179 o avec. Rendu : pistes en simple filet, surfaces non remplies |
| Esri World Dark Gray Base | `https://server.arcgisonline.com/ArcGIS/rest/services/Canvas/World_Dark_Gray_Base/MapServer/tile/{z}/{y}/{x}` | non | Esri Master License Agreement ; attribution « Esri, HERE, Garmin, © OSM contributors » ; service « legacy », étiquette `retiring-2029-12` ; export hors ligne exclu | 200, 10 629 o (jpeg). z17-18 : 2 521 o (vide probable). Rendu : pistes en bandes gris moyen sur gris foncé, faible contraste, sans noms |
| VersaTiles « eclipse » (vectoriel) | tuiles `https://tiles.versatiles.org/tiles/osm/{z}/{x}/{y}` ; style `…/assets/styles/eclipse/style.json` | non | gratuit, « © OpenStreetMap contributors », disponibilité non garantie | style 200 167 122 o ; tuile 200 29 484 o. Pistes `rgb(82,82,82)` sur `rgb(40,39,37)` : **1,9:1**, le meilleur du lot (calculé sur le style, rendu non vérifié) |
| OpenFreeMap « dark » (vectoriel) | style `https://tiles.openfreemap.org/styles/dark` | non | gratuit « as-is » ; © OpenMapTiles, données OSM | style 200 20 959 o ; tuile pbf 200 49 173 o. Pistes `#000` + liseré `rgba(60,60,60,.8)` : en contour seulement (rendu non vérifié) |

Les trois vectoriels exigent MapLibre ou `ol-mapbox-style` avec OpenLayers 10 : ce ne sont pas des remplaçants d'une ligne.

## 2. Couches aéronautiques
| Source | URL / service | Clé | Licence | Vérifié le 09/10 |
|---|---|---|---|---|
| IGN SCAN OACI (Géoplateforme) | couche `GEOGRAPHICALGRIDSYSTEMS.MAPS.SCAN-OACI`, WMTS `https://data.geopf.fr/private/wmts` (TILEMATRIXSET=PM) | clé personnelle du compte cartes.gouv.fr, profil avec SIRET (entreprise ou association) | Fiche IGN : « pas libres de droit », **les particuliers ne sont pas autorisés à les télécharger, même à des fins personnelles** ; usage pro/asso gratuit ; grand public numérique = licence payante (licence 2021 relayée, non relue à la source) | WMTS public : 400 « Layer … unknown » (188 o) pour 4 noms ; `/private` : 401 (139 o). La couche existe au catalogue (édition avril, le SIA fabrique la carte depuis 2026) mais est **inutilisable ici** |
| openAIP | `https://api.tiles.openaip.net/api/data/openaip/{z}/{x}/{y}.png?apiKey=…` (`.pbf` vectoriel ; style `/api/styles/openaip-default-style.json`, 200 sans clé) | **oui** (compte, « API Client ») ; sans clé 403 (98 o), clé bidon 404 | Données **CC BY-NC 4.0** ; attribution lien openaip.net ; limites de débit, cache chez soi conseillé | Contenu d'après le style : aérodromes + pistes, espaces (CTR, TMA, zones…), balises, obstacles, points de report. Rendu PNG non vu (pas de clé) |
| Proxy ADS-B Exchange (déjà dans `openlayers-map-manager.js:231`) | `https://map.adsbexchange.com/mapproxy/tiles/1.0.0/openaip/ul_grid/{z}/{x}/{y}.png` | non | aucune autorisation trouvée, grille `ul_grid` non standard, données openAIP NC | 200, 9 321 o (z10). **À écarter** |
| Open Flightmaps (LF couvert) | surcouche `https://nwy-tiles-api.prod.newaydata.com/tiles/{z}/{x}/{y}.png?path=2610/aero/latest` ; fond `.jpg?path=2610/base/latest` ; tuiles 512 px | non | Licence OFMA General Users' License : usage commercial inclus, attribution OFMA ; serveur d'un tiers, sans engagement | z12 : 200, 8 632 o (pistes bleues sur zone teintée) ; **z13+ : 200, 1 233 o, transparent** ; cycle AIRAC à changer toutes les 4 semaines (2611 le 29/10) |
| Plan IGN v2 (fond, pas aéro) | `https://data.geopf.fr/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=GEOGRAPHICALGRIDSYSTEMS.PLANIGNV2&STYLE=normal&FORMAT=image/png&TILEMATRIXSET=PM&TILEMATRIX={z}&TILEROW={y}&TILECOL={x}` | non | Licence Ouverte ; `Fees: none` ; CGU cartes.gouv.fr/cgu | z13 36 665 o ; z14 61 678 o ; z15 30 988 o. Pistes et taxiways nets, thème clair uniquement |
| IGN `TRANSPORTS.DRONES.RESTRICTIONS` | même WMTS, matrice PM | non | idem | z12 : 200, 23 009 o, **quasi tout rouge** sur Paris : inutile |

FAA VFR/IFR (ArcGIS, déjà dans `map-engine.js`) : États-Unis seulement, non testé, inutile pour Paris.

## 3. Pistes et plans d'aéroport
**Tuile OSM standard sur CDG (vue à l'image).** z13 (18 713 o) : pistes en larges bandes grises sans libellé, taxiways en filets, terminaux en aplats. z14 (24 241 o) : T1 « camembert », deux pistes nord et réseau de taxiways nets. z15 (12 133 o) : désignation de piste (« 27L ») et noms de taxiways (Y3, Y5, BD9…) lisibles. z16 (7 521 o) : piste à pleine largeur. Orly z14 (21 300 o) : piste et taxiways nets. z12 non mesuré. Donc les pistes sont lisibles dès z13 sur OSM **clair** ; aucun rendu **sombre** testé ne fait mieux (voir tableau 1).

| Données ouvertes | Accès | Licence | Mesure |
|---|---|---|---|
| OurAirports `runways.csv` | `https://davidmegginson.github.io/ourairports-data/runways.csv` | domaine public, régénéré chaque nuit | 200, 3 969 443 o ; seuils lat/lon, cap, longueur, largeur : LFPG 4 pistes + 1 hélistation herbe, LFPO 3, LFPB 3, LFPN 2, LFPV 1, LFPZ 2 |
| OSM Overpass | `https://overpass.openstreetmap.fr/api/interpreter` (POST) | ODbL (attribution, partage à l'identique des bases dérivées) | `overpass-api.de`, `lz4`, `z` : 504 ; `kumi`, `private.coffee` : 500 ; `.openstreetmap.fr` : 200, 308 169 o en 0,8 s. Pistes/taxiways/aires/terminaux : LFPG 7/779/42/24, LFPO 3/168/29/3, LFPB 6/113/22/8, LFPN 2/126/24/10, LFPV 4/25/5/3, LFPZ 2/79/15/1 |
| openAIP export | compte requis ; bucket Google public = 400 « requester pays » | CC BY-NC 4.0 | non récupéré |

## 4. Cartes VAC du SIA
- **Format : PDF**, plusieurs pages (LFPZ : 8 p., 607 123 o), non géoréférencé (aucune structure GPTS/Measure). Note SIA de mars 2021 : « pas en mesure de proposer » de produits géoréférencés. Ni tuiles ni API publique trouvées ; le Visualisateur AIP (`/vaip`) et l'appli SOFIA-VAC les affichent sur carte.
- **Lien direct, mesuré** : `https://www.sia.aviation-civile.gouv.fr/media/dvd/eAIP_01_OCT_2026/Atlas-VAC/PDF_AIPparSSection/VAC/AD/AD-2.<OACI>.pdf` : LFPG 200 (497 474 o), LFPO 200 (307 611), LFPB 200 (618 929), LFPN 200 (760 031), LFPZ 200 (607 123), **LFPV 404** (aérodrome militaire de la liste 2, hors Atlas VAC).
- **Pas stable** : le dossier porte la date du cycle AIRAC (28 jours, 01/10 puis 29/10/2026). L'ancien (`03_SEP_2026`), `29_OCT_2026` (pas encore publié), `eAIP_current` et `eAIP_latest` donnent 404. Il faut calculer la date du cycle et prévoir un repli vers `/documents/htmlshow?f=dvd/eAIP_01_OCT_2026/Atlas-VAC/home.htm`.
- **Réutilisation** (page Informations légales) : information publique librement réutilisable si non altérée, sens non dénaturé, source et date de mise à jour mentionnées ; lien direct permis sans autorisation, mais pas d'imbrication dans une autre page. Un lien sortant est sûr ; une copie dans le dépôt demande de garder la date et ne sert à rien (périmée en 28 jours).

## Recommandation
1. **Corriger d'abord le fond actuel** : soit une clé CARTO gratuite (le propriétaire la demande avec son e-mail, 5 M req/mois non commercial), à saisir en configuration locale et jamais dans le dépôt ; soit un changement de fond. Vider le cache du service worker dans les deux cas.
2. **Sombre avec pistes lisibles, sans clé** : le seul candidat mesuré est VersaTiles « eclipse » (1,9:1), qui demande MapLibre ; à valider à l'œil avant décision. À défaut, OSM ou Plan IGN clair filtré en CSS (inversion), qui garde les pistes nettes dès z13.
3. **Dessiner les pistes nous-mêmes** : `runways.csv` (domaine public) donne les seuils de LFPG, LFPO, LFPB, LFPN, LFPZ ; taxiways et aires via Overpass `.openstreetmap.fr` en un script ponctuel, GeoJSON statique avec attribution ODbL. Aucun droit à acheter, rendu identique sur tout fond.
4. **Espaces aériens** : openAIP (CC BY-NC, clé personnelle, jamais committée) ou Open Flightmaps (sans clé, z12 au plus, serveur tiers sans garantie). Écarter SCAN OACI (licence) et le proxy ADS-B Exchange.
5. **VAC** : bouton qui ouvre le PDF du SIA par lien calculé sur le cycle AIRAC, avec repli vers l'Atlas ; rien dans le dépôt ; pas de VAC pour LFPV.
