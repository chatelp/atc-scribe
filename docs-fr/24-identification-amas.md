# 24 — L'amas 132-133 identifié canal par canal

*16 septembre 2026, après-midi. Passe de transcription par canal sur la captation
nocturne du 15-16, avec la chaîne de production exacte.*

Le mode `amas-132-133` a été construit pour identifier sept fréquences que le balayage
avait vues sans savoir ce qu'elles portaient. `21-nuit-132-133.md` avait établi leur
régime — quatre noyées sous le peigne à 100 Hz, trois porteuses — et conclu de trente
segments tirés au sort que **l'ensemble était de l'espace supérieur, FL160 à FL390**.

**Cette conclusion est vraie en moyenne et fausse canal par canal.** Voici pourquoi, et
ce que les sept canaux sont réellement.

## Pourquoi il fallait transcrire par canal

Le direct passe par `aero.mp3`, qui mélange les sept canaux : chaque transmission y perd
sa fréquence d'origine, et les 1 321 transmissions de la journée portent toutes
`frequency_id = "aero-melange"`. On savait que « 133,255 » avait été citée six fois dans
l'amas, **pas par lequel de ses canaux**.

La captation nocturne est le seul corpus par canal du dossier. Elle ne contient que
33,4 minutes de parole au total : une petite passe suffisait.

**462 transcriptions**, chaîne de production à l'identique — même découpage (seuil 0,005,
600 ms de silence, 400 ms minimum), même VAD (Silero 0,5 / 250 / 300), même modèle
(`large-v3-atco2`, langue `en`). Neuf minutes de calcul.

| canal | segments avec parole | transcrits | français |
|---|---|---|---|
| **132,275** | 294 | **285** | **10 %** |
| **132,500** | 92 | 91 | 2 % |
| 132,783 | 35 | 35 | 3 % |
| 133,250 | 32 | 29 | 0 % |
| 132,825 | 9 | 9 | 0 % |
| 132,733 | 8 | 7 | 43 % *(3 sur 7)* |
| 133,000 | 6 | 6 | 0 % |

**132,275 porte à elle seule 62 % de la parole de l'amas.** Un échantillon tiré au
hasard dans le tout est donc, aux deux tiers, un échantillon de ce seul canal — et c'est
exactement ce qui a produit la moyenne trompeuse de la nuit.

## 132,275 — Paris Contrôle, secteur d'arrivée de De Gaulle

**Ce n'est pas de l'espace supérieur.** Médiane **FL200**, avec FL130 sept fois, FL120
trois fois, FL190, FL220, FL230 — un étagement de descente, pas de croisière.

**Elle passe ses avions à l'approche de De Gaulle, quatorze fois en quatre heures :**

```
Lot Seven November Alpha redirected two five zero knots contact the goal
    one two one one five five good bye
Foetix Five One Six Five we have fixed the goal one two one decimal one five five
    good night
Air France Four Cent Quatre One Two Victor contact hele de Gaulle runway one three
    fifty five au revoir
```

> **« the goal » est « de Gaulle ».** Le modèle l'écrit ainsi treize fois sur quatorze —
> et la quatorzième, il l'écrit correctement, sur la même fréquence. La corroboration
> est interne au corpus, pas une supposition de lecture.

`06-catalogue.csv` porte **121,150 — « Approche De Gaulle + Le Bourget »**, dont le
désignateur 8,33 est **121,155**. C'est exactement la fréquence citée.

**Et un pilote la nomme.** Une transmission sur 285, mais la forme ne laisse pas de
place au doute — c'est un appel initial, et un appel initial nomme l'organisme qu'on
appelle :

```
hello Paris good evening Surgic Four One Yankees climbing flight level two nine zero
    cleared flight level two three zero
```

**132,275 est un secteur de Paris Contrôle.** Trois preuves indépendantes qui
convergent : le profil de niveaux (descente), la destination des transferts (approche
CDG, ×14), et l'organisme nommé à l'appel.

Elle est aussi **le seul canal francophone de l'amas** — 10 %, contre 0 à 3 % partout
ailleurs. Surtout des « bonjour » d'appel, mais aussi du vrai français de contrôle :
*« descendu niveau cent soixante »*, *« contact RN sur quitrente au revoir »*.

### Ses autres voisins

