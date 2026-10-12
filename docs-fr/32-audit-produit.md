# Audit produit : atc-scribe n'est plus co-atc — 09/10

*Ouvert le 9 octobre 2026 à la demande du propriétaire. Étude, pas chantier : rien n'est codé
avant discussion (Q52 dans `05-decisions.md`).*

## Pourquoi maintenant

Mesuré le 09/10 contre `upstream/main` (dernier commit amont : 3 mai 2026) :

| | |
|---|---|
| Commits du fork | 263 |
| Fichiers touchés | 169, dont **130 nouveaux** et 38 fichiers amont modifiés |
| Lignes | **+33 941 / −1 312** |
| Serveur Go aujourd'hui (hors tests) | 35 077 lignes |
| Interface (JS + HTML) | 13 036 lignes |

Ce que le fork a ajouté n'est pas une adaptation : transcription locale par sidecar, lecture
française à deux modèles, liaison avec la station (`radio-ctl-sync`), un flux par fréquence,
appariement avion-transmission validé contre l'ADS-B, vue PAR, juge des pistes validé contre
FlightAware, authentification, accès réseau. Ce que l'amont visait (Toronto, API OpenAI, chat
ATC, simulation) n'est pas ce qui tourne ici. L'interface, elle, est restée celle de l'amont,
avec nos ajouts par-dessus : c'est là que ça se voit le plus (D70 et le son, 02/10 ; la coupure
à −40 dB, 04/10 — deux défauts amont découverts parce qu'on s'en sert tous les jours).

## Périmètre et méthode

Trois études parallèles, factuelles, chiffrées, limitées à 150 lignes chacune, rapports bruts
dans `docs-fr/audit-2026-10-09/` (copiés dans le dépôt le 10/10 pour les sessions cloud) :

1. **`ux.md`** : inventaire de chaque panneau, bouton, modale et réglage, classé essentiel /
   utile / inutilisé chez nous / douteux ; frictions (écran d'accueil, clics pour le son,
   téléphone à 390 px) ; ce qui manque pour notre usage (antenne, station, sidecar) ; dix
   changements classés par gain et effort.
2. **`backend.md`** : paquets Go, lignes, utilisé ou non dans notre configuration, couverture
   de tests ; ce qui est à nous contre l'amont ; code mort, fonctions de plus de 150 lignes ;
   `config.toml` réel contre `config.toml` lu ; dix simplifications avec risque pour la
   production.
3. **`frontend-code.md`** : carte de `www/`, dépendances, code mort (chat, simulation,
   météo, OpenAI temps réel), grosses fonctions, et un plan de découpage de `app.js`
   par étapes qui laissent l'interface fonctionnelle.

Les trois rapports sont ensuite consolidés **ici**, en une liste unique classée par
gain / effort / risque, et le propriétaire tranche ce qui devient chantier.

## Ce que l'audit doit respecter

- **Ne pas casser ce qui marche** (consigne du propriétaire, 10/10). Ce qui tourne aujourd'hui
  a été mesuré et validé : juge des pistes 368/368 contre FlightAware, appariement mesuré
  contre l'ADS-B, PAR, lecture française, liaison station, archive audio. Optimiser le code
  ne vaut rien si l'un d'eux recule. Les garde-fous, à chaque étape :
  1. **Un pas à la fois, un commit par pas**, défaisable par `git revert`, et le binaire
     précédent gardé (`runs/co-atc.avant-<date>`) pour revenir en une minute.
  2. **Le filet avant de retirer quoi que ce soit** : côté interface, le script qui vérifie que
     chaque `$store.atc.X` d'`index.html` et chaque `store.X` des autres fichiers existe
     encore, `node --check` et les tests de la PAR ; côté serveur, `go build`, `go vet`,
     `go test -race ./...`. Le compilateur et ce script tranchent, pas l'intuition.
  3. **L'instance d'essai d'abord** (`runs/son-test/`, compte, vrais flux de la station) : tout
     changement se regarde dans un navigateur et s'écoute avant de toucher la production.
  4. **Les mesures de référence se rejouent** après tout changement d'un chemin qui compte :
     l'outil d'appariement (`cmd/phraseology`), le juge des pistes sur une journée, les compteurs
     d'animation avant/après avec 80 à 110 avions pour le flux d'avions.
  5. **Le propriétaire valide à l'usage**, une journée d'écoute, avant le pas suivant sur
     le même terrain.
  6. **On ne renomme rien de ce qui est enregistré chez lui** : les 44 clés `localStorage`
     (ses préférences), les noms du store lus par `index.html`, les routes de l'API utilisées
     par `radio-ctl-sync`, le sidecar et les outils.
  7. **Retirer avant de déplacer** : supprimer du code mort ne change pas l'ordre
     d'initialisation ; déplacer, si. Et on ne déplace pas ce qui va être réécrit.
- **Le serveur tourne tous les jours.** Chaque étape doit laisser la production utilisable ;
  rien de « grand soir ».
- **Garder identifiable ce qui est contribuable** (CLAUDE.md) : sidecar, correctif de
  rotation (Q26), authentification, état serveur — même si on ne rebase plus.
- **L'UX se juge sur l'usage réel** : une personne, tous les jours, Mac et téléphone par
  Tailscale, 1 à 7 fréquences. Pas sur ce que l'interface amont permet.
- **Mesurer avant de conclure** : chaque suppression proposée cite le fichier et le nombre
  de lignes, chaque friction cite le geste et le nombre de clics.

## Alignement avec les objectifs de départ (10/10, à valider avant tout code)

Demande du propriétaire : avant de coder, vérifier que le plan ne contredit pas les objectifs
de départ, qui visaient un fork réintégrable dans co-atc. Relu : `00-mission.md`, le
« Why a fork » du README public, Q11 (15/09), D19 (16/09), D3 (15/09), Q7.

### Ce que les textes fondateurs disent

| Texte | Ce qu'il promet | Tenu ? |
|---|---|---|
| `00-mission.md` | « Adapter Co-ATC pour qu'il tourne entièrement en local … et en faire un dépôt public réutilisable » ; trois apports : zéro cloud, bilingue, multicanal réel | oui ; les trois apports existent et sont mesurés |
| `00-mission.md`, hors périmètre | l'assistant vocal et l'extraction de clairances par IA : « on décidera plus tard s'il passe en local, devient optionnel ou disparaît » | la question était ouverte ; tranchée le 10/10 : il disparaît |
| `00-mission.md`, périmètre | météo française (SIA) à la place de Windy (Q7) | **non fait** ; Windy marche (144 relevés/jour) et l'audit la garde |
| Q11 (15/09) | « vrai fork rebasable », parce que notre part est profonde mais étroite : « nous ne touchons pas une ligne de l'interface » | **plus vrai** : 17,9 % d'`index.html`, 8,8 % d'`app.js`, et le plan UX va bien au-delà |
| D19 (16/09) | le dépôt reste un fork git (historique, remote, MIT, crédit) ; **la rebasabilité n'est plus une règle** ; « ce qui est contribuable reste identifiable » (sidecar et `local.go`, Q26, `auth`, état serveur) | tenu ; c'est la règle en vigueur |
| D3 (15/09) | « transcription entièrement locale, l'API OpenAI est retirée » | décidé, **pas fait** : 1 990 lignes du chemin OpenAI sont toujours là |
| README public | « Upstream's map, ADS-B ingestion, flight-phase detection and web interface are used **as they are** … nothing here improves on them » ; assistant vocal « untouched, and off unless you supply a key » | **faux depuis la PAR et le juge des pistes** ; faux dès que le chat part |

### Les contradictions, et comment les lever

1. **« Fork réintégrable » (Q11) contre « produit à part » (Q52).** Q11 a été révisée par D19
   dès le 16/09 : on ne rebase plus, on ne code plus pour rester rebasable. L'audit ne crée pas
   la rupture, il la constate et l'étend à l'interface. Ce qui reste de l'intention d'origine,
   et qui a un sens : **contribuer en amont ce qui est utile à d'autres**, par des correctifs
   autonomes qu'on peut proposer tels quels, sans que le dépôt entier ait à se rapprocher.
   Lever : une décision explicite (D71 ci-dessous) qui remplace l'objectif de Q11.
