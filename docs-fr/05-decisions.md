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
| D4 | ~~**Nom du dépôt : `co-atc-local`**~~ — **remplacée par D20 le 16/09** | 15/09 | Destiné à un dépôt public. C'est ce qu'on cherche quand on veut Co-ATC sans OpenAI. Whisper est le moyen, « local » est la promesse. |
| D5 | **Le service reste sur le réseau local**, jamais exposé | 15/09 | Co-ATC n'a aucune authentification et son auteur déconseille l'exposition. |
| D6 | **La licence amont est MIT — le fork est publiable** | 15/09 | Vérifié le 15/09 dans `LICENSE` sur la branche `main` du dépôt amont : « MIT License — Copyright (c) 2025 Yegor S ». MIT autorise la modification et la redistribution sans réciprocité et sans autorisation préalable. **Seule obligation : conserver la notice de copyright et le texte de la licence dans toute copie.** Concrètement : garder le fichier `LICENSE` amont tel quel, et signaler le fork dans le README. |
| D7 | **Le fork est publié sous licence MIT**, comme l'amont | 15/09 | Décision du propriétaire. Un fork MIT d'un projet MIT n'ajoute aucune friction pour qui voudrait reprendre le travail et garde ouverte la possibilité de renvoyer des morceaux en amont (consigne « le fork reste rebasable »). Le fichier `LICENSE` amont est conservé tel quel ; la notice du fork s'ajoute, elle ne remplace pas. |
| D8 | **La source ADS-B de la station fonctionne sans adaptation** — mode `tar1090` de l'amont, tel quel | 15/09 | Mesuré le 15/09 : `tar1090_base_url = "http://192.168.1.10:8080/data/"`, validation réussie au démarrage, **128 avions suivis dont 96 positionnés**, phases détectées (68 CRZ, 17 ARR, 9 DEP, 1 T/O). Aucune ligne de Go écrite. La moitié du produit est acquise. |
| D9 | **Les données de référence sont déjà mondiales — rien à fournir pour l'Île-de-France** | 15/09 | `assets/` est livré peuplé dans le dépôt amont (OurAirports + tar1090-db + OpenFlights). Les sept terrains (LFPG, LFPO, LFPB, LFPZ, LFPX, LFPN, LFPV) sont présents avec leurs pistes. Au démarrage : 307 aérodromes, 67 pistes et 43 balises dans les 100 NM. L'affirmation contraire de `02-co-atc-amont.md` était fausse et a été corrigée. |
| D10 | **Le moteur d'exécution est mlx-whisper** | 15/09 | Q2 tranchée par la mesure (`08-premiere-mesure.md`) : le modèle affiné de jacktol se convertit en 10,5 s et se charge. Mesuré sur 16 transmissions réelles, **× 8,5 le temps réel en médiane contre × 1,4 pour `large-v3-turbo`**. Et le moteur que l'amont recommande, faster-whisper, n'a pas d'accélération Metal. Décision révisable si l'affinage maison (stratégie C) impose un autre format. |
| D11 | **Passe de confidentialité sur les documents publiables** — les coordonnées de la station restent, le reste part | 15/09 | `CLAUDE.md` demande « pas d'adresse exacte » tout en notant que la position est déjà publique via FlightAware. Arbitrage du propriétaire : **on garde la position** (48.80586 / 2.04932 / 152 m), sans laquelle aucune mesure de portée ou de propagation n'a de sens, et **on retire de `01-station.md` l'étage, l'orientation de la prise d'antenne et les identifiants de compte FlightAware et Flightradar24** — aucune valeur technique, et ensemble ils désignaient un logement plutôt qu'une station. Fait avant le premier commit : **rien de tout cela n'est entré dans l'historique git**. |
| D12 | **Le modèle anglais est `large-v3-atco2` multilingue, pas `jacktol medium.en`** | 16/09 | Départagé par l'ADS-B, pas par un accord entre modèles : sur 1 468 transmissions de la captation du 15/09, le multilingue affiné ATC produit **94 appariements vrais contre 62,6**, et gagne sur les cinq fréquences sans exception. Sur 124,625 le modèle anglais tombe **au niveau du hasard**. `03-transcription.md` désignait le mauvais candidat ; c'est corrigé. Détail dans `18-nuit-du-15.md`. |
| D13 | **La base SQLite ne s'efface jamais** — *amendée le 21/09, voir D29* | 15/09 | Un `rm -rf data/` a détruit l'historique ADS-B d'une fenêtre de captation. Récupéré depuis `globe_history` de readsb, mais la règle tient : cette base est la seule trace locale du croisement radio/ADS-B, et elle ne se reconstitue pas toute seule. |

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

### Q9 — Transcrire les ATIS : **oui, mais pour une seule fréquence** *(doublement infirmée le 20/09 — voir Q32)*

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

> ⚠️ ~~**Un obstacle connu** : l'ATIS de Saint-Cyr **sature le récepteur** — le~~
> ~~propriétaire l'a constaté à l'oreille, un souffle continu par-dessus la voix.~~
>
> **Mesuré le 20/09 : faux.** Au gain quotidien de 40,2 dB la transcription est déjà
> exploitable ; réduire à 32,8 dB l'améliore mais ne la débloque pas. « Sature le
> récepteur » décrivait une écoute, pas une mesure, et a tenu cinq jours. **Et le contenu
> espéré — piste en service, niveau de transition — n'est pas diffusé à cette heure.**
> Voir Q32 ; la boucle de jour reste à vérifier, c'est Q33.

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

### Q11 — Vrai fork rebasable, ou dépôt séparé ? *(mesuré le 15/09, **révisé par D19 le 16/09**)*

> **La réponse « oui » tient toujours pour la forme du dépôt, mais plus pour la
> méthode de travail.** Ce qui suit a été mesuré avant que le fork n'écrive une ligne ;
> D19 le remesure après 38 commits. Lire les deux dans l'ordre.

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
de deux groupes d'écoute seulement : `paris5-avec-enregistrement` et `amas-132-133`,
renommé `en-route-et-descente-cdg` le 16/09.

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

### D14 — L'appariement en ligne remplace l'étage GPT-4o *(16/09)*

L'amont découpe la transcription en deux étages : le texte brut, puis un
post-traitement GPT-4o qui remplit le locuteur, l'indicatif et les autorisations.
**Notre grammaire prend la place du second, à sa propre couture** —
`post_processing.backend = "local"` — et écrit dans les tables de l'amont avec son
vocabulaire. Zéro migration de schéma, zéro ligne de JavaScript.

**Pourquoi c'est la bonne forme de greffe** : remplacer un composant à la couture
que l'amont a lui-même définie se rebase ; ajouter une fonction à côté, non. Et le
propriétaire peut rebasculer sur GPT-4o par une clé de configuration pour comparer
les deux sur le même point.

Réglages fixés par balayage, pas au jugé : **3 chiffres, fenêtre 60 s, refus des
ambigus, pas de seuil de score supplémentaire**. Le tableau complet est dans
`19-appariement-en-ligne.md` et recopié en commentaire dans `config.toml`, pour que
le compromis reste celui du propriétaire.

### D15 — La position de l'indicatif départage le locuteur *(16/09)*

La détection du locuteur ne couvrait que **10 %** des transmissions, et **0 %** du
français. Deux ajouts : les verbes d'instruction à l'impératif — c'est le temps qui
discrimine, *descend* contre *descending*, *descendez* contre *on descend*, donc ces
formes ne doivent jamais être lemmatisées — et la règle structurelle OACI selon
laquelle le contrôleur ouvre par l'indicatif et le pilote le termine par le sien.

Couverture **10 % → 54 %**. La justesse, elle, **n'est pas établie** : voir Q20.

### Q20 — La détection du locuteur est-elle juste ? *(nouvelle, 16/09)*

Couverture mesurée, justesse non. Un contrôle falsifiable a été écrit à défaut
d'annotations : un collationnement suit son instruction, donc les étiquettes doivent
alterner sur deux transmissions proches partageant un nombre. Résultat **61 % et
59 % contre 50 % au hasard — sur 35 paires seulement, ce n'est pas concluant.**
Le contrôle est rejouable ; il demande un corpus plus long.

### Q21 — macOS refuse le réseau local aux binaires fraîchement compilés *(nouvelle, 16/09)*

co-atc ne démarre plus : `connect: no route to host` sur la station, alors que
`curl` sur la même URL répond 200 **au même instant, cinq fois sur cinq**.

Trois tests qui *opposent* les hypothèses, et non un seul qui confirme la première :

| test | résultat | ce qu'il élimine |
|---|---|---|
| Go vers `1.1.1.1` contre Go vers `192.168.1.x` | public OK, local en échec | ce n'est pas une panne réseau générale |
| `curl` et Go lancés au même instant, 5 fois | curl 5/5, Go 0/5 | ce n'est pas la station, ni le moment |
| Go forcé sur `en0` puis sur `en1` | les deux en échec | ce n'est pas le choix d'interface |

Reste une seule explication : **la permission « Réseau local » de macOS, accordée par
exécutable.** `curl` et `ssh` sont des binaires système déjà autorisés ; un binaire Go
recompilé est une identité neuve, et le refus remonte précisément en `EHOSTUNREACH`.

**Ce qu'il faut faire, et que je ne peux pas faire à votre place** : autoriser
`bin/co-atc` dans *Réglages Système → Confidentialité et sécurité → Réseau local*. Si
l'entrée n'y est pas, lancer `./bin/co-atc -config configs/config.toml` depuis votre
propre Terminal fait apparaître la demande.

> **Deux corrections successives, 16/09 — et la seconde annule la première.**
>
> À 15 h 55 j'ai écrit ici que la signature ad-hoc ne préservait pas l'autorisation,
> « mesuré » par un test apparié `curl` contre Go. **Ce test ne pouvait rien mesurer.**
> Il passait par `go run` et `go build -o /tmp/probe`, qui produisent un binaire neuf
> à chaque appel : une identité neuve est refusée par construction, quelle que soit
> l'autorisation accordée à `bin/co-atc`. J'ai confondu « un binaire Go est refusé »
> avec « ce binaire-ci est refusé ».
>
> **Testé correctement à 16 h 00, sur `bin/co-atc` lui-même, trois lancements
> successifs : `ADS-B source validation succeeded` trois fois sur trois**, avec `curl`
> à 200 aux mêmes instants. Le binaire recompilé et signé fonctionne. La panne de
> 15 h 52 était un incident passager — cohérent avec les dix chutes du lien Ethernet
> relevées par l'agent de la station en une nuit (`20-captation-nuit.md`).
>
> **Ce qui reste établi** : la permission « Réseau local » de macOS existe, elle est
> accordée par exécutable, et un binaire fraîchement compilé y est soumis.
>
> **Ce qui n'est PAS établi, contrairement à ce que ce document a affirmé deux fois** :
> que `bin/co-atc` perde son autorisation à chaque recompilation. Aucun test propre ne
> l'a montré, et le seul test propre disponible montre l'inverse.
>
> **La leçon, et elle est coûteuse** : sur cette machine, une panne réseau passagère et
> un refus de permission produisent *le même message d'erreur*. Un échec unique ne
> distingue pas les deux. **Relancer trois fois avant de diagnostiquer** coûte trente
> secondes ; j'ai écrit deux conclusions fausses pour les avoir économisées.

### Q22 — Le Mac est sur le même sous-réseau deux fois *(nouvelle, 16/09 ; toujours vrai le 26/09, mais **pas la cause de la panne du 25/09** : voir la fin)*

> **26/09 : de nouveau suspect.** Le 25/09 au soir, depuis le Mac, six coupures de lectures
> longues d'`aero.mp3` (21:54 – 22:24), puis « No route to host » vers la station de 22:24:40 à
> 23:48 — la station, elle, joignait le Mac sans un échec, et un ssh du Mac est passé à 22:51
> (Q45). Relevé le 26/09 à 10:35 : **`en0` Ethernet en `.28` et `en1` Wi-Fi en `.24`**, la
> station en ARP sur `en0` seulement à cet instant. Une veille tourne pour trancher entre Q21 et
> Q22 à la prochaine panne (`whisper-lab/scripts/exploitation/veille-liaison.py` : connexion
> toutes les 5 s, et à la première erreur, `curl` au même instant plus la route, l'ARP et les
> interfaces). **Recommandation au propriétaire, inchangée depuis le 16/09 : couper le Wi-Fi du
> Mac tant qu'il est relié par câble** — un réglage système, qui lui revient.
>
> **Plus tard le 26/09 : Q21 plutôt que Q22.** La station fait remarquer qu'un ssh parti de ce même
> Mac, par la même adresse `.28`, a réussi à 22:51 en pleine panne — même machine, même double
> réseau : une cause propre au programme est plus probable. Et **le propriétaire a trouvé ce matin
> une demande d'autorisation « Réseau local » de macOS en attente.** Le refus de la confidentialité
> remonte précisément en « No route to host » (Q21). Le programme en panne était
> l'enregistrement de contrôle, lancé par le Python du laboratoire
> (`~/.pyenv/versions/3.12.3/bin/python3.12`). Non établi : de quel programme venait la demande.
> La veille note désormais, en cas d'échec, le programme refusé et son parent.
>
> **La demande en attente nommait « node », selon le propriétaire.** Sur le Mac, les seuls `node`
> en marche sont le serveur `desktop-commander` (un greffon de l'application Claude, lancé par
> `npx`, démarré le 25/09 à 09:12 et 09:44, et le 26/09 à 09:00) — **pas dans la filiation du
> programme refusé**, qui descend de `claude-code` 2.1.281 (démarré le 25/09 à 09:44, pas de mise à
> jour la nuit). Cette demande-là n'explique donc pas directement la panne ; la piste de la
> confidentialité reste la première, sans être établie. La veille tranchera à la prochaine panne.
>
> **Établi le 26/09 par le journal de macOS** (`/usr/bin/log show` ; dans le shell de cette
> session, `log` est une commande intégrée de zsh qui masque l'outil — la première recherche, la
> veille au soir, n'avait donc rien lu) :
> - **22:24:24** : `en0 link INACTIVE` — le câble décroche ; le Wi-Fi (`en1`) était inactif, **le
>   double adressage n'a pas joué** ;
> - **22:24:28 – 22:24:30** : lien revenu, bail DHCP `.28` retrouvé, routes réappliquées ;
>   à 22:24:29, macOS **recharge les règles « Réseau local »** (95 règles d'applications) ;
> - **de 22:24:40 à 23:48** : `LocalNetwork: found bundle id com.anthropic.claude-code by PID`
>   **2 376 fois, toutes les 2 s** — au rythme exact des tentatives de l'enregistrement, et **jamais
>   avant la chute du lien**. macOS rattache la connexion à l'application qui a lancé le
>   programme, **Claude Code**, et la refuse : « No route to host ». Le ssh de 22:51, lancé depuis
>   le Terminal (déjà autorisé), passait ;
> - **26/09, 08:42** : même contrôle, accès accordé — la demande trouvée en attente ce matin avait
>   été acceptée.
>
> **Cause probable** : Claude Code s'était mis à jour le 25/09 à 08:08 (2.1.281), identité neuve
> pour macOS ; l'accès tenait jusqu'au rechargement des règles provoqué par la chute du lien. Les
> six silences d'avant 22:24 ne sont pas expliqués (aucun changement de lien journalisé).
> **Conséquences** : (1) co-atc lancé depuis Claude Code hérite de cette fragilité — une mise à
> jour de Claude Code plus une chute du câble, et co-atc ne reçoit plus rien, sans erreur visible,
> jusqu'à ce qu'on réponde à macOS ; **co-atc doit être lancé depuis le Terminal du propriétaire
> (ou un agent launchd), pas depuis la session de travail** ; (2) **les chutes du lien Ethernet
> (doc 20 : dix par nuit) déclenchent tout** — câble, prise ou port du commutateur à vérifier.
>
> **Recoupé par la station le 26/09** : son ping vers le Mac (toutes les 10 s) passe à 297 ms à
> 22:24:29 et 346 ms à 22:24:39, puis 7 ms, sans perte — cohérent avec la chute d'`en0`. Et la
> veille a vu, le 26/09, une panne d'une autre nature : **12:56:33, Python et `curl` en échec au
> même instant, retour après 50 s** — le câble de la station, tombé de 12:56:34 à 12:57:45 selon
> son journal. La veille distingue donc bien les deux cas. **La station a aussi ses chutes**
> (26/09 : 00:25, 00:51, 04:48, 06:43, 12:56) ; son réseau passe par un maillage Wi-Fi que le
> propriétaire garde en l'état (pas de boîtiers EBM68).
>
> **26/09, 15:51 : un programme orphelin perd le réseau local.** La veille, lancée à 10:57 par une
> session de travail, reçoit « No route to host » dès 15:51:05 — l'instant où l'application Claude
> redémarre et où cette session disparaît — pendant que `curl` passe ; le journal de macOS rattache
> chaque refus à `com.anthropic.claude-code`. **Aucune demande d'autorisation n'apparaît**, et un
> Python lancé depuis la nouvelle session joint la station sans difficulté. Un programme dont
> l'application lanceuse a disparu garde son rattachement à elle, et macOS lui refuse le réseau
> local sans rien demander. Règle de plus pour co-atc : **tout ce qui doit durer se lance depuis le
> Terminal ou launchd, jamais depuis une session de travail**, que l'application peut redémarrer à
> tout moment.

Relevé au passage, sans lien avec Q21 mais à corriger : le Mac porte **deux adresses
sur 192.168.1.0/24**, `en0` Ethernet en `.28` et `en1` Wi-Fi en `.48`, et le cache ARP
contient la station **sur les deux interfaces**.

```
? (192.168.1.10) at <mac de la station> on en0 ifscope [ethernet]
? (192.168.1.10) at <mac de la station> on en1 ifscope [ethernet]
```

*(La même adresse matérielle sur les deux lignes — c'est le fait ; l'adresse
elle-même est retirée du dépôt public, elle n'apporte rien à la lecture.)*

Un paquet peut partir par une interface et revenir par l'autre. Ça n'est pas la cause
de Q21 — le forçage de source l'a écarté — mais c'est une source d'intermittence
gratuite. **À voir avec le propriétaire** : couper le Wi-Fi quand la station est
utilisée par câble.

> **Piège de diagnostic, noté pour la prochaine fois.** J'ai conclu deux fois trop vite
> sur cette panne : au bac à sable de la session, puis au double adressage — les deux
> plausibles, les deux fausses. Pire, une coupure réelle de la station est survenue
> pendant le diagnostic et a fait échouer `curl` aussi, ce qui m'a fait *abandonner la
> bonne hypothèse* au moment où je la tenais. Une hypothèse qui explique l'observation
> ne vaut rien tant qu'un test ne l'a pas opposée aux autres.
>
> **Le confondant a un nom, depuis `20-captation-nuit.md`** : *« dix chutes du lien
> Ethernet entre 22 h et 8 h 45, toutes suivies d'un retour en 4 à 5 secondes »*. La
> coupure qui a brouillé le diagnostic en était une. Q21 tient — le test apparié
> `curl` 5/5 contre Go 0/5 au même instant l'établit indépendamment — mais je n'avais
> aucun moyen de distinguer les deux pannes sans ce relevé côté station.


### Q23 — Le lexique des compagnies date de 2014 *(nouvelle, 16/09)*

`assets/airlines.dat` est la base OpenFlights que l'amont embarque. Mesure sur les
avions réellement vus au-dessus de la station ce matin :

**77 opérateurs distincts, dont 23 absents du fichier — 30 %.**

```
BEL EFW EJU EZS FJO FRO FSF ITY JFA KMM LHX NJE
NOZ NSZ OYO RHH RUK VJH VLJ VXS WMT WUK XGO
```

Ce sont pour l'essentiel des compagnies nées ou renommées après 2014 : Malta Air
(2019), ITA Airways (2021), les filiales easyJet Europe et Suisse, Ryanair UK,
NetJets Europe. Le fichier ne peut pas les connaître.

**Ce que ça coûte** : un opérateur nommé vaut +0,40 au score et restreint les
candidats à sa flotte. Pour 30 % du trafic, ce levier est inerte et les chiffres
décident seuls. Observé en direct : *« Fraction Four Four Seven Lima »* a bien été
apparié à **NJE447L** — « Fraction » *est* l'indicatif radio de NetJets Europe —
mais par les chiffres seuls, le mot n'ayant rien trouvé dans le lexique.

**Résolu autrement, le 16/09 : on ne complète pas de mémoire, on relève.**

Quand une transmission a déjà été appariée à un avion **par les chiffres et les
lettres seuls**, le mot qui précède ses chiffres *est* l'indicatif radio prononcé.
Il suffit de le lire. `assets/spoken-operators.csv` est donc une **table de formes
entendues sur cette station**, pas une référence aéronautique — et les formes
déformées y ont leur place, « mazda » pour Malta Air étant utile précisément parce
que c'est ce que le modèle produit.

Relevé du 16/09, avec le nombre d'avions distincts et de transmissions qui
soutiennent chaque ligne : `fraction → NJE` (3 avions), `malta → WMT` (2
transmissions), `alpine → EJU`, `benavis → PVD`, `skynet → KMM`, et une douzaine
d'autres à une seule observation.

**Effet mesuré : nul sur les appariements.** Avec ou sans le fichier, 20 / 42 / 44
appariements sur les trois groupes — à l'identique. Il ne fait que monter les
scores des appariements que les chiffres avaient déjà décidés, et il fait
*légèrement monter le hasard*, parce que le bonus d'opérateur se déclenche aussi
dans les témoins mélangés.

Conservé malgré tout, pour deux raisons : il rend la trace honnête — `operator
named` apparaît quand l'opérateur a bien été reconnu — et il devient décisif si
`min_score` est un jour relevé, ce que `config.toml` documente comme option.

Détail relevé au passage : le chargeur d'`airlines.dat` exige `active == "Y"`.
C'est pour ça que **Ryanair** manquait — la ligne existe, elle est marquée inactive.


### Q24 — Le plancher d'auditeurs Icecast est 1, jamais 0 *(nouvelle, 16/09)*

Relevé par l'agent de la station : `filtre-voix` consomme `aero.mp3` en permanence
pour produire `aero-clair.mp3`. Le champ `auditeurs` de `/radio/etat` ne tombe donc
jamais à zéro tant que la chaîne tourne. **Tout test « personne n'écoute » doit
raisonner sur `auditeurs − 1`.** Le piège a failli annuler la captation nocturne en
silence.

**Vérifié : notre code n'est pas concerné** — rien dans `atc-scribe` ne lit
`/radio/etat`. À retenir tout de même pour D2, le mode de diffusion par canal : c'est
exactement là qu'un test de ce genre serait tentant.

Corollaire à garder en tête : **co-atc compte lui aussi comme auditeur.** Il tire
`aero.mp3` par un `ffmpeg` tant qu'il tourne. Une mesure d'audience faite pendant que
notre binaire tourne compte donc au moins deux auditeurs qui ne sont personne.


### D16 — Le fork touche le frontend pour la première fois *(16/09)*

Filtrer les avions dont la voix a parlé demandait un bouton. Jusqu'ici le fork
modifiait **0 % de `www/`**, ce qui garantissait l'absence de conflit de rebase du
côté que l'amont fait le plus évoluer. C'est fini, et c'est assumé : l'appariement
produisait une information que personne ne pouvait voir.

Trois fichiers touchés, et l'empreinte a été tenue au minimum :

| fichier | ajout |
|---|---|
| `www/index.html` | un bouton, 14 lignes |
| `www/app.js` | un réglage persisté, une bascule, un compteur, deux prédicats, une mise à jour en direct |
| `www/map/core/visibility-rules.js` | une fonction `isVisibleByVoice` et son branchement |

**Le serveur ne filtre pas.** `handleFilterUpdate` porte la note de l'amont :
*« Server-side filtering has been removed. All filtering is done client-side. »* On
suit ce choix — le serveur porte le fait sur chaque avion, le navigateur décide. Le
filtre côté serveur aurait été plus simple à écrire et à contre-courant de
l'architecture.

**Piège rencontré, à retenir.** L'application a **trois** prédicats de filtrage :
`_performFiltering` pour la liste, `aircraftPassesFilters` dans le store, et
`MapVisibilityRules.shouldShowAircraftOnMap` pour la carte. N'en modifier qu'un
donne une liste filtrée et une carte qui ne l'est pas — c'est exactement ce qui est
arrivé au premier essai, et seule la capture d'écran l'a montré.


### Q25 — Le bonus d'opérateur discrimine-t-il vraiment ? *(16/09 — répondue le jour même, dans l'autre sens)*

Prémisse d'origine : le complément de lexique faisait monter le hasard de 3,6 à 5,4 sur
l'amas, donc le bonus se déclencherait aussi bien sur une flotte prise au mauvais
moment. **Remesuré sous la règle de production, avec et sans le complément, sur quatre
fenêtres : le hasard bouge de ± 1 selon la fenêtre, sous la résolution de l'instrument
(écart-type 2 à 3 appariements sur huit témoins).** La prémisse n'est pas établie.

Réponse du panel (doc 23, A.5), vérifiée dans le code : **le bonus ne discrimine rien
par construction.** `restrict` garantit que tous les candidats survivants portent
l'opérateur nommé, ou aucun ; le +0,40 est un décalage constant qui ne change ni le
classement ni l'écart d'ambiguïté. Son seul effet est sur le plancher de 0,60 — et là
il porte une classe entière : les indicatifs à queue de lettres, `letters + operator
named = 0,70`. Six appariements sur 168 dans la journée en dépendent, cinq en fenêtres
de 51 min, pour **0,38 de hasard sur huit témoins : 93 % au-dessus**. C'est l'inverse de
la règle « à un près ».

**Décision : le bonus reste.** Le retirer éteindrait AFR89VR, AFR44SA, AFR78HN, AFR65AR,
AFR74UP — ceux que doc 19 crédite de la précision des tours.


### D17 — L'outil `-db` applique la règle de production *(16/09)*

Le panel a trouvé que `cmd/phraseology -db` ignorait `-strict`, `-min-score` et
`-context` : déclarés, jamais transmis. Les quatre fenêtres de groupes comptaient les
ambigus comme attachés. Corrigé — le chemin SQLite porte maintenant la mémoire par
fréquence et refuse les ambigus — et remesuré : au plus 3 appariements et 5 points
d'écart, aucune conclusion ne bouge (doc 19). Deux divergences avec la production
subsistent, listées en Q27.

### Q19 — *(réponse partielle, 16/09)*

La saturation divise presque par deux les appariements vrais sur 125,825 entre sa
moitié chargée et sa moitié calme — mais 124,350 et 124,625 montrent l'effet inverse.
« La charge nuit à la précision » n'est pas une loi sur ce corpus. Et 29 % des
appariements de 125,825 reposent sur le seul palier « suffixe » (18 % ailleurs).
L'occupation instantanée suggérée ici n'est pas calculable depuis la base :
`transcriptions` ne porte ni durée ni occupation. Ouverte.

### Q26 — La base de données ne tourne jamais *(16/09 — **corrigée le 20/09, voir D22**)*

Vérifié dans le code par le panel puis par moi : le stockage est ouvert **une fois** au
démarrage et jamais rouvert ; `ensureTodayDatabaseFile` (`cmd/server/main.go:415`)
crée un fichier vide pour le nouveau jour et le referme ; `cleanupOldDailyDatabases`
(`:437`) saute le fichier actif. Un processus qui tourne une semaine écrit *un* fichier
sans limite, que la rétention ne touche pas.

Mesuré le 16/09 : `adsb_targets` à 64,7 lignes/s, **1,49 Go en 4 h 15** (~8,4 Go par
jour pleine), dont 54 % de `raw_data` — une copie JSON de l'objet ADS-B complet. À
48 Gio libres : **disque plein en ~6 jours** de marche continue.

C'est un défaut de l'amont et **un bloqueur pour toute marche sans surveillance**. Le
correctif est du code — rouvrir la base à minuit, ou ne plus stocker `raw_data` — et un
candidat naturel à une pull request amont. Pas de veilleur : il dirait au jour 5 ce que
le code dit au jour 0.

### Q27 — Ce que les instruments hors ligne ne mesurent pas *(16/09 — **le compteur de rejet VAD existe et a répondu, voir D27**)*

Trois écarts entre les deux outils de mesure et la production, relevés par le panel :

1. **La phase n'est jamais renseignée** dans la flotte hors ligne (`-capture` comme
   `-db`), donc les deux pénalités de −0,25 de `matcher.go` (« en croisière, pas en
   atterrissage » ; « en vol, pas au départ ») ne sont validées par **aucune** mesure du
   dossier. `phase_changes` existe dans la base : à porter.
2. **Les 74 transmissions à lettres seules** (5 % de la captation) sont exclues des
   deux outils, qui exigent un indicatif chiffré — alors que la production les tente.
   Plafond mesuré ≤ 6,25 vrais sur 1 415 : petit, mais non mesuré.
3. **La fenêtre de flotte** est symétrique (± 60 s) hors ligne, unilatérale vers le passé
   en production (`GetAllAircraftWithLastSeenFilter(1)`).

Et un compteur absent des deux côtés : **le taux de rejet du VAD en production** n'est
journalisé nulle part au niveau courant (`local.go:194` en Debug ; le sidecar ne le
trace pas). « Le parasite est-il absent le jour ? » reste sans réponse tant qu'il
n'existe pas.


### D18 — L'amas 132-133 est identifié canal par canal *(16/09)*

Une passe de transcription **par canal** sur la captation nocturne — 462 transcriptions,
neuf minutes de calcul, chaîne de production à l'identique — a identifié ce que le mode
`amas-132-133` cherchait depuis sa création. Détail dans `24-identification-amas.md`,
catalogue mis à jour.

- **132,275 = Paris Contrôle, secteur d'arrivée de De Gaulle.** Trois preuves
  convergentes : médiane FL200 avec FL120–FL130 (descente, pas croisière), **14
  transferts vers 121,155** que le corpus nomme lui-même « de Gaulle », et un appel
  initial de pilote — *« hello Paris good evening… »*. C'est aussi le seul canal
  francophone de l'amas, 10 %.
- **132,825 = Brest Contrôle, probable.** Un appel initial — *« breast control
  hello… »* — sur neuf transcriptions seulement.
- **132,500 · 132,733 · 132,783 · 133,000 · 133,250 = croisière haute**, FL270 à FL390.
  Nature établie, organisme inconnu. 132,500 recoupe `16-grammaire-et-125933.md` :
  125,933 la citait trois fois, et son profil de niveaux le confirme de l'autre côté.

**L'amas n'est donc ni un organisme ni un étage** : c'est une tranche de spectre qui
contient au moins deux centres et deux fonctions.

### Q28 — Le flux mixé fabrique des moyennes trompeuses *(nouvelle, 16/09)*

`21-nuit-132-133.md` concluait de trente segments tirés au hasard que l'amas était « de
l'espace supérieur, FL160 à FL390 ». **Vrai en moyenne, faux canal par canal** :
132,275 porte 62 % de la parole de l'amas et c'est le seul canal qui ne soit pas en
croisière. Tirer au hasard dans un mélange déséquilibré, c'est échantillonner le canal
dominant en croyant échantillonner l'ensemble.

C'est un argument pour Q4 / D2 **distinct** de celui du doc 23 (R1, qualité de
transcription) : ici ce n'est pas la transcription qui souffre du mélange, c'est
**l'interprétation**. Et il n'est pas théorique — il a produit une conclusion fausse
dans ce dossier, corrigée le lendemain.


### D19 — Le dépôt reste un fork ; la rebasabilité cesse d'être une contrainte *(16/09)*

Q11 avait tranché « vrai fork » sur une mesure faite **avant** d'écrire une ligne. Après
38 commits, la même question se remesure — et la réponse se dédouble : **la forme du
dépôt était le bon choix, la règle de conception qui en avait été tirée ne l'est plus.**

**Ce que le fork est devenu, mesuré le 16/09 :** 38 commits, 85 fichiers,
**+11 663 / −210 lignes**, 63 fichiers nouveaux contre 22 fichiers amont modifiés. Le
`−210` est le chiffre qui compte : *nous ne supprimons presque rien chez eux.*

> Relevé sur les **38 premiers commits** du fork, c'est-à-dire avant les commits de
> publication qui suivent (D19 lui-même, le README anglais, la passe de sanitation).
> Rejouer `git diff --shortstat` aujourd'hui donne un autre chiffre : c'est normal,
> et le décompte ci-dessus est celui qui a motivé la décision.

Profondeur de nos modifications sur les fichiers amont, rapportée à leur taille :

| fichier amont | lignes | notre empreinte |
|---|---|---|
| `www/app.js` | 5 512 | **1,7 %** |
| `internal/adsb/service.go` | 1 987 | **0,2 %** |
| `www/index.html` | 2 268 | 3,1 % |
| `internal/api/handlers.go` | 2 157 | 3,7 % |
| `internal/frequencies/service.go` | 1 109 | 2,6 % |
| `cmd/server/main.go` | 486 | 24,7 % |
| `internal/api/routes.go` | 117 | 116 % *(mais 117 lignes)* |

Les seuls fichiers amont réellement réécrits sont **courts**. Les gros sont effleurés.
D16 annonçait « le fork touche le frontend pour la première fois » comme une rupture :
la mesure dit 1,7 %.

**Et l'amont n'a pas bougé : dernier commit le 3 mai 2026**, soit 136 jours. La réserve
honnête de Q11 — « un amont silencieux depuis 4 mois pourrait ne jamais reprendre » —
s'est vérifiée quatre mois de plus.

**Ce que la contrainte a coûté, en revanche, est concret.** Quatre accesseurs existent
*uniquement* pour ne pas modifier une signature amont :

```
DBOf(s *TranscriptionStorage) *sql.DB     expose un champ privé
(r *Router) Handler() *Handler            accesseur
(h *Handler) AttachRuntime(...)           câblage après-coup
(s *Service) SetVoiceIndex(...)           idem
```

Aucun n'existerait si le constructeur avait pris un paramètre de plus. Et D16 a hésité
longuement sur **un bouton de 14 lignes**. C'est de la discipline dépensée contre un
rebase qui n'a aucune raison d'arriver.

**Décision, en trois points :**

1. **Le dépôt reste un fork git** — historique amont, remote `upstream`, `LICENSE` MIT
   de Yegor S conservé tel quel, fork signalé dans le README. Ça ne coûte rien et c'est
   dû : l'interface, la carte OpenLayers et le suivi ADS-B sont leur travail, et
   représentent toujours l'essentiel du produit.
2. **La rebasabilité n'est plus une règle de conception.** On modifie un fichier amont
   quand c'est la façon la plus simple d'écrire la chose. Les quatre accesseurs
   ci-dessus restent en place — les défaire maintenant serait du bruit — mais on n'en
   écrit plus de nouveaux pour cette raison.
3. **Ce qui est contribuable reste identifiable**, parce que c'est utile à d'autres, pas
   par espoir de rebase : le sidecar de transcription et `local.go` (ils implémentent le
   contrat de **leur propre** `docs/LOCAL-STT.md`, jamais construit), le correctif de
   rotation de base (Q26, leur défaut), `internal/auth/` (ils n'en ont aucune) et
   `internal/api/server_handlers.go` (ils n'exposent aucun état opérationnel).

**Ce que ça ne change pas** : D7 (licence MIT), D6 (obligation de conserver la notice
amont), et le fait que `docs/` reste à eux.


### D20 — Le dépôt s'appelle `atc-scribe` *(16/09)*

D4 avait choisi `co-atc-local` le premier jour, avant qu'une ligne soit écrite, sur un
raisonnement de découvrabilité : *« c'est ce qu'on cherche quand on veut Co-ATC sans
OpenAI »*. Deux choses l'ont périmé.

**Le nom ne discrimine pas.** La description GitHub de l'amont dit déjà *« leverages
local SDRs »*, et son serveur est déjà cantonné au réseau local. « local » ne nomme donc
pas notre différence, qui est **l'absence d'IA dans le nuage**.

**Et la découvrabilité qu'il achetait est faible.** Mesuré le 16/09 sur l'API GitHub :
une recherche `co-atc in:name` renvoie **huit résultats AtCoder sur neuf**. On n'est pas
trouvé par le nom, on est trouvé par le réseau de forks — qui affiche le dépôt parent de
toute façon, quel que soit notre nom.

**Le motif décisif est ailleurs :** l'amont est muet depuis le 3 mai 2026 (D19) et le
projet a vocation à continuer sans lui. Un nom construit comme un suffixe de l'amont
dit le contraire de ce qu'on fait.

**Pourquoi `atc-scribe`.** Un scribe écrit ce qui a été dit, fidèlement, **sans
inventer** — et c'est l'axe de tout le dossier. La détection de voix existe parce qu'un
modèle nourri de silence rend des phrases plausibles (Q1, doc 21). La grammaire est
fermée pour la même raison. L'appariement se fait contre la flotte réellement vue,
jamais contre une liste imaginée. « Scribe » nomme la vertu que ce projet passe son
temps à défendre ; « hallucination » en est exactement le contraire. Et il dit ce que
fait le produit à quelqu'un qui n'a jamais entendu parler de Co-ATC.

Disponibilité vérifiée le 16/09 : **aucun dépôt GitHub nommé `atc-scribe` ni
`atcscribe`**. Forme avec tiret retenue — `atcscribe` se casse à l'œil, et le tiret suit
la convention de l'amont (`co-atc`).

**Ce qui ne change pas :** le dépôt reste un fork (D19), la licence MIT et la notice de
Yegor S restent (D6, D7), et l'attribution est portée par le README.

**Deux choses volontairement non renommées, et il faut savoir pourquoi :**

1. **Le binaire reste `co-atc`.** Q21 a établi que la permission « Réseau local » de
   macOS est accordée **par exécutable**. Renommer le binaire crée une identité neuve,
   donc une nouvelle demande d'autorisation — et si elle passe inaperçue, l'échec
   ressemble trait pour trait à une coupure réseau, ce qui a déjà coûté deux
   conclusions fausses en une journée. À faire le jour où le chemin du module Go
   changera aussi, en une fois et devant le clavier.
2. **Le chemin du module Go reste `github.com/yegors/co-atc`.** Le changer ferait
   marcher `go install` depuis notre dépôt, mais c'est la seule modification qui
   toucherait *chaque bloc d'import de chaque fichier Go* — et donc tout diff futur avec
   l'amont, y compris les pull requests que D19 veut garder possibles.

**Et deux documents gardent l'ancien nom à dessein** : `11-demande-station.md` et
`20-captation-nuit.md` sont de la correspondance datée entre agents. On ne réécrit pas
une lettre après coup.

### D21 — Le sidecar vit et meurt avec co-atc *(16/09)*

**Le défaut trouvé d'abord.** Rien ne vérifiait la présence du sidecar. co-atc démarrait
parfaitement — carte, ADS-B, flux audio, interface — et ne transcrivait rien : une ligne
d'erreur par transmission, rien de stocké. Sur une journée comme celle du 15, **≈ 1 300
lignes d'erreur et zéro transcription**, sans aucun signal dans l'interface. L'amont sonde
pourtant sa source ADS-B au démarrage (`ValidateSource`, `main.go:118`) ; c'est notre
moitié du produit qui n'avait pas la même garantie, et `local.go` est notre code.

**La mesure qui a tranché la forme.** J'avais d'abord recommandé un service permanent
(LaunchAgent) en arguant qu'il fallait garder les modèles chauds. Le propriétaire a
écarté la prémisse — *« co-atc est un outil lancé ponctuellement, il n'y a pas de raison
qu'un sidecar soit en permanence en route »*, et ça ne vaut pas plus pour qui teste
l'outil. Chiffres relevés dans la foulée :

| | durée |
|---|---|
| lancement du sidecar → `/health` répond | **1,56 s** |
| surcoût de la 1ʳᵉ transcription sur un processus neuf | **+2,2 s** (3,53 s contre 1,33 s) |

**Moins de quatre secondes à froid.** L'argument du démon permanent ne tient pas.

> **Au passage : `--preload` ne précharge pas les poids.** Le journal dit « model
> registered (1,06 s) puis (0,00 s) » — `load()` importe le module, mlx-whisper résout le
> modèle paresseusement à la première transcription. C'est bien là que partent les
> +2,2 s. Le commentaire du code le disait ; la mesure le confirme.

**Décision.** `[transcription.local] command` lance le sidecar avec le serveur et
l'arrête à la sortie. **Laissée vide par défaut** : le contrat de l'amont
(`docs/LOCAL-STT.md`) spécifie une *URL*, précisément pour que le service puisse vivre
ailleurs — autre terminal, autre machine, conteneur. Dans les deux cas, **la sonde
`/health` est obligatoire et co-atc refuse de démarrer sans elle**.

Trois comportements vérifiés en conditions réelles le 16/09 :

| cas | résultat |
|---|---|
| sidecar sain | lancé et sain en **1,2 s**, PPID = co-atc, s'arrête avec lui |
| ni sidecar ni commande | refus, code 1, message nommant les trois remèdes |
| commande cassée | refus en **1 s**, portant la sortie de l'enfant |

**Deux pièges rencontrés en l'écrivant, tous deux trouvés par un test :**

1. **`kill(pid, 0)` réussit sur un zombie**, et `cmd.ProcessState` n'est renseigné
   qu'après `Wait()`. Un sidecar mort au démarrage était donc indiscernable d'un sidecar
   qui charge, et l'opérateur attendait les 60 s pour un « n'a pas répondu » sans cause.
   Corrigé : l'enfant est moissonné dès son lancement.
2. **Il faut signaler le groupe de processus, pas le processus.** On a observé sur cette
   machine un `resource_tracker` de multiprocessing survivre à son parent (Q24 en a
   montré un). `Setpgid` au lancement, `kill(-pid)` à l'arrêt, et un test dédié le
   vérifie sur un petit-enfant.

**Un troisième piège, trouvé en répondant à « et il se ferme avec lui ? ».** La question
avait l'air rhétorique ; la vérifier a rapporté deux défauts.

- **`SIGHUP` n'était pas écouté.** Fermer la fenêtre du terminal tuait co-atc par l'action
  par défaut, sans arrêt propre : sidecar orphelin, port 8178 retenu.
- **Et surtout, les signaux étaient armés trop tard.** `signal.Notify` se trouvait après
  le démarrage de tous les services. Chronométré : le sidecar part à **+0,05 s**, le
  serveur finit de démarrer à **+1,78 s** — **1,7 s pendant lesquelles n'importe quel
  signal tuait co-atc sans nettoyage.** Je m'en suis aperçu parce qu'un essai a d'abord
  paru contredire un essai antérieur ; la première explication qui m'est venue était « mon
  banc d'essai est sale », et elle était fausse.

Corrigé en armant `signal.NotifyContext` **en tête de `main`**, avant toute création de
service. Effet de bord utile : le contexte sert aussi d'attente au sidecar, donc un Ctrl-C
pendant sa sonde interrompt le démarrage au lieu de le subir.

Vérification finale, **six cas** — `SIGTERM`, `SIGINT`, `SIGHUP`, chacun tiré dans la
fenêtre de course puis une fois tout démarré : co-atc s'arrête, le sidecar part, aucun
orphelin.

**Ce qui n'était pas couvert** : un `kill -9` sur co-atc orphelinait le sidecar. Écrit
ici comme une fatalité — *« aucun parent ne peut s'en prémunir »* — ce qui était vrai du
parent et faux de l'enfant. **Corrigé le 21/09, voir D35.** En revanche il n'y a **aucun appel `Fatal`
après le démarrage du sidecar** dans `main.go` (vérifié), donc le chemin d'arrêt normal
et tous les échecs de démarrage sont couverts.

**Contribuable en amont tel quel** (D19) : c'est leur contrat, leur seam, et leur manque.

### D22 — La base du jour tourne à minuit *(20/09)*

Q26 est corrigée. C'était le seul défaut qui empêchait une marche sans surveillance :
**~8,4 Go par jour, disque plein en six jours**, et rien dans le programme ne le disait.

**Pourquoi c'était plus qu'un `if` à ajouter.** Le fichier du jour était ouvert une fois
au démarrage, et **quatre types de stockage plus plusieurs services avaient chacun copié
le même pointeur `*sql.DB`**. Remplacer la connexion dans l'un ne changeait rien chez les
autres. `ensureTodayDatabaseFile` créait bien le fichier du lendemain — et personne n'y
écrivait jamais.

**La forme retenue : une poignée qui porte le même jeu de méthodes.**
`sqlite.DB` expose `Query`, `QueryRow`, `Exec`, `Begin` — exactement ce que le projet
utilise, mesuré avant d'écrire : 32 `Query`, 26 `Exec`, 7 `QueryRow`, 3 `Begin`, aucune
variante `Context`. Changer le type d'un champ de `*sql.DB` à `*DB` a donc suffi :
**zéro site d'appel modifié** dans toute la couche de stockage.

Trois détails qui comptent :

- **La fermeture de l'ancienne connexion part sur sa propre goroutine.**
  `sql.DB.Close` attend les requêtes déjà commencées, y compris un `*sql.Rows` qu'un
  lecteur parcourt encore ; fermer sur place rendrait la rotation aussi lente que le
  plus lent des lecteurs.
- **Une rotation qui échoue ne déplace pas la poignée** et ne tue pas le serveur : on
  continue d'écrire dans le fichier courant, l'erreur est journalisée, la minute
  suivante réessaiera.
- **Deux cadences** : la rotation est vérifiée chaque minute pour tomber près de minuit,
  la rétention reste horaire — et passe immédiatement après une rotation, puisque le
  fichier de la veille vient d'être relâché.

**Cinq tests**, dont les deux qui portent le sens : des stockages construits *avant* la
rotation écrivent bien dans le fichier d'après (c'est tout l'enjeu du pointeur copié), et
le fichier de la veille est réellement relâché, donc supprimable.

`GET /api/v1/server` annonçait `rotates: false` — un aveu honnête tant que c'était vrai.
Il annonce `rotates: true`.

**Non observé** : un vrai passage de minuit en production. Le mécanisme est testé, le
câblage est vérifié au démarrage ; la première rotation réelle reste à constater.
*(⚠️ Constatée après coup, le 23/09 : la rotation du 21/09 à 00:00:02 a bien eu lieu, et
le fichier qu'elle a ouvert n'avait pas de table `transcriptions` — voir D56.)*

**Candidat à une pull request amont** (D19) : le défaut est le leur, le correctif ne
touche que leur couche de stockage, et il n'ajoute aucune dépendance.

### D23 — Le dépôt est publié *(20/09)*

**https://github.com/chatelp/atc-scribe**, public, 94 commits (l'historique amont
compris), branche par défaut `main` alimentée par notre `local`.

Dépôt autonome plutôt que fork GitHub : la relation est portée par l'historique, le
remote `upstream`, le `LICENSE` de Yegor S conservé tel quel et le README qui l'annonce
en sous-titre. Un fork GitHub aurait ajouté le bandeau « forked from », mais les forks
n'apparaissent pas dans la recherche et leurs issues sont fermées par défaut — mauvais
échange pour un projet qu'on veut voir trouvé et repris (D19, D20).

**Contrôle avant publication**, sur *tous* les commits : `gho_`, `ghp_`, `github_pat_`,
`sk-` sous leur forme réelle (préfixe + longueur) — **zéro**. Un seul signal, et c'était
`Barsalogho_Airport`, un aérodrome du Burkina Faso dans le CSV OurAirports de l'amont.
À retenir : un scan de secrets sur préfixe seul lève des faux positifs sur des données
géographiques ; c'est la longueur qui discrimine.

### Q29 — La porte française : mesurée *(20/09)* — **décidée et en service, voir D24**

Q1 posait A (aiguillage par langue), B (un seul multilingue) ou C (affinage maison).
**Le cadre était faux** : le français de cette station n'est pas sur des fréquences
françaises, il est *entrelacé* sur les fréquences anglaises de Paris Contrôle — un a
priori par fréquence ne peut donc pas l'attraper. Et la vraie question n'est pas « quel
modèle » mais « un modèle ou deux ».

**Pourquoi deux.** Sur les 172 transmissions francophones, l'anglais et bofenghuang
apparient largement des enregistrements *différents* : 14 pour l'anglais seul, 12 pour
bofenghuang seul, **8 en commun**. Ils n'échouent pas au même endroit.

**Mesure, capture du 15/09, 1 378 clips, règle de production, 40 témoins.** Le mode
`-union` de `cmd/phraseology` unit les passes par enregistrement — **et donne aux témoins
les mêmes deux chances**, sans quoi on mesurerait la règle. Validé par un témoin
en+en, qui redonne l'anglais seul à l'unité près.

| cas | appariés | hasard | **vrais** | précision | calcul |
|---|---|---|---|---|---|
| anglais seul | 139 | 39,1 | 99,9 | **72 %** | ×1,00 |
| bofenghuang seul | 93 | 32,1 | 60,9 | 65 % | ×2,50 |
| union complète | 173 | 59,4 | 113,6 | **66 %** | ×2,50 |
| **union filtrée** | **151** | **42,6** | **108,4** | **72 %** | **×1,15** |

**L'union complète est un mauvais marché** : +13,7 vrais, mais la précision tombe de 72
à 66 %. Sur 34 appariements gagnés, une vingtaine sont du hasard — c'est-à-dire des
paroles attribuées au mauvais avion. Un manque se voit, une erreur plausible non.

**L'union filtrée est un bon marché.** On ne lance le modèle français que sur les 12,5 %
de transmissions dont la sortie anglaise *paraît* française : **+8,5 vrais, précision
inchangée à 72 %, coût ×1,15**. Elle capte 62 % du gain pour 10 % du surcoût.

**Le gain est réparti**, ce qui est le contrôle qui compte (doc 19) : +1,2 / +2,4 / +2,8
/ +2,2 sur quatre fréquences, −0,1 sur la cinquième. Aucune ne le porte seule.

> **Estimation contre mesure.** J'avais estimé le gain de l'union à +22 % en appliquant
> la précision de bofenghuang à ses appariements exclusifs. La mesure donne +13,7 %.
> L'erreur : **le hasard monte aussi** — deux textes, c'est deux chances de tomber juste
> par accident, et le hasard passe de 39,1 à 59,4. Une estimation qui oublie son propre
> témoin se trompe dans le sens qui arrange.

**Ce qui reste à trancher, et qui n'est pas une mesure :**

1. **Quel texte stocker ?** La mesure accepte un appariement venu de l'une *ou* l'autre
   passe ; `transcriptions.content` n'a qu'une colonne. Garder l'anglais et n'emprunter
   au français que l'indicatif ? Garder celui qui a apparié ? C'est une décision produit.
2. **Le modèle français est un lien symbolique vers un disque USB externe**
   (`/Volumes/Crucial X8/`, 5,7 Go). Aujourd'hui sans conséquence ; le jour où la chaîne
   en dépend, un disque débranché devient une panne.
3. **Une seule session** de mesure, celle du 15/09.
4. **La liste de marqueurs** qui définit la porte a été écrite avant de voir le moindre
   résultat (elle est dans `q1-francais.py`), mais le *recours* aux marqueurs, lui, vient
   d'avoir regardé les données. Une validation sur un corpus tenu à l'écart serait plus
   propre.

### D24 — Le second avis français est en service *(20/09)*

Q29 mesurée, décision prise : **la porte est implémentée et activée**. Q1 est close par
la même occasion, et pas dans le sens où elle était posée.

**Ce que Q1 demandait mal.** A (aiguillage par langue), B (un seul multilingue) ou C
(affinage maison) supposaient tous qu'on choisit *un* modèle. La mesure dit que les deux
échouent sur des enregistrements différents — 8 accords sur 34 appariements — donc la
bonne question était « un ou deux », et la réponse est deux, **mais pas tout le temps**.

**Comment ça marche.** Le sidecar transcrit avec le modèle anglais, comme avant. Si le
texte obtenu contient des mots français, il relit le même son avec le modèle français et
renvoie les deux. La porte s'ouvre sur 12,5 % des transmissions.

**Ce qui est stocké**, et pourquoi c'est important :

| colonne | contenu |
|---|---|
| `content` | **toujours** la lecture principale — le corpus reste comparable de bout en bout |
| `content_second` | la seconde lecture, quand la porte s'est ouverte |
| `callsign` | l'indicatif retenu |
| `callsign_source` | `en`, `fr`, ou **`en>fr`** quand les deux ont nommé des avions différents |

Le choix de toujours stocker la lecture principale n'est pas cosmétique : stocker
« celle qui a apparié » rendrait le corpus hétérogène — deux modèles selon la ligne — et
toute remesure ultérieure impossible. Sur les douze appariements que la porte fait
gagner, **un vient d'une boucle de dégénérescence** (*« 104, Alain, 8, 7, 11, 14, 15… »*
apparié à KLM1920 sur « 19, 20 ») : c'est exactement le texte qu'on ne veut pas voir
s'installer dans la base comme transcription.

**La règle d'arbitrage : la lecture principale gagne.** Elle tourne sur tout, et c'est sa
précision qui est mesurée — 72 % contre 65 %. Les deux n'ont jamais nommé d'avions
différents à l'intérieur de la porte sur la capture du 15/09 (8 accords sur 8), mais
**0 sur 8 ne borne rien** — la borne haute à 95 % est de 31 %. La règle existe donc, elle
est testée, et le désaccord est enregistré pour être remesuré quand le corpus aura grossi.

**Vérifié en service le 20/09** : porte fermée sur l'anglais (1,88 s pour 4,9 s d'audio),
ouverte sur le français, et sur le cas exact de la mesure —

