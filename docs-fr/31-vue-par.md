# 31 — La vue PAR : le ciel de profil, piste par piste

> **État au 29/09 : proposition validée par le propriétaire (D67), rien n'est codé.**
> Ce document est à la fois la conception et le **suivi de l'implémentation** : le tableau de la
> dernière partie se tient à jour à chaque étape, avec ce qui a été mesuré pour la valider.

## La demande

Le propriétaire, le 29/09 : *« j'aimerais bien avoir une vue "horizon" switcheable dans co-atc pour
chaque aéroport suivi individuellement. Au lieu d'une vue du dessus, ça montre les avions en live,
qui se posent, décollent… »*

Puis, sur les propositions :

- *« si ça existe vraiment c'est encore mieux, dans ce cas on peut coller aux conventions des outils
  "PAR" (tout en restant accessible comme co-atc en général) »* ;
- *« oui profil le long de piste comme proposé. Effectivement il faudrait une vue par piste »* ;
- *« ça serait bien qu'on ait plus loin que phases d'approche ou décollage. Mais c'est sûr que ça
  pose la question du rattachement à quelle piste si on n'a pas l'axe de piste »* ;
- *« toutes les pistes côte à côte (les unes en dessous des autres ?) »* ;
- *« j'aimerais bien que le ciel soit découpé avec les FL standards par ex. surtout que co-atc est
  capable de l'inférer de l'ADS-B et/ou transmissions audio pour un avion donné »* ;
- *« profil et azimut ne peuvent pas être affichés conjointement ou ça n'a pas de sens ? »* — si :
  c'est justement la convention du PAR (ci-dessous).

## La référence : l'écran du PAR

