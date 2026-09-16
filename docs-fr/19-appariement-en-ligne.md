# 19 — L'appariement passe en ligne

*16 septembre 2026.*

Jusqu'ici le rapprochement entre une transmission et un avion vivait dans
`cmd/phraseology`, un outil de mesure. Rien dans co-atc en service ne s'en servait :
`grep -rn phraseology --include='*.go'` ne renvoyait, hors du paquet lui-même et de
ses tests, **aucune occurrence**. On avait un instrument, pas une fonction.

Ce document décrit la mise en service, et les mesures qui ont fixé ses réglages.

## Le point de greffe existait déjà

L'amont sépare la transcription en deux étages. Le premier écrit le texte brut. Le
second — `PostProcessor`, un appel à GPT-4o toutes les dix secondes — remplit trois
choses : **qui a parlé**, **de quel avion il s'agissait**, et **quelle autorisation
a été délivrée**.

Vérifié dans le code, et c'est le résultat qui a décidé de l'architecture :

| Ce dont on avait besoin | Ce que l'amont a déjà |
|---|---|
| une colonne pour le locuteur | `transcriptions.speaker_type` |
| une colonne pour l'indicatif | `transcriptions.callsign` |
| une table pour les autorisations | `clearances(transcription_id, callsign, type, text, runway, …)` |
| relier une autorisation à un avion de la carte | `handlers.go:327` — jointure sur `aircraft.Flight` |
| afficher le locuteur en couleur | `index.html:1842` — ATC orange, PILOT vert |
| propager la mise à jour au navigateur | `app.js:3100` — événement `transcription_update` |

**Aucune migration de schéma. Aucune ligne de JavaScript.** Notre grammaire écrit
dans les tables de l'amont, avec le vocabulaire de l'amont — `"ATC"`, `"PILOT"`,
`"takeoff"`, `"landing"`, `"approach"` — et toute la chaîne d'affichage suit.

C'est aussi le meilleur argument de rebasabilité qu'on ait rencontré : on ne greffe
pas une fonction à côté de l'amont, on **remplace un composant à sa propre couture**.
Le choix se fait par configuration :

```toml
[post_processing]
backend = "local"   # ou "openai" pour l'étage GPT-4o d'origine
```

### Un détail qui dormait

`app.js` sait traiter l'événement `transcription_update` et recopier les trois
champs dans toutes ses listes. **Mais l'amont ne l'émet plus** : son post-traitement
écrit le résultat dans un fichier journal — commentaire à l'appui, *« Log the
processed transcription instead of broadcasting »*. Le gestionnaire est donc du code
mort chez lui. Nous le réveillons : c'est ce qui fait apparaître un indicatif sans
recharger la page.

## Les réglages, mesurés

L'outil de mesure acceptait les appariements **ambigus** — ceux où un second avion
marque presque aussi bien. Le processeur en ligne les refuse. Les deux
configurations n'étant pas les mêmes, **le chiffre de 75 % publié dans
`18-nuit-du-15.md` ne s'appliquait pas à ce qu'on met en service.** J'ai ajouté
`-strict` à l'outil et tout remesuré. La règle d'acceptation est appliquée
identiquement au tirage réel et aux huit tirages témoins ; l'appliquer d'un seul
côté mesurerait la règle au lieu du signal.

Le refus des ambigus ne coûte rien et fait gagner en précision :

| | appariements | hasard | au-dessus du hasard | vrais |
|---|---|---|---|---|
| ambigus acceptés | 138 | 43,3 | 69 % | 94,0 |
| **ambigus refusés** | **138** | **43,3** | **69 %** | **94,7** |

### Balayage des chiffres et de la fenêtre

*Passe `atc-multi`, 1 415 transmissions, 8 témoins mélangés.*