```
principal : "rehear les niveaux unité nine zero Air France Zero Seven Two"
second    : "Enchire les niveaux, unité, 90, Air France, 062."
ADS-B     : AFR062
```

Sur le trafic réel, 1 transmission sur 7 a ouvert la porte dans les premières minutes.

**Ce qui reste fragile**, et qui n'est pas du code : le modèle français est un lien
symbolique vers un disque USB externe (`/Volumes/Crucial X8/`, 5,7 Go). La chaîne en
dépend désormais — un disque débranché fera échouer la seconde lecture. Le sidecar
journalise l'échec et la lecture principale tient, donc ce n'est pas une panne ; mais
c'est une dégradation silencieuse, et c'est le genre de chose que ce dossier n'aime pas.

> **Décision du propriétaire, 20/09 : le modèle reste sur le disque externe**, faute de
> place sur l'interne. Relevé au même moment : 38 Gio libres sur 460, `data/` à 2,9 Go
> et `whisper-lab/` à 4,9 Go.
>
> **Trois gisements de place, par ordre de rapport :**
>
> 1. ~~3,5 Go de modèles éliminés~~ — **supprimés le 20/09**, voir ci-dessous.
> 2. **`data/co-atc-2026-09-16.db`, 1 847 Mo** : le fichier gonflé par le défaut corrigé
>    en D22. La rétention à 7 jours l'efface le 23/09 sans rien faire.
> 3. **`raw_data` pèse 57 % de chaque base** (remesuré le 20/09). Voir Q31 : ce n'est
>    **pas** du mort — j'ai écrit le contraire ici et c'était faux — mais du redondant.

**La panne est rendue visible *(20/09, à la demande du propriétaire)*.** Puisque la
chaîne dépend d'un disque amovible, le silence devenait le vrai risque : débranché, la
transcription continue, le serveur tourne, et le seul symptôme est *moins d'avions
identifiés sans cause énoncée*.

- `/health` du sidecar déclare chaque modèle configuré, si un chemin local résout encore,
  et les compteurs de secondes lectures réussies et échouées.
- co-atc le lit au démarrage et **avertit modèle par modèle** — sans refuser de démarrer :
  la transcription marche, c'est une capacité qui manque, pas le service.
- `GET /api/v1/server` porte un bloc `transcription` pour qui regarde plus tard.

Deux choix de conception qui méritent d'être dits : une sonde qui expire **n'invente pas
une panne** — la dernière lecture tient, marquée `stale` ; et le plafond est à **deux
secondes**, parce que le sidecar décode pendant des secondes entières et qu'un point de
mesure opérationnel ne doit pas bloquer derrière lui.

Vérifié dans les deux sens le 20/09 : chemin absent → avertissement au démarrage et
`status: degraded` avec le motif ; chemin présent → `status: ok` et le second avis annoncé.

### D25 — Deux des trois modèles français sont supprimés *(20/09)*

**Ce qui a failli être une erreur.** J'allais conclure « les deux perdants sont
supprimables » sur leur score *isolé*. Or le gain de la porte vient de la
**complémentarité**, pas de la qualité isolée : un modèle plus faible pourrait très bien
être plus complémentaire. Il fallait mesurer l'union avec chacun, pas leur score seul.

**Mesure, union filtrée, capture du 15/09, 40 témoins :**

| second avis fourni par | appariés | hasard | **vrais** | précision | gain |
|---|---|---|---|---|---|
| *(aucun — anglais seul)* | 139 | 39,1 | 99,9 | 72 % | — |
| **`bofenghuang/whisper-large-v3-french`** | 150 | 42,6 | **107,4** | **72 %** | **+7,5** |
| `bofenghuang/…-distil-dec16` | 148 | 43,5 | 104,6 | 71 % | +4,7 |
| `pierreguillou/whisper-medium-french` | 139 | 39,5 | 99,4 | 72 % | **−0,5** |

**Et une troisième lecture fait perdre :**

| lectures | appariés | hasard | vrais | précision |
|---|---|---|---|---|
| anglais + bofenghuang | 150 | 42,6 | **107,4** | **72 %** |
| anglais + bofenghuang + distil | 151 | 45,5 | 105,5 | 70 % |

Un appariement de plus, **2,9 de hasard en plus**. C'est la leçon de l'union complète en
plus petit : chaque lecture supplémentaire est une chance de plus de tomber juste par
accident, et passé un point le hasard monte plus vite que le signal.

**Supprimés** : `mlx-fr-distil` (2,1 Go) et `mlx-fr-pierreguillou` (1,4 Go). Le premier
n'est pas nul — il apporte 63 % de ce qu'apporte le retenu — mais il est **dominé** :
moins bon partout, et ajouté par-dessus il dégrade. Aucune configuration mesurée ne
justifie de le garder.

**37 Gio → 40 Gio libres**, `whisper-lab/` de 4,9 à 1,4 Go.

**Rejouable sans eux** : leurs 172 transcriptions et les unions construites avec elles
sont conservées en JSON dans `whisper-lab/q1-francais/`. Et leur provenance est notée
ici, donc un retéléchargement est possible — les identifiants Hugging Face sont dans le
tableau ci-dessus et dans `14-mesure-modeles-francais.md`.

**Non fait** : rapatrier `mlx-fr-bofenghuang` (5,7 Go) du disque externe vers l'interne,
ce que ces 3,5 Go rendraient possible pour un coût net de 2,2 Go. La dépendance au disque
amovible subsiste — elle est au moins visible depuis D24.

### Q30 — 8 % de vrais appariements : où part le reste ? *(nouvelle, 20/09)*

Question du propriétaire après avoir vu le filtre « entendu à la radio » monter lentement :
**après tout ce travail, on est à 8 %.** Le chiffre est juste et mérite d'être regardé en face.

**L'entonnoir, capture du 15/09, règle de production, 40 témoins :**

| | | |
|---|---|---|
| transmissions transcrites | 1 332 | 100 % |
| portant un indicatif plausible | 888 | **67 %** |
| appariées à un avion visible | 139 | **10 %** |
| vraies, hasard déduit | 100 | **8 %** |

**84 % des candidats ne trouvent aucun avion.** Tout est là.

**Ce qui n'est PAS en cause**, mesuré et non supposé :

| variante | appariés | vrais | précision |
|---|---|---|---|
| **production** — 3 chiffres, ambigus refusés | 139 | **99,8** | **72 %** |
| ambigus acceptés | 139 | 98,8 | 72 % |
| fenêtre 120 s | 143 | 98,1 | 69 % |
| fenêtre 300 s | 152 | 96,0 | 63 % |

Élargir la fenêtre **fait perdre** : plus d'appariements, plus de hasard, moins de vrais.
Le refus des ambigus est gratuit. Les réglages de D14 tiennent.

**Ce qui est en cause**, d'après un échantillon au hasard des 1 193 non appariées —
trois familles, et une seule est un échec de notre chaîne :

1. **Rien à apparier.** *« okay »*, *« two eight zero »*, *« definite refer »* — un
   collationnement partiel, un accusé de réception. Les compter comme des échecs fausse
   le dénominateur.
2. **La transcription est fausse sur les chiffres.** *« Franske Trone Tango Zulu »*,
   *« Rotter Air Two Cent One Two »*, *« trait mole base five three four »*. Aucune règle
   d'appariement ne rattrape ça : c'est le modèle acoustique.
3. **Le seuil de trois chiffres** écarte le reste.

**Un angle mort du doc 19, trouvé le 20/09 :** le balayage n'a testé que **3 et
4 chiffres**, jamais 2. Mesuré maintenant :

| seuil | appariés | vrais | précision |
|---|---|---|---|
| 3 chiffres *(production)* | 139 | 99,8 | **72 %** |
| **2 chiffres** | **246** | **128,2** | **52 %** |

**+28 % de vrais, et vingt points de précision perdus** — près d'un indicatif affiché sur
deux serait faux. C'est le contraire de ce que ce dossier défend depuis le début : un
manque se voit, une erreur plausible non. **Non retenu, mais désormais mesuré.**

**Les trois voies, par ordre de ce que la mesure suggère :**

1. **Un meilleur modèle acoustique sur les chiffres.** Seule voie qui attaque la cause.
   C'est la stratégie C de Q1 — un affinage maison sur de la phraséologie — jamais tentée.
   Et ça recoupe D24 : le second avis français gagne +8,5 % non pas en corrigeant la
   règle, mais en corrigeant **les chiffres** (*« Zero Seven Two »* contre *« 062 »*).
2. **Un dénominateur honnête.** On ne sait pas combien de transmissions *contiennent
   réellement* un indicatif. Si c'est 40 %, alors 10 % d'appariés vaut 25 % du possible,
   pas 10 %. **C'est le seul usage qui justifie vraiment les 120 annotations** — et il
   les justifie complètement.
3. **Pas le seuil à deux chiffres**, sauf changement de doctrine assumé.

### Q31 — `raw_data` : redondant, pas mort *(20/09 — **corrigé le 21/09, voir D32**)*

**Correction d'abord.** Q26 puis D24 affirmaient que `raw_data` était *« une copie JSON de
colonnes déjà analysées, que rien ne relit »*. **C'est faux, vérifié dans le code le
20/09** : `getLatestADSBData` (`internal/storage/sqlite/aircraft.go:445`) le désérialise
pour reconstituer l'objet `ADSBTarget` complet. J'ai repris l'affirmation d'un document à
l'autre sans la vérifier dans les sources — exactement ce que `CLAUDE.md` interdit au
point 4.

**Mais le gisement existe, sous une autre forme.** La requête qui le lit porte
`ORDER BY timestamp DESC LIMIT 1` : **on ne lit jamais que la dernière ligne de chaque
avion**, et on l'écrit sur toutes. À 82 lignes/s, c'est du redondant, pas du mort.

**Le coût, mesuré le 20/09 sur une fenêtre de dix minutes en marche continue :**

| | |
|---|---|
| `adsb_targets` | **82,2 lignes/s** (cohérent avec les 64,7/s du 16/09) |
| poids moyen | ~1 441 octets/ligne |
| croissance | **407 Mo/h, soit 9,5 Go/jour** |
| dont `raw_data` | **57 %** |

**Et c'est un mur, pas une gêne.** À 7 jours de rétention : **66 Go au régime établi pour
39 Go libres** — le disque se remplit en **~4 jours**. La rotation de D22 a borné chaque
*fichier* ; c'est la *fenêtre de rétention* qui est maintenant la contrainte.

| | volume au régime établi |
|---|---|
| tel quel, 7 jours | **66 Go** ❌ |
| rétention 3 jours | 28 Go |
| `raw_data` sur la dernière ligne seulement, 7 jours | ~29 Go |
| les deux | ~12 Go |

**Non tranché.** Réduire la rétention est gratuit et immédiat mais perd de l'historique ;
ne garder `raw_data` que sur la dernière ligne demande du code et un choix de schéma
(table séparée, ou effacement périodique des lignes anciennes). **À décider avant toute
marche continue de plus de quatre jours.**

### Q32 — L'ATIS sature-t-il vraiment, ou est-ce le pic DC du tuner ? *(répondue le 20/09 — non, et la question d'après était la mauvaise)*

**Trouvé par l'agent de la station, pas par nous, et ça aurait invalidé la mesure.**

Q9 tient depuis le 15/09 que l'ATIS de Saint-Cyr *« sature le récepteur »* — un souffle
continu par-dessus la voix, à +49,4 dB. La demande envoyée le 20/09 en découlait : une
échelle de gain pour trouver le réglage qui le rend intelligible.

**L'agent a relevé ce que la demande ne disait pas : la fréquence centrale.** La station
a une convention, écrite en commentaire dans `fixe136.tmpl` :

```
centerfreq = 135.800;   # decale de 475 kHz : le pic DC du tuner ne tombe pas sur le canal
```

Un gabarit centré **sur** 131,025 place le pic DC du tuner exactement sur la porteuse de
l'ATIS — et *« un souffle continu par-dessus la voix »* est précisément ce que ça produit.

**Ce que ça aurait coûté sans lui.** Quatre prises à quatre gains, toutes centrées sur la
porteuse, toutes également mauvaises — et la conclusion « l'ATIS est irrécupérable, même
à gain réduit ». Fausse, bien formée, indistinguable d'une vraie. C'est la faute que ce
dossier a déjà payée en Q21 et Q22 : **une hypothèse qui explique l'observation ne vaut
rien tant qu'un test ne l'a pas opposée aux autres**, et j'ai écrit une demande entière
sans en opposer aucune.

**Protocole retenu**, cinq prises de 4 min dont 3 analysées, avant 22 h pour éviter le
parasite secteur :

| prise | centre | gain | rôle |
|---|---|---|---|
| A | 130,550 (−475 kHz) | 40,2 dB | témoin propre |
| B · C · D | 130,550 | 32,8 · 25,4 · 16,6 dB | l'échelle de gain |
| **E** | **131,025 (sur la porteuse)** | 40,2 dB | **témoin de confondant** |

