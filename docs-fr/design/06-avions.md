# Les avions : inventaire, et ce que doit faire notre propre jeu

*13/10/2026, session design. Demande du propriétaire : avant de choisir un avion pour la marque,
faire l'inventaire de la façon dont l'interface représente les avions, puis dessiner notre propre
jeu, dans lequel on choisira celui du logo. La planche : `avions/inventaire.html` (servir la
racine du dépôt, `python3 -m http.server`, pour que les dessins de `www/assets/` se chargent).*

## Méthode

- Lecture du code : `www/map/renderers/aircraft-webgl.js` (choix, taille, teinte, rotation de
  l'icône), `www/par/par-view.js`, `www/store/sidebar.js`, `www/timeline/flight-timeline.js`,
  `index.html`, `tools/menubar/`.
- **Mesure** : cinq relevés du flux ADS-B de la station (`aircraft.json`, lecture seule), le 13/10
  vers midi, à 30 s d'intervalle, soit **94 avions distincts**. Pour chacun, le type vient de
  `assets/aircraft.csv` (base tar1090-db, la même que l'enrichissement du serveur), et l'icône est
  choisie avec les mêmes expressions régulières que la carte, portées en Python. Un relevé de
  midi en semaine ; le soir et le week-end, la part des avions légers est sans doute plus forte :
  **non mesuré**.

## Ce qui existe

| Où | Représentation | Taille |
|---|---|---|
| **Carte, par type** | 20 silhouettes vues de dessus (`www/assets/aircraft/*.svg`) : a320, b737, b787, a330, b777, b767, b747, a340, a380, md11, crjx, erj, e195, f100, fa7x, glf5, learjet, c130, dh8a, cessna | 27 px |
| **Carte, par catégorie ADS-B** | 8 icônes (`www/assets/a0–a7.svg`) quand aucun type ne correspond : inconnu, léger, petit, gros, fort sillage, lourd, haute performance, giravion | 21 à 31 px |
| Carte, avion choisi / survolé | cercle blanc / gris autour de l'icône | rayon ≈ taille |
| **Vue PAR** | un triangle de 10 px qui pointe dans le sens de la marche, et un point dans la bande d'azimut | 10 px |
| Onglet PAR du téléphone | Font Awesome `fa-plane-arrival` | 14 px |
| Chronologie de la fiche | `fa-plane-departure`, `fa-plane-arrival` (T/O, T/D) | 14 px |
| Liste | aucun avion : le badge de phase en tient lieu | — |
| Barre de menus | aucun avion : symboles SF d'ondes radio | 18 pt |

Sur la carte, l'icône est teintée par la bande d'altitude (`getColorizedAircraftIconSrc`), tournée
selon le cap, et sa taille dépend de sa classe (`getAircraftIconSize`, de 21 à 31 px).

## Ce que montre la mesure

Sur 94 avions :
- **74 (79 %)** reçoivent une silhouette par type, **11** l'icône de leur catégorie, **9** l'icône
  « inconnu » (aucun type reconnu et aucune catégorie émise ; 26 avions sur 94 n'émettent pas de
  catégorie) ;
- **deux silhouettes font 63 % de la carte** : A320 (37 avions : A318 à A321, A20N, A21N) et B737
  (22 : B738, B38M) ;
- viennent ensuite B787 (4), A330 (3), ERJ (2, des E190), B777 (2), Cessna (2), B767 (1), B747 (1) ;
- **onze silhouettes sur vingt n'ont servi à aucun avion** (A340, A380, MD-11, CRJ, E195, F100,
  Falcon 7X, Gulfstream V, Learjet, C-130, Dash 8).

Ce qui n'a pas de dessin et qui passait au-dessus de la station :
- **les avions légers** : Robin DR400 (×2, l'avion d'aéroclub français), Tecnam P2002 (×2),
  Diamond DA40, Mooney M20. Ils reçoivent l'icône « léger » ou « inconnu ». C'est pourtant le
  trafic d'un terrain comme LFPZ, celui qu'écoute la station ;
- l'Airbus A220 (×2), l'A350 (×2), les jets d'affaires récents (Global, G650, Falcon 2000), les
  turbopropulseurs (King Air, PC-12).

## Ce qui ne va pas

1. **À la taille de la carte, les dessins ne se distinguent pas.** A320, B737, B787, A330, B777,
   B767 et B747 sont la même croix à 27 px ; A0, A3, A4 et A5 aussi. Seuls les dessins à aile haute
   ou droite (Cessna, C-130, Dash 8) et l'hélicoptère se reconnaissent. Vingt-huit fichiers pour,
   à l'œil, quatre formes.
2. **Deux styles qui ne vont pas ensemble.** Les vingt silhouettes sont des tracés vectorisés
   depuis des images (coordonnées sur 512 pt, détail irrégulier : le Cessna a un trait de contour,
   le C-130 pèse 9,5 Ko contre 1,2 Ko pour le Cessna). Les huit icônes de catégorie viennent d'un
   autre dessin.
3. **L'origine des dessins n'est pas établie.** Les huit icônes de catégorie portent toutes
   l'identifiant `Thin-L`, celui des modèles d'export des symboles SF d'Apple : c'est un indice,
   pas une preuve, mais la licence des symboles SF ne permet pas leur usage hors des interfaces
   des plateformes Apple. Les vingt silhouettes n'ont aucune mention d'origine. Le README ne crédite
   aucune source de dessins. Pour un dépôt public, c'est un risque à lever, et la meilleure façon
   est de dessiner les nôtres.
4. **L'avion inconnu est dessiné comme un avion de ligne.** Quand on ne sait rien, la carte montre
   un biréacteur : elle affirme ce qu'elle ignore.
5. **Trois dessins d'avion différents** dans l'interface : les silhouettes de la carte, le triangle
   de la PAR, les avions de Font Awesome. Aucun ne sert à la marque.
6. **Du code mort** : `getPhaseIconClass` (`store/sidebar.js:466`) associe six icônes Font Awesome
   aux phases et n'est appelé nulle part.

## Ce que doit faire notre jeu

**Un seul dessin, une seule famille**, de la carte au logo. Les classes sont choisies sur ce que
la station voit et sur ce qui compte pour celui qui écoute, pas sur la liste des types :

| Classe | Couvre | Vu le 13/10 | Signe distinctif, vu de dessus |
|---|---|---:|---|
| Monomoteur léger | DR400, Tecnam, DA40, Mooney, Cessna, Piper | 6 | aile droite, grande envergure pour sa longueur, l'hélice en travers du nez |
| Turbopropulseur | ATR, Dash 8, King Air, PC-12 | 2 | aile droite, deux nacelles avec leur hélice |
| Jet d'affaires | Falcon, Gulfstream, Global, Learjet, Citation | 3 | petit, aile en flèche, deux réacteurs à l'arrière du fuselage |
| Régional et moyen-courrier biréacteur | A220, A320, B737, E190, CRJ | 66 | aile en flèche, deux réacteurs sous l'aile |
| Gros-porteur biréacteur | A330, A350, B777, B787, B767 | 10 | le même, plus grand, plus élancé |
| Gros-porteur quadriréacteur | A380, B747, A340 | 1 | quatre réacteurs sous l'aile |
| Hélicoptère | | 0 | le disque du rotor et la poutre de queue |
| Planeur | | 0 | aile très longue et fine, fuselage fin |
| Inconnu | aucun type, aucune catégorie | 9 | **pas un avion** : un chevron qui donne le cap, et rien d'autre |

Règles du dessin :
- **vu de dessus, nez en haut**, centré sur le centre de la boîte, qui est le point de rotation
  sur la carte ;
- **proportions réelles** (envergure, longueur, flèche, position de l'aile), simplifiées, et non
  inventées : le dessin vient de cotes, pas d'une image ;
- **lisible à 24 px**, où la carte l'affiche : les différences entre classes doivent tenir à cette
  taille (hélice, nacelles, réacteurs arrière, flèche), sinon la classe ne sert à rien ;
- **la taille dit la classe**, comme aujourd'hui (du léger au gros-porteur) ;
- **un aplat sans contour**, teinté par la carte (rampe d'altitude, `03-systeme.md`), comme
  aujourd'hui ;
- **une seule source** : un script qui dessine chaque classe depuis ses cotes, pour que la famille
  reste cohérente et qu'on puisse la retoucher d'un paramètre ;
- **à nous, sous la licence du dépôt (MIT)**.

Pour la PAR, le triangle reste : à 10 px, de profil, une silhouette n'apporterait rien. Le logo
sera tiré de la famille, choisi parmi les classes une fois dessinées.

## Notre famille, première version (13/10)

`node tools/design/avions.mjs` écrit les neuf classes dans `avions/famille/` (une par fichier SVG,
aplat noir comme la carte les teinte, boîte de 64 centrée sur le point de rotation) et
`famille.json` (nom, types couverts, taille sur la carte). La planche `avions/famille.html` les
montre en grand, aux tailles de la carte, à côté de l'ancien dessin de la même classe, et dans le
même bout de ciel. Cotes de référence (arrondies) : DR400 / C172, ATR 72, Falcon 2000, A320,
A350-900, B747-400, H125, ASK 21. L'épaisseur minimale est de 3 unités sur 64, soit 1,1 px à
24 px : elle épaissit l'hélice, les nacelles, la poutre et le rotor de queue, le fuselage du
planeur.

Tailles sur la carte proposées : inconnu 18, léger 20, affaires, hélicoptère et planeur 22,
turbopropulseur 24, moyen-courrier 26, gros-porteur 30, quadriréacteur 32 px.

Le logo, finalement, n'est pas un avion : le 13/10, sur une planche de huit signes (trois tours,
trois radars, deux avions de la famille), le propriétaire a retenu le scope radar
(`05-identite.md`). La famille reste celle de la carte.

Ce qui reste ouvert :
l'adaptation des règles de `getAircraftIconClass` aux neuf classes (un code de type → une classe,
par la base `aircraft.csv` et la catégorie ADS-B) ; le remplacement des 28 fichiers de
`www/assets/`, au moment de la migration.
