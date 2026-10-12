# Le système de design

*Troisième livrable de la mission design (`MISSION.md`, §4), 12/10/2026. Il applique les dix
principes (`02-principes.md`) aux constats de l'inventaire (`01-inventaire.md`). Il ne dépend pas
du nom (D74) : la marque viendra après le choix du propriétaire, sur les mêmes jetons.*

## Comment il est fait

- **La source unique des valeurs est [`jetons.json`](jetons.json)** (91 jetons, format du groupe
  W3C Design Tokens : `$value`, `$type`, `$description`).
- **`node tools/design/tokens.mjs`** les vérifie et les écrit :
  - [`maquettes/tokens.css`](maquettes/tokens.css), les propriétés CSS dont se servent les
    maquettes ;
  - [`03-systeme-verif.md`](03-systeme-verif.md), chaque contraste et chaque distance pour les
    daltoniens.
  Le script sort en erreur si une exigence recule : on ne peut pas éclaircir un fond ou foncer un
  texte sans le voir.
- Les valeurs n'ont pas été choisies à l'œil. Les gris, les verts et l'urgence l'ont été par
  calcul de contraste. Les familles de phase l'ont été par une recherche sur 40 000 tirages : on
  cherchait la plus grande distance minimale sous les quatre visions (normale, protanopie,
  deutéranopie, tritanopie), puis les teintes ont été attribuées par le sens.
- **Ce qui ne change pas** : le fond `#0a0a0a`, les panneaux `#1a1a1a`, le texte principal
  `#e0e0e0`, la bordure `#333`, le vert `#4CAF50`. L'inventaire les a trouvés justes.

---

## 1. Couleurs

### Surfaces

| Jeton | Valeur | Rôle |
|---|---|---|
| `surface.map` | `#000000` | carte et PAR (fond « None ») |
| `surface.page` | `#0a0a0a` | fond derrière les panneaux |
| `surface.panel` | `#1a1a1a` | liste, barre radio, fiche, réglages |
| `surface.raised` | `#222222` | popover, menu, bulle d'aide, chip sur un panneau |
| `surface.hover` | `#2a2a2a` | survol d'une ligne ou d'un contrôle |
| `surface.selected` | `#163020` | ligne choisie, onglet et filtre actifs |
| `surface.urgent` | `#2b1214` | ligne d'urgence (journal, liste) |
| `surface.warn` | `#2e2410` | ligne d'attention |
| `scrim` | noir à 70 % | derrière la page des réglages |

La sélection devient **un vert opaque** au lieu des neuf transparences de vert d'aujourd'hui
(`/10` à `/95`). Sa valeur ne dépend plus de ce qu'il y a derrière, donc le contraste du texte
non plus (principe 3).

### Texte : trois niveaux nommés

| Jeton | Valeur | Contraste minimal (sur `hover`) | Emploi |
|---|---|---:|---|
| `text.primary` | `#e0e0e0` | 10,9 | valeurs, indicatifs, transcriptions |
| `text.secondary` | `#b8b8b8` | 7,2 | libellés, type d'avion, valeurs secondaires |
| `text.tertiary` | `#949494` | **4,7** | unités, heures, indications ; le plancher |
| `text.disabled` | `#6b6b6b` | (2,7) | contrôles désactivés seulement, exemptés |

Les trois marches de clarté sont régulières (L 89, 75, 61). Elles **remplacent les dix
`text-text/xx`** et les `opacity-*` posés sur du texte. Correspondance pour la migration :

- `/80`, `/85`, `/90` → `primary` ;
- `/60`, `/70` → `secondary` ;
- `/25` à `/50` → `tertiary`, ou `disabled` si c'est le sens.

### Bordures

| Jeton | Valeur | Rôle |
|---|---|---|
| `border.subtle` | `#262626` | séparateurs de lignes dans un panneau |
| `border.default` | `#333333` | bords de panneau (inchangé) |
| `border.control` | `#787878` | contour d'un champ, d'une case non cochée, d'un interrupteur éteint : ≥ 3:1 partout (WCAG 1.4.11) |
| `border.focus` | `#4caf50` | anneau de focus, 2 px |

