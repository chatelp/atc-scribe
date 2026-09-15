# Décisions et questions ouvertes

*Tenir ce fichier à jour est une consigne, pas une option : c'est la mémoire du
chantier. Une décision prise se déplace dans la première section avec sa date et son
motif. Une question qui se pose s'ajoute à la seconde.*

## Décisions prises

| # | Décision | Date | Motif |
|---|---|---|---|
| D1 | **Le service tournera sur le Mac mini M4**, pas sur la station | 15/09 | Deux cœurs Haswell déjà chargés côté station ; la transcription locale demande de la puissance. L'ADS-B et l'audio traversent le réseau : quelques centaines de kbit/s, sans commune mesure avec le lien disponible. |
| D2 | **Le mode de diffusion par canal s'ajoute au mélangeur, il ne le remplace pas** — et il est activable à la demande | 15/09 | Le flux mélangé `aero.mp3` est écouté tous les jours et ne doit pas changer. Le mode par canal rejoint la mécanique de bascule existante de `radio-ctl`. |
| D3 | **Transcription entièrement locale**, l'API OpenAI est retirée | 15/09 | Coût récurrent, envoi de l'audio chez un tiers, dépendance réseau sur une station autonome. C'est la raison d'être du fork. |
| D4 | **Nom du dépôt : `co-atc-local`** | 15/09 | Destiné à un dépôt public. C'est ce qu'on cherche quand on veut Co-ATC sans OpenAI. Whisper est le moyen, « local » est la promesse. |
| D5 | **Le service reste sur le réseau local**, jamais exposé | 15/09 | Co-ATC n'a aucune authentification et son auteur déconseille l'exposition. |
| D6 | **La licence amont est MIT — le fork est publiable** | 15/09 | Vérifié le 15/09 dans `LICENSE` sur la branche `main` du dépôt amont : « MIT License — Copyright (c) 2025 Yegor S ». MIT autorise la modification et la redistribution sans réciprocité et sans autorisation préalable. **Seule obligation : conserver la notice de copyright et le texte de la licence dans toute copie.** Concrètement : garder le fichier `LICENSE` amont tel quel, et signaler le fork dans le README. |
| D7 | **Le fork est publié sous licence MIT**, comme l'amont | 15/09 | Décision du propriétaire. Un fork MIT d'un projet MIT n'ajoute aucune friction pour qui voudrait reprendre le travail et garde ouverte la possibilité de renvoyer des morceaux en amont (consigne « le fork reste rebasable »). Le fichier `LICENSE` amont est conservé tel quel ; la notice du fork s'ajoute, elle ne remplace pas. |
| D8 | **La source ADS-B de la station fonctionne sans adaptation** — mode `tar1090` de l'amont, tel quel | 15/09 | Mesuré le 15/09 : `tar1090_base_url = "http://192.168.1.10:8080/data/"`, validation réussie au démarrage, **128 avions suivis dont 96 positionnés**, phases détectées (68 CRZ, 17 ARR, 9 DEP, 1 T/O). Aucune ligne de Go écrite. La moitié du produit est acquise. |
| D9 | **Les données de référence sont déjà mondiales — rien à fournir pour l'Île-de-France** | 15/09 | `assets/` est livré peuplé dans le dépôt amont (OurAirports + tar1090-db + OpenFlights). Les sept terrains (LFPG, LFPO, LFPB, LFPZ, LFPX, LFPN, LFPV) sont présents avec leurs pistes. Au démarrage : 307 aérodromes, 67 pistes et 43 balises dans les 100 NM. L'affirmation contraire de `02-co-atc-amont.md` était fausse et a été corrigée. |
| D10 | **Le moteur d'exécution est mlx-whisper** | 15/09 | Q2 tranchée par la mesure (`08-premiere-mesure.md`) : le modèle affiné de jacktol se convertit en 10,5 s et se charge. Mesuré sur 16 transmissions réelles, **× 8,5 le temps réel en médiane contre × 1,4 pour `large-v3-turbo`**. Et le moteur que l'amont recommande, faster-whisper, n'a pas d'accélération Metal. Décision révisable si l'affinage maison (stratégie C) impose un autre format. |
| D11 | **Passe de confidentialité sur les documents publiables** — les coordonnées de la station restent, le reste part | 15/09 | `CLAUDE.md` demande « pas d'adresse exacte » tout en notant que la position est déjà publique via FlightAware. Arbitrage du propriétaire : **on garde la position** (48.80586 / 2.04932 / 152 m), sans laquelle aucune mesure de portée ou de propagation n'a de sens, et **on retire de `01-station.md` l'étage, l'orientation de la prise d'antenne et les identifiants de compte FlightAware et Flightradar24** — aucune valeur technique, et ensemble ils désignaient un logement plutôt qu'une station. Fait avant le premier commit : **rien de tout cela n'est entré dans l'historique git**. |
| D12 | **Le modèle anglais est `large-v3-atco2` multilingue, pas `jacktol medium.en`** | 16/09 | Départagé par l'ADS-B, pas par un accord entre modèles : sur 1 468 transmissions de la captation du 15/09, le multilingue affiné ATC produit **94 appariements vrais contre 62,6**, et gagne sur les cinq fréquences sans exception. Sur 124,625 le modèle anglais tombe **au niveau du hasard**. `03-transcription.md` désignait le mauvais candidat ; c'est corrigé. Détail dans `18-nuit-du-15.md`. |
| D13 | **La base SQLite de Co-ATC ne s'efface jamais** | 15/09 | Un `rm -rf data/` a détruit l'historique ADS-B d'une fenêtre de captation. Récupéré depuis `globe_history` de readsb, mais la règle tient : cette base est la seule trace locale du croisement radio/ADS-B, et elle ne se reconstitue pas toute seule. |

