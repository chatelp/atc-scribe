# Plan — affiner la reconnaissance par augmentation de données

*Note de projet du 24 septembre 2026. L'idée est celle du propriétaire (Q43) ; ce document
dit ce qu'on ferait, dans quel ordre, et à quoi on saurait que ça marche. **Étapes 0 et 1
faites la nuit du 24 au 25/09 : voir « État au 25/09 » à la fin.***

## En une phrase

Prendre des enregistrements de contrôle aérien **déjà transcrits par d'autres**, leur faire
subir ce que la station fait subir à la voix — parasite, hachage, bande étroite, écrêtage,
compression, chevauchements —, et s'en servir pour affiner le modèle anglais, **sans
retranscrire des heures de captation à la main**.

## Pourquoi c'est la bonne cible

- **C'est la voie n° 1 restée intacte** (Q30, doc 23) : un meilleur modèle acoustique sur les
  chiffres, par un affinage maison (stratégie C de Q1). Jamais tentée, faute de données
  annotées. L'augmentation retire cet obstacle.
- **Le défaut dominant est l'invention**, pas la mauvaise écoute (D44) : de 59 % d'insertions
  (anglais) à 93 % (français) sur les 42 premiers clips annotés.
- **Le bruit fait inventer** (Q41) : des boucles dans 23 % des morceaux de 30 s bruités,
  contre 7 % ailleurs.
- **Le modèle anglais n'a jamais entendu cette station.** C'est un Whisper large-v3
  généraliste, affiné par jlvdoorn sur du contrôle aérien : 2 800 pas, lot de 16, taux
  d'apprentissage 1e-5, plusieurs GPU ; le jeu d'affinage n'est pas documenté sur sa fiche.

## Ce qu'on a déjà

### La voix et le texte : des corpus transcrits

| Corpus | Contenu | Taille | Licence |
|---|---|---|---|
| **ATCOSIM** (TU Graz, Eurocontrol) | contrôleurs en **simulation**, micro-casque, **son propre** ; 10 locuteurs non natifs | 10 h | gratuit, **sans redistribution** |
| **UWB-ATCC** | contrôle tchèque **réel**, en anglais, déjà passé par la radio | ~20 h, 14 113 segments, 8 kHz | CC BY-NC-SA 4.0 |
| **ATCO2**, échantillon | communications **réelles**, captées sur des fréquences publiques ou fournies par des prestataires | 1 h, 871 segments | accord d'utilisation à lire |

ATCOSIM, propre, est **la matière à dégrader**. UWB-ATCC et ATCO2, déjà « radio », servent de
référence de réalisme et de complément réel. **Aucun corpus de contrôle aérien français
public** n'existe (03) : le modèle français n'est pas concerné par ce plan.

### Le bruit : le matériau de la station, sans aucune transcription

| Source (laboratoire, `audio/…`) | Ce qu'on en tire |
|---|---|
| `2026-09-15-132-133-nuit-continu` — 7 canaux, 4 h continues | **parasite secteur pur**, par canal : 4 des 7 canaux ne portent que lui (doc 21) |
| `2026-09-15-orly-approche-continu` — 5 canaux, 2 h 38 de jour | bruit de jour, par canal, dont **124,350, 124,625 et 125,825**, trois des quatre fréquences de `gros-porteurs` |
| `2026-09-20-gros-porteurs-nuit` (disque externe) — 14 251 morceaux du mélange | milliers de morceaux sans parole, chevauchements réels |
| mesures déjà faites | hachage : silences numériques de 30 à 310 ms, en grappes (Q37) ; écrêtage (D42–D43) ; bande 300–2 700 Hz ; MP3 |

### Le juge

- **`verite-terrain.json`** : 55 transmissions annotées par le propriétaire au 23/09 (42 figées
  pour le réglage, les autres en contrôle). **Réserve** : elles viennent de quatre fréquences
  du 12/09 (127,750, 128,950, 129,525, 132,500), pas du groupe `gros-porteurs`.
- **L'appariement ADS-B** de la captation du 15/09 (1 468 transmissions), avec ciel mélangé :
  72 % de précision, ~100 vrais à battre (Q30).
