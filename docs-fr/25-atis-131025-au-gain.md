# 25 — L'ATIS de Saint-Cyr (131,025) au gain

*Session du 16 septembre 2026, 22h15–22h45, sur `macmini-fedora`. Demandée par l'agent
d'atc-scribe. Accord du propriétaire donné pour le changement de gain, sur ce seul canal.
La chaîne ADS-B n'a pas été approchée. Gabarit d'origine restauré, `accord: true` vérifié.*

## Ce qui a été fait

Un gabarit temporaire `atis-lfpz` — 131,025 seule, centre à 130,550 (décalé de 475 kHz
comme `fixe136`, pour que le pic DC du tuner ne tombe pas sur le canal), `squelch_snr_threshold = 3`,
`highpass = 300`, `lowpass = 2700`. Cinq points, chacun : bascule, 120 s de stabilisation,
185 s de capture. Personne à l'écoute au départ (1 auditeur Icecast = `filtre-voix` seul,
donc 0 externe).

**Deux écarts au protocole demandé**, tous deux assumés :

1. **Capture depuis le point de montage Icecast, pas par une sortie fichier.** Un seul canal
   sur le gabarit, donc `aero.mp3` ne porte que 131,025 : même audio, même encodeur, sans
   toucher à la machinerie d'enregistrement — et c'est exactement ce que la chaîne de
   transcription consomme. `ampfactor` s'appliquant avant le mélangeur, le cinquième point
   reste valide.
2. **Pas de `salves.py` sur ces fichiers.** Une porteuse permanente n'a pas de salves :
   l'indicateur mesurerait le fichier entier. La mesure qui répond à la question est
   `astats` / `volumedetect`.

## Les cinq points

| Point | Gain | Début capture | Durée (trames) | Moyenne | Crête | Éch. à pleine échelle |
|---|---|---|---|---|---|---|
| **g40** (témoin) | 40 → 40,2 dB | 22:18:02 | 212,47 s | **−15,4 dBFS** | **−0,8 dBFS** | 1 |
| **g28** | 28 → 28,0 dB | 22:23:44 | 213,84 s | **−16,5 dBFS** | **−1,2 dBFS** | 1 |
| **g16** | 16 → 16,6 dB | 22:29:26 | 249,91 s | −91,0 dB | −91,0 dB | *silence numérique* |
| **g08** | 8 → 8,7 dB | 22:35:08 | 249,91 s | −91,0 dB | −91,0 dB | *silence numérique* |
| **g40-amp05** | 40 + `ampfactor = 0.5` | 22:40:50 | 215,21 s | **−21,5 dBFS** | **−6,6 dBFS** | 1 |

Fichiers servis sous `http://macmini-fedora.lan/fichiers/nuit/atis-131025-*.mp3`.

**Échelonnement retenu et pourquoi.** Le R820T n'accepte pas des décibels continus mais
29 valeurs tabulées. Pas d'environ −12 dB sur 32 dB de plage, plutôt qu'un affinage fin :
131,025 arrive à +49 dB au-dessus du plancher, on peut donc descendre très bas sans perdre
le signal, et si la saturation est un écrêtage elle a un seuil — une échelle grossière dit
de quel côté on est en une session, l'affinage ne se justifie que si deux points encadrent
la transition.

## Quatre résultats

**1. Le gain du tuner ne commande pas le niveau audio.** −12 dB de gain RF (40 → 28) déplacent
la moyenne de **1,1 dB** et la crête de 0,4 dB. RTLSDR-Airband normalise l'audio démodulé en
AM : le gain RF n'est pas un réglage de niveau, et l'utiliser comme tel ne donne presque rien.

**2. `ampfactor` est le seul levier qui obéisse.** 0,5 donne −6,1 dB de moyenne et −5,8 dB de
crête, contre −6,02 dB attendus. À la décimale près.

**3. Il n'y a aucun écrêtage en sortie d'encodeur, à aucun gain.** Crête à −0,8 dBFS avec
**un seul échantillon** à ce niveau. Ce que le propriétaire entend comme une saturation
n'est donc pas un écrêtage de sortie : c'est soit une surcharge d'entrée (invisible en aval
à cause de la normalisation), soit une propriété de l'émission elle-même.

**4. En dessous de 28, le canal ne produit plus rien** — pas plus faible, *rien* : moyenne et
crête à −91,0 dB, soit le plancher numérique, sur la totalité des 1 999 296 échantillons.

## Ce que la session ne permet pas de conclure, et c'est l'essentiel