Et un second amendement, également juste : **`salves.py` ne peut pas départager une
porteuse permanente** — elle ne ferme jamais le squelch, il rapporterait une salve unique
sur les cinq prises. Remplacé par `ffmpeg -af volumedetect` (RMS et crête, la crête
montrant l'écrêtage) plus une transcription de chaque prise.

**Conséquence déjà acquise, quel que soit le résultat** : le gain est un réglage **du
tuner**, appliqué à toute la fenêtre de 2,56 MHz. Si l'ATIS n'est lisible qu'à gain
réduit, il ne pourra pas cohabiter avec des canaux faibles — le livrable serait un **mode
dédié `atis-131025`** déclenché à la demande, pas un canal ajouté à un groupe existant.

> **Le garde de `salves.py` était déjà en place** depuis la nuit du 15 au 16 ; notre
> demande le réclamait comme une tâche à faire. Document périmé, corrigé.

---

#### La réponse, mesurée par l'agent de la station le 20/09 à 21 h 45

**Le confondant n'était pas là.** A (décalé) et E (sur la porteuse) sont à **0,1 dB l'une
de l'autre**, en RMS comme en crête, histogrammes superposés. Le gabarit dit pourquoi :
`highpass = 300` retire le terme quasi continu avant la sortie. La précaution était déjà
prise, ailleurs, et probablement pas pour cette raison. **Mon hypothèse était fausse ; la
prise qui la teste valait quand même d'être faite**, c'est la seule façon de le savoir.

**Et la prémisse de Q9 était fausse aussi.** La prise témoin, au gain de 40,2 dB auquel la
station tourne tous les jours, rend **déjà** un texte exploitable. L'ATIS n'était pas
irrécupérable : il n'avait jamais été transcrit. *« Sature le récepteur »*, écrit le 15/09,
décrivait une écoute, pas une mesure — et tenait depuis cinq jours.

| prise | gain | RMS | crête | éch. à 0-3 dB de la butée | transcription | calcul |
|---|---|---|---|---|---|---|
| A témoin | 40,2 | −15,1 | −0,1 | 11 / 200 / 1 416 / 6 392 | exploitable | 115 s |
| **B** | **32,8** | −15,5 | −0,7 | 2 / 82 / 651 / 3 763 | **la plus propre** | 114 s |
| C | 25,4 | −16,4 | −1,0 | 1 / 18 / 148 / 1 117 | boucle et se répète | 273 s |
| D | 16,6 | **−35,5** | −2,0 | 0 / 1 / 3 / 12 | échec | 26 s |
| E sur porteuse | 40,2 | −15,1 | −0,2 | 15 / 214 / 1 462 / 6 582 | exploitable, noms déformés | 234 s |

**Ce que le gain fait, et que le niveau ne montre pas.** De A à C le RMS ne bouge
quasiment pas (−15,1 → −16,4) pendant que la population d'échantillons à moins de 3 dB de
la butée **s'effondre d'un facteur 6**. On retire l'écrêtage sans rien perdre : la
normalisation rattrape le niveau, l'écrêtage non. **Chercher l'écrêtage dans le RMS, c'est
le chercher là où il n'est pas.** Domaine utile 40 → 25 dB, point recommandé **32,8**.
Sous 20 dB le lien casse et whisper hallucine sa sortie de silence classique.

**Le temps de calcul est un indicateur de détresse.** Quand le décodeur peine, il boucle,
et son temps explose : 115 s pour la prise propre, 273 s pour la même durée d'audio à gain
trop bas. Signal gratuit, non prévu, et potentiellement utilisable en production.

#### Mais le livrable espéré n'existe pas

**L'ATIS de Saint-Cyr, à 21 h, n'est pas un ATIS météo.** C'est une boucle
d'auto-information bilingue : service non assuré, piste 11G/29D fermée, FATO hélicoptère
fermée, taxiways Bravo et Charlie fermés, transit vertical interdit sauf mission d'État,
renvoi au supplément AIP 113/26.

**Ni vent, ni QNH, ni piste en service, ni niveau de transition.** Q9 justifiait
l'opération par *« la piste en service, le type d'approche, le niveau de transition »* :
rien de cela n'est diffusé. Ce qu'on récupère est du contenu de type NOTAM. Ça a une
valeur — mais pas celle qui fondait la demande. **Aucune lettre de version** non plus : la
question de comparabilité est sans objet, pour une autre raison que celle imaginée.

**Q9 est donc doublement corrigée** : l'obstacle technique n'existait pas, et le contenu
attendu non plus. **Q7 (météo française) ne peut pas s'appuyer sur cet ATIS.**

#### Ce qui sert vraiment à notre chaîne

**Un : le forçage d'une langue mutile l'autre, confirmé de l'extérieur.** Les cinq prises
sont décodées `language="fr"`, et les segments anglais de la boucle ressortent mâchés dans
les cinq. C'est **l'image miroir exacte de D24/D31** — où c'est le forçage anglais qui
mutilait le français, et où l'union portée par un second avis rendait +11,7 %. Deux
mesures indépendantes, deux sens opposés, même conclusion : **sur une bande bilingue, une
passe monolingue perd l'autre langue, quelle qu'elle soit.** La décision « un modèle ou
deux, mais adossés » ne repose plus sur un seul corpus.

**Deux : notre corpus de nuit a été enregistré au gain qui écrête.** La station tourne à
40,2 dB tous les jours, et le témoin A montre 6 392 échantillons à moins de 3 dB de la
butée. ⚠️ **Ne pas surinterpréter** : c'est mesuré sur un émetteur au sol très proche à
+49 dB. Un avion à 40 NM arrive des dizaines de décibels plus bas et n'écrête certainement
pas. **La mesure ne se transporte pas d'un émetteur fort à un émetteur faible** — savoir
si le gain quotidien écrête les *avions* demande de le mesurer sur des avions, ce qui n'a
pas été fait.

### Q33 — La boucle de jour contient-elle la météo ? *(en attente, décidé le 21/09)*

La séance a eu lieu à 21 h, aérodrome fermé. **La boucle diurne d'un terrain ouvert est
une autre boucle** et peut porter le vent, le QNH et la piste en service — c'est-à-dire
exactement ce que Q9 cherchait. Une séance, vingt minutes de jour, mode `atis-131025` déjà
en place au point recommandé (`gain = 32.8`, `centerfreq = 130.550`, déclenchable par
`aero-mode atis-131025`).

**Mise en attente le 21/09.** La question est propre et sa réponse vaut pour Q7, mais le
coût a changé de nature : la nuit la bascule ne dérangeait personne ; **de jour elle coupe
`gros-porteurs` vingt minutes à l'heure où le propriétaire écoute**. Et ce n'est pas sur
le chemin critique — ce qui bloque la reconnaissance vocale, ce sont les 120 annotations.

**Ce qu'on ne saura toujours pas** : la stabilité d'un jour à l'autre. Une prise par gain,
une séance. Et whisper n'est pas parfaitement reproductible — la transcription de E est un
peu moins bonne que celle de A, ce qui **ne suffit pas** à conclure quoi que ce soit sur
le pic DC au-delà du niveau.


### D26 — Le veilleur de mode a lancé 17 co-atc en une nuit *(21/09)*

**Une panne que j'ai entièrement fabriquée**, et qui mérite d'être écrite en entier parce
que la faute est structurelle, pas une coquille.

**Ce que le veilleur devait faire.** Suivre `/radio/etat` et aligner co-atc dessus :
pause quand la station quitte `gros-porteurs` pour la mesure ATIS, relance au retour.
Écrit et lancé le 20/09 à 20 h 58.

**Ce qu'il a fait.** Il a détecté la bascule en 12 secondes, laissé co-atc en pause
correctement, puis, au retour à 21 h 20, **relancé une instance toutes les vingt secondes
jusqu'à 9 h 34.** Dix-sept tournaient au réveil.

**La cause, en une ligne :**

```bash
tourne() { pgrep -f "$PROJET/bin/co-atc"; }      # cherche /Users/.../atc-scribe/bin/co-atc
demarre() { cd "$PROJET"; nohup ./bin/co-atc ... }   # la ligne de commande est ./bin/co-atc
```

Le motif absolu ne matche jamais une ligne de commande relative. **Le test « tourne-t-il
déjà ? » répondait toujours non.**

**La faute de méthode, qui est le vrai sujet.** J'ai vérifié que le veilleur *détectait*
la bascule — et c'est tout ce que j'ai vérifié. **Je n'ai jamais testé qu'il ne
relancerait pas un processus déjà vivant**, qui est la seule propriété qui définisse un
superviseur. Un test aurait pris trente secondes : lancer, attendre un tour, compter.

**La cascade.** 17 instances se disputant la même base SQLite ont produit des tempêtes de
`SQLITE_BUSY` ; les écritures de transcription ont commencé à échouer vers 7 h *(⚠️ faux :
dès 00:00:02, et pour une autre raison — le fichier de minuit n'avait pas de table
`transcriptions`, voir D56)*. Et la
charge a fait expirer les appels `curl` du veilleur lui-même, qui a conclu *« station
injoignable »* pendant neuf heures — **mon bug a cassé mon propre instrument de mesure**,
et `accord` valait `True` tout du long.

**Un trou dans la supervision du sidecar, révélé par là.** Chaque nouvelle instance
lançait son sidecar, qui échouait à prendre le port 8178 ; mais la sonde `/health`
réussissait, parce que **le sidecar de la première instance répondait**. `Start()` sonde
avant de vérifier que son propre enfant a survécu, donc co-atc démarrait en utilisant le
sidecar d'un autre. À corriger : vérifier `exited()` avant de conclure au succès.

**Le coût.** 21 Go de disque, une charge de 6,4 toute la nuit, une base contaminée
(197 doublons exacts le 20, 652 le 21), et 13 Go d'archive dont l'essentiel est redondant.

**Le veilleur est retiré.** Pas corrigé : la valeur qu'il apportait — éviter un relais
humain de quelques minutes — ne justifie pas un processus qui lance des serveurs sans
surveillance.

### D27 — Le parasite secteur, mesuré depuis le Mac *(21/09)*

Q27 demandait un compteur de rejet du détecteur de voix en production : *« Le parasite
est-il absent le jour ? reste sans réponse tant qu'il n'existe pas. »* Il existe depuis
le 20/09, et la nuit a répondu.

**Taux de rejet du VAD, par heure, 66 860 clips archivés :**

| | | | | |
|---|---|---|---|---|
| 20h **10 %** | 21h 16 % | 22h 61 % | 23h 61 % | |
| 00h 86 % | 01h 88 % | 02h **96 %** | 03h **98 %** | 04h 95 % |
| 05h 72 % | 06h 47 % | 07h 26 % | 08h **4 %** | 09h **5 %** |

**C'est la courbe du parasite secteur**, que la station documentait comme montant dès
17 h et saturant de 22 h à 7 h. **Première mesure côté Mac, et elle la confirme
indépendamment.** Réponse à Q27 : oui, le parasite est absent le jour — 4 % à 8 h contre
98 % à 3 h.

> **Le contrôle qui rend le chiffre valide malgré D26** : à 8 h et 9 h, les 17 instances
> tournaient toujours, et le taux est à 4-5 %. Le nombre d'instances ne pilote donc pas
> le taux — c'est une propriété de l'audio, pas du nombre de lecteurs. Sans ce contrôle,
> la courbe n'aurait rien valu.

**Et ça chiffre ce que le VAD sauve.** À 3 h du matin, **98 % de ce qui franchit le
squelch n'est pas de la parole**. Sans lui, la chaîne transcrirait 11 000 clips de bruit
en une nuit — et un modèle nourri de bruit ne rend pas le silence, il rend des phrases
plausibles. Le VAD n'est pas une optimisation, c'est ce qui empêche la base de se remplir
de faux.

### Q33 — Les deux lectures se contredisent plus que prévu *(nouvelle, 21/09)*

D24 notait 8 accords sur 8 dans la porte, en soulignant que **0 sur 8 ne borne rien**.
La nuit donne le premier vrai relevé, sur la base du 21 :

| provenance de l'indicatif | nombre |
|---|---|
| `en` — la lecture principale | 116 |
| `fr` — la seconde seule | 1 |
| **`en>fr` — désaccord, principal retenu** | **3** |

**Sur les quatre cas où la lecture française a aussi apparié, trois ont nommé un avion
différent.** La règle d'arbitrage, que je prédisais quasi inutile, tranche donc
régulièrement. Reste à savoir **qui a raison** — et ça, aucune mesure ne le dit encore :
il faudrait écouter les trois clips, qui sont archivés.

### D28 — Le corpus de la nuit, compacté *(21/09)*

Le sidecar écrivait du **WAV non compressé** : 12,6 Go pour 13 heures, quand le corpus du
15/09 tient en 24 Mo pour 1 468 clips parce qu'il est en MP3. Erreur de conception de
l'archivage, corrigée après coup.

**Le débit a été mesuré, pas choisi.** Dix clips tirés au hasard, transcription du WAV
comparée à celle du MP3 :

| débit | compression | transcriptions identiques |
|---|---|---|
| 32 kbit/s | 11,8× | **3/10** |
| **64 kbit/s** | **5,9×** | **9/10** |
| 96 kbit/s | 4,0× | 8/10 |
| 128 kbit/s | 3,0× | 9/10 |

**À 32 kbit/s la compression change le texte** — un corpus dont la transcription dépend
du codec ne vaut rien comme référence. À 64 le palier est atteint et monter n'améliore
plus : le 1/10 restant n'est pas un problème de débit mais la sensibilité du modèle sur
des clips marginaux.

**Deux sorts.** Les **13 122 clips transcrits** sont le corpus : tous convertis, aucun
perdu. Les **53 738 rejetés par le VAD** sont du bruit et avaient déjà produit la courbe
de D27 ; il en reste **402, trente par heure**, stratifiés — parce que *« le VAD avait-il
raison de les jeter ? »* reste ouvert et ne se répond pas sur des clips supprimés.

**12,6 Go → 1,04 Go, zéro échec, zéro fichier manquant**, les 28 marqueurs conservés. Le
WAV n'était supprimé qu'après vérification que le MP3 existait et n'était pas vide.

> **Ce qu'il faut changer dans le sidecar** : écrire directement en MP3 64 kbit/s à
> 16 kHz. Le WAV n'a servi à rien qu'à occuper douze fois la place.


> **Déplacé le 23/09** sur le disque externe, `/Volumes/Crucial X8/whisper-corpus/nuit-20260920`,
> et remplacé dans `whisper-lab/` par un lien — renommé le même jour
> `audio/2026-09-20-gros-porteurs-nuit` (D50) —, mais **ce corpus n'est lisible que disque
> branché**. Copie vérifiée fichier par fichier avant
> suppression de l'original (14 252 fichiers, SHA-256 identiques). Gardé parce que c'est
> **le seul enregistrement existant** des approches de Roissy — le trafic visé par D48 — :
> la station n'archive pas le groupe `gros-porteurs`. Trace aussi dans
> `whisper-lab/CORPUS.md` et dans un `LISEZMOI.md` à côté du corpus.
>
> Le même jour, **315 Mo supprimés du Mac** après vérification d'une copie sur la station :
> la nuit de la campagne de gain, l'ADS-B décodé des 12, 14 et 15/09, des fichiers d'essai.
### D29 — Les bases brutes sont remplacées par des extraits *(21/09)*

**D13 disait « la base ne s'efface jamais », et son motif était juste** : *« cette base
est la seule trace locale du croisement radio/ADS-B, et elle ne se reconstitue pas toute
seule »*. Ce motif ne tient plus, parce que la trace a été extraite.

**Ce que pesaient les bases** : 11,57 Go pour cinq journées. Décomposé :

| | |
|---|---|
| les transcriptions — l'irremplaçable | **~2,3 Mo** (6 105 lignes) |
| l'historique ADS-B brut | **11,5 Go** (7,6 M lignes) |

**L'historique brut ne sert qu'à une chose** : savoir quels avions étaient visibles à
quel instant, pour vérifier qu'un indicatif transcrit correspond à un avion réel. Ça se
résume, et le format existait déjà — celui que `cmd/phraseology -adsb` consomme.

**11,57 Go → 339 Mo, 34× plus petit.** Extraits dans `whisper-lab/extraits/` : un fichier
ADS-B et un fichier de transcriptions par jour.

**Vérifié avant de supprimer, et le premier test était mauvais.** J'ai d'abord voulu
rejouer la mesure de référence du 15/09 avec l'extrait de la base du même jour :
**0 appariement**. La cause n'était pas l'extrait — la base du 15 couvre 16 h 07 à
22 h 21, alors que la capture de référence va de 12 h 05 à 14 h 43. **co-atc ne tournait
pas encore**, et `adsb-captation.json` vient du `globe_history` de la station, pas de sa
base. Deux sources différentes pour la même journée.

**Le bon test, sur la nuit du 20 au 21**, où la base couvre bien la période :
**11 467 transmissions, 1 000 appariements, 736 vrais, 74 % de précision** — cohérent
avec les 72 % du corpus du 15/09. L'extrait rejoue l'appariement.

> **Et il révèle l'ampleur de D26** : le processeur en direct n'a enregistré que
> **238 appariements** cette nuit-là, contre **1 000 au rejeu**. Les écritures échouaient
> sur `SQLITE_BUSY` à cause des 17 instances. Les quatre cinquièmes du travail de la nuit
> n'ont jamais atteint la base.

**Contrôle d'intégrité avant suppression** : 6 105 transcriptions extraites contre 6 105
en base, à l'unité près, et les douze fichiers relus sans erreur.

**La règle amendée** : ce qui ne s'efface jamais, ce sont **les transcriptions et le
résumé ADS-B**. La base brute, elle, est un format de travail — et l'historique ADS-B est
de toute façon reconstituable depuis `globe_history` sur la station (Q18).

### D30 — Le panneau de réglages serveur, et la borne du journal *(21/09)*

Deux pièces qui attendaient depuis le 16/09, dans l'ordre convenu alors —
*« réglages serveur → auth → écriture »*. L'écriture existe enfin.

**La borne du journal.** L'amont écrit sur la sortie standard : la taille de ce dans quoi
on la redirige n'est bornée par rien. Mesuré ici, ce n'est **pas** un problème de place en
régime normal — 0,7 Mo/h, **17 Mo par jour**, trois ordres de grandeur sous la base. La
borne est pour le cas qui est arrivé : D26 a mis **103 Mo dans un fichier en une nuit**.

Trois bornes, parce qu'elles attrapent des choses différentes — **un fichier par jour**
(« que s'est-il passé la nuit du 20 ? » est une question qu'on pose à un fichier), un
**plafond de taille** pour le jour qui déraille, un **nombre de fichiers** pour que le
dossier cesse de grossir. Défauts : 100 Mo × 7, soit ~120 Mo en régime normal.

Quatre-vingts lignes plutôt qu'une dépendance, et **sept tests** — dont : un redémarrage
en cours de journée *ajoute* au journal du jour au lieu de le tronquer (c'est justement
la partie qu'on va chercher après un plantage), un fichier plein est *sauté* et non
rouvert (sinon le plafond n'en est plus un), et l'élagage ne supprime jamais le fichier
en cours d'écriture.

**L'écriture des réglages.** `PUT /api/v1/server/settings`, derrière le même groupe
authentifié que le reste — la rétention décide quand des données sont supprimées, ce
n'est pas un bouton pour qui atteint le port. Un champ absent **garde sa valeur** plutôt
que de devenir zéro : un client qui ne veut changer que le niveau de journal ne doit pas
mettre la rétention à rien sans le savoir.

**Le panneau.** Une section « Server » dans le panneau existant de l'amont, et la
séparation est délibérée : **tout ce qui est au-dessus est une préférence de navigateur**
gardée en `localStorage`, tout ce qui est en dessous appartient au serveur. L'un vous
suit d'une machine à l'autre, l'autre décide quand des données disparaissent.

**Deux réglages modifiables, le reste rapporté.** Niveau de journal et rétention changent
à chaud ; le chemin du journal, le plafond, le nombre de fichiers demandent un
redémarrage et sont donc affichés sans être éditables. **Un panneau qui échoue en silence
à appliquer la moitié de ce qu'il montre est pire qu'un panneau qui dit laquelle.**

Et il dit ce qui ne va pas plutôt que de l'enjoliver : `NEVER ROTATES` en rouge si la
base ne tourne pas, `UNBOUNDED` en jaune si le journal n'est pas borné, le détail de
l'erreur si un modèle de transcription est injoignable, et le nombre de jours avant
disque plein en rouge sous sept jours.

**Baisser la rétention demande une confirmation** qui nomme le nombre de fichiers qui
seront supprimés au prochain balayage. C'est exactement le conseil que j'ai donné le
20/09 sans en mesurer la conséquence, et qui aurait détruit les bases du 15, 16 et 17.

**Vérifié dans le navigateur, pas seulement par `curl`** : la section rend, le sélecteur
applique, « Saved. » s'affiche, le serveur bascule en `warn`, le fichier
`runtime-settings.json` le garde, et le journal note qui l'a fait.

### D31 — Q29 se réplique sur un corpus indépendant *(21/09)*

La réserve numéro 3 de Q29 disait : *« une seule session de mesure, celle du 15/09 »*.
Elle est levée. La nuit du 20 au 21 fournit un second corpus — autre nuit, autres
fréquences (`gros-porteurs` au lieu de `orly-approche`), autre trafic, **11 474
transmissions** contre 1 378.

| | appariés | hasard | **vrais** | précision |
|---|---|---|---|---|
| lecture anglaise seule | 1 000 | 264,7 | **735,3** | **74 %** |
| + second avis, porte | 1 122 | 300,5 | **821,5** | 73 % |

**+86 appariements vrais, soit +11,7 %.** Q29 mesurait **+8,5 %** sur le corpus du 15/09.
Deux corpus indépendants, même conclusion, magnitude du même ordre.

**Et la porte s'ouvre sur 12,2 %** des transmissions ici, contre **12,5 %** mesuré hors
ligne le 20/09. À trois dixièmes de point. Le détecteur de français n'était pas réglé sur
ce corpus-ci et s'y comporte pareil.

**Une nuance honnête** : Q29 annonçait « précision inchangée », ici elle passe de 74 à
73 %. Un point, dans le sens attendu — deux lectures, deux chances de tomber juste par
accident. La conclusion ne bouge pas ; la formulation « inchangée » était un peu large.

**Ce que ça vaut.** C'est la première réplication indépendante du dossier. Toutes les
autres mesures — le choix du modèle anglais (D12), les seuils (D14), la porte (D24) —
reposent encore sur la seule après-midi du 15 septembre. Celle-ci ne repose plus.

> **Et la précision de 74 % elle-même est une réplication** : 72 % sur le corpus du
> 15/09, 74 % ici. Le chiffre n'était pas une propriété de cette après-midi-là.

### D32 — `raw_data` déménage dans la table `aircraft` *(21/09)*

Q31 avait établi le fait et s'était trompée sur le remède. Le champ n'est **pas** mort —
`getLatestADSBData` le désérialise pour reconstituer l'objet ADS-B complet — mais il est
**redondant** : les deux requêtes qui le lisent portent `LIMIT 1` et `MAX(timestamp)`,
donc elles ne veulent que **la ligne la plus récente de chaque avion**. Il était écrit sur
les 82 lignes par seconde et lu sur 0,05 % d'entre elles.

**Le correctif n'est pas un élagage, c'est un déménagement.** La table `aircraft` existe
déjà, avec `hex` en clé primaire — **une ligne par avion**, exactement ce que les deux
requêtes reconstruisaient à grands frais. `raw_data`, `source_type`, `registration` et
`aircraft_type` y vont ; `adsb_targets` ne les porte plus.

**Mesuré sur cinq minutes de trafic réel :**

| | avant | après |
|---|---|---|
| octets par ligne | 1 441 | **445** |
| croissance | 418 Mo/h | **112 Mo/h** |
| par jour | 9,8 Go | **2,6 Go** |
| **à 7 jours de rétention** | **69 Go** | **18 Go** |

**−73 %**, mieux que les 57 % attendus de la seule part de `raw_data` : retirer la colonne
allège aussi la ligne et les index.

**Et la lecture devient une recherche par clé primaire** au lieu d'un balayage trié par
date. La requête par lot perd sa jointure sur `MAX(timestamp)` *et* la liste d'identifiants
passée deux fois.

**Ce que ça débloque, et c'était le but.** À 50 Gio libres et 18 Go au régime établi,
**co-atc peut enfin tourner sans surveillance** — il ne le pouvait pas au-delà de quatre
jours. C'était le dernier défaut bloquant de la mission.

**Vérifié** : 155 avions avec `raw_data` dans `aircraft`, **zéro octet** dans
`adsb_targets`, l'API rend les 155 avec leurs données complètes, et une base à l'ancien
schéma migre en place — 8 colonnes à 12, ligne préservée.

**Contribuable en amont** : c'est leur schéma, leur défaut, et le correctif rend leurs
deux requêtes plus simples et plus rapides.

### D33 — Un co-atc refuse de démarrer sur le sidecar d'un autre *(21/09)*

Le trou révélé par D26, et qui explique comment dix-sept serveurs ont partagé un sidecar :
chacun lançait le sien, chacun échouait à prendre le port 8178, **et chacun était rassuré
par la réponse du premier**. `Start()` sondait `/health` et concluait au succès sans
vérifier que *son propre* enfant avait survécu.

**Une sonde qui réussit dit que quelque chose est là, pas que c'est le vôtre.**

**Le temps ne peut pas trancher.** Un enfant qui échoue à prendre le port meurt en
quelques millisecondes, et savoir si sa mort a été moissonnée quand la première sonde
revient est une course — vérifié : le premier essai du correctif échouait pour cette
raison. Une temporisation n'aurait réglé le problème qu'en moyenne.

**Donc le sidecar déclare son PID** dans `/health`, et le Go compare au **groupe de
processus** de son enfant — `Setpgid` au lancement, donc tout descendant le partage, ce
qui couvre aussi une commande qui serait un script d'enveloppe. C'est exact, pas
statistique.

Trois détails qui comptent :

- **Un sidecar qui ne déclare pas de PID est toléré.** Refuser sur un champ absent
  casserait une installation qui marche, et le champ est neuf.
- **Le cas délibéré reste permis** : `command` vide signifie « je lance le sidecar
  moi-même », et une réponse étrangère est alors exactement ce qu'on veut.
- **Avant d'échouer, on laisse l'enfant finir de mourir** (500 ms) pour que son propre
  message — *« address already in use »* — atteigne l'erreur que lit l'opérateur. Et on
  l'arrête, parce que refuser de démarrer n'est pas une raison de laisser un processus
  derrière soi.

**Vérifié en conditions réelles**, exactement le scénario de la nuit : un co-atc tourne,
un second est lancé, il sort en code 1 avec *« pid 72978 answered, ours is … »*, et il
reste **un co-atc et un sidecar**. Plus deux tests : le refus, et le fait que le cas
délibéré passe toujours.

### D34 — La première configuration se fait dans le navigateur *(21/09)*

L'amont livre sans authentification et un README qui dit de ne jamais exposer le serveur.
D30 avait donné les réglages ; il manquait la façon d'**répondre à la question** plutôt
que d'éditer du TOML.

**Une page web, pas une fenêtre native.** co-atc est un serveur : lui donner une fenêtre
demanderait une bibliothèque graphique en dépendance, alors qu'il sert déjà une interface.
C'est aussi le motif habituel — Home Assistant, Portainer, Nextcloud font tous ça.

**Pas de jeton de configuration, et le propriétaire avait raison de le contester.** Ma
première proposition imprimait une URL à usage unique au démarrage. Objection :
*« tous les autres outils auto-hébergés que j'ai n'ont jamais eu besoin de ça »*. Exact,
et le raisonnement tient — **si le serveur écoute sur `127.0.0.1`, atteindre la page c'est
déjà être sur la machine**, la même frontière de confiance que le terminal où on aurait
lancé `-add-user`. Un jeton n'y ajoute rien.

Le jeton n'aurait servi que si le serveur était joignable de l'extérieur — et ce cas est
couvert plus nettement par un **refus de démarrer**. Un mécanisme au lieu de deux.

| situation | comportement |
|---|---|
| aucun compte, écoute sur `127.0.0.1` | la page s'affiche, deux choix |
| aucun compte, écoute ailleurs | **refus de démarrer**, avec le remède dans le message |
| un compte existe | la page disparaît définitivement |

> **Complété le 23/09 par D51** : le choix « cette machine uniquement » n'était pas
> enregistré, et la page revenait sans fin. Il l'est, et les deux choix se changent
> depuis les réglages.

**Les deux choix sont écrits pour être compris, pas pour être cliqués vite.** « Cette
machine uniquement » dit que c'est ce que fait l'amont, *choisi plutôt que subi*. « Joignable,
avec un compte » dit qu'il faudra **encore** TLS ou un proxy de confiance déclaré, parce
qu'un mot de passe en clair sur un réseau qu'on ne contrôle pas ne protège rien.

**Les comptes vont dans `configs/users.json`, pas dans `config.toml`** — que le programme
ne réécrit jamais, puisqu'il porte les tableaux de mesures en commentaires. Les deux
sources sont fusionnées, le fichier l'emportant sur la configuration : c'est la page que
quelqu'un vient d'utiliser.

**Sept tests**, dont ceux qui portent le sens : la page cesse de pouvoir créer un compte
dès qu'il en existe un (elle travaille sans être authentifiée), le fichier contient
l'empreinte Argon2id et pas le mot de passe, il est en `0600`, et les comptes survivent au
redémarrage — sinon la page se déferait en silence.

**Vérifié dans le navigateur** : la page s'affiche, refuse un mot de passe trop court,
crée le compte, bascule sur la connexion, et les données passent de 200 à 401 sans session.

> **Deux défauts de ma part au passage, tous deux du même genre** : un `replace` de CSS
> visant 14 espaces d'indentation là où il y en avait 12 — sans assertion, donc silencieux
> —, et une structure HTML où `<b>` et `<span>` étaient deux enfants flex côte à côte au
> lieu d'être empilés. Les deux vus à l'écran, aucun des deux par le code.

### D35 — Le sidecar s'arrête quand co-atc meurt brutalement *(21/09)*

D21 déclarait ce cas incorrigible : *« un `kill -9` sur co-atc orpheline le sidecar,
aucun parent ne peut s'en prémunir »*. La phrase était vraie et la conclusion fausse —
**le parent ne peut rien faire, l'enfant peut remarquer.**

Trois orphelins constatés en une journée : le `resource_tracker` de Silero le 20/09, les
seize sidecars morts de la nuit, et un troisième au cours des essais du 21. Le dernier
tournait depuis une heure avec `PPID 1` quand le propriétaire a demandé ce qui restait en
arrière-plan.

**Le mécanisme** : un fil de veille regarde `os.getppid()` toutes les cinq secondes et
appelle `os._exit(0)` s'il change. Sortie dure et non gracieuse, délibérément : l'arrêt
propre d'uvicorn attend les requêtes en cours, et le client qui les aurait terminées vient
de mourir.

**Le critère est déclaré, pas deviné — et mon premier essai le devinait.** J'avais écrit
« si le PPID devient 1, s'arrêter », en me protégeant du seul cas où il valait déjà 1 au
démarrage. Ça tuait un sidecar lancé à la main avec `nohup ... &` dès la fermeture du
terminal, puisque son parent devient init aussi. **co-atc pose donc `COATC_SPAWNED=1`
dans l'environnement du processus qu'il lance**, et le sidecar ne veille que si elle est
là. Une autre implémentation du contrat ignorera simplement la variable.

**Les deux cas, vérifiés :**

| | |
|---|---|
| lancé par co-atc, `kill -9` sur le parent | *« watching parent 62829 »* puis **arrêt en 3 s** |
| lancé à la main | *« not spawned by co-atc; not watching »*, **vivant après 15 s** |

> Son dernier message n'apparaît nulle part : sa sortie passe par le tuyau de co-atc, qui
> vient de mourir. C'est inhérent, et sans conséquence — la preuve est dans le délai, qui
> vaut exactement l'intervalle de veille.

### D36 — Un flux mélangé n'a pas de fréquence *(21/09)*

Constaté par le propriétaire en vérifiant l'interface : la barre du bas annonçait
**« VHF melangee (amas 132-133, espace superieur) — 132,7625 MHz »** alors que la station
diffusait `gros-porteurs` depuis la veille. **L'interface affichait la mauvaise
fréquence.**

Et sa question était la bonne : *« c'est un outil qui a vocation à être public, ma config
audio multiplexée est très spécifique à mon setup, comment on traite ça ? »*

**Le défaut est générique, pas local.** L'amont modélise une entrée de fréquence comme
*un* canal avec *un* nom et *un* nombre en MHz. C'est vrai d'un canal et faux d'un
mélange — et le mélangeur de RTLSDR-Airband est une fonction standard. `frequency_mhz`
devient donc **facultatif** : absent, l'interface affiche `mixed` au lieu d'inventer un
nombre. Rien ne change pour qui a de vraies fréquences.

**La partie spécifique reste dehors.** Ce que le mélange porte change quand la station
bascule de groupe, et co-atc ne peut pas le savoir. Embarquer `/radio/etat` en amont
serait exactement le mauvais échange : personne d'autre ne l'a. Le mécanisme est donc
générique — `PUT /api/v1/frequencies/{id}/label` — et **la politique reste chez celui qui
sait**. `radio-ctl`, un `cron`, une ligne de shell : qui change ce qui est diffusé
l'annonce, avec ce qu'il utilise déjà. Une étiquette vide restaure le nom configuré.

**Quatre tests**, dont celui qui refuse une étiquette trop longue plutôt que de la
tronquer — une étiquette coupée en deux est pire qu'une absente, parce que personne ne
cherche un message qu'on ne lui a pas montré.

### D37 — Un compte dans le fichier active l'authentification *(21/09)*

**Un défaut que j'ai créé une heure plus tôt et qui laissait le serveur ouvert.**

Après un redémarrage : le compte existait dans `configs/users.json`, la page de
configuration ne se proposait plus — **et `enabled` valait `false`**. Le serveur avait
l'air configuré et ne protégeait rien, ce qui est pire que l'un ou l'autre.

La cause : `AddUser` activait l'authentification **en mémoire**, mais au démarrage le
service se reconstruit depuis `config.Auth.Enabled`, et la page n'écrit jamais dans
`config.toml` — délibérément, puisqu'il porte les mesures en commentaires.

**La règle** : un compte présent dans le fichier de comptes **active** l'authentification,
quoi que dise la configuration. Créer un compte depuis la page *est* l'acte de l'activer.
En revanche, des comptes écrits à la main dans `config.toml` continuent d'obéir à
`auth.enabled` — un `enabled = false` écrit à la main est une décision, pas un oubli.

Trouvé en testant autre chose : `GET /api/v1/frequencies` répondait **200 sans session**.
Le genre de défaut qu'aucun test ne trouve quand on n'a pas pensé à redémarrer.

> **Au passage** : les sessions vivent en mémoire et ne survivent pas à un redémarrage.
> C'est cohérent — une session est un jeton de présence, pas une donnée — mais ça veut
> dire qu'on se reconnecte après chaque mise à jour du binaire.

### D38 — `git add -A` sur un dépôt qui porte maintenant de l'état *(21/09)*

**J'ai publié l'empreinte du mot de passe du propriétaire sur un dépôt public**, cinq
minutes après avoir écrit D37 sur le fait qu'un compte doit protéger quelque chose.

`git add -A`, et `configs/users.json` est parti avec le commit. Le fichier n'était pas
dans `.gitignore` — je l'avais créé le matin même sans l'y ajouter, alors que ses trois
voisins y étaient déjà (`config.toml`, `runtime-settings.json`, `data/`). L'oubli n'est
pas d'avoir mal choisi : c'est de ne pas avoir choisi du tout.

**Ce qui change dans le dépôt** : `configs/users.json` est ignoré, retiré de l'index et
du commit, `main` réécrit et re-poussé de force.

**Ce qui ne s'efface pas** : GitHub continue de servir le blob par le SHA du commit
abandonné (`08f80431`) tant qu'il n'a pas ramassé ses miettes. Vérifié — il répond encore.
Dépôt à 0 fork, 0 étoile, 0 observateur, exposition de quelques minutes ; mais les flux
d'événements publics enregistrent les poussées, donc ce n'est pas zéro.

Une empreinte Argon2id `m=64 Mio, t=3` n'est pas un mot de passe : l'attaquer coûte cher.
C'est une raison de ne pas la distribuer, **pas** une raison de la laisser traîner. **Le
mot de passe se change** — c'est le seul geste qui rend la fuite sans objet.

**La règle** : dans ce dépôt, `git add -A` se relit avant de committer. La règle
« pas de secret dans le dépôt » ne se tient pas par intention, elle se tient par
`.gitignore` — et un fichier d'état nouveau s'y ajoute **dans le même geste qui le crée**,
pas au commit suivant.

> **Au passage, une lacune** : il n'existe aucun moyen de changer un mot de passe.
> `AddUser` refuse dès qu'un compte existe. Changer le sien, c'est aujourd'hui supprimer
> `configs/users.json` et refaire la page de premier lancement. Ça marche, et ça ne
> devrait pas être la réponse.

### D39 — Montrer les modèles à l'annotateur, mais le dire *(21/09)*

**Demandé par le propriétaire**, et l'argument tenait en deux moitiés qui ne valaient pas
pareil : *« je risque d'avoir du mal à reconnaître la voix sans aide, et en plus ça
permettrait d'indiquer quel modèle s'est le mieux débrouillé »*.

**La seconde moitié ne justifie rien.** « Quel modèle s'est le mieux débrouillé » se
calcule *après*, en confrontant chaque sortie à la vérité terrain. C'est ce à quoi une
vérité terrain sert, et c'est plus solide qu'une impression formée clip par clip. Ce que
cet argument justifie vraiment, c'est de **faire tourner les modèles sur les 120 clips** —
fait, voir plus bas.

**La première moitié est réelle.** Une référence pleine de `[unclear]` ne mesure rien.

**Mais montrer coûte, et le coût n'est pas neutre.** Qui lit « Fox-trot Golf Alpha Bravo »
l'entend. Le biais va **systématiquement dans le sens qui flatte le modèle** : la référence
se rapproche de sa sortie, son score monte, et le chiffre a l'air d'une mesure. C'est la
faute que ce dossier redoute le plus — celle qui ne ressemble pas à une faute.

**Le dispositif retenu** — ni caché, ni affiché :

1. **Caché par défaut.** Les lectures ne sont pas dans la page ; elles ne partent au
   navigateur qu'au clic, via un point d'accès séparé. Un annotateur qui essaie de ne pas
   regarder ne les trouve pas dans l'inspecteur.
2. **La tentative à l'aveugle est photographiée** à l'instant précédant l'affichage, et
   conservée à côté du texte final. C'est la seule version non ancrée qui existera jamais.
3. **Le clip est marqué `assisté`**, pour que les clips aidés et non aidés se notent
   séparément.
4. **Les lectures sont sans étiquette et dans un ordre tiré au hasard par clip**, graine
   20260921, conservée dans le fichier. On ne favorise pas un modèle qu'on ne sait pas
   lire — et la comparaison entre modèles, qui est le livrable, reste propre.

**Ce que ça donne en plus** : la taille du biais, **mesurée**. Combien de fois voir le
modèle a changé la réponse. Personne n'a ce chiffre d'habitude, parce que personne ne
garde la tentative d'avant.

**Les lectures**, produites par `whisper-lab/candidats-jeu-de-test.py` : les deux modèles
sur **tous** les clips, pas seulement là où la porte de D31 s'ouvrirait — ici on cherche
l'aide maximale et une comparaison complète, pas à rejouer l'arbitrage de production.
120/120 en 7 min, 0 échec, 2 sorties vides côté anglais. `bofenghuang` est **resté sur le
disque externe** (5,7 Go) : l'interne est déjà juste, et il se charge en 4,7 s de là.

> **Un défaut trouvé à l'écran, pas dans le code** : `save()` appelle `render()`, qui
> repliait le panneau à l'instant où il s'ouvrait. Le bouton basculait, la pastille
> apparaissait, et les lectures disparaissaient — tout avait l'air de marcher. Corrigé en
> retenant à quel clip le panneau ouvert appartient. **Aucune relecture ne l'aurait
> trouvé ; un clic l'a trouvé tout de suite.**

> **Aperçu franc, sur le premier clip** (Chavenay, français attendu) : *« Il m'a autorisé
> à vous toucher, je rappelle qu'en arrière »* contre *« Heli Maus Zero Zero Two Shreds
> rappelment arrire »*. **Les deux sont faux, et faux différemment.** C'est exactement
> pourquoi l'ancrage est dangereux ici : il n'y a pas de bonne réponse à copier.

### Q34 — Le gain quotidien coûte-t-il quelque chose sur les avions ? *(répondue le 22/09 : **non**)*

Q32 a montré que le témoin à **40,2 dB — le réglage quotidien de la station — écrête** :
6 392 échantillons à moins de 3 dB de la butée. Nous avons écrit dans la foulée que la
mesure **ne se transporte pas** : elle porte sur un émetteur au sol à +49 dB à quelques
kilomètres, et un avion à 40 NM arrive des dizaines de décibels plus bas.

Donc la question reste entière, et elle n'est pas mineure : **si le gain quotidien abîme
le signal des avions, tout notre travail de reconnaissance vocale part d'une source
dégradée sans que nous le sachions.**

**Demande envoyée le 21/09** (`whisper-lab/demande-gain-avions.md`) : gain alterné toutes
les 30 min entre 40,2 et 32,8 dB, de 22 h à 7 h, groupe `en-route-et-descente-cdg`,
18 blocs, 9 par réglage. Alterné et non pas une moitié de nuit chacun — **le trafic varie
d'un facteur 30 entre 23 h et 03 h ; une nuit coupée en deux comparerait les heures, pas
les gains.** Mesure par appariement ADS-B, sans annotation.

#### Pourquoi la nuit ici, et pourquoi la nuit ne peut rien pour le français

Le propriétaire proposait des captations nocturnes pour affiner le modèle. **Mesuré sur
cinq jours d'ADS-B, en ne comptant que les immatriculations `F-XXXX`** :

| Heure UTC | Points ADS-B | dont aviation générale |
|---|---|---|
| 00 h | 24 945 | **0** |
| 03 h | 25 396 | **0** |
| 14 h | 272 756 | **23 871** |
| 21 h | 51 748 | **0** |
| 23 h | 35 019 | **0** |

**Le contrôle qui rend la mesure valable** : aux heures creuses le récepteur *regardait* —
24 945 points à 00 h, 51 748 à 21 h. Ce n'est pas un trou de données, c'est une absence
réelle. L'aviation générale française vole **de 07 h à 22 h locales**, pointe à 16 h, et
disparaît la nuit. **Une nuit de captation ne contiendrait aucune transmission
francophone d'aéroclub.**

La question française demande donc du jour, ce qui coupe `gros-porteurs` aux heures
d'écoute — même coût que Q33. **Et surtout : nous n'avons pas encore mesuré ce que nous
avons.** 120 transmissions tirées, les deux modèles passés dessus (D39), taux d'erreur
inconnu. Demander des données avant d'exploiter les existantes est précisément la faute
que ce dossier se reproche depuis le début.

#### Le confondant signalé dans la demande

**Le squelch.** S'il est à seuil absolu, baisser le gain de 7,4 dB rend le squelch plus
sélectif : des transmissions faibles n'ouvriront plus du tout. On mesurerait alors une
sélectivité en croyant mesurer une transcription. **Un gain qui capte moins mais transcrit
mieux ce qu'il capte n'est pas un gain qui gagne** — il faudra compter les deux, ouvertures
et taux d'appariement. Question posée à la station, qui seule connaît la sémantique de son
seuil.

#### Réponse de la station, le 21/09 : le confondant se dissout, deux amendements

**Le seuil est relatif** (`squelch_snr_threshold`, un rapport signal/bruit au-dessus du
plancher estimé), jamais le seuil absolu que RTLSDR-Airband expose par ailleurs. Baisser le
gain baisse le signal **et** le plancher ensemble : la sélectivité est préservée, il ne
faut **rien** décaler — décaler créerait le biais qu'on cherchait à éviter. 10 dB sur six
canaux, 25 dB sur 132,783. Réserve datée : l'équivalence tient tant que le plancher de
bruit reste au-dessus du plancher de quantification ; à 16,6 dB il n'y était plus, à
32,8 dB on est encore linéaire.

> L'agent précise **n'avoir pas pu faire imprimer au binaire son seuil effectif** : sa
> réponse repose sur le nom du paramètre et le comportement observé, pas sur une valeur
> lue. Et il fournit de quoi le vérifier — **compter les déclenchements par bloc avant de
> regarder les transcriptions** : si le seuil était absolu, ils s'effondreraient à 32,8.

**Amendement 1 — le vrai coût des bascules est le squelch, pas le trou.** Le trou audio
fait 2 à 3 s (dérivé de la campagne ATIS : cycle 267 s, audio 264,10 s). Mais l'auto-squelch
réestime son plancher à chaque redémarrage, deux minutes. Sur 18 bascules, **le biais va
dans le même sens à chaque début de bloc : il ne se moyenne pas.** Écarter les deux
premières minutes de chaque bloc (−6,7 %) plutôt qu'allonger les blocs, ce qui perdrait la
décorrélation d'avec la courbe de trafic.

**Amendement 2 — nous avions choisi la fenêtre et la bande du parasite secteur.** 22 h–07 h
sur 132-133, c'est exactement là où il est caractérisé : 92 à 98 % de fichiers vides toute
la nuit. L'alternance décorrèle, la comparaison tient — mais **le piège d'interprétation
est sérieux et il nous rattrape** : entre 22 h et 7 h, l'émetteur le plus fort n'est
peut-être pas un avion. *« Lisez l'écrêtage transmission par transmission, jamais sur le
bloc entier. Sinon vous mesurerez le chauffe-eau. »* Nous aurions lu le bloc.

#### Ce que la campagne peut détecter — et pourquoi ça change la mesure principale

De ~1 040 transmissions porteuses de parole annoncées, moins l'exclusion : **~485 par
bras.** Calculé le 21/09 :

| Mesure | Base | Écart détectable |
|---|---|---|
| Appariement ADS-B | 10 % | **3,8 pts, soit 38 % en relatif** |
| Transcription non vide | 50 % | 6,3 pts (13 %) |
| **Écrêtage par transmission** *(continu)* | — | **0,13 écart-type** |

**Le taux d'appariement, notre mesure habituelle, est ici sous-dimensionné** : il faudrait
un effet de 38 % en relatif pour le voir. Mais la question principale n'en a pas besoin —
« le gain quotidien écrête-t-il les avions ? » se répond en mesurant l'écrêtage sur les
transmissions porteuses de parole, **sans transcrire**, sur tout le n, et sur une mesure
continue. C'est exactement la lecture que le garde-fou de l'amendement 2 impose : **son
piège était aussi la bonne mesure principale, et nous ne l'avions pas vu.**

Une seule nuit suffit pour la question principale. Le volet transcription sera rapporté
avec son incertitude, sans conclusion sous 38 %.

#### Prédiction posée avant les données

Le convertisseur est partagé par toute la fenêtre de 2,56 MHz, et le parasite est un peigne
sur toute la tranche 132-133 — donc dedans. **Un parasite qui sature la conversion affame
tous les canaux, avions compris.** Si ce mécanisme joue, baisser le gain aide les avions
*parce que* ça empêche le parasite d'écrêter.

- 32,8 gagne **et le gain est plus grand à 00 h–05 h qu'aux extrémités** → affamement du
  convertisseur ;
- gain **uniforme sur la nuit** → simple effet de gain.

Écrit avant les données pour ne pas pouvoir raconter l'histoire après coup.

#### Lancée le 21/09 à 22 h — et notre prédiction n'était pas testable

Bloc 01/18 à **22 h 00 min 00,54**, gabarit sauvegardé avant modification, restauration du
matin par **copie de la sauvegarde** et non réécriture de la ligne de gain. Deux échecs
consécutifs d'`aero-mode` arrêtent la campagne et restaurent — on récupère une nuit
partielle plutôt qu'une station dans un état intermédiaire. Les 18 bascules sont alignées
**sur l'horloge murale et non sur des sommeils cumulés** : enchaînées naïvement, les 27 s
de chaque bascule auraient décalé le dernier bloc de huit minutes.

**Quatrième rattrapage de la station en trois échanges, et celui-ci porte sur la
prédiction elle-même.** Ils ont ajouté un **témoin continu sur 132,275**, non demandé :

> *« Vos fichiers par transmission ne s'ouvrent que quand le squelch s'ouvre — ils ne
> peuvent pas, par construction, montrer le bruit entre les transmissions. Or votre
> prédiction en a besoin. »*

C'est exact. Distinguer « 32,8 gagne davantage aux heures chargées en parasite » d'un
simple effet de gain suppose une mesure de l'intensité du parasite **indépendante des
déclenchements**. Le matériau que nous avions demandé ne pouvait pas la fournir. **Une
prédiction non testable avec les données qu'on a soi-même demandées n'est pas une
prédiction.**

#### Validation de l'outil contre les chiffres publiés, avant les données

Fait dans la nuit plutôt que d'attendre 7 h. Les cinq prises ATIS sont encore servies par
la station ; on remesure et on compare à leur rapport.

| Prise | RMS publié / mesuré | Crête | Somme histogramme, publiée → mesurée | rapport |
|---|---|---|---|---|
| A | −15,1 / −15,1 | −0,1 / −0,0 | 8 019 → 9 670 | **×1,21** |
| B | −15,5 / −15,5 | −0,7 / −0,7 | 4 498 → 5 421 | **×1,21** |
| C | −16,4 / −16,3 | −1,0 / −1,0 | 1 284 → 1 578 | **×1,23** |
| E | −15,1 / −15,1 | −0,2 / −0,2 | 8 273 → 9 964 | **×1,20** |

**Deux règles opératoires, vérifiées et non supposées :**

1. **Un seul décodeur pour tout.** RMS et crête se reproduisent au dixième de dB, mais les
   comptes près de la butée portent un **décalage systématique de ×1,21** entre notre
   chaîne et la leur. La comparaison entre bras est intacte — les deux bras passent par le
   même décodeur. La comparaison d'un chiffre absolu avec un autre outil ne l'est pas.
2. **Jamais de ré-encodage avant mesure.** Première tentative : les deux fichiers de la
   prise A concaténés en repassant par LAME donnaient **×0,64 et 0,5 dB d'erreur de RMS**.
   Mesurés séparément sans ré-encodage, ils redonnent ×1,21. L'anomalie s'allume et
   s'éteint avec le ré-encodage — **c'était ma faute, et c'est la démonstration.**

#### Un défaut trouvé par un test, pas par une relecture

`campagne-gain.py` + 19 tests. La garde de fin de bloc écartait une transmission
**terminée avant la bascule** : le trou audio suit la bascule, ce qui s'est achevé avant
n'a rien subi. Ce qui compte en fin de bloc n'est pas un délai fixe mais **le chevauchement**
— exact dès qu'on connaît la durée de la transmission, et une garde de 10 s aurait de
toute façon raté les transmissions longues (43,8 s existe dans le jeu de test). Corrigé,
et le test qui l'a trouvé était écrit pour ça.

### D40 — Le gain quotidien n'écrête pas les avions *(22/09 — ⚠️ **titre faux, corrigé par D42**)*

**Réponse à Q34, et elle est nette.** Nuit du 21 au 22, 18 blocs, 29 823 déclenchements,
**999 transmissions porteuses de parole** — l'estimation de la station (~1 040) était juste
à 4 % près.

#### 1. Le confondant du squelch est mort, sur leur propre test

Ils avaient proposé de compter les déclenchements par bloc **avant** de regarder la moindre
transcription : si le seuil était absolu malgré son nom, baisser le gain de 7,4 dB les
aurait effondrés.

| Bras | Déclenchements |
|---|---|
| 40,2 dB | 13 529 |
| 32,8 dB | 14 266 |
| **rapport** | **1,054** |

Pas d'effondrement. **Le seuil est bien relatif, ils avaient raison** — et nous le savons
sur nos données, pas sur leur parole.

#### 2. La réponse : non, et ce n'est pas un simple résultat nul

Écrêtage lu **transmission par transmission, sur les seuls fichiers porteurs de parole**,
comme leur garde-fou l'imposait :

| Source | Gain | Butée (p. 1000) | RMS |
|---|---|---|---|
| ATIS Saint-Cyr (émetteur au sol) | 40,2 | **4,564** | −15,1 |
| ATIS Saint-Cyr | 32,8 | **2,567** | −15,5 |
| **Avions, médiane** | **40,2** | **0,000** | **−22,8** |
| **Avions, médiane** | **32,8** | **0,000** | **−23,2** |
| Avions, moyenne | 40,2 | 2,352 | −23,0 |
| Avions, moyenne | 32,8 | 2,738 | −23,3 |

- **La médiane est nulle aux deux gains.** 51 % des transmissions à 40,2 dB et 55 % à
  32,8 dB n'ont **aucun** échantillon à moins de 3 dB de la butée.
- L'écart des moyennes va dans le **mauvais sens** (−0,39 p. 1000, Welch t = −0,67) : rien.
- Les avions arrivent à **−23 dB**, soit 23 dB de marge sous la butée.

**Ce qui rend la conclusion solide, c'est qu'elle ne repose pas sur un test négatif.** Un
résultat nul issu d'une mesure sous-dimensionnée ne vaut rien ; ici une mesure *positive*
dit qu'il n'y a rien à écrêter. La moyenne de 2,35 p. 1000 est portée par une queue (15 %
des transmissions au-dessus de 1 p. 1000), et cette queue a **la même taille dans les deux
bras** — 37 contre 38 transmissions au-dessus de 10 p. 1000.

#### 3. Le non-transport, confirmé quantitativement

C'est le point qui comptait. **Sur l'ATIS, baisser le gain marche** : 4,56 → 2,57 p. 1000,
la moitié. **Sur les avions, ça ne change rien**, parce qu'il n'y avait rien à corriger.
Un émetteur au sol à quelques kilomètres arrive **8 dB plus haut** qu'un avion.

La prudence écrite en Q32 — *« la mesure ne se transporte pas d'un émetteur fort à un
émetteur faible »* — était justifiée, et il aura fallu une nuit pour le savoir plutôt que
de le supposer dans un sens ou dans l'autre.

#### 4. Le bénéfice gratuit est un résultat négatif

Fichiers vides heure par heure : **96 à 98 % toute la nuit**, contre les 92-98 % de
référence du 15-16. **Le retrait des adaptateurs CPL n'a rien changé au parasite.** Seule
exception, 22 h à 44,6 % de vides — mais 233 déclenchements seulement, sur le point bas de
la courbe, et on n'en conclut rien.

#### 5. Deux incidents

**Deux scripts de campagne ont tourné en parallèle.** Armés à 21 h 39 et 21 h 45, le
premier jamais arrêté, tous deux écrivant `gain = ...` dans le même gabarit et appelant
`aero-mode` au même horaire. **Sans effet sur les données** : les deux calculaient le même
gain pour le même bloc, et Docker a sérialisé — **un seul démarrage de conteneur par bloc,
vérifié identique sur les 18**. Mais c'est exactement la classe de faute qui nous a valu
17 co-atc en une nuit (D26) : un processus lancé, remplacé par un meilleur, et jamais tué.

**Le témoin continu a perdu la moitié de ses données.** `append = false` sur une sortie
rouverte toutes les 30 minutes : chaque redémarrage tronque, donc chaque fichier horaire ne
contient **qu'un bloc** (27 à 30 min d'échantillons réels) au lieu de deux. Lequel des deux
a survécu ne se établit pas depuis les données — tenté par corrélation entre l'énergie du
témoin et la densité de déclenchements, non concluant, le parasite est trop uniforme.
**L'idée était juste, le drapeau était faux** ; `append = true` suffit. Sans conséquence
ici, la prédiction qu'il devait tester étant sans objet dès lors que 32,8 ne gagne pas.

> **Au passage** : `ffprobe` annonce des durées fausses de −17 à +12 minutes sur ces
> fichiers VBR. La station nous avait renvoyé l'avertissement ; il valait bien au-delà de
> ce qu'on en disait. **On compte les échantillons décodés, jamais l'en-tête.**

> **Et 501 fichiers illisibles** sur 29 823 (1,7 %), écartés et comptés comme tels.

### D41 — La nuit n'était pas nécessaire, et c'est la vraie leçon *(22/09)*

Le propriétaire a posé la seule question qui comptait : *« donc la nuit ne nous a rien
appris ? »* Vérifié plutôt que défendu.

**473 transcriptions porteuses de parole des 14-17/09 sur 132,275, enregistrées au gain
quotidien, déjà présentes sur la station depuis une semaine :**

| | 14-17/09, déjà sur disque | Campagne de nuit, bras 40,2 |
|---|---|---|
| Butée, médiane | **0,000** | **0,000** |
| Sans aucun échantillon près de la butée | **53 %** | **51 %** |
| RMS médian | −20,0 dB | −22,8 dB |

**La question principale se répondait en dix minutes, sur des fichiers déjà là, sans
mobiliser personne.** Il suffisait de regarder avant de demander.

**Ce que j'ai fait à l'envers.** J'ai écrit à la station, à propos du corpus français :
*« nous n'avons pas encore mesuré ce que nous avons ; demander des données avant
d'exploiter les existantes serait la faute que ce dossier se reproche depuis le début. »*
Puis j'ai demandé une nuit entière pour une question dont la réponse dormait sur leur
disque. **La règle était juste, je ne me la suis pas appliquée.**

**Ce que la nuit a quand même apporté**, et qu'il faut compter honnêtement :

- **à la station**, leur propre question : le parasite est inchangé après le retrait des
  adaptateurs CPL, ce qui désigne le bloc d'alimentation de la Freebox par élimination ;
- **deux défauts trouvés dans leur montage** — les deux scripts concurrents, et
  `append = false` qui détruit la moitié du témoin ;
- le bras à 32,8 dB, c'est-à-dire la preuve que baisser le gain ne gagne rien. Mais **dès
  lors que rien n'écrête à 40,2, ce bras ne pouvait rien apprendre** : il répondait à une
  question déjà close ;
- la résolution de mon confondant de squelch — qui **n'existait que parce que j'avais
  conçu la campagne**. Un problème qu'on se crée et qu'on résout n'est pas un acquis.

**La règle, maintenant explicite** : *avant de demander une donnée à qui que ce soit,
mesurer sur ce qui est déjà sur disque.* Ça vaut pour la station, et ça vaut pour les
120 annotations qui attendent depuis une semaine.

### D42 — Si, ça écrête — et j'avais le chiffre sous les yeux *(22/09)*

**La station a mesuré la même nuit et conclu l'inverse.** Vérifié plutôt que défendu : ils
ont raison.

#### Où était la faute

Nous ne mesurions pas la même chose. Ils sélectionnaient par taille de fichier et lisaient
*« la crête atteint-elle la butée ? »* ; je sélectionnais par détection de voix et lisais
*« quelle fraction des échantillons est près de la butée ? »*. Les deux séries se
recoupent une fois recalculées sur le même ensemble — **~10 % des transmissions de parole
touchent la butée, aux deux gains** (9,8 % à 40,2 et 10,3 % à 32,8 ; eux trouvaient 16,3 et
14,6 sur leur sélection, plus large).

**La faute est d'avoir lu la médiane d'une distribution à queue.** J'ai écrit « médiane
nulle, donc rien n'écrête ». Pour un phénomène de queue, la médiane est exactement la
statistique qui l'efface. **Et j'avais le chiffre** : D40 dit « 15 % des transmissions
au-dessus de 1 p. 1000, 37 au-dessus de 10 p. 1000, la queue a la même taille dans les deux
bras ». J'ai lu cette queue comme du bruit autour du zéro. C'**était** le phénomène.

#### Ce n'est pas un effleurement

Parmi les transmissions de parole qui atteignent la butée :

| | 40,2 dB | 32,8 dB |
|---|---|---|
| Échantillons collés à la butée, médiane | **263** | **261** |
| … en proportion de la transmission | 1,56 % | 1,87 % |
| Transmissions avec ≥ 100 échantillons en butée | **71 %** | **80 %** |
| Transmissions avec 1 ou 2 seulement | 8 % | 8 % |

263 échantillons à 8 kHz, c'est **33 ms de forme d'onde écrasée**. Pas une crête qui
effleure : de la distorsion franche.

#### Et la cause n'est pas le gain, elle est en aval

La mesure qui referme le dossier — l'écrêtage en fonction du **niveau délivré** :

| RMS délivré | n | % vraiment écrêtées |
|---|---|---|
| sous −20 dB | 771 | **0,0 %** |
| −20 à −15 dB | 189 | 19,6 % |
| au-dessus de −15 dB | 39 | **100 %** |

RMS médian des écrêtées **−14,9 dB**, des propres **−23,6 dB**. C'est un seuil, pas une
tendance.

**Lecture probable, à vérifier** : la parole a un facteur de crête d'environ 15 dB. Une
transmission normalisée à −15 dB de RMS a donc ses crêtes **à la butée par construction**.
Ce ne serait pas une saturation radio mais **un niveau de sortie trop chaud pour la
dynamique de la parole**.

Ce qui explique enfin pourquoi le gain ne sert à rien, et la station l'a mesuré : **7,4 dB
de gain de tuner en moins donnent 0,2 dB de niveau délivré en moins** — la normalisation
aval absorbe le gain. Le levier n'est pas là où nous l'avons cherché toute la nuit.

#### Ce qui tient de D40 et ce qui tombe

- **Tient** : baisser le gain ne change rien. Mesuré deux fois, indépendamment, et le
  mécanisme est maintenant expliqué.
- **Tient** : la mesure de l'ATIS ne se transportait pas — mais pas pour la raison écrite.
  Ce n'est pas que les avions sont trop faibles pour écrêter ; c'est que **le gain n'est
  pas la variable**, ni pour l'un ni pour l'autre.
- **Tombe** : « le gain quotidien n'écrête pas les avions ». Une transmission sur dix est
  écrêtée, et franchement.
- **Tient** : la nuit n'était pas nécessaire pour le savoir (D41). Les 473 transmissions
  des 14-17/09 déjà sur disque portaient la même queue ; je ne l'aurais pas plus vue.

#### Deux choses de leur côté, dites en clair par eux

**La restauration automatique du gabarit a échoué** — fichier laissé vide à 07 h 00 min 00,
sauvegarde intacte, disque à 165 Go. Restauré à la main à 07 h 21. Ils écrivent *« je n'ai
pas d'explication et je n'en fabrique pas »*, et en tirent la seule leçon disponible : une
restauration qui ne se vérifie pas n'est pas une restauration, les prochaines finiront par
un `cmp`.

**Et le levier contre le parasite n'est pas le gain, c'est le seuil de squelch.**
132,783 tournait déjà à `squelch_snr_threshold = 25` au lieu de 10 : **~115 déclenchements
sur la nuit contre ~3 800 par canal pour les six autres, −97 %.** Ils signalent eux-mêmes le
confondant qu'ils ne peuvent pas lever — ce canal était peut-être simplement calme — et
refusent de vendre la conclusion qui les arrange. **C'est la piste actionnable, et elle
demande une nuit avec un second canal à 25 pour être tranchée.**

### D43 — L'écrêtage coûte de la transcription, et la baisse de niveau n'est pas le remède *(22/09)*

Suite de D42. La station a cherché où se règle le niveau des fichiers : **nulle part.**
`ampfactor` est écrit dans le bloc de sortie `mixer` de tous leurs gabarits, donc il agit
sur le flux Icecast et **pas un octet des fichiers que nous consommons**. Le niveau est
fixé par la démodulation AM interne. Ce qui explique enfin leurs 0,2 dB, et pourquoi deux
campagnes de gain ne pouvaient pas répondre à la question qu'elles posaient.

#### Est-ce que ça coûte vraiment ? Essai apparié, dose calibrée

Comparer les écrêtées aux propres ne répond pas : même dans la bande −20/−15 dB, les
écrêtées restent plus fortes (−16,0 contre −18,9) **et plus courtes** (2,4 s contre 4,0 s).
On mesurerait le niveau et la durée.

Donc **chaque transmission est son propre témoin** : on part d'une transmission propre, on
l'écrête à dose voulue, on **redescend au niveau d'origine**. Niveau et durée identiques,
seule la distorsion change.

| Dose visée | Éch. en butée | Transcriptions identiques | Similarité médiane |
|---|---|---|---|
| 0,03 % | 9 | 60 % | 1,000 |
| 0,5 % | 168 | 43 % | 0,937 |
| **1,56 %** *(la dose mesurée)* | **522** | **38 %** | **0,879** |
| 5,0 % | 1 670 | 29 % | 0,830 |

Test des signes apparié de 0,5 % à 5 % : **61 se dégradent, 20 s'améliorent, p < 0,0001.**
Le taux de sorties vides ne bouge pas (35 à toutes les doses) et le nombre de mots non plus :
**l'écrêtage ne tue pas la transcription, il la corrompt.**

Et le mode de défaillance est exactement notre goulot de Q30 :

> propre : *« **Lufthansa** Six Seven Four Three four zero one one »*
> écrêtée : *« **hello** Six Seven Four Three Four Zero One One »*

**Un indicatif détruit.** Q30 mesurait que 84 % des candidats ne trouvent aucun avion à
cause de chiffres et de lettres massacrés dans la transcription ; en voici une cause
identifiée, et elle frappe **les transmissions fortes, c'est-à-dire les avions les plus
proches**.

> **Mon premier essai était nul, et faux.** Je visais une cible de −15 dB de RMS, ce qui ne
> produisait que **9 échantillons** en butée là où les vraies écrêtées en ont **263**.
> Trente fois trop faible, résultat nul, et j'allais conclure « ça ne coûte rien ». **Une
> dose non calibrée donne une mesure qui a l'air valide et ne répond à rien** — c'est la
> même faute que la médiane de D42, sous un autre habit. La réponse graduée est ce qui
> protège : un point isolé n'aurait rien prouvé dans un sens ni dans l'autre.

#### La baisse de niveau proposée ne peut pas être le remède

La station prédit qu'une cible abaissée d'environ 6 dB ferait tomber l'écrêtage à zéro.
**Deux corrections, tirées de la distribution des niveaux mesurée sur 999 transmissions :**

| Baisse | Écrêtées restantes | Transmissions sous −35 dB |
|---|---|---|
| 0 dB | 76/76 | 7/999 (1 %) |
| 6 dB | **22/76** | 86/999 (9 %) |
| 10 dB | 0/76 | **333/999 (33 %)** |

1. **6 dB ne suffit pas** : 29 % des écrêtées le resteraient. Il en faut **10**.
2. **Et 10 dB n'est pas payable.** La prise ATIS à −35,5 dB de RMS n'a **rien** rendu
   (Q32) : à −10 dB, **un tiers du corpus passe sous ce niveau**. On échangerait un
   problème qui touche 8 % des transmissions contre un qui en touche 33 %.

**Le remède n'a pas la bonne forme.** Il ne faut pas baisser tout le monde, il faut
**écrêter moins les forts sans toucher aux faibles** — un limiteur ou une compression, pas
un facteur multiplicatif. Ça relève de notre aval à nous, pas de leur gabarit.

**Leur test `ampfactor` au niveau du canal reste à faire** : il répond à une question de
capacité — le niveau des fichiers est-il atteignable du tout ? — qu'aucune de nos données
ne peut trancher. Une ligne, en journée, sans rien couper.

### D44 — Premiers résultats de la vérité terrain : les modèles inventent *(23/09)*

**42 clips annotés par le propriétaire**, dont 32 à l'aveugle. Ordre entrelacé : l'échantillon
est représentatif des quatre fréquences à chaque arrêt. Premier vrai chiffre du projet sur
une référence humaine plutôt que sur un appariement ADS-B.

#### Le chiffre brut, et pourquoi il ne suffit pas

| | WER, tous | WER, aveugle |
|---|---|---|
| atco2-en | 127,5 % | 124,0 % |
| bofenghuang-fr | 148,6 % | 146,0 % |

Au-dessus de 100 % : les modèles produisent plus de mots faux que la référence n'a de mots.
**Le taux seul ne dit pas pourquoi.** Décomposé :

| Lot | Modèle | Insertions | Substitutions | Suppressions |
|---|---|---|---|---|
| tous (42) | atco2-en | **59 %** | 65 % | 6 % |
| tous (42) | bofenghuang-fr | **93 %** | 55 % | 5 % |
| sans lacune (18) | atco2-en | 46 % | 66 % | 8 % |
| anglais, aveugle (9) | atco2-en | 48 % | **21 %** | 0 % |
| français, aveugle (20) | atco2-en | 52 % | **71 %** | 10 % |
| français, aveugle (20) | bofenghuang-fr | **90 %** | 52 % | 7 % |

**Le défaut dominant n'est pas de mal entendre, c'est d'inventer.** Les suppressions plafonnent
à 10 % : les modèles ne ratent presque rien, ils ajoutent. Et ça frappe d'abord les clips
courts — **WER médian de 300 % sur les clips de quatre mots ou moins**, 100 % au-delà :

> référence (2 mots) : *« la station... ? »*
> atco2-en : *« Lufthansa X-ray Kilo Technic One College Lufthansa O »*

**Le contrôle qui écarte l'artefact** : sur les 18 clips où l'annotateur a tout compris
(aucune lacune), les insertions restent à 46 % et 68 %. Ce n'est donc pas seulement la
référence qui serait trop courte ; les modèles inventent aussi quand tout a été entendu.

#### Ce que ça dit, langue par langue

- **Anglais** : atco2-en entend bien — **21 % de substitutions** seulement. Son WER de 69 %
  vient presque entièrement de ce qu'il ajoute.
- **Français** : atco2-en substitue 71 % des mots, il entend mal le français. ~~Mais
  **bofenghuang-fr fait pire sur le français lui-même** : 149 % contre 133 %, porté par 90 %
  d'insertions.~~ **Faux — corrigé par D45** : c'est un artefact du WER. Sur ce qui compte,
  la part de ce qui a été dit qu'on retrouve, bofenghuang fait **deux fois mieux** (41 %
  contre 20 %) ; il invente aussi davantage, et le WER laisse les inventions l'emporter.

> ⚠️ **Ce n'est pas une contradiction de D24/D31, et il ne faut pas le lire comme tel.**
> D31 mesurait le *gain de l'union* par appariement ADS-B : un modèle au WER plus mauvais
> peut très bien apporter des indicatifs justes que l'autre rate. Mais c'est une réserve
> sérieuse sur l'idée que bofenghuang serait « le bon modèle pour le français ». Il est
> surtout **le modèle qui invente le plus**.

#### Et une conséquence pour Q30

Q30 attribuait les 84 % de candidats sans avion à des chiffres **massacrés**. D43 en a
identifié une cause, l'écrêtage. D44 en suggère une autre, plus massive : une partie de ces
« indicatifs » sont peut-être **inventés de toutes pièces** sur des transmissions courtes.
*« Lufthansa X-ray Kilo »* sur une transmission qui disait *« la station »* produira un
candidat qui ne trouvera jamais d'avion — et aucune règle d'appariement n'y peut rien.

#### Un signal que j'avais annoncé et qui ne tient pas

À **13 clips**, les clips assistés donnaient un WER nettement plus bas que les clips à
l'aveugle (141 % contre 200 %), et j'avais écrit que l'ancrage de D39 était « déjà
visible ». **À 32 clips à l'aveugle, l'écart s'est inversé** (143 % contre 124 %). C'était du
bruit sur 5 clips et 26 mots. Je l'avais assorti de réserves, mais je l'avais présenté comme
un signal : il ne l'était pas.

#### Une lacune de l'outil, trouvée par l'usage

**8 clips sur les 50 ouverts étaient restés vides** : le propriétaire entendait parler sans
saisir un mot, et rien dans l'outil ne permettait de le dire — « no speech » aurait été
faux. Ces clips étaient ignorés du calcul, alors que ce sont **les plus précieux** : sur une
transmission qu'aucun humain ne déchiffre, tout ce qu'un modèle produit est inventé. Ajouté :
un état « parole, mais rien de compréhensible », noté à part — mots inventés par clip, et
nombre de clips que chaque modèle a eu la sagesse de laisser vides.

### D45 — Aucun garde-fou du modèle ne sépare l'invention de la transcription *(23/09)*

Suite de D44. Si les modèles inventent, les réglages de whisper prévus pour ça devraient
aider : probabilité moyenne des tokens, probabilité de silence, relance à température
non nulle — et l'accord entre les deux modèles. **Mesuré sur les 42 clips figés, aucun
n'aide.**

#### D'abord, le modèle inventait bien

Deux des cinq clips que le propriétaire a jugés incompréhensibles datent du 12/09, jour dont
la station a gardé l'ADS-B (`globe_history`, décodé par `heatmap.py`) :

- le modèle entend *« Lufthansa 6935 »* — le seul Lufthansa en l'air à ±10 min est **DLH29Y** ;
- il entend *« Lufthansa… 6011 »* — **aucun Lufthansa en l'air**.

L'oreille humaine avait raison, et la question « le modèle entend-il mieux que l'humain ? »
avait mérité d'être posée avant de bâtir un indicateur qui suppose la réponse. **Et le
premier des deux sortait avec une probabilité moyenne de −0,58 : rien ne le signalait.**

#### Un contrôle qui a sonné : 20 textes sur 110 non reproductibles

La relance du modèle devait redonner les textes de `candidats.json`. **20 sur 110
différaient.** Les 20, sans exception, viennent de passes où le modèle a **relancé avec du
hasard** (température > 0) après avoir échoué à ses propres critères ; les 89 passes
directes sont identiques au mot près.

Deux conséquences : **19 % des passes relancent au hasard**, et sur ces clips-là la sortie
de production n'est pas reproductible d'un passage à l'autre. Le contrôle était dans le
script précisément pour ça — sans lui, on comparait des textes qui n'étaient pas les mêmes.

#### La mesure : précision et rappel plutôt que WER

Le WER récompense l'effacement : sur des sorties à 300 %, tout effacer donne 100 % et
« améliore ». On mesure donc séparément **la part des mots affichés qui sont justes**
(précision) et **la part des mots dits qu'on affiche encore** (rappel).

| Clips | Modèle | Filtre | Précision | Rappel | Effacés |
|---|---|---|---|---|---|
| anglais (11) | atco2-en | aucun | 47 % | **79 %** | 0 |
| anglais | atco2-en | confiance ≥ −0,6 | 52 % | 66 % | 3 |
| français (28) | atco2-en | aucun | 15 % | 20 % | 0 |
| français | bofenghuang-fr | aucun | 26 % | **41 %** | 0 |
| français | bofenghuang-fr | confiance ≥ −0,8 | 30 % | 31 % | 13 |
| tous (39) | atco2-en | confiance ≥ −0,4 | 56 % | **7 %** | 36/42 |

Accord entre les deux modèles, probabilité de silence, absence de relance : aucun ne fait
mieux que quelques points de précision contre davantage de rappel perdu.

**Conclusion : le modèle invente avec aplomb.** Ses propres indices de confiance ne
distinguent pas ce qu'il entend de ce qu'il fabrique. On n'achète de la justesse qu'en
effaçant presque tout — 36 clips sur 42 pour passer de 21 à 56 %.

#### Et une correction de D44

J'avais écrit, et dit au propriétaire, que **le modèle français fait pire sur le français**
(WER 149 % contre 133 %). **C'est un artefact du WER.** Sur les clips français, bofenghuang
retrouve **41 %** de ce qui a été dit contre **20 %** pour le modèle anglais — deux fois plus
— et affiche des mots justes plus souvent (26 % contre 15 %). Il invente aussi davantage,
et le WER laisse les inventions l'emporter sur les mots retrouvés.

C'est **cohérent avec D31**, pas en contradiction : le modèle français apporte ce que
l'anglais rate. Le WER seul m'avait fait dire le contraire.

#### Ce qui reste valable, et le jeu de contrôle intact

Aucun filtre n'ayant été retenu, **les 13 clips annotés depuis le figement n'ont servi à
rien** — ils restent vierges pour la prochaine expérience. C'est la bonne situation : rien
n'a été ajusté dessus.

**Piste suivante, non testée** : plutôt que filtrer après coup, **orienter le décodeur
avant** en lui donnant en amorce les indicatifs réellement en l'air (l'ADS-B les connaît).
C'est viser la cause — le modèle invente des indicatifs faute d'en connaître de vrais — et
c'est mesurable sur les clips du 12/09 dont on a l'historique.

