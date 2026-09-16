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
