# L'appariement des indicatifs — la première mesure de justesse du projet

*15 septembre 2026. Code : `internal/transcription/phraseology/matcher.go`.
Mesure : `go run ./cmd/phraseology -db data/co-atc-<date>.db`.*

## Pourquoi cette mesure vaut plus que toutes les précédentes

Depuis le début, l'absence de vérité terrain bloque toute mesure de justesse : les
indicateurs employés — accord entre modèles, lexique aéronautique, taux de boucles — sont
des **indices**, pas des taux d'erreur.

Co-ATC en fabrique une, gratuitement et en continu. **Si la transcription prononce un
numéro de vol et qu'un avion portant ce numéro est réellement dans le ciel à cette
seconde-là, ce n'est pas une coïncidence.** L'ADS-B fournit la vérité, la radio fournit la
prédiction, et personne n'a besoin d'annoter quoi que ce soit.

C'est aussi, exactement, ce que le prompt de l'amont demande à GPT-4o : *« focus on the
flight number, not airline code which may be hard to understand »*, et interdiction
d'inventer un indicatif absent de la liste.

## Comment l'appariement est construit

Tout repose sur un fait mesuré : **les modèles entendent bien les chiffres et mal les noms
de compagnie**. Ils produisent « Air Berlin » et « Germanwings » — disparues en 2017 et
2020 — parce que leurs corpus d'affinage datent. Donc :

- **les chiffres décident.** Correspondance exacte 0,90 ; suffixe 0,60 ; un chiffre
  d'écart 0,50.
- **le nom d'une compagnie ne peut que monter un score, jamais le baisser** (+0,40). Un
  opérateur halluciné ne coûte donc rien.
- **les groupes de lettres OACI** (+0,30) rattrapent les indicatifs à suffixe littéral —
  `EZY36VJ` énoncé « three six victor juliett ».
- **l'ambiguïté est signalée, pas tranchée** : deux avions à moins de 0,15 l'un de l'autre
  ressortent tous les deux.

Les indicatifs radio viennent d'`assets/airlines.dat`, déjà livré par l'amont : il porte
`SPEEDBIRD` pour BAW et `EASY` pour EZY à côté des noms de compagnie. Rien n'est codé en
dur.

## Le témoin, sans lequel le chiffre ne vaut rien

Avec une centaine d'avions dans la fenêtre, **une part des appariements tombe juste par
arithmétique**. Il faut donc mesurer ce que donne le hasard, et deux tentatives ont échoué
avant la bonne :

- **décaler la fenêtre d'une heure** : la plupart des transmissions sortent de la période
  couverte, l'échantillon tombe de 384 à 60. Sans valeur.
- **tirer un instant au hasard dans la période** : l'enregistrement a un trou de 3 h 30,
  deux tirages sur trois tombent dedans. Sans valeur non plus.
- **tirer l'instant parmi les observations ADS-B elles-mêmes** : mêmes 232 candidats,
  mêmes tailles de flotte, et la seule chose qui change est que ces avions-là ne peuvent
  pas être ceux qui parlaient. **C'est le bon témoin.**

## Le résultat

384 transmissions du 15/09, 294 520 observations ADS-B, fenêtre ±60 s, flotte médiane de
**106 avions** :

| | |
|---|---|
| Transmissions avec couverture ADS-B | **384** |
| Portant un candidat d'indicatif | **232** (60 %) |
| **Rattachées à un avion réellement présent** | **47** |
| Attendu par hasard (5 tirages témoins) | **11,4 ± 3,3** |
| **Précision estimée** | **76 %** |
| Significativité | **z = 10,8** |

**Trois chiffres, c'est le seuil.** Le levier n'est ni la fenêtre temporelle ni le score,
c'est la longueur du groupe :

| Longueur minimale | Appariés | Hasard | Précision | Vrais |
|---|---|---|---|---|
| 2 chiffres | 73 | 32,6 | 55 % | 40,4 |
| **3 chiffres** | **47** | **11,4** | **76 %** | **35,6** |
| 4 chiffres | 14 | 4,0 | 71 % | 10,0 |

À deux chiffres, presque un appariement sur deux est de l'arithmétique. À quatre, on perd
l'essentiel du signal — beaucoup de numéros de vol font trois chiffres. **`MinDigits = 3`
est la valeur par défaut, et elle est mesurée, pas choisie.**

La fenêtre temporelle, elle, ne change presque rien : le nombre d'appariements vrais reste
entre 35 et 39 de ±30 s à ±300 s. Les avions sont vus en continu, donc rétrécir la fenêtre
ne rétrécit pas la flotte.

## À quoi ça ressemble

