# Plan de migration — appliquer le système à l'interface vivante

*Quatrième livrable de la mission design (`MISSION.md`, §6), 12/10/2026. Il commence après la
validation du propriétaire. Il se fait par la session atc-scribe, sous les sept garde-fous de
`32-audit-produit.md`. Chaque étape se livre seule, se vérifie seule et se défait par un
`git revert`.*

## Avant tout : où vit `www/`

**Mesuré le 12/10.** `co-atc` sert `www/` depuis le disque, à chaque requête
(`internal/api/static.go`, chemin relatif `www`). `runs/production.command` et
`runs/son-test/son-test.command` font tous deux `cd /Users/pierre/Dev/Aero/atc-scribe` : **la
production et l'instance d'essai lisent le même dossier `www/`**.

Conséquences :
- un fichier de `www/` modifié dans le dépôt principal est en production au rechargement suivant
  de la page, sans recompiler ;
- et l'instance d'essai ne l'isole pas.

**Proposition (étape 0)** : pour la durée de la migration, l'instance d'essai se lance depuis un
arbre de travail à part (`git worktree`, branche de l'étape en cours). Un `cd` différent dans
`son-test.command`, ou une variante `son-test-design.command`, suffit. Ses données restent sous
`runs/son-test/`.

- Le garde-fou 3 (« l'instance d'essai d'abord ») redevient vrai.
- Le garde-fou 1 se simplifie : défaire une étape, c'est un `git revert` sur `local`. Le binaire
  ne change pas, car aucune étape ne touche au Go.

## Les mesures qui jugent chaque étape

| Mesure | Outil | Ce qu'elle tranche |
|---|---|---|
| Filet | `node tools/www-check/check.mjs` | aucune référence du store cassée, scripts valides, tests PAR ; classes Tailwind présentes dans le CSS compilé (empreinte) |
| Jetons | `node tools/design/tokens.mjs` | aucune exigence de contraste ou de daltonisme en recul |
| Rendu | `tools/design/contrast-probe.js` dans la page, à 1 440 × 900 et 375 × 812, sur les six écrans de l'inventaire | part des caractères sous 11 px, part sous AA, cibles sous 24 / 44 px. Points de départ : `01-inventaire.md` § 1 et § 3 |
| Identité de rendu | comparaison des styles calculés, comme au 4.3 (280 combinaisons balise + classes) | pour une étape « à comportement identique » : zéro écart |
| Inventaire | `node tools/design/inventory.mjs` | tailles, couleurs en dur, états : le compte doit baisser, jamais remonter |
| Usage | une soirée d'écoute du propriétaire (garde-fou 5) | ce qu'aucun chiffre ne dit |

Chaque étape **donne les chiffres avant / après** dans son message de commit et dans le doc 32.

## Les étapes

Effort en jours de travail de session (j), vérification comprise. Les étapes 1 à 3 ne demandent
aucune décision. Les étapes marquées **(décision)** attendent un choix listé à la fin de
`03-systeme.md`.

### 0. Préparer : instance d'essai isolée, sonde rangée — 0,5 j

- **Change** : `son-test` lancée depuis un arbre de travail. La sonde de rendu devient un script
  rejouable par l'outil de navigateur, sans copier-coller : un signet, ou une page de
  `tools/design/` qui l'injecte.
- **Ne change pas** : rien à l'écran, rien en production.
- **Vérifier** : la page de l'instance d'essai sert bien les fichiers de l'arbre de travail (un
  commentaire témoin dans `index.html`).

### 1. Les jetons dans le code, à comportement identique — 1 j

- **Change** :
  - `www/tokens.css` (produit par `tokens.mjs`, chargé avant `style.css`) ;
  - le `theme.extend` de `tools/tailwind/tailwind.config.js` reçoit les jetons comme couleurs
    (`surface-panel`, `text-secondary`…), tailles, espacements et rayons, qui pointent vers les
    variables CSS ;
  - les huit noms d'aujourd'hui (`background`, `panel`, `text`, `highlight`, `warning`, `danger`,
    `border`, `hover`) **restent**, rebranchés sur les mêmes valeurs.
- **Ne change pas** : rien à l'écran. Toutes les valeurs reprises sont celles d'aujourd'hui.
- **Vérifier** :
  - styles calculés identiques sur les 280 combinaisons ;
  - `www-check` vert, `build.sh` relancé ;
  - capture avant / après identique au pixel près, à l'œil.
- **Risque** : nul. L'ordre des feuilles du 11/10 est à garder (Tailwind après `style.css`).

### 2. Le texte : trois niveaux nommés, plancher 11 px — 2 j

- **Change** :
  - les 134 `text-text/xx` deviennent `text-primary`, `text-secondary` ou `text-tertiary`
    (correspondance au § 1 de `03-systeme.md`) ;
  - les `opacity-*` sur du texte disparaissent ;
  - `text-[7px]` à `text-[10px]` (108 emplois) et les tailles de `style.css` deviennent les cinq
    tailles ;
  - les chiffres-raccourcis des filtres (7 px) passent dans les bulles d'aide.
- **Ce qui se voit** :
  - unités, types d'avion et badges plus grands ;
  - lignes de la liste un peu plus hautes (de 50 vers 44 px annoncés, à vérifier : la place gagnée
    sur les tailles est reprise par la police) ;
  - en-têtes de la barre du bas lisibles.
- **Ne change pas** : couleurs, disposition, comportements.
- **Vérifier** :
  - sonde : moins de 11 px de **45 % à 0 %** à l'écran principal, sous AA de **18 % à moins de
    1 %** (le reste vient des couleurs, étape 3) ;
  - aucune colonne de la liste ne déborde à 480 px avec 7 fréquences et 150 avions.
- **Risque** : faible ; c'est le premier changement visible. Une soirée d'écoute avant l'étape 3.

### 3. Couleurs : un vert, sélection opaque, statuts avec formes — 1,5 j

- **Change** :
  - `#4ADE80` remplacé par l'accent partout, y compris `login.js` (dont la palette propre
    disparaît) ;
  - les dix-sept transparences de vert deviennent `surface-selected` ;
  - voyants à formes (● ▲ ■ ○) ;
  - tuile en échec en clair au lieu d'estompée ;
  - compteur de la cloche vert, rouge seulement s'il compte une urgence ;
  - aplat d'urgence bordé ;
  - locuteur par la forme (plus d'orange ni de vert) ;
  - force des indices en nombre de barres ;
  - consigne en clair neutre avec ✓ pour la clairance.
  - Les couleurs en dur de `style.css` passent aux variables.
- **Ne change pas** : les couleurs des phases et des altitudes (étapes 4 et 5), la disposition.
- **Vérifier** :
  - sonde : sous AA à 0 % sur les six écrans ;
  - `inventory.mjs` : couleurs distinctes de 215 vers moins de 60, valeurs en dur de 284 vers
    celles de la carte seulement ;
  - essai d'une urgence simulée (squawk 7700 injecté sur l'instance d'essai) : ligne, journal et
    compteur.
- **Risque** : faible. L'habitude change sur le locuteur (orange / vert) : le dire au propriétaire
  avant.

### 4. Phases : cinq familles — 1 j *(décision)*

- **Change** : les dix couleurs deviennent cinq, aux trois endroits qui les portent ensemble :
  - `store/sidebar.js` (`getPhaseColorClass`) ;
  - `par/par-view.js` (`PHASE_COLORS`) ;
  - `map/renderers/aircraft-webgl.js` (`phaseColorMap`).
  - S'y ajoutent les badges de la liste, de la fiche et du journal, et les badges pleins pour T/O
    et T/D.
- **Ne change pas** :
  - **les dix filtres de phase et leurs raccourcis 1 à 0**, ni la clé `phaseFilters` du
    `localStorage` (garde-fou 6) ;
  - passer à cinq filtres de famille est une seconde décision, qui demanderait de traduire la clé
    enregistrée.
- **Vérifier** :
  - `tokens.mjs` (distance minimale entre familles 23,8) ;
  - les trois vues côte à côte sur l'instance d'essai, le même avion repéré dans chacune ;
  - tests PAR.

### 5. Altitude des avions : rampe de clarté — 0,5 j *(décision)*

- **Change** : `getAltitudeBandColor` (`aircraft-webgl.js`) rend les cinq valeurs de
  `color.altitude.dark` ou `.light`. Le correctif du 12/10 (`46dc425`, fonds clairs nommés) est
  gardé tel quel.
- **Ne change pas** : les bandes elles-mêmes (seuils), les étiquettes.
- **Vérifier** :
  - capture de la carte à 1 440 avec 100 avions, fond noir puis IGN ;
  - les icônes de croisière restent au-dessus de 3:1 sur le noir ;
  - compteurs d'animation inchangés (garde-fou 4 : le rendu WebGL est un chemin qui compte).

### 6. Deux familles de police — 0,5 j *(décision)*

- **Change** : `font-text` (police du système) pour les transcriptions, les consignes en clair,
  les bulles d'aide, les messages et les textes des réglages. Le reste en monospace.
- **Vérifier** :
  - la largeur des lignes du fil de 320 px : plus de mots par ligne à taille égale (1,26 fois, mesuré) ;
  - la sonde.

### 7. Composants, un par un — 3 j

Un commit par composant, dans cet ordre, du plus vu au moins vu. Chaque composant suit
`03-systeme.md` § 4 et se vérifie sur sa maquette (`maquettes/`, ouverte à côté de l'instance
d'essai) :

1. bande d'avion ;
2. tuile de fréquence ;
3. fil de fréquence ;
4. fiche et Timeline ;
5. chips ;
6. bulle d'aide (déclencheur pointillé, police du système) ;
7. contrôle segmenté Radar / PAR (« PAR » cesse de ressembler à un bouton) ;
8. onglets ;
9. messages vides ;
10. bandeau de connexion perdue, qui remplace l'écran « CONNECTION LOST » de 36 px.

À chaque commit, le composant est **remplacé**, pas doublé (garde-fou 7 : retirer avant de
déplacer), et ses règles de `style.css` partent avec lui.

### 8. Focus, clavier, mouvement — 1 j *(décision pour Tab)*

- **Change** :
  - anneau de focus sur `:focus-visible` partout, et retrait des deux `focus:outline-none` ;
  - les `<div @click>` deviennent des `<button>` ;
  - `aria-label` sur les boutons-icônes ;
  - `lang="fr"` sur les transcriptions françaises ;
  - `prefers-reduced-motion` ;
  - Tab rendu au navigateur, et passage d'un avion au suivant sur J / K.
- **Ne change pas** : les raccourcis 0-9, A, G, Espace.
- **Vérifier** : un parcours entier au clavier (ouvrir un fil, écouter, choisir un avion, ouvrir
  les réglages, fermer) sans souris ; macOS « Réduire les animations » coché.

### 9. Téléphone — 1 j

- **Change** :
  - cibles de 44 px (onglets, tuiles, boutons de fiche) ;
  - ligne d'avion de 56 px, toute la ligne cliquable ;
  - filtres sur une ligne défilable.
- **Vérifier** :
  - sonde à 375 px : cibles sous 44 px de 35 sur 41 à 0 ;
  - essai réel sur l'iPhone par Tailscale, une soirée.

### 10. Identité — 1 j, après le choix du nom

- **Change** :
  - `<title>` ;
  - favicon SVG, manifeste d'application et `theme-color` (le téléphone peut alors l'ajouter à
    l'écran d'accueil) ;
  - la marque sur la page de connexion ;
  - l'icône de la barre de menus à deux états (`tools/menubar/`, symboles SF remplacés) ;
  - les occurrences de « Co-ATC » dans l'interface (3 dans `index.html`, 12 dans les scripts) ;
  - la présentation du README (`00-nom.md` § 2).
- Le renommage du dépôt et des dossiers est un chantier à part (D74).

## Effort total

| Étapes | Effort |
|---|---:|
| 0 à 3 (sans décision) | 5 j |
| 4 à 6 (après les choix) | 2 j |
| 7 (composants) | 3 j |
| 8 et 9 | 2 j |
| 10 (après le nom) | 1 j |
| **Total** | **≈ 13 j**, étalés : une soirée d'écoute du propriétaire après chaque étape visible (2 à 9) |

**Ce que l'estimation ne couvre pas** : un retour du propriétaire qui demande de reprendre un
composant après usage. C'est probable sur la liste et la tuile, les deux composants qu'il regarde
le plus. Compter une demi-journée de reprise par étape visible serait prudent, soit ≈ 17 j au
pire.

## Ce qui reste hors de ce plan

- La refonte de la disposition générale : le propriétaire l'a voulue gardée (doc 32) ; le système
  s'y applique sans la changer.
- Les traductions (en / fr) : le système prévoit `lang` et des libellés courts, la version
  française de l'interface est une décision à part (10/10).
- Le fond de carte : les jetons couvrent les avions et les étiquettes ; les fonds IGN, OSM et
  « None » restent ceux de la session atc-scribe (0.2 du doc 32).