| chiffres minimum | fenêtre | appariements | hasard | au-dessus | vrais |
|---|---|---|---|---|---|
| 3 | 30 s | 131 | 39,9 | 70 % | 91,1 |
| **3** | **60 s** | **138** | **43,3** | **69 %** | **94,7** |
| 3 | 120 s | 140 | 48,1 | 66 % | 91,9 |
| 4 | 30 s | 53 | 15,3 | 71 % | 37,7 |
| 4 | 60 s | 58 | 16,9 | 71 % | 41,1 |
| 4 | 120 s | 59 | 17,9 | 70 % | 41,1 |

Exiger quatre chiffres divise les appariements vrais par plus de deux pour gagner
deux points. Élargir la fenêtre à deux minutes ajoute deux appariements et onze
coïncidences. **Trois chiffres, soixante secondes** est l'optimum du balayage.

### Balayage du seuil de score

| seuil | appariements | au-dessus du hasard | vrais | fréquences vivantes |
|---|---|---|---|---|
| **0** (plancher interne 0,6) | **138** | **69 %** | **94,7** | **5 / 5** |
| 0,9 (chiffres exacts exigés) | 104 | 71 % | 73,4 | 5 / 5 |
| 1,0 (+ une corroboration) | 26 | 88 % | 22,9 | **2 / 5** |

Le seuil 1,0 — chiffres exacts **plus** une confirmation par l'altitude ou
l'opérateur — atteint 88 %, et c'était tentant. Mais le détail par fréquence montre
qu'il **éteint trois fréquences sur cinq** : la corroboration vient presque toujours
de l'altitude, qu'on énonce en route et quasiment jamais en finale. 123,875, 125,825
et le français tombent à zéro appariement.

Retenu : **pas de seuil supplémentaire**, le plancher interne de 0,6 et le refus des
ambigus. Le réglage reste exposé dans `config.toml` avec ce tableau en commentaire,
pour que le compromis soit celui du propriétaire et pas le mien.

## Le locuteur : un trou trouvé par un test

Un des tests écrits pour la mise en service a échoué sur
`« descend flight level one zero zero »` : aucun locuteur attribué. Mesure sur les
1 415 transmissions de la captation :

| | ATC | PILOT | sans avis |
|---|---|---|---|
| avant | 20 | 125 | **90 %** |
| après | 454 | 309 | **46 %** |
| français, avant | 0 | 0 | **100 %** |
| français, après | 63 | 30 | 67 % |

La table ne contenait que treize tournures anglaises et **pas un mot de français**.
Ajouté :

1. **Les verbes d'instruction à l'impératif.** C'est le temps qui discrimine, pas le
   verbe : le contrôleur dit *descend*, le pilote répond *descending*. Idem en
   français : *descendez* contre *on descend*. Ces formes ne doivent donc jamais
   être lemmatisées.
2. **La position de l'indicatif.** En phraséologie OACI le contrôleur ouvre en
   nommant l'avion auquel il s'adresse, le pilote signe avec le sien. Utilisé
   uniquement comme départage — une transmission dont les mots tranchent déjà n'est
   jamais contredite par la place des chiffres. Et seulement près d'un bord : un
   indicatif au milieu ne dit rien.

### Ce que je n'ai pas pu établir

**La couverture n'est pas la justesse.** Sans annotations, je ne peux pas dire quelle
proportion des 454 « ATC » est correcte.

J'ai construit un contrôle falsifiable à défaut : un collationnement suit son
instruction, donc sur deux transmissions proches de la même fréquence qui partagent
un nombre, **les étiquettes doivent alterner**.

| passe | paires | étiquetées | alternent | témoin mélangé |
|---|---|---|---|---|
| `atc-multi` | 29 | 18 | 61 % | 50 % |
| `atc-en` | 25 | 17 | 59 % | 51 % |

Environ dix points au-dessus du hasard, dans le bon sens, **sur 35 paires : ce n'est
pas concluant.** Sur 11 succès en 18 tirages, le hasard seul produit ce résultat une
fois sur quatre. Le contrôle est écrit et rejouable ; il demande un corpus plus long.

## Ce qui reste vrai

- **Le français n'est toujours pas traité.** 3,4 appariements vrais sur 280
  transmissions. La grammaire française existe maintenant ; le modèle, non.
