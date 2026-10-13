# Les écrans finaux

*13/10/2026, session design. Quatorze écrans assemblés avec le design system seul (`08-design-system.md`) :
ses jetons et ses classes `cl-`, aucun style propre. Ils sont la référence d'acceptation de la
migration : chaque étape livrée se compare à eux.*

- Source : `tools/design/ecrans.py`, qui écrit `ecrans/*.html` (ouvrir `ecrans/index.html` en servant
  `docs-fr/design/`). Le cadre PAR vient de `tools/design/par-mini.py`.
- Galerie publiée (privée, au propriétaire) : https://claude.ai/artifact/2gZSBGbTkEySxThbNRYhMF.
  La copie publiée intègre les feuilles dans chaque écran ; elle se refait par
  `python3 tools/design/ecrans.py <dossier>`.
- Règle de fabrication : un écran qui a besoin d'un style absent du système révèle un composant
  manquant ; on l'ajoute au système, pas à l'écran. Seuls sont permis en ligne la position d'une
  fenêtre flottante et les illustrations qui tiennent lieu d'un moteur de rendu (la carte, la PAR).
  Vérifié le 13/10 : aucune classe hors du système, aucune balise `<style>`.

## Les quatorze écrans

| Écran | Ce qu'il montre |
|---|---|
| `a-maintenant` | A par défaut : le fil, le ciel, la colonne sur « Right now » (synthèse, séquence, « Worth knowing », météo, compteurs) |
| `a-trafic` | A sur « Traffic » : recherche, altitude avec histogramme, préréglages et QNH, Air / Ground / Heard, familles, Active / Signal lost, tri ; la carte suit les filtres |
| `a-frequence` | le menu d'une fréquence (volume, « seulement elle dans le fil ») ; « ↓ 3 new exchanges » après avoir remonté le fil |
| `b-replie` | B : la colonne repliée, le fil en transcription, « Right now » en carte sur la carte |
| `c-suivre` | C : l'histoire du vol dans la colonne, sa trace sur la carte centrée sur lui, son approche dans le cadre PAR |
| `c-details` | C, onglet Details : historique des phases, consignes entendues, informations de base, sections repliées |
| `a-calques` | les calques et fonds de carte ; la fiche d'un aérodrome |
| `a-journal` | le journal (la cloche) : ses six sortes d'événements et ses sons |
| `par` | la vue PAR existante, son en-tête et sa séquence, à la place du ciel |
| `reglages` | la page des réglages par-dessus l'écran |
| `connexion`, `premier-lancement` | la connexion ; la question du premier lancement |
| `etats` | rien à l'antenne, connexion perdue, « cliquez pour reprendre l'écoute » |
| `telephone` | trois volets du téléphone, barre d'onglets en bas, tailles tactiles |

## Où va chaque fonction d'aujourd'hui

Tiré de l'inventaire de `www/` fait le 13/10 sur `origin/main` (≈ 150 fonctions). Ne sont listées
que celles qui changent de place ; le reste garde la sienne (carte, PAR, liste, fiche).

| Aujourd'hui | Demain |
|---|---|
| Barre radio : tuile par fréquence (écouter / couper, MHz ou « mixed », échec / nouvel essai) | `ThreadHead` : un bouton de fréquence en deux parties |
| Volume par fréquence (curseur dans la tuile) | menu de la fréquence (▾), `ChannelMenu` |
| « all » écouter / couper tout | « Mute all » / « Listen to all » en tête du fil |
| Fil de transcription par fréquence (fenêtre, recherche, non-lus) | un seul fil, filtre « Show » par fréquence, recherche du fil, une pastille « ↓ N new » |
| Niveau et secondes depuis la dernière émission | `LevelMeter` dans le fil ; l'âge dans le menu |
| METAR, TAF, NOTAM (barre du bas) | « Right now » › `Weather` (METAR lu en clair, TAF et NOTAM en boutons) |
| Piste en service par aérodrome (barre du bas) | la phrase de synthèse et l'en-tête de la PAR |
| Bandeau d'état (antenne, transcription, ADS-B) | `TopBar` |
| Horloges locale / UTC, cliquables pour choisir l'heure affichée | les deux dans `TopBar` ; le choix passe dans Réglages › Display |
| Engrenage et cloche en haut à gauche de la carte | `TopBar`, à droite |
| Titre « Tracked Targets » | l'onglet « Traffic » |
| Fiche avion : Timeline, Details, Tracks, Proximity | onglets Story, Details, Track, Nearby de la colonne (C) |
| Fiche avion : pas de bouton fermer au bureau | « ‹ Back to Traffic » dans la colonne, ✕ « Stop following » dans `TopBar` |
| Tab / Maj-Tab : avion suivant | J / K, ↓ / ↑ dans la liste (choix du 13/10) ; Tab rend la main au navigateur |
| Téléphone : Map, PAR, List | Radio (accueil), Sky (carte ou PAR), Traffic, Right now |
| Écran « Connection Lost » plein écran | `ErrorBanner` en haut du ciel, la carte reste utilisable |

## Six propositions, validées par le propriétaire le 13/10

Présentées une à une, avant / après sur les écrans réels ; toutes retenues telles quelles.


1. **Le volume dans le menu de chaque fréquence** (▾), puisque la barre du bas disparaît.
2. **Un seul fil et une seule pastille de nouveaux échanges**, au lieu d'un fil et d'un compteur
   par fréquence ; le filtre « Show » remplace l'ouverture d'un fil particulier.
3. **La météo dans « Right now »**, le METAR lu en clair en une phrase.
4. **Le choix UTC / heure locale dans les réglages**, les deux horloges restant affichées.
5. **Le téléphone à quatre onglets**, la radio en accueil.
6. **Les consignes entendues sans verdict** : la colonne « Status » de la fiche (prévue pour
   issued / complied / deviation, mais qui n'affiche jamais que « issued » : rien n'écrit les deux
   autres) devient « Then » et dit ce que l'avion a fait (« level at 4000 ft »), sans
   qualifier l'écart. C'est le principe « constater, ne pas juger » appliqué à une fonction qui
   existe ; le propriétaire a confirmé qu'il s'applique ici.

## Ce qui n'est pas dessiné

Les volets Track et Nearby de C (ils gardent leur contenu actuel dans les composants du système),
les fiches de balise et d'héliport (même forme que celle d'un aérodrome), le détail des onze
règles d'appariement et des rapports système dans les réglages (repliés, contenu inchangé).