### D46 — Amorcer le décodeur avec l'ADS-B : non concluant, et pourquoi ça ne peut pas l'être ici *(23/09)*

Suite de D45. Si le modèle invente des indicatifs faute d'en connaître de vrais, lui donner
en amorce (`initial_prompt`) les avions que l'ADS-B voit à ce moment devrait l'orienter.
Deux bras identiques à l'amorce près, **température 0 dans les deux** pour que les relances
au hasard de D45 ne noient pas l'effet. `whisper-lab/amorce-adsb.py` et `amorce-analyse.py`.

#### Le risque qu'il fallait mesurer d'abord

Aujourd'hui un indicatif inventé ne trouve aucun avion : il ne trompe personne. Avec une
amorce, le modèle pourrait produire **un avion réel — mais pas celui qui parlait**, et
l'appariement le trouverait. **Mesuré sur les 5 clips que le propriétaire n'a pas
compris : aucun avion de la liste cité, par aucun des deux modèles.** Le faux *« Lufthansa
Six Niner three five »* persiste à l'identique, amorce ou pas : le modèle ne va même pas
piocher dans la liste. Le risque ne s'est pas matérialisé — sur 5 clips.

#### Le résultat

| | Réglage (42 clips) | Contrôle (13 clips) |
|---|---|---|
| Indicatifs de l'amorce effectivement entendus | **2** | 0 |
| atco2-en, anglais — mots justes sans / avec | 47 % → 45 % | 35 % → 48 % |
| bofenghuang-fr, français — mots justes | **26 % → 16 %** | 51 % → 55 % |
| atco2-en, français — mots justes | 16 % → 17 % | 42 % → 49 % |
| Indicatifs justes gagnés grâce à l'amorce | 0 | 0 |