2. **« Interface utilisée telle quelle » (README, Q11) contre la refonte UX (paliers 1, 2, 4).**
   Une fois la page Réglages, les alertes et le téléphone refaits, l'interface est la nôtre.
   L'amont n'a rien publié depuis le 3 mai ; rien à absorber. Lever : le dire dans D71 et
   réécrire la section « What is different » du README.
3. **« Dépôt public réutilisable » contre « retirer ce qui ne sert pas ici ».** Pas de
   contradiction si « réutilisable » est défini : *une autre station, avec son propre
   récepteur et ses propres flux, sur Apple Silicon ou avec un service compatible
   `LOCAL-STT`*. Ce qui sert cette définition se garde ou s'étend (sources audio par ffmpeg,
   options d'entrée, sources ADS-B autres que tar1090) ; ce qui ne la sert pas se retire
   (chat OpenAI, simulation, couches de cartes américaines, chemin OpenAI déjà retiré par D3).
4. **D3 non exécutée.** Retirer le chemin OpenAI (1.2) n'est pas une décision nouvelle :
   c'est l'exécution de D3, en retard de trois semaines.
5. **Q7 abandonnée en silence.** La météo française n'a jamais été faite et Windy convient ;
   à fermer proprement (Q7 : « Windy gardée, Q36 »), pour que la mission ne promette plus ce
   que le plan ne fait pas.

### Décision : D71 — atc-scribe est un produit à part *(validée le 10/10)*

1. **Le dépôt reste un fork git** : historique amont, remote `upstream`, licence MIT de Yegor S,
   crédit dans le README — D19.1, D6, D7 inchangés ; `docs/` reste à eux.
2. **On ne rebase plus, jamais** ; si l'amont reprenait, on y prendrait des cerises
   (`cherry-pick`) commit par commit, pas l'inverse.
3. **Ce qui n'a pas de rôle ici se retire**, il ne dort pas : chat vocal OpenAI, simulation,
   couches américaines, chemin de transcription OpenAI (D3). Tout reste dans l'historique git.
4. **L'interface est la nôtre** : réglages, alertes, barre radio, téléphone, et ce qu'atc-scribe
   sait et ne montre pas. La disposition générale reste celle de l'amont, par choix du
   propriétaire.
5. **Le contribuable reste identifiable, et s'allonge** : en plus de la liste de D19 (sidecar
   et `local.go`, rotation Q26, `auth`, état serveur), le correctif WebSocket (0.1), les deux
   index SQLite (0.4), la fuite de contexte signalée par `go vet`, et les options d'entrée
   ffmpeg avec les options HTTP réservées aux sources HTTP (code écrit le 10/10, en réserve).
   Règle : **un correctif contribuable = un commit autonome**, qui ne dépend d'aucun de nos
   retraits, pour qu'on puisse l'offrir en l'état.
6. **« Réutilisable » veut dire** : une station avec ses propres flux (tout ce que ffmpeg lit),
   tar1090/readsb ou l'une des autres sources ADS-B, Apple Silicon pour le sidecar ou tout
   service qui honore `docs/LOCAL-STT.md`. Le README le dit en ces termes.

