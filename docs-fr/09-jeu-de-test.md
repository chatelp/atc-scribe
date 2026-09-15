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
