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
dans `runs/audit-2026-10-09/` (hors dépôt) :

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

- **Le serveur tourne tous les jours.** Chaque étape doit laisser la production utilisable ;
  rien de « grand soir ».
- **Garder identifiable ce qui est contribuable** (CLAUDE.md) : sidecar, correctif de
  rotation (Q26), authentification, état serveur — même si on ne rebase plus.
- **L'UX se juge sur l'usage réel** : une personne, tous les jours, Mac et téléphone par
  Tailscale, 1 à 7 fréquences. Pas sur ce que l'interface amont permet.
- **Mesurer avant de conclure** : chaque suppression proposée cite le fichier et le nombre
  de lignes, chaque friction cite le geste et le nombre de clics.

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

## État

- 09/10 : cadre posé ; trois études lancées en parallèle, rapports attendus dans
  `runs/audit-2026-10-09/`. Jugement du propriétaire et propositions inscrits.
  Consolidation à suivre dans ce fichier.
