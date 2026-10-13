# Esquisses : le fil au centre

*13/10/2026, session design. Les esquisses vivent dans un canevas Claude (privé, au propriétaire),
pas dans le dépôt : ce document garde ce qui y a été décidé et ce qui reste ouvert. Les maquettes
de `maquettes/` sont antérieures : elles gardent la disposition actuelle et seront refaites à
partir du design system.*

## La direction

Le 12/10, le propriétaire a demandé d'aller plus loin que le cosmétique et choisi **le fil au
centre** : la radio, transcrite et expliquée, est ce qu'on vient chercher ; la carte la situe. La
disposition actuelle n'est plus une contrainte. Méthode : esquisses, puis maquettes.

## Une disposition, deux états

| | Ce qu'on voit | Pour qui, quand |
|---|---|---|
| **A, par défaut** | le fil (≈ 440 px), le ciel, une colonne à deux onglets (≈ 380 px) | l'écoute ordinaire, l'écran d'un ordinateur |
| **B, colonne repliée** | le fil élargi en transcription (qui à gauche, quoi à droite), le ciel ; « Right now » en carte sur la carte, avec « Traffic » et « « » pour rouvrir | lire beaucoup d'échanges, un écran étroit |

B n'est pas une autre interface : c'est A dont la colonne est repliée (« » » dans l'en-tête de la
colonne). Les boutons d'écoute des fréquences sont en tête du fil dans les deux états ; la console
du bas de la première esquisse B est abandonnée. Ce partage A/B est la proposition du propriétaire
(13/10).

**La colonne** bascule entre deux onglets :
- **Right now**, par défaut : une phrase de synthèse, la séquence d'arrivée, « Worth knowing »
  (remises des gaz, changements de piste, 30 dernières minutes), les compteurs ;
- **Traffic** : la liste à plat des avions, que la première esquisse avait fait disparaître
  (remarque du propriétaire, 13/10) : recherche, curseur d'altitude avec son histogramme, Air /
  Ground / Heard, cinq familles de phases, Active / Signal lost, tri. **La carte suit les filtres**
  (« Showing 64 of 107 · SFC – FL 195 »).

**Suivre un vol** (C) remplace le contenu de la même colonne par l'histoire de l'avion, « ‹ » pour
revenir à l'onglet d'où l'on vient ; le fil se restreint à ses échanges sur toutes les fréquences.
Le principe est à valider par le propriétaire.

**La PAR existe et ne se redessine pas.** Son approche, sous la carte de C, est le cadre de la vue
PAR (`www/par/par-view.js`) réduit à la piste où il atterrit : échelle d'altitude en racine
carrée, couche de transition, cônes de 7° et 15°, plan de 3° et ses marges de 0,7°, bande
d'azimut, triangle plein quand le juge de piste l'a rattaché à cette piste, trace de l'avion
suivi en blanc. L'esquisse est générée avec les mêmes formules (remarque du propriétaire, 13/10).
De même, la séquence de « Right now » est le tableau « Arrivals to 26L » de la PAR, sorti de la
PAR : distance au seuil, heure, écart avec le précédent, ambre sous 3 NM. Le design system
reprendra ces deux composants tels qu'ils sont ; seule leur palette de phases passera aux cinq
familles (`04-migration.md`).

## D'où viennent les données

Vérifié dans le code par la session atc-scribe le 13/10. **D** disponible, **P** calculable dans
la page, **S** travail serveur, **X** pas aujourd'hui.

| | Élément | Source, ou travail |
|---|---|---|
| D | piste en service par aérodrome suivi, depuis quand | juge de piste du serveur (`/api/v1/station`, `runway_in_use`) |
| P | séquence d'arrivée hors de la PAR | les calculs de la PAR (`buildFrames`, `placeInFrame`) passent de `par-view.js` au store : ≈ ½ j page, rien au serveur |
| P | « Worth knowing » | journal de la page (100 événements), **perdu au rechargement** ; le garder : `localStorage` borné (petit) ou une route serveur « événements depuis T » (≈ 1 j) |
| D | compteurs en vol, au sol, entendus | filtres du store, `voiceAircraftCount` |
| P | phrase de synthèse | gabarits, rien de génératif ; **elle dit qu'elle vient de l'ADS-B** |
| D | antenne : à l'antenne, choisi où, depuis quand | `/api/v1/status` → `antenna` |
| D | messages ADS-B par seconde | non exposé lors de la réponse, puis **ajouté le 13/10 à 12 h 47** (`131ba7b`, `messages_per_second` dans `/api/v1/status`) : déjà dans la barre d'état de l'outil (« 84 · 703/s ») |
| P | explication en français sous l'échange | un gabarit par rôle de consigne de `Plain`, page ou `explain.go` (≈ ½ j) ; la tour n'est nommée que si sa fréquence est connue (`/frequencies`) |
| P | « someone is speaking » | niveau audio de chaque fréquence, avec un seuil ; **« transcription dans 2 s » : X, retiré** |
| P | avion suivi : première détection, transferts, écart au plan de 3°, toucher estimé | `created_at` du serveur ; le reste dans la page, même déplacement que la séquence |
| D | onglet Traffic | existe dans l'outil, réorganisé |

En somme, toute la colonne tient avec l'existant ; le seul vrai déplacement de code est celui de
la séquence d'arrivée, et le seul choix à faire est la survie de « Worth knowing » au rechargement.

## Corrections faites aux esquisses le 13/10

- « ✓ readback » remplacé par « ↩ pilot replied, 4 s later » : constater, jamais juger (rien de
  type SLASH) ;
- pas de bouton pour changer l'antenne depuis la page : « chosen at the station » ;
- « transcription in 2 s » retiré (X ci-dessus) ; le débit ADS-B garde le format de l'outil (« 107 · 700/s ») ;
- la phrase de synthèse porte « Read from the ADS-B tracks, not announced by the controllers » ;
- le signe de l'en-tête est l'état « à l'écoute » (R2) dès qu'une fréquence joue (`05-identite.md`).

## Ce qui reste

- valider le principe de « Suivre un vol » (C) ;
- choisir la survie de « Worth knowing » : `localStorage` (proposé, rien au serveur) ou une route ;
- signaler à la session atc-scribe que le cadre PAR prend sa largeur une fois par seconde : rendu
  au moment où le conteneur est encore masqué, ou onglet en arrière-plan (minuteries ralenties),
  il reste à 480 px ; un `ResizeObserver` le corrigerait (petit, non urgent) ;
- puis construire le design system (composants du canevas : échange, bande d'avion, voyant,
  bascule, onglets, carte) et refaire les maquettes à partir de lui.
