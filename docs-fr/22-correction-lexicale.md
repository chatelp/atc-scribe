# 22 — Corriger le texte sans IA : mesuré, et abandonné

*16 septembre 2026.*

Question posée : puisque GPT-4o réparait le texte avant de l'analyser — *« Tear Canada
123 »* → *« Air Canada 123 »* — un correcteur local sans modèle de langue pourrait-il
faire la même chose ? La phraséologie OACI est un vocabulaire fermé de quelques
centaines de mots ; corriger vers ce vocabulaire ressemble à un problème de
dictionnaire, pas de génération.

**Mesuré avant d'être écrit. Le résultat est négatif, et il est utile.**

## Le mécanisme envisagé

Un correcteur orthographique dont le dictionnaire ne contient que la phraséologie,
les chiffres, l'alphabet OTAN et les noms de compagnies. Pour chaque mot absent du
dictionnaire, chercher les mots connus à une lettre près ; s'il n'y en a qu'un,
remplacer. Jamais les chiffres — les corriger demanderait de connaître la réponse.

## Premier plafond : le dictionnaire fermé se retourne contre lui-même

Sur les 15 120 mots non numériques de la captation :

| | mots | part |
|---|---|---|
| déjà dans le vocabulaire | 10 888 | 72,0 % |
| à une lettre d'un mot connu | 1 069 | 7,1 % |
| à deux lettres | 2 040 | 13,5 % |
| hors de portée | 1 123 | 7,4 % |

Mais **63 % des mots « corrigeables » ont plusieurs candidats à égale distance**, et
les corrections dites sûres étaient majoritairement fausses :

```
that -> thai   x10      now -> no     x25      time -> tame  x18
call -> cal    x23      ils -> is     x28      run  -> un    x10
```

La cause est structurelle : **un dictionnaire fermé à la phraséologie déclare fautif
tout le langage ordinaire**, et un mot courant est très souvent à une lettre d'un
terme technique. `that` devient `thai` parce que Thai Airways est dans le lexique des
compagnies et que « that » n'est dans aucun lexique aéronautique.

## Deuxième version : un garde-fou de langue

Correctif évident : ne corriger que les mots qui **n'existent dans aucune langue**,
en s'appuyant sur `/usr/share/dict/words` (234 430 mots).

| | mots | part |
|---|---|---|
| déjà dans le vocabulaire | 10 888 | 72,0 % |
| mot de la langue, laissé tel quel | 2 722 | 18,0 % |
| à une lettre, **un seul** candidat | 185 | **1,2 %** |
| à une lettre, plusieurs candidats | 189 | 1,2 % |
| à deux lettres, un seul candidat | 357 | 2,4 % |

Les catastrophes disparaissent. Il reste **1,2 % de mots corrigeables sûrement**, dont
quelques vraies trouvailles — `tierra → sierra`, `tourne → tournez`, `kilos → kilo` —
et encore beaucoup de bruit, surtout en français : `ils → is` ×28, `onze → one` ×3,
parce que le lexique système est anglais.

## Le test qui décide : l'effet sur ce que la grammaire extrait

Le compte de mots ne prouve rien — un seul mot réparé peut débloquer une transmission
entière. Ce qui compte est le nombre de transmissions qui **livrent** quelque chose.
Correction appliquée aux 1 415 transmissions, tout le reste identique :

| | avant | après | écart |
|---|---|---|---|
| mots corrigés | — | 96 dans 86 transmissions (6,1 %) | |
| transmissions livrant une valeur | 1 079 | 1 080 | **+1** |
| indicatifs candidats | 1 405 | 1 401 | **−4** |

**Gain nul, et une légère perte.** La proposition est abandonnée.

## Pourquoi, et où est la vraie fuite

La grammaire s'ancre sur des **mots-clés courts et courants** — *level, runway,
heading, contact*. Whisper les écrit correctement, ou les massacre bien au-delà d'une
lettre : `cry` pour `climb` coûte trois lettres, `fight` pour `flight` une seule mais
`fight` est un mot anglais, donc protégé par le garde-fou. Le correcteur arrive
toujours trop tard ou trop tôt.

Et surtout : **les erreurs qui nous coûtent vraiment sont dans les chiffres et les
noms de compagnies** — or les chiffres sont intouchables par construction, et les noms
n'apportent qu'un bonus au score, jamais l'appariement lui-même.

> **Ce qu'il faut retenir pour la suite.** Les erreurs de transcription qui comptent
> sont exactement celles qu'un correcteur par dictionnaire ne doit pas toucher. Ce
> n'est pas une limite de l'implémentation, c'est une propriété du problème — et elle
> s'appliquera aussi à un modèle de langue local, qui devrait « corriger » des chiffres
> sans moyen de savoir lesquels.

## La piste que ce résultat désigne

