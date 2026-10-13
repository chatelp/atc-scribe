# L'identité : Clairance

*13/10/2026, session design. Le nom est décidé par le propriétaire le 13/10 (D74, `00-nom.md` § 4),
le signe le même jour. Ce document dit comment la marque est construite et comment on s'en sert.
Les fichiers sont dans `identite/` ; la planche de présentation est `identite/index.html` (à ouvrir
en servant `docs-fr/design/`, comme les maquettes).*

## Comment le signe a été choisi

Le 13/10, dans l'ordre :
1. une **piste et une coche** « autorisé » : la piste, une barre plate, n'était pas reconnaissable
   à petite taille (le propriétaire) ;
2. un **avion pictogramme** qui décolle : le propriétaire n'aimait pas la forme de l'avion, et a
   demandé d'abord l'inventaire des avions de l'interface, puis notre propre famille
   (`06-avions.md`), dans laquelle choisir ;
3. sur la planche finale (huit signes aux mêmes tailles et dans les mêmes usages : trois tours de
   contrôle, trois radars, deux avions de la famille), le propriétaire a retenu **R1, le scope
   radar**.

Ce qui a pesé : le scope se lit à 16 px (un cercle, un trait, un point) ; son noir et son vert sont
ceux de l'outil, la marque et l'interface parlent la même langue ; il met moins le contrôleur au
centre qu'une tour. Ce qui reste à savoir : c'est l'image la plus attendue pour un outil qui affiche
une carte, et les traqueurs de vols l'emploient sans doute (non vérifié) ; le registre du radar est
aussi celui de la surveillance, que la présentation doit désamorcer par ses mots (les trois messages
de `00-nom.md` § 1). L'avion de la famille reste celui de la carte.

## Le nom

**Clairance** : l'autorisation que donne le contrôleur, dans l'orthographe du contrôle français.
Il porte « clair » : l'outil rend la clairance claire. Il se dit /klɛ.ʁɑ̃s/ en français et se lit
*clearance* en anglais. Il s'écrit avec une capitale initiale et jamais en capitales ; le dépôt et
les fichiers s'écrivent `clairance`.

## Le signe

**Le scope radar** : le cercle de l'écran, un anneau de distance, le balayage et le plot qu'il
vient d'afficher.

Construction, sur une grille de 64, centre (32, 32) :
- **le cercle de l'écran** : rayon 26,5, trait de 4 ;
- **un anneau de distance** : rayon 13, trait de 2, à 55 % ;
- **le balayage** : un trait de 4, bouts ronds, du centre jusqu'à 45° (nord-est) ;
- **la lueur** qu'il laisse derrière lui : le secteur de 0° à 45°, vert à 35 %. Le balayage
  tourne dans le sens des aiguilles d'une montre, la lueur est donc du côté de 0° ;
- **un plot** : un disque de rayon 3,8 à 25° et à 17 du centre, dans la lueur, celui que le
  balayage vient de peindre.

**À 16 px** (`signe-16.svg`) : la même grille divisée par quatre, sans l'anneau de distance (il
ferait une tache grise), les traits portés de 1 à 1,6 pour garder un pixel plein. Le plot a un
rayon de 1,15.

Couleurs : le cercle et l'anneau en `text.primary` (`#e0e0e0`) ; le balayage, la lueur et le plot
en `accent` (`#4caf50`). Sur fond clair : cercle `#1a1a1a`, vert `#2e7d32` (5,1:1 sur blanc), lueur
à 25 %. En une couleur (barre de menus, gravure) : tout dans la même teinte, la lueur à 35 %.

**Le signe ne bouge pas.** Un balayage qui tourne serait l'animation décorative que le principe 5
exclut. Le seul usage animé envisageable est l'indicateur de chargement (le balayage fait un tour
pendant l'attente) : à décider à la migration, jamais en boucle permanente.

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
| `signe.svg` | le signe, 64, fond sombre ; source des tailles ≥ 24 |
| `signe-clair.svg` | fond clair (README en thème clair, papier) |
| `signe-16.svg` | sans l'anneau de distance, pour l'onglet, l'en-tête et tout ce qui est sous 24 px |
| `favicon.svg` | tuile `#1a1a1a` à coins de 7, le scope au centre ; à exporter en 16, 32, 180 (iOS) et 512 (manifeste) |
| `marque.svg`, `marque-clair.svg` | signe et nom |
| `barre-menus-marche.svg`, `barre-menus-arret.svg` | images « template » de 18 pt pour `tools/menubar/` |
| `index.html` | la planche : tailles, onglet, barre de menus, connexion, bandeau, README, interdits |

Export en PNG, quand il faudra : `rsvg-convert -w 512 signe.svg > signe-512.png`, ou un navigateur.

## La barre de menus

Deux états, qui diffèrent **par la forme** et pas seulement par la teinte (principe 6) :
- **en marche** : le cercle, le balayage, sa lueur et le plot ;
- **à l'arrêt** : l'écran vide, le cercle et son seul centre, à 55 % d'opacité.

Ce sont des images « template » (noir et transparence) : macOS les teinte pour la barre claire,
la barre sombre et l'état pressé. Elles remplacent `dot.radiowaves.left.and.right` et
`antenna.radiowaves.left.and.right.slash` dans `tools/menubar/AtcScribe.swift`. Un troisième
état « dégradé » (sidecar tombé) pourra remplacer le plot par le triangle des voyants ; non dessiné.

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

- le cercle sans le balayage : ce n'est plus un radar (sauf l'état « arrêté » de la barre de
  menus, et rien d'autre) ;
- d'autres plots, un avion dans l'écran, une grille, des graduations : un seul plot ;
- une autre couleur que le vert de l'outil, un dégradé, une ombre, un relief, un reflet de verre ;
- le balayage ailleurs qu'à 45°, ou qui tourne en boucle ;
- le nom en capitales, dans une autre police, ou avec le signe à droite.

## Ce qui reste

- Le renommage lui-même est fait le 13/10 par la session atc-scribe (dépôt `chatelp/clairance`,
  lanceur, dossier de configuration, titre de page, documents). Restent de son côté : le favicon
  exporté et le manifeste, les deux images de la barre de menus, le signe et la première phrase
  dans le README.
- Les domaines `clairance.org` et `clairance.app`, libres le 13/10, si le propriétaire les veut ;
  rien n'est réservé.
- La lecture du brevet US12277862B1 (Q53) avant toute diffusion hors de France.