## Questions ouvertes

### Q1 — Quelle stratégie de modèle pour le bilinguisme ? **branche anglaise tranchée** *(16/09)*

> **La branche anglaise est réglée (D12)** : `sfabriece/whisper-large-v3-atco2-asr-mlx`,
> départagé par l'appariement ADS-B sur 1 468 transmissions réelles. Ce qui reste ouvert,
> et qui l'était depuis le début, **c'est le français** — et rien de ce qui a été mesuré
> depuis ne l'a fait bouger.


Deux modèles avec aiguillage par langue (A), un seul multilingue générique (B), ou un
affinage maison (C). **À trancher par la mesure**, protocole dans `04-corpus.md`. C'est la
question centrale du projet.

**Premiers éléments, 16 transmissions non annotées** (`08-premiere-mesure.md`) — à ne pas
confondre avec l'évaluation :

- Sur l'anglais, **jacktol est nettement meilleur que `large-v3-turbo`** et six fois plus
  rapide. Sur du vrai contrôle en route, il rend de la phraséologie OACI bien formée là
  où le générique produit du charabia, une boucle dégénérative et des caractères hébreux.
- Sur le français, **jacktol est pire qu'inutile : il est plausible.** Sur des
  transmissions d'aéroclub francophone, il a produit « cleared to land runway zero five »
  et « lufthansa one romeo romeo is ready for immediate clearance ». Une sortie bien
  formée, confiante et entièrement inventée — un échec indiscernable d'une réussite pour
  tout ce qui est en aval.
- **La vitesse ne départagera pas.** Sept canaux à 5 % d'occupation demandent 0,35 × temps
  réel ; les deux modèles tiennent largement. Le choix se fera sur la seule qualité.

**Ce qui manque avant de trancher, et qui est bloquant :** la vérité terrain (120
transmissions annotées), un nouveau comparatif **avec l'amorce de l'amont** — B a été
testé sans prompt alors que l'amont en fournit un, donc B n'a pas été jugé à la loyale —
et une mesure de la fiabilité de l'aiguillage lui-même, qui est le point faible de A.

**Deuxième série, 50 transmissions tirées au hasard** (`10-mesure-amorces.md`), qui
corrige l'échantillonnage alphabétique de la première :

