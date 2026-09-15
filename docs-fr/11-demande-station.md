# Demande à la station — enregistrer le groupe `orly-approche`

*Rédigé le 15 septembre 2026 par l'agent de `co-atc-local`, à l'intention de l'agent qui
tient la station. Rien n'a été modifié sur la machine : ce document est une demande, pas
un compte rendu.*

## Ce qu'on demande, en une phrase

**Une session d'enregistrement par transmission sur le groupe `orly-approche`, qui est
déjà à l'antenne, pendant deux à trois heures de trafic diurne.**

## Pourquoi

Le corpus de 4 642 transmissions ne contient **aucune fréquence anglophone** : croisé avec
`06-catalogue.csv`, zéro des onze fréquences marquées « anglais dominant » a le moindre
enregistrement (Q17). Les 4 642 fichiers sont le résidu de deux groupes seulement,
`paris5-avec-enregistrement` et `amas-132-133`.

Or l'évaluation de la transcription locale doit trancher sur les deux langues. Sans
anglais d'approche — débit rapide, secteur saturé, collationnements serrés, accents non
natifs — on mesure la moitié du problème. Et c'est précisément le régime pour lequel le
modèle candidat (`jacktol/whisper-medium.en-fine-tuned-for-ATC`) a été affiné : on risque
de le sous-estimer.

## La bonne nouvelle : aucun changement de groupe

Relevé le 15/09 sur `/radio/etat` : le mode d'écoute courant est **`orly-approche`**,
`accord: true`, un auditeur.

| Fréquence | Service | Langue au catalogue | Crête / médiane |
|---|---|---|---|
| 123,875 | Orly Approche | mixte | +37,6 / +16,1 |
| **124,350** | Approche CDG + Le Bourget | **anglais** | **+44,6 / +25,6** |
| **124,625** | Paris Contrôle, secteur DG | **anglais** | +46,9 / +6,2 |
| **125,825** | De Gaulle Approche | **anglais** | +35,4 / +14,1 |
| 125,933 | non identifiée, 8,33 | ? | +34,5 / +11,0 |

**Le matériel manquant est déjà démodulé.** Il n'est simplement pas écrit sur le disque.
Bonus : 125,933 est une des quinze inconnues du catalogue, jamais écoutée — l'enregistrer
la qualifie au passage.

## Le changement demandé

Il existe déjà, à l'identique, entre `paris5.tmpl` et `paris5-avec-enregistrement.tmpl` :
**une seconde sortie par canal**, à côté de la sortie mélangeur.

```
      outputs: (
        { type = "mixer"; name = "terminal"; balance = -0.3; ampfactor = 0.8; },
        { type = "file"; directory = "/rec/124350"; filename_template = "t";
          split_on_transmission = true; continuous = false; append = false; }
      ); },
```

Soit un `orly-approche-avec-enregistrement.tmpl` calqué sur l'existant. Deux détails
relevés dans le gabarit courant :

- le mélangeur s'appelle **`terminal`**, pas `paris` ;
- dans `paris5-avec-enregistrement`, l'`ampfactor` du canal le plus fort a été **abaissé**
  (1,2 → 0,7) en même temps que l'ajout de la sortie fichier. À reproduire ou non selon
  ce que l'agent de la station sait de cette décision — nous ne la connaissons pas.

## Les contraintes, et elles ne sont pas négociables

> ⚠️ **`split_on_transmission` dégrade le flux en direct.** Mesuré et consigné dans
> `01-station.md` : médiane de salve **4,6 s → 0,1 s**, et **0 % → 64 %** de salves sous
> la seconde, avec le processeur à 6 %. Ce n'est pas un problème de puissance mais de
> latence : RTLSDR-Airband ouvre et ferme un encodeur LAME à chaque squelch, dans le fil
> de sortie audio.

Donc :

1. **Jamais pendant que le propriétaire écoute.** `/radio/etat` donne le nombre
   d'auditeurs ; il valait 1 au moment de cette demande.
2. **Validation obligatoire avec `salves.py`**, avant et après, **à la même heure de la
   journée** — une bonne configuration donne une médiane de 3 à 5 s et 0 % de salves sous
   la seconde.
3. **Retour au gabarit sans enregistrement dès la session terminée.** Ce n'est pas un
   état permanent.
4. **La chaîne ADS-B n'est pas touchée.** Elle n'a rien à voir avec ceci, et elle ne doit
   même pas être approchée.

## Combien de temps — mesuré, pas estimé

Débit réel des sessions passées, calculé sur les horodatages du corpus existant :

| Fréquence | Transmissions | Durée de session | Par heure active |
|---|---|---|---|
| 129,525 Chavenay | 765 | 11h19 → 16h00 | **128 / h** |
| 127,750 Orly Départs | 543 | 11h21 → 15h59 | **109 / h** |
| 132,500 en route | 545 | sur 17 h, nuit comprise | 32 / h |

Une approche chargée en journée donne donc de l'ordre de **100 transmissions par heure et
par fréquence**. Avec trois fréquences anglophones simultanées, **deux à trois heures de
trafic diurne suffisent largement** pour 200 à 300 transmissions anglaises exploitables
après tri par détection de voix. La session du 12/09, 11h21–16h00, est le patron.

## Ce dont nous avons besoin en retour

Rien de plus que ce qui existe déjà pour les autres fréquences :

- les fichiers dans `/opt/adsb/public/transmissions/<freq>/`, nommés
  `t_AAAAMMJJ_HHMMSS.mp3`, servis sur
  `http://macmini-fedora.lan/fichiers/transmissions/<freq>/` ;
- le nom du dossier **dérivé de la fréquence réellement démodulée**. Le corpus actuel
  contient un dossier `128150` dont le gabarit dit lui-même qu'il s'agit de **128,142**
  (« le catalogue disait 128,150 : faux de 8,3 kHz »). Éviter de reproduire ça : pour
  125,933, écrire `125933`.

## L'alternative, et pourquoi on ne la demande pas aujourd'hui

Le mode de diffusion par canal (décision D2) — un point de montage Icecast continu par
fréquence — permettrait d'enregistrer **sans** ouvrir un encodeur à chaque squelch, donc
sans le défaut ci-dessus. C'est ce qu'il faudra de toute façon construire pour brancher
Co-ATC sur la station.

Mais **c'est une hypothèse non mesurée** : `01-station.md` dit explicitement que les
sorties Icecast « ne devraient pas souffrir de ça — mais c'est une hypothèse, pas une
mesure ». La construire et la valider est un chantier ; enregistrer deux heures avec un
mécanisme déjà éprouvé en est un autre.

**Recommandation : faire la session d'enregistrement maintenant avec le mécanisme connu**,
et garder le mode par canal pour le moment où il sera de toute façon nécessaire. La
mesure ne doit pas attendre l'architecture.

## Qui fait quoi

| | |
|---|---|
| **L'agent de la station** | modifie le gabarit, lance et arrête la session, valide avec `salves.py`, restaure l'état. Il possède `radio-ctl`, `serveur.py`, les gabarits et la référence de salves. |
| **L'agent de `co-atc-local`** | ne touche pas à la machine. Il consomme les fichiers en lecture seule, trie par détection de voix et mesure. |

Cette frontière est délibérée : la station est en production et sa configuration a des
invariants que ce chantier-ci ne connaît pas.
