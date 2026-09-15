# L'amorce change-t-elle la donne ? — cinq configurations sur tirage aléatoire

*15 septembre 2026, Mac mini M4 Pro. Chiffres bruts : `whisper-lab/q16-resultats.json`.*

## Pourquoi cette mesure

`08-premiere-mesure.md` jugeait `large-v3-turbo` **sans amorce**, alors que l'amont en
fournit une (`prompts/transcription_prompt.txt`) qui réclame explicitement la phraséologie
OACI et les chiffres en toutes lettres. La stratégie B n'avait donc pas été jugée à la
loyale, et l'échantillon était pris par ordre alphabétique — le piège que `04-corpus.md`
interdit. Ce test répare les deux.

**Échantillon** : 25 transmissions tirées **au hasard** dans 129,525 (Chavenay, français)
et 25 dans 132,500 (secteur en route, anglais supposé). Graine `20260915`, rejouable.

**Cinq configurations.** L'amorce de l'amont a été adaptée à chaque fréquence — terrain et
pistes réels de Chavenay, contexte « en route » pour 132,500 — puisque c'est ce que le
produit fait à l'exécution. La configuration E utilise la même amorce **traduite en
français sur la fréquence française**, pour tester l'hypothèse qu'une amorce anglaise tire
le modèle vers l'anglais.

## Le tableau

| Configuration | Fréq. | n | Calcul médian | × temps réel | mots/s d'audio | Langue = attendue | Boucles | Toponymes tchèques |
|---|---|---|---|---|---|---|---|---|
| **A** jacktol, sans amorce | 129525 | 25 | 1,15 s | 4,6× | 2,8 | — | 1 | 0 |
| | 132500 | 24 | **0,45 s** | **10,7×** | 2,1 | — | 0 | 2 |
| **B** jacktol, amorce EN | 129525 | 25 | 2,41 s | 3,0× | 1,5 | — | 0 | 1 |
| | 132500 | 25 | 0,57 s | 8,6× | 2,2 | — | 0 | 0 |
| **C** turbo, sans amorce | 129525 | 25 | 1,99 s | 2,7× | 2,1 | **84 %** | 1 | 0 |
| | 132500 | 25 | 3,53 s | 1,6× | 1,5 | **72 %** | 0 | 0 |
| **D** turbo, amorce EN | 129525 | 25 | 4,84 s | 1,3× | 5,2 | 84 % | **7** | 0 |
| | 132500 | 25 | 5,04 s | 0,9× | 4,5 | 72 % | 2 | 0 |
| **E** turbo, amorce dans la langue | 129525 | 25 | 1,99 s | 2,3× | 2,7 | 84 % | **1** | 0 |
| | 132500 | 25 | 4,98 s | 0,9× | 4,5 | 72 % | 2 | 0 |

*« Boucles » = transmissions où la répétition du 4-gramme le plus fréquent occupe plus de
la moitié du texte. C'est la dégénérescence classique de Whisper, et elle est objectivement
mauvaise : aucune vérité terrain n'est nécessaire pour la constater.*

*La colonne « langue » n'a pas de sens pour jacktol : c'est un modèle `.en`, il répond
toujours « en ».*

## Ce que ça dit

### 1. L'amorce de l'amont dégrade le résultat sur notre matériel

**Elle multiplie les boucles par sept** sur le français (1 → 7 sur 25) et double le temps
de calcul (1,99 s → 4,84 s). Le volume de texte produit passe de 414 à 1 013 mots pour la
même audio : c'est de la répétition, pas du contenu.

Pire, **elle fuit dans la sortie** :

```
D  « Aircraft are at cruise level and use ICA-7, Yankee Papa, confirm direction, reverse. »
D  « всю are at cruise level and use ICA to perform the same as possible. »
B  « <|en|> four three zero at lufthansa three one alfa good day »
```

Le modèle continue l'amorce au lieu de transcrire. Sur des transmissions de 2 à 7 secondes,
une amorce de 150 mots pèse plus que l'audio.

**Corollaire pour le fork** : reprendre tel quel le prompt de l'amont serait une erreur.
Il a été écrit pour l'API temps réel d'OpenAI sur un flux continu, pas pour des
transmissions isolées de quelques secondes passées à Whisper.

### 2. Mais une amorce **dans la langue de la fréquence** répare presque tout

Sur le français, la configuration E ramène les boucles de 7 à 1 et le temps de 4,84 s à
1,99 s — au niveau de l'absence d'amorce, avec 533 mots contre 414.

**C'est un usage inattendu de l'a priori par fréquence** : `03-transcription.md` le
proposait pour aiguiller entre deux modèles. Il sert aussi, et peut-être d'abord, à
**choisir la langue de l'amorce**. C'est gratuit — l'a priori est dans le catalogue.

### 3. Sur l'anglais, jacktol reste très supérieur — et l'écart n'est pas un artefact d'échantillonnage

Tirage aléatoire cette fois, même audio :