**Le réglage et le contrôle se contredisent**, et le contrôle ne compte que **4 clips par
case**. C'est la configuration exacte où D44 m'a fait annoncer un effet d'ancrage qui s'est
inversé à 32 clips. On ne conclut pas du contrôle. Ce que le réglage, plus fourni, dit
nettement : **l'amorce en phraséologie anglaise fait chuter le modèle français** de 26 à
16 % de mots justes, et **n'apporte aucun indicatif**.

#### Pourquoi le test ne peut pas trancher sur ces données

**Seuls 2 des avions entendus par l'annotateur figurent dans les amorces**, alors que 20
clips sur 42 contiennent un indicatif probable. Trois raisons, lisibles dans les exemples :

1. **La phraséologie abrège.** Après le premier contact, F-HVAC devient *« fox alpha
   charlie »*, et une compagnie est citée sans son numéro (*« … france soleil … »*). Une
   liste d'indicatifs complets ne rejoint pas ce qui se dit.
2. **Une bonne partie de l'aviation générale locale est probablement absente de l'ADS-B** :
   sur Chavenay, des indicatifs entendus (*« charly oscar »*, *« kilo papa »*) ne
   correspondent à aucun avion de la liste. L'emport ADS-B n'est pas obligatoire pour les
   avions légers en France. **Non vérifié** — il faudrait l'immatriculation complète.
3. Sur les rares cas où l'avion est dans la liste, le modèle le trouve **déjà sans amorce**.

**L'idée vise donc la mauvaise population** : elle ne peut aider que les avions qui émettent
en ADS-B, et c'est précisément le trafic français d'aéroclub — le plus mal reconnu — qui en
est le moins équipé.

#### Deux défauts de mesure trouvés en route, et corrigés

- Le détecteur d'indicatifs cherchait les **trois** dernières lettres d'une immatriculation ;
  la phraséologie française dit *« Fox »* + les **deux** dernières. Et *« charly »* n'était
  pas reconnu comme *« charlie »*. Trouvé en lisant les textes, pas les chiffres.
- L'option `--controle` était lue **après** que le chargement des autres scripts avait
  remplacé `sys.argv` : le rapport de contrôle ne s'affichait jamais, sans erreur.

#### Un vrai défaut de la production, trouvé au passage

La table de prononciation vient d'OpenFlights, figée en 2014. **Transavia France**, qui
domine les départs d'Orly (99 apparitions dans les amorces), y est **« French Sun »** au lieu
de **« France Soleil »** ; **easyJet Europe** (34) n'y est pas du tout (son indicatif radio
est *« Alpine »*). Quand le modèle entend correctement *« France Soleil »*, l'appariement de
production ne sait pas que c'est TVF. **Non corrigé ici** : `spoken-operators.csv` est par
construction une table de formes *observées*, y écrire des indicatifs de référence en
trahirait le rôle. Il faut une couche de correction distincte — décision à prendre.

### D47 — Une couche de correction des indicatifs radio, et ce qu'elle change vraiment *(23/09)*

Demandé par le propriétaire après D46. `assets/telephony-overrides.csv`, lu **avant**
`airlines.dat` par `NewMatcher`, et qui l'emporte sur lui.

#### Pourquoi ce n'est pas un bonus de score

Quand un opérateur est nommé **et** présent dans le ciel, l'appariement ne considère que
ses avions. Sans correction, un *« France Soleil 1234 »* bien entendu mais non reconnu
laisse en lice tout avion qui porte 1234. **Mesuré par les tests, sans le fichier :**
*« France Soleil 1234 »* part sur **AFR1234**, *« Bee Line 367 »* sur **AFR367**,
*« Fedex 512 »* sur **BAW512**. La mauvaise compagnie, à chaque fois.

#### Ce qui est corrigé

`airlines.dat` date de 2014. Sur 3 jours d'ADS-B (12, 14, 15/09), parmi les 40 opérateurs
les plus présents : **9 compagnies, 785 avions** — un sur dix — que l'appariement ne
savait pas nommer. Absentes (ITA, Wizz Air UK, Norwegian Suède), marquées inactives et
donc ignorées (FedEx, DHL, VistaJet, ASL), renommées (Transavia France y est *« French
Sun »*), ou classées sous un code retiré.

**Une seule collision, et elle justifie la priorité donnée au fichier** : *« Bee-Line »*
était attribué à **DAT**, l'ancien code de Brussels Airlines. La compagnie vole sous
**BEL**. La table d'origine avait tort, la correction doit gagner.

#### Deux surestimations, corrigées avant de conclure

**J'annonçais ~1 250 avions ; c'est 785.** easyJet Europe et NetJets étaient absents
d'`airlines.dat` **mais déjà appris par la station** dans `spoken-operators.csv`. Trouvé
parce qu'un test censé prouver la correction **passait sans elle**. Remplacé par un cas
qui échoue sans le fichier ; les quatre cas échouent désormais sans lui et passent avec.
**Un test qui passe sans le correctif ne teste pas le correctif.**

**Et ma correction phare ne sert presque à rien.** Dans 30 817 transcriptions existantes :

| Indicatif corrigé | Occurrences | … suivies d'un chiffre ou d'une lettre |
|---|---|---|
| Eurotrans (DHL) | 553 | **387** |
| Vista (VistaJet) | 185 | **142** |
| Quality (ASL) | 20 | 17 |
| Fedex | 5 | — |
| Red Nose (Norwegian) | 4 | — |
| **France Soleil** (Transavia) | **1** | — |

Transavia France fait 231 avions en 3 jours, et les modèles n'écrivent **jamais** son nom.
**Le gain réel est DHL** — près de 400 indicatifs qui nomment désormais leur opérateur —
que j'avais rangé parmi les détails « marqués inactifs ». Sans ce comptage, j'aurais
présenté la correction par son cas le moins utile.

*« Vista »* et *« Quality »* sont aussi des mots courants : un risque de restriction à tort.
Mesuré : suivis d'un chiffre ou d'une lettre phonétique dans 77 % et 85 % des cas, donc
employés comme indicatifs. Et la restriction ne joue que si l'opérateur est dans le ciel,
les chiffres devant encore concorder. Gardés.

#### Ce qui n'est pas fait

- **Aucune mesure sur l'appariement de bout en bout.** Les tests prouvent le mécanisme ;
  combien d'appariements du tunnel de Q30 changent, et dans quel sens, reste à mesurer —
  rejouer `cmd/phraseology -adsb` sur la nuit du 20 avec et sans le fichier.
- `internal/reference` (amont) lit aussi `airlines.dat`, pour **afficher** des noms dans
  l'interface ; un avion EJU y reste sans compagnie. Cosmétique, non touché.
- **Contribuable à l'amont** tel quel : le mécanisme est générique et les entrées sont des
  indicatifs OACI actuels, pas des réglages de cette station.

### D48 — Le français qui compte est celui des avions de ligne, pas des aérodromes *(23/09)*

**Précisé par le propriétaire** : la question du français est née de ce que **les
équipages français des avions de ligne font leurs phases d'approche et de départ en
français**. Les aérodromes (aéroclub, tour militaire) ne l'intéressent pas vraiment.

#### Conséquence : le jeu d'évaluation mesurait le mauvais français

Les 60 clips « français » du tirage du 15/09 viennent de **Chavenay (129,525) et de
Villacoublay (128,950)** — du trafic d'aérodrome. Aucun ne porte ce qui est visé. **Les
chiffres français de D44 et D45 décrivent l'aéroclub**, pas Air France en approche.

Le tirage avait cherché *« les fréquences qui parlent français »* sans demander *quel*
français comptait. Question à poser avant de tirer, pas après quarante annotations.

#### Le français visé est un problème bien plus facile

Tout ce qui rendait l'aéroclub désespéré ne s'y applique pas :

- **phraséologie normalisée** — caps, niveaux, fréquences, autorisations —, proche de ce
  que le modèle anglais traite déjà ;
- **indicatifs complets de compagnie**, pas les abréviations *« fox alpha charlie »* ;
- **tous les avions en ADS-B** : la réserve de D46 sur l'aviation générale non équipée ne
  concerne pas le trafic visé.

**Une mesure sur le bon trafic existe déjà** : l'union des deux modèles (D31), mesurée par
appariement ADS-B sur la nuit du 20 au 21 — groupe `gros-porteurs`, approches de De Gaulle.
Le second avis s'y ouvre sur environ une transmission sur huit, et apporte +11,7 %
d'indicatifs appariés. C'est le bon français, mesuré par une autre voie que la vérité
terrain.

#### Ce qui change dans le travail

- **On cesse d'annoter Chavenay et Villacoublay.** Les clips déjà faits restent valables
  comme description du comportement des modèles, mais ne guident plus les choix.
- **Orly Départs (127,750) est au cœur de la cible** : fréquence bilingue, et des clips
  déjà annotés en français y portent exactement ce trafic (*« … france soleil … bonne
  journée au revoir »*). À continuer.
- **L'en-route (132,500)** reste utile pour l'anglais.
- **Un vrai jeu d'évaluation français** devra être tiré sur les fréquences d'approche de De
  Gaulle, celles du groupe `gros-porteurs` que le propriétaire écoute tous les jours. Pas
  lancé.

### Q35 — Cinq écarts révélés par l'inventaire de ce que co-atc interprète *(ouverte, 23/09)*

`26-ce-que-co-atc-interprete.md` recense, vérifié dans le code, tout ce que co-atc tire de la
voix et de l'ADS-B et ce qu'il en affiche. Il fait apparaître cinq écarts, aucun traité :

1. **La vérification des autorisations n'est pas branchée.** Le statut *respectée / écart*
   existe dans les données, `UpdateClearanceStatus` n'est appelée nulle part. Et l'ADS-B
   transmet en direct l'altitude, le cap et le calage affichés par l'équipage : les
   confronter aux instructions entendues mesurerait la reconnaissance **sans annotation**,
   sur le trafic visé (D48). Seulement en direct : l'historique ne garde pas ces champs.
2. ~~**Phases d'approche et de départ, piste en service : calées sur Saint-Cyr (LFPZ)**~~ —
   **traité par D49** : l'aéroport de référence est réglable, et les phases sont jugées
   depuis lui. Reste : un seul aéroport à la fois.
3. **Langue et second avis français stockés, jamais affichés.**
4. ~~**Météo sur LFPZ, qui ne publie ni METAR ni TAF**~~ — **traité par D49**, la météo suit
   l'aéroport de référence. Reste le chat IA activé avec une clé vide, et l'API météo par
   défaut, qui est celle, privée, de Windy — **mesuré en Q36, gardée pour l'instant**.
5. **Aucune alerte sur code d'urgence** (7500, 7600, 7700), alors que le code transpondeur est
   reçu.

Le premier est le seul qui touche à la reconnaissance vocale — et le seul qui pourrait
sortir le projet de sa dépendance aux annotations manuelles.

### D49 — Les phases se jugent depuis un aéroport de référence réglable *(23/09)*

Décidé par le propriétaire : *« il faudrait mieux mettre Orly en aéroport de référence, c'est
le vrai aéroport le plus proche »* — le plus proche **qui reçoit des avions de ligne** —, puis,
entre trois options présentées : **corriger le code**, et *« prévoir ça comme une vraie
fonction configurable dans mes paramètres de l'app »*.

#### Le défaut de l'amont

**Dans le code de l'amont, « la station » désigne deux choses** : l'emplacement du
récepteur, et l'aéroport. Elles se confondent quand le récepteur est posé sur sa piste —
l'exemple de configuration est Toronto Pearson —, et divergent sinon. Toutes les règles de
phase mesuraient depuis le récepteur : une approche devait se diriger **vers la station**, une
montée initiale être **à moins de 5 NM de la station** et **s'en éloigner**, un avion au sol
n'était gardé que **près de la station**.

**L'amont voulait bien l'aéroport** : la règle d'approche est commentée *« verify heading
toward airport »* au-dessus d'un code qui lit la station, et un champ de journal s'appelle
`distance_from_airport` en calculant depuis la station. L'intention était juste, le code non.

Simplement mettre `airport_code = "LFPO"` aurait donné **les pistes d'Orly avec une géométrie
centrée sur le récepteur** : aucun départ d'Orly reconnu (Orly est à 14 NM, le rayon est de 5),
une approche seulement si elle se fait vers Fontenay.

#### La correction

- **`adsb.PhaseReference`** : un aéroport, sa position, ses pistes. Tenu derrière un pointeur
  **atomique** — il peut changer depuis le panneau pendant que la réception classe les avions,
  et une phase calculée à moitié avec un aéroport et à moitié avec un autre serait pire que
  l'une ou l'autre. Chaque calcul le lit une fois.
- **Ce qui passe à l'aéroport** : phases (APP, CLB, DEP, ARR, T/O, T/D), piste en service,
  filtre des avions au sol, correction des mesures près de la piste, atterrissage déduit
  quand le signal se perd. Grandeurs dérivées renommées pour dire ce qu'elles mesurent
  (`DistToAirportNM`, `IsApproachingAirport`).
- **Ce qui reste au récepteur** : distance affichée, anneaux, positions prévues, réglage
  manuel de la station.
- **Sans aéroport connu, repli sur le récepteur** : exactement le comportement de l'amont.
  **Pour une station posée sur son aéroport, rien ne change** — condition pour que la
  correction reparte vers l'amont telle quelle.

#### Le réglage

*Server → Reference airport*, dans les paramètres. Une liste des aéroports à moins de 50 NM
**dont les pistes sont utilisables** — 41 sur 319 dans le rayon de Paris, les autres n'ayant
pas les deux extrémités localisées —, triés par distance, avec la distance au récepteur.
Même mécanisme que la rétention et le niveau de journal : validé **en entier avant qu'une
seule chose s'applique**, gardé dans `configs/runtime-settings.json`, jamais écrit dans
`config.toml`. Un aéroport enregistré devenu inutilisable cède au démarrage la place à
`station.airport_code`, **dit dans le journal**, sans écraser le choix enregistré.

Le changement bascule ensemble **trois services** : pistes (référentiel), phases (ADS-B) et
météo. Les indices de piste en service sont oubliés — ils portaient sur d'autres pistes. Le
cache météo est vidé, et un résultat arrivé pour l'ancien aéroport est jeté plutôt que rangé
sous le nouveau : le cache garde la dernière bonne valeur quand une récupération échoue, sans
quoi les NOTAM de Saint-Cyr auraient été affichés comme ceux d'Orly.

#### Vérifié

- **Témoin** : même trajectoire, mêmes pistes, seule la référence change. Mesurées depuis le
  récepteur, une approche et une montée initiale tombent en **UNK** ; depuis l'aéroport,
  **APP** et **CLB**. 6 tests, dont un sous détecteur de concurrence.
- Le paquet ADS-B **n'avait aucun test**. Toute la détection de phases de l'amont tournait
  sans filet.
- **Chaque garde-fou échoue quand on le retire** : sans vidage, le cache garde l'ancien
  aéroport ; sans l'abandon des résultats périmés, un NOTAM de LFPZ est rangé sous LFPO.
  Leçon de D47 appliquée : un test qui passe sans le correctif ne prouve rien.
- **Instance d'essai isolée**, trafic réel : Orly accepté, code bidon refusé, pistes
  redessinées sur la carte, **METAR et TAF d'Orly** reçus, choix survivant au redémarrage,
  repli au démarrage vérifié. **4 approches d'Orly détectées en 9 minutes** (TAP432, RAM642J
  suivi de 3 300 à 2 600 ft) — il n'y en avait jamais.

#### Limites mesurées

