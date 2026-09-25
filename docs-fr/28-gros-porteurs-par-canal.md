# Les quatre canaux de `gros-porteurs`, enregistrés séparément — 24/09

*Mesure du 24 septembre 2026. Enregistrement demandé à la station le matin (Q37), avec
l'accord du propriétaire : les quatre canaux du groupe `gros-porteurs` écrits chacun dans son
fichier, en continu, **avant le mélangeur**, de **10:47:31 à 14:00:00**, plus les compteurs de
squelch de RTLSDR-Airband toutes les 15 s (`stats_filepath`). Rien d'autre n'a changé à la
station ; `aero.mp3` a été coupé deux fois quelques secondes, au début et à la fin.*

Données : `whisper-lab/audio/2026-09-24-gros-porteurs-par-canal/` (16 fichiers, 63 Mo, index
dans `CORPUS.md`). Analyse : `whisper-lab/scripts/station/par-canal-analyse.py`, résultat dans
`resultats/station/2026-09-24-par-canal.json`.

## En bref

**Deux canaux sont sains, deux ne le sont pas, et ce sont les mêmes pour les deux défauts.**

| Canal | Ouvert | Parole dans le temps ouvert | Ouvertures sans parole | Battements (station) | Coupures en grappes (audio) |
|---|---|---|---|---|---|
| 124,350 | 45 % | **81 %** | 8 % | **0** | 10 |
| 124,625 | 6 % | **79 %** | 15 % | 64 | 11 |
| **125,825** | 52 % | **31 %** | **64 %** | **1 950** | **237** |
| **126,425** | 39 % | **38 %** | **58 %** | **8 919** | **367** |

Sur 3 h 13. « Parole » selon le détecteur de voix du sidecar (Silero, 16 kHz, mêmes réglages).
125,825 et 126,425 sont aussi les deux canaux que le propriétaire avait déjà relevés dans le
mélange (`ampfactor` 1,4 et 1,3 contre 0,8 et 0,7) : **ce sont les canaux reçus faiblement.**

## 1. Le hachage : où et quand

**Méthode, et un piège évité.** Une coupure courte (silence numérique de 30 à 350 ms entre deux
passages ouverts) n'est pas forcément un hachage : sur une fréquence chargée, c'est souvent **le
pilote qui répond 200 ms après le contrôleur**. 124,350 en compte 506, toutes isolées sauf 10, et
la station n'y compte **aucun** battement. Seules les **coupures en grappes** — au moins deux en
une seconde — signent un squelch qui bat. Elles suivent les compteurs de la station minute par
minute (corrélation 0,34 sur 125,825, 0,52 sur 126,425) ; les coupures isolées, non (0,06 et 0,10).

**Le battement vient par épisodes, répartis sur toute la période** : 20 minutes de fort battement
sur 125,825, 23 sur 126,425 (au moins 5 coupures en grappes dans la minute), de 10:47 à 13:59.
**Ce n'est pas un effet du redémarrage** (première hypothèse de 10:53, écartée à 11:03).

## 2. Une cause commune ? Un lien avec 124,350, sans pouvoir dire lequel

- **Les épisodes des deux canaux faibles coïncident peu** : 5 minutes où les deux battent
  ensemble, pour environ 2,4 attendues par hasard. Corrélation minute par minute : 0,15.
- **Au moment des grappes de 125,825, 124,350 est ouvert 69 % du temps**, contre 51 % quand
  125,825 est ouvert sans battre ; 124,625, 14 % contre 7 %. Pour 126,425, les écarts sont faibles
  (124,350 : 54 % contre 51 % ; 125,825 : 73 % contre 65 %).
