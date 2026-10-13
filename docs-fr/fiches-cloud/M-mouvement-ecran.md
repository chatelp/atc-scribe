# Fiche M — Le mouvement à l'écran : le mesurer dans un vrai navigateur, et le rendre le plus fluide possible

*Vague 3, ajoutée le 13/10 dans la nuit, à la demande du propriétaire : « un suivi des avions le
plus fluide possible à l'écran, sans tous les problèmes d'affichage précédents ». Branche :
`cloud/m-mouvement-ecran`. Lis d'abord `README.md` de ce dossier : ses règles s'appliquent.
Lis ensuite, dans cet ordre : la décision **D79** (`docs-fr/05-decisions.md`), la fiche
**L** (`L-bloc-position.md`) et son **rapport** (`L-rapport.md`), et dans
`docs-fr/32-audit-produit.md` le retour de la fiche L du 13/10 (« fiche L intégrée »). C'est
l'histoire complète du défaut, et le point de départ de cette fiche.*

## Le constat, au 13/10 au soir

Les problèmes vus par le propriétaire, dans l'ordre où ils ont été trouvés : des avions qui
« pulsent », reculent puis avancent, font de grands sauts, des glyphes mal orientés (D79, 33 %
de pas à reculons) ; puis un tremblement, un saut de 76 m une fois par seconde (D79, suite) ;
puis une vitesse dessinée qui variait de 38 à 48 % (fiche L). **Après la fiche L, mesuré en
direct sur le Mac** : 0 % de pas à reculons, 0 à 0,7 % d'images lentes, 0 à 0,6 % de sauts,
8 à 13 % de variation de la vitesse dessinée.

Ce qui reste, et qui est ton sujet :
1. **Aucune mesure ne regarde l'écran.** Le banc de la fiche L (`internal/adsb/replay_test.go`
   puis `www/aircraft-replay.test.js`) rejoue le flux dans le moteur avec une **fausse carte**
   qui note les poses. La vraie carte (OpenLayers, rendu WebGL de `map/renderers/aircraft-webgl.js`),
   le fil principal partagé avec Alpine et la liste, le rythme réel des images, rien de tout
   cela n'est mesuré. La mesure en direct du Mac est une sonde à la main, sur le trafic du moment.
2. **La première minute après un redémarrage du serveur** : 76 % de variation de la vitesse
   dessinée, sur **une seule passe** de 20 s faite aussitôt après le redémarrage (la passe
   suivante, une minute plus tard : 8 %). La cause **n'est pas mesurée** : deux hypothèses du
   Mac, le moteur qui estime le décalage d'horloge serveur/page sur 30 s (`clockWindowSec`), et
   le suivi de trajectoire du serveur qui n'a pas encore d'historique (prévision depuis un seul
   relevé, `ForecastFromTarget`). Il peut y en avoir d'autres.
3. **Un avion qui retrouve sa position après un long trou** (cas réel : 18 s sans position,
   à 45 NM) **recule** pour la rejoindre.