- **`filtres.py`** mesure séparément la précision et le rappel (D45) ; le taux d'erreur seul
  récompense l'effacement.

### La machine

- Mac M4, 24 Go. Dans l'environnement du laboratoire : torch 2.14 et transformers 5.17, calcul
  sur la puce graphique disponible ; peft, datasets, accelerate absents.
- ⚠️ **Le `.venv` du laboratoire fait tourner le sidecar de production** (`config.toml`).
  L'entraînement se fait dans un **environnement séparé**, jamais en installant des paquets
  dans celui-là.
- ⚠️ **L'entraînement dispute la puce graphique à la production.** Pendant qu'il tourne, la
  transcription ralentit et la file déborde (Q41). À faire co-atc arrêté, ou en acceptant
  des pertes, décidé à chaque fois.
- La conversion vers MLX est un chemin connu (D10 ; c'est ainsi que le modèle actuel a été
  produit).

## Les étapes

Chaque étape a un livrable, une mesure, et une condition pour passer à la suivante. Les durées
sont **des estimations, pas des mesures**.

### 0 — Faisabilité *(½ jour)* — point d'arrêt

- Environnement d'entraînement séparé ; corpus téléchargés **hors dépôt**
  (`whisper-lab/entrainement/corpus-externes/`), licences lues et notées.
- **Essai minuscule** : 50 pas d'affinage léger (LoRA) sur large-v3, puis sur large-v3-turbo et
  medium ; mémoire maximale et secondes par pas relevées.
- **Aller-retour complet** : poids affinés → conversion MLX → chargement par un sidecar d'essai,
  sur un autre port que la production.
- **On passe si** un affinage de quelques milliers de pas tient en une nuit sur ce Mac. Sinon :
  modèle plus petit, ou quelques heures de GPU louées — n'en sortiraient que des données
  publiques et du bruit sans parole.

### 1 — Banque de bruit et empreinte de la station *(1 jour)*

- Extraction automatique, par le détecteur de voix, des passages **sans parole** des corpus de
  la station ; rangés par canal, heure et nature (parasite de nuit, souffle de jour, porteuse).
- **Empreinte mesurée, par canal** : spectre moyen, niveau de bruit, distribution des coupures
  de squelch (durée, espacement, grappes), taux d'écrêtage, taux de chevauchement dans le
  mélange.
- Livrable : `resultats/entrainement/empreinte-station.json`.

### 2 — Chaîne de dégradation, et preuve qu'elle ressemble *(1 à 2 jours)* — point d'arrêt