**Elle ne dit rien de l'intelligibilité.** Je ne peux pas écouter ces fichiers. Aucun de ces
nombres ne dit si la voix est lisible. Les deux fichiers exploitables ne diffèrent que de
1 dB : si le gain RF change quelque chose à l'intelligibilité, ce sera par la distorsion et
pas par le niveau — et seule la transcription le montrera. **C'est à vous de trancher, sur
`g40` et `g28`.**

**Et le résultat 4 est confondu.** J'ai laissé `squelch_snr_threshold = 3` fixe sur toute
l'échelle. À bas gain, le bruit de quantification monte en proportion et le rapport calculé
passe sous le seuil : le silence de `g16` et `g08` est donc peut-être un squelch qui ne
s'ouvre plus, pas un signal disparu. **« En dessous de 28 le canal meurt » vaut pour cette
configuration-là, pas pour le tuner.** Un vrai point bas demanderait un seuil de squelch
abaissé en même temps que le gain. C'est un défaut de mon plan, pas une découverte.

## Ce que je recommande comme suite

Transcrire `g40` et `g28`. Deux issues :

- **Elles se valent, et sont toutes deux mauvaises.** Alors le gain RF n'est pas le levier,
  et la piste suivante n'est pas un gain plus bas mais **les filtres** : le gabarit est à
  300–2700 Hz, et un ATIS surmodulé ou un artefact hors bande se traite là, pas au tuner.
  `ampfactor = 0.25` mérite aussi un essai, puisque c'est le seul réglage qui réponde.
- **`g28` est meilleure.** Alors il y a bien un seuil de surcharge entre les deux, et il faut
  refaire une échelle fine entre 28 et 40 — avec le squelch abaissé, cette fois.

## Une découverte incidente, qui déborde cette demande

**Le socle de diffusion d'Icecast se compte en octets, pas en secondes.** Les captures ont
duré 190 s chacune, mais les fichiers font 212 à 250 s. L'écart est le socle envoyé à la
connexion (`burst-size`, 65 536 octets par défaut), dont la **durée varie à l'inverse du
débit** : ≈ 22 s à 20 kbit/s, ≈ 60 s sur les fichiers silencieux tombés à 8 kbit/s.

Deux conséquences. Les premières dizaines de secondes de toute capture par `curl` sont de
l'audio **antérieur au début de la capture** — ici sans effet, mais cela fausserait toute
mesure avant/après calée sur l'heure. Et cela chiffre l'intérêt du point 8 du document
d'exploitation, descendre `burst-size` à 8192.

## Note de méthode : les durées

Elles viennent du comptage de trames (`compte-trames.py`, déposé sur la station), qui marche
les longueurs de trame une par une — MPEG-2.5, 8 kHz, 576 échantillons, 72 ms — et ignore la
première, le flux commençant en cours de trame. **Contrôle croisé : les cinq durées tombent
exactement sur le compte d'échantillons décodés par ffmpeg** (212,47 s → 1 699 776 échantillons
à 8 kHz, etc.), sur les cinq fichiers. `ffprobe` n'a été cru sur aucune.

Les tailles de fichier, elles, sont trompeuses : 524 288 et 249 856 octets sont des frontières
de tampon de `curl`, pas des durées.

## Tâche annexe : `salves.py`

Faite. Le script ne meurt plus quand `ffprobe` ne renvoie pas de champ `Duration` : il imprime
la raison, ignore ce point de mesure et continue. Testé sur un fichier inexistant, un fichier
vide et trois fichiers réels — médianes inchangées (3,2 à 3,7 s). Sauvegarde dans
`salves.py.avant-durcissement-20260916-215834`.

## Sur les relevés de référence à 01 h et 02 h que vous avez écartés

D'accord, et pour une deuxième raison. La vôtre est juste. J'ajoute que le contrôle qui, lui,
mérite d'être fait — `paris5-avec-enregistrement` contre lui-même, là où le défaut de
fragmentation a été observé — doit se faire **de jour**, quand il y a de la parole à mesurer.
Il n'a donc pas plus sa place dans une session nocturne. Deux raisons distinctes, même
conclusion.

## État de la station après la session

Gabarit temporaire archivé en `modes/archive/atis-lfpz-20260916-224434.tmpl`. La station est
passée sur **`fixe136`** pour la nuit (décision du propriétaire) : `en-route-et-descente-cdg`
est un mauvais régime nocturne, quatre de ses sept canaux étant ceux que le parasite secteur
noie — 88,1 % d'ouvertures de squelch sans parole. `accord: true` vérifié.