**Si D71 est validée, les documents changent ainsi** : `00-mission.md` reçoit un paragraphe
« Révision du 10/10 » (la phrase d'une ligne devient « … et en faire un produit public pour
une station de réception, né d'un fork de Co-ATC ») ; README : « What is different » réécrit,
« used as they are » retiré, assistant vocal « removed », une ligne sur les sources audio ;
Q11 reçoit une note « remplacée par D71 » ; Q7 est fermée ; CLAUDE.md cite D71.
**Validée le 10/10 ; les documents ont été mis à jour le même jour** (mission, README, Q11,
Q7, CLAUDE.md, D71 dans `05-decisions.md`).

### Où en est le code

Le retrait du lecteur SRT et les options d'entrée ffmpeg ont été écrits le 10/10 avant cette
mise au point, rangés en réserve git le temps de l'alignement, puis **commités après la
validation, en deux commits autonomes** comme D71.5 le demande : d'abord les options d'entrée
et les options HTTP réservées aux sources HTTP (offrable à l'amont tel quel : c'est un défaut
amont, mesuré — ffmpeg 8.0.1 s'arrête sur « Option reconnect not found » pour toute source non
HTTP), puis le retrait du lecteur SRT (le nôtre). Compilé, `go vet`, tests `-race` verts, deux
tests nouveaux. La production n'est pas relancée dessus : son binaire date du 2/10 ; le
prochain lancement prendra le nouveau, et l'instance d'essai passe avant (garde-fou 3).

## Le jugement du propriétaire (09/10)

Il est le seul utilisateur ; sa liste prime sur l'inventaire. Dans ses mots :

1. **Interface lourde et pas très moderne.**
2. **Affichage lent des déplacements d'avion** — « peut-être incontournable avec un
   affichage navigateur ». *À mesurer avant de l'admettre* : le serveur relève l'ADS-B
   toutes les secondes (`fetch_interval_seconds = 1`) et diffuse une prédiction par seconde
   (`livePredictionBroadcastInterval`), et `app.js` a un moteur d'animation avec ses propres
   compteurs (images/s, marqueurs mis à jour/s, lignes 339-343). Un navigateur sait animer
   à 60 images/s ; si c'est saccadé, c'est la chaîne relevé → diffusion → interpolation qui
   le fait, pas le navigateur. Premier relevé à faire : les compteurs du moteur sur une
   séance réelle, avec 80 à 110 avions.
3. **Les réglages sont « totalement mal foutus »** : ils méritent une **page à part,
   organisée, lisible**, plutôt qu'une modale.
4. **Les alertes en modale en haut de l'écran** ne sont « ni très intéressantes ni pratiques
   en l'état ».
5. **La disposition générale lui convient** : on ne refait pas l'agencement, on l'allège.

Ce que l'audit UX doit donc hiérarchiser : d'abord les réglages et les alertes (3, 4), puis
l'allègement (1), et une mesure pour trancher la lenteur (2) — sans toucher à la disposition
(5).

## Propositions (09/10, avant les rapports)

Le fil conducteur : **le serveur calcule déjà ce que l'interface ne montre pas.** Chaque
transmission a une preuve d'appariement (`callsign_evidence`), les clairances sont extraites
dans une table (`clearances`), les phases et les verdicts de piste sont datés, et le sidecar
archive **l'audio de chaque transmission** sur le disque externe. co-atc, pensé pour afficher
une carte et un fil de texte, n'a rien prévu pour ça. Efforts en jours de travail, estimés,
à affiner après les rapports ; « risque » = pour la production quotidienne.

### A. Montrer ce qu'on sait déjà (gain fort, effort faible)

| | Proposition | Ce qu'il faut | Effort | Risque |
|---|---|---|---|---|
| A1 | **Écouter la transmission transcrite** : un clic sur une ligne joue son audio. Pour un novice, entendre en lisant est le meilleur apprentissage ; pour nous, c'est le contrôle de qualité le plus direct. | le sidecar renvoie le nom du fichier archivé, co-atc le garde en base et le sert depuis le disque externe (désactivé si le disque manque) | 1 j | faible |
| A2 | **La fiche de vol** : l'histoire d'un avion, toutes fréquences confondues — première écoute, secteur, descente, transfert (« contact De Gaulle 126,425 » entendu), établi 27R, posé 27R à 14:32, confirmé FlightAware. Un clic sur un avion ouvre cette chronologie. | transcriptions par hex/indicatif + `phase_changes` + juge des pistes, et la détection des transferts dans le texte (fréquence citée → notre catalogue) | 1,5 j | faible |
| A3 | **La preuve de l'appariement** au survol de l'indicatif : pourquoi on croit que c'est cet avion (indicatif lu, altitude citée, position), avec un badge de confiance. Aujourd'hui l'indicatif s'affiche comme une certitude. | `callsign_source`, `callsign_evidence` déjà en base | 0,5 j | nul |
| A4 | **Les clairances en clair** sous la transmission : « Autorisation : descendre au niveau 100 » ; « Approche ILS 27R ». Le novice comprend ; l'expert vérifie l'extraction. | table `clearances`, un gabarit de phrase par type | 1 j | nul |
| A5 | **L'aide au survol partout**, comme dans la vue PAR (objet `HELP`) : chaque terme (squawk, QNH, établi, remise des gaz) expliqué en une phrase. | généraliser l'infobulle de `par-view.js` | 1 j | nul |

### B. Des outils de contrôleur, à hauteur de novice (comme la vue PAR)

| | Proposition | Ce qu'il faut | Effort | Risque |
|---|---|---|---|---|
| B1 | **La séquence d'arrivée par piste** (un AMAN simplifié) : qui atterrit ensuite, distance au seuil, heure estimée, espacement avec le précédent en NM et en secondes. À côté de la vue PAR. | juge des pistes + géométrie PAR + vitesse sol ; calcul pur | 1,5 j | nul |
| B2 | **Les strips de vol** par fréquence écoutée : une bande par avion en contact — indicatif, type, phase, dernier message, niveau autorisé (clairance), prochaine fréquence (transfert entendu). La bande passe d'une fréquence à l'autre quand le transfert est entendu ; si la fréquence est à l'antenne, l'avion y est attendu. | A2 + A4 ; le transfert est la pièce nouvelle | 2 à 3 j | faible |
| B3 | **Clairance contre trajectoire** : niveau autorisé contre altitude ADS-B, cap, vitesse — « il fait ce qu'on lui a dit ». Présenté comme information, pas comme alarme : une transcription fausse donnerait des faux écarts, et c'est justement ce que ça révèle. | A4 + ADS-B 1 Hz | 2 j | faible |
| B4 | **Vue d'approche en plan** par aéroport : axes de piste prolongés, portes à 5/10/15 NM, traînées — le complément de la PAR, qui est de profil. | géométrie PAR existante | 2 j | nul |
| B5 | **Procédures** (STAR, SID, points de report, attentes) en fond de carte, depuis l'eAIP France du SIA : rend le trafic lisible (« il suit MOPAR »). | une source de données à qualifier d'abord | inconnu | nul |

### C. L'interface, dans l'ordre du propriétaire

| | Proposition | Effort | Risque |
|---|---|---|---|
| C1 | **Page Réglages à part**, par usage : Compte et accès · Écoute (fréquences, volume, archive) · Transcription (modèles, français, seconde lecture) · Avions et pistes (station, aéroports) · Affichage · Données (rétention) · Station (liaison). Les réglages amont sans objet disparaissent ; ceux qui restent dans `config.toml` sont listés en lecture avec leur valeur. | 2 j | faible |
| C2 | **Les alertes** : plus de modale en haut ; un journal discret (dernier en tête) des événements qui comptent — remise des gaz, changement de piste en service, squawk d'urgence ou MAYDAY, type d'avion d'intérêt (A380, militaire, hélicoptère), bascule de la station, sidecar ou flux tombé — avec un son au choix par événement. Le juge des pistes fournit déjà les deux premiers. | 1 j | nul |
| C3 | **Alléger** : retirer ce que l'inventaire UX classera « inutilisé chez nous » (chat ATC, simulation, météo si elle ne sert pas, OpenAI temps réel) ; remplacer l'écran d'accueil par un seul bouton « activer le son », là où le navigateur l'impose. | 1 à 2 j | faible |
| C4 | **Téléphone** : à 390 px, transcriptions et tuiles d'abord, carte à la demande. | 1,5 j | nul |
| C5 | **Les fréquences comme un pupitre** : la tuile montre le secteur, l'aéroport, et la **qualité mesurée** (battements/min, bruit — `/radio/mesures`) ; et on choisit l'écoute depuis co-atc (`POST /radio/selection`, via la liaison), ce qu'on a fait à la main pendant deux heures le 02/10. | 2 j | moyen : ça bascule l'antenne, à cadrer (D-à-venir) |

### D. Technique

| | Proposition | Effort | Risque |
|---|---|---|---|
| D1 | **La fluidité, mesurée puis corrigée** : lire les compteurs du moteur d'animation sur une séance réelle ; puis, selon le résultat, rendu Leaflet en canvas, estime entre deux relevés (vitesse et cap, comme tar1090), réactivité Alpine bridée pour 100 avions, diffusion des différences (`change_detector`) plutôt que du lot complet chaque seconde. | 0,5 j de mesure, puis 1 à 2 j | faible |
| D2 | **Découper `app.js`** en modules, sans changement fonctionnel, par étapes qui laissent la page utilisable (plan du rapport `frontend-code.md`). | 3 j, étalés | faible |
| D3 | **Nettoyer le serveur** : paquets et sections de configuration sans objet (liste du rapport `backend.md`), `config.toml` réduit à ce qui est lu. | 1 à 2 j | faible |

### E. Fonctionnement

| | Proposition | Effort | Risque |
|---|---|---|---|
| E1 | **Relance automatique** de la production après un redémarrage du Mac (`launchd`, `caffeinate`) : la crainte de chaque absence. | 0,5 j | faible |
| E2 | **Programmation d'écoute** : un groupe par plage horaire (la nuit → `en-route`), avec retour — ce que `nuit.sh` a fait une fois à la main. Plus tard, « l'antenne sur les fréquences les plus claires » d'après les mesures : à discuter, ça déplace l'antenne seul. | 1 j | moyen |
| E3 | **L'état en un coup d'œil** dans co-atc : antenne (groupe, version, depuis quand), flux présents, sidecar (modèles, GPU), archive (fichiers, espace), bases (taille, rétention). | 1 j | nul |

### Ce que je recommande en premier

**A1, A2, B1, C1 + C2, A4** — dans cet ordre. Les trois premiers changent ce qu'on *fait* avec
atc-scribe (écouter en lisant, suivre un vol, voir qui atterrit ensuite) pour cinq jours de
travail, sans toucher à la disposition ; C1 et C2 sont les deux demandes du propriétaire ;
A4 ouvre B2 et B3. D1 se mesure dès la prochaine séance, avant de choisir. Le reste attend
les rapports.

## Consolidation des trois rapports (09/10)

Rapports bruts : `docs-fr/audit-2026-10-09/{ux,backend,frontend-code}.md` (131, 123 et 142
lignes). Tout ce qui suit est mesuré, sauf mention. Aucun serveur ne tournait pendant l'audit :
pas de mesure de charge réelle ni de fluidité sur trafic vivant.

### Ce que les trois rapports établissent

**Interface.**
- **Téléphone à 390 px : inutilisable.** Carte de 0 px de large (colonne droite fixe de
  480 px), page de 514 px coupée de 124 px, barre radio de 2 494 px sans défilement. Aucune
  règle `@media` dans `style.css`, aucun attribut `aria-`, 97 textes de 7 à 10 px.
- **Mac à 1 440 px : la barre radio déborde dès 5 fréquences** ; à 7, les pistes en service,
  la météo et l'heure sortent de l'écran. Chaque tuile fait 260 à 296 px et répète la fréquence.
- **Réglages** : un volet de 320 px, 700 lignes, 1 392 px de défilement ; 7 sections dont 5
  amont sans objet (Display, General, Debug — 39 % du défilement —, Station, Simulated) ; les
  22 réglages utiles sont en dernier. Pas de bouton de déconnexion.
- **Alertes** : 418 lignes ; 13 306 changements de phase le 04/10, dont 72 % de bruit (UNK,
  CRZ, NEW) ; l'alerte de clairance appelle `this.addAlert`, défini nulle part (à confirmer en
  console) ; le clic droit n'a pas d'équivalent tactile.
- **L'avion identifié est invisible dans 80 % des cas** : le badge `@indicatif` n'apparaît que
  sur les lignes pilote ; 345 des 1 696 transmissions appariées du 04/10 l'affichent.
- **Fond de carte par défaut** : la carte VFR américaine de la FAA (`vfr-sectional`), tuiles en
  404 sur Paris, 200 sur New York : un navigateur neuf voit une carte vide.
- **Chargement** : 5 ressources en CDN (Alpine et Tailwind sans version épinglée), 890 Ko sans
  compression ni cache (`static.go:107` : `no-store`) ; gzip ramènerait `app.js` de 278 à 56 Ko.
  **Sans internet, la page ne se charge pas.**
- **`app.js`** : 6 040 lignes, dont 5 586 dans un seul objet Alpine de 360 propriétés ; 21
  fonctions jamais appelées (346 lignes) ; 126 `console.log`, dont 21 dans le flux d'avions.