Dans l'ordre du trajet réel : passe-bande 300–2 700 Hz ; bruit de la banque, à un rapport
signal/bruit tiré dans la distribution mesurée ; écrêtage ; squelch simulé (coupures selon
l'empreinte) ; chevauchement d'une autre voix, à faible dose ; MP3 au débit de la station.

**La validation est le cœur du plan**, parce qu'un modèle entraîné sur un bruit irréaliste
apprend à vivre dans un monde qui n'existe pas :
- **statistique** : l'empreinte d'ATCOSIM dégradé doit tomber dans celle de la station ;
- **comportementale** : le modèle **actuel** doit se tromper sur ATCOSIM dégradé comme sur la
  station — insertions, boucles, chiffres faux, du même ordre. S'il réussit trop bien sur
  l'audio dégradé, la dégradation est trop douce ; on ne passe pas.

### 3 — Étiquetage honnête *(½ à 1 jour)*

- **Position de chaque mot dans le temps**, par alignement automatique sur l'audio propre, avant
  dégradation.
- **Un mot entièrement coupé par le squelch simulé est retiré de l'étiquette.** Le garder
  apprendrait au modèle à deviner — c'est-à-dire à inventer, le défaut qu'on veut réduire.
- **Des clips de bruit seul de la station, étiquetés vides**, pour apprendre à se taire ;
  proportion à régler (de l'ordre de 10 à 20 %).
- Les chiffres écrits comme la grammaire les lit (D14), pour que l'affinage ne change pas la
  forme du texte en même temps que son fond.

### 4 — Premier affinage *(1 à 2 jours, calcul compris)*

- Affinage léger du modèle anglais actuel, taux bas, peu de pas.
- Mélange : ATCOSIM dégradé, une part d'UWB-ATCC et d'ATCO2 (réels), les clips vides, et **une
  part de données non dégradées** pour ne pas désapprendre.
- Validation interne sur une partie du corpus dégradé mise de côté — **jamais sur les clips de
  la station**, réservés au jugement.

### 5 — Le jugement *(½ journée)* — point d'arrêt

| Mesure | Référence actuelle | Il faut |
|---|---|---|
| Clips annotés, contrôle seulement : mots inventés, précision, rappel (`filtres.py`) | D44–D45 | moins d'inventions **sans perte de rappel** |
| Clips annotés sans parole | le modèle actuel y invente | rien, ou presque |
| Appariement ADS-B du 15/09, ciel mélangé | 72 % de précision, ~100 vrais | plus de vrais, **précision ≥ 72 %** |
| Transmissions françaises (passent d'abord par le modèle anglais) | D44 | aucun recul |

Avec 55 clips, **seul un écart net compte**. En cas d'échec, les étapes 2 et 3 disent où
chercher : réalisme ou étiquetage.

### 6 — Mise en service *(si, et seulement si, l'étape 5 passe)*

- Le nouveau modèle en `--model-en` du sidecar ; suivi par l'appariement quotidien.
- **Les poids restent locaux** tant que les licences des données (non commerciales) ne sont pas
  vérifiées : pas de publication dans le dépôt public.

## Ce que le plan ne fait pas

- **Le français** : pas de corpus. Plus tard, peut-être, de la phraséologie française en voix de
  synthèse, dégradée de la même façon — avec un écart synthèse-réel à mesurer.
- **Le vocabulaire de Paris** — balises, indicatifs, mélange des langues : un corpus étranger ne
  l'apprend pas. Complément sans transcription : les **indicatifs confirmés par l'ADS-B** donnent
  des étiquettes partielles gratuites sur de vraies transmissions de la station.
- **Supprimer le hachage et le parasite** : l'augmentation apprend au modèle à vivre avec. Les
  causes se traitent à la source (Q37, Q42, 01-station).

## Risques

| Risque | Parade |
|---|---|
| Dégradation irréaliste | validation de l'étape 2, point d'arrêt |
| Apprendre à deviner | étiquetage de l'étape 3 |
| Désapprendre ce qui marchait | part de données propres, validation interne |
| Juge trop petit, et sur d'autres fréquences | seul un écart net compte ; élargir le jeu annoté aux fréquences `gros-porteurs` (captation par canal demandée le 24/09) |
| Production ralentie pendant l'entraînement | co-atc arrêté, ou pertes acceptées, décidé à chaque fois |
| Licences | corpus hors dépôt, poids non publiés |

**Ordre de grandeur** : 5 à 8 jours de travail, calcul compris, avec trois points d'arrêt
(après les étapes 0, 2 et 5). **Estimation, pas mesure.**

## Sources

- Modèle anglais : [jlvdoorn/whisper-large-v3-atco2-asr](https://huggingface.co/jlvdoorn/whisper-large-v3-atco2-asr), converti en MLX par sfabriece (fiche lue dans le cache local le 24/09).
- [ATCOSIM, TU Graz](https://www.spsc.tugraz.at/databases-and-tools/atcosim-air-traffic-control-simulation-speech-corpus.html)
- [UWB-ATCC](https://huggingface.co/datasets/Jzuluaga/uwb_atcc)
- [ATCO2, échantillon d'une heure](https://huggingface.co/datasets/Jzuluaga/atco2_corpus_1h)

## État au 25/09 au matin

### Préparation (24/09 au soir, accord du propriétaire)

- **Corpus téléchargés sur le disque externe** (`whisper-lab/entrainement/corpus-externes`) : ATCOSIM 2,2 Go,
  UWB-ATCC 0,7 Go, ATCO2 une heure 0,1 Go. Transcriptions en chiffres épelés (*« lufthansa four
  three nine three descend to flight level two seven zero »*), la forme que lit la grammaire.
  ATCO2 : accord complet non trouvé dans la copie, usage accepté par le propriétaire sur la base
  des conditions affichées (recherche, développement et évaluation de la reconnaissance ATC
  anglaise).
- **Environnement d'entraînement séparé**, sur le disque externe (`.venv-entrainement`, 1,1 Go) :
  torch 2.14, transformers 5.17, peft 0.21, datasets 5.0.1, accelerate 1.15.

### Étape 0 — faisabilité : **passée**

Sans aucun téléchargement de poids : les deux tailles viennent du cache du disque externe
(`whisper-medium.en` affiné ATC de jacktol ; l'architecture large-v3 prise sur le modèle français
de bofenghuang — on mesure le coût d'un pas, pas la qualité). Affinage léger LoRA (q, v), 20 pas
sur des clips ATCOSIM, puis fusion, conversion MLX et transcription par `mlx_whisper`.

**Premier essai, 22:32 : échec des deux tailles**, mémoire graphique saturée à 30 Go avec un lot
de 4 et sans rien d'autre (l'attention du codeur sur 1 500 trames, gardée pour la rétropropagation).
**Second essai, 01:13, co-atc tournant à côté** — lot de 2, points de reprise
(`gradient_checkpointing`), mémoire graphique plafonnée à 0,6 fois la recommandation (~10,7 Go)
pour qu'un dépassement échoue net plutôt que de faire swapper la production :

| | Medium | Large (taille du modèle anglais de co-atc) |
|---|---|---|
| Paramètres, dont entraînables | 764 M, 9,4 M | 1 543 M, 15,7 M |
| Secondes par pas | **2,5** | **4,8** |
| Mémoire graphique max | 5,6 Go | 9,1 Go |
| Perte avant → après 20 pas | 0,92 → 0,30 | 1,75 → 0,25 |
| Fusion et sauvegarde / conversion MLX | 4,5 s / 7,3 s | 21,6 s / 12,7 s |
| Transcription par le modèle converti (référence : *lufthansa four three nine three descend to flight level two seven zero*) | *lufthansa four three nine three descend flight level two seven zero* | *luftanzer four three nine three descent flight level two seven zero* |

**Aucune perte de transmission pendant les deux essais** (0 dépassement de délai). **Ordre de
grandeur** : 4 000 pas de large en ~5 h 20, une nuit. La perte ne mesure ici que l'apprentissage
de 36 clips, pas une qualité.

**Mesuré au passage** : un **second modèle en inférence** à côté de la production (la
transcription par canal) a fait tomber co-atc de 6,4 à 1,8 fois le temps réel et **perdre 2
transmissions** (24/09 21:38). L'entraînement long se fera co-atc arrêté, ou à pertes acceptées.

### Étape 1 — banque de bruit : **faite**

`/Volumes/Crucial X8/whisper-corpus/banque-bruit/` : **5 267 ouvertures de squelch sans parole,
4 h 51 de bruit réel, 546 Mo**, un WAV 16 kHz chacune, décrites dans `manifeste.jsonl`. Empreinte
dans `resultats/entrainement/2026-09-24-empreinte-bruit.json` :

| Source | Canaux | Ouvertures sans parole | Minutes |
|---|---|---|---|
| 24/09, `gros-porteurs`, jour | 124,350 · 124,625 | 6 % · 9 % | 4 |
| | **125,825 · 126,425** | **61 % · 50 %** | **81** |
| 15/09, 132–133, nuit | les quatre du parasite (132,733 · 132,825 · 133,000 · 133,250) | **96 à 99 %** | 127 |
| | 132,275 · 132,500 · 132,783 | 73 % · 85 % · 8 % | 44 |
| 15/09, `orly-approche`, jour | cinq canaux | 6 à 51 % | 35 |

Durées médianes de 1 à 5 s. **Réserve** : la mesure du peigne à 100 Hz (harmoniques contre points
intermédiaires) reste négative partout, un peu moins sur les canaux du parasite (−1,7 à −2,9 dB
contre −3,8 le jour) ; elle est trop grossière pour confirmer le peigne que l'analyse spectrale du
doc 21 a vu. **Ce n'est pas elle qui trie la banque** : le tri est fait par le détecteur de voix.

**Étape suivante : la 2**, la chaîne de dégradation et sa validation, avec pour référence ce que
le modèle actuel écrit sur ce bruit (doc 28, section 4).

### Étape 2 — chaîne de dégradation : **validée**, avec deux écarts expliqués *(25/09)*

`whisper-lab/scripts/entrainement/degradation.py`, réglée sur l'empreinte de la station mesurée par canal
(`scripts/entrainement/empreinte-station.py`, enregistrement du 24/09) et jugée sur une **grille** : 120 clips de
test d'ATCOSIM à plusieurs niveaux de bruit, plus 120 ouvertures sans parole de la banque, transcrits
par le modèle anglais actuel (sidecar d'essai, co-atc arrêté à 10:12 à la demande du propriétaire).
Résultats : `resultats/entrainement/2026-09-25-grille-analyse.json`.

**La chaîne**, dans l'ordre du trajet : bruit réel de la banque sous la voix ; bande 300–2 700 Hz ;
niveau de la station ; **limiteur** à −3 dB (et compression 1,25 sur les canaux faibles) ; coupures
de squelch rejouées d'après de vrais schémas de la station ; 8 kHz, MP3 16 kbit/s, retour à 16 kHz.
Deux profils : **sain**, bruit injecté de +5 à +20 dB ; **faible**, de −5 à +5 dB.

**Trois corrections apportées par la mesure elle-même** :
- **pas d'écrêtage dans le son des canaux** (0 %) : celui de D42 est radio, l'étape est retirée ;
- **un limiteur est indispensable** : sans lui, 38 % des clips « sains » écrêtaient ; la station a ses
  crêtes à −3,4 à −5 dB, sa voix 8 à 12 dB au-dessus du niveau médian — reproduits (−2,9 à −5,2 dB) ;
- **la mesure « voix contre bruit » de la station sature** sous +5 dB injectés (elle rend 0 à 2 dB) :
  elle ne pouvait pas régler seule les canaux faibles. C'est la grille qui a fixé les niveaux.

**À la mesure** (même règle que la station) :

| | Niveau voix | Crêtes | Aigus 1,5–2,6 kHz | Voix/bruit, médiane |
|---|---|---|---|---|
| Station, 124,350 (sain) | −24,1 | −3,8 | 0,173 | 0,8 |
| Grille, sain +10 dB | −24,2 | −3,1 | **0,171** | 4,5 |
| Station, 125,825 · 126,425 (faibles) | −21,3 · −21,6 | −5,0 · −4,5 | **0,246 · 0,229** | 0,4 · 0,35 |
| Grille, faible 0 dB | −21,9 | −5,2 | **0,260** | 0,8 |

**Au comportement du modèle actuel** :

| Condition | Taux d'erreur | Chiffres de l'indicatif justes | Sorties vides |
|---|---|---|---|
| ATCOSIM propre | 12 % | 99 % | 0 % |
| sain +10 dB | 38 % | **86 %** | **0 %** |
| faible 0 dB | 79 % | **27 %** | **12 %** |
| faible 0 dB, hachage forcé | 88 % | 14 % | 33 % |

- **L'écart sain/faible est celui de la station** : 27 / 86 = **0,31**, contre **0,33** pour les
  avions justes par transmission à l'ADS-B (124,350 contre 125,825 et 126,425, doc 28).
- **Sorties vides sur la parole** : station 0,4 % (sain), 7,6 à 8,2 % (faibles) ; grille 0 % et 12 %.
- **Sur le bruit seul**, à règle égale (le détecteur ne trouve pas 0,25 s de parole dans le clip) : la
  grille reçoit du texte sur **37 %** des clips (3,2 mots), la station sur **34 à 44 %** (2,4 à 3,1 mots).
- Le spectre, l'écart sain/faible, les sorties vides et le comportement sur le bruit **se recoupent** :
  sain autour de +10 dB, faible autour de 0 dB.

**Deux écarts, qui ne viennent pas du canal** :
- **Les boucles** : 6 à 10 % sur la parole de la station, presque 0 sur la grille. ATCOSIM, ce sont
  des instructions isolées de 3 s ; la station, des échanges où le collationnement répète l'instruction,
  que la règle des boucles compte aussi. C'est **la façon de parler**, pas le bruit — prévu dans le plan.
- **Les insertions** face aux 15 transmissions anglaises annotées (71 %) : **14 des 15 références sont
  partielles**, ce que l'annotateur n'a pas entendu compte comme une insertion. Non comparable.
  La part de **substitutions**, elle, est proche : 26 % à la station, 24 % pour sain +10 dB.

**Trouvé pour l'étape 3 — à corriger avant d'étiqueter** : **59 % des clips de la banque de bruit
contiennent de la parole** pour le détecteur jugeant le clip seul (0,25 s ou plus), alors qu'ils
n'en contenaient pas jugés dans l'heure entière. Ce sont vraisemblablement des voix faibles. Étiquetés
« vides », ils apprendraient au modèle à ignorer de la vraie parole. **Seuls les clips sans parole au
jugement du clip seul (41 %) peuvent servir d'étiquette vide.**

**Décision de passage** : la chaîne reproduit le canal de la station sur tout ce qui a pu être mesuré ;
les deux écarts restants tiennent à la façon de parler et à des références partielles. **On passe à
l'étape 3.**

### Mini-essai 1 — ATCOSIM seul : **meilleur sur l'imitation, deux fois pire sur la station** *(25/09)*

Les poids d'origine du modèle anglais de co-atc (`jlvdoorn/whisper-large-v3-atco2-asr`, 3,2 Go,
téléchargés avec l'accord du propriétaire), affinés 500 pas (LoRA sur toute l'attention, codeur et
décodeur, 31,5 M paramètres, 49 min sur le Mac, co-atc arrêté) sur 1 100 clips ATCOSIM dégradés au
bruit du **15/09** et 120 clips de bruit seul étiquetés vides. Scripts : `whisper-lab/scripts/
entrainement/mini-essai-*.py` ; résultats : `resultats/entrainement/2026-09-25-mini-essai-*`.

**Contrôle d'abord** : les poids d'origine, reconvertis sans entraînement, rendent **la même
transcription que la production sur 97,5 % des 720 clips** de la grille. Les écarts qui suivent
viennent de l'entraînement, pas de la conversion.

| | Production | Mini-essai 1 |
|---|---|---|
| Grille (ATCOSIM de test) : indicatif lu juste, sain / faible 0 dB | 86 % / 27 % | **90 % / 31 %** |
| Grille : texte écrit sur du bruit seul | 37 % | **6 %** |
| **Station, 24/09, avions justes à l'ADS-B (4 canaux, parole)** | **236** | **104** |
| Station : précision, 125,825 | 67 % | 34 % |
| Station : texte écrit sur les ouvertures sans parole | 20 à 44 % | 7 à 10 % |

**Le modèle a pris le vocabulaire d'ATCOSIM.** Sur les 2 075 transmissions de la station :
*« rhein »* 355 fois (le secteur « Rhein Radar » d'ATCOSIM), *« is identified »* 205 fois,
*« lufthansa »* 487 fois contre 242, des compagnies disparues (*« sabena »*, *« alitalia »*) — et
*« air france »* 64 fois contre 194. Il n'invente presque plus sur le bruit, mais il entend
ATCOSIM partout, et les indicatifs de Paris se perdent.

**Leçons** :
- **l'imitation ne suffit pas pour juger** : son vocabulaire est celui d'ATCOSIM, elle flattait le
  modèle. Seule la station a démasqué le défaut — le point d'arrêt de l'étape 5 était indispensable ;
- **le bruit, lui, s'apprend** : moins d'invention sur le bruit seul, ici comme à la station ;
- **le défaut est dans la moitié qui rédige** (le décodeur), qui a appris les phrases d'ATCOSIM.

**Suite, lancée le 25/09 à 14:25** : **mini-essai 2**, un mélange de corpus (500 ATCOSIM dégradés,
500 UWB-ATCC et 100 ATCO2 tels quels, les mêmes 120 vides), choix du propriétaire ; puis **mini-essai
3**, le même mélange en n'adaptant que la moitié qui écoute (le codeur), remède direct au défaut
observé. Même entraînement, même jugement.
