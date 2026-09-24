# Plan — affiner la reconnaissance par augmentation de données

*Note de projet du 24 septembre 2026. L'idée est celle du propriétaire (Q43) ; ce document
dit ce qu'on ferait, dans quel ordre, et à quoi on saurait que ça marche. **Rien n'est
commencé.***

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
  (`whisper-lab/corpus-externes/`), licences lues et notées.
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
- Livrable : `resultats/empreinte-station.json`.

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