### La voix de l'outil, et les statuts

| Jeton | Valeur | Sens, et lui seul |
|---|---|---|
| `accent.default` | `#4CAF50` | **la voix de l'outil** : la marque, ce qui est vivant (fréquence écoutée, chaîne qui marche), ce qui est choisi, le focus |
| `accent.strong` | `#66c56a` | texte d'accent qui doit peser plus, sur la surface sélectionnée |
| `status.urgent` | `#ff6e6e` | urgence et panne, en texte ou en pictogramme, **jamais sans forme ni mot** |
| `status.urgent-fill` | `#b3262f` | badge d'urgence (texte blanc 6,5:1), toujours bordé de `status.urgent` |
| `status.warn` | `#f5b83d` | attention : service dégradé, appariement faible, remise des gaz, changement de piste |
| `status.info` | `#6cb6ff` | information qui n'est pas la voix de l'outil : marque d'aide, lien |

**Un seul vert.** Le `#4ADE80` de la page de connexion, des voyants et du locuteur disparaît. Le
vert ne dit plus « pilote » ni « clairance » (voir plus bas). Il reste « ok » sur un voyant,
parce qu'une chaîne qui marche est une chaîne vivante : c'est le même sens.

**Rouge et vert pour un daltonien.** Aucune teinte de rouge ne s'éloigne du vert `#4CAF50` pour
un deutéranope : à clarté égale, ils se confondent (ΔE 8,0 pour `#ff5f57`, 0,7 pour `#f2555a`).
Seule la clarté sépare. D'où un aplat d'urgence **sombre** (`#B3262F`, L 40 contre 64 pour le
vert) : ΔE 18,4 sous les quatre visions. S'y ajoute toujours une forme (■, ▲) ou un mot.

### Phases de vol : cinq familles au lieu de dix couleurs

| Famille | Codes | Valeur | Pourquoi cette couleur |
|---|---|---|---|
| `phase.ground` | TAX | `#dccb9e` sable | la terre ; clair et chaud |
| `phase.departure` | T/O, CLB, DEP | `#ec8b34` orange | ça part, ça monte ; chaud et soutenu |
| `phase.cruise` | CRZ | `#b8d3f2` ciel pâle | haut et de passage ; clair et froid |
| `phase.arrival` | ARR, APP, T/D | `#4c9ce8` bleu | ça vient vers nous ; froid et soutenu |
| `phase.unknown` | NEW, UNK | `#969696` gris | rien n'est encore su |

- **Mesure** : distance minimale entre familles sous les quatre visions **23,8**, contre **1,8**
  aujourd'hui (TAX / CRZ en deutéranopie). Tableau complet dans le fichier de vérification.
- **Pourquoi cinq** : un dichromate perçoit deux dimensions de couleur, l'axe bleu–jaune et la
  clarté. Cinq classes y tiennent (soutenu chaud, soutenu froid, clair chaud, clair froid,
  neutre), dix non. C'est aussi ce qu'un novice retient : « orange, ça part ; bleu, ça arrive ».
- **Le détail est dans le code**, toujours écrit (principe 6) : T/O, CLB et DEP sont orange tous
  les trois, et le badge dit lequel.
- **T/O et T/D**, les deux instants qui comptent, ont un **badge plein** (couleur de famille,
  texte `#0a0a0a`, de 6,7 à 12,9:1). Les autres codes ont un badge au trait : la différence est
  de forme, pas de teinte.
- **Familiarité** : départ et croisière gardent leur teinte d'aujourd'hui (orange, bleu clair).
  Le sol passe du violet au sable, les arrivées du rose et du jaune au bleu, et NEW et UNK
  deviennent un même gris.

### Altitude des avions sur la carte : une rampe de clarté, sans teinte