- **Toujours aucun taux d'erreur de mots.** Les 120 annotations restent le seul
  chemin, et l'outil attend sur `http://127.0.0.1:8777/`.
- **Les chiffres ci-dessus viennent d'une captation de nuit de 2 h 38.** Le trafic
  de jour est plus dense : plus d'avions dans la fenêtre, donc plus de coïncidences.
  Le compromis devra être remesuré de jour.


---

## Ce que la première heure en direct a appris

Mise en service le 16/09 à 09:04 sur `aero.mp3`. Chaîne complète vérifiée :
flux → segmenteur → sidecar `atco2` → base → grammaire → appariement → carte.

**Le segmenteur va bien, contrairement à ce que je soupçonnais.** Je pensais que le
réencodage MP3 du flux mixé avait détruit le silence numérique sur lequel il
s'appuie. Mesure sur 60 s de flux, fenêtres de 20 ms : **87,3 % sont à exactement
zéro**. Le silence à −90 dBFS traverse le mélange et l'encodage. `silence_threshold
= 0.005` est le bon réglage, et 12,3 % des fenêtres portent de la parole — c'est
l'occupation du flux mixé.

Transcription en **0,80 s** par transmission, conforme aux 0,72 s médians mesurés
la veille. Débit observé : environ **3 transmissions par minute**.

### La mesure qui change l'ordre des priorités

Sur le premier échantillon en direct, **37 % des transmissions contiennent des mots
français**, et 12 % partent en boucle de dégénérescence — le symptôme du modèle
anglais sur de l'audio français. Le catalogue déclare `language = "en"` sur le flux
mixé, donc tout part vers `atco2`, français compris.

C'est le premier chiffre de proportion réelle qu'on ait. Le français n'est pas un
complément : **c'est plus du tiers de ce que la station entend.**

### Le premier appariement, juste pour la mauvaise raison

> *« one two four three five five seven six zero Papa X-ray thank you »*
> → **N760PX**, réellement en vue à 2 175 ft.

L'avion est le bon. Mais la trace montre que la grammaire y est arrivée par accident,
et découvre trois défauts d'une seule racine :

| | avant | après |
|---|---|---|
| valeurs extraites | un bloc unique `124355760` | **fréquence 124.355** + **indicatif 760** |
| lettres | aucune | **PX** |
| score | 0,60 « suffixe de chiffres » | **1,20 « chiffres exacts + lettres »** |
| locuteur | **ATC**, faux | aucun — le départage ne se trompe plus |

1. `readNumber` lisait goulûment les chiffres de la fréquence **et** ceux de
   l'indicatif en un seul nombre de neuf chiffres. Il s'arrête maintenant sur une
   fréquence VHF complète : aucun indicatif, transpondeur ou niveau ne fait six
   chiffres, donc six chiffres ouvrant dans la bande 118–136 sont une fréquence et
   rien d'autre.
2. `letterGroups` exigeait **trois** lettres et jetait « Papa X-ray ». Or un avion
   léger est appelé par les deux dernières lettres de son immatriculation — c'est
   l'essentiel du trafic des aérodromes voisins. Seuil ramené à deux. Le risque est
   nul par construction : le matcher ne peut que *monter* un score avec des lettres.
3. Le locuteur était **ATC** sur ce qui est un collationnement de pilote, parce que
   le départage positionnel avait pris la fréquence de tête pour un indicatif. Le
   découpage corrige la cause.

**Vérifié contre régression sur les 1 468 transmissions enregistrées** — et c'est un
gain, pas seulement une correction :

| | appariements vrais (`atc-multi`) |
|---|---|
| avant | 94,7 |
| **après** | **104,7** — **+11 %** |

La précision tient sur les cinq fréquences, et 124,625 passe de 56 % à 63 %. Une
heure de direct a rapporté plus que la nuit de mesure sur ce point précis : les
défauts de goulotte ne se voient que sur du trafic qu'on n'a pas choisi.


---

## La règle « chiffres à un près » était 81 % de bruit

*16/09, sur le groupe `gros-porteurs`.* Sur les huit premiers appariements en direct,
trois étaient douteux — et le journal, qui enregistre le score et la raison, montre que
**deux venaient de la même règle** :

> *« hello bonjour **Air France Three Six Seven** … Air France Three Six Seven … France
> Three Six Seven »* → apparié à **AFR3 7 7**, `digits off by one + operator named`.
> L'avion dit trois fois son numéro et ce n'est pas celui-là.

> *« Wifi **Heli One Six Two** si on descend niveau six zero »* → apparié à **N1 3 2QS**,
> `digits off by one + altitude agrees`.

Le défaut est structurel, pas statistique. Un appariement à un chiffre près vaut 0,50,
sous le plancher de 0,60 : il ne passe qu'avec une corroboration — et **une seule
suffit**. Au-dessus de Paris, « Air France » corrobore à peu près n'importe quoi.

Mesuré sur la captation, la règle appliquée identiquement au tirage réel et aux huit
témoins :

| | appariements | hasard | au-dessus | vrais |
|---|---|---|---|---|
| avec la règle | 152 | 47,3 | 69 % | **104,7** |
| **sans** | 144 | **40,8** | **72 %** | 103,2 |

Elle ajoute **8 appariements dont 1,5 vrai et 6,5 de coïncidence**. Éteinte par défaut ;
`-fuzzy` la rallume pour qui veut refaire la mesure. La précision par fréquence gagne
partout sauf sur 125,825 : 124,350 passe de 78 à 81 %, 124,625 de 63 à 70 %.

**Ce que ça dit de la méthode.** Cette règle avait l'air raisonnable — les modèles
entendent mal, un chiffre près semble une tolérance prudente. Elle a tenu depuis hier
parce que personne ne l'avait mesurée *seule*. Huit appariements en direct ont suffi à
la rendre suspecte, et le corpus enregistré à la condamner.


---

## Ce que co-atc faisait de la voix, et ce qu'il en fait maintenant

Question posée le 16/09 : en cliquant sur un avion, on ne voyait rien de la radio.
Inventaire fait dans le code, pas de mémoire :

| donnée | stockée | affichée sur l'avion, avant |
|---|---|---|
| texte brut, texte normalisé | `transcriptions` | non — seulement dans la liste par fréquence |
| locuteur ATC / PILOTE | `transcriptions.speaker_type` | non |
| indicatif apparié | `transcriptions.callsign` | non |
| autorisation | `clearances` | **oui, la seule** — et sans son texte |
| niveau, cap, vitesse, piste, QNH, transpondeur | **nulle part** | non |

Soit, mesuré sur les transmissions du jour : **66 % portent au moins une valeur
structurée, 4 % une autorisation.** La fiche d'un avion montrait donc environ 4 % de
ce que la voix contenait, et seulement quand l'appariement avait réussi en plus.

Le plus frustrant : `GET /api/v1/transcriptions/callsign/{callsign}` **existe en
amont**, il est indexé, il répond — et `grep transcriptions/callsign www/` ne renvoie
rien. C'est logique de leur côté : l'endpoint ne sert à rien tant que la colonne
`callsign` est vide, et elle ne se remplit qu'avec une clé OpenAI. Notre grammaire la
remplit localement, donc la plomberie était vivante des deux bouts sans raccord.

### Les trois manques comblés

**1. Une section « Radio » sur la fiche avion.** Les transmissions rattachées à cet
avion, avec l'heure, le locuteur en couleur et le texte, par l'endpoint qui existait
déjà.

**2. Les valeurs extraites, enfin conservées.** `Result.Values` était calculé puis
jeté à chaque transmission. Elles vont dans une table `phraseology_values`, une ligne
par valeur — les questions utiles sont relationnelles : tous les niveaux donnés à un
avion, toutes les pistes entendues cette nuit.

> **Pourquoi une table et non une colonne.** L'amont crée `transcriptions` avec
> `CREATE TABLE IF NOT EXISTS` et ne migre jamais. Une colonne ajoutée laisserait
> toute base existante un schéma en arrière, sans rien pour s'en apercevoir. Une
> table séparée apparaît d'elle-même sur les bases anciennes comme neuves, et elle
> ne gêne pas un rebase.

**3. Le texte de l'autorisation**, colonne remplie depuis toujours et jamais affichée.

### Le rattrapage, et un compteur qui mentait

La table arrivant après les transcriptions, tout ce qui était déjà annoté serait resté
muet pour toujours. Un rattrapage borné tourne au démarrage : anti-jointure sur les
transmissions annotées sans valeur, idempotent par construction.

Premier passage : `examined 341, stored 254`. **Le chiffre était faux.** La table n'en
contenait que 92 : `storeValues` écarte les indicatifs — ils ont leur propre colonne —
et les valeurs sans rôle, si bien que le compteur comptait les transmissions examinées
avec au moins une valeur, pas celles qui avaient écrit quelque chose. Corrigé en
`transmissions_with_values` et `values_stored`, distincts.

Ce que le rattrapage a trouvé sur la matinée :

| rôle | valeurs |
|---|---|
| niveau de vol | 27 |
| fréquence de transfert | 26 |
| altitude | 15 |
| vitesse | 15 |
| piste | 12 |
| cap | 9 |
| QNH | 6 |
| transpondeur | 1 |

### Une limite vue en passant

`« one two zero heading Delta Two Two Zero »` ne rend aucun cap : la grammaire est
ancrée sur le mot-clé et lit **vers l'avant**, alors qu'ici le nombre précède son
mot-clé. `trailingRoles` gère déjà ce cas pour *feet*, *knots* et *degrees* ; *heading*
n'y est pas. Non corrigé, non mesuré.


---

## Quel groupe d'écoute rapporte le plus ? Deux mesurés, un en cours

Trois groupes, **fenêtres de 51 minutes identiques**, et surtout : mesurés **hors
ligne sur la base du jour**, avec le même matcher et le même témoin mélangé à huit
tirages. C'est ce qui rend la comparaison possible — pendant la matinée le matcher a
changé deux fois, et des comptes relevés en direct n'auraient pas été comparables.

> La base garde `adsb_targets` : 270 000 relevés sur la matinée. Toute la journée peut
> donc être remesurée a posteriori, avec la version du jour de la grammaire. C'est à
> ça que servent `-from` et `-to`.

| | tours (`decollages-atterrissages`) | `gros-porteurs` |
|---|---|---|
| fenêtre | 09:04 → 09:56 | 09:56 → 10:47 |
| transmissions | 192 | **256** |
| avec un indicatif candidat | 114 — 59 % | **197 — 77 %** |
| appariés | 20 | **42** |
| hasard, 8 témoins | 1,7 | 9,0 |
| **appariements vrais** | 18,3 | **33,0** |
| au-dessus du hasard | **91 %** | 79 % |
| avions dans la fenêtre, médiane | 76 | 93 |

**`gros-porteurs` rapporte 80 % d'appariements vrais en plus.** Il porte aussi
sensiblement plus de transmissions contenant un indicatif — 77 % contre 59 %, ce qui
est la différence entre une fréquence d'approche, où chaque échange nomme l'avion, et
une tour, où l'on entend des rappels de piste et des messages courts.

**Mais les tours sont plus précises**, 91 % contre 79 %, et ce n'est pas un hasard :
la médiane des avions dans la fenêtre y est de 76 contre 93. Moins d'avions, moins de
coïncidences arithmétiques possibles. Le trafic local à indicatifs en lettres, qui ne
s'apparie jamais par chance, tire aussi la précision vers le haut.

Autrement dit, les deux chiffres ne mesurent pas la même chose : le volume suit le
trafic, la précision suit l'inverse de la densité. Un groupe dense donne plus
d'appariements *et* plus de faux.

**Troisième groupe en cours** : l'amas 132-133, basculé à 10:48 sur le gabarit sans
sortie fichier. La mesure de la nuit le situe dans l'espace supérieur, FL160 à FL390 —
donc peu d'avions à la fois et de l'anglais, ce qui devrait donner beaucoup de
précision. Reste à voir le volume.
