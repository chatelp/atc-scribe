# L'identité : Clairance

*13/10/2026, session design. Le nom est décidé par le propriétaire le 13/10 (D74, `00-nom.md` § 4).
Ce document dit comment la marque est construite et comment on s'en sert. Les fichiers sont dans
`identite/` ; la planche de présentation est `identite/index.html` (à ouvrir en servant
`docs-fr/design/`, comme les maquettes).*

## Le nom

**Clairance** : l'autorisation que donne le contrôleur, dans l'orthographe du contrôle français.
Il porte « clair » : l'outil rend la clairance claire. Il se dit /klɛ.ʁɑ̃s/ en français et se lit
*clearance* en anglais. Il s'écrit avec une capitale initiale et jamais en capitales ; le dépôt et
les fichiers s'écrivent `clairance`.

## Le signe

Une **piste** et la **coche « autorisé »** dont le sommet touche la piste : l'avion arrive de la
gauche, se pose, et l'ensemble se lit « cleared ». C'est le même signe que la chip « ✓ Cleared ILS
26L » du fil de transcription : la marque et le composant disent la même chose.

Construction, sur une grille de 64 :
- la piste : un rectangle de 48 × 8 à (8, 48), rayon 4 ; son axe, trois tirets de 2 de haut
  évidés dans la couleur du fond (ils disparaissent sous 32 px) ;
- la coche : la ligne brisée (16, 32) → (28, 44) → (52, 20), trait de 8, bouts et jointure ronds ;
  son sommet (28, 44) plus la moitié du trait touche le haut de la piste (48) ;
- la grille de 16 est la même divisée par quatre, sans axe : piste 12 × 2 à (2, 12), coche
  (4, 8) → (7, 11) → (13, 5), trait de 2. **Mesuré** : rien ne se touche ni ne se bouche à 16 px.

Couleurs : la piste en `text.primary` (`#e0e0e0`), la coche en `accent` (`#4caf50`). Sur fond
clair : piste `#1a1a1a`, coche `#2e7d32` (5,1:1 sur blanc). En une couleur (barre de menus,
gravure) : tout dans la même teinte.

## La marque

Le signe à 40 px, puis le nom en monospace, graisse 600, 26 px, séparés de 10 px. **Le nom n'est
pas dessiné** : `marque.svg` le type dans la police monospace de la machine, comme l'interface
(principe 8). Il change donc légèrement d'une machine à l'autre, et c'est voulu : aucune police
n'est livrée, et la marque est ce que l'on voit dans l'en-tête de l'outil.

Espace de protection : la moitié de la hauteur du signe, tout autour. Taille minimale de la
marque complète : 24 px de signe ; en dessous, le signe seul.

## Les fichiers

| Fichier | Usage |
|---|---|
| `signe.svg` | le signe, 64, fond sombre ; source de toutes les tailles |
| `signe-clair.svg` | fond clair (README en thème clair, papier) |
| `signe-16.svg` | la grille de 16, pour l'en-tête et les cas où 16 px suffisent |
| `favicon.svg` | tuile `#1a1a1a` à coins de 7, le signe au centre ; à exporter en 16, 32, 180 (iOS) et 512 (manifeste) |
| `marque.svg`, `marque-clair.svg` | signe et nom |
| `barre-menus-marche.svg`, `barre-menus-arret.svg` | images « template » de 18 pt pour `tools/menubar/` |
| `index.html` | la planche : tailles, onglet, barre de menus, connexion, bandeau, README, interdits |

Export en PNG, quand il faudra : `rsvg-convert -w 512 signe.svg > signe-512.png`, ou un navigateur.

## La barre de menus

Deux états, qui diffèrent **par la forme** et pas seulement par la teinte (principe 6) :
- **en marche** : la piste et la coche, pleines ;
- **à l'arrêt** : la piste et la **ligne d'attente avant piste** (*hold short*), le signe derrière
  lequel un avion attend d'être autorisé ; l'ensemble à 55 % d'opacité.

Ce sont des images « template » (noir et transparence) : macOS les teinte pour la barre claire,
la barre sombre et l'état pressé. Elles remplacent `dot.radiowaves.left.and.right` et
`antenna.radiowaves.left.and.right.slash` dans `tools/menubar/AtcScribe.swift`. Un troisième
état « dégradé » (sidecar tombé) pourra ajouter le triangle des voyants ; non dessiné.

## Où elle apparaît

- **L'en-tête de l'outil** : le signe à 20 px et « Clairance » en monospace 600, à gauche de la
  barre d'état (esquisses A, B, C). Au téléphone, le signe seul.
- **L'onglet** : favicon et `<title>Clairance</title>`, plus un manifeste d'application avec
  `theme-color` `#0a0a0a`, pour que le téléphone l'ajoute à son écran d'accueil.
- **La connexion** : la marque en tête de la carte, puis « Sign in » et la promesse en une
  phrase (`maquettes/connexion.html`).
- **Le bandeau de reprise** : le signe tient lieu de voyant devant « click anywhere to resume
  listening ».
- **Le README** : `<picture>` qui sert `signe.svg` ou `signe-clair.svg` selon le thème, le nom en
  titre, et la première phrase qui porte les trois messages (`00-nom.md` § 1) : *Hear the airband
  you receive, read it in English and French, and see which aircraft is speaking. Runs on your
  own machine: nothing leaves it. Made for the listener; it judges no one.*
- **Le texte** : « Clairance » dans une phrase, sans article (« Clairance reads… », « ouvrir
  Clairance »).

## Ce qu'on ne fait pas

- la coche sans la piste : elle devient une coche quelconque ;
- une autre couleur que le vert de l'outil, un dégradé, une ombre, un relief, une rotation ;
- le nom en capitales, dans une autre police, ou avec le signe à droite ;
- un avion, une antenne, des ondes autour du signe : il est complet.

## Ce qui reste

- Le renommage lui-même (dépôt `chatelp/clairance`, lanceur, dossier de configuration, titre de
  page, documents) : par la session atc-scribe, en une fois (D74).
- Les domaines `clairance.org` et `clairance.app`, libres le 13/10, si le propriétaire les veut ;
  rien n'est réservé.
- La lecture du brevet US12277862B1 (Q53) avant toute diffusion hors de France.