```
16:08:51  ICE546   0,90  chiffres exacts    « to the right cleared to land i say five four six »
16:09:09  MEA229   0,90  chiffres exacts    « speed one five zero knots jet two two nine »
16:12:55  DAH1214  0,90  chiffres exacts    « left heading three one zero to intercept ils two six left »
16:15:34  TGZ628   0,90  chiffres exacts    « contact departure tomah six two eight go »
16:20:49  BAW308   0,90  un chiffre + opérateur nommé  « one eight zero knots speed bird three two eight »
```

## Une correction, et son coût

Un appariement montrait le défaut de la règle : **« one nine zero ryan air one seven
four » rattaché à AFR174.** L'opérateur nommé est Ryanair, l'avion retenu est une Air
France — parce que le nom ne pouvait que monter un score, jamais restreindre.

Correction retenue : **si l'opérateur nommé est effectivement dans le ciel, seuls ses
avions sont candidats.** Ce n'est pas une contradiction de la règle initiale — un
opérateur *absent* du ciel, les Air Berlin hallucinées, ne contraint rien et les chiffres
décident seuls.

**Mais la mesure ne valide pas ce changement** : 47 → 40 appariés, 35,6 → 30,2 vrais, et
la précision reste à 76 %. On perd cinq appariements vrais pour retirer 1,6 appariement de
hasard. Mauvais marché sur les comptes.

Il est conservé quand même, et le raisonnement doit être explicite : **le témoin par
brassage ne sait pas voir cette erreur-là.** Il mélange les flottes, donc un appariement
fait au bon instant sur la mauvaise compagnie y compte comme « vrai ». Attribuer une
transmission Ryanair à Air France est une faute que l'utilisateur verrait et que la
métrique ne voit pas. **Quand la mesure est aveugle à un type d'erreur, elle ne peut pas
arbitrer sur lui.**

## Ce que ça vaut, et ce qu'il manque

**76 % de précision, 20 % de rappel.** C'est un vrai résultat — le premier taux de justesse
du projet — et c'est insuffisant pour alimenter directement l'extraction de clairances :
une fausse clairance d'atterrissage sur un avion réel est une faute grave, et une sur
quatre le serait.

## Le contexte de vol — ajouté, et mesuré

**C'est la seule corroboration où les deux côtés sont mesurés plutôt qu'entendus** : une
altitude prononcée et une altitude barométrique. Quand elles s'accordent à moins de
1 500 pieds, +0,40 ; quand elles divergent de plus de 8 000 pieds, **−0,30**. C'est le seul
endroit où une règle *retire* du score — un nom de compagnie ne le fait jamais, parce qu'il
peut être halluciné ; une altitude ADS-B, non.

S'y ajoutent deux règles de bon sens : une clairance d'atterrissage ne va pas à un avion en
croisière, une clairance de décollage ne va pas à un avion en l'air.

**Et une leçon de méthode.** Le premier jet comparait un niveau de vol prononcé à
l'altitude ADS-B — et ne s'est **jamais déclenché**. Diagnostic : sur 409 transmissions de
ce corpus d'approche, **8 seulement portent un niveau de vol**. Les contrôleurs d'approche
disent « four thousand feet », pas « flight level four zero ». La grammaire ne savait pas
lire les pieds : il lui manquait les **mots-clés qui suivent le nombre** au lieu de le
précéder — `feet`, `knots`, `degrees`.

Résultat après correction :

| | Appariés | Hasard | Précision | Vrais | z |
|---|---|---|---|---|---|
| Sans contexte de vol | 40 | 9,8 | 76 % | 30,2 | 10,7 |
| **Avec contexte de vol** | **45** | **9,5** | **79 %** | **35,5** | **11,8** |

Curiosité qui mérite d'être dite : **un seul appariement porte la mention « altitude
agrees »**. Le gain ne vient donc pas du bonus mais de la **pénalité**, qui écarte les
avions concurrents dont l'altitude contredit ce qui est dit, et laisse le bon remonter. La
règle travaille surtout en éliminant, pas en confirmant.

Les leviers restants :

1. **La continuité.** Un indicatif entendu se répète sur plusieurs transmissions
   consécutives ; un appariement isolé est plus douteux qu'un appariement répété.
3. **La qualité de la transcription elle-même**, qui reste le plancher de tout le reste.

> ⚠️ **Une limite de méthode à garder en tête.** Ces 384 transmissions viennent du flux
> **mélangé** des cinq canaux d'`orly-approche`, transcrites avec un modèle anglais alors
> que 123,875 est bilingue. Le taux mesuré porte donc sur un mélange, pas sur une
> fréquence. La captation par canal permettra de refaire la mesure proprement.
