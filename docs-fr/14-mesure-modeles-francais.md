# Les Whisper spécialisés français — mesure

*15 septembre 2026. 25 transmissions de Chavenay (129,525) tirées au hasard, plus
30 d'Orly Départs. Chiffres bruts : `whisper-lab/q22-resultats.json`.*

## Ce qui est mesuré

La stratégie A′ (`13-strategie-bilingue.md`) remplace la branche « Whisper multilingue
générique » par un **Whisper spécialisé français**. Trois candidats, tous convertis en MLX
depuis HuggingFace (aucun n'est distribué en MLX) :

| | Modèle | Base |
|---|---|---|
| **FR1** | `bofenghuang/whisper-large-v3-french` | large-v3, 32 couches |
| **FR2** | `pierreguillou/whisper-medium-french` | medium — **auteur différent** |
| **FR3** | `bofenghuang/whisper-large-v3-french-distil-dec16` | distillé, 16 couches décodeur |

`aihpi/FrWhisper` a échoué à la conversion : ses poids sont fragmentés en deux fichiers et
le `convert.py` de `mlx-examples` n'en lit qu'un seul.

**L'indicateur de plausibilité.** Faute de vérité terrain, on compte la proportion de mots
appartenant à un **lexique strictement aéronautique** — `piste`, `autorisé`, `décollage`,
`rappelez`, `finale`, `verticale`, `transpondeur`, `QNH`, `collationnez`… Les mots français
ordinaires en sont exclus : un premier jet incluait `vous`, `sur`, `dans`, ce qui gonflait
le score de tout modèle parlant français.

> ⚠️ **Cet indicateur récompense le fait de produire des mots d'aviation, pas de les
> produire au bon endroit.** Un modèle qui écrirait « autorisé décollage » partout aurait
> un excellent score. C'est un indice de plausibilité, pas une mesure de justesse.

## Le tableau

| Modèle | Calcul médian | × temps réel | Lexique aéro | Clips avec ≥1 terme | Boucles |
|---|---|---|---|---|---|
| **FR1 `bofenghuang` large-v3** | 1,21 s | 6,0× | **3,9 %** | **9 / 25** | **0** |
| FR3 `bofenghuang` distil-dec16 | 0,77 s | 7,8× | 1,9 % | 7 / 25 | 0 |
| FR2 `pierreguillou` medium | **0,63 s** | **8,5×** | 1,5 % | 6 / 25 | 0 |
| FR1 sans forçage de langue | 1,71 s | 3,9× | 3,9 % | 9 / 25 | 0 |

Et pour comparaison, sur **exactement les mêmes 25 fichiers** :

| Modèle déjà testé | Lexique aéro | Clips avec ≥1 terme |
|---|---|---|
| `large-v3-turbo` générique | 2,2 % | 6 / 25 |
| `large-v3-atco2` ATC multilingue | 1,8 % | 5 / 25 |
| `jacktol medium.en` ATC anglais | **0,0 %** | **0 / 25** |

**Les termes réellement produits**, qui comptent plus que le taux :

```
FR1  autorisé ×5, rappel ×4, finale ×3, décollage, piste, transpondeur, rappelez
ATC  rappel ×6, atterrissage ×3, décollage, fréquence
EN   (aucun)
```

## Ce qui est acquis

**1. FR1 est le meilleur français mesuré, et de loin.** Deux fois le lexique aéronautique
du meilleur des trois modèles précédents, et dix fois celui du modèle ATC anglais.

**2. Aucune boucle de dégénérescence, sur aucun des trois modèles français.** C'est un
progrès net : `large-v3-turbo` bouclait, et le modèle ATC avec amorce bouclait sept fois
sur vingt-cinq.

**3. La vitesse n'est pas un sujet.** 6 à 8,5 × le temps réel. Sept canaux à 5 %
d'occupation demandent 0,35×.

**4. Forcer `language="fr"` ne change pas une virgule du texte mais fait gagner un tiers
du temps** (1,21 s contre 1,71 s) en sautant la passe de détection. Sur une fréquence dont
le catalogue connaît la langue, c'est gratuit — et ça conforte le principe 1 de la
stratégie A′.

**5. Une transcription qui tient debout**, pour montrer que le signal existe :

```
20260912_113612
  FR1         : « Fox Alpha Charlie, 28 au tourisme d'atterrissage, vent 282, 5 kt.
                  On atterrit, piste 28, Fox Alpha Charlie. »
  ATC multi   : « Foxtrot Alpha Charlie seven eight also reached atterrissage... »
  ATC anglais : « lufthansa six lima yankee roger contact climb flight level three eight zero »
```

Indicatif, piste 28 — qui existe à Chavenay —, vent plausible, et le collationnement du
pilote reprenant l'indicatif en fin de message. C'est la structure exacte d'un échange de
tour. Le modèle anglais, lui, invente une Lufthansa au niveau 380 au-dessus d'un aéroclub.

## Ce qui n'est pas acquis, et c'est l'essentiel

**Deux modèles français d'auteurs différents ne convergent pas.**

| Paire | F1 médian | Lecture |
|---|---|---|
| FR1 vs FR3 | **0,36** | même auteur, FR3 distillé de la même famille — **mesure une parenté, pas une vérité** |
| **FR1 vs FR2** | **0,15** | **auteurs différents, corpus différents — la seule paire indépendante** |
| FR2 vs FR3 | 0,15 | idem |

**0,15, c'est le même ordre que tout ce qu'on a mesuré sur le français** depuis le début.
Un mot commun sur sept. Deux modèles qui divergent à ce point ne peuvent pas avoir raison
tous les deux.

Cette lecture oblige d'ailleurs à **corriger une conclusion antérieure** : l'accord de 0,37
entre les deux modèles ATC sur l'en route anglophone (`12-concatenation.md`) est lui aussi
gonflé par la parenté — ils sont affinés tous les deux sur ATCO2. Toutes les paires
réellement indépendantes mesurées à ce jour tombent entre **0,15 et 0,20**, quelle que soit
la langue.

Et **16 clips sur 25 ne contiennent aucun terme aéronautique** chez FR1. La structure est
là, les mots ne suivent pas : « Le fusil d'extrait en vent arrière droite », « raté à
8 minutes du débutant en athlète ».

## La conclusion, qui est constructive

**La stratégie A′ est bonne dans son architecture et insuffisante dans sa branche
française.** Un Whisper spécialisé français est nettement meilleur que tout ce qu'on avait,
mais il ne connaît pas la phraséologie : il écrit du français, pas de l'ATC français.

**Cela désigne la suite avec précision.** La stratégie C — affiner soi-même — ne part plus
de `large-v3` générique mais de **`bofenghuang/whisper-large-v3-french` comme modèle de
base**. Il parle déjà bien français ; il lui manque le vocabulaire du métier. C'est un
point de départ beaucoup plus proche de la cible que tout ce qui était envisagé dans
`03-transcription.md`, et ça réduit d'autant le corpus d'affinage nécessaire.

## Suite

1. Annoter les 120 transmissions. **Aucune de ces conclusions n'est une mesure de
   justesse ; toutes reposent sur des indices.**
2. Mesurer FR1 sur les fréquences bilingues avec aiguillage, et non avec forçage — sur
   Orly Départs, forcer le français produit « 35 à 45 manteuils ».
3. Chiffrer l'affinage de FR1 sur de l'ATC français : combien d'heures annotées pour
   quel gain.
