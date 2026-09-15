# Stratégie bilingue — aiguillage informé vers deux modèles spécialisés

*Posée le 15 septembre 2026, après les mesures de `10-mesure-amorces.md` et
`12-concatenation.md`. Elle remplace la formulation de `03-transcription.md`, qui
opposait « un modèle ATC anglais » à « un Whisper multilingue générique ».*

## Pourquoi la formulation initiale était mauvaise

`03-transcription.md` proposait, pour la stratégie A, d'aiguiller entre **`jacktol` pour
l'anglais** et **un Whisper multilingue générique pour le français**. Les mesures ont
montré que la seconde branche ne tient pas : `large-v3-turbo` sur du français d'aéroclub
produit « rin David », « Attends sur la Faribah », une transmission sur six étiquetée en
japonais, et ne s'accorde avec aucun autre modèle sur aucune fréquence (F1 = 0,00).

**Un modèle multilingue générique n'est pas un modèle français.** La recherche du 15/09 a
identifié ce qui manquait : des Whisper **spécialisés français** existent —
[`bofenghuang/whisper-large-v3-french`](https://huggingface.co/bofenghuang/whisper-large-v3-french)
(1 219 téléchargements) et [`aihpi/FrWhisper`](https://huggingface.co/aihpi/FrWhisper).
Aucun n'est affiné pour l'ATC — **aucun modèle ATC francophone n'existe, c'est vérifié** —
mais ils traitent le français bien mieux qu'un générique.

## La stratégie A′

```
         transmission
              │
              ▼
      ┌───────────────┐   moins de 0,25 s de parole
      │  VAD Silero   ├──────────────────────────────►  rejetée, pas de transcription
      └───────┬───────┘
              │
              ▼
   a priori de fréquence (06-catalogue.csv, colonne langue_attendue)
              │
      ┌───────┼────────────────┬──────────────────┐
      ▼       ▼                ▼                  ▼
    « fr »  « en »         « mixte »            « ? »
      │       │                │                  │
      │       │        détection de langue        │
      │       │         sur la transmission       │
      │       │          │             │          │
      ▼       ▼          ▼             ▼          ▼
  ┌────────────────┐  ┌────────────────────────────────┐
  │ Whisper FR     │  │  modèle ATC anglais            │
  │ spécialisé     │  │  (large-v3-atco2 / jacktol)    │
  └────────┬───────┘  └────────────┬───────────────────┘
           └──────────┬────────────┘
                      ▼
       langue retenue + indice de confiance stockés (Q13)
                      │
     confiance basse ─┴─► marquée « incertaine », écartée du post-traitement
```

### Les quatre principes, chacun issu d'une mesure

**1. L'a priori de fréquence prime sur la détection.** Mesuré : la détection se trompe sur
16 % du français et 28 % de l'anglais, et ses erreurs sont des sorties en russe, japonais
ou norvégien — c'est-à-dire du charabia mal étiqueté. Le catalogue, lui, ne se trompe pas :
129,525 est un aéroclub français, 124,625 est un secteur en route anglophone. **On ne
consulte la détection que là où le catalogue dit « mixte ».**

**2. Le VAD n'est pas une optimisation, c'est une garde.** Mesuré : la part de parole
médiane est de 73 à 82 % en journée, mais 3 fichiers sur 30 tombent sous 1 s de parole. Et
la nuit, jusqu'à 98 % des déclenchements ne contiennent rien. Un modèle nourri de silence
hallucine — « Thank you. » sur 2,2 s de bruit en est l'exemple type.

**3. Un mauvais aiguillage doit se taire, pas se tromper.** C'est le principe le plus
important, et il vient de la mesure la plus inquiétante : un modèle anglais nourri de
français produit « cleared to land runway zero five oscar kilo papa » sur la tour d'un
aéroclub — **bien formé, confiant, entièrement faux**. Si ça atteint le post-traitement,
celui-ci fabriquera une fausse clairance d'atterrissage sur un vrai avion de la carte.
**Le schéma de l'amont n'a aucune colonne de langue : il faut l'ajouter, et c'est une
exigence de sûreté, pas un confort.**

**4. Pas d'amorce longue.** Mesuré : celle de l'amont multiplie les boucles par sept sur
le français et fuit dans la sortie. Si amorce il y a, elle sera courte et **dans la langue
de la branche**.

### Ce que la stratégie ne fait pas

- **Pas de concaténation.** Testée et fermée (`12-concatenation.md`) : l'accord entre
  modèles n'y gagne rien et la transmission précédente déborde sur la cible.
- **Pas de modèle unique.** L'hypothèse « un ATC multilingue suffit » a été testée :
  `large-v3-atco2` annonce « en » sur 100 % des fichiers, y compris du français pur, ce qui
  le disqualifie comme aiguilleur même s'il produit parfois du français.

## Le périmètre, revu à la baisse et assumé

Relevé le 15/09 sur le trafic réel de la station : **2 appareils légers sur 115
identifiés**, et 93 indicatifs sur 96 de forme compagnie. L'aviation légère francophone
des terrains voisins **n'est pas sur la carte** — pas d'ADS-B, ou pas de position. Une
transcription parfaite de Chavenay n'aurait presque rien à quoi s'accrocher.

Le français utile au produit, c'est donc surtout **Air France et Transavia France sur les
fréquences de contrôle** — 19 indicatifs sur 93, soit 20 % du trafic, mais le premier
opérateur du ciel local. C'est cette part-là que la branche française doit servir. Les
fréquences d'aéroclub peuvent sortir du périmètre de transcription sans grande perte.

## Ce qui reste à mesurer

1. **Les deux modèles français s'accordent-ils** entre eux sur le français de cette bande ?
   C'est le seul indice de justesse disponible avant annotation.
2. **Produisent-ils de la phraséologie française réelle** — « rappelez vent arrière »,
   « autorisé décollage piste 28 » — ou du français quelconque ?
3. **Quel coût** : deux modèles `large-v3` en mémoire, sur un M4 à 24 Gio.
4. **Le seuil de confiance** au-dessus duquel on accepte l'arbitrage de la détection sur
   les fréquences mixtes.