Ce qui manque n'est pas la correction du texte, c'est **le contexte entre
transmissions**. GPT-4o lit vingt transmissions plus trois précédentes ; notre
grammaire analyse chaque transmission seule. Le cas mesuré ce matin le montre :

> *« hello bonjour Air France Three Six Seven … Air France Three Six Seven … France
> Three Six Seven »* → apparié à **AFR377**.

L'avion dit **trois fois** son numéro dans la même transmission, et rien n'exploite
cette répétition. Un lecteur de contexte — pas nécessairement un modèle de langue —
aurait vu la concordance et refusé l'appariement approximatif.

C'est là qu'est le gain, et c'est mesurable avec l'instrument existant.


---

# Le contexte entre transmissions : plafonds mesurés

Le résultat négatif ci-dessus désignait le contexte comme la vraie piste. **Mesuré
avant d'être écrit, comme le reste. Le plafond est beaucoup plus bas que je ne l'avais
laissé entendre.**

## Deux formes, deux plafonds

**A — la répétition à l'intérieur d'une transmission.** Le cas qui m'avait frappé,
*« Air France Three Six Seven »* dit trois fois : **15 transmissions sur 1 415, soit
1,1 %.** C'est une curiosité, pas un gisement.

**B — l'échange entre transmissions.** Une instruction puis son collationnement
partagent une valeur : *« descend FL100 »* / *« descending FL100 »*.

| fenêtre | transmissions sans indicatif | voisine avec indicatif **et** valeur commune | voisine avec indicatif seulement |
|---|---|---|---|
| 30 s | 668 | **0** | 347 — 52 % |
| 60 s | 668 | **0** | 484 — 73 % |
| 120 s | 668 | **1** | 586 — 88 % |

**Zéro.** La raison est mécanique : seules 4 % des transmissions portent un niveau, donc
deux voisines portant *le même* niveau n'arrivent pour ainsi dire jamais.

## La version qui rapporterait, et pourquoi je ne l'ai pas faite

Entre 52 et 88 % des transmissions non identifiées ont une voisine qui porte un
indicatif. Recopier cet indicatif attribuerait d'un coup la moitié du trafic.

**Mais cette version n'est pas mesurable avec l'instrument de ce chantier.** Le témoin
mélangé teste *« cet avion était-il dans le ciel ? »* — pas *« cette transmission
parlait-elle de lui ? »*. Une propagation par proximité hérite de la justesse de son
ancre : elle passe le témoin **même quand elle attribue au mauvais avion**. Le chiffre
publié monterait sans que rien ne dise s'il est vrai.

> C'est exactement le genre de gain que ce chantier refuse. La règle tient en une
> ligne, et elle est écrite dans le code : *le contexte ne sert qu'à choisir entre des
> avions que la transmission nomme déjà, jamais à en nommer un qu'elle ne nomme pas.*

## Ce qui a été implémenté, et ce que ça donne

Deux règles, toutes deux strictement conservatrices :

1. **Un groupe de chiffres répété** dans la transmission vaut +0,15.
2. **Une égalité entre deux candidats** est tranchée en faveur de celui que la
   fréquence vient d'adresser — les deux étaient déjà nommés par les chiffres, le
   contexte choisit seulement lequel.

Mesure sur la captation, historique tenu séparément pour le tirage réel et pour chacun
des huit témoins :

| contexte | appariements | hasard | au-dessus | vrais |
|---|---|---|---|---|
| aucun | 144 | 40,8 | 72 % | 103,2 |
| 60 s | 144 | 40,8 | 72 % | 103,2 |
| 120 s | 144 | 40,8 | 72 % | 103,2 |
| 300 s | 144 | 40,8 | 72 % | 103,2 |

**Effet strictement nul.** Trois tests unitaires prouvent que les mécanismes
fonctionnent quand les conditions existent — une égalité tranchée, une répétition
valorisée, et le refus d'inventer un appariement à partir du seul contexte. Ce n'est
donc pas un code mort : **c'est un corpus qui n'offre aucune occasion.**

Le code reste en place et actif en production. Le trafic d'une captation de nuit sur
cinq fréquences n'est pas celui d'une approche chargée, et les égalités y sont rares.
Si elles deviennent fréquentes ailleurs, la règle sera là — et la mesure se refait en
une commande.

## Ce que ces deux résultats négatifs disent ensemble

La correction lexicale et le contexte étaient les deux pistes « évidentes » pour
rattraper GPT-4o. Mesurées, elles rapportent respectivement **+1 transmission** et
**zéro**.

La contrainte est ailleurs, et elle est simple : **668 transmissions sur 1 415 — 47 % —
ne contiennent aucun indicatif à trois chiffres.** Ni la correction, ni le contexte, ni
un modèle de langue ne créent une information que la transmission ne porte pas. Ce qui
reste à gagner est dans la qualité de la transcription elle-même, et cela se mesure avec
un taux d'erreur de mots — donc avec les annotations.
