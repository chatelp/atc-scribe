# Le jeu d'évaluation — 120 transmissions, et comment l'annoter

*Constitué le 15 septembre 2026. Tirage reproductible, graine `20260915`.*

`04-corpus.md` pose la règle : **rien ne se décide sur Q1 avant d'avoir une vérité
terrain.** Ce document décrit le jeu tiré, l'outil d'annotation, et les pièges de
méthode à tenir.

## Le tirage

Script : `whisper-lab/jeu-de-test.py`. **Tirage aléatoire** — `random.Random(20260915)` —
et non par ordre alphabétique ni par taille de fichier. `04-corpus.md` rappelle qu'une
évaluation faite sur les quatre plus gros fichiers d'une fréquence a déjà produit une
fausse conclusion publiée : les gros fichiers sont les atypiques.

| Fréquence | Tirés | Disponibles | Langue attendue | Service |
|---|---|---|---|---|
| 129,525 | 43 | 765 | **fr** | Chavenay Tour (LFPX), aéroclub |
| 128,950 | 17 | **17** | **fr** | Villacoublay Tour (LFPV), militaire |
| 132,500 | 30 | 545 | **en** | secteur en route, non identifié |
| 127,750 | 30 | 543 | **en** | Orly Départs |
| | **120** | | 60 fr / 60 en | |

> ⚠️ **Une réserve sur 127,750.** `04-corpus.md` la range du côté anglais, mais
> `06-catalogue.csv` la donne **mixte** (Orly Départs, bilingue). Ses 30 transmissions ne
> peuvent donc pas servir de vérité de langue ; elles serviront de **jeu d'épreuve pour
> l'aiguillage**, qui est justement le point faible de la stratégie A. La vérité de langue
> repose sur les 60 françaises et les 30 de 132,500.

> ⚠️ **128,950 est pris en entier** : 17 fichiers, c'est tout ce que la station a. Ce
> n'est plus un échantillon, c'est la population. À ne pas traiter comme une mesure
> généralisable — c'est de la tour militaire de Villacoublay, en VFR français.

Le jeu vit dans `whisper-lab/jeu-de-test/` avec son `manifeste.json`. **Il reste hors du
dépôt** : ce sont des enregistrements de communications réelles, et leur rediffusion est
une question distincte (voir plus bas).

## L'outil d'annotation

`tools/annotate/` — un serveur Python de la bibliothèque standard, sans dépendance, et
une page unique.

```bash
python3 tools/annotate/server.py \
  --corpus ~/Dev/Aero/whisper-lab/jeu-de-test \
  --out    ~/Dev/Aero/whisper-lab/verite-terrain.json
```

Puis <http://127.0.0.1:8777/>. Lecture automatique, <kbd>espace</kbd> rejoue,
<kbd>⌘↵</kbd> enregistre et passe au suivant. Enregistrement à chaque frappe, écriture
atomique, reprise là où on s'est arrêté. Il n'écoute que sur la boucle locale.

Chaque transmission reçoit : le texte entendu, la **langue réellement parlée**
(fr / en / les deux / autre), et deux drapeaux — « aucune parole » et « partiellement
inintelligible ».

## Les trois règles de méthode

**1. Aucune sortie de modèle n'est montrée à l'annotateur.** L'outil ne pré-remplit rien.
Pré-remplir avec la meilleure transcription automatique ferait gagner du temps et
ruinerait la mesure : on corrigerait à la marge ce que le modèle propose, et l'on
adopterait ses erreurs plausibles sans les voir. C'est exactement le risque démontré au
chapitre de `08-premiere-mesure.md` — un modèle qui écrit « lufthansa one romeo romeo »
sur une fréquence d'aéroclub produit une phrase qu'on relit sans sourciller.

**2. On écrit ce qui est dit, pas ce qui serait correct.** Hésitations, erreurs de
phraséologie, collationnements partiels compris. Les nombres comme ils sont prononcés :
« flight level three five zero », pas « FL350 ». C'est le post-traitement qui normalise,
et on veut pouvoir mesurer s'il le fait bien.

**3. `[unclear]` plutôt qu'une hypothèse.** Un annotateur qui devine fabrique de la
fausse vérité terrain, et pénalise ensuite un modèle qui, lui, avait raison.

## Un défaut du corpus, mesuré au passage

**1 fichier sur 170 est illisible par ffmpeg.** Sur les 170 transmissions téléchargées le
15/09 (jeu de test + échantillon pilote), une a fait échouer le décodage :
`132500/t_20260914_225745.mp3`, `Invalid data found when processing input`.

Ce n'est pas un téléchargement tronqué — les 8 593 octets correspondent à ce que sert la
station. Le fichier commence par la balise `LAME3.100` suivie de remplissage `0xAA`, **sans
la trame MPEG qui devrait l'envelopper** ; la première synchro de trame n'arrive qu'à
l'octet 19. RTLSDR-Airband a écrit un en-tête malformé à la fermeture de ce squelch-là.

La réparation est triviale : **jeter les octets qui précèdent la première synchro**
(`0xFF` suivi de trois bits à 1). Le fichier rend alors 4,50 s d'audio parfaitement
lisible.

```python
d = open(path, 'rb').read()
i = next(k for k in range(len(d)-1) if d[k] == 0xFF and (d[k+1] & 0xE0) == 0xE0)
open(path, 'wb').write(d[i:])
```

Appliqué au jeu de test : 0 fichier illisible restant. **À prévoir dans toute chaîne qui
consomme ces enregistrements** — 0,6 % de perte silencieuse, c'est peu, mais une
transcription qui échoue sans qu'on regarde pourquoi est une mesure faussée.

