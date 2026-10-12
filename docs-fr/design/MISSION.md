# Mission de l'agent « design » — une identité et un système de design pour atc-scribe

*Écrite le 12/10/2026 par la session atc-scribe (« Co-ATC local fork adaptation (fork) »), à la
demande du propriétaire (Pierre). À lire en premier. Tout ce qui s'adresse au propriétaire est en
français ; le code, ses commentaires et les commits sont en anglais.*

## La demande

Le propriétaire, le 12/10 :

> Étudier une vraie identité visuelle et fonctionnelle (UX) distincte de co-atc, notre adaptation
> étant maintenant suffisamment mature. La dominance noir / vert est probablement à garder, elle
> fait référence à une interface « professionnelle », mais j'aimerais tout remettre à plat et
> avoir un VRAI design system pour l'outil.

**C'est ta mission** : l'identité (nom, marque, icône, page de connexion, ce que voit quelqu'un
qui découvre l'outil) et le système de design (couleurs, typographie, espacements, composants,
états, mouvement, accessibilité), avec des maquettes des écrans principaux et un plan de
migration. **Tu n'implémentes pas dans l'interface vivante** : tu produis l'étude, le système et
les maquettes ; la mise en œuvre se fera ensuite, par étapes, avec la session atc-scribe, sous ses
garde-fous. Tu peux coder des maquettes HTML statiques, pas toucher à `www/` en production.

## Ce qu'est l'outil, et pour qui

Lis dans cet ordre : `README.md`, `docs-fr/00-mission.md`, `docs-fr/32-audit-produit.md` (le
plan de l'audit et tout ce qui a été fait depuis le 10/10 : réglages, journal d'événements,
curseur d'altitude, réécoute, fiche de vol, séquence d'arrivée, bandeau d'état, barre radio,
téléphone), `docs-fr/audit-2026-10-09/ux.md` (le jugement UX de départ, mesuré), et
`docs-fr/31-vue-par.md` (la vue PAR, l'écran le plus « métier »).

Puis **regarde l'interface tourner** : `runs/son-test/son-test.command` lance une instance d'essai
sur http://127.0.0.1:8020 (compte dans `runs/son-test/compte-essai.json`, à ne jamais recopier
dans un document) avec du trafic réel ; les deux fréquences se remettent par
`runs/son-test/radio-ctl-sync -station http://192.0.2.10 -station-host station.lan
-streams http://audio.lan -coatc http://127.0.0.1:8020 -user essai -once` (mot de passe dans
`COATC_PASSWORD`). Arrête l'instance en partant : rien ne doit tourner en permanence sur le Mac.

**L'utilisateur** : le propriétaire, novice en aéronautique, qui écoute tous les jours, souvent le
soir, sur un Mac à 1 440 px et parfois sur son téléphone par VPN ; demain, peut-être, des
radioamateurs et des passionnés (une étude de communauté est en cours à part). Deux exigences
qu'il a posées : « utilisable par un novice », et des **aides contextuelles** partout où un mot
d'aviation surprend (c'est fait : 408 éléments portent une bulle `data-help`). Le noir et le vert
restent la dominante : c'est la lecture de nuit, et le code de l'outil professionnel.

## Ce qui existe, mesuré le 12/10 (à refaire, c'est ton inventaire de départ)

- **Typographie** : monospace partout (`font-mono` sur `body`). **Onze tailles** en usage dans
  les classes : `text-xs` (94 occurrences), `text-sm` (60), 10 px (54), 9 px (25), 11 px (13),
  8 px (14), **7 px (15)**, puis `base`, `lg`, `xl`. Le corps de la liste est à 12 px, des étiquettes
  à 7 et 8 px : c'est le premier chantier.
- **Couleurs** : 8 couleurs de thème nommées dans `tools/tailwind/tailwind.config.js` (background
  `#0a0a0a`, panel `#1a1a1a`, text `#e0e0e0`, highlight `#4CAF50`, warning, danger, border, hover),
  mais **75 classes de couleur distinctes** dans les gabarits (red, green, amber, sky, emerald,
  orange, violet, lime, pink, purple, teal, gray…) et **une centaine de valeurs hexadécimales** en
  dur dans `www/style.css` (54), `www/par/par-view.js` (13) et le rendu des avions
  (`www/map/renderers/aircraft-webgl.js`, 34). Les dix phases de vol ont chacune leur couleur, les
  voyants d'état sont vert / ambre / rouge / gris, les bandes d'altitude colorent les avions sur
  la carte, la vue PAR a sa palette.