- **L'amorce de l'amont est à jeter.** Elle multiplie par sept les boucles de
  dégénérescence sur le français, double le temps de calcul, et **fuit dans la sortie**
  (« Aircraft are at cruise level and use ICA-7, Yankee Papa… »). Écrite pour un flux
  continu via l'API temps réel, elle pèse plus que l'audio sur une transmission de 3 s.
- **Une amorce dans la langue de la fréquence répare presque tout** : boucles 7 → 1,
  temps 4,84 s → 1,99 s. L'a priori par fréquence ne sert donc pas qu'à aiguiller entre
  deux modèles : il choisit aussi la langue de l'amorce. C'est gratuit.
- **Les deux modèles échouent différemment, et une seule façon est réparable.** jacktol
  produit 14 noms de compagnie, dont 4 « Air Berlin » — compagnie disparue en 2017, donc
  nécessairement faux — et 4 « Lufthansa » sur la tour d'un aéroclub. turbo n'en produit
  qu'un seul, mais rend « opharnentureth » et « Jepenst er brent ». **L'erreur de jacktol
  est structurée : le post-traitement de Co-ATC jette déjà le nom de compagnie et apparie
  sur les chiffres contre la liste ADS-B. Celle de turbo n'offre aucune prise.** Argument
  de poids pour A, invisible avant d'avoir les deux sorties côte à côte.
- **La détection de langue échoue surtout sur des fichiers déjà perdus** (sorties en
  russe, japonais, norvégien = charabia). L'aiguillage est donc moins fragile qu'on ne le
  craignait, **à condition** de garder l'a priori par fréquence par défaut.

### Q2 — mlx-whisper et le modèle de jacktol : **tranchée, oui** *(15/09)*

**Oui, après conversion et un renommage de fichier.** Mesures et détail dans
`08-premiere-mesure.md`. Trois points à retenir pour les instructions d'installation :

- `convert.py` n'est pas dans le paquet pip `mlx-whisper` — il faut `ml-explore/mlx-examples`,
  dossier `whisper/`. Dépendance non versionnée.
- La conversion prend **10,5 s** et produit 1,42 Gio en float16.
- **`convert.py` écrit `model.safetensors`, `mlx_whisper` cherche `weights.safetensors`.**
  Un `cp` corrige ; sans lui, l'erreur (`[load_npz] Input must be a zip file`) n'oriente
  pas du tout vers la vraie cause.

Q2 ne bloque plus Q1. Voir aussi D10.

### Q3 — Comment Co-ATC apprend-il quelles fréquences sont actives ?

La station change de groupe de fréquences à la demande. Trois options dans
`01-station.md`. Dépend de ce que la configuration de Co-ATC permet de recharger à
chaud — **pas encore cherché dans le code, à faire**.

Deux éléments relevés le 15/09 qui aideront :

- `/radio/etat` renvoie déjà le mode actif **et la liste de ses fréquences** sous forme
  exploitable (`"freqs": ["123.875", "124.350", ...]`), plus un champ `accord` qui dit si
  ce qui sort d'Icecast correspond au gabarit chargé. Mesuré : mode `chaine`, groupe
  `orly-approche`, `accord: true`.
- Côté Co-ATC, une fréquence dont la source ne répond pas provoque une **relance de
  ffmpeg toutes les 4 à 5 secondes, sans recul de cadence**, indéfiniment (observé sur
  les flux LiveATC de la configuration par défaut). Déclarer toutes les fréquences et
  accepter que la plupart soient muettes n'est donc **pas** gratuit : à vérifier sur un
  flux Icecast silencieux, qui n'est pas la même chose qu'un flux absent.

### Q4 — Duplication des gabarits ou injection à la génération ?

Pour le mode par canal. L'injection dans `radio-ctl` est recommandée (une seule source
de vérité pour les fréquences) mais demande de toucher à `serveur.py`, qui tourne en
production.

### Q5 — L'assistant vocal : **mis de côté** *(15/09)*

Décision du propriétaire : hors chantier pour l'instant. Il dépend d'un modèle de
langue, et la question ne se pose utilement qu'une fois la transcription locale
acquise. Ne pas y consacrer de temps ; se contenter de ne pas casser le code
existant, ou de le neutraliser proprement derrière un drapeau de configuration.