Sans objet pour la production : la chaîne en direct consomme un flux continu, pas des
fichiers par transmission.

## Ce qu'on mesurera dessus

Dans cet ordre de priorité, d'après `04-corpus.md` :

1. **Taux de reconnaissance des indicatifs et des chiffres** — niveaux, caps, QNH, pistes.
   C'est le critère du produit. Un WER flatteur qui rate tous les indicatifs ne vaut rien.
2. **WER, séparément par langue.** Moyenné sur les deux, il ne veut rien dire ici.
3. **Fiabilité de l'aiguillage** : part des transmissions où la langue détectée contredit
   la langue réellement parlée, et ce que l'a priori par fréquence y change.
4. **Hallucinations de toponymes** (Q14) — indicateur propre à ce produit, bien moins
   coûteux à produire qu'un WER.

## Une question à trancher avant publication

Le dépôt est destiné à être public. **Le jeu de test, lui, ne peut pas l'être sans y
réfléchir** : ce sont des enregistrements de communications radio réelles, avec des
indicatifs identifiables. Ce qui est publiable, c'est probablement la **liste des noms de
fichiers, la graine du tirage et les annotations** — de quoi rejouer la mesure pour qui
possède la station, sans rediffuser l'audio. À arbitrer (Q16).

## Écrêtage dans le jeu : négligeable, et c'est une bonne nouvelle

Vérifié le 22/09, avant d'engager les heures d'annotation. D43 a montré qu'environ 8 % des
transmissions de la nuit du 21 sont **franchement écrêtées** et que ça change la
transcription dans près de deux cas sur trois. Il fallait donc savoir si le jeu
d'évaluation en portait.

| | clips |
|---|---|
| Franchement écrêtés (≥ 100 éch. en butée) | **1** |
| Légèrement (1 à 99) | 4 |
| Intacts | **115** |

**1 % contre 8 % dans le corpus de nuit.** Les 120 annotations mesureront donc bien les
modèles et non la chaîne d'enregistrement — c'est ce qu'on voulait.

> ⚠️ **La contrepartie, à ne pas oublier au moment de conclure** : le jeu
> **sous-représente l'écrêtage**. Le taux d'erreur qu'on en tirera sera donc *optimiste*
> par rapport au trafic réel de nuit, où une transmission sur douze est abîmée avant même
> d'arriver au modèle. Le jeu mesure les modèles ; il ne mesure pas la chaîne.

## Le calcul est prêt et attend les annotations

`whisper-lab/note-modeles.py`, 18 tests. **Utilisable dès le premier clip** : il n'y a pas
de seuil en dessous duquel on ne rapporte rien, seulement une incertitude à afficher.

Trois précautions qui changent le résultat :

- **normalisation** — la consigne demande « flight level three five zero » quand les
  modèles rendent « FL350 » ; comparer sans normaliser mesurerait la typographie ;
- **les `[unclear]`** — une référence dont l'annotateur n'a pas compris un passage ne peut
  pas servir de référence sur ce passage ; score rapporté avec et sans, l'écart est une
  donnée ;
- **le partage aveugle / assisté** (D39) — un clip où l'annotateur a regardé les lectures
  est contaminé ; les deux lots sont notés séparément et **l'écart mesure l'ancrage**.

Le contrôle qui valide la chaîne entière : si l'on donne comme référence la sortie d'un
modèle, ce modèle marque exactement 0 %.

## L'ordre d'annotation : entrelacé, pour que tout arrêt soit exploitable

Le tirage rangeait les clips **par fréquence** — 43 Chavenay, 17 Villacoublay, 30 en route,
30 Orly. Constaté le 22/09 après les six premières annotations : le propriétaire faisait
les 43 plus difficiles d'affilée et n'aurait atteint l'anglais qu'au clip 61. **Un arrêt à
40 clips n'aurait donné aucun anglais et aucun chiffre exploitable.**

Réordonné le 22/09, graine 20260922 : mélange à l'intérieur de chaque fréquence, puis
**entrelacement proportionnel**. Pas un mélange simple — celui-ci ne garantit la
représentativité qu'en espérance, l'entrelacement la garantit à **chaque** position :

| | 127750 | 128950 | 129525 | 132500 |
|---|---|---|---|---|
| cible | 25 % | 14 % | 36 % | 25 % |
| après 20 clips | 25 % | 15 % | 35 % | 25 % |
| après 30 | 23 % | 13 % | 37 % | 27 % |
| après 60 | 25 % | 13 % | 37 % | 25 % |

**La composition du jeu n'a pas bougé**, seulement l'ordre de présentation ; l'ordre
d'origine est conservé dans `manifeste-ordre-origine.json`. Les annotations sont repérées
par identifiant de clip et non par position : rien n'a été perdu.

> **Conséquence utile** : 30 clips suffisent désormais à un premier résultat honnête, au
> lieu d'exiger d'aller jusqu'à 61 pour voir la première transmission anglaise.

## Aérodromes masqués, depuis le 23/09 (D48)

Le français visé est celui des avions de ligne en approche et au départ, pas des
aérodromes. L'outil est donc lancé en masquant Chavenay et Villacoublay :

```bash
python3 tools/annotate/server.py --corpus whisper-lab/jeu-de-test \
  --out whisper-lab/verite-terrain.json --exclude-freq 129525,128950
```

Masqué pour l'annotateur seulement : le manifeste et le fichier d'annotations sont
intacts, et le calcul des scores voit toujours les clips déjà faits. Restent présentés
Orly Départs (bilingue, au cœur de la cible) et l'en-route (anglais) — 60 clips, 24 faits
au moment du masquage.