- **Composants** de fait : tuiles de fréquence, bandes d'avion (« flight strips »), fiche avion à
  quatre onglets, filtres (boutons, cases, curseur à deux poignées avec histogramme), tableau de
  trajectoire, fils de transcription (popover), journal d'événements (panneau), page des réglages
  (plein écran, cinq rubriques), bulles d'aide, voyants, barre radio, onglets téléphone, vue PAR
  (SVG), carte (OpenLayers, fond noir par défaut, plan d'aérodrome en GeoJSON).
- **Identité** : le titre de la page dit encore `Co-ATC`, pas de logo ni de favicon propres,
  l'icône de la barre de menus est un symbole SF, la page de connexion est un formulaire nu.
- **Pile** : Alpine.js, Tailwind **compilé** (`tools/tailwind/build.sh`, à relancer après tout
  changement de classe, le filet `tools/www-check/check.mjs` le réclame), `style.css`,
  OpenLayers, Font Awesome 6 (solid et regular, servies localement), aucun hôte externe.

## Ce qui est demandé

### 1. L'inventaire visuel

Refais la mesure ci-dessus proprement, par script, et complète-la : chaque couleur avec ses
usages, chaque taille avec les siens, les espacements, les rayons, les ombres, les icônes
utilisées (lesquelles, combien), les états (survol, actif, désactivé, sélectionné, erreur), les
contrastes réels (WCAG, texte sur fond) des combinaisons les plus fréquentes. Dis ce qui est
incohérent et ce qui est juste. Livrable : `docs-fr/design/01-inventaire.md`, avec le script
dans `tools/design/`.

### 2. Les principes

Dix principes au plus, chacun justifié par un usage de l'outil, pas par une mode : lecture de
nuit, densité d'un écran de contrôle contre lisibilité d'un novice, ce qui doit attirer l'œil
(une urgence, un changement de piste) et ce qui doit se taire, la cohérence entre la carte, la
liste et la PAR, le téléphone. Livrable : `docs-fr/design/02-principes.md`.

### 3. L'identité

**Le nom change, et c'est à toi de le chercher** (ajouté le 12/10, après la décision du
propriétaire) : « Scribe » est le nom de l'outil de transcription ATC de la DSNA, contesté par le
SNCTA en septembre 2025 ; pour un contrôleur français, le mot évoque la surveillance, et c'est le
public des annotateurs. Livrable préalable, `docs-fr/design/00-nom.md` : cinq à huit propositions
(courtes, prononçables dans les deux langues, sans « scribe », sans aviation d'État ni
surveillance, disponibilité vérifiée sur GitHub et dans l'écosystème), chacune avec sa
justification et le signe graphique qu'elle permet ; et, dans le même document, **la pertinence
d'un positionnement « espace aérien français »** dans le nom, la marque et la présentation du
projet sur GitHub, avec des variantes de nom dans les deux cas (français assumé / générique avec
le français comme première langue servie). Le propriétaire choisit ; rien n'est marqué d'un nom
avant son choix, et la marque se conçoit comme un système applicable au nom retenu. Propose : une marque
(typographie du nom, un signe simple qui tienne à 16 px dans un onglet et dans la barre de
menus), un favicon, l'icône de la barre de menus (deux états : production arrêtée / en marche),
la page de connexion, l'écran d'accueil (« Start Monitoring » aujourd'hui), et ce que montrerait
un README. Fichiers SVG dans `docs-fr/design/identite/`. Pas de fabrication par IA générative
d'images : du SVG dessiné, lisible, modifiable.

### 4. Le système de design

`docs-fr/design/03-systeme.md`, et ses **jetons** dans un fichier exploitable
(`docs-fr/design/jetons.json` ou un `tailwind.config.js` proposé, à côté de l'actuel, pas à sa
place) :

- **couleurs** : une échelle de gris pour les fonds et les textes, le vert et ses nuances, et une
  palette **sémantique** (urgence, avertissement, succès, information, neutre) avec, pour chaque
  texte sur chaque fond, le contraste calculé ; les dix phases de vol, les niveaux d'altitude de
  la carte, les voyants, les niveaux d'indices de la preuve d'appariement, le locuteur (ATC /
  pilote) : chacun une couleur justifiée, **distinguable par un daltonien** (le rouge / vert des
  voyants actuels ne l'est pas : propose la forme ou la position en plus de la couleur) ;
- **typographie** : garder ou non le monospace, et où ; une échelle de **quatre ou cinq tailles**
  au plus, avec un plancher (11 px ?) et la règle pour les étiquettes d'unité ;
- **espacements, rayons, bordures, ombres, élévation** (popovers, panneaux, page des réglages) ;
- **composants**, chacun avec ses états et ses variantes : bouton, bouton-filtre, case, onglet,
  tuile de fréquence, bande d'avion, fiche, chip (indicatif, valeur, consigne en clair),
  voyant, curseur, tableau, popover, panneau, bulle d'aide, message vide, message d'erreur ;
- **mouvement** : ce qui bouge (les avions, une alerte) et ce qui ne bouge pas ;
- **densité** : deux niveaux au plus (écran, téléphone), pas de troisième ;
- **accessibilité** : contrastes, cibles tactiles, focus clavier (les raccourcis existent :
  0-9, A, G, Espace, Tab), `prefers-reduced-motion`.

### 5. Les maquettes

Des pages HTML statiques, dans `docs-fr/design/maquettes/`, construites avec les jetons proposés
(un `tokens.css` partagé), qui montrent **les écrans réels avec des données plausibles** : l'écran
principal à 1 440 × 900 (carte + liste + barre radio), le même avec la vue PAR, la fiche d'un
avion (onglet Timeline), la page des réglages, le journal d'événements ouvert, le téléphone à
375 px (carte, liste, fiche), la page de connexion. Pas de captures d'écran retouchées : du HTML
qu'on ouvre et qu'on compare côte à côte avec l'instance d'essai. L'application Claude Desktop
sait aussi produire des artefacts de type « design » et « design system » : utilise-les si ça
t'aide à itérer avec le propriétaire, mais le livrable est dans le dépôt.

### 6. Le plan de migration

`docs-fr/design/04-migration.md` : comment appliquer le système à l'interface vivante **sans
rien casser** (règle du chantier, sept garde-fous dans `32-audit-produit.md`), par étapes
livrables séparément : d'abord les jetons dans `tailwind.config.js` et `style.css` (couleurs et
tailles, à comportement identique), puis composant par composant ; pour chaque étape, ce qui
change à l'écran, ce qui ne change pas, et comment on le vérifie (le filet `www-check`, la
comparaison des styles calculés comme au 4.3, une capture avant / après). Estime l'effort.

## Les règles

- **Rien dans `www/` ni dans `tools/tailwind/`** pendant cette mission : tes fichiers vont dans
  `docs-fr/design/` et `tools/design/`. La mise en œuvre viendra après, avec la session atc-scribe.
- **Une branche à toi** : `design`, depuis `local`, dans un arbre de travail séparé pour ne pas
  gêner la session atc-scribe qui commite sur `local`. Commits en anglais, signés comme les
  autres (`Co-Authored-By: Claude <modèle> <noreply@anthropic.com>`). Pas de push sur `main` : le
  propriétaire décide quand l'étude entre dans le dépôt public ; d'ici là, la branche reste
  locale ou poussée sous son nom.
- **Mesurer avant de conclure** : chaque constat sur l'interface actuelle vient de l'inventaire,
  chaque contraste est calculé, chaque taille est justifiée par l'usage et la distance de
  lecture. Dis ce que tu n'as pas mesuré.
- **Le noir et le vert restent** ; tout le reste se discute, principes à l'appui.
- Les textes de l'interface sont en anglais à ce stade (décision du 10/10, une version en/fr
  viendra) ; les maquettes aussi, sauf les interprétations de communications françaises.
- Pas de secret, pas de mot de passe, pas de coordonnées exactes de la station dans les
  documents ni les maquettes ; les indicatifs des maquettes sont inventés.
- **Parle à la session atc-scribe** (« Co-ATC local fork adaptation (fork) », par SendMessage)
  pour toute question sur ce que fait l'outil ou sur un chiffre ; elle te répondra en français.
  Quand le système et les maquettes sont prêts, dis-le au propriétaire : c'est lui qui valide, et
  c'est après sa validation que la migration commence.

## Les livrables, en résumé

```
docs-fr/design/
  MISSION.md            cette fiche
  01-inventaire.md      l'existant, mesuré
  02-principes.md       dix principes au plus
  03-systeme.md         le système : jetons, composants, états
  jetons.json           les jetons, exploitables
  identite/             marque, favicon, icône de barre de menus (SVG)
  maquettes/            tokens.css et les écrans HTML statiques
  04-migration.md       le plan, par étapes vérifiables, avec l'effort
tools/design/           le script d'inventaire
```