4. Non mesurés : l'onglet caché puis montré, un zoom ou un déplacement de carte pendant le
   mouvement, la rotation du glyphe en virage, 150 avions avec l'onglet Traffic ouvert
   (l'étape 5 a mesuré 25 blocages de plus de 30 ms en 10 s avec la liste, aucun sans).
5. **La stratégie elle-même n'a jamais été comparée.** Le moteur extrapole depuis le dernier
   point connu et efface l'écart à chaque position neuve. L'autre famille, celle des jeux en
   réseau, **dessine dans le passé** : l'avion est placé à `maintenant − d`, interpolé entre
   deux positions réellement reçues ; il n'y a jamais de correction, au prix d'un retard `d`
   (à 230 m/s, 2 s de retard font 460 m). Personne n'a mesuré laquelle est la plus fluide
   **et** la plus juste ici, avec une position toutes les 1,6 à 1,8 s.

## Étapes, dans cet ordre, un commit chacune

1. **Le banc navigateur, avant toute modification.** Dans `tools/motion-bench/` : le **vrai
   serveur** compilé, avec une copie de `configs/config.toml.example` (sans connexion :
   `[auth] enabled = false` et serveur sur 127.0.0.1 ; la station placée au **récepteur fictif
   de la fiche L, 45,0 / 4,67**, jamais ailleurs), sans transcription (fiche H), en source
   `tar1090` sur une adresse locale où le générateur sert `data/aircraft.json`,
   `receiver.json` et `stats.json` ; un `aircraft.json` que le **générateur de la fiche L**
   réécrit chaque seconde **en temps réel**. Ce générateur vit aujourd'hui **dans un fichier de
   test** (`internal/adsb/replay_test.go`, paquet `adsb`, horloge simulée par la couture
   `s.now()`) : un exécutable ne peut pas l'importer. Sors-le d'abord dans un paquet ordinaire
   (par exemple `internal/adsb/simtraffic`) que le test de rejeu et le banc partagent, **sans
   changer la référence de la fiche L** (le rejeu doit redonner ses chiffres à l'arrondi
   près), puis donne-lui un nombre d'avions réglable (12 aujourd'hui ; 60 et 150 ici) (même régime : 0,6 position par seconde et par avion, `seen_pos` qui grimpe, des
   pertes de 30 s, et un trou de 18 s suivi d'une reprise) ; la **vraie page** servie par ce
   serveur ; **Chrome sans interface** piloté par le protocole DevTools, fenêtre de
   1 440 × 900, onglet au premier plan. La mesure se prend **dans la page, à chaque image**
   (`requestAnimationFrame`) : la position **réellement posée** sur chaque entité de la carte
   (lue sur l'entité OpenLayers, pas à la sortie du moteur), son orientation, l'intervalle
   entre images, les tâches longues (`PerformanceObserver`, `longtask`). La vérité vient du
   générateur, par une route de test ou un fichier, jamais par la page. Métriques, les mêmes
   que la fiche L pour qu'elles se comparent, plus celles de l'écran :
   - pas à reculons, images lentes (< 30 % de la vitesse médiane), sauts (> 2×), écart-type
     de la vitesse dessinée rapportée à la vraie, plus grande correction, erreur moyenne et
     p95 contre la vraie position ;
   - rythme : intervalle médian et p95 entre images, part des images à plus de 50 ms, tâches
     longues par 10 s ;
   - glyphe : sauts d'orientation de plus de 3° d'une image à l'autre.

   Chrome sans interface : s'il n'est pas sur ta machine, installe-le (Chrome for Testing,
   ou le paquet Chromium) ; si c'est impossible, dis-le dans le rapport et arrête-toi à ce
   qui se mesure sans navigateur, sans passer aux étapes suivantes.

   Scénarios, chacun 60 s après une minute de chauffe, sauf le premier : **(a)** redémarrage
   du serveur, mesuré dès la première seconde ; **(b)** 60 avions, onglet Right now ;
   **(c)** 150 avions, onglet Traffic ouvert ; **(d)** un zoom de 10 à 50 NM et un
   déplacement de carte pendant le mouvement ; **(e)** l'onglet caché 30 s puis montré ;
   **(f)** le trou de 18 s. Les chiffres de ce premier passage sont la **référence**, gardée
   dans le dépôt comme celle de la fiche L. Dis dans le rapport ce que le Chrome sans
   interface de ta machine ne représente pas (WebGL logiciel, pas de GPU, pas l'écran à 120 Hz
   du Mac).
2. **Les trois défauts connus**, un commit chacun, mesurés avant et après sur les deux bancs :
   la première minute (**d'abord en trouver la cause** sur le scénario (a), seconde par
   seconde ; puis la corriger : par exemple amorcer le décalage d'horloge dès le premier
   message plutôt qu'au bout de 30 s, si c'est lui) ; la reprise après un long trou (au-delà
   d'un écart que tu chiffres, l'avion **ne glisse pas à reculons** : il se fond de l'ancienne
   pose vers la nouvelle, ou saute s'il est hors de l'écran ; la règle est dite et testée) ;
   l'onglet montré à nouveau (aucun balayage de rattrapage : la pose repart de l'état courant).
3. **L'étude des stratégies.** Le moteur gagne une option de stratégie (`config`), **la valeur
   par défaut ne change pas** : **A**, l'actuelle (extrapolation, écart qui s'efface) ; **B**,
   le dessin dans le passé (`d` réglable ; essaie au moins le p95 de l'intervalle entre deux
   positions, et la moitié) ; **C**, un hybride que tu justifies (par exemple un retard d'une
   seconde et une extrapolation courte). Mesure les trois sur les deux bancs et les six
   scénarios. Le retard de B se compte dans l'erreur contre la vraie position : c'est là
   qu'il se paie. Lis aussi comment font les outils publics dont le code est ouvert
   (tar1090 en premier) et dis-le en trois lignes chacun, sources citées ; ne copie pas leur
   code.
4. **Le reste de ce que montre le banc**, un commit par cause, seulement là où une mesure
   désigne la cause : le fil principal (ce qui bloque plus de 50 ms pendant le mouvement),
   la rotation du glyphe, l'application des poses à la carte. Une cause qui relève de la
   liste ou de la colonne (Alpine, `index.html`) se **note** dans le rapport, elle ne se
   corrige pas ici.

## Ce que tu ne touches pas

La migration de l'interface tourne en même temps sur le Mac : **ni `index.html`, ni les CSS
(`style.css`, `tokens.css`, `components.css`), ni `vendor/` à la main, ni `store/sidebar.js`,
`store/station.js`, `store/now.js`**. Exception mécanique : toute modification d'un script de
`www/` change la ligne d'empreinte de `vendor/tailwind.css`, et le filet échoue tant qu'elle
n'est pas régénérée par `tools/tailwind/build.sh` ; régénère-la, ne l'écris jamais à la main
(elle sera régénérée de nouveau à la fusion, comme pour la fiche L). Les fichiers du mouvement sont à toi
(`aircraft-animation.js` et ses tests, `store/feed.js` pour ce qui le concerne,
l'application des poses dans `map/openlayers-map-manager.js`, `map/renderers/aircraft-webgl.js`
seulement si la mesure le désigne, et alors au plus juste : la session design y prépare des
étiquettes allégées à faible zoom, dis chaque ligne touchée dans le rapport, `internal/adsb/live_prediction.go` et le suivi de trajectoire). Les
formes des messages WebSocket et des routes restent celles que la fiche L a figées, tests
compris. **Le choix de la stratégie par défaut revient au propriétaire** : c'est une décision
structurante ; tu la recommandes, chiffres à l'appui, tu ne la fais pas. Rien de la station :
le générateur suffit, et lui seul connaît la vraie position.

## Rapport

`docs-fr/fiches-cloud/M-rapport.md` (voir README), en français, 100 lignes au plus. En plus :
le tableau des métriques par scénario, avant et après l'étape 2 ; le tableau A / B / C sur
les deux bancs ; ta recommandation et son prix (fluidité contre retard, en mètres) ; la
commande qui rejoue le banc navigateur, pour que la session du Mac le relance sur la vraie
machine, avec le vrai écran, avant de juger.