| Bande | Fond sombre | Fond clair (IGN, OSM) |
|---|---|---|
| très basse | `#f5f5f5` (19,3:1) | `#141414` |
| basse | `#cfcfcf` | `#333333` |
| moyenne | `#a8a8a8` | `#4d4d4d` |
| haute | `#858585` | `#666666` |
| croisière | `#6b6b6b` (3,9:1) | `#808080` (3,9:1) |

- **Pourquoi sans teinte** : l'avion porte déjà la couleur de sa phase dans son étiquette. Deux
  codes de teinte sur le même objet se brouillent. Avec une rampe de clarté, l'icône dit
  l'altitude et l'étiquette dit la phase.
- **Pourquoi « plus bas = plus clair »** : pour qui écoute depuis le sol, l'avion bas (approche,
  tour) est celui qui compte. Le survol en croisière se tait (principe 5).
- **Daltonisme** : sans teinte, la rampe est identique pour toutes les visions. L'écart minimal
  entre bandes voisines est ΔE 10,3, et chaque bande reste au-dessus de 3:1 sur son fond.
- **Ce que ça remplace** : la rampe jaune-orange-rouge d'aujourd'hui, dont le rouge « très bas »
  empiétait sur l'urgence et l'orange sur les départs.

### Les autres codes, sans teinte propre

| Information | Aujourd'hui | Proposé |
|---|---|---|
| **Locuteur** | ATC orange, PILOT vert, inconnu gris | **par la forme** : ATC = étiquette « ATC » au trait (`border.control`, `text.secondary`) ; pilote = la chip d'indicatif de l'avion ; inconnu = « ? » `text.tertiary`. Le vert et l'orange sont libérés |
| **Force des indices** | trois barres, couleur ambre / bleu ciel / vert selon la force | trois barres ; le **nombre** de barres pleines dit la force (`text.secondary`) ; seule la force faible passe en `status.warn`, car c'est une raison de douter |
| **Consigne en clair** | clairance vert d'eau, autre ambre | chip neutre (`surface.raised`, `text.primary`) ; la clairance porte **✓ et la graisse 600** ; plus de couleur |
| **Voyants** | vert, ambre, rouge, gris : couleur seule | **forme + couleur + mot** : ● ok (`accent`), ▲ attention (`warn`), ■ panne (`urgent`), ○ inconnu (`tertiary`), – éteint, ◐ bascule |
| **Tuile de fréquence** | voyant de niveau rouge pâle / vert / gris | voir le composant ; l'échec est **en clair**, plus estompé |

---

## 2. Typographie

### Deux familles, aucune police chargée

- `font.family.mono` (`ui-monospace`, soit SF Mono sur Mac) pour **ce qui se compare** :
  indicatifs, fréquences, niveaux, caps, vitesses, distances, heures, codes, squawk.
- `font.family.text` (`system-ui`, soit SF Pro sur Mac) pour **ce qui se lit** : transcriptions,
  consignes en clair, bulles d'aide, messages, titres, libellés de réglage.
- **Mesure** (principe 8) : le monospace est 1,24 à 1,32 fois plus large à taille égale. Une
  transcription passée en police système à 14 px tient dans moins de place qu'aujourd'hui à
  12 px en monospace.
- *À trancher par le propriétaire* : si le tout-monospace est gardé, l'échelle reste la même. Les
  phrases restent alors à 12 px dans les colonnes étroites (fil de 320 px).

### Cinq tailles, plancher 11 px

| Jeton | Taille / interligne | Emploi |
|---|---|---|
| `caption` | **11** / 14 | unités, libellés de champ (capitales, interlettrage 0,04 em), heures, badges, compteurs |
| `small` | 12 / 16 | données denses : cellules de tableau, seconde ligne d'une bande, fil étroit |
| `body` | 14 / 20 | donnée principale (indicatif, niveau), transcription, texte des réglages |
| `title` | 16 / 22 | titre de panneau, de section |
| `display` | 20 / 26 | titre de page, le grand indicatif de la fiche |