| cité | fois | ce que c'est |
|---|---|---|
| 121,155 | 14 | **approche De Gaulle + Le Bourget** (catalogue) |
| 125,965 | 9 | **Brest** — nommé dans le message : *« contact breast one two five decimal nine six five »* |
| 135,9xx | 1 | Brest — *« contact the break one three five decimal nine »* |
| **132,785** | 1 | **un des sept canaux de l'amas** (132,783) |
| 131,1 · 131,229 · 131,452 · 128,430 | 1 ch. | hors catalogue |

## 132,825 — Brest Contrôle

Neuf transcriptions seulement — c'est l'un des quatre canaux noyés sous le peigne à
100 Hz. Mais l'une des neuf est un appel initial, et il nomme l'organisme :

```
breast control hello Arabia Maroc Two Two Eight Alpha ... flight life three nine zero
```

**« breast control » est « Brest Control ».** Un seul cas : c'est une identification
**probable**, pas établie. Elle est cohérente avec le reste — FL390 entendu, aucun
français, et 132,275 qui envoie des avions vers Brest sur 125,965.

## 132,500 — croisière haute, organisme inconnu

Médiane **FL370** : FL380 six fois, FL370 quatre fois, FL360 trois fois. De la croisière
franche, à l'opposé de 132,275.

Cela recoupe `16-grammaire-et-125933.md`, qui avait mesuré **125,933 citant 132,505 trois
fois sur quarante transmissions** — 125,933 portant elle-même FL370 à FL430. Deux
secteurs de croisière adjacents, l'un passant à l'autre. Le lien était mesuré d'un côté
en septembre ; la passe par canal le confirme de l'autre par le profil de niveaux.

L'organisme n'est pas nommé : une seule mention, ambiguë (*« break can you get us a spot
wind at level three six zero »*).

## Les quatre canaux noyés

| canal | transcriptions | niveaux entendus | organisme |
|---|---|---|---|
| 133,250 | 29 | FL340 ×2, FL320, FL270 | — |
| 132,783 | 35 | FL330, FL340 | — |
| 132,733 | 7 | FL390, FL370 | — |
| 133,000 | 6 | FL340 | — |

Tous sont en **croisière haute**, ce qui confirme qu'ils sont de vraies fréquences de
contrôle en route — la nuit du 15 l'avait déjà établi sur deux d'entre eux par un
échantillon. Mais six à trente-cinq transmissions ne suffisent pas à les identifier :
le peigne à 100 Hz les prive de la matière nécessaire.

## Ce que l'amas est, au total

**Ce n'est ni un organisme, ni un étage.** C'est une tranche de spectre de 975 kHz qui
contient au moins :

- **un secteur d'arrivée de Paris Contrôle** (132,275), qui descend les avions vers
  FL120–FL200 et les livre à l'approche de De Gaulle ;
- **au moins un secteur de Brest** (132,825, probable) ;
- **cinq secteurs de croisière haute** (132,500 · 132,733 · 132,783 · 133,000 ·
  133,250), FL270 à FL390, dont l'un est relié à 125,933 par une mesure faite des deux
  côtés.

Le nom de secteur de chacun reste inconnu, et le restera sans une source officielle —
`16-grammaire-et-125933.md` le disait déjà : *« ce que ça ne dit pas : le nom du secteur
et l'organisme »*. Ce qu'on a maintenant, c'est leur **fonction**, leur **étage**, leurs
**voisins**, et pour deux d'entre eux **l'organisme nommé par un pilote**.

## Ce que ça apprend sur la méthode

**Le flux mixé ne fait pas que perdre l'attribution : il fabrique une moyenne
trompeuse.** La conclusion de la nuit — « l'amas est de l'espace supérieur » — venait de
trente segments tirés au hasard dans le mélange. Mais 62 % de la parole vient d'un seul
canal, et ce canal-là est le seul qui ne soit pas en croisière. Tirer au hasard dans un
mélange déséquilibré, c'est échantillonner le canal dominant en croyant échantillonner
l'ensemble.

C'est un argument de plus pour Q4 / D2, le mode de diffusion par canal — et il est
différent de celui du doc 23 (R1), qui portait sur la qualité de transcription. Ici, ce
n'est pas la transcription qui souffre du mélange, c'est **l'interprétation**.

> **Le corpus reste disponible** : `whisper-lab/nuit-132-133/par-canal.json`,
> 462 transcriptions avec canal, heure, offset et durée. C'est le premier corpus du
> dossier qui porte l'attribution par fréquence sur du texte transcrit.