### Q6 — Le post-traitement peut-il se faire par règles plutôt que par un LLM ? *(précisée, 15/09)*

Lecture faite de `prompts/post_processing_prompt.txt` (90 lignes) et de
`internal/transcription/post_processor.go`. Le prompt réclame **cinq choses**, et elles
ne sont pas du même niveau de difficulté :

| | Ce qui est demandé | Codable par règles ? |
|---|---|---|
| 1 | corriger librement les erreurs de transcription | **non** — c'est le seul vrai besoin de modèle de langue |
| 2 | écrire nombres, caps, niveaux et fréquences en chiffres | oui, grammaire de phraséologie |
| 3 | décider si le locuteur est ATC ou pilote | probablement, aux tournures |
| 4 | rattacher la transmission à un indicatif **de la liste ADS-B courante** | oui — appariement approximatif sur une liste courte et connue |
| 5 | extraire les clairances et la piste | oui, motifs fermés et énumérés dans le prompt |

**L'astuce centrale du produit est le point 4**, et elle n'est pas magique : le prompt
reçoit la liste des avions que l'ADS-B voit en ce moment et interdit au modèle d'en
inventer d'autres. Sur notre station cette liste fait 128 avions (mesuré le 15/09 à 11 h).
Apparier « nonsense 353 » contre 128 indicatifs connus est une distance d'édition
pondérée, pas un modèle de langue.

Et le point 1 est celui dont on peut se passer : `content` brut reste stocké à côté de
`content_processed`. **Q6 est donc plus prometteuse qu'elle n'en avait l'air — mais rien
n'est mesuré.** Il faut le jeu de test annoté de `04-corpus.md` d'abord.

Détail complet dans `07-carte-openai.md`.

### Q7 — Quelles sources pour la météo française ?

Windy est à remplacer. METAR et TAF sont disponibles publiquement ; les NOTAM français
passent par le SIA. À arbitrer selon les conditions d'utilisation.

### Q8 — Licence du fork : **tranchée** *(15/09)*

Voir D6 (la licence amont est MIT, le fork est publiable) et D7 (le fork est publié
sous MIT). Plus rien d'ouvert ici.

### Q9 — Transcrire les ATIS : **oui, mais pour une seule fréquence**

La question posée était : à quoi bon, si le METAR est gratuit en ligne ? Vérifié le
15 septembre auprès d'Aviation Weather Center, requête sur les terrains de la zone :

| Terrain | METAR public | ATIS reçu ici |
|---|---|---|
| LFPG De Gaulle | **oui** | +16,6 / +20,5 dB, faible |
| LFPO Orly | **oui** | +4,6 / +6,9 dB, sous le seuil |
| LFPB Le Bourget | **oui** | +14,6 dB, faible |
| LFPN Toussus | **oui** | +21,8 dB, correct |
| LFPV Villacoublay | **oui** | pas d'ATIS catalogué |
| **LFPZ Saint-Cyr** | **NON** | **+49,4 dB, le plus fort de toute la bande** |
| **LFPX Chavenay** | **NON** | +13,8 dB, faible |

Le résultat s'inverse : **le seul ATIS que la station reçoive parfaitement est le seul
dont la météo ne soit disponible nulle part en ligne**. Les cinq terrains qui publient
un METAR gratuit sont précisément ceux dont l'ATIS arrive mal ou pas du tout.

Et surtout, **un ATIS n'est pas un METAR**. Il porte en plus la **piste en service**,
le type d'approche, le niveau de transition et les travaux en cours sur le terrain —
rien de tout cela n'est dans un METAR, ni disponible gratuitement en temps réel. Pour
Co-ATC en particulier, qui détecte les phases de vol et la piste utilisée, **la piste en
service de Saint-Cyr est une donnée directement exploitable par sa logique**.

**Recommandation : un module tardif, optionnel, sur une seule fréquence** —
131,025, échantillonnée toutes les quelques minutes, dont on extrait la piste en service
plutôt qu'on ne reconstitue un METAR complet. C'est une porteuse permanente (médiane
collée à la crête), donc aucun problème de squelch : il suffit d'enregistrer N secondes
à intervalle régulier.

