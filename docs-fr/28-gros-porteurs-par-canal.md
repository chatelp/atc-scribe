# Les quatre canaux de `gros-porteurs`, enregistrés séparément — 24/09

*Mesure du 24 septembre 2026. Enregistrement demandé à la station le matin (Q37), avec
l'accord du propriétaire : les quatre canaux du groupe `gros-porteurs` écrits chacun dans son
fichier, en continu, **avant le mélangeur**, de **10:47:31 à 14:00:00**, plus les compteurs de
squelch de RTLSDR-Airband toutes les 15 s (`stats_filepath`). Rien d'autre n'a changé à la
station ; `aero.mp3` a été coupé deux fois quelques secondes, au début et à la fin.*

Données : `whisper-lab/audio/2026-09-24-gros-porteurs-par-canal/` (16 fichiers, 63 Mo, index
dans `CORPUS.md`). Analyse : `whisper-lab/scripts/par-canal-analyse.py`, résultat dans
`resultats/2026-09-24-par-canal.json`.

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
- **D2** (flux par canal) : un argument de plus. Deux canaux sur quatre fournissent l'essentiel du
  bruit et du hachage, et le mélange les répand sur les quatre.

## Limites

- Un seul après-midi, un seul groupe de fréquences.
- Le détecteur de voix est le juge de « parole » ; son verdict sur des voix très faibles est
  précisément ce qui est en question (écoute ci-dessus).
- Le détecteur à 8 kHz sous-estimait fortement la parole (2,9 s contre 15,7 s à 16 kHz sur les
  mêmes dix minutes) : les chiffres ci-dessus sont ceux à 16 kHz.
