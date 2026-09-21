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

### Q9 — Transcrire les ATIS : **oui, mais pour une seule fréquence** *(prémisse remise en cause le 20/09, voir Q32)*

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

### Q22 — Le Mac est sur le même sous-réseau deux fois *(nouvelle, 16/09)*

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

**Ce qui n'est pas couvert, et c'est assumé** : un `kill -9` sur co-atc orpheline le
sidecar — aucun parent ne peut s'en prémunir. En revanche il n'y a **aucun appel `Fatal`
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

### Q32 — L'ATIS sature-t-il vraiment, ou est-ce le pic DC du tuner ? *(nouvelle, 20/09)*

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
`SQLITE_BUSY` ; les écritures de transcription ont commencé à échouer vers 7 h. Et la
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