> ⚠️ **Un obstacle connu** : l'ATIS de Saint-Cyr **sature le récepteur** — le
> propriétaire l'a constaté à l'oreille, un souffle continu par-dessus la voix. À +49 dB
> c'est attendu. Il faudra un gain réduit ou une quantification plus basse **sur ce canal
> seulement** — ce que le mode de diffusion par canal rend justement possible.

C'est probablement la seule donnée réellement originale que la station produise : tout
le reste est, en principe, téléchargeable.

### Q10 — Écart de comptage entre les documents et `06-catalogue.csv` *(ouvert, 15/09)*

Relevé le 15/09 en comptant le CSV : **68 lignes**, réparties en 28 « identifiee »,
14 « non identifiee », 7 « porteuse permanente », 2 « absente de l'AIP » et 17 « sous le
seuil ». Or `00-mission.md` et `01-station.md` annoncent tous deux « 52 fréquences
identifiées », et `01-station.md` parle de « quinze fréquences actives dans aucune liste
publique ».

Les chiffres se recoupent si « 52 » désigne les fréquences **au-dessus du seuil**
(68 − 17 = 51) et « 15 » les non catalogées (14 + 2 = 16) — à une unité près dans les
deux cas. Il manque donc soit une ligne au CSV, soit une définition explicite.
**Ne pas corriger les documents à l'aveugle** : trancher en reprenant la mesure d'origine
sur la station. Sans importance pour la suite du chantier, mais à régler avant publication
— un dépôt public qui annonce trois nombres différents pour la même chose s'expose.

### Q11 — Vrai fork rebasable, ou dépôt séparé ? *(mesuré le 15/09)*

**La question posée était : est-ce que ça vaut le coup, si on prévoit des modifications
de fond ?** Elle se tranche par deux mesures, faites le 15/09 sur le clone amont.

**Mesure 1 — notre empreinte réelle.** Le fork est *profond* mais *étroit* :

| Zone | Lignes Go | Notre sort |
|---|---|---|
| `internal/transcription` | 2 457 | réécrite |
| `internal/atcchat` | 1 119 | neutralisée (Q5) |
| `internal/weather` | 690 | réécrite (Q7) |
| **sous-total réécrit** | **4 266** | **17 % du Go** |
| `internal/adsb` | 7 084 | intacte |
| `internal/api`, `storage`, `frequencies`, `audio`, `templating`, `reference`… | ~13 000 | intactes |
| `www/` (JS + HTML + CSS) | 15 798 | **intacte, à 100 %** |

Total Go amont : 24 834 lignes. **Nous en réécrivons 17 %, et nous ne touchons pas une
ligne de l'interface.**

**Mesure 2 — où l'amont travaille, lui.** 46 commits, du 10 juillet 2025 au 3 mai 2026,
**silencieux depuis 134 jours**. Lignes ajoutées et supprimées par zone sur tout
l'historique :

| Zone | Amont | Lecture |
|---|---|---|
| `www` | 23 317+ / 7 006− | c'est là qu'il vit |
| `internal/adsb` | 8 806+ / 1 722− | activité soutenue |
| `internal/api` | 3 553+ / 196− | croissance, peu de remaniement |
| `internal/transcription` | 2 594+ / **78−** | écrite une fois, presque jamais retouchée |
| `internal/atcchat` | 1 123+ / **4−** | idem |
| `internal/weather` | 690+ / **0−** | **jamais modifiée depuis sa création** |

**Les trois paquets que nous réécrivons sont exactement ceux que l'amont ne touche pas.**
Nos conflits de rebase porteront donc sur des fichiers dont il ne bouge presque rien,
pendant qu'il travaille sur les 72 % de code et les 15 798 lignes d'interface que nous
laissons intacts.