| | jacktol (A) | turbo (C) |
|---|---|---|
| | `right air berlin zero seven yankee papa confirm direct to rapet` | `Revisabria, 07 Yankee Papa, confirm direct to River.` |
| | `ups two nine zero` | `«Jepenst er brent, ja».` |
| | `departure on golf csa eight charlie whiskey flight level four seven zero on track abdo` | `Hello, Gophlin, Sierra Charlie Whiskey, flight number 4, Senator, on track Abno.` |
| | `three four three zero air berlin three one alfa good day` | `opharnentureth` |

### 4. La vitesse : le chapitre 8 était juste mais incomplet

L'avantage ×8,5 de jacktol **ne tient que sans amorce**. Avec, il tombe à 7,5× — et le
ralentissement est spectaculaire sur le français (4,6× → 3,0×). Sur turbo l'amorce coûte
encore plus cher : 1,9× → 1,2×.

La signature — beaucoup plus lent **et** sortie plus longue et répétitive — est celle du
**repli de température** de Whisper : quand le seuil de taux de compression ou de
log-probabilité n'est pas atteint, le segment est redécodé plusieurs fois.

Cela dit, **aucune configuration n'est en dessous du besoin** : sept canaux à 5 %
d'occupation demandent 0,35 × temps réel, et la plus lente ici tient 0,9×. La vitesse ne
départage toujours pas.

### 5. Deux familles d'erreur, et une seule est réparable

C'est le résultat le plus utile pour l'architecture.

| | jacktol | turbo |
|---|---|---|
| « air berlin » | **4** | 0 |
| « lufthansa » | **7** | 0 |
| « csa » | **3** | 0 |
| Noms de compagnie produits au total | 14 | 1 |

**Air Berlin a cessé toute activité en octobre 2017.** Ces quatre occurrences sont donc
nécessairement fausses. Quatre des sept « Lufthansa » sont sur **129,525 — la tour d'un
aéroclub des Yvelines**, où Lufthansa ne parle pas. Même famille que les toponymes
tchèques : le modèle rabat ce qu'il ne comprend pas sur le vocabulaire de son corpus
d'affinage.

Mais regardons la forme :

```
A  « lufthansa six lima yankee roger contact climb flight level three eight zero »
```

« six lima yankee » est un indicatif d'aviation légère française épelé lettre par lettre.
**Le modèle entend probablement les bonnes lettres et leur colle devant un nom de
compagnie qu'il connaît.**

> **Et c'est exactement l'erreur que l'architecture de Co-ATC répare déjà.** Son prompt de
> post-traitement dit : *« focus on the flight number, not airline code which may be hard
> to understand »*, et interdit tout indicatif absent de la liste ADS-B courante. Le nom de
> compagnie halluciné est **jeté** ; ce sont les chiffres et les lettres qui servent à
> l'appariement.
>
> Autrement dit : **les erreurs de jacktol sont structurées, donc contraignables. Celles de
> turbo — `opharnentureth`, `Jepenst er brent` — ne le sont pas.** Pour un produit qui
> croise la radio et l'ADS-B, c'est un argument de poids en faveur de A, et il ne se voyait
> pas avant d'avoir les deux sorties côte à côte.

### 6. La détection de langue échoue là où l'audio est mauvais

Turbo place la bonne langue sur 84 % du français et 72 % de l'anglais. Mais les échecs ne
sont pas des confusions français/anglais — ce sont des sorties vides de sens :

```
129525  [ru]  « Я mandателем лейтенка. »
129525  [ja]  « 再用 mobile »
129525  [en]  « Thank you. »            <- l'hallucination classique sur du quasi-silence
132500  [no]  « Jepenst er brent, ja »
132500  [es]  « ¿Querén ya va a anunciar? »
```

**La langue exotique est un symptôme, pas une cause** : ces transmissions n'auraient été
correctement transcrites par aucun modèle. Un aiguillage fondé sur la détection se
tromperait donc surtout sur des fichiers déjà perdus — ce qui rend la stratégie A moins
fragile qu'elle n'en avait l'air au chapitre 8, **à condition** de garder l'a priori par
fréquence comme valeur par défaut et de ne suivre la détection que lorsqu'elle est sûre.

## Ce que ça ne dit toujours pas

Aucun WER, aucun taux de reconnaissance d'indicatifs : **il n'y a toujours pas de vérité
terrain**. Tout ce qui précède repose sur des indicateurs calculables sans elle — vitesse,
boucles, langue détectée, anachronismes vérifiables. Ils suffisent à écarter l'amorce de
l'amont et à orienter l'architecture. **Ils ne suffisent pas à trancher Q1.**

Et la mesure ne porte que sur deux fréquences, dont aucune n'est un grand aéroport (Q17).

## Suite

1. Annoter les 120 transmissions (`09-jeu-de-test.md`).
2. Écrire une amorce **courte**, adaptée à des transmissions de quelques secondes, et la
   mesurer contre l'absence d'amorce. Celle de l'amont fait 150 mots ; le problème vient
   peut-être autant de sa longueur que de son contenu.
3. Mesurer l'appariement d'indicatifs contre la liste ADS-B réelle sur les sorties de
   jacktol — c'est l'hypothèse du point 5, et elle est testable.