- **T/O et T/D restent invisibles à Orly** : ils se jugent sur le passage sol/air, et aucun
  avion n'est reçu sous 500 ft à moins de 5 NM d'Orly (3 jours d'historique). APP et CLB
  fonctionnent : 375 avions reçus entre 500 et 1 000 ft à moins de 5 NM.
- **Un seul aéroport à la fois.**
- **Le Bourget comme référence confondrait** ses approches avec celles de De Gaulle, pistes
  presque parallèles à 4 NM.

#### Trouvé en route, hors du sujet

- ~~**Le choix « local » du premier lancement n'est pas enregistré**~~ — **corrigé par D51** : le code n'écrivait qu'une
  ligne de journal, alors que son commentaire parle d'enregistrer le choix. L'écran revient à
  chaque visite. Invisible pour le propriétaire, qui a un compte ; bloquant pour quiconque
  installe l'outil en local.
- **L'API météo par défaut de l'amont est celle, privée, de Windy**, qui répond elle-même
  *« Do not steal this API »*.
- **La mise en page du panneau** : la première version de la ligne débordait (345 px dans un
  panneau de 276) et coupait **toutes** les listes de la section. Trouvé à l'écran, mesuré,
  corrigé.

### D50 — Les corpus du laboratoire sont rangés sous `audio/`, datés du prélèvement *(23/09)*

Demandé par le propriétaire, *« si ça ne casse rien »*. `whisper-lab/` n'est pas sous git,
et ses dix dossiers audio portaient des noms d'usage (`corpus/`, `orly/`, `suites/`…) qui ne
disaient ni la date ni ce qu'ils contenaient.

**La règle** : tout l'audio sous `whisper-lab/audio/`, un dossier par corpus, nommé
`AAAA-MM-JJ-groupe-contenu`. **La date est celle du prélèvement sur la station**, pas celle
des enregistrements : le jeu de test, tiré le 15/09, contient des clips du 12, du 14 et du
15/09. Le groupe est celui de la station quand il y en a un. Les résultats, scripts et
journaux restent à la racine.

**Les documents datés gardent les anciens noms** — ils décrivent ce qui a été fait à
l'époque. La table de correspondance est dans `whisper-lab/CORPUS.md`, qui est désormais
l'index complet du laboratoire. Seuls les passages qui donnent un chemin **à utiliser
aujourd'hui** ont été corrigés : la commande de l'outil d'annotation (doc 09) et la
disponibilité du corpus par canal (doc 24).

**Vérifié** : sauvegarde des scripts avant ; outil d'annotation arrêté pendant l'opération,
fichier d'annotations inchangé (même empreinte, 55 annotations) ; dossiers renommés sur le
même disque, rien copié ni supprimé ; chemins réécrits dans 21 scripts, qui se compilent
tous ; les 18 chemins qu'ils citent existent ; l'outil présente les mêmes 60 clips ;
`note-modeles.py` rend **les mêmes scores** ; 21 + 19 tests au vert.

> **Une erreur corrigée en route** : la première passe datait cinq dossiers du 12/09, date
> du plus ancien clip. Le laboratoire a été créé le 15/09 à 10:58, et tout ce qu'il contient
> a été prélevé à partir de ce jour-là. Renommés au 15/09 avant publication de l'index.

**Complété le même jour, à la demande du propriétaire : la racine aussi.** Les 89 fichiers
en vrac sont rangés par nature, sous leur nom : `scripts/` (39), `resultats/` (19 et
`q1-francais/`), `journaux/` (24), `echanges-station/` (6). **Restent à la racine**
`.venv` et le lien `mlx-fr-bofenghuang`, parce que **co-atc lui-même les appelle** depuis
`configs/config.toml` — les déplacer aurait arrêté la transcription —, avec les autres liens
de modèles, `audio/`, `extraits/` et l'index.

**Vérifié par comparaison** : quatre scripts qui ne font que lire, lancés avant et après,
donnent **la même sortie** ; 36 lignes réécrites dans 22 scripts, tous compilent ; sur les
60 chemins que citent les scripts, **aucun n'a disparu du fait du rangement**. Huit
manquaient déjà : les données supprimées le matin même, qu'il faudrait redécoder ou
reprendre sur la station pour relancer `amorce-adsb.py` ou `depouille-nuit-gain.py`, et
deux modèles français absents du laboratoire. Dans le dépôt, seuls les pointeurs à usage
courant suivent : un commentaire du sidecar, les docs 09 et 26.


**Complété le 25/09** (demande du propriétaire, *« organise tout bien dans le projet lab »*) :
un sous-dossier par chantier sous `scripts/`, `resultats/` et `journaux/` — `station` (doc 28),
`entrainement` (doc 27), `exploitation` — et des liens sous `whisper-lab/entrainement/` vers les
données lourdes, qui restent sur le disque externe (le disque interne n'en reçoit rien : le
laboratoire y pèse 2,0 Go). 90 fichiers déplacés par `scripts/exploitation/ranger-2026-09-25.py`,
qui a réécrit toutes les références à leurs anciens chemins, jusque dans ce dossier ; tous les
scripts se compilent, la chaîne de dégradation se recharge. Index : `whisper-lab/CORPUS.md`.
### Q36 — Remplacer Windy par aviationweather.gov : ce qu'on perdrait *(en attente, décidé le 23/09 : Windy reste)*

Question du propriétaire : *« est-on sûr qu'on ne perd rien au niveau données ? »*. Mesuré
le 23/09 à 14:36Z sur Orly, les deux sources interrogées au même moment, puis l'historique
d'AWC sur quatre jours.

- **TAF : rien de perdu.** Texte identique. Windy déclare lui-même sa source,
  `ADDS-stored` : ADDS est le service d'aviationweather.gov. **Windy relaie déjà AWC.**
- **METAR : presque rien.** Mêmes textes, mais AWC a des trous que Windy, qui a sa propre
  source (`Internal`), n'avait pas ce jour-là : 12:00, 13:00 et 14:00Z manquaient chez AWC
  pour Orly, De Gaulle, Le Bourget et Toussus. Sur les quatre jours qu'AWC renvoie : **8
  METAR absents sur 204 à Orly (3,9 %), 3 sur 203 à De Gaulle**, jamais plus d'une heure
  d'affilée. Effet : le dernier METAR affiché a parfois une heure au lieu d'une demi-heure.
- **NOTAM : tout perdu.** Windy en donne **22 pour Orly**, texte intégral — dont la
  fermeture de la piste 06/24 du 10 août au 17 décembre. **AWC n'en publie pas** : aucun des
  20 services de son API n'en porte (`openapi.yaml`, lu le 23/09).
- **La phrase décodée** que Windy fabrique (*« Wind 220° 4kt… »*, `trend[0].txt[0]`) ne sert
  qu'au prompt du chat IA, inactif ici (Q35, n° 4). L'interface n'affiche que le texte brut.
  AWC fournit de toute façon les champs décodés (vent, visibilité, nuages, catégorie de vol).
- **Changer l'URL ne suffit pas** : l'interface et le prompt lisent la structure de Windy
  (`trend[].metar`, `taf.taf`, `notams[].raw`). Il faut traduire la réponse d'AWC dans le
  serveur.

AWC est gratuit, sans clé, limité à 100 requêtes par minute ; co-atc en fait 3 toutes les
10 minutes.

**Décidé par le propriétaire le 23/09 : on enregistre le résultat et on ne touche pas à
Windy pour l'instant.** Le jour où la question reviendra, elle porte sur les NOTAM seuls :
garder Windy pour eux, chercher une autre source (pas cherchée à ce stade), ou s'en passer.

### D51 — Local ou compte : le choix est enregistré, et se change dans les réglages *(23/09)*

Demandé par le propriétaire : *« corrige-le avec le test — et il faut aussi avoir la
possibilité de basculer de l'un à l'autre dans les settings »*.

#### Le défaut, pire que noté en D49

Choisir « cette machine uniquement » à la page de premier lancement **n'écrivait rien**.
La page se rechargeait, redemandait au serveur s'il fallait poser la question — oui,
puisqu'aucun compte n'existait — et **la même fenêtre réapparaissait aussitôt**. La seule
sortie était de créer un compte. Invisible pour le propriétaire, qui en a un ; bloquant
pour quiconque installe le projet. C'était notre code (D34), pas l'amont.

#### Ce qui est décidé

- **Le choix va dans `configs/users.json`**, à côté des comptes, champ `access` —
  jamais dans `config.toml`. Un fichier d'avant, sans ce champ, se comporte comme avant :
  des comptes, donc la connexion. **Vérifié sur le fichier du propriétaire**, relu par
  le nouveau code : mode compte, un compte, fichier inchangé.
- **Passer en local garde les comptes**, inutilisés. Repasser en compte les réactive
  avec leur mot de passe ; sans compte, le panneau en fait créer un.
- **Sans connexion seulement si personne d'autre ne peut atteindre le serveur** : écoute
  sur `127.0.0.1` **et aucun proxy déclaré**. Le proxy est ajouté par rapport à D34 : c'est
  précisément ce qui rend un serveur sur `127.0.0.1` joignable d'ailleurs, et le réglage
  permet maintenant d'**ôter** un mot de passe à un serveur qui tourne — la page de
  premier lancement ne faisait qu'en poser un. Ailleurs, la page n'offre plus le choix
  local, le panneau le refuse en disant pourquoi, et un « local » enregistré puis devenu
  impossible est **ignoré et dit dans le journal**, les comptes restant en vigueur.
- **Ôter la connexion demande d'être connecté** ; la remettre, non — qui atteint un
  serveur local est déjà sur la machine.
- **Une seule règle, écrite à un seul endroit** (`resolve`), recalculée au démarrage et
  à chaque changement. `Enabled`, lu à chaque requête, l'est désormais sous verrou : la
  réponse peut changer pendant que des requêtes arrivent.

*Settings → Server → Access* : l'état en clair, un bouton pour basculer, une
confirmation qui dit ce qui va se passer, puis la page se recharge — avec le formulaire
de connexion si elle est désormais demandée.

#### Vérifié

- **12 tests nouveaux** : 8 dans `auth`, 4 par les vraies routes de l'API — le défaut tel que le
  navigateur le rencontrait, les deux sens depuis le panneau, la création du premier
  compte, les refus. Les redémarrages sont simulés en relisant le fichier.
- **Chaque garde-fou échoue quand on le retire** — six mutations, six attrapées, dont
  **le code d'avant exactement** : la page qui n'enregistre pas le choix fait échouer le
  test du premier lancement. `Enabled` sans verrou est attrapé par le détecteur de
  concurrence.
- **Instance d'essai isolée** (port 8090, dossier et fichier de comptes à part) : choix
  local, rechargement — **l'application s'ouvre au lieu de la même question** ; bloc
  *Access* dans les deux modes et en mode impossible ; page de premier lancement avec un
  proxy déclaré, choix local grisé et expliqué ; avertissement du journal, dont le
  premier libellé désignait mal la cause et a été corrigé.
- **Non fait dans le navigateur** : saisir un mot de passe. Le compte d'essai a été créé
  et utilisé par l'API ; l'affichage du mode compte a été vérifié en posant l'état du
  panneau dans la page, sans rien envoyer au serveur.

> **Trouvé par le propriétaire au premier essai, le soir même** : repasser en « Require
> sign-in » n'a **pas** fait réapparaître le formulaire de connexion. Sa session d'avant la
> bascule était toujours valide : exiger à nouveau la connexion ne la demandait à
> personne. C'était le seul chemin que je n'avais pas parcouru dans le navigateur, faute
> d'y saisir un mot de passe — et les tests créaient chaque fois une session neuve.
> **Corrigé** : passer en local ferme toutes les sessions. Test ajouté, qui suit ce chemin
> exact et échoue sans la correction — **13 tests** au lieu de 12.
>
> **Au même essai** : l'icône qui ouvre les réglages sortait à moitié du bord gauche de la
> fenêtre, panneau replié. Défaut de l'amont (le bouton chevauche le bord du panneau, et
> replié ce bord est celui de la fenêtre). Il se décale de 1,5 rem une fois replié ;
> mesuré à l'écran : de −17…15 px à 7…39 px, inchangé panneau ouvert.
>
> **Confirmé par le propriétaire au second essai** : formulaire de connexion revenu,
> icône en place, message du sidecar disparu.

### D52 — Le sidecar répond pendant qu'il transcrit *(23/09)*

Signalé par le propriétaire en bas des réglages : *« Transcription ok · second opinion (not
reached just now: … context deadline exceeded) »*.

**La cause** : la route `/transcribe` du sidecar est `async`, et elle appelait le modèle
directement. Pendant tout le décodage, la boucle d'événements était tenue et `/health` ne
pouvait pas répondre. Le panneau attend 2 s, puis affiche le dernier état connu avec ce
message — prévu ainsi par D30, qui avait vu le symptôme sans en chercher la cause.

**La correction** : le décodage passe dans un fil à part, derrière un verrou qui garde **une
transcription à la fois**, comme avant — rien ne dit que les modèles supportent des appels
simultanés, et ils partageraient de toute façon le même GPU.

**Mesuré avant et après**, même clip de 7,3 s envoyé directement au sidecar, `/health`
interrogé toutes les 100 ms :

| | pendant la transcription | pire attente | au-delà de 2 s |
|---|---|---|---|
| avant | 2 requêtes | coupées à 10 s | 2 sur 2 |
| après | 17 requêtes | 874 ms | 0 |

Sur le trafic réel : avant, **2 sondes sur 113** au-delà de 2 s en une minute (une au-delà
de 5 s) ; après, **0 sur 178** en 90 s, pire attente **40 ms**, avec 5 transmissions
transcrites dont un second avis réussi, aucune erreur au journal. Le texte transcrit du
clip d'essai est identique avant et après.

### D53 — La fiche d'un avion entendu montre enfin ce qui a été entendu *(23/09)*

Signalé par le propriétaire, en deux temps : pour les avions du filtre *« Heard on the
radio »*, la section *Radio* de la fiche **clignote** — et, *« une fois de plus »*, elle
est **vide**, *« ce n'est pas logique »*.