**Conclusion : oui, et l'argument s'inverse même.** Ce n'est pas « malgré » les
modifications de fond qu'il faut un vrai fork, c'est **à cause** d'elles. Plus notre part
est profonde et concentrée, plus il est précieux de pouvoir absorber sans effort le
travail de l'amont sur la carte, l'ADS-B et les phases de vol — qui est la moitié du
produit et 100 % de son activité récente. Un dépôt séparé nous ferait payer ce travail-là
en réintégration manuelle, indéfiniment.

**Réserve honnête** : un amont silencieux depuis 4 mois pourrait ne jamais reprendre. Dans
ce cas le vrai fork n'aura rien coûté — c'est le même `git clone` avec un remote en plus.
Le pari est asymétrique.

**Décision du propriétaire.** Reste le détail de nommage : l'amont a déjà un `docs/` bien
rempli (dont `LOCAL-STT.md`, qu'on veut garder). Nos documents français ont besoin d'un
dossier à eux — `dossier/` ou `chantier/`.

### Q12 — Quel `airport_code` pour la station ?

`[station] airport_code` sert à deux choses : choisir les pistes du « terrain de
référence » et interroger la météo. Aucun terrain n'est à la station ; le plus proche est
**LFPZ Saint-Cyr, à environ 1,5 km**, retenu provisoirement le 15/09.

Conséquence mesurée immédiatement : **Windy renvoie 204 sur LFPZ pour METAR et TAF**
(les NOTAM passent). C'est la confirmation empirique de Q9 — LFPZ ne publie pas de METAR.
Le service continue de tourner, il réessaie trois fois puis renonce et journalise une
erreur toutes les dix minutes.

Trois options : garder LFPZ et accepter l'absence de météo jusqu'à ce que Q7 la remplace ;
mettre LFPO Orly, qui publie un METAR mais dont les pistes n'ont rien à voir avec ce
qu'on entend sur les fréquences d'aéroclub ; ou dissocier les deux réglages, ce qui
demande de toucher au code.

### Q13 — Faut-il stocker la langue détectée ?

Le schéma amont (`transcriptions`) n'a **aucun champ pour la langue** : `content`,
`content_processed`, `speaker_type`, `callsign`. Sur une station bilingue c'est une
information qu'on voudra garder — pour mesurer, pour afficher, et pour alimenter l'a
priori par fréquence décrit dans `03-transcription.md`. Ajouter une colonne `language`
est une divergence avec l'amont : petite, mais à assumer et à signaler comme telle si on
propose le code en retour.

### Q14 — Contraindre la sortie contre la géographie réelle ? *(nouvelle, 15/09)*

Mesuré le 15/09 : le modèle de jacktol produit des toponymes **tchèques** — `praha`,
`ruzyne`, `venox` — y compris sur un fichier de **Chavenay**, aéroclub des Yvelines. Or
`large-v3-turbo` n'en produit nulle part. Ce n'est donc pas de la réception lointaine mais
un **biais d'affinage** : jacktol est entraîné sur ATCO2 et UWB-ATCC, corpus de contrôle
aérien tchèque, et rabat ce qu'il ne comprend pas sur le vocabulaire qu'il connaît.

Le produit a déjà la parade en magasin : Co-ATC charge **43 balises dans les 100 NM** de
la station et contraint déjà les indicatifs contre la liste ADS-B courante. Étendre la
même contrainte aux points de report et aux toponymes est peu coûteux et directement
utile. À évaluer avec Q6.

Second usage, pour l'évaluation : **compter les hallucinations de toponymes est un
indicateur en soi**, bien moins coûteux à produire qu'un WER et plus parlant pour ce
produit qu'un taux d'erreur moyen.

### Q16 — Que publier du jeu d'évaluation ? *(nouvelle, 15/09)*

Le jeu de 120 transmissions (`09-jeu-de-test.md`) est constitué et **volontairement laissé
hors du dépôt**. Ce sont des enregistrements de communications radio réelles portant des
indicatifs identifiables : les rediffuser n'est pas le même geste que publier du code.

Ce qui est publiable sans difficulté : **la graine du tirage, la liste des noms de
fichiers, et les annotations**. Cela suffit à rejouer la mesure à l'identique pour qui
possède la station, et à comparer des modèles sur les transcriptions de référence, sans
rediffuser une seconde d'audio.

Ce qui ne se décide pas ici : s'il faut, un jour, publier un sous-ensemble audio — ce
serait le premier corpus d'ATC francophone ouvert, et c'est précisément le vide que
`03-transcription.md` identifie. Le gain scientifique est réel ; la question ne se pose
utilement qu'après la mesure, et elle demande un avis qui n'est pas technique.

### Q17 — Le corpus ne contient aucune fréquence anglophone **bloquante pour Q1**, *(mesurée le 15/09)*

Croisement fait le 15/09 entre `06-catalogue.csv` et les 12 dossiers réellement présents
dans `/opt/adsb/public/transmissions/` :

| Langue attendue au catalogue | Fréquences | Dont enregistrées |
|---|---|---|
| **anglais dominant** | 11 | **0** |
| français dominant | 6 | 2 — 128,950 et 129,525 |
| bilingue | 6 | 2 — 127,750 et 127,850 |
| langue inconnue | 22 | 7 — l'amas 132-133 |

**33 des 44 fréquences marquées « à transcrire » n'ont aucun enregistrement**, dont la
totalité de CDG (approche, tour, sol, prévol), Orly Tour et Orly Approche, Le Bourget,
Saint-Cyr, Toussus, et les secteurs DG et DO de Paris Contrôle. Le corpus est le résidu
de deux groupes d'écoute seulement : `paris5-avec-enregistrement` et `amas-132-133`.

**Ce que ça invalide.** L'anglais du jeu d'évaluation (`09-jeu-de-test.md`) repose sur
132,500 — non identifiée, langue *supposée* — et 127,750, étiquetée bilingue. La mesure ne
dira donc rien de l'anglais d'approche d'un grand aéroport : débit rapide, secteur saturé,
collationnements serrés, accents non natifs. **C'est exactement le régime pour lequel
jacktol a été affiné** (ATCO2, UWB-ATCC), donc la mesure risque de le sous-estimer.
Conclure sur Q1 sans ça serait conclure sur la moitié du problème.

**Comment y remédier, et pourquoi ça réordonne le chantier.** Enregistrer un groupe CDG
demande de basculer le groupe d'écoute — donc l'accord du propriétaire — et la sortie
fichier par transmission **dégrade le flux en direct** (`01-station.md` : médiane de salve
4,6 s → 0,1 s, 0 % → 64 % de salves sous la seconde).

> **Mais le mode de diffusion par canal (D2) est aussi la façon propre de collecter.**
> Un point de montage Icecast continu par fréquence n'ouvre pas un encodeur LAME à chaque
> squelch : il échappe par construction au piège de fragmentation. Ce qu'il faut bâtir de
> toute façon pour brancher Co-ATC sur la station est aussi ce qui débloque le corpus
> anglophone.
>
> **Proposition : remonter le mode par canal avant l'enrichissement du corpus**, au lieu
> de l'ordre initialement prévu. Décision du propriétaire.

En attendant, le jeu de 120 reste utile — il tranche la question française, qui est
l'apport revendiqué du projet — mais **il ne suffit pas à trancher Q1**.

---

## Mesure du 15/09 — la sortie fichier continue ne dégrade pas le direct

**Q2 de D2 est tranchée.** Session de captation sur `orly-approche`,
**12:05:16 → 14:43:10 CEST**, cinq canaux, sortie `type = "file"` continue
(`split_on_transmission = false`, `continuous = true`) **ajoutée** au mélangeur.

Trois relevés `salves.py` à chaque état, parce qu'une prise isolée ne vaut rien :

```
AVANT    12:01  med 4,4 s  <1 s  0 %      12:03  med 2,3 s  30 %    12:04  med 1,2 s  46 %
PENDANT  12:08  med 5,0 s  <1 s 20 %      12:10  med 2,7 s  25 %    12:11  med 3,9 s   0 %
APRES    14:46  med 2,5 s  <1 s 40 %      14:48  med 5,0 s  25 %    14:49  med 6,3 s   0 %
```

Moyennes : 2,6 s / 25 % avant, **3,9 s / 15 % pendant**, 4,6 s / 22 % après.
**Aucune dégradation** — et la dispersion à l'intérieur de chaque triplet est plus
grande que l'écart entre les triplets. Rien à voir avec les sorties *par
transmission*, qui faisaient 4,6 s → 0,1 s et 0 % → 64 %.

**Le mécanisme est confirmé** : c'est l'ouverture/fermeture d'un encodeur LAME à
chaque squelch qui fragmentait le direct, pas l'écriture elle-même. Un encodeur
unique tenu ouvert pour la session ne coûte rien au flux.

Charge machine pendant la captation : `rtl_airband` à 7,8 %, aucun processus en
attente disque, iowait plat. Les pointes de charge observées venaient des relevés
de contrôle eux-mêmes, pas de la captation.

### Ce que la session a produit

Quinze fichiers, trois par canal, **découpés à l'heure** — `c_AAAAMMJJ_HH.mp3` :

| dossier | volume | ce que c'est |
|---|---|---|
| `123875` | 11 Mo | Orly Approche |
| `124350` | 11 Mo | Approche CDG + Le Bourget |
| `124625` | 9,7 Mo | Paris Contrôle secteur DG |
| `125825` | 13 Mo | De Gaulle Approche |
| `125933` | 9,7 Mo | **non identifiée** — premier enregistrement |

Servis en HTTP : `http://macmini-fedora.lan/fichiers/transmissions/<freq>/`

> ⚠️ **Ne pas se fier à la durée annoncée par l'en-tête.** Ce sont des MP3 à débit
> variable sans en-tête Xing : `ffprobe` en déduit des durées fausses, de 1,99 h à
> 2,95 h selon le fichier. Le comptage de trames donne la vérité — et elle est
> rassurante : **45 554 / 49 998 / 36 093 trames, identiques sur les cinq canaux**,
> soit 3 280 + 3 600 + 2 599 = **9 479 s = 2 h 38 min**, ce qui recoupe l'horloge à
> cinq secondes près.
>
> **Les cinq canaux sont alignés à la trame près.** C'est une propriété précieuse
> pour recouper avec l'ADS-B : un même instant porte le même numéro de trame sur
> les cinq fichiers. Trame MP3 à 8 kHz = 576 échantillons = 72 ms.

Quelques avertissements `Header missing` du décodeur sur la première trame de
certains fichiers — le flux commence en cours de trame. Sans conséquence, mais
ignorer la première trame par prudence.

**Gabarit restauré à 14:43**, zéro sortie fichier dans `orly-approche.tmpl` comme
dans `rtl_airband.conf`. La sauvegarde du gabarit modifié reste dans
`modes/orly-approche.tmpl.avant-captation-20260915-120506` si la manipulation est
à refaire.

### Q18 — L'historique ADS-B de readsb : à préserver *(nouvelle, 16/09)*

`/opt/adsb/globe_history` sur la station contient un historique permanent complet —
une tranche binaire gzip par demi-heure, avec positions, altitudes **et indicatifs**.
Format non documenté, décodé le 15/09 et reconstitué sans perte : 53 568 positions,
1 092 avions nommés, 100 % avec indicatif sur la fenêtre de captation. Décodeur dans
`whisper-lab/heatmap.py`.

**Conséquence** : aucune captation future n'a besoin d'un enregistrement ADS-B en
parallèle. Il suffit que ces tranches ne soient pas purgées. À dire à l'agent de la
station, et à vérifier : combien de jours sont conservés ?

### Q19 — Pourquoi 125,825 est-elle la moins bien transcrite ? *(nouvelle, 16/09)*

49 % de précision, la dernière des cinq, alors qu'elle porte le plus de trafic —
491 transmissions, **26 % d'occupation** contre les 3 à 5 % d'une fréquence ordinaire.
L'hypothèse simple est que la saturation est la cause : chevauchements, débit rapide.
**Non vérifiée.** Le test serait de mesurer la précision en fonction de l'occupation
instantanée — faisable avec les données déjà en main.
