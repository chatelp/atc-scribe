# Audit produit : atc-scribe n'est plus co-atc — 09/10

*Ouvert le 9 octobre 2026 à la demande du propriétaire. Étude, pas chantier : rien n'est codé
avant discussion (Q52 dans `05-decisions.md`).*

## Pourquoi maintenant

Mesuré le 09/10 contre `upstream/main` (dernier commit amont : 3 mai 2026) :

| | |
|---|---|
| Commits du fork | 263 |
| Fichiers touchés | 169, dont **130 nouveaux** et 38 fichiers amont modifiés |
| Lignes | **+33 941 / −1 312** |
| Serveur Go aujourd'hui (hors tests) | 35 077 lignes |
| Interface (JS + HTML) | 13 036 lignes |

Ce que le fork a ajouté n'est pas une adaptation : transcription locale par sidecar, lecture
française à deux modèles, liaison avec la station (`radio-ctl-sync`), un flux par fréquence,
appariement avion-transmission validé contre l'ADS-B, vue PAR, juge des pistes validé contre
FlightAware, authentification, accès réseau. Ce que l'amont visait (Toronto, API OpenAI, chat
ATC, simulation) n'est pas ce qui tourne ici. L'interface, elle, est restée celle de l'amont,
avec nos ajouts par-dessus : c'est là que ça se voit le plus (D70 et le son, 02/10 ; la coupure
à −40 dB, 04/10 — deux défauts amont découverts parce qu'on s'en sert tous les jours).

## Périmètre et méthode

Trois études parallèles, factuelles, chiffrées, limitées à 150 lignes chacune, rapports bruts
dans `runs/audit-2026-10-09/` (hors dépôt) :

1. **`ux.md`** : inventaire de chaque panneau, bouton, modale et réglage, classé essentiel /
   utile / inutilisé chez nous / douteux ; frictions (écran d'accueil, clics pour le son,
   téléphone à 390 px) ; ce qui manque pour notre usage (antenne, station, sidecar) ; dix
   changements classés par gain et effort.
2. **`backend.md`** : paquets Go, lignes, utilisé ou non dans notre configuration, couverture
   de tests ; ce qui est à nous contre l'amont ; code mort, fonctions de plus de 150 lignes ;
   `config.toml` réel contre `config.toml` lu ; dix simplifications avec risque pour la
   production.
3. **`frontend-code.md`** : carte de `www/`, dépendances, code mort (chat, simulation,
   météo, OpenAI temps réel), grosses fonctions, et un plan de découpage de `app.js`
   par étapes qui laissent l'interface fonctionnelle.

Les trois rapports sont ensuite consolidés **ici**, en une liste unique classée par
gain / effort / risque, et le propriétaire tranche ce qui devient chantier.

## Ce que l'audit doit respecter

- **Le serveur tourne tous les jours.** Chaque étape doit laisser la production utilisable ;
  rien de « grand soir ».
- **Garder identifiable ce qui est contribuable** (CLAUDE.md) : sidecar, correctif de
  rotation (Q26), authentification, état serveur — même si on ne rebase plus.
- **L'UX se juge sur l'usage réel** : une personne, tous les jours, Mac et téléphone par
  Tailscale, 1 à 7 fréquences. Pas sur ce que l'interface amont permet.
- **Mesurer avant de conclure** : chaque suppression proposée cite le fichier et le nombre
  de lignes, chaque friction cite le geste et le nombre de clics.

## Le jugement du propriétaire (09/10)

Il est le seul utilisateur ; sa liste prime sur l'inventaire. Dans ses mots :

1. **Interface lourde et pas très moderne.**
2. **Affichage lent des déplacements d'avion** — « peut-être incontournable avec un
   affichage navigateur ». *À mesurer avant de l'admettre* : le serveur relève l'ADS-B
   toutes les secondes (`fetch_interval_seconds = 1`) et diffuse une prédiction par seconde
   (`livePredictionBroadcastInterval`), et `app.js` a un moteur d'animation avec ses propres
   compteurs (images/s, marqueurs mis à jour/s, lignes 339-343). Un navigateur sait animer
   à 60 images/s ; si c'est saccadé, c'est la chaîne relevé → diffusion → interpolation qui
   le fait, pas le navigateur. Premier relevé à faire : les compteurs du moteur sur une
   séance réelle, avec 80 à 110 avions.
3. **Les réglages sont « totalement mal foutus »** : ils méritent une **page à part,
   organisée, lisible**, plutôt qu'une modale.
4. **Les alertes en modale en haut de l'écran** ne sont « ni très intéressantes ni pratiques
   en l'état ».
5. **La disposition générale lui convient** : on ne refait pas l'agencement, on l'allège.

Ce que l'audit UX doit donc hiérarchiser : d'abord les réglages et les alertes (3, 4), puis
l'allègement (1), et une mesure pour trancher la lenteur (2) — sans toucher à la disposition
(5).

## État

- 09/10 : cadre posé ; trois études lancées en parallèle, rapports attendus dans
  `runs/audit-2026-10-09/`. Consolidation à suivre dans ce fichier.