**Une seule cause pour les deux.** La requête qui lit les transmissions d'un indicatif
demandait 12 colonnes et n'en lisait que 9 : `sql: expected 12 destination arguments in
Scan, not 9`, **à chaque appel**, 80 fois dans le journal du jour. Introduit le **20/09**
par le second avis (`e90433e`), qui avait ajouté trois colonnes à la requête, déclaré les
variables, et oublié de les lire — ce qui compile. La marque « entendu » passe par une
autre requête, correcte : d'où l'avion marqué entendu et la fiche vide.

**Le clignotement venait d'un second défaut, côté page.** La fiche est rafraîchie à chaque
mise à jour de position, environ une fois par seconde, et relançait la requête **chaque
fois que la réponse précédente était vide** — ou en erreur. Donc « loading… », puis rien,
puis « loading… ». Cela touchait aussi **tout avion non entendu**, une requête par seconde.

**Corrigé** : la requête lit ses 12 colonnes ; la fiche ne redemande que si l'avion change
ou si **quelque chose de nouveau** est entendu sur lui (le compteur de transmissions le
dit) ; et une erreur du serveur s'affiche comme une erreur, plus comme « rien d'attribué »
— c'est ce déguisement qui a caché le défaut trois jours.

**Vérifié** : test sur la requête, qui échoue avec l'erreur exacte du journal sans la
correction ; balayage de toutes les requêtes du stockage, colonnes contre lectures — aucun
autre écart. **Instance d'essai sur une copie de la base du jour** : AFR32UN, entendu,
affiche sa transmission et la valeur FL100, stable sur 12 relevés en 6 s, **une seule
requête** au lieu d'une par seconde ; VLG35WR, non entendu, « Nothing matched » stable, une
requête ; une transmission nouvelle simulée dans la page déclenche **exactement une**
relecture.

### D54 — Le choix d'écouter une fréquence est retenu *(23/09)*

Demandé par le propriétaire : *« régler une fois pour toutes ce problème de l'activation ou
non de l'audio : quelquefois j'ai juste des transcriptions sans audio, d'autres fois les
deux »*.

**Ce n'était pas une panne, c'était la conception de l'amont.** La page ne joue rien
d'elle-même : toute fréquence démarre **coupée**, et le son ne part qu'au clic sur sa
tuile. Ce choix n'était pas retenu — **chaque rechargement** (connexion, redémarrage du
serveur, bascule d'accès, F5) repartait coupé. Les transcriptions, elles, sont faites par
le serveur, qui écoute le flux lui-même : elles arrivent quoi que fasse la page. D'où
« les deux » si la tuile avait été cliquée depuis le dernier chargement, « le texte seul »
sinon. Le seul signe à l'écran était la couleur des barres, gris ou vert.

**Corrigé** :
- **Le choix est retenu par le navigateur**, comme les préférences d'affichage (D30).
- **Il est réappliqué au clic sur « Start Monitoring »** — les navigateurs n'autorisent le
  son qu'après un clic sur la page, et celui-là est déjà demandé à chaque chargement.
- **Un haut-parleur sur la tuile** dit l'état en clair : barré et grisé si coupé.
- Un navigateur qui n'a jamais choisi reste coupé, comme avant : rien ne se met à parler
  sans qu'on l'ait demandé une fois.

**Vérifié dans le navigateur**, instance d'essai sur le même flux : jamais choisi →
coupé ; clic sur la tuile → son, choix retenu ; **rechargement puis « Start Monitoring »
seul → le son revient**, position de lecture qui avance de 2,6 s en 3 s ; coupé puis
rechargé → reste coupé.

### D55 — Les mots qui ont nommé l'avion sont gardés au moment de l'association *(23/09)*

Demandé par le propriétaire : *« mettre en gras et colorer la partie du texte qui a servi à
faire l'association avec l'avion »*, dans la liste des transcriptions et dans la section
*Radio* de la fiche. Ma première proposition les retrouvait à l'affichage ; le propriétaire
l'a refusée — *« ça ne peut pas être gardé au moment de l'association plutôt que retrouvé
rétroactivement ? »* — **et il avait raison** : recalculer peut désigner d'autres mots que
ceux qui ont réellement décidé.

**Ce qui est gardé** : les mots qui ont **nommé** l'avion — les chiffres qui ont concordé
(toutes leurs occurrences quand l'indicatif est répété, puisque la répétition compte), les
lettres d'une queue (*victor juliett*), l'opérateur quand il est dit. **Pas** ce qui a
seulement corroboré — l'altitude, une autorisation : elles soutiennent une association
sans nommer personne.

**Où** : une colonne `callsign_evidence`, des positions en unités UTF-16 — celles dont le
navigateur se sert —, **dans le texte affiché** pour la lecture qui a décidé : le texte
normalisé pour le modèle principal (celui où « niveau un zéro zéro » devient « FL100 », les
positions tenant compte de ce décalage), le texte brut pour le second avis français. Quand
c'est le second avis qui a nommé l'avion, il est désormais **affiché sur une ligne « FR »**,
faute de quoi le surlignage n'aurait eu nulle part où aller.

**Au passage** :
- **Les sept fonctions qui lisent les transcriptions n'en font plus qu'une** — même liste
  de colonnes, même lecture. C'est leur duplication qui avait produit le défaut de D53.
- L'affichage échappe désormais le texte ; la mise en évidence de recherche de l'amont
  l'injectait tel quel dans la page.

**Limite** : les transcriptions associées avant ce changement n'ont pas de mots gardés et
restent sans surlignage — rien n'est reconstruit après coup, c'est le principe même.

**Vérifié** : 10 tests, dont les positions après une valeur réécrite, dans un texte brut
avec accents et ponctuation, une occurrence répétée, des lettres ; de bout en bout par le
processeur et la base, pour les deux lectures. **Six retraits, six attrapés.** Instance
d'essai sur une copie de la base du jour, 73 transmissions retraitées : la colonne est
ajoutée à une base existante, MEA230 (« sydar jet », pour *Cedar Jet*) a ses deux
« two three zero » en gras dans la liste, et la fiche les affiche aussi.

### Q37 — Le hachage entendu sur certaines réceptions *(ouverte, 23/09 — cause mesurée le soir : le squelch, pas le réseau)*

Signalé par le propriétaire : *« un problème de hachage rapide sur certaines réceptions ;
quelque chose qu'on a changé ? »*. **Rien n'a changé, mesuré** :
- **Station** : mode actif `gros-porteurs`, gabarit inchangé depuis le 14/09 (le fichier daté
  du 22/09 est le retour à ce mode après le test d'ampfactor, contenu identique). Gain 40,
  squelch à 10 dB, **aucune sortie fichier par transmission** — la cause de hachage déjà
  connue (01-station). Réception et filtre redémarrés proprement à 09:54, même
  configuration, aucune erreur depuis, processeur libre à 76 %.
- **Notre côté** : le code de la chaîne audio est **identique à l'amont** depuis le fork.
- **Trois minutes enregistrées en même temps**, en direct et à travers co-atc : flux de la
  station **sain au repère de `salves.py`** (médiane 4,6 s, 0 % sous la seconde) ; à travers
  co-atc, transmissions de **durée identique** et **14 contre 13** micro-coupures (des
  pauses entre mots). co-atc raccourcit seulement des silences entre transmissions.

**Piste, non prouvée** : la station est branchée sur une borne mesh **dont le lien de retour
est radio** (01-station). Pendant ces mesures, la station est devenue **injoignable
quelques secondes** depuis le Mac (« no route to host » à 18:24), et le ping varie de 5 à
57 ms sur 30 s. Une coupure du lien hache tout ce que le Mac reçoit. Autre possibilité, des
signaux faibles qui battent autour du seuil de squelch. **Pour trancher : l'heure à la
minute d'un cas entendu**, et où il a été entendu. Les boîtiers CPL prévus traiteraient la
première.

**Mesuré le 23/09 à 21:24, pendant un hachage signalé en direct par le propriétaire** —
deux captations du flux de la station (1 min 30 et 2 min 30), en direct, sans passer par
co-atc :
- **Ce n'est pas le réseau.** 0 % de perte sur 150 pings vers la station (4 à 55 ms), et tout
  le son est arrivé : 172 s de son reçues en 150 s d'écoute (l'avance est le tampon qu'Icecast
  envoie à la connexion). Aucun trou de transport.
- **C'est le squelch, à la station.** Les coupures sont du **silence numérique pur** (–180 dBFS)
  de 30 à 310 ms **au milieu d'une transmission** : la voix passe, s'arrête net, reprend. Un
  arrêt du réseau ne peut pas écrire de zéros dans le fichier ; seul le squelch de
  RTLSDR-Airband le fait, quand un signal faible repasse sous son seuil (`squelch_snr_threshold
  = 10`).
- **Une seule fréquence.** Chaque canal est placé à gauche ou à droite dans le mélange
  (`balance`), ce qui permet de dire d'où vient chaque coupure par l'écart de niveau
  gauche/droite. Première captation : **9 coupures, toutes sur 125,825 (De Gaulle
  Approche)**, en trois grappes — la signature d'un squelch qui bat. Seconde : 2 coupures, dont
  une sur 124,350. D'où *« pas sur toutes les transmissions »* : seulement les plus faibles.
  125,825 est aussi la fréquence dont le niveau est le plus relevé dans le mélange
  (`ampfactor = 1.4`) et la moins bien transcrite (Q19).
- **`salves.py` ne pouvait pas le voir.** Sa règle compte les silences de **0,35 s et plus** ;
  ces coupures durent de 30 à 310 ms. La mesure du 23/09 après-midi (« flux sain ») était
  juste pour ce qu'elle mesure et aveugle à ce défaut. Il faut lui adjoindre le compte des
  coupures courtes en silence numérique.

**Confirmé le 24/09 par les compteurs de RTLSDR-Airband**, pendant l'enregistrement continu
par canal demandé à la station (`whisper-lab/echanges-station/`, accord du propriétaire,
`stats_filepath` ajouté). En 6 minutes, de 10:47:28 à 10:53 : **125,825 compte 296 ouvertures
de squelch et 341 battements** (`channel_flappy_counter`), contre 14 à 38 ouvertures et 0 à 35
battements sur les trois autres. Bruits estimés voisins (−40,8 dBFS contre −40,6 à −42,8).
**Le battement vient par épisodes** : 332 des 341 dans les deux minutes et demie qui suivent le
démarrage, puis environ un par relevé de 15 s. Démarrage
ou vrai épisode de trafic faible : à trancher dans les fichiers par canal.

**Tranché à 11:03 sur le premier fichier fini** (10:47:31 → 11:00:00, quatre fichiers de 748,7 s,
lisibles, sans trou) : **ce n'est pas le démarrage.** Un second épisode a lieu de 10:55 à 10:57
(162 battements sur 125,825), et **126,425 bat aussi** (244 battements de 10:56 à 10:57, puis 44 à
11:02) ; 124,350 et 124,625 jamais. Les coupures courtes comptées dans l'audio tombent aux mêmes
minutes que les compteurs de la station (125,825 : 16 à 10:47, 13 à 10:49, 12 à 10:55, 12 à
10:56) : les deux mesures se confirment. **Deux fréquences qui battent à la même minute**
suggèrent une cause commune plutôt que deux émetteurs faibles — hypothèse, non vérifiée, à
confronter aux trois heures complètes et au niveau des autres canaux au même instant
(écrêtage, D42).

**Analyse des 3 h 13 complètes : `28-gros-porteurs-par-canal.md`.** Deux canaux sains (124,350,
124,625 : 0 et 64 battements, environ 80 % de parole dans le temps ouvert), deux faibles
(125,825, 126,425 : 1 950 et 8 919 battements, 31 et 38 % de parole, plus de la moitié des
ouvertures sans parole) — les deux que le mélange relève déjà (`ampfactor` 1,4 et 1,3). Le
battement vient par épisodes sur toute la période. Méthode corrigée en route : une coupure
courte **isolée** est le plus souvent un changement d'interlocuteur ; seules les coupures **en
grappes** suivent les compteurs de la station.

**Remède possible, côté station, non appliqué** (configuration de production : accord
explicite du propriétaire requis) : baisser le seuil du squelch **sur 125,825 seulement**,
et mesurer avant/après à la même heure — coupures courtes, et ouvertures sur le bruit
(`salves.py`). Outils rangés dans le laboratoire : `scripts/station/hachage-coupures-courtes.py` (la règle de
`salves.py` plus les coupures courtes) et `scripts/station/hachage-par-frequence.py` (quelle
fréquence est coupée).


**Précision de la station, 25/09** : un réglage de squelch par `/radio/reglage` (ou le crayon du
tableau de bord) **ne vaut que pour la sélection libre**. Pour changer le squelch de 125,825 ou
126,425 **dans le groupe `gros-porteurs`**, il faut modifier son gabarit, **sur décision du
propriétaire**. La station attend une **proposition chiffrée** : fréquence, valeur, et preuves
tirées de `/radio/mesures` (bruit, seuil, part du temps ouvert, ouvertures et battements, sur 10 min
glissantes) — elle l'appliquera alors au gabarit.
### Q38 — Les mots accentués sont coupés en deux par l'analyse *(répondue le 23/09 : corrigé, voir la suite)*

Trouvé en écrivant les tests de D55 : le découpage en mots ne garde que `a-z`, `0-9` et le
trait d'union. **« zéro » devient « z » + « ro »** — alors que la table des chiffres français
contient « zéro », comme celle des rôles contient « degrés » et « nœuds ». Ces entrées ne
peuvent jamais servir. Le modèle français écrit avec accents : un « zéro » en tête d'un
numéro de vol est perdu. **À mesurer avant de corriger** — changer le découpage change
l'association sur tout le français, et c'est l'effet sur les appariements qui dira si c'est
un gain.

> **Mesuré puis corrigé le même soir**, à la demande du propriétaire. Les accents sont
> **repliés pour la reconnaissance** (« zéro » se lit « zero », et rencontre les tables qui
> avaient déjà les deux graphies), **et gardés à l'affichage** : le texte normalisé montre
> « départ », là où il montrait « d part ».
>
> **Sur les appariements, rien ne bouge** — ni en français (le modèle français sur les
> 1 378 transmissions du 15/09 : 93 appariements dont 62,2 vrais avant comme après, fréquence
> par fréquence), ni en anglais (les trois modèles du même corpus, identiques ; 143 dont
> 102,4 vrais pour le modèle de production). Pas de fausse association introduite non plus.
>
> **Sur ce que la grammaire extrait, 94 transmissions sur 1 378 changent**, lues une à une :
> - « à », jusqu'ici un blanc, **sépare les nombres** : « passez le niveau 115 à 190 » donnait
>   `115190` et donne **FL115** ; même chose pour « niveau 100 à 36 », « 160 à 190 »…
> - « degrés » **introduit un cap** : « 300 degrés » était un numéro de vol, c'est un cap 300 ;
> - « zéro » est lu : « 2 6 zéro » → 260 ;
> - **les lettres OTAN à la française** sont reconnues — « Québec », « Hôtel », « écho » —,
>   13 → 21 groupes, ce qui compte pour les immatriculations d'aviation légère ;
> - **une régression, corrigée** : un ordinal collé au nombre précédent (« 300, 3ème » →
>   `3003eme`). Un ordinal termine désormais un nombre, ce qui règle aussi « 300, 3ème »
>   qui donnait `3003` avant ce changement, sans toucher « 27L ».
> - **Le locuteur change sur 62 transmissions**, dans les deux sens (+40 identifiés, −21,
>   1 inversé). Certains cas sont plausibles (« cap 300 autorisé » → ATC), d'autres
>   invérifiables : ce corpus est le modèle français passé sur *toutes* les transmissions,
>   anglaises comprises, et beaucoup de textes sont inventés. **Non mesuré contre une
>   vérité** — et le locuteur n'entre pas dans l'association, seulement dans l'étiquette.
>
> **Vérifié** : 8 tests, dont la piste « 27L » qui doit survivre à la règle des ordinaux ;
> trois retraits, trois attrapés.

### Q39 — L'étiquette ATC / PILOTE n'est pas fiable, et une règle plus prudente n'a pas fait mieux *(ouverte, 23/09)*

Question du propriétaire : *« je ne comprends pas sur quoi il se base pour l'attribution
ATC ou pilote, ça me semble très approximatif »*.

**La règle en service** (`speakerOf`, la nôtre, écrite le 15/09 en remplaçant l'IA de
l'amont) compte des mots de deux listes — impératifs pour le contrôleur, formes en *-ing* et
politesses pour le pilote — et, à égalité seulement, regarde si l'indicatif ouvre ou ferme
la transmission. Défaut visible à la lecture : un pilote **collationne l'instruction mot pour
mot** (« descend five thousand, QNH 1030 » est venu des deux côtés d'un même échange), donc
l'impératif anglais ne dit rien de qui parle — et c'est lui qui l'emporte.

**Ce qui en dépend** : l'étiquette @ATC/@PILOT affichée, et **l'enregistrement des
autorisations** (une autorisation n'est enregistrée que sur une transmission étiquetée ATC).

**Mesures, et une erreur de ma part corrigée** :
- 3 932 transcriptions uniques depuis le 15/09 : **48 % sans étiquette**, 34 % ATC, 18 % pilote.
- J'avais d'abord annoncé « une paire d'échange sur dix dans le bon ordre, 43 % ATC puis
  ATC ». **C'était faux** : les extraits des 20 et 21/09 contiennent **849 doublons**, et 138
  des « paires » étaient la même transmission lue deux fois à la même seconde. Nettoyé :
  sur les échanges réels (même avion, 1 à 20 s d'écart), environ **4 sur 10** sont dans le
  bon ordre — toujours médiocre, moins que ce que j'avais dit.
- **Aucune référence fiable n'a pu être construite** sans oreille humaine : les paires où la
  réponse répète une valeur de l'ordre ne sont que 7, et plusieurs sont fausses (un pilote
  qui appelle deux fois). Le propriétaire ne peut pas trancher à l'écoute qui parle.

**Essayé et écarté** : une règle prudente — plus d'impératifs anglais, la place de
l'indicatif d'abord, l'appel d'un organisme reconnu, seulement des mots propres à un camp,
et pas d'étiquette si les indices se contredisent. Sur 68 échanges réels, elle alterne
**moins** bien (10 paires justes contre 17, 24 fausses contre 23) ; sur les 7 paires de
référence, égalité (5/9 contre 6/11). Elle étiquette moins (55 % sans étiquette contre 42 %)
sans étiqueter mieux. **Pas mise en service** ; gardée hors dépôt.

**Hypothèse pour la suite, non mesurée** : le texte seul ne porte pas l'information. Une
partie des segments contient **les deux voix** — le découpage coupe sur 600 ms de silence, et
un collationnement suit souvent de moins (« air france three two bravo yankee hello … hello »).
Le locuteur se lirait mieux dans l'audio que dans le texte.

### D56 — À minuit, le fichier du jour naissait sans ses tables radio *(23/09)*

Le propriétaire, avant de laisser tourner la nuit : *« tu es sûr à 100 % que tu as tout
bien programmé pour la nuit ? on a eu plusieurs fois des bugs qui ont invalidé plusieurs
heures de captation »*. Non. La vérification a trouvé ceci.

**Le défaut.** La rotation de minuit (D22) ouvre le nouveau fichier par `openOne`, qui
appelait `initDatabase` : tables `aircraft`, `adsb_targets`, `phase_changes`, et rien
d'autre. Les tables `transcriptions`, `clearances` et `phraseology_values` étaient créées
par les constructeurs de leur stockage, **une fois par processus, au démarrage**. Le fichier
ouvert à minuit n'avait donc pas de table `transcriptions`, et chaque écriture y échouait
jusqu'au redémarrage suivant.

**Ça s'est déjà produit, et on ne l'a pas vu.** Nuit du 20 au 21/09 (`runs/local.log`) :
à 00:00:02 « Rotated to a new daily database », et dans les trois minutes qui suivent
**829 erreurs « no such table: transcriptions »** — 92 échecs d'enregistrement, 487 de
l'index vocal, 248 du traitement par lots, 2 de mise à jour. Entre 0 h et 10 h, **1 877
transmissions transcrites, aucune conservée** : la base du 21 commence à 10:51, au
redémarrage. D26 mettait les échecs d'écriture sur le compte des 17 serveurs et de
`SQLITE_BUSY` « vers 7 h » ; ils ont commencé à 00:00:02, pour cette raison-ci. D22 disait
« non observé : un vrai passage de minuit en production » — il avait été observé, et il
avait échoué.

**Pourquoi les tests ne l'ont pas vu.** `TestStoragesBuiltBeforeRotationFollowIt`
écrivait, à travers chaque stockage, **dans la table `aircraft`** — la seule que la rotation
créait. Il prouvait que la poignée suit, pas que les tables existent.

**Le correctif** (`d25c182`). Le schéma de toutes les tables est créé par `initDatabase`,
donc à chaque ouverture — démarrage et rotation ; les constructeurs ne créent plus rien.
Deux tests écrivent ce que chaque stockage écrit réellement : sur le fichier créé par la
rotation, et sur un fichier plus ancien auquel manquent des colonnes ajoutées depuis. Sans
le correctif, les deux échouent (vérifié par mutation). Le binaire reconstruit a été
relancé à 21:08, `vcs.modified=false`.

**Ce qui restait non observé** : un passage de minuit réel avec ce correctif. Surveillance
passive cette nuit — `whisper-lab/scripts/exploitation/surveillance-nuit.sh` écrit une ligne toutes
les dix minutes dans `journaux/exploitation/2026-09-23-nuit-surveillance.log` (processus, mémoire,
fichier du jour, transcriptions, erreurs, disque). À lire demain : le compte de
transcriptions du fichier du 24 doit croître dès 00:10, et `no_such_table` rester à 0.

**Observé le 24/09** : rotation à **00:00:49**, fichier du 23 fermé dans la foulée, 0 « no
such table », 0 écriture refusée. Le fichier du 24 a reçu **831 transcriptions de 00:00 à
08:56** (157 dans la première heure, 32 à 35 au creux de 1 h à 3 h) et 619 000 positions
ADS-B. co-atc et le sidecar ont tenu toute la nuit sur les mêmes processus, co-atc entre 260
et 350 Mo. Seule perte côté transcription : **une** transmission à 02:02:27, requête coupée
par le sidecar pendant l'envoi (*« write: connection reset »*) — cause probable, non
vérifiée : une connexion gardée ouverte que le sidecar venait de fermer, que Go ne rejoue
pas pour un POST. Aucune perte par dépassement de file au pic du matin. Côté station, l'ADS-B
a manqué deux minutes à 01:41 (station injoignable) et une trentaine de secondes à 08:26
(`aircraft.json` en 404, conteneurs non redémarrés) ; les deux fois, reprise sans
intervention.

**Leçon de méthode.** Une seconde voie qui fait la même chose que la première masque le
trou de la première : les constructeurs créaient les tables au démarrage, donc rien ne
manquait *au démarrage*, et c'est le seul moment qu'on regarde. Un test doit écrire ce que
le programme écrit, pas un substitut.

### D57 — co-atc suit la station fréquence par fréquence, par son interface *(25/09)*

Accord du propriétaire le 25/09 au soir : *« oui, commence le code co-atc »*, pour être prêt quand
la station ouvrira ses flux séparés (Q45) — le gain le plus fort mesuré à ce jour, ×4 d'avions
justes (doc 28).

**Le partage suit D36.** co-atc ne lit pas `/radio/etat` : personne d'autre ne l'a. Il gagne un
mécanisme générique, et la politique reste dehors, dans un petit programme à part.

**Dans co-atc (contribuable en amont tel quel)** :
- **des sources ajoutées et retirées en marche** : `PUT /api/v1/sources/{id}` et `DELETE`, derrière
  l'authentification comme les autres écritures (`internal/frequencies/runtime_sources.go`). Les
  sources de la configuration ne se remplacent ni ne se retirent ainsi. Renvoyer la même source ne
  change rien ; un nouveau nom ou un nouveau rang **ne reconnecte pas** (une reconnexion coûte du
  son) ; une nouvelle adresse, langue ou décision de transcrire, si. La page web redessine la liste
  d'elle-même (message `frequencies_changed`) ;
- **`ffmpeg_reconnect = false`** par source : ffmpeg ne relance plus lui-même un flux fini ou en
  404 — le piège signalé par la station ; co-atc le relance après `reconnect_interval_secs` ;
- **la datation par la position dans le flux** (`internal/transcription/stream_clock.go`) : le
  tampon d'Icecast livre 50 à 65 s de son passé en une seconde, et l'amont datait tout à
  l'arrivée. L'horloge compte le son reçu contre l'horloge murale ; les transmissions coupées
  pendant le tampon attendent qu'il soit passé pour être datées ; une reconnexion (un trou de plus
  de 2 s) repart de zéro, et **ce qu'elle rejoue n'est pas transcrit deux fois** ;
- **un bug de l'amont corrigé** : arrêter un flux pouvait faire planter co-atc (la boucle de
  surveillance lisait un minuteur que l'arrêt venait de mettre à nil). Rare au démarrage et à
  l'arrêt, il devenait certain avec des sources retirées à chaque changement de groupe — le test
  l'a déclenché du premier coup ;
- **un second défaut de l'amont, trouvé par l'essai** : entre ffmpeg et la transcription, un
  anneau de **64 Ko (1,3 s de son)** sans garde. Quand ffmpeg décode le tampon d'Icecast d'un coup,
  il réécrit ce que la transcription n'a pas lu, sans que personne le sache — et l'ordre même des
  données pouvait se mélanger. Remplacé par des positions qui ne bouclent pas, **4 Mo (87 s)**, et
  un lecteur dépassé qui saute au plus ancien encore tenu en le journalisant
  (`internal/audio/multireader.go`) ;
- **une transmission coupée par une reconnexion** restait ouverte et se voyait collée au début du
  tampon de la nouvelle connexion — **datée 40 s trop tôt** à l'essai, pire qu'un doublon puisqu'une
  mauvaise heure peut l'attacher au mauvais avion. Un trou de réception la clôt désormais telle
  quelle, datée par l'horloge de l'ancienne connexion.

**Hors de co-atc (propre à cette station)** : `cmd/radio-ctl-sync`, qui lit `/radio/etat` et
`/radio/frequences` toutes les 10 s et **ne bascule jamais la station**. Une fréquence de la
sélection qui a un `flux` devient une source ; `langue` `mixte` est lue en anglais (la porte
française de D24 fait le reste) ; `transcrire` vient du catalogue ; le rang suit la fréquence, pas
la place dans la sélection ; récepteur arrêté, aucune source. Station injoignable : les sources
restent telles quelles. Avec `-mix-id`, l'étiquette du flux mélangé suit aussi la sélection (D36).

**Essai de bout en bout, sans toucher la vraie station** : une station simulée
(`whisper-lab/scripts/station/station-simulee.py`) rejoue l'enregistrement par canal du 24/09
comme Icecast — **64 Ko de tampon, soit 47,5 s de son** à la connexion, puis le direct au rythme
des trames —, avec `/radio/etat` au format réel. co-atc de test (port 8011, base à part, modèle de
production), liaison, changements de groupe. Premier essai, 116 transmissions jugées contre
l'heure en direct simulée (`station-simulee-verifier.py`) :

| | Transmissions | Écart à l'heure en direct (médian / max) | Avec la datation à l'arrivée |
|---|---|---|---|
| Arrivées dans le tampon | 14 | **0,4 s / 1,8 s** | 17,9 s / 54,1 s |
| Arrivées en direct | 102 | 0,5 s / 1,2 s | idem |

La base garde l'heure à la seconde, d'où l'essentiel de ces écarts. Aucun plantage. Une adresse
en 404 est retentée **toutes les 5 s**, sans boucle.

**Quatre essais, trois défauts trouvés et corrigés en chemin** (`whisper-lab/resultats/station/
2026-09-25-station-simulee-essai-{1..4}.txt`) :
- essai 1 : au changement de groupe, les canaux restés à l'antenne étaient reconnectés, parce que
  leur rang avait changé — rang par fréquence, et un rang nouveau ne reconnecte plus. Essai 2 :
  125,825 et 126,425 gardent leur connexion à travers deux changements de groupe ;
- essai 2 : 4 transmissions seulement arrivaient du tampon, le reste perdu dans l'anneau de 64 Ko
  — anneau remplacé. Essai 3 : **9 transmissions du tampon rien qu'à la première connexion** ;
- essai 3 : une coupure réseau en pleine transmission l'a collée au tampon rejoué — **datée 40 s
  trop tôt**, et transcrite deux fois — trou de réception clos. Essai 4, même coupure : la
  transmission interrompue **une seule fois, à son heure** ; sa copie rejouée est écartée.

| Essai 4 (tout corrigé) | Transmissions | Écart à l'heure en direct (médian / max) | Datation à l'arrivée |
|---|---|---|---|
| Arrivées dans le tampon | 13 | **0,6 s / 1,05 s** | 27,4 s / 56,5 s |
| Arrivées en direct | 80 | 0,6 s / 1,06 s | idem |

Les « doublons » que le vérificateur signale encore (un par essai 2 et 4) sont deux transmissions
voisines découpées autrement que dans la référence, sur des canaux jamais reconnectés — pas des
rejeux. **Reste à faire** : le même essai sur la vraie station, quand ses flux séparés seront en
service (après le réglage des micro-coupures d'`aero.mp3`, Q45).

**Pas fait** : étiqueter chaque transcription de la `version` de la station (promis à la station le
25/09). Pour un flux par canal, la fréquence suffit ; la liaison journalise chaque version. À
reprendre si le flux mélangé reste en service.

### D58 — Les indicatifs à lettres sont lus, et les règles d'association se règlent depuis le panneau *(26/09)*

Décision du propriétaire, après la mesure de Q46 (+27 % d'avions justes, précision 73 -> 76 %) :
*« on active les nouvelles règles — mais peut-être le mieux c'est d'avoir une section règles dans
les réglages qui permette de les activer / varier »*.

**Ce qui est en service** (commit `9437ecf`) : une section **« Callsign matching »** dans le
panneau Server, à côté de la rétention et du niveau de journal, qui s'applique **à la transmission
suivante, sans redémarrage**, et se garde dans `runtime-settings.json` comme les autres :

| Réglage | Par défaut | Ce qui a été mesuré |
|---|---|---|
| Lettres dans le numéro de vol (« seven uniform echo » = 7UE) | **activé** | +27 % d'avions justes, précision 73 -> 76 % (Q46) |
| Exploitant entendu approximativement (« welling » = Vueling), parmi ceux du ciel | **activé** | +1 % en nombre ; rattrape les indicatifs à moitié entendus |
| Nombre de chiffres minimal | 3 (2 à 4) | à 2, le hasard égale la vérité (Q30) |
| Un chiffre d'écart toléré | désactivé | 81 % de bruit (doc 19) |

Chaque ligne du panneau porte son chiffre mesuré. Un réglage hors bornes est refusé avec le reste
du changement ; un fichier de réglages antérieur garde ses valeurs et reçoit ces défauts.

**Complété le 26/09, décision du propriétaire après les mesures de la piste 1 (Q46)** : trois
réglages de plus, pour un avion entendu sur la fréquence dans les deux dernières minutes —
**ses dernières lettres** (« Sierra Bravo », +8) et **ses deux derniers chiffres** (« cinq
quatre », +13) sont **activés** ; **sa compagnie seule** (+68, justesse invérifiable à l'ADS-B)
est proposée **désactivée**, en attendant une vérification à l'oreille.
Vérifié sur une instance de test : la case décochée depuis la page arrive au serveur, se journalise
(« Server settings changed from the panel ») et se relit au redémarrage. **Contribuable en amont** :
les règles et le mécanisme sont génériques.

### D59 — Le ciel de chaque fréquence : le secteur *(26/09 — construit, **validé le soir même et en service par défaut**)*

Suite de la piste 3 de Q46 (mesurée sur l'approche de Roissy : hasard divisé par deux, avions
justes en plus). **Décisions du propriétaire, 26/09** : les tailles par défaut, et une copie dans
notre dépôt du tableau des 68 fréquences fourni par l'agent de la station.

**La source des secteurs.** L'agent de la station a dressé le tableau des 68 fréquences depuis
l'eAIP (GEN 3.4 §3.4.5.4, ENR 2.1, AD 2.18 de LFPG, LFPO, LFPB, LFPV, LFPN ; AIRAC 03/09/2026) :
61 lignes eAIP, 4 catalogue ou écoute, 1 supposée, 1 conflit (`whisper-lab/echanges-station/
station-vers-atc-scribe-2026-09-26-secteurs-68-frequences.*`). Il identifie au passage onze
fréquences jusque-là inconnues (118.800 = Le Bourget Approche ; 125.933, 132.500, 132.783, 133.000,
133.250, 135.033, 135.500 = Reims ; 118.883, 132.733 = Brest ; 135.550 = Paris TN), à confirmer à
l'écoute. **Conflit sur 132.275** : catalogue « Paris Contrôle arrivée CDG », eAIP « Reims UAC
KF ». *Corrigé le 26/09 au soir* : nos transcriptions **tranchent** — elles ont été prises canal
par canal la nuit du 15 au 16/09, et `24-identification-amas.md` en a tiré trois preuves
indépendantes que ce qu'on reçoit sur 132,275 est **Paris Contrôle, arrivées de De Gaulle** :
étagement de descente (médiane FL200), 14 transferts vers l'approche de De Gaulle (121,155), et
un appel initial « hello Paris good evening ». L'eAIP pourrait réutiliser la fréquence pour un
secteur de Reims éloigné ; c'est à vérifier dans GEN 3.4 côté station. (Écrit le matin sans
relire le document 24 : l'affirmation « ne tranchent pas » était fausse.)

**Les tailles**, par nature de fréquence, réglables au panneau :

| Nature | Rayon autour de l'aéroport | Plafond | Fondement |
|---|---|---|---|
| approche, départ | 60 NM | 20 000 ft | mesuré le 24/09 (approche de Roissy) |
| tour | 15 NM | 6 000 ft | à mesurer |
| sol, prévol | 5 NM | 1 500 ft | à mesurer |
| secteurs de contrôle | — | — | **pas de filtre** : l'eAIP ne publie pas les limites des secteurs, et son rangement ACC/UAC ne dit rien de l'altitude (124,625 y est rangée en UAC, alors que les avions reconnus le 24/09 volaient à 15 700 ft en médiane ; les bornes FL195/FL660 d'abord portées au tableau étaient une déduction, retirée par l'agent de la station le 26/09) ; à apprendre des données |

**Ce qui est construit** (commit `3fc52d6`, générique et contribuable, sauf la table) : une
fréquence peut nommer son aéroport et sa nature (configuration, ou API des sources) ; avec le
réglage « Only aircraft in the frequency's sector », l'association ne garde que les avions dans le
rayon et sous le plafond, autour de la position de l'aéroport tirée des données de référence ; un
avion sans position ni altitude est gardé ; dans un secteur, la partie vol amputée (« 7U ») est
acceptée (0,6). `radio-ctl-sync` attribue aéroport et nature depuis la copie du tableau (40 des
68 fréquences). **Vérifié sur le flux réel de 123,875 (Orly Approche)** : le ciel passe de 74 à
29 avions, contre 32 comptés indépendamment sur l'ADS-B de la station.

**Séance de validation, 26/09 de 16:44 à 18:35** : co-atc de test sur quatre flux séparés choisis
par le propriétaire (123,875 Orly Approche, 124,350 et 125,825 De Gaulle, 124,625 Paris Contrôle
sans secteur), secteur **désactivé** pendant l'enregistrement, 681 transmissions ; mesurée ensuite
hors ligne (`cmd/phraseology -db`, règles en service dont l'attente de D60 et « France » de D61,
cinq ciels mélangés ; `whisper-lab/resultats/station/2026-09-26-validation-secteur/mesure/`) :

| | Rattachés | Hasard | Avions justes | Précision |
|---|---|---|---|---|
| sans secteur | 120 | 10,6 | 109,4 | 91 % |
| **avec secteur** | 115 | **4,4** | **110,6** | **96 %** |

Par fréquence : 123,875 (Orly) 4,2 → 5,6 justes, hasard 1,8 → 0,4 ; 124,350 66,2 → 67,8, hasard
6,8 → 2,2 ; 125,825 20,2 → 19,2, hasard 2,8 → 1,8 ; 124,625 inchangée (pas de secteur). **Le
résultat du 24/09 se confirme un autre jour et sur un autre aéroport** : hasard divisé par 2,4,
autant d'avions justes. **Activé par défaut** (`DefaultMatchingRules`), désactivable au panneau.

Au passage : Orly ne rattache que 6 transmissions sur 129 (124,350 : 73 sur 307) — la séance a
coïncidé avec le hachage de 123,875 (Q47).

**Reste** : apprendre des données les secteurs des fréquences de contrôle.

### D60 — Une transmission sans avion attend une minute que l'ADS-B le décode *(26/09)*

**Le cas, signalé par le propriétaire pendant la séance de validation** : sur 125,825 à 16:46:27,
« Lufthansa bien bonjour Air Algerie one two one four descending flight level one one zero… »,
transcrit correctement, n'est rattaché à aucun avion alors que DAH1214 est bien dans le ciel.
Relevé dans la base de la séance : **l'avion n'existait pas encore pour co-atc**. La station
reçoit son adresse (452134) à 16:46:44, sans indicatif ni position ; l'indicatif arrive à 16:47:06,
la position à 16:47:18 — à 69 NM à l'est-sud-est de la station, au FL120, transpondeur ADS-B
version 0. Le rattachement avait été tenté à la passe suivant la transcription (16:46:57), une fois,
et jamais repris. Quatre minutes plus tard, « only call sign Algerie one two one four » est
rattaché sans difficulté (chiffres exacts, 0,9).

**Ce n'est pas un accident** : le premier appel sur une fréquence se fait à l'entrée du secteur,
souvent là où l'avion entre aussi dans la portée du récepteur ADS-B.

**Mesuré** sur la capture du 24/09 (règles en service et secteurs, la mesure `tout-propre` de
Q46), en bornant le ciel à *n* secondes après le début de la transmission (option `-after` de
`cmd/phraseology`, résultats `whisper-lab/resultats/station/2026-09-26-apres-transmission/`) :

| Ciel lu jusqu'à | Avions justes | Hasard | Précision |
|---|---|---|---|
| 0 s | 326,4 | 66,6 | 83 % |
| 15 s — **la production aujourd'hui**, en ordre de grandeur (durée + décodage + passe de 10 s) | **343,4** | 67,6 | 84 % |
| 30 s | 361,0 | 68,0 | 84 % |
| 60 s — **ce que publient Q46, D58 et D59** | 367,6 | 68,4 | 84 % |
| 75 s — **15 s + une minute d'attente** | **372,4** | 68,6 | 84 % |

**Deux conclusions.** Les chiffres publiés jusqu'ici lisaient le ciel ±60 s autour de la
transmission : ils prêtaient à la production des avions décodés après coup, environ 7 % d'avions
justes de trop (343 au lieu de 368). Et une minute d'attente rattrape tout : **+29 avions justes
(+8 %), sans perte de précision, hasard +1**. L'ADS-B du 24/09 est échantillonné à la seconde
(écart médian entre deux points d'un même avion : 1 s), la mesure n'est pas faussée par son pas.

**Ce qui est fait** (`grammar_processor.go`) : une transmission restée sans avion est gardée en
mémoire et retentée à chaque passe (10 s) contre le ciel du moment, pendant 60 s ; au-delà elle est
abandonnée. Un rattachement tardif s'écrit et se diffuse comme le premier — la page remplace la
transmission par son identifiant, les valeurs (niveaux, caps) déjà enregistrées passent sous
l'indicatif, les clairances s'écrivent à ce moment-là. Le journal le signale par `late`. Rien en
base, rien au panneau : l'attente se perd au redémarrage, sans conséquence. Pas de réglage : la
mesure ne montre pas de coût. Contribuable tel quel.

**Pas encore vu en service** : la séance de validation tourne sur l'ancien binaire ; le
prochain lancement le portera.

### D61 — « France » seul désigne Air France *(26/09)*

**Le cas, signalé par le propriétaire** : sur 124,350 à 17:00:29, « bonsoir France Quatre Nine
Sierra Romeo pass the metro 100 pied air level 100 … Sierra Romeo », pas rattaché alors
qu'AFR89SR est dans le ciel depuis 17:00:27. Rejoué avec le ciel exact de ce moment : les chiffres
sont mal entendus (« quatre nine », 49 pour 89 ; la seconde lecture a bien « France 89 », mais
séparé des lettres), les lettres sont justes (« Sierra Romeo », +0,3), et **« France » seul ne
désignait aucune compagnie** — ni `airlines.dat` (AIRFRANS), ni nos deux tables. 0,3 reste sous le
plancher de 0,6 ; avec le nom, 0,7, sans ambiguïté, depuis chacune des deux lectures.

**Ce n'est pas un cas isolé** : sur la capture du 24/09, « France » sans « Air » apparaît 157 fois,
contre 314 « Air France ». Au moins 11 de ces transmissions (11 avions) étaient déjà rattachées à
un Air France par les chiffres ou les lettres seuls.

**Mesuré** (capture du 24/09, règles en service, secteurs, attente de D60 ; contrôle par ciel
mélangé ; `whisper-lab/resultats/station/2026-09-26-france-seul/`) :

| | Avions justes | Hasard | Précision |
|---|---|---|---|
| sans | 372,4 | 68,6 | 84 % |
| **« France » = AFR** | **386,8** | 70,2 | **85 %** |

**+14 avions justes (+4 %)**, sur les quatre fréquences. « Bair France », forme vue aussi, n'apporte
rien de plus. **Fait** : une ligne dans `assets/spoken-operators.csv`. « France Soleil » reste
Transavia, la forme à deux mots étant cherchée d'abord. Au passage, la recherche des noms ne relit
plus le second mot d'une forme à deux mots : « Air France » surlignait aussi « France » seul.

### D62 — Orly et De Gaulle suivis ensemble *(26/09)*

**Demande du propriétaire, 26/09** : dans l'interface, seuls les axes de piste de Saint-Cyr
apparaissaient (l'instance de test lisait l'aéroport de sa configuration, `LFPZ`, et non le
réglage de production, `LFPO`), mais surtout **un seul aéroport à la fois** : Q40 n'avait jamais
été codée. **Choix du propriétaire**, sur recommandation : Orly et De Gaulle ; la météo du seul
aéroport principal ; la phase affichée avec son aéroport.

**Ce qui est fait** (générique, contribuable ; un seul aéroport suivi = comportement inchangé) :

- **Réglage** : `reference_airport` reste l'aéroport principal (météo, champs d'API à aéroport
  unique) ; `also_airports`, au plus trois, sont suivis en plus — cases « Also follow » au
  panneau. Un réglage enregistré avant reste valide.
- **Attribution de chaque avion** (`internal/adsb/airport_attribution.go`) : l'aéroport **dont il
  suit un axe de piste prolongé**, dans un sens ou dans l'autre (approche ou montée initiale),
  avec les tolérances d'approche déjà configurées — 10 NM du seuil, 0,5 NM de l'axe, cap à 30°,
  **sous 5 000 ft** (au-dessus, survoler un axe ne dit rien) ; cette attribution est gardée pour
  le reste du vol. À défaut, l'aéroport suivi le plus proche, réévalué à chaque mise à jour.
  Toutes les règles de phase (APP, CLB, DEP, ARR, atterrissage à la perte du signal) mesurent
  ensuite contre cet aéroport.
- **Piste en service par aéroport** : les identifiants de piste ne portent pas leur aéroport
  (LFPO et LFPB ont tous deux 07-25) ; chaque aéroport a son propre compteur.
- **Phase et aéroport enregistrés ensemble** : colonne `airport` ajoutée à `phase_changes` (les
  bases existantes la reçoivent à l'ouverture) ; vide pour la croisière, et pour un décollage ou
  un atterrissage à plus de `airport_range_nm` (5 NM) de l'aéroport — un avion léger qui se pose
  à Toussus n'atterrit pas à Orly. Transmise dans l'API et les messages temps réel.
- **Page** : axes de piste de tous les aéroports suivis ; sous la phase, son aéroport (seulement
  quand plusieurs sont suivis) ; dans les alertes (« ICE547 → DEP LFPG ») ; la piste en service de
  chacun dans la barre du bas.

**Vérifié en direct** sur une seconde instance d'essai (port 8012, sans audio, ADS-B de la
station), à côté de la séance de validation : `/station` rend les deux aéroports (3 et 4 pistes),
la carte trace leurs axes, les premières alertes portent leur aéroport, la piste en service de
De Gaulle (26R) s'établit en quelques minutes.

**Mesuré sur 32 min d'essai** (17:36–18:08, 585 changements de phase) : APP 12 Orly / 7 De Gaulle,
ARR 29 / 13, DEP 21 / 15, CLB 3 / 0. **Les 19 approches sont toutes plus près de leur aéroport que
de l'autre**, la plus lointaine à 11 NM (IBE05SF, De Gaulle, 26 NM d'Orly). **Mais aucun des 41
atterrissages ni des 14 décollages n'avait d'aéroport** :
- 23 n'ont aucune position ;
- la plupart des autres sont des avions légers de Saint-Cyr et de Toussus, **à juste titre sans
  aéroport** (le plus proche aurait donné Orly, à 13 NM) ;
- mais les avions de ligne aussi : la station les perd à quelques centaines de pieds, et la dernière
  position d'un atterrissage date d'une minute, **5 à 7,5 NM** avant le seuil (AFR1655, AFR36KQ,
  AFR1855, IBE0579, DAH1108) — au-delà des 5 NM d'`airport_range_nm`.

Corrigé en deux temps :
- `33a62ac` : un avion vu sur un axe de piste de son aéroport garde celui-ci jusqu'à
  `approach_max_distance_nm` (10 NM) ;
- `68532b4` : l'atterrissage est détecté une minute après la dernière position, quand
  `aircraft.json` ne la donne plus. **Aucun** atterrissage ne pouvait donc être placé ; la dernière
  position de la trajectoire, de moins de deux minutes, en tient lieu.

**Vérifié en direct**, 31 min (18:41–19:12) : APP 12 De Gaulle / 10 Orly, ARR 24 / 23, DEP 18 / 3 ;
**11 atterrissages d'avions de ligne avec leur aéroport** (8 De Gaulle : EJU42YG, AIC143, LBT508,
AFR71FP, AFR1703, AFR647, LGL821P, CCA933 ; 3 Orly : 39de4f, TVF81YS, AFR94FM), chacun le plus
proche de sa dernière position (4,1 à 7,6 NM). Aucune attribution fausse. Restent sans aéroport :
- les avions légers de Saint-Cyr, Toussus et Le Bourget ;
- 13 atterrissages sans aucune position ;
- AFR71TY, dont la dernière position avait une heure.

Les 7 décollages sont tous d'avions légers ou sans position. La station ne voit pas les avions de
ligne au sol à Orly ni à De Gaulle.

### Q40 — Plusieurs aéroports de référence, pas un seul *(23/09 — **tranchée et construite le 26/09, voir D62**)*

Question du propriétaire : *« est-ce qu'on peut avoir deux aéroports rattachés ? Orly ET
CDG »*, puis *« à vrai dire la question serait pour n aéroports »*. D49 avait noté la
limite : **un seul aéroport à la fois**.

**Ce qui dépend aujourd'hui de l'aéroport unique** (lu dans le code) : les phases (APP,
CLB, DEP, ARR, T/O, T/D, mesurées depuis un point et une liste de pistes), la piste en
service (un seul compteur, indexé par identifiant de piste nu), le filtre des avions au sol,
l'atterrissage déduit à la perte du signal, la météo (un METAR, un TAF, des NOTAM), les axes
de piste tracés sur la carte, et le réglage du panneau (une liste à choix unique).

**La vraie question n'est pas le réglage, c'est l'attribution** : pour chaque avion, de quel
aéroport s'agit-il ? *Le plus proche* ne marche pas. Mesuré le 23/09 de 17:12 à 21:19 (heure
de Paris), avions distincts reçus :

| aéroport | < 500 ft, 5 NM | 500–1 000 ft, 5 NM | 1 000–3 000 ft, 5 NM | < 3 000 ft, 10 NM |
|---|---|---|---|---|
| Orly (LFPO) | 1 | 78 | 115 | 130 |
| De Gaulle (LFPG) | 0 | 4 | 169 | 245 |
| Le Bourget (LFPB) | 0 | 5 | 165 | 198 |
| Beauvais (LFOB) | 0 | 0 | 0 | 1 |
| Toussus (LFPN) | 8 | 22 | 22 | 31 |
| Villacoublay (LFPV) | 9 | 23 | 18 | 112 |

- **De Gaulle est exploitable pour APP et CLB** (169 avions entre 1 000 et 3 000 ft), pas pour
  T/O et T/D — même limite qu'Orly, pour la même raison (rien sous 500 ft).
- **Le Bourget « voit » surtout les avions de De Gaulle**, à 4 NM : attribuer au plus proche
  donnerait au Bourget une partie des approches de Roissy.
- **Beauvais est hors de portée** à basse altitude : l'ajouter n'apporterait rien.

**Piste envisagée, non codée** : attribuer l'avion à l'aéroport **dont il suit l'axe de
piste** — le code sait déjà choisir la piste la mieux alignée dans une liste, il suffit de lui
donner les pistes de tous les aéroports choisis —, et seulement à défaut au plus proche dans
le rayon. Identifiants de piste qualifiés par l'aéroport (`LFPG 27R`), piste en service par
aéroport, météo par aéroport, phase affichée avec son aéroport (`APP LFPG`), colonne
`airport` dans `phase_changes`. Avec un seul aéroport choisi, comportement identique à
aujourd'hui : la contribution amont de D49 reste possible.

**À trancher par le propriétaire** : quels aéroports (la mesure plaide pour Orly et De Gaulle,
les autres apportant peu) ; la météo de chacun ou du seul principal (Windy, API privée : 3
requêtes par aéroport toutes les 10 minutes) ; la présentation de plusieurs METAR.

### Q41 — Ce que la nuit du 23 au 24/09 apprend sur la transcription *(ouverte, 24/09)*

Mesuré sur les 1 459 transmissions transcrites du 23/09 21:08 au 24/09 09:00 (journal de
co-atc, bases du 23 et du 24, en lecture seule).

- **Du bruit est envoyé au modèle.** 11 % des morceaux (163) sont coupés à la limite de
  30 s ; ils portent **28 % de l'audio traité** (82 min sur 288) pour une médiane de **6,5 s
  de parole sur 30**. Aux heures calmes (5 h–7 h), 4,5 à 6,6 min de parole pour 17 à 29 min
  d'audio. Et le sidecar a rejeté **2 690 morceaux sans parole** sur la même période (26 la
  veille au soir). Hypothèse, non vérifiée : le squelch s'ouvre sur du bruit (Q37).
- **Ce bruit fait inventer le modèle.** Des boucles (« Focter Nion Focter Nion… ») dans
  **23 % des morceaux coupés à 30 s**, contre 7 à 8 % ailleurs. Le détecteur de parole du
  sidecar ne sert qu'à accepter ou refuser le morceau entier : accepté, **tout l'audio part
  au modèle, bruit compris**. Piste : ne décoder que les passages de parole. Mesurable hors
  production sur le jeu annoté (09).
- **La limite de 30 s coupe des transmissions** : sur le flux mélangé, quatre fréquences
  laissent rarement 600 ms de silence. Remède structurel connu : le flux par fréquence (D2).
- **Attente derrière le changement de modèle** : 102 décodages plus lents que le temps réel
  (jusqu'à 43 s pour 8,7 s d'audio), le modèle français de la seconde lecture étant chargé à
  tour de rôle avec l'anglais. Sans perte cette nuit, 4 pertes au pic de l'après-midi du 23.
- **L'association s'améliore avec la longueur** : 6 % des morceaux de moins d'une seconde
  de parole associés, 19 % au-delà de 8 s.
- **Petits défauts logiciels** : une perte à 02:02 (connexion coupée pendant l'envoi, non
  rejouée) ; les journaux du sidecar partent sur la sortie standard de co-atc, jetée au
  lancement, ce qui empêche de dater les 2 690 rejets.

**Complément du 25/09.** Les pertes au pic du matin sont maintenant comptées : **6 transmissions
perdues** par dépassement du délai de 60 s (06:51, deux ; 07:28, quatre), rien d'autre ne tournant
sur le Mac à ces heures. Et le coût du mélange est mesuré (doc 28, section 4) : **canal par canal,
environ quatre fois plus d'avions justes** que dans le mélange, aux mêmes heures, à précision
égale.

**Mesure lancée le 25/09 à 20 h 40** (accord du propriétaire) : « ne donner au modèle que la
parole », hors ligne, sur les 2 075 morceaux acceptés de l'enregistrement par canal du 24/09 —
passages de parole du détecteur élargis de 0,3 s ou de 1,0 s, recollés, modèle de production,
jugés à l'ADS-B contre la référence (235,8 avions justes). Le tri retire 20 % de l'audio à 0,3 s
(1 722 morceaux touchés), 11 % à 1,0 s (715). Sur le flux par canal, le bruit envoyé au modèle est
bien moindre que sur le mélange : 35 morceaux seulement atteignent 30 s. Scripts :
`whisper-lab/scripts/station/q41-parole-seule.py`, `q41-juger.py`.

**Premier résultat, 25/09 à 21 h 24** (`whisper-lab/resultats/station/2026-09-25-q41-jugement.json`) :

| Ce qu'on donne au modèle | Avions justes (vrais) | Texte trop long pour la parole | Parole rendue vide, 125,825 / 126,425 |
|---|---|---|---|
| Tout le morceau (référence) | 235,8 | 10,7 % | 8,2 / 7,6 % |
| La parole, élargie de 0,3 s | 199,4 | 1,8 % | 5,1 / 9,1 % |
| **La parole, élargie de 1,0 s** | **248,6** | **5,9 %** | 4,5 / 6,0 % |

Par canal, à 1,0 s : 124,350 **146,6** (85 %) contre 134,4 (78 %) ; 124,625 18,0 contre 18,2 ;
125,825 **40,2** (61 %) contre 47,8 (67 %) ; 126,425 **43,8** (77 %) contre 35,4 (63 %).
~~Sans entraînement, un peu plus d'avions justes (+5 %)~~ — **corrigé par le balayage du même soir** :

| Marge autour de la parole | Avions justes | Texte trop long | 125,825 |
|---|---|---|---|
| Tout le morceau (référence) | 235,8 | 10,7 % | 47,8 |
| 0,3 s | 199,4 | 1,8 % | 33,0 |
| 0,6 s | 221,0 | 3,8 % | 35,6 |
| 1,0 s | 248,6 | 5,9 % | 40,2 |
| 1,5 s | 231,8 | 7,1 % | 38,8 |
| 2,5 s | 226,6 | 9,6 % | 40,4 |

**Le gain d'avions justes à 1,0 s est du bruit** : les largeurs voisines tombent sous la
référence, le compte varie de ±15 entre réglages presque identiques. **Ce qui tient** :
l'indicateur d'invention baisse régulièrement quand la marge se resserre (presque divisé par
deux à 1 s) sans perte mesurable d'avions justes au-dessus de 0,6 s ; à 0,3 s, la perte est
nette. **Et 125,825, la fréquence faible, perd à toutes les largeurs** (33 à 40 contre 48) :
explication probable, non vérifiée — le détecteur de voix manque une partie de la parole faible,
et la couper retire de vrais mots. **Pas adopté en l'état** : à reprendre avec un détecteur plus
sensible pour le tri que pour l'acceptation, et à juger sur un autre jour.

### Q42 — Nettoyer l'audio avant la reconnaissance, sans IA *(ouverte, 24/09)*

Question du propriétaire : *« il n'y a pas des techniques non IA pour améliorer fortement la
qualité audio, supprimer les parasites, clarifier la voix en amont de la reconnaissance ? »*

**Ce qui existe déjà, vérifié le 24/09** :
- à la station, chaque canal est filtré en passe-bande par RTLSDR-Airband (`highpass = 300`,
  `lowpass = 2700`) ;
- `filtre-voix` publie `aero-clair.mp3` : passe-bande 300–2 800 Hz, compresseur, limiteur
  (`/opt/adsb/filtre-voix.sh`). **co-atc écoute `aero.mp3`, le brut.** L'effet de
  `aero-clair` sur la reconnaissance **n'a jamais été mesuré** ;
- le détecteur de voix du sidecar refuse les morceaux sans parole (jusqu'à 98 % la nuit,
  D27), mais ne retire pas le bruit des morceaux acceptés (Q41).

**Ce qu'un filtre audio ne peut pas faire** : rendre un son que le squelch a coupé (Q37),
ni empêcher le squelch de s'ouvrir sur le parasite secteur (01-station). Ces deux-là se
traitent à la source.

**Réserve de principe, non mesurée ici** : ce qui s'entend mieux ne se transcrit pas
forcément mieux ; un débruiteur spectral laisse des artefacts qu'aucun des deux modèles n'a
rencontrés. *Précision apportée sur objection du propriétaire* : les deux modèles sont des
Whisper généralistes à la base (large-v3). **L'anglais** (`jlvdoorn/whisper-large-v3-atco2-asr`,
converti par sfabriece) y ajoute un **affinage sur ATCO2**, un corpus de vraies
communications de contrôle aérien ; sa fiche annonce ~17 % de WER sur ATCO2 contre 37 à 55 %
pour large-v3 d'origine. La taille de ce corpus d'affinage n'est pas vérifiée. **Le français**
(bofenghuang) est un généraliste francophone, **sans aucune radio ni aviation**. La réponse
peut donc différer d'un modèle à l'autre : **mesurer les deux**.

**À mesurer sur le jeu annoté**, en précision et en rappel (`filtres.py`, D45) : brut,
chaîne d'`aero-clair`, débruitage spectral, peigne coupe-bande à 100 Hz, normalisation de
niveau, et ne garder que la parole (Q41).

**Où ça s'implanterait**, proposé le 24/09 par le propriétaire : **à la station**, en
service, comme `filtre-voix`. C'est le seul endroit où les fréquences sont encore séparées
(squelch, filtre par canal, peigne sur les seuls canaux touchés), et ça profite à tous les
auditeurs. `aero.mp3` reste inchangé ; un flux « pour la reconnaissance » s'ajouterait à
côté si la mesure le justifie. Côté Mac ne resterait que le tri de la parole (Q41), qui
dépend du détecteur de voix du sidecar. co-atc ne traite pas l'audio : il lit le flux
qu'on lui désigne.

### Q43 — Dégrader un corpus déjà transcrit pour affiner un modèle *(ouverte, 24/09)*

Idée du propriétaire : *« dégrader un corpus texte/audio déjà mappé avec le type de bruit,
parasite, hachage qu'on rencontre sur ma station pour créer ou affiner un modèle ? Ça
m'éviterait d'avoir à retranscrire à la main des heures de captation. »*

C'est l'**augmentation de données** (simulation du canal), une technique classique de
l'entraînement de la reconnaissance. Elle rejoint la voie n° 1 de Q30 — un meilleur modèle
acoustique sur les chiffres, par un affinage maison (stratégie C de Q1), **jamais tenté** —
en retirant son principal obstacle, l'annotation.

**Ce que la station fournit sans rien transcrire** : du bruit réel en quantité (jusqu'à 98 %
des ouvertures de squelch la nuit sont sans parole, D27) ; la forme du hachage (silences
numériques de 30 à 310 ms, en grappes, Q37) ; le canal (bande 300–2 700 Hz, AM, MP3,
écrêtage D42–D43, quatre fréquences mélangées qui se chevauchent).

**Deux cibles qui correspondent à des défauts mesurés** :
- **l'invention**, défaut dominant (D44 : 59 à 93 % d'insertions) — des clips de bruit de la
  station étiquetés *vides* apprennent au modèle à se taire ;
- **les chiffres** (Q30) — les corpus de phraséologie en sont pleins.

**Limites** :
- les corpus transcrits disponibles sont anglais (ATCOSIM, simulé et propre ; ATCO2 ;
  UWB-ATCC), sous licences non commerciales à vérifier ; **aucun corpus ATC français public**
  (03). Ils n'apprennent ni les balises de Paris, ni le mélange français-anglais, ni les
  pilotes pressés qui se chevauchent ;
- **le hachage piège l'étiquetage** : garder le texte complet sur un son dont des mots ont
  été coupés apprend au modèle à deviner — c'est-à-dire à inventer. Il faut retirer de
  l'étiquette les mots entièrement coupés, ce qui demande leur position dans le temps ;
- **l'évaluation reste humaine** : on n'échappe pas à un jeu annoté de la station, mais
  quelques centaines de clips au plus, pas des heures (55 annotés à ce jour) ;
- **le calcul** : affiner un large-v3 sur un Mac de 24 Go est à la limite, non mesuré ;
  une méthode allégée, un modèle plus petit ou quelques heures de GPU louées (données
  publiques, bruit sans parole) sont les options.

Complément sans transcription : les indicatifs **confirmés par l'ADS-B** fournissent des
étiquettes partielles gratuites sur de vraies transmissions de la station.

**Plan détaillé** : `27-augmentation-donnees.md` (24/09), à la demande du propriétaire.

**État au 25/09 au soir** : étapes 0 à 2 faites, trois mini-essais de 500 pas jugés sur la
station du 24/09 (doc 27). **Aucun ne bat la production** (236 avions justes) : ATCOSIM seul 104,
mélange de corpus 191, mélange en n'entraînant que l'oreille 158. Tous inventent moins sur le
bruit seul — mais sur des ouvertures que le détecteur de voix écarte déjà en production (D27) ;
sur les morceaux vraiment transcrits, pas de gain mesurable. Et les essais 2 et 3 **rendent vide un tiers à la moitié des morceaux de parole** des
fréquences faibles (8 % en production) — le silence appris déborde sur la parole faible. La
question reste ouverte : un essai de plus n'aurait de sens qu'avec beaucoup moins de clips vides,
et la réception séparée par fréquence (Q45, ×4) passe avant.

### Q44 — Le bloc de la Freebox remplacé : le parasite de nuit disparaît-il ? *(répondue le 25/09 : **non**)*

Complément de la station, 24/09 après-midi (`whisper-lab/echanges-station/complement-2026-09-24-apres-midi.md`) :
le propriétaire a remplacé **vers 16:15** le bloc d'alimentation CPL de la Freebox par une
alimentation classique. D41 l'avait désigné **par élimination** comme source du parasite de nuit,
après le retrait des adaptateurs CPL.

- **Le brouillage de jour n'a pas baissé**, mesuré par la station en rejouant la captation du
  22/09 (16:42–18:50, 132–133 MHz) : 17 % puis 27 % d'ouvertures sans parole, dans la fourchette
  normale de jour (18–34 %), l'écart changeant de sens selon l'heure.
- **Pour la nuit, la station conclut aussi à la mise hors de cause**, le bloc étant branché
  24 h/24 alors que le parasite ne sort que de 22 h à 7 h. C'est un raisonnement, pas encore une
  mesure : **cette nuit est la première sans ce bloc.**
- **Mesure passive côté Mac, cette nuit** : le taux de morceaux rejetés sans parole par le sidecar,
  heure par heure (la courbe de D27), relevé toutes les dix minutes dans
  `whisper-lab/journaux/exploitation/2026-09-24-nuit-surveillance.log`. Repères : 96–98 % à 2–3 h la nuit du
  20 au 21 ; **64 % en moyenne** de 21:08 à 08:56 la nuit dernière ; **6 % en journée** le 24
  (175 rejets pour 2 751 transcriptions de 08:56 à 21:11).

**Trou dans les données de co-atc du 24/09** : de **16:38:22 à 18:50:07**, `aero.mp3` portait le
groupe 132–133 MHz, pas `gros-porteurs`. La base ne le dit nulle part (`frequency_id` reste
`aero-melange`) ; **à exclure de toute analyse de `gros-porteurs`.** co-atc s'est reconnecté seul
aux trois bascules (16:35, 16:38, 18:50), première transcription 0 à 12 s après chaque coupure.

**Mesuré la nuit du 24 au 25/09 : le parasite est toujours là.** Part des morceaux rejetés sans
parole par le sidecar, heure par heure (relevés toutes les dix minutes ; co-atc arrêté de 21:55 à
22:35 pour la transcription par canal, l'heure de 22 h est donc partielle) :

| | 22 h | 23 h | 0 h | 1 h | 2 h | 3 h | 4 h | 5 h | 6 h | 7 h |
|---|---|---|---|---|---|---|---|---|---|---|
| **Nuit du 24 au 25** (sans le bloc) | 43 % | 36 % | 39 % | **87 %** | **91 %** | **87 %** | 51 % | 26 % | 14 % | 8 % |
| Nuit du 20 au 21 (D27, avec le bloc) | 61 % | 61 % | 86 % | 88 % | **96 %** | **98 %** | 95 % | 72 % | 47 % | 26 % |

Au cœur de la nuit, de 1 h à 4 h, **87 à 91 % de rejets, comme avant** : le bloc de la Freebox
n'était pas la source du parasite de nuit, ce que la station concluait par raisonnement. La courbe
paraît **plus étroite** (0 h et 4–7 h nettement plus bas), mais une nuit contre une nuit, avec un
trafic et des conditions différents (17 instances de co-atc la nuit du 20, sans effet sur le taux
d'après D26), ne permet pas d'en tirer un changement. Le parasite reste sans coupable.

**Cohérence avec 28** : en journée, le sidecar ne rejette que 6 % des morceaux du mélange, alors
que 125,825 et 126,425 s'ouvrent plus d'une fois sur deux sans parole. Le mélange colle ces
ouvertures à la parole des autres canaux : elles ne sont pas rejetées, **elles partent au modèle**
(Q41).

### Q45 — Choisir les fréquences une par une à la station, par une API claire *(en grande partie faite le 25/09 ; restent les flux séparés)*

Demande du propriétaire : *« compléter les groupes par des fréquences spécifiques […] avec une API
claire pour switcher les fréquences sur la station »*. Aujourd'hui `radio-ctl` ne bascule que sur
l'un des dix groupes, **et une simple lecture de `/radio/mode/<nom>` bascule la station**
(`do_POST = do_GET`).

Demande détaillée rédigée pour l'agent de la station :
`whisper-lab/echanges-station/demande-selection-frequences.md` — un catalogue des fréquences comme
source unique, les groupes comme sélections nommées, la sélection libre avec ses règles (fenêtre,
centre, écartement), des **flux séparés par fréquence** en option (`/aero-<id>.mp3`), un retour
automatique pour les essais, les actions en POST et JSON seulement, et un état versionné et daté.

**Pourquoi co-atc en a besoin** : le gain le mieux mesuré du projet est la réception par canal
(×4 d'avions justes, doc 28) ; et le trou du 24/09 (16:38–18:50, un autre groupe transcrit sans
que co-atc le sache) disparaît si co-atc étiquette chaque transcription avec la `version` de l'état.
**co-atc ne bascule jamais la station de lui-même.**

**Mis en place par la station le 25/09** (`whisper-lab/echanges-station/reponse-selection-frequences.md`,
lu en lecture seule) : catalogue de 68 fréquences, sélection libre (fenêtre utilisable **2,08 MHz**,
écart minimal 12,5 kHz, 8 fréquences au plus), vérification, retour automatique, **réglages de squelch
et de niveau par fréquence** (`/radio/reglage`), état versionné et daté — les routes nouvelles en
POST et JSON seulement. **Restent** : les flux séparés par fréquence (« phase 3, après mesure du
processeur ») ; et les anciennes routes, qui basculent encore sur une simple lecture (GET), par
compatibilité avec le pupitre.

**Décision du propriétaire, 25/09 au soir : des flux séparés sur toute écoute**, groupe ou sélection
libre (`http://audio.lan/aero-<id>.mp3`, MP3 mono 8 kHz pris avant le mélangeur, adresses dans
`/radio/etat`). Calendrier de la station : mesure du processeur, limite de sources Icecast portée à
12, construction derrière un interrupteur. Les commandes en GET seront fermées (405) après une
semaine d'observation. Nouveau : `GET /radio/mesures` (bruit, seuil, ouvertures et battements du
squelch par fréquence, 10 min glissantes). **Réponse d'atc-scribe** aux six questions de la station
(`whisper-lab/echanges-station/reponse-atc-scribe-flux-separes.md`) : 32 kbit/s suffit, niveau natif
conservé, 13 flux au plus, aucune commande en GET chez nous, pas de flux filtrés, et le retard de
16 s corrigé côté co-atc. **À faire côté co-atc** : retirer les options `-reconnect` de ffmpeg pour
ces flux (elles bouclent sur le 404 d'Icecast), suivre `/radio/etat` pour ouvrir et fermer les
sources, étiqueter chaque transcription avec la fréquence et la `version`. **Fait le 25/09 au soir,
sauf la `version` : voir D57** — essayé de bout en bout sur une station simulée, pas encore sur la
vraie.

**Mise à jour de la station, 25/09 à 20 h 20** (`whisper-lab/echanges-station/reponse-station-2026-09-25-20h20.md`) :
- **feu vert technique** : 8 flux séparés lus ensemble pendant 20 min, 10 % d'un cœur au pire, sans
  interruption ni débordement. Reste la construction ; la station préviendra quand `/radio/etat`
  donnera les adresses. Six bascules d'essai entre 18 h 29 et 20 h 16 (versions 12 à 17) — sans
  effet pour nous, co-atc était arrêté ;
- **deux corrections à notre réponse** : les flux seront en **débit variable, 8 à 13 kbit/s**
  (réglages LAME fixes de RTLSDR-Airband ; l'étiquette `icy-br=32` d'Icecast est fausse) — c'est
  l'encodage exact des fichiers du 24/09, donc des mesures ×4. Et le tampon d'Icecast (64 ko)
  vaut **50 à 65 s de son passé, pas 16 s** : toute la fenêtre ADS-B de ±60 s. **La correction
  de l'heure devient obligatoire** : compter les trames décodées (576 échantillons = 72 ms) contre
  l'horloge depuis la connexion, ou réduire le tampon des seuls `aero-<id>.mp3` (décision du
  propriétaire) ;