- **Retirable sans toucher à la station** : chat IA 1 225, simulation 360, fonctions mortes 330,
  couches américaines 210, clones 230 : **2 355 lignes (13,6 % du JS et du HTML)**.
- **Déplacements** : tar1090 publie à 1 Hz une position déjà âgée d'1,2 à 1,8 s ; co-atc diffuse
  un delta par avion plus une extrapolation serveur par seconde ; le navigateur extrapole encore
  (plafond 30 ips, gain 0,72, recalage lissé). Deux extrapolations empilées sur une donnée à
  1 Hz. **Le navigateur n'est pas seul en cause ; la latence de bout en bout n'est pas mesurée.**

**Serveur.**
- 35 077 lignes de Go, **32 % à nous** ; ≈ 6 500 lignes amont (18,6 %) **inactives dans notre
  configuration** (chat ATC, gabarits, simulation, chemin OpenAI, sources ADS-B hors tar1090,
  SRT) ; 1 485 lignes **mortes quoi qu'on configure** (81 fonctions, 80 amont).
- **Un gel démontré en test** : un client WebSocket qui cesse de lire (téléphone en veille,
  Tailscale qui bascule) fige `Broadcast` après ≈ 0,5 Mo, et la boucle ADS-B avec lui, jusqu'à
  la fermeture TCP par le noyau. *Jamais vu dans les journaux* : le seul trou diurne (02/10,
  13:42-13:51) est l'arrêt par le propriétaire et la relance, documentés plus haut.
- `config.toml` : 559 lignes, 151 clés, dont **46 sans effet** et 20 égales au défaut ; une
  version réduite tient en ≈ 110 lignes et 85 clés. `atc_chat.enabled = true` sans clé faisait
  afficher un bouton de chat « Disconnected » en permanence — **passé à `false` le 09/10.**
- Gains bon marché mesurés : **−14,8 % de base** en retirant deux index SQLite redondants
  (136 → 116 Mo sur une copie), **−51 % de lignes de journal** en rétrogradant deux messages.
  L'écriture SQLite (12,3 ms par cycle d'une seconde) et le JSON (2,1 µs) ne sont pas des goulots.
- Couverture de tests 26,8 % ; `websocket` 0 %, `api` 8 %, `storage` 15 %, `audio` 17 %.
- **D19 est périmé** : notre empreinte sur `app.js` est passée de 1,7 % à 8,8 %, sur
  `index.html` de 3,1 % à 17,9 %, sur `adsb/service.go` de 0,2 % à 9 %. La règle tient (modifier
  l'amont quand c'est le plus simple) ; l'argument « empreinte faible » ne tient plus.

### La liste unique, classée

Classement par gain pour l'usage quotidien, puis effort, puis risque pour la production.
Efforts en heures (h) ou jours (j), estimés par les rapports ou par moi. **État au 10/10**
indiqué en fin de ligne quand il a bougé ; chaque ligne se fait sous les sept garde-fous.

**0. Corrections — avant tout chantier (≈ 1 j)**

| # | Quoi | Effort | Risque |
|---|---|---|---|
| 0.1 | **WebSocket** : échéance d'écriture, ping/pong, verrou relâché pendant l'écriture ; le test du gel devient test de non-régression — ***fait le 10/10*** (`24b8cd0`) : le test échoue sur l'ancien code (Broadcast bloqué 10 s), passe en 0,45 s sur le nouveau ; vérifié dans un navigateur sur l'instance d'essai, 55 s, même connexion, 8 286 messages, aucune reconnexion ; la fermeture 1005 d'une page rechargée n'est plus une erreur (`c82df04`) | 4 h | moyen-faible |
| 0.2 | **Fonds de carte — urgent** (étude du 09-10/10, `docs-fr/audit-2026-10-09/fonds-de-carte.md`, recoupée sur le Mac) : **les fonds Carto `dark` et `light` sont cassés** (tuile « API KEY REQUIRED », même fichier pour toutes les tuiles : clé gratuite désormais exigée) et les fonds FAA sont vides sur Paris ; **seul `osm` marche**. Le cache du service worker masque la panne sur les zones déjà vues et y fige le filigrane. Aucun fond sombre testé ne rend les pistes lisibles (Carto vectoriel : contraste 1,02:1). Proposition à arbitrer : fond **Plan IGN v2** (Licence Ouverte, sans clé, pistes et taxiways nets dès z13) ou OSM, **assombri par filtre CSS** sur sa seule couche ; **les pistes, taxiways et aires dessinés par nous** (OurAirports, domaine public ; OSM Overpass, ODbL) en couche vive, lisible sur tout fond ; **la VAC du SIA en lien** calculé sur le cycle AIRAC (réutilisation permise si non altérée, source et date citées) ; espaces aériens plus tard (Open Flightmaps sans clé jusqu'à z12, ou openAIP avec clé, CC BY-NC). Écartés : SCAN OACI de l'IGN (interdit aux particuliers), proxy ADS-B Exchange (aucune autorisation). **Validée par le propriétaire le 10/10 et confiée à la fiche B** (étapes 6 à 8 : fonds IGN et OSM assombris, défaut selon le pays de la station, Carto retiré, cache du service worker vidé ; plan des aéroports généré par station depuis Overpass, non commité ; lien VAC dans la PAR) ; le réglage à l'œil se fera sur le Mac | 2 h + 1 j | nul |
| 0.3 | Deux messages de journal en `debug` ; fuite de contexte signalée par `go vet` ; `go mod tidy` — ***fait le 10/10*** (`4978ed1`, `0bf7f6b`, `e9c336b`) : 48 % des lignes du 04/10 ; `go vet` propre sur tout le dépôt | 1 h | nul |
| 0.4 | Deux index SQLite redondants, après `EXPLAIN QUERY PLAN` (−14,8 % de base) — ***fait le 10/10*** (`7a17b4c`) : plans identiques avant/après sur les sept requêtes ; base du 01/10 compactée 129 → 110 Mo ; supprimés aussi à l'ouverture d'une base qui les a ; vérifié sur la base de l'instance d'essai | 1 h | faible |
| 0.5 | Bouton **Se déconnecter** (aucun aujourd'hui) — ***fait le 10/10*** (`3f183cb`) : en tête du volet de réglages ; vérifié à 1 440 × 900, cliquable, session fermée côté serveur | 0,5 h | nul |
| 0.6 | `atc_chat.enabled = false` | fait | — |

**1. Retirer ce qui ne sert pas ici (≈ 2 à 3 j)**

| # | Quoi | Effort | Risque |
|---|---|---|---|
| 1.1 | Interface : chat IA, simulation, fonctions mortes, couches américaines, clones (étapes 1 à 5 du plan `frontend-code.md`, −2 355 lignes) ; volets Debug, Station, Simulated | 9 h | faible |
| 1.2 | Serveur : 81 fonctions mortes, chemin OpenAI (D3 le dit déjà retiré, −1 990 lignes) | 10 h | nul-faible |
| 1.3 | Serveur : chat ATC et gabarits (−3 250 lignes) — *tranché le 10/10 : on retire* | 8 h | faible-moyen |
| 1.4 | Serveur : simulation (−375 lignes) — *tranché le 10/10 (D71) : les sources ADS-B hors tar1090 sont **gardées** (réutilisable) ; le lecteur SRT est **retiré** (fait, `e9c336b`, −208 lignes, −9 modules), remplacé par les options d'entrée ffmpeg (`1858fc6`)* | 2 h | faible |
| 1.5 | Accueil : plus d'écran bloquant ; un bandeau « reprendre l'écoute » au premier clic (le seul geste que le navigateur exige) — ***fait le 10/10*** : l'écran (84 lignes, 29 de CSS, son d'accueil) est retiré ; « Click anywhere to resume listening » s'affiche tant qu'une écoute mémorisée attend, et le premier clic ou la première touche la relance. Vérifié sur l'instance d'essai, après une connexion : bandeau visible, puis au clic sur la carte la fréquence mémorisée joue et l'autre reste coupée. La frappe du mot de passe ne compte pas (la page s'initialise après la connexion) : il faut un clic | 2 h | nul |
| 1.6 | ~~Relance automatique (`launchd`)~~ — *refusée le 10/10 : la production ne doit pas tourner en permanence sur le Mac.* **Remplacée par un lanceur dans la barre de menus** : une icône qui dit si la production tourne (et l'instance d'essai) ; Démarrer, Arrêter ; ouvrir co-atc (local, Tailscale) ; antenne et sidecar en une ligne. Application SwiftUI native (`MenuBarExtra`), sans dépendance tierce : Xcode et Swift 6.4 sont sur le Mac (vérifié). Elle reprend la logique de `runs/production.command` (mot de passe rangé, liaison station, arrêt propre) et inscrit/retire la ligne du planning du GPU. — ***fait le 10/10*** (`7a65ab1`, `tools/menubar/`) : installé dans `~/Applications/atc-scribe.app`, réglages dans `~/.config/atc-scribe/launcher.json` ; il lance le script de production existant et l'arrête par SIGTERM ; état (depuis quand, sidecar, antenne), ouvrir la page, copier l'adresse Tailscale, journal ; tient la ligne du planning du GPU. Vérifié par son mode `--start`/`--stop` (même code que les boutons) : démarrage en 11 s, arrêt propre en 9 s, ligne du planning écrite puis retirée. Le menu lui-même est à essayer par le propriétaire (l'outil de contrôle de l'écran ne voit pas les applications de barre de menus) | 1 j | faible |

