# La grammaire de phraséologie, et ce qu'elle a identifié

*15 septembre 2026. Code : `internal/transcription/phraseology/`, outil de mesure
`cmd/phraseology`. Données : `whisper-lab/q24-125933.json`.*

## Pourquoi une grammaire plutôt qu'un modèle de langue

`07-carte-openai.md` a montré que le post-traitement de l'amont demande cinq choses à
GPT-4o, et qu'**une seule exige vraiment un modèle de langue** : la correction libre du
texte. Les quatre autres — normaliser les nombres, décider qui parle, rattacher un
indicatif, extraire une clairance — portent sur un vocabulaire fermé.

Et les mesures disent comment s'y prendre. Les modèles de transcription **entendent
correctement les chiffres et très mal les noms de compagnie** : ils produisent « Air
Berlin » et « Germanwings », deux compagnies disparues, parce que leurs corpus d'affinage
datent. La grammaire est donc construite autour des chiffres, chacun recevant **le rôle
que lui donne le mot-clé qui le précède**.

Elle est volontairement **ancrée sur des mots-clés plutôt que grammaticale** : sur une
sortie bruitée, s'accrocher à « flight level » ou « runway » survit au charabia qui
l'entoure, là où une analyse de phrase complète échouerait sur la phrase entière.

## Ce qu'elle extrait

Niveau de vol, altitude, cap, vitesse, fréquence, piste, QNH, transpondeur, groupes de
lettres OACI (les indicatifs d'aviation légère), le locuteur (contrôle ou pilote), et les
clairances de décollage, d'atterrissage et d'approche — dans l'ensemble fermé que le
prompt de l'amont énumère, clairances conditionnelles exclues.

**Les tests utilisent de vraies sorties de modèle sur l'audio de cette station**, pas des
phrases inventées. Tester sur des exemples fabriqués ne prouverait rien : ce qu'il faut,
c'est survivre à ce que les modèles produisent réellement.

Trois défauts ont été trouvés par ces tests-là, et corrigés :

**1. Les homophones.** `« flight level three eight zero to tepac »` donnait FL3802 : `to`
est un homophone de `two`. Un mot ambigu ne compte comme chiffre que si un autre chiffre
le suit.

**2. Les fréquences nues.** Sur 40 transmissions de 125,933, `« one three two five zero
five »` était compté **trois fois comme un indicatif d'avion**. La bande aéro va de
118,000 à 136,975 MHz : un groupe de cinq ou six chiffres commençant dans cette plage est
un transfert de fréquence, pas un numéro de vol.

**3. Les niveaux impossibles.** `« level to six points »` donnait FL26. Un niveau doit
tomber entre FL30 et FL660, sinon il est déclassé.

Et une boucle infinie que j'avais introduite en corrigeant le premier point — un index qui
ne progressait plus quand un mot ambigu ne lisait aucun chiffre.

## Première application : identifier 125,933

Cette fréquence est au catalogue comme **« jamais cataloguée — des. 125.935, non
identifiée »**, écart crête-médiane 23,5 dB. La captation du 15/09 en a livré le premier
enregistrement : 134 transmissions. Quarante tirées au hasard, transcrites, puis passées
à la grammaire :

| Ce que la grammaire extrait | Valeurs |
|---|---|
| **Niveaux de vol** | FL370 ×2, FL380, FL390, FL398, FL400, FL410, FL430 |
| **Fréquences citées** | **132.505 ×3**, 123.15 |
| Pistes | 14 |

**Huit niveaux sur neuf entre FL370 et FL430.** C'est de la croisière en espace supérieur.
**125,933 est un secteur de contrôle en route, pas une approche** — ce qui la distingue
nettement de ses voisines 124,350 et 125,825 dans la même fenêtre de réception.

Les indicatifs entendus vont dans le même sens : Jetex, Ryanair, Flexjet, « sky travel »,
avec des tournures de transfert de secteur — « good day control », « back with you ».
Occupation 4 à 5 %, le régime d'une vraie fréquence de contrôle.

### Et elle relie deux inconnues entre elles

**132,505 est citée trois fois sur quarante transmissions**, toujours en fin de message,
suivie d'un indicatif et d'un « good day » — la forme exacte d'un transfert :

```
« One Three Two Five Zero Five, Flex Jet Six Six Seven, good day »
« One Three Two Five Zero Five, runway one four, cleared to Charlie, thank you, bye bye »
```

Or `06-catalogue.csv` porte **132,500 — « jamais cataloguée, des. 132.505, non
identifiée »**, avec 545 enregistrements déjà au corpus, et `04-corpus.md` la donne comme
anglophone sur la foi d'un « Qatari, flight level 380 ».

**Les deux fréquences inconnues du catalogue sont deux secteurs en route adjacents, et
125,933 passe ses avions à 132,500.** Aucune des deux n'était identifiée ; le lien entre
elles est mesuré, pas supposé.

> ⚠️ **Ce que ça ne dit pas.** Le nom du secteur et l'organisme (Paris Contrôle, Reims,
> autre) ne se déduisent pas de là. Et l'identification repose sur des transcriptions non
> vérifiées : si le modèle entend systématiquement mal les niveaux, tout l'argument tombe.
> **Le contrôle qui trancherait est le croisement ADS-B** — comparer les niveaux entendus
> aux altitudes réelles des avions présents. Il n'a pas pu être fait : l'historique ADS-B
> de la fenêtre de captation a été perdu (voir plus bas).

## Une perte de données à consigner

L'historique ADS-B du 15/09 entre 10:05 et 12:43 UTC — la fenêtre de la captation —
**n'existe plus**. Il a été effacé par un `rm -rf data/` lors d'une relance de Co-ATC à
16:07 CEST. La base ne couvre que 16:07 → 17:23.

Conséquence : l'appariement d'indicatifs contre les avions réellement présents, qui est la
mesure de justesse sans annotation, **n'est pas faisable rétroactivement sur ce
matériel** — précisément ce que ces 13 heures d'anglais d'approche rendaient possible.

**Règles adoptées :** la base SQLite de Co-ATC ne s'efface plus, et toute captation future
s'accompagne d'un enregistrement ADS-B simultané. À porter dans la prochaine demande à la
station.

## Le jeu d'évaluation anglophone

Constitué selon le protocole de `09-jeu-de-test.md`, graine `20260915` :
**20 transmissions par fréquence anglophone**, tirées au hasard sur les 1 054 extraites
de la captation — 430 sur 124,350, 133 sur 124,625, 491 sur 125,825. Manifeste dans
`whisper-lab/jeu-anglais.json`.

Il complète les 120 du premier jeu et comble le trou de Q17 : le jeu initial n'avait
aucune fréquence que le catalogue donne comme anglophone.