- **Deux lectures possibles, que ces données ne départagent pas** : un signal fort sur 124,350 qui
  sature la réception et fait passer sous le seuil les signaux faibles des autres canaux
  (l'écrêtage est mesuré, D42) ; ou une simple coïncidence de trafic, les secteurs voisins étant
  chargés aux mêmes moments. Prochaine mesure : le niveau radio de 124,350 aux instants des
  grappes, dans les compteurs de la station.
- **Le niveau audio ne distingue pas** les transmissions hachées des autres (−21,7 dBFS de
  médiane pour les 56 transmissions de 125,825 qui battent, −21,5 pour les 980 autres) : il ne
  mesure pas la force du signal radio.

Ce qui reste compatible avec tout : **des signaux faibles, proches du seuil du squelch**, sur les
deux canaux que la station reçoit mal.

## 3. Le bruit : ouvertures sans parole

Sur 125,825 et 126,425, **plus de la moitié des ouvertures ne contiennent pas de parole** pour le
détecteur (64 % et 58 % ; entre 48 % et 64 % selon qu'on juge l'ouverture seule ou dans son
contexte) — **37 et 33 minutes** d'ouverture sans parole en 3 h 13, contre 6 sur 124,350. Ce
n'est **pas** le parasite secteur de la nuit : aucun peigne à 100 Hz dans leur spectre.

Leur durée ressemble à celle d'une transmission (médiane 3,0 s et 2,7 s, 90 % sous 9 s). Hypothèse,
**non vérifiée** : ce sont en partie de vraies transmissions, trop faibles pour que le détecteur y
reconnaisse une voix — des avions lointains sur ces secteurs. **À trancher à l'oreille** : dix
ouvertures tirées au hasard par canal, `ecoute/sans-parole-125825.mp3` et
`ecoute/sans-parole-126425.mp3`.

## Ce que ça change

- **Q37** : le hachage n'est pas un réglage général ni un problème de réseau ; il est concentré sur
  deux canaux, par épisodes. Un essai de seuil de squelch porterait sur **ces deux-là seulement**,
  et se jugerait sur les compteurs de la station, désormais disponibles.
- **Q41–Q42** : le bruit envoyé au modèle vient surtout de ces deux canaux. Par canal, on saurait
  lesquels filtrer ; dans le mélange, on ne peut pas.
- **Q43** : l'empreinte de la station est mesurée par canal ; les deux profils — sain et faible —
  sont à reproduire tous les deux.
- **D2** (flux par canal) : **l'argument le plus fort mesuré à ce jour** — quatre fois plus
  d'avions justes canal par canal que dans le mélange (section 4). Et deux canaux sur quatre
  fournissent l'essentiel du bruit et du hachage, que le mélange répand sur les quatre.

## Limites

- Un seul après-midi, un seul groupe de fréquences.
- Le détecteur de voix est le juge de « parole » ; son verdict sur des voix très faibles est
  précisément ce qui est en question (écoute ci-dessus).
- Le détecteur à 8 kHz sous-estimait fortement la parole (2,9 s contre 15,7 s à 16 kHz sur les
  mêmes dix minutes) : les chiffres ci-dessus sont ceux à 16 kHz.

## 4. Transcrit canal par canal : quatre fois plus d'avions justes que le mélange

*Mesuré la nuit du 24 au 25/09* (`whisper-lab/scripts/station/par-canal-transcrire.py` puis
`par-canal-apparier.py`, résultats `resultats/station/2026-09-24-apparier-*.txt`).

**Protocole.** Les quatre fichiers par canal (10:47:31 → 14:00:00) découpés **exactement comme
le segmenteur de co-atc** (fin sur 600 ms de silence, 400 ms au moins, 30 s au plus, 200 ms
avant), soit 2 415 morceaux, envoyés à un sidecar d'essai — même code, même modèle anglais,
sans seconde lecture française. Face à eux, **ce que co-atc a tiré du mélange aux mêmes
heures** (606 transcriptions de la base du 24). Même ADS-B, même outil (`cmd/phraseology`),
même contrôle par ciel mélangé (5 tirages), refus des ambigus, 3 chiffres.

| | Transmissions | Associées | Dont par hasard | Précision | **Vrais** |
|---|---|---|---|---|---|
| 124,350 | 691 | 173 | 34,8 | 80 % | **138,2** |
| 124,625 | 191 | 23 | 7,6 | 67 % | **15,4** |
| 125,825 | 643 | 74 | 29,2 | 61 % | **44,8** |
| 126,425 | 577 | 56 | 19,4 | 65 % | **36,6** |
| **Les quatre canaux** | 2 102 | 326 | 91,0 | 72 % | **235,0** |
| **Le mélange de co-atc** | 606 | 88 | 28,4 | 68 % | **59,6** |
| Mélange, fenêtre de 120 s | 606 | 94 | 32,0 | 66 % | 62,0 |

**Environ quatre fois plus d'avions justes (235 contre 60), à précision égale ou meilleure.**
La fenêtre élargie ne rattrape presque rien : le retard avec lequel co-atc date une transmission
n'explique pas l'écart. La seconde lecture française, présente dans le mélange et absente ici,
joue **en faveur du mélange**. Ce que le découpage par canal change : plus de morceaux de 30 s
où plusieurs fréquences parlent en même temps (Q41), 3,5 fois plus de transmissions distinctes.

**Le hachage ne coûte rien de mesurable à l'association** : sur les transmissions qui
contiennent des coupures en grappes, 125,825 associe 2,4 vrais sur 32 (7,5 %) contre 43,6 sur
558 (7,8 %) pour les autres ; 126,425, 6,8 sur 42 contre 37,2 sur 468. Mais **74 transmissions
hachées seulement** : un petit échantillon, qui ne dit rien de ce que le hachage coûte à l'oreille.

**Ce que le modèle écrit sur du bruit** — la référence que demande l'étape 2 du plan 27. Sur les
morceaux où le détecteur trouve moins de 0,25 s de parole, que le sidecar d'essai a transcrits
quand même : **44 % reçoivent du texte sur 125,825** (3,1 mots en moyenne), **34 % sur 126,425**
(2,4 mots). Parfois un indicatif entier inventé : *« lufthansa five six three five six »*,
*« hello jetstar six two one five four six »*. Les boucles y sont rares (1 à 3 %) ; sur la
parole, 6 à 10 %, comme dans le mélange (Q41).

**Limites** : un après-midi, un groupe de fréquences ; le modèle anglais seul ; les morceaux sont
datés au début, le mélange à la fin de la transcription (d'où le contrôle à 120 s).