**2. Les deux demandes du propriétaire (≈ 2 j)**

| # | Quoi | Effort | Risque |
|---|---|---|---|
| 2.1 | **Page Réglages** à part, 5 sections (Écoute · Appariement avec préréglage Mesuré / Prudent / Expérimental · Affichage « ce navigateur » · Compte · Système en lecture) ; supprime le volet de 700 lignes — ***fait le 10/10*** : page plein écran (roue dentée ; Échap ou croix pour fermer ; la barre radio reste utilisable), cinq rubriques Listening · Callsign matching · Display · Account · System, le diagnostic de performance replié en bas de System ; contrôles déplacés tels quels, liaisons inchangées ; la surcharge de position de la station par GPS est retirée de la page (son code JS reste, à retirer avec le découpage d'`app.js`). Les préréglages Mesuré / Prudent / Expérimental ne sont pas faits : il faudrait d'abord fixer leurs valeurs | 12 h | faible |
| 2.2 | **Alertes** : supprimer la barre (418 lignes, 72 % de bruit, clairance cassée), puis un **journal discret des événements qui comptent** (remise des gaz, changement de piste, MAYDAY ou squawk d'urgence, type d'intérêt, bascule de la station, sidecar ou flux tombé), son au choix par événement — ***fait le 10/10*** (`7690d41`, `www/events.js`) : la barre et ses fonctions retirées (−360 lignes, avec l'appel cassé de la clairance) ; une cloche à côté de la roue dentée ouvre le journal (100 événements, badge des non-lus). Événements : squawk 7500/7600/7700, remise des gaz (le juge de piste la rapporte désormais, `cc47f6a`), changement de piste en service tenu 2 min, type rare ou très gros (A388, A124, B748…), bascule de canaux de la station, flux audio perdu ou revenu ; un clic montre l'avion ; son par type, l'urgence seule par défaut. Au passage, **le squawk n'arrivait pas dans les mises à jour en direct** (`a322f8f`, correctif contribuable) : un 7700 déclaré en vol ne serait jamais apparu. Vérifié sur l'instance d'essai (urgence simulée sur un avion réel, bascule de piste simulée, PAR sous les boutons). **Pas fait** : « sidecar tombé » (la page ne suit pas encore son état) et MAYDAY entendu à la radio (relève du palier 3, avec la transcription) | 1,5 h + 1 j | nul |
| 2.3 | **Filtre d'altitude** (demande du propriétaire, 10/10 : « un curseur intelligent, marge haute et basse, gestion des flight levels ») — ***fait le 10/10*** (`633637d`, `www/altitude/`) : plancher et plafond sur un seul curseur, la liste et la carte suivent pendant le glissé ; échelle non linéaire (pas de 100 ft jusqu'à l'altitude de transition, soit la moitié de la course ; 500 ft jusqu'au FL195 ; FL par dix jusqu'au FL450 ; puis « sans plafond ») ; pieds jusqu'à la TA, niveaux de vol au-dessus ; histogramme du trafic du moment sur le curseur, compteur « n of m airborne », préréglages (≤ 3000 ft, ≤ TA, ≤ FL100, FL100–195, ≥ FL195). Les avions au sol restent sous le bouton Ground. **Altitudes lues comme un contrôleur les dit** (11/10, `075a089`, à la demande du propriétaire) : pieds sur le QNH jusqu'à la TA, niveau de vol au-dessus, GND au sol, dans la liste, la fiche, la trajectoire, les étiquettes de la carte et les deux filtres ; QNH = médiane des calages ADS-B (`nav_qnh`) des avions à moins de 40 NM sous la TA, l'estimation de la vue PAR, recalculée toutes les 30 s (mesuré le 11/10 : 1015,2 hPa sur 11 à 12 avions) ; standard tant qu'aucun n'en envoie. **Aides contextuelles** (même demande) : `www/help-tips.js` étend la bulle de la vue PAR à toute la page (attribut `data-help`) ; posées sur l'altitude de chaque avion, le QNH, la TA, l'échelle, les préréglages et compteurs du curseur, et le « ALT » à côté de la piste en service (piste alternative, pas une altitude). À étendre à chaque palier. Côté serveur, `min_altitude`/`max_altitude` de `/aircraft` filtrent enfin (`56c3404`, défaut 1 de la fiche F, correctif contribuable) | 1 j | faible |

**3. Montrer ce que le serveur sait déjà (≈ 6 à 7 j)**

| # | Quoi | Effort | Risque |
|---|---|---|---|
| 3.1 | **Avion identifié sur 100 % des lignes appariées** (20 % aujourd'hui) ; corps de texte ≥ 11 px — ***fait le 11/10*** (`e2d0629`) : dans le fil par fréquence, toute ligne appariée nomme son avion (« ATC → AFR123 » quand le contrôleur lui parle), cliquable tant qu'il est à portée, grisé sinon ; bulles d'aide sur le locuteur, l'avion et la lecture FR ; en-tête de ligne 9 → 11 px (le corps était déjà à 12 px). Mesuré sur la base du 04/10 : 1 696 lignes appariées, 345 affichaient l'avion (20 %), 1 696 maintenant. Vérifié sur l'instance d'essai avec des transmissions injectées dans la page | 2 h | nul |
| 3.2 | **Écouter la transmission transcrite** (A1) : le sidecar renvoie le fichier archivé, co-atc le sert — ***fait le 11/10*** (`7d333be`, `6aa6d9c`) **sans toucher au sidecar** : la clé (fréquence, `created_at` comparés en instants) existait déjà dans les manifestes quotidiens de l'archive ; `internal/audioarchive` les indexe et les relit par incréments. `GET /api/v1/transcriptions/{id}/audio` (WAV, plages, derrière la connexion, jamais hors de l'archive, 404 sinon) et `GET /api/v1/audio-archive` (disque branché ou non) ; dossier = `--save-audio` du sidecar, ou `audio_archive_dir`. Bouton lecture sur chaque ligne du fil et de la section Radio, un extrait à la fois, le direct baissé à 20 % pendant la réécoute. **Mesuré** : 8 322 transmissions sur 8 322 du 03/10 et 5 512 sur 5 512 du 04/10 retrouvent leur audio. Vérifié sur l'instance d'essai avec les 25 transmissions du 10/10 copiées dans sa base (lecture, arrêt, durée 6,26 s égale au manifeste, cas sans audio, atténuation) | 1 j | faible |
| 3.3 | **Fiche de vol** (A2) : chronologie d'un avion toutes fréquences, transferts entendus, verdicts de piste — ***fait le 11/10*** (`8e7b8c4`, `www/timeline/`) : onglet TIMELINE du panneau avion (DETAILS reste l'onglet par défaut), du plus ancien au plus récent : première détection, phases (NEW et UNK omises), pistes du juge, remise des gaz, chaque transmission appariée sur toutes les fréquences avec sa réécoute et ses valeurs, transferts (fréquence donnée par le contrôleur, extraite par la grammaire : 947 le 03/10, nommée si la station l'écoute), clairances, signal perdu ; heures d'un autre jour datées. Sans code serveur. **Non fait** : la confirmation FlightAware (quota AeroAPI). Vérifié sur l'instance d'essai avec les transmissions réelles d'un vol du 10/10 | 1,5 j | faible |
| 3.4 | **Séquence d'arrivée par piste** (B1), à côté de la PAR — ***fait le 11/10*** (`90351b6`) : sous chaque cadre de la vue PAR, les avions entrant dans le cône de l'extrémité d'arrivée, du plus proche du seuil au plus loin : distance au seuil (décalé compris), temps à la vitesse sol du moment et heure estimée, écart avec le précédent en NM et en secondes, en ambre sous 3 NM ; l'autre extrémité aussi quand des avions y arrivent (piste en service inconnue, face-à-face). Fonction pure `arrivalSequence` dans `par-geometry.js`, deux tests. Vérifié sur l'instance d'essai (LFPG et LFPO ajoutés à ses aérodromes suivis) : AFR1061 à 7,3 NM de la 26L, 2:32 ; DAL264 à 9,0 NM de la 27R | 1,5 j | nul |
| 3.5 | **Clairances en clair** (A4) sous la transmission — ***fait le 11/10*** (`7ac7930`, `2a8eb72`) : plus large que les clairances, toute consigne reconnue par la grammaire est dite en clair (« Descend to FL100 », « Turn left heading 270 », « Contact 119.255 », « QNH 1015 », « Cleared to land, runway 27R ») sous chaque transmission du fil, de la section Radio et de la chronologie ; vert pour une clairance, ambre pour le reste, bulle d'aide. `Result.Plain()` dans la grammaire (verbe dans l'expression ou les trois mots avant, montée/descente avant maintien, verbes français, « left » après un nombre = côté de piste) ; champ `plain` sur chaque transmission servie, relu à la lecture (**17 µs par transmission**, mesuré sur les 5 512 du 04/10, dont **36 %** disent quelque chose), et dans la mise à jour en direct. Défauts trouvés sur données réelles et corrigés avant commit : « remontez » non reconnu, côté de piste pris pour un virage, cap doublé, zéro en tête | 1 j | nul |
| 3.6 | **Bandeau d'état** (E3) : antenne (sélection, depuis quand, mode), sidecar (statut et `degraded`), ADS-B, « récepteur prêté ou éteint » au lieu d'une barre vide — ***fait le 11/10*** (`28be23b`, `3bc36f2`, `1a1822d`) : `radio-ctl-sync` rapporte à chaque lecture l'état de la station à co-atc (`PUT /api/v1/receiver` : chaine → on_air, exploration → lent, transition → switching, arret → off, avec le libellé, la sélection et « depuis » de la station) ; co-atc le garde en mémoire, périmé après une minute sans rapport. `GET /api/v1/status` réunit antenne, sidecar (relu au plus toutes les 15 s), ADS-B (avions actifs) et archive audio ; il remplace `/audio-archive`. Trois voyants avant les horloges (Antenna, Transcription, ADS-B) avec bulles d'aide ; message à la place de la barre vide quand le récepteur est prêté, éteint ou en bascule. Générique : sans rapporteur, le voyant antenne dit « ? » et pourquoi. Vérifié sur l'instance d'essai avec la vraie station (lecture seule) : « On air · A l antenne · Selection libre - 124,350 + 126,425 · since 09/10 », 71 avions ; récepteur prêté simulé. **Binaire `bin/radio-ctl-sync` reconstruit** (ancien : `runs/radio-ctl-sync.avant-etat`) | 7 h | faible |
| 3.7 | Preuve de l'appariement au survol (A3) ; aide au survol partout (A5) — ***fait le 11/10*** (`266e324`, `aee93d9`, puis les bulles) : **D72** (validée par le propriétaire) : score et raisons du matcher gardés avec la transmission (`match_score`, `match_reason`, écrits dans la même requête que l'indicatif ; « in sector » ajouté quand la recherche est restreinte au secteur). À côté de l'avion apparié, trois barres disent la **force des indices** (forte : indicatif exact + un autre indice concordant ; moyenne : un indice, ou un indicatif partiel soutenu ; faible : indice partiel ou contradiction) et la bulle dit pourquoi en clair, avec le score ; c'est un compte d'indices, **pas une probabilité mesurée**, la bulle le dit. Dans le fil, la section Radio et la chronologie ; rien pour les transmissions d'avant. Bulles d'aide ajoutées sur les filtres de phase, cap (m/t/trk), TAS, GS, distance, Air/Ground, « Heard on the radio », Active/Signal Lost, préréglages, piste en service : **408 éléments** de la page portent une aide (compté sur l'instance d'essai). **Suite** : mesurer la précision par niveau d'indices sur le jeu annoté du labo | 1,5 j | nul |