- **`aero.mp3` a une micro-coupure de 125 ms toutes les 5 à 10 min** quand les flux séparés
  tournent ; les flux séparés n'en ont pas. C'est un changement du flux écouté chaque jour :
  **au propriétaire de l'accepter ou non** ;
- les commandes en GET seront fermées **le 26/09 au soir** (405, et `Content-Type:
  application/json` obligatoire en POST). **Sans effet pour nous** : ni co-atc ni les scripts du
  laboratoire n'appellent `/radio/` (vérifié le 25/09 au soir).

**Décisions du propriétaire, 25/09 au soir** : **réduire le tampon des seuls `aero-<id>.mp3`**
(`aero.mp3` inchangé ; coût accepté : une ou deux secondes de plus au démarrage d'une écoute seule
dans un navigateur) — co-atc corrigera l'heure quand même, en comptant les trames ; et pour la
micro-coupure d'`aero.mp3`, **la cause d'abord**, avant que les flux séparés tournent en permanence.
Réponse à la station : `whisper-lab/echanges-station/reponse-atc-scribe-2026-09-25-soir.md`.

**La cause de la micro-coupure, selon la station** (25/09 au soir, lecture du code de
RTLSDR-Airband 5.0.12 ; `whisper-lab/echanges-station/reponse-station-2026-09-25-soir-coupures.md`) :
un seul fil encode et envoie toutes les sorties, le mélange compris, toutes les 1/8 s ; chaque flux
séparé lui ajoute un encodage MP3 et un envoi à Icecast. Quand il prend plus de 1/16 s de retard, le
mélangeur superpose le lot suivant à celui qui n'est pas parti : **le mélange saute 125 ms**
(superposition, pas silence). Compteur `output_overrun_count` : **0 en 30 min sans flux, 10 en
60 min avec 4 à 8 flux**. Remède candidat : l'option `multiple_output_threads`, qui donne au
mélange son propre fil — essai d'environ 1 h 30 **quand le propriétaire n'écoute pas** (8 flux sans
lecteur avec un seul fil ; 8 flux et 8 lecteurs avec fils séparés ; aucun flux avec fils séparés,
pour vérifier qu'`aero.mp3` ne change pas). **Pas de flux séparés en permanence tant que ce n'est
pas réglé.** Tampon des `aero-<id>.mp3` : **environ 4 ko (3 à 4 s)**, à la mise en service.
Pour co-atc : ces sauts ne touchent pas les flux séparés ; sur `aero.mp3`, ils ne coûtent que
125 ms de son chacun, et le comptage des trames contre l'horloge les verra.

**L'essai de la station, 25/09 de 21 h 19 à 22 h 49** (`whisper-lab/echanges-station/
reponse-station-2026-09-25-22h55-essai-fils.md`) : avec un seul fil et 8 flux, **2 dépassements en
20 min** (12 en 80 min sur deux essais, environ un toutes les 7 min) ; avec `multiple_output_threads`,
**aucun en 40 min**, 8 flux et 8 lecteurs ; l'option seule ne change rien à `aero.mp3` ni au
processeur. Retenu par la station, à la décision du propriétaire : l'option avec chaque
configuration à flux séparés, et le tampon réduit sur les seuls `aero-<id>.mp3`. Rien n'est encore
en service.

**Notre comptage de trames ne confirme pas les 2 sauts de T1** (`whisper-lab/resultats/station/
2026-09-25-horloge-essai-fils.txt`) : sur la fenêtre T1, le son reçu d'`aero.mp3` suit l'horloge du
Mac à ±50 ms sans décrochage, où deux sauts donneraient −250 ms ; la dérive entre horloges
(+3 à +37 ppm) ne peut pas les cacher. Soit un dépassement compté ne raccourcit pas le mélange,
soit la mesure a un défaut non vu — non tranché, et sans conséquence pratique : avec les fils
séparés il n'y en a plus, et co-atc date le son par sa place dans le flux (D57). Deux anomalies
de notre côté, non expliquées : six silences de 15 s ou plus sur notre lecture d'`aero.mp3` pendant
T2 (21:54 – 22:24), alors que le Mac joignait l'ADS-B de la station sans erreur ; puis « No route
to host » vers la station de 22:24:40 à 23:48, la station n'ayant pas redémarré. Réponse :
`whisper-lab/echanges-station/reponse-atc-scribe-2026-09-26-essai-fils.md`.

**Réponse de la station, 26/09 au matin** (`whisper-lab/echanges-station/reponse-station-2026-09-26-matin.md`) :
**le comptage de trames avait raison**. Un dépassement compté retarde le lot du mélangeur de
1/8 s au plus, **sans perte ni superposition** dans le cas courant — la station avait surestimé
l'effet. Les fils séparés restent retenus (ils suppriment même ce retard). **Les deux anomalies
sont côté Mac** : pendant nos six silences, Caddy (le proxy devant Icecast) note une réponse
interrompue *vers* notre client ; pendant le « No route to host », la station pingait le Mac
sans un échec et un ssh du Mac vers elle a réussi à 22:51. Voir Q22.

**Phase 3 en service le 26/09 à 11:16 (version 23)** (`whisper-lab/echanges-station/reponse-station-2026-09-26-13h45-phase3.md`) :
un flux par fréquence sur toute écoute, `http://audio.lan/aero-<id>.mp3`, adresse stable ;
`/radio/etat` donne `flux_url` et `flux_present` par fréquence et un bloc `flux_separes` ; tampon de
4 096 octets sur les seuls `aero-<id>.mp3` (`aero.mp3` garde 65 535, décision du propriétaire) ;
`multiple_output_threads` partout ; une fréquence seule à niveau ≠ 1 porte ce niveau dans son flux
(`ampfactor_canal`). **Les commandes en GET restent ouvertes** (fermeture annulée par le
propriétaire). **Mesuré depuis le Mac le 26/09 vers 14 h** sur `aero-123875.mp3` (Orly approche,
seule) : premier paquet **2,75 s** après la connexion, 2 800 octets (≈ 2,6 s de son ancien), puis
des paquets de 1 400 octets toutes les 1,4 s, **8,5 kbit/s**. `radio-ctl-sync` n'ouvre plus que
les flux `flux_present` à l'adresse donnée. **Reste** : le premier essai réel de co-atc sur ces
flux.


### Q46 — Les indicatifs à lettres échappent presque à l'association *(26/09 — mesurée : +27 % d'avions justes ; **décidée, voir D58**)*

Relevé par le propriétaire pendant le premier essai sur les flux séparés (123,875, Orly Approche) :
*« j'ai eu un Uniform Echo que moi-même j'ai compris à l'oreille, l'avion est bien là, mais il n'a
pas réussi à retranscrire l'indicatif »*. L'avion est à l'ADS-B : **VLG7UE**, « Vueling Seven
Uniform Echo », à 6 700 ft à l'est d'Orly.

**Deux causes, dont une de structure.** La transcription a mal rendu l'indicatif. Mais même bien
transcrit, **l'association ne s'appuie que sur un nombre d'au moins trois chiffres** (Q30 : en
dessous, le hasard égale la vérité) ; les lettres épelées n'ajoutent qu'un bonus à un score déjà
acquis par les chiffres, ou presque (`internal/transcription/phraseology/matcher.go`). « 7UE » n'a
qu'un chiffre.

**Mesuré sur la capture ADS-B de référence du 24/09** (1 408 indicatifs distincts) :

| Forme de l'indicatif | Part | Exemple |
|---|---|---|
| **lettres, moins de 3 chiffres** | **59 %** | AFR11NQ, VLG7UE, EZY36VJ |
| chiffres seuls, 3 ou plus | 24 % | AFR1234 |
| lettres, 3 chiffres ou plus | 7 % | — |
| autres (immatriculations, militaires) | 7 % | F-GXXX |
| chiffres seuls, moins de 3 | 2 % | — |

Même proportion en direct le 26/09 à 13:41 (91 indicatifs : 59 %). **L'association ne voit
vraiment qu'un tiers des avions.** Le point de départ de Q30 (≈ 8 % de transmissions reliées) se
lit autrement : la plupart des indicatifs entendus ne pouvaient pas être reliés.

**Piste** : compter les lettres épelées comme les chiffres. « 7UE » a 10 × 26 × 26 = 6 760
combinaisons, plus que les 1 000 de trois chiffres : une règle « au moins trois caractères,
lettres comprises » serait au moins aussi sûre contre le hasard. **À mesurer avant de décider**,
sur les transcriptions par canal du 24/09 avec le contrôle par ciel mélangé : avions justes gagnés,
précision.

**Le même avion a reparlé quelques minutes plus tard**, transcrit *« speed 200 welling seven uniform
make on »* : « welling » pour Vueling, « seven uniform » exact, « echo » devenu « make on ». Dans le
ciel à cet instant (91 indicatifs), **un seul commence par « 7U » : VLG7UE**, et trois vols seulement
sont de Vueling (VUELING est bien dans `assets/airlines.dat`). L'information pour trouver l'avion
était dans la transcription ; c'est l'association qui la jette. La mesure devra donc essayer aussi :
un nom d'exploitant approché (« welling »), une lettre manquante tolérée, et le rapprochement vers le
mot de l'alphabet aéronautique le plus proche après une lettre épelée (« make on » -> « echo ») —
chacun jugé contre le hasard.

**Mesuré le 26/09** (`cmd/phraseology`, options `-alnum`, `-fuzzy-operators`, `-every` ;
résultats `whisper-lab/resultats/station/2026-09-26-q46/`). Deux règles, désactivées par défaut :
**chiffres et lettres ensemble** (« seven uniform echo » lu 7UE ; la partie vol entière pèse comme
trois chiffres exacts, 0,9 ; amputée de sa dernière lettre, 0,5, sous le seuil de 0,6, donc
seulement avec une autre preuve) et **nom d'exploitant approché** (une lettre d'écart, deux pour les
noms longs, ou mêmes consonnes : « welling » = VUELING), cherché parmi les seuls exploitants du ciel.

| 24/09, 4 fréquences, contrôle par ciel mélangé | Avions justes | Hasard | Précision |
|---|---|---|---|
| Règles actuelles | 239,6 | 88,4 | 73 % |
| **+ chiffres et lettres** | **303,4** (+27 %) | 97,6 | **76 %** |
| + nom approché | 306,0 (+28 %) | 97,0 | 76 % |

Le gain se retrouve **sur les quatre fréquences** (124,350 : 137 -> 163 ; 124,625 : 20 -> 28 ;
125,825 : 48 -> 56 ; 126,425 : 35 -> 57), et la précision monte : ce n'est pas du bruit. Le nom
approché ajoute peu en nombre, mais c'est lui qui sauve l'exemple du propriétaire. Sur l'essai
d'Orly du 26/09 (40 transmissions couvertes par l'ADS-B) : 2 avions -> 4, dont **VLG7UE deux fois** —
à 13:37 (« holding seven uniform echo », partie vol entière) et à 13:42 (« welling seven uniform
make on », partie vol amputée + nom approché) ; la troisième (« holding seven uniform make on »,
sans nom) reste à juste titre sans avion.

**Un défaut de mesure trouvé en chemin** : l'outil de mesure n'appelait l'association que si la
transmission contenait un groupe d'au moins deux chiffres — ce qui cachait précisément ces
indicatifs. La production, elle, envoie tout ; `-every` fait de même. Les mesures antérieures
(Q30, doc 28…) sont peu touchées : +4 avions justes sur 236 le 24/09 avec les règles actuelles.

**Décidé le 26/09 (D58)** : les deux règles sont activées, et réglables depuis le panneau.

**Pistes suivantes, relevées le 26/09 sur les 1 672 transmissions du 24/09 restées sans avion**
(`whisper-lab/scripts/station/q46-abreviations.py` ; hasard estimé en remplaçant les avions
rattachés sur la même fréquence dans les 5 minutes par ceux d'une autre fréquence, un à un) :

| Ce que la transmission contient | Même fréquence | Hasard | Net |
|---|---|---|---|
| les **dernières lettres** d'un avion rattaché juste avant (« Sierra Bravo ») | 17 | 1,3 | **≈ 16, propre** |
| les **deux derniers chiffres** d'un avion rattaché juste avant (« cinq quatre ») | 22 | 13,3 | ≈ 9, bruité |
| le **nom de sa compagnie** seulement | 90 | 50,7 | ≈ 39, bruité |

1. **Le contexte de l'échange** : après le premier contact, pilote et contrôleur abrègent. co-atc
   garde déjà les avions entendus récemment sur la fréquence, mais seulement pour départager des
   ex-æquo. Les lettres de fin sont une piste propre (+16, soit +5 %) ; le nom seul et les deux
   chiffres demandent une condition de plus (un seul avion de la compagnie récemment sur la
   fréquence).
2. **Les immatriculations** : 52 immatriculations françaises (F + 4 lettres) le 24/09, **écartées
   entièrement** par l'association (`matcher.go` : ni chiffres ni lettres après le préfixe). Elles
   comptent surtout sur les fréquences locales (Saint-Cyr, Toussus, transit d'Orly), épelées
   (« Fox Golf Alpha Bravo Charlie ») ou abrégées (« Fox Bravo Charlie »). À mesurer sur un
   enregistrement d'une fréquence locale : le corpus du 24/09 est l'approche de CDG.
3. **Le ciel propre à chaque fréquence** : l'association compare à une centaine d'avions ; sur
   Orly Approche, seuls ceux qui descendent vers Orly sont plausibles. Moins de candidats, moins de
   hasard (24 % des rattachements le 24/09, 97 sur 403), et des indices plus faibles (« 7U » seul)
   deviendraient acceptables. À mesurer, avec le service de chaque fréquence du catalogue.
4. **Une lettre mal entendue** (« make on » pour « echo ») : tolérer une lettre fausse quand la
   compagnie est nommée. Petit gain attendu, à mesurer contre le hasard.

**Pistes 1 et 3 mesurées le 26/09** (choix du propriétaire ; `cmd/phraseology` : `-context-letters`,
`-context-digits`, `-context-names`, `-context-swap`, `-sectors`, `-positions`, `-partial` ;
résultats `whisper-lab/resultats/station/2026-09-26-q46-pistes/`). Référence : les règles en
service (D58) avec la mémoire de 2 minutes de la production, **307 avions justes, 76 %**.

*Le fil de l'échange.* Le contrôle par ciel mélangé est **trop indulgent** pour ces règles :
chaque transmission du contrôle tire son ciel à un autre moment, où les avions que ce contrôle
vient de rattacher ne sont presque jamais, et la règle n'y joue pas. Contrôle équitable ajouté :
le vrai ciel, avec la mémoire d'**une autre** fréquence (des avions présents, qui ne lui parlent pas).

| Règle ajoutée | Rattachés, sa fréquence | Rattachés, mémoire d'une autre (hasard) | Gain net |
|---|---|---|---|
| — | 405 | 403 | — |
| dernières lettres d'un avion entendu | 413 | 403 | **+8, sans hasard** |
| deux derniers chiffres (un seul avion récent finit ainsi) | 419 | 404 | **+13** |
| nom seul (un seul avion récent de la compagnie) | 482 | 412 | +68 au-dessus du hasard |

Le nom seul rapporte le plus, mais **sa justesse échappe à l'ADS-B** : l'avion désigné est
toujours dans le ciel, et un autre avion de la même compagnie, pas encore rattaché, peut être
celui qui parle. À juger à l'oreille sur un échantillon avant toute activation.

*Le secteur de la fréquence.* Positions reconstituées des traces heatmap de la station
(`adsb-2026-09-24-par-canal-positions.json`, 84 687 points). Sur 124,350, 95 % des avions
rattachés sont à moins de 37 NM de Roissy et sous 16 000 ft ; le ciel entier compte 123 avions à
10:00 UTC, dont 27 dans 40 NM sous 15 000 ft. Un avion sans position connue est gardé.

| Secteur (approche / Paris Contrôle DG) | Avions justes | Hasard | Précision | 125,825 |
|---|---|---|---|---|
| aucun (référence) | 307,0 | 98,0 | 76 % | 57 |
| 45 NM, 17 000 ft / 60 NM, 27 000 ft | 319,0 | 53,0 | 86 % | 46 |
| **60 NM, 20 000 ft / 80 NM, 30 000 ft** | **330,8** | **61,2** | **84 %** | 52 |
| + partie vol amputée acceptée (0,6) | 346,6 | 68,4 | 84 % | 52 |
| **+ dernières lettres + deux derniers chiffres** | **367,6** | 68,4 | **84 %** | 55 |

**Bilan des règles propres ensemble : +20 % d'avions justes et 8 points de précision** (307 ->
368, 76 % -> 84 %), sans le nom seul. Réserves : les bornes du secteur ont été choisies en voyant
les données de 124,350 (deux nombres par type de fréquence, risque de sur-ajustement faible mais
réel) — **à confirmer sur un autre jour et un autre aéroport** (l'essai d'Orly du 26/09) ;
125,825 perd encore quelques avions justes (57 -> 55).

**Ce que demande la production** : le secteur de chaque fréquence (aéroport, rayon, tranche
d'altitude) porté par la source — pour les flux séparés, `radio-ctl-sync` le déduirait du champ
`service` du catalogue de la station (« Approche De Gaulle », « Orly Approche », « croisière haute
FL360-380 ») ; les positions dans le ciel que reçoit l'association ; et les réglages au panneau.

### Q47 — 123,875 hache dans la sélection à quatre fréquences *(ouverte, 26/09)*

**Signalé par le propriétaire à l'écoute**, pendant la séance de validation : « bcp de
hachage/squelch sur Orly Approche, mais ça dépend de la transmission ». Le matin, 123,875 seule
lui avait paru « vraiment bonne ».

**Mesuré** sur les flux séparés, en simple auditeur (`whisper-lab/scripts/station/hachage.py` ;
enregistrements `whisper-lab/audio/2026-09-26-*-hachage/`). Squelch fermé, RTLSDR-Airband sort
un silence numérique ; une fermeture de moins de 0,3 s au milieu d'une transmission est un
hachage. Même seuil (SNR 10), même niveau de canal (×1,10) aux deux moments.

| Flux | Écart au centre de la clé | Transmissions hachées | Coupures / min de son | Ouvertures < 0,3 s |
|---|---|---|---|---|
| 123,875 seule, 13:32–14:17 (centre 123,4) | +0,475 MHz | 5 % (3/58) | 1,7 | 4 en 44 min |
| 123,875, 17:01–17:21 (centre 124,85) | −0,975 MHz | **66 % (23/35)** | **42,7** | **101 en 20 min** |
| 124,350, même moment | −0,500 MHz | 35 % (23/66) | 5,3 | 1 |
| 124,625, même moment | −0,225 MHz | 0 % (0/13) | 0 | — |
| 125,825, même moment | +0,975 MHz | 37 % (23/63) | 8,3 | 16 |

- **C'est la radio, pas le numérique** : durées de coupure dispersées (10 à 290 ms), pas une
  valeur fixe comme le saut de 125 ms du mélangeur (Q45, 25/09).
- **Le contrôleur hache aussi** (émetteur au sol, fixe) : 2 transmissions sur 4 sur 123,875,
  6 sur 9 sur 125,825, contre 0 sur 9 à midi. La distance des avions n'explique donc pas tout.
  Effectifs petits (attribution par l'horodatage de co-atc, ±3 s).
- **Les deux fréquences au bord de la bande hachent le plus**, mais pas à égalité : 123,875
  cinq fois plus que 125,825. Hypothèse à vérifier côté station, **non mesurée** : l'image
  repliée de 126,425 (Paris Contrôle, chargée) tombe à 123,865 quel que soit le centre
  (126,425 − 2,56) ; à midi 126,425 était à 3 MHz du centre, très atténuée par le filtre du
  tuner, maintenant à 1,6 MHz, juste hors de la bande passante.

**Relevés de la station** (agent de la station, 26/09) :
- compteurs d'airband de 16:44 à 17:24 : sur 123,875, 603 ouvertures de 0,37 s en moyenne,
  alors que son bruit de fond est le plus bas des quatre (−41,9 dBFS). C'est la signature d'un
  brouilleur intermittent, pas d'une perte de sensibilité. 125,825 bat de façon chronique, même
  dans `gros-porteurs` (Q37) : c'est un témoin imparfait ;
- **aucun retard du mélangeur** en 66 min (`output_overrun_count` à 0, 257 relevés) ;
- pourtant `aero.mp3` est haché à 17:50 : salves de 0,6 s en médiane, 54 % de moins d'1 s, contre
  4,4 s et 17 % à 16:46. **La cause est côté réception**, pas dans le mélange.

**Reste** : la contre-épreuve. Le retour automatique de 18:45 remet 123,875 seule, recentrée ;
enregistrement de 18:46 à 19:30, comparé à 17:01–17:21, avec les compteurs par canal de la
station relevés à la minute. Et la diaphonie de 126,425 : les transcriptions ne tranchent pas.
