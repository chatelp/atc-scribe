# Fiche B — Retirer côté interface ce qui n'a pas de rôle ici, et réparer la carte

*Palier 1 de l'audit (doc 32, ligne 1.1) et ligne 0.2 (fonds de carte), sous D71.3. Branche :
`cloud/b-retraits-interface`.
Lis d'abord `README.md` de ce dossier : ses règles s'appliquent.*

## Le but

Exécuter les **étapes 0 à 5** du plan de découpage de `docs-fr/audit-2026-10-09/frontend-code.md`
(section 5) : poser un filet, puis retirer environ 2 355 lignes de `www/` qui ne servent pas
ici, **sans rien déplacer** (déplacer change l'ordre d'initialisation ; retirer, non).
L'interface doit rester identique pour tout ce qui sert.

## Étapes, dans cet ordre, un commit chacune

0. **Le filet**, avant tout retrait : un script Node sans dépendance (`tools/www-check/check.mjs`,
   par exemple) qui vérifie que chaque `$store.atc.X` lu dans `index.html` (attributs Alpine :
   `x-text`, `x-show`, `:class`, `@click`, etc.) et chaque `store.X` / `Alpine.store('atc').X`
   des autres fichiers de `www/` **existe** dans le store défini par `app.js` ; que chaque
   fonction appelée depuis `index.html` existe ; et qu'aucun `<script src>` d'`index.html` ne
   pointe vers un fichier absent. Il sort en erreur sinon. Il doit passer sur `main` tel quel
   (l'audit : 169 noms, 0 non résolu). La façon de le lancer s'écrit en tête du script et
   dans ton rapport. Ce script tourne ensuite **à chaque commit**, avec `node --check` et
   `node --test www/par/`.
1. **Le chat IA « AI Advisory »** : `www/atc-chat.js`, son `<script>`, la tuile
   d'`index.html` (~2256-2371), sa règle CSS. Aucun nom du store n'en dépend (l'audit : 0
   référence depuis `app.js`) : vérifie-le.
2. **La simulation** : champs et fonctions du store (`app.js` ~465-474, ~1416-1558), les trois
   blocs d'`index.html` (volet ~493-541, fiche ~1593-1657, modale ~2552-2649), les lignes de
   `map/openlayers-map-manager.js` qui s'y rapportent.
3. **Les fonctions mortes** : les 21 fonctions jamais appelées et les 3 champs jamais lus de
   l'audit (section 3), moins celles déjà parties aux étapes 1 et 2. Corrige au passage le
   défaut latent signalé : `cleanup()` lit `window.animationEngine`, jamais assigné.
4. **Les fonds et couches américains** : VFR et IFR de la FAA (`vfr-sectional`, `terminal`,
   `ifr-low`, `ifr-high`), NEXRAD, NOAA (index.html, `app.js`, `www/map/core/map-engine.js`,
   `www/map/openlayers-map-manager.js`). **Le fond par défaut devient `osm`**, et c'est aussi
   le repli de `normalizeStyle` (aujourd'hui `'dark'`) : vérifié le 10/10, **c'est le seul fond
   qui marche encore** — les fonds Carto `dark` et `light` renvoient une tuile « API KEY
   REQUIRED », les fonds FAA sont vides sur Paris. **À cette étape, ne retire pas encore
   `dark` ni `light`** et ne touche pas à `www/sw.js` : c'est l'étape 6. **Attention
   aux préférences enregistrées** : un navigateur dont la clé `localStorage` `mapStyle` vaut un
   fond retiré doit retomber sur `osm`, pas sur une carte vide ; ne renomme pas la clé.
5. **Les clones** (section 4 de l'audit) : les `set*Opacity`, les `toggle*` de couches, les
   trois `toggle*Details` METAR/TAF/NOTAM, `cycleToNext/PreviousAircraft`, fusionnés en
   fonctions paramétrées. **Garde les anciens noms** comme enveloppes d'une ligne s'ils sont lus
   par `index.html` ou les autres JS (le filet te le dira), ou mets à jour `index.html` en même
   temps, dans le même commit.

## Puis la carte (ligne 0.2 du doc 32, décidée par le propriétaire le 10/10)

Lis d'abord `docs-fr/audit-2026-10-09/fonds-de-carte.md` : les fonds Carto `dark` et `light`
renvoient une tuile « API KEY REQUIRED », les fonds FAA sont vides sur Paris, seul `osm`
marche ; aucun fond sombre ne rend les pistes lisibles. Le propriétaire veut **un fond sombre
et des plans d'aéroport bien visibles**. Trois étapes, un commit chacune au moins.

6. **Des fonds sombres qui marchent, sans clé.**
   - Ajoute le **Plan IGN v2** (Géoplateforme, Licence Ouverte, sans clé ; couvre la France)
     en XYZ : `https://data.geopf.fr/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=GEOGRAPHICALGRIDSYSTEMS.PLANIGNV2&STYLE=normal&FORMAT=image/png&TILEMATRIXSET=PM&TILEMATRIX={z}&TILEROW={y}&TILECOL={x}`,
     attribution « © IGN – Plan IGN v2 ».
   - Deux styles nouveaux : `ign-dark` et `osm-dark`, le même fond **assombri par un filtre CSS
     appliqué à la seule couche de fond** (OpenLayers : un `className` propre à cette couche,
     pour qu'elle ait son canvas ; une règle du genre
     `filter: invert(1) hue-rotate(180deg) brightness(0.85) contrast(0.9)`). Le filtre ne doit
     toucher ni les avions (rendu WebGL), ni les traînées, ni les autres couches : vérifie-le.
     Garde `osm` et ajoute `ign` en clair.
   - **Défaut** : `ign-dark` si la station (sa position est connue du store) est en France
     métropolitaine (boîte 41–51,5 N, −5,5–9,8 E), sinon `osm-dark`. Repli de
     `normalizeStyle` et des préférences qui pointent un fond retiré : la même règle.
   - **Retire les fonds Carto `dark` et `light`** (cassés), avec la même règle de repli.
   - **`www/sw.js`** ne met en cache que les tuiles Carto, sans expiration, et y a figé le
     filigrane : il doit **vider ses caches à l'activation et ne plus intercepter les tuiles**
     (laisse-le enregistré, sans rien mettre en cache ; ne mets pas en cache les tuiles IGN ou
     OSM : leurs conditions demandent de respecter leurs en-têtes de cache).
7. **Le plan des aéroports, dessiné par nous.**
   - Un script `tools/airport-layout/fetch.py`, **bibliothèque standard Python seulement**
     (3.11+, `tomllib`, `urllib`) : il lit la position de la station dans le `config.toml` donné
     (`--config`, section `[station]`), interroge **Overpass** (miroir
     `https://overpass.openstreetmap.fr/api/interpreter` d'abord, `https://overpass-api.de/api/interpreter`
     ensuite ; un `User-Agent` qui nomme atc-scribe) pour les `aeroway` `runway`, `taxiway`,
     `apron`, `terminal`, `helipad` (pas les hangars) dans un rayon donné (`--radius-nm`,
     35 par défaut), ways et relations multipolygones, et écrit un GeoJSON compact (coordonnées
     à 6 décimales, propriétés : `aeroway`, `ref`, `name`, `width`, `surface`) dans
     `www/data/airport-layout.geojson`. Mesuré le 10/10 autour de la station : 88 pistes, 1 818
     taxiways, 289 aires, 73 terminaux, 2,1 Mo bruts hangars compris.
   - **Ce fichier n'est pas commité** : il est propre à chaque station (ajoute-le à
     `.gitignore`). Chaque utilisateur le génère pour chez lui, ce qui règle aussi la licence
     ODbL d'OpenStreetMap : rien n'est redistribué par le dépôt. Pour tes essais, génère-le
     autour de 48.80586 N 2.04932 E (la position est publique, sur FlightAware), sans le
     commiter. Documente la commande dans le README (section installation, en anglais).
   - Une **couche vectorielle** « Airport layout », chargée depuis `/data/airport-layout.geojson`
     (absente : pas de couche, **aucune erreur affichée**), au-dessus du fond, sous les
     traînées et les avions. Style lisible sur fond sombre comme clair : **pistes** remplies,
     larges de leur largeur réelle en mètres (tag `width`, 45 m par défaut) convertie en
     pixels selon la résolution, 2 px au minimum, d'un gris très clair, avec leur désignation
     (`ref`) à partir de z13 ; **taxiways** en trait fin jaune pâle à partir de z12 ; **aires**
     et **terminaux** en aplats discrets ; rien sous z9 sauf les pistes. Interrupteur dans le
     menu des couches, préférence dans une **nouvelle** clé `localStorage`, activée par défaut.
     Attribution de la couche : « © OpenStreetMap contributors (ODbL) ».
8. **La VAC en un clic** (vue PAR ; c'est la seule chose que tu fais dans `www/par/`).
   - Une fonction pure dans `www/par/` qui donne le cycle AIRAC en vigueur à une date (cycles de
     28 jours ; repère : 1ᵉʳ octobre 2026) et l'adresse du PDF du SIA :
     `https://www.sia.aviation-civile.gouv.fr/media/dvd/eAIP_<JJ>_<MMM>_<AAAA>/Atlas-VAC/PDF_AIPparSSection/VAC/AD/AD-2.<OACI>.pdf`
     (`MMM` en anglais majuscule : `eAIP_01_OCT_2026`). Ses tests dans le style de
     `par-geometry.test.js` : le 09/10/2026 donne `01_OCT_2026`, le 29/10/2026 `29_OCT_2026`, le
     30/09/2026 `03_SEP_2026`, et une date de 2027.
   - Un lien **« VAC »** à côté du nom de l'aérodrome dans la vue PAR, pour les codes `LF..`
     seulement, qui ouvre le PDF dans un **nouvel onglet** (`target="_blank" rel="noopener"`,
     jamais dans un cadre : les conditions du SIA interdisent l'imbrication) ; son infobulle
     cite « SIA, cycle AIRAC du JJ/MM/AAAA ». Les aérodromes militaires (LFPV…) n'ont pas de VAC
     au SIA : le lien mènera à une 404, c'est accepté.

**Ne touche pas** : la météo (METAR, TAF, NOTAM : active), le reste de la vue PAR, le
diagnostic de performance (volet Debug : il servira à mesurer la latence, palier 4.3), le volet
Station, l'accueil, les réglages, les alertes (paliers 1.5, 2.1, 2.2 : ils seront réécrits sur
le Mac), `login.js`, `audio-client.js`. Les alertes et les réglages, même s'ils contiennent du
mort, attendent leur refonte.

## Vérifications

À chaque commit : le filet, `node --check` sur chaque JS touché, `node --test www/par/`. Si tu
peux, sers `www/` en statique et charge `index.html` dans un navigateur sans tête pour
vérifier qu'aucune erreur JavaScript n'apparaît au chargement (le serveur absent donnera des
erreurs réseau, attendues ; une `ReferenceError` ou `TypeError` ne l'est pas). Mesure les
lignes retirées par étape.

## Rapport

`docs-fr/fiches-cloud/B-rapport.md` (voir README). En plus : la liste des gestes à refaire
sur le Mac dans un vrai navigateur, avec les vrais flux, pour valider (écoute, sélection
d'un avion, PAR, panneau Serveur, changement de fond, réglages des couches restantes) ; pour la
carte, des captures si ton navigateur sans tête en produit (fond sombre IGN et OSM sur CDG à
z13 et z15, avec et sans la couche des aéroports), sinon dis-le ; la taille du GeoJSON généré
et le temps de la requête Overpass. **Les réglages fins de couleur et de filtre se jugeront à
l'œil sur le Mac** : donne des valeurs raisonnables et dis où elles se règlent.