**4. Lisibilité et accès (≈ 2,5 j)**

| # | Quoi | Effort | Risque |
|---|---|---|---|
| 4.1 | **Barre radio** : 7 tuiles visibles à 1 440 px (≈ 150 px la tuile, sans fréquence répétée, défilable) ; pistes en service et heure fixées à droite ; « tout écouter / tout couper » ; volume par fréquence — ***fait le 11/10*** (`47da917`) : tuiles de 160 px (nom seul quand il commence par la fréquence), bande défilable à la molette, reste de la barre à largeur fixe ; bouton « all » (écouter tout / couper tout) ; curseur de volume par tuile, mémorisé par le navigateur ; le fil de transcriptions garde 320 px. **Mesuré à 1 440 px, 7 fréquences, 3 aérodromes suivis : barre de 1 440 px (plus de débordement), 5 tuiles visibles, les 2 autres à un coup de molette** — l'objectif de 7 visibles n'est pas atteint : les voyants d'état (3.6) et les pistes de trois aérodromes prennent ≈ 590 px à droite ; avec deux fréquences et un aérodrome, tout tient sans défiler | 5 h | nul |
| 4.2 | **Téléphone** (sous 768 px) : colonne pleine largeur, carte / PAR / liste en onglets, barre radio défilable, fiche avec bouton fermer, aide au toucher — ***fait le 11/10*** (`ebd74d0`) : trois onglets Map / PAR / List au-dessus de la barre radio, un volet à la fois ; la sélection d'un avion sur la carte ouvre sa fiche dans le volet liste, bouton « Back to the list » ; cap et TAS cachés sur téléphone ; barre radio défilable ; appui long = bulle d'aide. **Mesuré à 375 px** (émulation) : document 375 px (avant : la colonne de 480 px forçait 563), lignes 349 px sans chevauchement, carte 375 px. Non essayé sur un vrai téléphone | 10 h | nul |
| 4.3 | **Chargement** : dépendances servies par co-atc et épinglées (plus d'hôte externe), Tailwind compilé, gzip, scripts en `defer` ; **mesure de latence de bout en bout** (instant d'observation contre instant de dessin) avant toute décision sur le moteur d'animation — ***en partie fait le 11/10*** (`d52592a`). **Mesuré avant** : 1,76 Mo par chargement, dont 1,2 Mo non compressés servis par co-atc (plan d'aérodrome 871 Ko en `text/plain`, `app.js` 251 Ko), et tout rechargé à chaque F5 (`no-store`). **Fait** : gzip sur le texte (app.js 56 Ko, plan 187 Ko en `application/geo+json` ; audio et WebSocket hors des types, `Flush`/`Hijack` transmis) ; `no-cache` au lieu de `no-store` (304 si inchangé : **un rechargement transfère 8 Ko**) ; tous les scripts en `defer`, Alpine en dernier et **épinglé 3.14.9** (était `3.x.x`). **Latence mesurée** (30 s, 709 positions) : observation → réception p50 325 ms, p90 480, p99 592 (sondage tar1090 à 1 s) ; réception → premier dessin p50 54 ms, p90 97. **Décision** : le moteur d'animation n'est pas le poste de latence, on le garde. **Terminé le 11/10** (`7e6991a`, accord du propriétaire pour les téléchargements) : OpenLayers, Font Awesome (CSS + polices solid et regular) et Alpine dans `www/vendor/`, servis par co-atc, **plus aucun hôte externe** ; Tailwind compilé par `tools/tailwind/build.sh` (31 Ko au lieu du compilateur CDN de 400 Ko), placé après `style.css` comme le CDN l'injectait. **Vérifié** : sur les 280 combinaisons balise + classes de la page (réglages ouverts), tous les styles calculés sont identiques au CDN ; l'unique écart trouvé (l'ombre d'un bouton) venait de l'ordre des feuilles et a été reproduit. Le filet `www-check` recalcule l'empreinte des sources inscrite en tête du CSS compilé : une classe ajoutée sans recompilation est signalée. Au second chargement : 11 Ko transférés. **Réactivité, mesurée le 12/10** (fil principal, boucle `MessageChannel`, 30 s, 130 à 157 avions, même `app.js`) : **CDN Tailwind : 92 % occupé (916 ms/s), 122 blocages > 50 ms, le plus long 573 ms ; compilé : 24 à 26 % (241 à 257 ms/s sur deux passes), 17 à 24 blocages, le plus long 91 ms.** Le compilateur du CDN re-scannait le DOM à chaque mise à jour d'avion : c'est lui que le propriétaire a senti partir (« plus réactive, même avec beaucoup d'avions ») | 6 h | faible |

**5. Structure, sans changement fonctionnel (≈ 4 j, étalés)**

| # | Quoi | Effort | Risque |
|---|---|---|---|
| 5.1 | `app.js` : étapes 6 à 10 du plan (audio, station, fiche avion, flux) ; l'étape 11 (réglages, alertes) se fait avec 2.1 et 2.2 — ***fait le 12/10*** (`961a156`) : **5 139 → 910 lignes**. Six domaines dans `www/store/` (audio 402, transcripts 553, station 526, aircraft-details 1 120, feed 755, sidebar 645), chacun une fabrique poussée dans `window.AtcStore` qu'`app.js` appelle dans `alpine:init` (même instant d'évaluation que le littéral unique : les valeurs initiales lisent `generateUUID`, `API_BASE_URL`…, ce que le bac à sable a attrapé au premier essai) ; assemblage par descripteurs (les 8 accesseurs restent des accesseurs). Cache de tuiles → `tile-cache.js`, `serverSettings()` → `server-settings.js`. Déplacé à l'identique par script (reconstruction du littéral vérifiée à l'octet). **Preuve** : `tools/www-check/store-dump.mjs` avant/après, 343 entrées, 339 identiques, les 4 autres sont l'heure et l'identifiant de client ; filet vert (1 109 `this.X`) ; parcours complet sur l'instance d'essai ; **chemin chaud** : fil principal occupé 17 % avec 106 avions (24 à 26 % le matin même avec 130 à 157, avant). En passant : l'étape 4 de E fusionnée sous test (`743c995`), la surcharge GPS retirée partout (`12d502a`) | 19 h | faible, chemin chaud à mesurer |
| 5.2 | `config.toml` réduit (559 → ≈ 110 lignes) avec défauts en code et détection des clés inconnues | 3 h | moyen-faible |
| 5.3 | Découper `main()` (463 lignes) et `handlers.go` (2 299) — ***fait le 10/10*** par la fiche E (`bc311b7`) : `main.go` 112 lignes, `handlers.go` 166 plus six fichiers par domaine ; la fusion des branches décollage/atterrissage d'`adsb/service.go` (étape 4) n'est pas faite, faute de test | 6 h | moyen, gain nul à l'exécution |
| 5.4 | Tests sur ce qui porte la production sans filet : websocket, api, storage, audio | continu | — |

**Plus tard, à discuter** : strips de vol (B2), clairance contre trajectoire (B3), vue d'approche
en plan (B4), procédures du SIA (B5), les fréquences comme un pupitre (C5, ça bascule l'antenne),
programmation d'écoute par plage horaire (E2).

### Ce que le propriétaire doit trancher

| # | Arbitrage | État |
|---|---|---|
| 1 | Retirer le chat ATC (« AI Advisory ») et la simulation | **tranché le 10/10 : oui** — c'est l'assistant vocal OpenAI, pas les transcriptions par fréquence |
| 2 | Sources ADS-B hors tar1090, et SRT | **tranché le 10/10 (D71)** : sources ADS-B gardées ; lecteur SRT retiré, ffmpeg lit SRT ; options d'entrée ajoutées (UDP brut, carte son) |
| 3 | Langue de l'interface | **tranché le 10/10 : anglais** ; l'interprétation d'une communication suit sa langue ; version en/fr plus tard |
| 4 | Fond de carte par défaut | **en cours** : préférence pour le sombre, et des fonds aéronautiques avec pistes et plans d'aéroport ; étude des sources et des licences en cours (VAC du SIA comprises) |
| 5 | Ordre des paliers | **tranché le 10/10 : 0, 1, 2 puis 3** ; 4 et 5 ensuite, 5.4 en continu. Avec une exception : pas de relance automatique (1.6), un lanceur dans la barre de menus à la place |

Le détail de chaque arbitrage, tel qu'il a été instruit, est dans les sections précédentes et
dans D71 (`05-decisions.md`).

### Retour de la vague 1 (10/10)

Intégrée dans `local` le 10/10 après relecture et vérification sur le Mac (branche d'essai
`integration/vague1`). Ensemble : **+3 983 / −10 342 lignes** ; Go hors tests 35 077 → 27 498,
interface (JS + HTML) 13 036 → 10 881. Rapports : `docs-fr/fiches-cloud/{A,B,C}-rapport.md`.

- **Conflits de sens** entre C (écrit avant A) et A : trois tests de C appelaient des fonctions
  retirées par A ; adaptés. Un test de C utilisait `t.Chdir` (Go 1.24, le module déclare 1.23) :
  réécrit avec `os.Chdir`, il a révélé **un défaut propre au Mac** — la rétention ne reconnaissait
  la base du jour que par son nom ; à travers un lien symbolique (`/var` → `/private/var`), elle
  la prenait pour ancienne et la supprimait au-delà du plafond. Corrigé : reconnue aussi par
  identité de fichier (`os.SameFile`). En production les chemins sont relatifs : pas d'incident.
- **Vérifié sur le Mac** : ta configuration réelle se charge et se valide ; l'instance d'essai
  (faux sidecar `runs/son-test/fake-sidecar.py`, sans GPU) démarre sans aucune ligne chat,
  simulation ni prompt ; dans le navigateur à 1 440 × 900 : fond `ign-dark` choisi seul (et une
  préférence pointant un fond retiré y retombe), plan des aéroports (891 Ko, 35 s d'Overpass ;
  88 pistes, 1 818 taxiways, 289 aires, 71 terminaux, 117 hélistations) lisible à z13 et z15,
  lien VAC de la PAR au cycle AIRAC du 01/10 (PDF du SIA en 200), son, coupure, réglages,
  déconnexion.
- **Tuiles IGN** : le serveur refuse environ une tuile sur vingt (400 « layer unknown »), par
  périodes qui peuvent tenir une même tuile plusieurs secondes ; sous le filtre sombre, un trou
  noir. Réessai ajouté (7 essais espacés de 0,5 à 16 s, ~30 s). Pas une limite de requêtes
  simultanées (25 requêtes parallèles toutes servies).
- **Défauts de C** : n°3 (l'auditeur qui revient repartait jusqu'à 87 s derrière le direct) et
  n°4 (course sur le compteur de ports) **corrigés** ; vérifiés par leurs tests (échec sur
  l'ancien code) et, pour le n°3, sur l'instance d'essai (retour après 5 s et 40 s : débit du
  direct). **N°1 et n°2 à arbitrer** (changent ce qui est stocké) :
  - n°1 : une colonne vide (`tas`, position…) empêche la contrainte d'unicité de reconnaître une
    position répétée ; **mesuré sur la base du 04/10 : 923 816 doublons exacts sur 2 328 654
    positions (40 %)**. Corriger réduirait les bases d'environ 40 %, mais change ce que lisent la
    PAR, le juge des pistes, `cmd/phraseology` et le laboratoire (un état répété n'est plus
    réécrit chaque seconde) ;
  - n°2 : les transcriptions sont datées en heure locale avec décalage, les positions en UTC ;
    les comparaisons de texte se trompent d'une à deux heures et la nuit du changement d'heure
    (25/10) trie à l'envers. Corriger change le format lu par les scripts du laboratoire :
    à coordonner avec Whisper-lab.
- **Les deux défauts de stockage, corrigés le 10/10** (`7db6f42`, `91a3553`), chacun avec son
  test qui échoue avant et passe après : n°1, une position déjà stockée n'est plus réécrite
  même quand une colonne est vide (recherche par `IS`, servie par l'index de la contrainte :
  plan vérifié) — effet attendu : environ −40 % de lignes de positions, à mesurer sur la
  prochaine journée ; n°2, les transcriptions, clairances et valeurs de phraséologie sont
  datées en UTC comme les positions, les anciennes lignes converties une fois à l'ouverture
  du fichier (vérifié sur une copie de la base du 10/10 : 25 lignes `+02:00` → `Z`, ordre
  inchangé), l'en-tête `X-Created-At` du sidecar et les bornes de `cmd/phraseology` suivent.
  **Whisper-lab n'est pas encore prévenu** (session fermée) : `created_at` se termine désormais
  par `Z`, et `phraseology_values.created_at` passe du format Go par défaut à RFC3339.
- **Fond de carte** : « None » ajouté (plan des aéroports et avions sur noir) et **choisi par
  défaut** le 10/10, jugement du propriétaire ; IGN et OSM, clairs ou sombres, restent à un clic.
- **À juger à l'œil par le propriétaire** : le filtre d'assombrissement
  (`style.css`, `.basemap-dark .atc-basemap`) et les couleurs du plan
  (`AIRPORT_LAYOUT_COLORS`, en tête de `openlayers-map-manager.js`).
- Binaire de production reconstruit ; l'ancien est gardé (`runs/co-atc.avant-vague1`).

### Retour de la vague 2, fiches D et F (10/10)

Intégrées dans `local` le 10/10 sans conflit ; `go vet` propre, 15 paquets verts avec `-race`.
- **D, configuration réduite** : l'exemple passe de 538 à 156 lignes et de 150 à 57 clés ;
  défauts en code pour le reste ; une clé inconnue est nommée au démarrage, jamais refusée.
  **Vérifié sur la configuration réelle du propriétaire** : chargée par le code d'avant et par
  celui d'après, elle ne diffère que par trois réglages d'authentification qui passent de 0 à
  720 h, 8 essais et 15 min, exactement les valeurs que l'ancien code appliquait quand ils
  valaient 0 (`orDefault`, `max <= 0`) : aucun changement de comportement. Elle produisait 23
  avertissements (chat, clés OpenAI, deux doublons) ; **nettoyée le 10/10** (58 lignes retirées,
  copie `runs/config.toml.avant-vague2`) : même configuration chargée, aucun avertissement.
- **F, tests de l'API** : couverture 10,9 % → 69,9 % ; 29 routes testées en 401 sans session.
  **Défaut** : `/aircraft` reçoit `min_altitude` et `max_altitude` et ne s'en sert pas ;
  aucun client du dépôt ne les envoie. *Tranché le 10/10 : corrigé plutôt que retiré, l'API les
  documente (`docs/api_spec.md`) ; fait (`56c3404`).* Les filtres de dates (`took_off_after`,
  `took_off_before`, `landed_after`, `landed_before`), même fonction, TODO de l'amont : **faits le
  11/10** (`45001d9`) ; un avion compte par n'importe lequel de ses décollages ou atterrissages dans
  la fenêtre (tours de piste), bornes dans tout fuseau.
- **Observations de F traitées le 11/10** : une référence de proximité introuvable
  (`distance_nm` + `ref_hex`/`ref_flight` inconnus) répond 404 au lieu de tout le ciel, et 400 sans
  référence (`2344a7b`). Laissées : `Access-Control-Allow-Methods` sans `PUT`/`HEAD` (sans effet tant
  qu'aucun client d'une autre origine n'appelle l'API), `transcriptions: null` (sans effet), hex
  français sans indicatif (amont). Mesuré sur le Mac : `internal/api` 71,7 % de couverture, 11 s.
- Instance d'essai démarrée sur cet état : santé et page en 200. Binaire de production
  reconstruit, ancien gardé (`runs/co-atc.avant-vague2`).
- **La fiche E** (découpage de `main()` et de `handlers.go`) peut partir : D est intégrée. Elle
  doit garder la boucle des avertissements de configuration dans `main.go`.

### Retour de la fiche E (10/10)

Intégrée le 10/10 (`bc311b7`), sans conflit avec le palier 2 fait entre-temps. `main()` tient
en une page (`main.go` 695 → 112 lignes, le reste dans `startup.go`, `serve.go`,
`retention.go`, `add_user.go`) ; `handlers.go` 1 636 → 166 lignes, six fichiers par domaine,
`GetAllAircraft` en dix fonctions privées. **Vérifié sur le Mac** :
- la liste des routes figée (`internal/api/testdata/routes.txt`, 45 lignes) passe ;
- le banc de démarrage (`tools/startup-bench/`, désormais dans le dépôt) rejoué sur `local`
  puis sur la fusion, deux passes : mêmes lignes de `main()` (34), même ordre de construction
  (28 étapes), mêmes messages (87) ;
- un démarrage sur la configuration de l'instance d'essai, avant puis après : mêmes 50 lignes de
  `main()`, même ordre, mêmes 88 messages ; le seul écart vu (un message de décollage écarté)
  dépend du trafic et apparaît aussi entre deux démarrages du même binaire ;
- `go test -race ./...` vert ; la boucle des avertissements de configuration est restée dans
  `main()`, avant toute construction.

**Laissé** : l'étape 4, la duplication décollage/atterrissage de
`sendImmediateGroundTransitionAlerts` (≈ 30 lignes), qu'aucun test ne couvre. À reprendre avec
un test d'abord, ou à laisser : son seul effet visible est l'effet de décollage/atterrissage
sur la carte.

## État

- 12/10 : **première séance du propriétaire sur la version des paliers 0 à 4** (journal
  d'événements, réglages, curseur d'altitude en QNH/FL, réécoute, fiche de vol, séquence
  d'arrivée, consignes en clair, bandeau d'état, barre radio, bibliothèques locales) :
  « une NETTE amélioration UX par rapport à co-atc de base ». Garde-fou 5 (validation à
  l'usage) tenu pour ces paliers ; aucun défaut remonté à cette séance.

- 09/10 : cadre posé ; trois études menées en parallèle (Sonnet), rapports dans
  `docs-fr/audit-2026-10-09/` ; jugement du propriétaire, propositions et **consolidation**
  inscrits. `atc_chat.enabled` passé à `false`. D19 et CLAUDE.md corrigés sur l'empreinte.
- 10/10 : chat « AI Advisory » retiré (arbitrage 1), interface en anglais (arbitrage 3),
  règle « ne rien casser » (sept garde-fous), question des sources audio instruite ;
  **alignement avec les textes fondateurs, D71 validée**, documents mis à jour ; premier
  code sous D71 : options d'entrée ffmpeg, puis retrait du lecteur SRT. Ordre tranché (0, 1,
  2 puis 3), lanceur de barre de menus au lieu de la relance automatique.
- 10/10 : **palier 0 fait, sauf 0.2** (fond de carte, étude en cours) : WebSocket, journaux,
  index, déconnexion, chacun dans son commit, vérifiés sur l'instance d'essai. `bin/co-atc`
  reconstruit ; l'ancien est gardé (`runs/co-atc.avant-2026-10-09`, binaire du 02/10) pour
  revenir en une minute. La production ne tourne pas : le prochain lancement prendra le
  nouveau, et la validation à l'usage (garde-fou 5) se fera à cette séance.
- 10/10 : **le crédit cloud du propriétaire (expire le 5/11) est mis au service du chantier.**
  Ce qui se vérifie sans la station part au cloud par fiches autonomes
  (`docs-fr/fiches-cloud/`) ; chaque branche revient sur le Mac, où elle est relue et vérifiée
  (garde-fous) avant d'être intégrée. Vague 1, en parallèle : A, retraits côté serveur (1.2,
  1.3, 1.4) ; B, retraits côté interface (1.1, étapes 0 à 5 du plan) ; C, tests de storage,
  audio et frequencies (5.4). Vague 2 après intégration : configuration réduite (5.2),
  découpage (5.3), tests de l'API. Restent sur le Mac : l'accueil (1.5), le lanceur (1.6),
  les réglages et les alertes (palier 2), tout ce qui demande la station ou le navigateur.