Le **PAR** (*Precision Approach Radar*, radar d'approche de précision) est un radar posé au bord de
la piste qui guide un avion à la voix jusqu'au seuil. Son écran est exactement cette vue « de
profil » :

- **en haut, l'élévation** : l'altitude de l'avion en fonction de sa distance au seuil, avec la
  **pente de descente** tracée (3° d'ordinaire, environ 300 ft par NM) et ses lignes de tolérance ;
- **en bas, l'azimut** : l'écart latéral de l'avion à l'axe de piste, en fonction de la même
  distance, avec l'axe tracé et ses tolérances ;
- **les deux bandes partagent l'axe des distances** : un avion y apparaît deux fois, à la même
  abscisse, et l'œil lit d'un coup « trop haut et à gauche ».

La couverture d'un PAR est un secteur étroit devant la piste : d'après l'annexe 10 de l'OACI, de
l'ordre de 20° en azimut, 7° en élévation et 9 NM de portée (cité de mémoire, **non vérifié dans le
texte**). C'est le point de départ ; on l'élargit pour la demande « plus loin que les phases
d'approche ».

On en garde les conventions, pas l'austérité : les étiquettes, les couleurs de phase et le clic
restent ceux de la carte de co-atc.

## Ce qui est décidé

### Une vue par piste physique, les deux sens dans le même cadre

Une piste physique (06-24 à Orly) est un cadre. La piste est dessinée à plat, à son altitude, et
son axe est prolongé **des deux côtés** : devant un seuil arrivent les avions qui s'y posent, et
derrière l'autre extrémité partent ceux qui en décollent. Arrivées et départs du même sens
d'exploitation sont donc dans le même cadre, de part et d'autre de la piste.

Le cadre s'oriente pour que **les arrivées du sens en service aillent toujours de gauche à droite**,
vers la piste, comme sur un écran PAR. Quand le sens d'exploitation s'inverse (24 → 06), le cadre
se retourne.

```
 FL150 ┤        ╲ limite 7°
       │         ✈ AFR23P FL120 ↓ vise FL80
 FL100 ┤                    ╲
       │                           ╲                                              ↗ EZY41TK
       │· · · · ·⋱· · · · · · · · · · ·╲· · · · · · · · ·  transition · · · · · ·↗· · · · · · · ·
  5000 ┤                ⋱                 ╲                                    ↗
       │                      ⋱ pente 3°    ╲                               ↗
  3000 ┤                            ⋱          ╲                         ↗
       │                                  ✈ TVF62Y╲                    ↗
  1000 ┤                                         ⋱  ╲                ↗
   290 ┼───────────────────────────────────────────────▬▬▬▬▬▬▬▬▬▬▬▬───────────────────────────────
                20 NM     15        10        5       24         06         5         10
       ───────────────────────────────────────────────────────────────────────────────────────────
       │         · AFR23P
   az. ┼──────────────────────────────────✈ TVF62Y─────▬▬▬▬▬▬▬▬▬▬▬▬──────────────↗ EZY41TK────────
       │
       ───────────────────────────────────────────────────────────────────────────────────────────
```

*(croquis, pas à l'échelle, indicatifs inventés. Orly en 24 : les arrivées viennent de la gauche
sur la pente de 3°, les départs montent à droite après l'extrémité 06. Dessous, la bande d'azimut :
AFR23P est dans le cône mais à gauche de l'axe, il est estompé en haut.)*

### Toutes les pistes empilées, la piste en service en premier

Les cadres de toutes les pistes de l'aéroport sont **les uns sous les autres**, à la même échelle
des distances, **la piste en service en tête et mise en avant** (celle que co-atc déduit déjà par
aéroport, D62). Chaque cadre se replie en une ligne, ou se masque. Un aéroport par onglet, la vue
se bascule avec la carte.

### Deux bandes par piste : élévation et azimut

La bande d'élévation porte l'essentiel ; la bande d'azimut est mince (le quart de la hauteur
environ). Un avion **hors des tolérances d'azimut** reste dans la bande d'élévation, estompé : il
est dans le cône, mais pas sur l'axe.

### Le rattachement à une piste : la géométrie seule

La question posée par le propriétaire — à quelle piste rattacher un avion qui ne suit pas un axe —
se règle par un **cône de couverture** à chaque extrémité de chaque piste, comme celui d'un PAR mais
plus large :

- **20 NM** de portée depuis le seuil ;
- **±20°** autour de l'axe prolongé ;
- **jusqu'à 7°** au-dessus de l'horizontale, soit environ 15 000 ft à 20 NM (davantage pour un
  avion qui s'éloigne de la piste : voir V6).

Un avion dans un cône apparaît dans le cadre de cette piste, **quelle que soit sa phase** : c'est
ce qui donne « plus loin que l'approche et le décollage ». Un avion dans les cônes de **deux pistes
parallèles** va à celle dont l'axe est le plus proche. Les trois valeurs sont des réglages.

Ce rattachement est **indépendant de l'attribution d'aéroport de D62** (axe à 0,5 NM, 10 NM, sous
5 000 ft) : celle-ci décide des phases ; le cône ne décide que de l'affichage. Un avion peut donc
figurer dans l'onglet d'Orly et dans celui de De Gaulle s'il est dans leurs deux cônes.

### Le ciel découpé en niveaux standard

L'axe vertical est gradué **en niveaux de vol tous les 1 000 ft au-dessus du niveau de transition**
(FL60, FL70… FL150) et **en pieds QNH en dessous de l'altitude de transition** (1 000, 2 000…),
avec la couche de transition marquée entre les deux.

L'axe mesure l'**altitude QNH**, la hauteur vraie à la température près : c'est ce qui permet de
tracer une pente de 3° qui part du seuil et de comparer un avion à cette pente. Les graduations de
niveaux de vol sont placées là où ces niveaux se trouvent ce jour-là ; avec un QNH de 1 016 hPa,
FL70 est à environ 7 075 ft. L'écart est petit (environ 27 ft par hPa), mais il est juste.

### Pour chaque avion : le niveau où il est, celui qu'il vise, celui qu'on a entendu

- **Où il est** : l'altitude barométrique de l'ADS-B (`alt_baro`), corrigée du QNH sous la
  transition. C'est la position du symbole.
- **Ce qu'il vise** : l'altitude sélectionnée au pilote automatique (`nav_altitude_mcp`), en repère
  sur l'axe et en flèche depuis l'avion. Mesuré le 29/09 à 15:27 : **73 avions positionnés sur 77**
  l'émettaient.
- **Ce qu'on a entendu** : le dernier niveau ou altitude **lu par la grammaire** dans une
  transmission rattachée à cet avion. Juge J2 (doc 29) : **84 % des niveaux lus étaient justes** le
  27/09, avant la correction des valeurs impossibles du 29/09.
- **Accord** : discret (une coche) quand le niveau entendu et le niveau visé concordent ; **en
  désaccord, les deux sont montrés, sans jugement**. co-atc ne sait pas qui a raison, et la
  transcription se trompe bien plus souvent que l'équipage : un niveau lu sur six est faux (J2).

C'est la première place où co-atc confronterait une instruction entendue à ce que l'équipage a
affiché — une case « ❌ absent » du doc 26.

### Le reste, comme la carte

Étiquette d'indicatif, altitude et flèche de montée ou de descente, courte traîne, couleurs de
phase de la carte, clic qui sélectionne l'avion comme sur la carte. Au sol, seuls les avions sur la
piste même apparaissent.

**Ce que la vue n'est pas** : un outil de contrôle. Elle n'alerte sur rien, ne juge aucun écart à
la pente, et ne dit jamais qu'un avion est « trop bas ».

## Les données : d'où vient chaque chose

| Donnée | Source | Vérifié le 29/09 |
|---|---|---|
| Position des seuils | `assets/runways.csv` (OurAirports), servie par `/api/v1/station`, champ `airports[].runways` | ✅ servie, **position seule** |
| Altitude du seuil, cap vrai, seuil décalé | `assets/runways.csv` : `le_elevation_ft`, `le_heading_degT`, `le_displaced_threshold_ft` | ✅ présents pour les 7 pistes d'Orly et De Gaulle ; **pas servis** à l'interface |
| Piste en service | `RunwayInUseTracker`, un par aéroport (D62), `airports[].runway_in_use` | ✅ servie |
| Avions : position, altitude, niveau visé, QNH affiché | ADS-B par le websocket existant : `alt_baro`, `nav_altitude_mcp`, `nav_qnh`, `baro_rate` | ✅ 77 positionnés : `alt_baro` 77, `nav_altitude_mcp` 73, `nav_qnh` 73, `baro_rate` 75 |
| QNH du jour | **à trancher** (V1) : le calage affiché par les avions bas, ou le METAR | 9 avions sur 10 sous 6 000 ft émettaient `nav_qnh`, entre 1 014 et 1 016 hPa |
| Pente de chaque piste | PAPI des cartes VAC | ✅ 3,0°, une à 3,4° à Orly ; les VAC donnent aussi l'altitude des terrains, 291 ft à Orly et 392 ft à De Gaulle, cohérente avec `runways.csv` |
| Altitude et niveau de transition | **à relever** dans l'eAIP France, par aéroport (V2) : elle **n'est pas sur les VAC** | ❌ |
| Niveau entendu | la grammaire (`phraseology`), rôles « altitude » et « niveau de vol » | ❌ **pas rattaché aux avions aujourd'hui** : les autorisations gardées par avion (`ClearanceData`) ne sont que décollage et atterrissage |

Deux remarques sur les seuils :

- la position fournie est l'**extrémité physique** de la piste ; le seuil décalé, quand il existe
  (984 ft pour la 06 d'Orly, 1 427 ft pour la 25, 1 969 ft pour la 26R et la 27L de De Gaulle), est
  là où la pente de descente arrive : il faut l'appliquer ;
- les altitudes de seuil vont de 277 à 392 ft : les ignorer abaisserait toute la pente d'autant,
  soit plus d'un mille nautique sur une pente de 3°.

## Réglages par défaut proposés

| Réglage | Défaut | Pourquoi |
|---|---|---|
| Portée du cône | 20 NM | au-delà de l'approche (10 NM pour les phases, D62), en restant lisible |
| Demi-ouverture en azimut | 20° | le double du PAR, pour attraper les avions en dernier virage |
| Élévation maximale | 7° vers la piste, 15° en s'éloignant (V6) | celle du PAR pour les arrivées, environ 15 000 ft à 20 NM ; plus pour les départs, qui montent plus raide |
| Pente de descente | celle du PAPI de la piste, 3° à défaut | VAC du propriétaire (`~/Downloads/AD-2.LFPO.pdf`, `AD-2.LFPG.pdf`) : PAPI à 3,0° partout, sauf un à 3,4° à Orly — la 20, d'après la mise en page du texte extrait, à confirmer sur la carte |
| Hauteur de franchissement du seuil | 50 ft | la valeur usuelle d'un ILS |
| Tolérance d'élévation | ±0,7° | l'ordre de la pleine échelle d'un glide ILS |
| Tolérance d'azimut | ±2,5° | l'ordre de la pleine échelle d'un localizer |
| Traîne | 60 s | celle d'une lecture « d'où il vient » sans encombrer |

## Questions ouvertes

- **V1 — Le QNH.** Le calage affiché par les avions bas autour de l'aéroport (`nav_qnh`, médiane des
  avions sous l'altitude de transition dans les cônes) est présent, frais, et dit ce que les
  équipages ont réellement affiché. Le METAR n'est chargé que pour l'aéroport principal (D62) et
  arrive brut (`interface{}`), à décoder. **Recommandation** : l'ADS-B d'abord, le METAR à défaut,
  et sinon l'altitude pression, en le disant sur l'axe.
- **V2 — L'altitude et le niveau de transition** de chaque aéroport, à relever dans l'eAIP. Le
  niveau de transition varie avec le QNH et c'est le contrôle qui le donne ; l'ATIS l'annonce. Un
  réglage par aéroport suffit pour commencer.
- **V3 — Les pistes sécantes.** Orly a 02-20 en travers de 06-24 et 07-25 : un avion peut être dans
  les cônes de deux pistes non parallèles. **Proposition** : la règle de l'axe le plus proche vaut
  aussi pour elles, et un avion à égale distance de deux axes reste dans le cadre où il était, pour
  ne pas sauter d'une piste à l'autre à chaque mise à jour.
- **V4 — Où calculer la géométrie.** **Recommandation** : dans le navigateur. Tout ce qu'il faut y
  arrive déjà, sauf les champs de piste (étape 1) et le niveau entendu (étape 6), et la vue
  n'influe sur rien d'autre. Le calcul vit dans un module pur, testé avec `node --test` — ce serait
  le premier test JavaScript du dépôt.
- **V5 — Où loger la vue.** Un onglet par aéroport à côté de la carte, ou un panneau qu'on ouvre
  sous la carte : à décider sur une maquette.
- **V6 — Le plafond des départs.** Le cône teste l'angle sous lequel on voit l'avion **depuis le
  bout de piste**. Mesuré le 29/09 de 15:38 à 15:44 (35 relevés ADS-B, De Gaulle face à l'est, chaque
  avion rattaché à l'axe le plus proche) : **les 4 départs de De Gaulle ont tous dépassé 7°**, entre
  9,2° et 13,5°, **sur leurs 4,5 à 6,4 premiers NM**. Ils rentrent ensuite dans le cône en se
  stabilisant vers 4 000 à 5 000 ft. Avec 7°, la vue perdrait donc chaque décollage juste après la
  piste, là où on veut le voir. Avec 15°, aucun des quatre n'en sort. En revanche, deux avions en
  croisière vus au même moment, à FL180-200 et FL257 entre 13 et 20 NM, étaient sous 11,5° et 12,4° :
  **15° seul les ferait entrer**, ce que le plafond absolu d'environ 15 000 ft empêche. **Proposition**
  : 15° pour un avion qui s'éloigne de la piste, avec ce plafond absolu. Limites : 6 minutes, un seul
  aéroport, un seul sens d'exploitation ; aucun départ d'Orly n'a été capté.

## Suivi de l'implémentation

Chaque étape se valide par ce qu'elle permet de **mesurer ou vérifier**, inscrit dans le journal
ci-dessous au moment où elle est faite.

| # | Étape | Vérification | État |
|---|---|---|---|
| 1 | **Pistes complètes servies** : altitude du seuil, seuil décalé, pente, pour chaque aéroport suivi | les 7 pistes d'Orly et De Gaulle servies avec des valeurs identiques à `runways.csv` ; test Go | 🔲 |
| 2 | **Géométrie** : distance le long de l'axe, écart latéral, angle d'élévation, test du cône, piste la plus proche | tests `node --test` ; sur une heure de trajectoires de la base, les avions posés sur une piste (événements d'atterrissage du `RunwayInUseTracker`) sont sur cette piste à 5 NM du seuil dans au moins 98 % des cas ; part des départs gardés jusqu'à 10 NM avec 7° et 15° (V6) | 🔲 |
| 3 | **Cadre d'une piste, sans avion** : axe gradué en niveaux et en pieds, transition, pente de 3° et tolérances, bande d'azimut | capture comparée au croquis ; FL70 placé selon le QNH | 🔲 |
| 4 | **Avions en direct** : symbole, étiquette, flèche verticale, traîne, couleurs de phase, clic | un avion suivi 10 minutes sur la carte et dans la vue au même moment ; correction du QNH jugée au toucher : l'altitude corrigée des derniers points avant l'atterrissage tombe à ±100 ft de l'altitude du seuil sur une journée d'atterrissages | 🔲 |
| 5 | **Pistes empilées** : piste en service en tête, repli, masquage, retournement quand le sens change | un changement de sens d'exploitation observé en direct ou rejoué | 🔲 |
| 6 | **Niveau visé et niveau entendu** : `nav_altitude_mcp` affiché ; le dernier niveau lu par la grammaire rattaché à l'avion et servi | taux d'accord entendu / visé sur une séance, à comparer au J2 du doc 29 | 🔲 |
| 7 | **Un onglet par aéroport**, bascule avec la carte, réglages au panneau | Orly et De Gaulle ouverts ensemble, un avion dans les deux cônes visible dans les deux | 🔲 |

États : 🔲 à faire · 🟡 en cours · ✅ fait.

### Journal

*(vide — une entrée datée par étape, avec la mesure qui la valide)*
