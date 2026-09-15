# Mission

## En une phrase

Adapter [Co-ATC](https://github.com/yegors/co-atc) pour qu'il tourne **entièrement en
local, sans aucune API cloud**, sur une station bilingue français/anglais — et en faire
un dépôt public réutilisable.

## Pourquoi un fork plutôt qu'une contribution

Co-ATC amont fait reposer la transcription, l'extraction de clairances et l'assistant
vocal sur l'**API OpenAI**. Trois conséquences que le propriétaire refuse : un coût
récurrent proportionnel à l'écoute, l'envoi continu de l'audio capté chez un tiers, et
une dépendance réseau sur une station conçue pour être autonome.

Le fork remplace cette dépendance par une chaîne locale. Ce n'est pas un détail de
configuration : la transcription est au centre du produit, et la remplacer touche le
format des résultats, la latence, la gestion des files d'attente et la qualité.

## Les trois apports revendiqués

1. **Zéro cloud.** Transcription et post-traitement en local. Aucune clé d'API requise
   pour faire fonctionner le produit.
2. **Bilingue français / anglais.** C'est l'apport le moins évident et le plus utile.
   Le contrôle aérien français se fait en français avec les pilotes francophones et en
   anglais avec les autres, **souvent en alternance sur la même fréquence**. Les modèles
   Whisper affinés pour l'ATC qui existent sont anglophones. Voir
   `03-transcription.md` — c'est le cœur technique du chantier.
3. **Adapté à une réception multicanale réelle.** La station démodule 5 à 7 fréquences
   en parallèle et change de groupe de fréquences selon ce qu'on veut écouter. Co-ATC
   suppose une liste de fréquences figée.

## Périmètre

**Dans le périmètre :**

- remplacement de la transcription OpenAI par une chaîne Whisper locale, bilingue
- adaptation de l'ingestion audio à la chaîne RTLSDR-Airband existante
- configuration station : position, fréquences, terrains et pistes d'Île-de-France
- remplacement des sources météo par des sources françaises (METAR/TAF/NOTAM du SIA
  ou d'Aviation Weather Center)
- documentation d'installation reproductible pour un tiers

**Hors périmètre pour l'instant, à rediscuter :**

- l'assistant vocal (`[atc_chat]`) — dépend d'un modèle de langue ; on décidera plus
  tard s'il passe en local, s'il devient optionnel ou s'il disparaît
- l'extraction de clairances par IA — même question
- toute exposition sur Internet : Co-ATC n'a **aucune authentification** et son auteur
  le dit explicitement. Le service reste sur le réseau local.

## Ce qui existe déjà et qu'il ne faut pas refaire

Le propriétaire a construit sur cette station, en une semaine, un ensemble d'outils de
mesure et un catalogue de fréquences vérifié. **Tout est décrit dans `01-station.md` et
`04-corpus.md`.** En particulier :

- un catalogue de **52 fréquences identifiées** avec niveaux de crête et de médiane
- **4 642 enregistrements de transmissions réelles**, déjà triés pour ne garder que
  celles qui contiennent de la parole — un jeu d'évaluation prêt à l'emploi
- une chaîne faster-whisper qui tourne et qui a déjà produit des transcriptions

Ne pas repartir de zéro là-dessus.

## Critère de réussite de la première étape

Co-ATC affiche la carte du trafic réel autour de la station, avec les transcriptions
des fréquences d'Île-de-France, **sans qu'aucune clé d'API n'ait été renseignée**, et
en transcrivant correctement aussi bien « Air France 1234, autorisé décollage piste 08 »
que « Charlie Oscar, touché autorisé, piste 28 ».
