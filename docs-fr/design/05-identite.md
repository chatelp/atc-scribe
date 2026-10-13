# L'identité : Clairance

*13/10/2026, session design. Le nom est décidé par le propriétaire le 13/10 (D74, `00-nom.md` § 4).
Ce document dit comment la marque est construite et comment on s'en sert. Les fichiers sont dans
`identite/` ; la planche de présentation est `identite/index.html` (à ouvrir en servant
`docs-fr/design/`, comme les maquettes). Première version avec une piste et une coche, le 13/10
au matin ; le propriétaire ne reconnaissait pas la piste à petite taille, l'avion l'a remplacée
le soir même.*

## Le nom

**Clairance** : l'autorisation que donne le contrôleur, dans l'orthographe du contrôle français.
Il porte « clair » : l'outil rend la clairance claire. Il se dit /klɛ.ʁɑ̃s/ en français et se lit
*clearance* en anglais. Il s'écrit avec une capitale initiale et jamais en capitales ; le dépôt et
les fichiers s'écrivent `clairance`.

## Le signe

**Un avion qui s'arrache de la piste**, autorisé au décollage : la clairance que tout le monde
connaît (« autorisé décollage »). C'est un pictogramme trapu, dans l'esprit des pictogrammes
d'aéroport (les « départs » de Cook et Shanosky, 1974), choisi pour rester un avion à 16 px : un
avion fin s'efface sous 24 px (mesuré dans un onglet de navigateur, 13/10).

Construction :
- **l'avion** est dessiné sur une grille de 16, nez en haut, centré sur (8, 8) : fuselage de 4 de
  large, ailes en flèche jusqu'à (1, 12,5) et (15, 12,5), empennage jusqu'à (4,2, 15,2) et
  (11,8, 15,2). Son tracé est le même dans tous les fichiers :
  `M8 1 L10 3.5 L10 5 L15 10.5 L15 12.5 L10 9.5 L10 12 L11.8 13.8 L11.8 15.2 L8 14 L4.2 15.2 L4.2 13.8 L6 12 L6 9.5 L1 12.5 L1 10.5 L6 5 L6 3.5 Z` ;
- il est **tourné de 45°**, nez vers le haut à droite : il monte ;
- **à 64**, il est posé à l'échelle 3,3 autour de (36, 26) ; **la piste qu'il quitte** est une barre
  de 28 × 6 à (3, 55), rayon 3, sous sa queue ;
- **à 16** (onglet, en-tête, barre de menus), l'avion seul, à l'échelle 0,76 autour de (8, 8) :
  la barre n'apporterait qu'un trait sous un avion de 10 px. La règle : la piste apparaît à
  partir de 32 px.

Couleurs : l'avion en `accent` (`#4caf50`), la piste en `text.primary` (`#e0e0e0`). Sur fond
clair : avion `#2e7d32` (5,1:1 sur blanc), piste `#1a1a1a`. En une couleur (barre de menus,
gravure) : tout dans la même teinte.

Pourquoi un avion vert et non gris : le vert est la voix de l'outil, ce qui est vivant ; l'avion
qui décolle est ce qui vit. La piste, elle, est du décor, dans la couleur du texte.

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
| `signe.svg` | le signe, 64, fond sombre, avec la piste ; source des tailles ≥ 32 |
| `signe-clair.svg` | fond clair (README en thème clair, papier) |
| `signe-16.svg` | l'avion seul, pour l'onglet, l'en-tête et tout ce qui est sous 32 px |
| `favicon.svg` | tuile `#1a1a1a` à coins de 7, l'avion au centre ; à exporter en 16, 32, 180 (iOS) et 512 (manifeste) |
| `marque.svg`, `marque-clair.svg` | signe et nom |
| `barre-menus-marche.svg`, `barre-menus-arret.svg` | images « template » de 18 pt pour `tools/menubar/` |
| `index.html` | la planche : tailles, onglet, barre de menus, connexion, bandeau, README, interdits |

Export en PNG, quand il faudra : `rsvg-convert -w 512 signe.svg > signe-512.png`, ou un navigateur.

## La barre de menus

Deux états, qui diffèrent **par la forme** et pas seulement par la teinte (principe 6) :
- **en marche** : l'avion monte, à 45°, plein ;
- **à l'arrêt** : l'avion est posé, à plat, nez à droite, sur la barre du sol ; l'ensemble à 55 %
  d'opacité.

Ce sont des images « template » (noir et transparence) : macOS les teinte pour la barre claire,
la barre sombre et l'état pressé. Elles remplacent `dot.radiowaves.left.and.right` et
`antenna.radiowaves.left.and.right.slash` dans `tools/menubar/AtcScribe.swift`. Un troisième
état « dégradé » (sidecar tombé) pourra ajouter le triangle des voyants ; non dessiné.

## Où elle apparaît

- **L'en-tête de l'outil** : l'avion à 20 px et « Clairance » en monospace 600, à gauche de la
  barre d'état (esquisses A, B, C). Au téléphone, l'avion seul.
- **L'onglet** : favicon et `<title>Clairance</title>`, plus un manifeste d'application avec
  `theme-color` `#0a0a0a`, pour que le téléphone l'ajoute à son écran d'accueil.
- **La connexion** : la marque en tête de la carte, puis « Sign in » et la promesse en une
  phrase (`maquettes/connexion.html`).
- **Le bandeau de reprise** : l'avion tient lieu de voyant devant « click anywhere to resume
  listening ».
- **Le README** : `<picture>` qui sert `signe.svg` ou `signe-clair.svg` selon le thème, le nom en
  titre, et la première phrase qui porte les trois messages (`00-nom.md` § 1) : *Hear the airband
  you receive, read it in English and French, and see which aircraft is speaking. Runs on your
  own machine: nothing leaves it. Made for the listener; it judges no one.*
- **Le texte** : « Clairance » dans une phrase, sans article (« Clairance reads… », « ouvrir
  Clairance »).

## Ce qu'on ne fait pas

- l'avion à la verticale, à plat ou en descente : il monte, à 45° (à plat, c'est l'état
  « arrêté » de la barre de menus, et rien d'autre) ;
- une autre couleur que le vert de l'outil, un dégradé, une ombre, un relief, une traînée ;
- le nom en capitales, dans une autre police, ou avec le signe à droite ;
- une antenne, des ondes, une coche autour de l'avion : il est complet.

## Ce qui reste

- Le renommage lui-même est fait le 13/10 par la session atc-scribe (dépôt `chatelp/clairance`,
  lanceur, dossier de configuration, titre de page, documents). Restent de son côté : le favicon
  exporté et le manifeste, les deux images de la barre de menus, le signe et la première phrase
  dans le README.
- Les domaines `clairance.org` et `clairance.app`, libres le 13/10, si le propriétaire les veut ;
  rien n'est réservé.
- La lecture du brevet US12277862B1 (Q53) avant toute diffusion hors de France.
