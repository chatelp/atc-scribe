# Le corpus déjà disponible

## Ce qu'il y a

`/opt/adsb/public/transmissions/` sur la station, servi aussi en HTTP sur
`http://macmini-fedora.lan/fichiers/transmissions/`.

**4 642 enregistrements MP3 de transmissions réelles**, un fichier par transmission,
rangés par fréquence, **déjà triés** : 13 050 fichiers sans parole ont été effacés le
15 septembre à l'aide du détecteur Silero. Ce qui reste contient de la voix.

| Dossier | Fichiers | Ce que c'est | Langue attendue |
|---|---|---|---|
| `127750` | 543 | Orly Départs | bilingue |
| `129525` | 765 | Chavenay Tour, aéroclub | **français** |
| `133250` | 574 | non identifiée, amas 132-133 | ? |
| `132500` | 545 | non identifiée, secteur en route | **anglais** (« Qatari, flight level 380 ») |
| `132825` | 484 | non identifiée | ? |
| `132275` | 454 | non identifiée | ? |
| `132783` | 369 | non identifiée | ? |
| `127850` | 259 | Reims Contrôle KN | bilingue |
| `132733` | 255 | non identifiée | ? |
| `133000` | 218 | non identifiée | ? |
| `128150` | 140 | contrôle de ligne | bilingue |
| `128950` | 17 | Villacoublay Tour, base militaire | **français**, VFR |

Le nom de fichier porte l'horodatage : `t_AAAAMMJJ_HHMMSS.mp3`. Il est écrit à la
**fermeture** du squelch, pas à l'ouverture — la date de modification du fichier est
donc la fin de la transmission.

## Pourquoi c'est précieux

C'est un **jeu d'évaluation prêt à l'emploi et représentatif** : même antenne, même
récepteur, même chaîne de démodulation, mêmes conditions de propagation que ce que
atc-scribe devra transcrire en production. Aucun corpus public d'ATC ne peut en dire
autant pour cette station.

Trois dossiers ont une langue **connue** et servent de vérité partielle :

- `129525` et `128950` → français, VFR et tour d'aérodrome
- `132500` → anglais, secteur en route, phraséologie OACI stricte

## Ce qui manque

**Il n'est pas annoté.** Personne n'a écrit ce qui est dit dans ces 4 642 fichiers.
Pour mesurer un taux d'erreur de mots il faut une vérité terrain.

Chemin le plus court : **annoter à la main 100 à 200 transmissions**, équilibrées entre
français et anglais, en tirant au hasard dans les dossiers de langue connue. Deux heures
de travail pour un jeu de test qui servira tout le projet. Le faire **avant** de choisir
une stratégie de modèle, pas après.

> ⚠️ **Toujours échantillonner au hasard, jamais par les extrêmes.** Une évaluation
> faite sur les quatre plus gros fichiers d'une fréquence a conclu « rien que du vide »
> alors qu'un tirage aléatoire de 40 fichiers donnait une médiane toute différente : les
> gros fichiers sont justement les atypiques, de longs silences avec un clic. L'erreur a
> coûté une fausse conclusion publiée.

## Protocole d'évaluation suggéré

1. Tirer au hasard 60 transmissions dans `129525` + `128950` (français) et 60 dans
   `132500` + `127750` (anglais). Les transcrire à la main. C'est la vérité terrain.
2. Passer les mêmes fichiers dans les trois configurations : `jacktol` seul,
   `large-v3-turbo` seul, et l'aiguillage par langue. Mesurer le **WER** séparément sur
   les deux langues.
3. Mesurer aussi ce qui compte vraiment pour le produit : **le taux de reconnaissance
   des indicatifs et des chiffres** (niveaux de vol, caps, QNH, pistes). Un WER moyen
   flatteur qui rate tous les indicatifs ne vaut rien pour de l'ATC.
4. Mesurer la **vitesse** sur le M4, en temps réel relatif. La station peut produire
   plusieurs transmissions simultanées sur des fréquences différentes : la chaîne doit
   tenir le débit d'un groupe de 7 canaux à l'heure de pointe.
5. Consigner les résultats dans `docs/` avec les chiffres bruts. C'est ce tableau qui
   décidera, pas une impression.

## Enrichir le corpus

Le groupe `paris5-avec-enregistrement` et le groupe `amas-132-133` produisent des
fichiers par transmission. Pour constituer un corpus français riche, le meilleur choix
est **Chavenay (129,525)** : c'est un aéroclub, ça parle français en continu, et les
transmissions y sont longues — médiane double des fréquences de contrôle, avec des
pointes à 44 secondes.

> ⚠️ Mais ce mode **dégrade le flux en direct** (voir `01-station.md`). Il ne se lance
> pas sans prévenir le propriétaire, et jamais pendant qu'il écoute.

## Outils déjà écrits, à réutiliser

Sur la station, `/home/pierre/nuit/` :

- **`purge2.py`** — tri par détection de voix Silero, avec sa calibration
- `depouille.py` — activité par fréquence et par heure
- `salves.py` — structure des salves, **l'outil qui valide toute modification audio**

`/home/pierre/transcription/` : un venv **faster-whisper** fonctionnel sur Python 3.14
(`/usr/bin/python3 -m venv`, surtout pas le `python3` de Linuxbrew qui n'a pas numpy),
avec `transcris.py` et les résultats déjà obtenus dans `resultats.txt`.

La méthode complète de tri et de qualification est documentée dans le projet Claude
« Radio » du propriétaire, document `methode-tri-enregistrements.md`. **Demande-la lui
si tu en as besoin**, elle contient les calibrations et les pièges.