**Graisses : 400 et 600.** Le gras dit « ceci est la valeur » ou « ceci est une clairance »,
jamais l'importance d'un titre : la taille s'en charge.

**Correspondance avec aujourd'hui** (quinze tailles) :

| Aujourd'hui | Devient |
|---|---|
| 7, 8, 9, 9,6, 10 px | **11** (`caption`) ; ce qui n'y tient pas se replie ou passe dans une bulle |
| 11 px | 11 |
| 12, 13 px | 12, ou 14 pour une phrase en police système |
| 14 px | 14 |
| 15, 16 px | 16 |
| 18, 20 px | 20 |
| 36 et 60 px (« Connection Lost ») | 20, dans un bandeau (voir « Message d'erreur ») |

### Règles

- **Unités** : après la valeur, séparées par une espace fine. En `caption` et `text.tertiary`,
  alignées sur la ligne de base. Jamais sous 11 px. Exemples : « FL 120 », « 1 800 ft »,
  « 248 kt », « 119.255 ».
- **Libellés de champ** : `caption`, capitales, `text.secondary`, interlettrage 0,04 em. Plus
  de 7 px à 0,2 em.
- **Chiffres** : alignés par la chasse fixe en monospace ; `tabular-nums` quand un nombre est en
  police système (compteur dans une phrase).
- **Raccourcis clavier** : ils ne s'écrivent plus en 7 px dans le bouton. Ils vont dans sa bulle
  (« Raccourci : 3 »). Ils apparaissent aussi en `caption` quand la touche ⌥ est tenue, si la
  migration le juge utile.

---

## 3. Espace, rayons, bordures, élévation

**Espacement** (`space.*`) : 2, 4, 6, 8, 12, 16, 24, 32 px.
- Les quatre premiers couvrent déjà 77 % des emplois. On garde le demi-pas de 2 px jusqu'à 8,
  parce que la densité de l'écran de contrôle en a besoin (40 % d'emplois en demi-pas aujourd'hui).
- Au-delà de 8, on avance par 4 px.
- Les 25 valeurs en dur de `style.css` s'y ramènent.

**Rayons** (`radius.*`) :
- 2 px : badges, barres ;
- **4 px : le défaut** (77 emplois aujourd'hui, écrit de deux façons) ;
- 8 px : ce qui flotte sur la carte ;
- plein : voyants, compteurs, curseurs.

**Bordures** : 1 px partout, 2 px pour le focus, l'avion sélectionné et le soulignement d'onglet.

**Élévation, sans ombre** : sur fond noir une ombre ne se voit pas (inventaire § 4). Le niveau
se dit par la surface et la bordure.

| Niveau | Quoi | Surface | Bordure | Ombre |
|---|---|---|---|---|
| 0 | carte, PAR | `map` | — | — |
| 1 | panneau (liste, barre radio, fiche) | `panel` | `default` | — |
| 2 | ce qui flotte : popover, menu, bulle, fil de fréquence | `raised` | `default` | `overlay`, seulement au-dessus de la carte (le fond peut y être clair) |
| 3 | page : réglages | `page` sur `scrim` | — | — |

**Plans** (`z.*`) : sept étages au lieu de quinze.
- 10 superpositions de carte, 20 panneaux, 30 barre radio ;
- 40 popovers, 50 page, 60 notifications ;
- 70 connexion.

---

## 4. Composants

Pour chacun : ce qui le compose, ses états, ses variantes. Les états communs valent partout et
ne sont pas répétés :
- **survol** : `surface.hover` (et texte `primary` si le repos était `secondary`), 120 ms ;
- **focus clavier** : anneau `border.focus` 2 px, décalé de 2 px, sur `:focus-visible` seulement ;
- **désactivé** : `text.disabled`, contour `border.subtle`, pas de survol, curseur normal ;
- **chargement** : le libellé reste, un point de suspension animé ou une barre indéterminée ;
  jamais un composant vide.

### Boutons

| Variante | Repos | Actif / pressé | Emploi |
|---|---|---|---|
| **Principal** | aplat `accent.fill`, texte `on-accent` 600 | aplat assombri de 8 % | une action par écran au plus : se connecter, enregistrer |
| **Secondaire** | contour `border.control`, texte `primary` | `surface.hover` | annuler, réessayer, tout écouter |
| **Discret** | sans contour, texte `secondary` | texte `primary` | actions de panneau (fermer, rafraîchir) |
| **Icône** | 24 × 24 au bureau, 44 × 44 au téléphone, pictogramme 14 px `secondary` | `primary` | roue dentée, cloche, fermer, haut-parleur |

Hauteur 24 px au bureau (`control-desk`), 44 au téléphone. Texte `small`, en police `text`.
Le bouton « fermer » existe sur **toute** fiche ou page : l'audit du 09/10 en notait l'absence.

### Bouton-filtre (bascule)

C'est le filtre de phase, Air / Sol, « Heard on the radio », les préréglages d'altitude.
- **Éteint** : contour `border.control`, texte `secondary`.
- **Allumé** : `surface.selected`, contour `accent`, texte `accent.strong`.
- Une seule façon de dire « allumé » (aujourd'hui trois : fond vert transparent, ombre intérieure,
  bordure verte).
- Le filtre de phase porte **la pastille de sa famille** (8 px, plein) devant son code : la
  couleur n'est pas seule.

*À voir à la migration* : dix filtres de phase ou cinq de famille. Le système permet les deux ;
cinq réduiraient les dix-neuf contrôles qui précèdent la première ligne de la liste.

### Case à cocher, interrupteur

- Case de 14 px, contour `border.control`. Cochée : aplat `accent`, coche `on-accent`.
- Le libellé est à droite, cliquable, en `body` police `text` dans les réglages.
- Interrupteur 28 × 16 au bureau, 44 × 24 au téléphone. Éteint : rail `border.control` ; allumé :
  rail `accent`, pastille `on-accent`.

### Onglets et contrôle segmenté

- **Onglets** (fiche : Timeline, Details, Tracks ; liste : Active, Signal lost) : texte `small`
  `secondary`. L'onglet courant est en `primary`, souligné de 2 px `accent`. Pas de fond.
- **Contrôle segmenté** (Radar · LFPZ · LFPG · LFPO ; Map · PAR · List au téléphone) : un rail
  `surface.panel` à contour `border.default`. Le segment courant prend `surface.selected` et le
  texte `accent.strong`.
- « PAR » n'est plus une étiquette qui ressemble à un bouton (inventaire : « PAR » rendu à 3,2:1,
  et pris pour un bouton pendant la mesure). Le groupe s'intitule « PAR » en `caption`
  au-dessus, ou l'aérodrome porte l'icône de la piste.

### Tuile de fréquence

```
┌──────────────────────────────┐
│ 🔊  Approche De Gaulle    ⋯ 3 │  nom (small, police text, primary), menu, non lus (compteur)
│     124.350          12 s ▮▮▯ │  MHz (small mono, secondary), dernière émission (caption), niveau
│ ───────○──────                │  volume (curseur)
└──────────────────────────────┘
```

| État | Rendu |
|---|---|
| Coupée | contour `border.default`, haut-parleur barré `secondary`, barre de niveau `border.control` |
| Écoutée | contour `accent`, haut-parleur `accent`, barre de niveau `accent` |
| Émission en cours | la barre de niveau bouge (c'est l'information) ; la tuile ne clignote pas |
| Connexion | « connecting… » en `caption` `warn`, haut-parleur inactif |
| **Échec** | contour `status.urgent`, ■ et « failed — retry » en `caption` `urgent`, **pleine opacité** : une panne ne s'estompe pas |
| Non lus | compteur `accent` (aplat, texte `on-accent`) ; il passe en `urgent-fill` s'il contient une urgence |

Largeur 160 px au bureau (comme depuis le 11/10), pleine largeur au téléphone (une tuile par
ligne dans la barre défilable). Le nom ne répète pas la fréquence.

### Bande d'avion (ligne de la liste)

```
▌ AFR1234   FL 120 ↓   270°   248 kt   12 NM     ◉ 14 s
▌ A320  APP  ── ft/min −1 200                     radio
```

- **Repère de phase** : un trait de 3 px à gauche, couleur de la famille, plus le **badge de
  code** en `caption` mono, plein pour T/O et T/D.
- **Indicatif** : `body` mono, `primary`, 600. Type d'avion : `caption`, `secondary`.
- **Valeurs** : `body` mono, `primary`. Unités en `caption` `tertiary`. Tendance ↑ ↓ ↔ en
  `secondary`.
- **« Vient de parler »** : un pictogramme radio `accent` et l'âge en `caption`. Il disparaît
  après 60 s. Pas de clignotement.

| État | Rendu |
|---|---|
| Repos | `surface.panel`, séparateur `border.subtle` |
| Survol | `surface.hover` ; l'avion est mis en évidence sur la carte (lien du principe 7) |
| **Choisi** | `surface.selected` ; trait gauche de 2 px **`accent`** à côté du repère de phase ; même anneau `accent` sur l'icône de la carte et le chevron de la PAR |
| Signal perdu | valeurs en `tertiary`, mot « lost » en `caption` ; pas d'opacité |
| Urgence (squawk 7500/7600/7700) | `surface.urgent`, badge `urgent-fill` « 7700 » à droite, bordé de `urgent` |

Hauteur 44 px au bureau (`row-desk`, deux lignes ; 50 aujourd'hui). Au téléphone, 56 px
(`row-touch`), toute la ligne est la cible. La colonne du cap et de la TAS se cache au téléphone,
comme depuis le 11/10.

### Fiche avion

- **En-tête** :
  - l'indicatif en `display` mono ;
  - le badge de phase, le type et la compagnie en `small` police `text` ;
  - le bouton fermer (24 / 44) ;
  - les trois valeurs clés (niveau, cap, vitesse) en `body` mono.
- **Onglets** (Timeline, Details, Tracks). *Proximity disparaît* si l'audit 1.1 se confirme.
- **Détails** : grille libellé / valeur. Libellé en `caption` capitales `secondary`, valeur en
  `body` mono `primary`. Une valeur absente s'écrit « — » en `tertiary`, une fois, sans les
  soixante tirets mesurés à 4,2:1.
- **Timeline** : une ligne par événement.
  - L'heure en `caption` mono `tertiary`, le pictogramme de catégorie en 14 px `secondary`, le
    texte en `small` police `text` `primary`.
  - La transmission appariée montre la chip d'indicatif, la transcription et la chip de consigne
    en clair.
  - Le texte d'explication passe dans une bulle d'aide sur le titre (aujourd'hui 84 caractères à
    10 px et 3,2:1).

### Chips

| Chip | Rendu | États |
|---|---|---|
| **Indicatif** | `small` mono `primary`, `surface.raised`, contour `border.default`, rayon 4, hauteur 20 (bureau) | survol : contour `border.control` ; hors portée : texte `tertiary`, contour pointillé, non cliquable ; choisi : contour `accent` |
| **Valeur** | « FL 100 ↓ », « 270° » : `small` mono, sans fond, unité `tertiary` | — |
| **Consigne en clair** | `small` police `text`, `surface.raised`, contour `border.subtle` | clairance : ✓ devant, graisse 600 |
| **Badge de phase** | code `caption` mono 600, couleur de famille, contour de la couleur de famille, fond `surface.raised` ; T/O et T/D en aplat, texte `on-warn` | — |
| **Locuteur ATC** | « ATC » `caption` capitales, contour `border.control`, texte `secondary` | — |

### Voyant

Forme de 8 px, puis le mot en `caption` `secondary` : ● ok · ▲ attention · ■ panne · ○ inconnu ·
– éteint · ◐ bascule. La couleur suit le statut. **La forme et le mot sont obligatoires**, la
couleur s'y ajoute. Le bandeau d'état (antenne, transcription, ADS-B) en est fait.

### Curseurs

- **Volume** :
  - rail de 2 px `border.control`, partie remplie `accent` ;
  - pastille de 12 px au bureau, 20 au téléphone ;
  - la zone de clic fait 24 px de haut, même si le dessin est fin (aujourd'hui 10 px).
- **Altitude, à deux poignées** : même rail, l'intervalle choisi en `accent`.
  - Histogramme du trafic en `surface.hover` hors intervalle, `accent` à 40 % dedans.
  - Repères (SFC, TA, FL100…) en `caption` `tertiary`.
  - Au clavier, ← et → déplacent la poignée qui a le focus.

### Tableau

En-têtes en `caption` capitales `secondary` ; cellules en `small` mono ; nombres alignés à
droite. Séparateurs `border.subtle`, survol de ligne `surface.hover`. Pas de zébrure.

### Popover, panneau, page

- **Popover** (fil de fréquence, menus de la carte, METAR) :
  - `surface.raised`, contour `border.default`, rayon 8, ombre `overlay`, marge 12 ;
  - titre en `small` 600, bouton fermer discret.
  - Le fil de transcription fait 320 px de large au bureau ; au téléphone, il occupe le volet
    entier.
- **Panneau** (liste, fiche) : `surface.panel`, en-tête de 40 px (titre `title`, actions
  discrètes), contenu défilant.
- **Page** (réglages) : niveau 3, une colonne de 640 px, navigation à gauche au bureau et en haut
  au téléphone. Titre en `display`, sections en `title`. Fermeture par Échap ou ✕.

### Bulle d'aide

- **Déclencheur** : un soulignement pointillé `tertiary` sous le mot, en plus du curseur d'aide.
  Il dit qu'il y a une explication sans ajouter de pictogramme.
- **Ouverture** : survol de 400 ms ou focus clavier au bureau ; appui long de 500 ms au
  téléphone.
- **Corps** :
  - `surface.raised`, contour `border.default`, ombre `overlay`, rayon 4 ;
  - largeur maximale 280 px ;
  - titre en `caption` capitales `secondary`, texte en `small` **police `text`** `primary`.
- Ce composant porte les **591 éléments** d'aide de l'écran principal ; sa lisibilité compte plus
  que celle de n'importe quel autre.

### Message vide

- Pictogramme 16 px `tertiary`, puis une phrase en `small` police `text` `secondary`.
- La phrase dit **ce qui arrivera et comment le provoquer** : « No transcription yet on 124.350.
  Lines will appear here as soon as someone speaks. »
- Pas de gris à 4,2:1 : le message du journal vide (148 caractères) en était le plus gros cas.

### Message d'erreur

- **En ligne** : ■ `urgent`, puis la phrase en `small` `primary`, puis l'action (« retry »,
  bouton secondaire).
- **Bandeau** (connexion perdue, chaîne station tombée) :
  - en haut de la carte, `surface.urgent`, trait gauche de 2 px `urgent` ;
  - le texte en `body` dit quoi et depuis quand : « Connection lost — reconnecting (attempt 3),
    next in 5 s ».
  - Il **remplace l'écran « CONNECTION LOST »** en 36 px rouge clignotant, qui masquait la carte
    restée utilisable.

### Compteur

Pastille pleine, `caption` mono 600, minimum 16 px. Aplat `accent` et texte `on-accent` pour des
non-lus ordinaires. Aplat `urgent-fill` bordé de `urgent` dès qu'un élément urgent est compté.
Aujourd'hui, la cloche affiche ses non-lus en rouge quel qu'en soit le contenu.

---

## 5. Mouvement

| Ce qui bouge | Comment |
|---|---|
| Les avions | interpolation existante (moteur d'animation, 30 i/s) : c'est l'information |
| La barre de niveau d'une fréquence | suit le son |
| Une urgence | trois pulsations de 1 s sur son badge et sa ligne, puis fixe ; son au choix (journal) |
| Un changement d'état (survol, bascule) | 120 ms, `standard` |
| L'ouverture d'un popover ou d'un panneau | 200 ms, opacité et 4 px de glissement |

**Ne bougent pas** : une nouvelle ligne de la liste (elle apparaît), un compteur qui change, une
transmission qui arrive dans le fil, la page des réglages (sans glissement), et rien en boucle
hors urgence.

**`prefers-reduced-motion`** :
- les durées tombent à 0 et l'urgence ne pulse pas (le badge reste) ;
- les avions sont toujours redessinés à chaque position, l'interpolation entre deux positions
  étant gardée. *À confirmer par le propriétaire* : c'est de l'information, pas de la décoration.

## 6. Densité : deux niveaux

| | Bureau (≥ 768 px) | Téléphone (< 768 px) |
|---|---|---|
| Cible minimale | 24 px | **44 px** |
| Hauteur de contrôle | 24 px | 44 px |
| Ligne d'avion | 44 px | 56 px, toute la ligne cliquable |
| Disposition | carte + liste + barre radio, tout visible | un volet à la fois (Map · PAR · List), barre radio défilable |
| Survol | oui (aides, mise en évidence) | remplacé par l'appui long |
| Tailles de texte | les cinq | les cinq, mêmes jetons |

Pas de troisième niveau. Le seuil de 768 px est celui des classes `md:` déjà en place.

## 7. Accessibilité

- **Contrastes** : garantis par `tokens.mjs` pour chaque paire prévue. Tout texte ≥ 4,5:1, tout
  contour de contrôle ≥ 3:1. La seule exception est le texte désactivé, exempté.
- **Couleur jamais seule** : formes des voyants, codes de phase écrits, mots d'état.
- **Focus clavier** : anneau `accent` 2 px sur `:focus-visible`. Les deux `focus:outline-none`
  disparaissent.
- **Tab** rend la main au navigateur pour aller d'un contrôle au suivant. Le passage d'un avion
  au suivant, aujourd'hui sur Tab (`app.js:791`), passe sur **J / K**, ou sur **↓ / ↑** quand la
  liste a le focus. *Changement d'habitude : à valider par le propriétaire.* Les autres raccourcis
  (0-9, A, G, Espace) restent.
- **Sémantique** : ce qui se clique est un `<button>` (pas un `<div>` avec `@click`). Les onglets
  ont `role="tab"`, la liste `role="listbox"` ou `list`. Les icônes seules ont un `aria-label`.
  La langue des transcriptions françaises est marquée `lang="fr"` : la synthèse vocale les lit
  alors correctement.
- **Cibles** : 24 px au bureau, 44 au téléphone (aujourd'hui 35 sur 41 sous 44 px au téléphone).
- **Mouvement** : `prefers-reduced-motion` respecté (§ 5).

## 8. La marque, en attente du nom

Quand le nom sera choisi (`00-nom.md`), la marque s'écrira avec ces jetons :
- le nom en `font.family.mono`, 600, `text.primary`, avec un seul signe en `accent` ;
- un pictogramme de 16 px dessiné sur une grille de 16 avec des traits de 2 px, lisible en
  `accent` sur `surface.page` et en monochrome dans la barre de menus ;
- deux états pour la barre de menus (production en marche ou arrêtée), qui diffèrent par la forme
  et pas seulement par la couleur (principe 6) ;
- une page de connexion faite des mêmes surfaces et du même vert que l'outil : la palette propre
  de `login.js` (7 couleurs) disparaît.

## Ce qui reste à trancher par le propriétaire

1. Le monospace pour les phrases ou la police du système (principe 8).
2. Les cinq familles de phase à la place des dix couleurs.
3. La rampe de clarté sans teinte pour l'altitude des avions.
4. J / K au lieu de Tab pour passer d'un avion à l'autre.
5. L'interpolation des avions gardée sous `prefers-reduced-motion`.
