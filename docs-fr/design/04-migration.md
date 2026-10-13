# Plan de migration — des écrans validés à l'interface vivante

*Réécrit le 13/10/2026 par la session design, après la validation des écrans finaux (D77, D78) ;
**validé par le propriétaire le 13/10**, après la relecture côté code de la session atc-scribe. Il
remplace le plan du 12/10, qui gardait la disposition : ses étapes de fond (jetons, couleurs,
phases, police) restent, mais s'appliquent maintenant aux composants de la nouvelle structure au
lieu de retoucher ceux qui vont disparaître. La migration se fait par la session atc-scribe, sous
les sept garde-fous de `32-audit-produit.md` ; la session design relit chaque étape.*

## Les références

| Quoi | Où | Rôle |
|---|---|---|
| Les écrans | `ecrans/*.html` (`python3 tools/design/ecrans.py`), galerie publiée | **le critère d'acceptation** : chaque étape se compare à son écran |
| Les composants | `systeme/project/components/` (`bundle.css`, une fiche par composant) | le seul CSS d'interface autorisé dans une étape |
| Les valeurs | `jetons.json` → `tools/design/tokens.mjs` (contrastes) → `ds-export.mjs` | la seule source des couleurs, tailles, espaces |
| Ce qui doit survivre | `09-ecrans.md`, « Où va chaque fonction d'aujourd'hui » | la liste à cocher : rien ne se perd |
| D'où viennent les données | `07-esquisses.md` | tout est disponible dans le store depuis le 13/10 |
| Les choix tranchés | `03-systeme.md` (fin), D77, D78 | police, cinq familles, rampe d'altitude, J / K ; les six décisions des écrans |

## Qui fait quoi

| Qui | Rôle |
|---|---|
| Le propriétaire | décide ; toute question nouvelle de comportement remonte à lui ; une soirée d'écoute clôt chaque étape visible |
| La session atc-scribe | implémente : verse `design` dans `local`, code les étapes dans `www/` et le serveur (≈ 4 h 30), tient l'instance d'essai, publie, relance la production, tient `05-decisions.md` et `33-pilotage.md` |
| La session design | garde la référence (`jetons.json`, le système, les écrans) et ses outils (`tools/design/`) ; relit chaque étape livrée ; complète le système quand un manque apparaît ; ne touche jamais `www/` |

Le déroulé d'une étape : livraison sur l'instance d'essai → la session design compare à l'écran de
référence et rejoue les mesures, répond « conforme » ou nomme les écarts → corrections → soirée
d'écoute → versement dans `local`. Un composant manquant s'ajoute d'abord au système, par la session
design, puis se recopie ; une valeur se change dans `jetons.json` ; un écart volontaire se consigne
dans `09-ecrans.md` et l'écran se met à jour.

## Avant tout : où vit `www/`

Mesuré le 12/10 : la production sert une copie figée (`runs/production/www`, réglage
`server.static_dir`), l'instance d'essai lit `www/` en direct. L'étape 0 donne à l'instance d'essai
le `www/` d'un arbre de travail : chaque étape s'y regarde avant d'atteindre `local`, et la
production ne la voit qu'à son prochain lancement.

## Les règles propres à cette migration

En plus des sept garde-fous du doc 32 :

1. **Le système entre dans `www/` par copie vérifiée.** `www/tokens.css` et `www/components.css`
   sont des copies de `systeme/project/tokens.css` et `components/bundle.css`, écrites par
   `node tools/design/ds-export.mjs --www` (écrit par la session design le 13/10) ; le filet
   (`tools/www-check/check.mjs`) appelle `ds-export.mjs --check`, qui n'écrit rien et échoue si
   une copie manque, a été retouchée ou retarde sur sa source. Une valeur ne se corrige jamais
   dans `www/`.
2. **Pas de nouveau style hors du système.** Une étape qui a besoin d'une règle absente l'ajoute
   au système (`bundle.css` et une fiche), puis la recopie. `style.css` ne fait que rétrécir.
3. **Remplacer, ne pas doubler** (garde-fou 7). Un composant nouveau remplace l'ancien dans le
   même commit, ou dans le commit suivant de la même étape, jamais plus tard.
4. **Les 43 clés du navigateur restent** (garde-fou 6), avec leurs valeurs : `frequencyVolumes`,
   `listeningFrequencies`, `phaseFilters`, `statusFilters`, `minAltitude`, `maxAltitude`,
   `mainView`, `parAirport`, `parCollapsed`, `showLocalDates`, `listSort`, `lostListSort`,
   `activeListSort`, `sidebarAircraftTab`, `lastSeenMinutes`, `voiceOnly`, `voiceBubbles`, `eventSounds`,
   `eventLog`, `showAirAircraft`, `showGroundAircraft`, `excludeOtherAirportsGrounded`, les
   dix-huit de la carte (`mapStyle`, neuf `show*`, six `*Opacity`, `trailLength`, `tracksLimit`)
   et les trois de l'animation. Relevé sur `origin/main` le 13/10. `sidebarAircraftTab` est le
   choix Active / Signal lost de la liste : elle reste dans l'onglet Traffic. Une clé dont le sens
   changerait se **lit** sous son ancien nom.
5. **Chaque étape visible se termine par une soirée d'écoute** du propriétaire (garde-fou 5) avant
   l'étape suivante sur le même terrain.

## Les mesures qui jugent chaque étape

| Mesure | Outil | Ce qu'elle tranche |
|---|---|---|
| Filet | `node tools/www-check/check.mjs` | références du store, scripts, tests PAR, classes Tailwind ; et, nouveau, copies du système identiques à leur source |
| Écran de référence | capture de l'instance d'essai à 1 440 × 900 (375 × 812 au téléphone), côte à côte avec `ecrans/<écran>.html` | disposition, composants, états ; l'écart se nomme ou se corrige |
| Fonctions | la table de `09-ecrans.md` | chaque fonction déplacée est atteignable à sa nouvelle place, en autant de clics ou moins |
| Rendu | `tools/design/contrast-probe.js` à 1 440 et 375 | texte sous 11 px : 0 ; sous AA : 0 hors désactivé ; cibles sous 24 / 44 px : 0 |
| Inventaire | `node tools/design/inventory.mjs` | couleurs en dur, tailles, classes : le compte baisse, jamais ne remonte |
| Animation | compteurs avant / après, 80 à 110 avions (garde-fou 4) | pour les étapes qui touchent la carte |
| Usage | une soirée d'écoute | ce qu'aucun chiffre ne dit |

Chaque étape donne ses chiffres avant / après dans son message de commit et dans le doc 32.

## Les étapes

Effort en jours de session (j), vérification comprise. Chaque étape se livre seule, se vérifie
seule et se défait par `git revert`.

### 0. Préparer — 0,5 j

- **Change** :
  - une configuration d'essai dont `static_dir` désigne le `www/` d'un arbre de travail ;
  - la sonde de rendu, rejouable sans copier-coller (signet ou page de `tools/design/`) ;
  - l'appel de `ds-export.mjs --check` dans le filet (le script et ses deux modes existent depuis
    le 13/10).
- **Ne change pas** : rien à l'écran, rien en production.
- **Vérifier** : l'instance d'essai sert l'arbre de travail (commentaire témoin dans `index.html`).

### 1. Le système dans `www/`, à comportement identique — 1 j

- **Change** :
  - `www/tokens.css` et `www/components.css` chargés avant `style.css` ;
  - le `theme.extend` de Tailwind reçoit les jetons (`surface-panel`, `text-secondary`…) qui
    pointent vers les variables ;
  - les huit noms d'aujourd'hui (`background`, `panel`, `text`, `highlight`…) restent, rebranchés
    sur les mêmes valeurs.
- **Ne change pas** : rien à l'écran ; aucune classe `cl-` n'est encore employée.
- **Vérifier** : styles calculés identiques sur les 280 combinaisons balise + classes ; capture
  avant / après identique.
- **Risque** : nul.

### 2. La carte et la PAR : cinq familles, rampe d'altitude — 1,5 j

Indépendante de la disposition, elle passe en premier.
- **Change** :
  - `phaseColorMap` (`map/renderers/aircraft-webgl.js`), `PHASE_COLORS` (`par/par-view.js`),
    `getPhaseColorClass` (`store/sidebar.js`) : cinq familles ; badges pleins pour T/O et T/D ;
  - `getAltitudeBandColor` : la rampe de clarté (`color-altitude-dark-*` ; `-light-*` sur les fonds
    IGN et OSM, dont la palette corrigée le 12/10, `46dc425`, est gardée).
  - le texte des étiquettes de la carte : `color-text-primary`, `color-text-tertiary` au signal
    perdu ; seul le préfixe de phase prend la couleur de sa famille ; le vert ne reste qu'aux
    anneaux de l'avion choisi et de celui qui vient de parler (`MapLabel` ; écart relevé à la
    relecture du 13/10 : `aircraft-webgl.js` peignait en vert toute étiquette d'avion actif).
- **Ne change pas** : les dix filtres de phase et leurs raccourcis 1 à 0, la clé `phaseFilters` ;
  les seuils des bandes d'altitude ; la géométrie de la PAR.
- **Référence** : `par.html`, la carte de `a-maintenant.html`.
- **Vérifier** : `tokens.mjs` (distance minimale entre familles 23,8) ; le même avion repéré sur la
  carte, la liste et la PAR ; tests PAR ; compteurs d'animation.

### 3. La barre du haut — 1 j

- **Change** : `TopBar` : le signe et le nom, ce qui est à l'antenne, les voyants (transcription,
  ADS-B et messages par seconde), l'heure UTC et l'heure locale, la cloche et l'engrenage. Ils
  quittent la carte (coin haut gauche) et la barre du bas. Le message du récepteur (prêté, éteint,
  en bascule : `store.receiverMessage`), qui tient aujourd'hui la place de la barre radio vide,
  rejoint « ce qui est à l'antenne » : sans cela il disparaîtrait à l'étape 7. Le choix UTC / locale passe dans
  Réglages › Display (décision 4), même clé `showLocalDates`.
- **Ne change pas** : la barre du bas garde, pour l'instant, les fréquences, la météo et la piste
  en service.
- **Référence** : la barre de `a-maintenant.html`.
- **Vérifier** : chaque voyant dit son état par forme et mot ; le signe passe à l'état « à
  l'écoute » quand une fréquence joue.

### 4. La colonne : l'onglet Traffic — 1,5 j

- **Change** : la barre latérale devient `cl-app__column` avec ses onglets (`Tabs`,
  `cl-tabs--column`) ; l'onglet Traffic reprend la liste en `AircraftStrip`, la recherche, la
  bande d'altitude avec histogramme, préréglages et QNH, Air / Ground / Heard, les familles,
  Active / Signal lost et sa rétention, le tri, les badges LP et S ; la carte affiche « Showing N of M »
  quand un filtre est actif.
- **Ne change pas** : la logique des filtres et leurs clés ; la fiche avion (elle bouge à l'étape 8).
  « Right now » est un onglet vide qui dit ce qu'il contiendra.
- **Référence** : `a-trafic.html`.
- **Vérifier** : liste de 150 avions sans débordement à 380 px ; raccourcis A, G, 0 à 9.

### 5. La colonne : Right now, et le repli (B) — 1,5 j

- **Change** : l'onglet par défaut « Right now » : `SummarySentence` (piste en service et depuis
  quand, source ADS-B dite ; le « depuis quand » n'existe que par avion aujourd'hui : la session
  atc-scribe l'expose au niveau de l'aérodrome dans `runway_in_use`, depuis le suivi des pistes,
  ≈ 1 h serveur), `ArrivalSequence` (`store.arrivals`), `EventList`
  (`events.recent(30)`), `Weather` (METAR lu en clair, TAF et NOTAM en boutons, décision 3), les
  compteurs. Le bouton » replie la colonne (disposition B) : « Right now » devient une carte sur le
  ciel.
- **Retire** : METAR, TAF, NOTAM et la piste en service de la barre du bas.
- **Référence** : `a-maintenant.html`, `b-replie.html`.
- **Vérifier** : la phrase de synthèse suit un changement de piste réel sur l'instance d'essai ;
  l'état replié est gardé au rechargement (nouvelle clé, aucune renommée).

### 6. Le fil — 2 j

- **Change** : la colonne de gauche : `ThreadHead` (boutons de fréquence en deux parties, « Mute
  all », filtre « Show », recherche) et le fil de tous les échanges (`Exchange` : locuteur,
  destinataire, force des indices, transcription, consignes, explication française par
  `plainFrench`, réponse du pilote, lecture) ; « quelqu'un parle » par le niveau audio ;
  « ↓ N new » quand le fil est remonté (décision 2) ; le menu de chaque fréquence avec son volume
  (décision 1), clés `frequencyVolumes` et `listeningFrequencies`.
- **Rebranche, avant que la barre parte** :
  - les bulles de la carte (`voice-bubbles-feature.js:42`) et les liens de la chronologie, qui
    ouvrent aujourd'hui la fenêtre d'une fréquence par `openTranscriptionAt(frequencyId, id)` :
    ils font défiler le fil jusqu'à l'échange et le surlignent ;
  - le niveau audio : `audio-client` dessine dans la barre de la tuile
    (`.audio-visualizer-container`, `index.html:2056`) ; `ThreadHead` héberge cet élément ou
    lit le niveau par le store, sinon « quelqu'un parle » perd sa source à l'étape 7.
- **Ne change pas** : la barre du bas reste encore, pour comparer pendant une soirée.
- **Référence** : `a-maintenant.html`, `a-frequence.html`.
- **Vérifier** : un échange arrive dans le fil au même moment que dans l'ancienne fenêtre ;
  écouter, couper, régler le volume, filtrer, chercher, rejouer ; une émission en français garde
  `lang="fr"`.

### 7. Retirer la barre du bas — 0,5 j

- **Retire** : la barre radio et les fenêtres de transcription par fréquence, avec leurs règles de
  `style.css` et le recalcul de leur disposition au redimensionnement et au défilement de
  `.radio-strip` (`app.js:878-879`) (garde-fou 7).
- **Vérifier** : la table de `09-ecrans.md`, lignes de la barre radio, toutes cochées ; le filet.

### 8. Suivre un vol — 2 j

- **Change** : choisir un avion remplace le contenu de la colonne par `AircraftStory` : l'en-tête,
  les onglets Story, Details, Track, Nearby (les anciens Timeline, Details, Tracks, Proximity, au
  contenu inchangé), « ‹ Back to Traffic », ✕ « Stop following » dans la barre du haut. La carte se
  centre sur lui avec sa trace ; son approche s'affiche dans le cadre PAR sous la carte. Le
  tableau des consignes entendues remplace « Status » par « Then », qui décrit ce que l'avion a
  fait, sans verdict (décision 6). Aujourd'hui le statut vaut toujours « issued » : rien n'écrit
  « complied » ni « deviation » (`grammar_processor.go:580`). « Then » se construit dans la page
  à partir de la trace de l'avion après l'heure de la consigne (`/aircraft/{hex}/tracks`) et de la
  valeur de la consigne ; pour ne pas relire l'anglais des consignes en clair, le serveur ajoute
  `value` et `unit` à chaque consigne (≈ 1 h, sans rupture), puis ≈ ½ j dans le store.
- **Ne change pas** : les données de la fiche, les rafraîchissements toutes les 5 s, Échap pour
  désélectionner.
- **Référence** : `c-suivre.html`, `c-details.html`.
- **Vérifier** : suivre, revenir, en suivre un autre, depuis la liste, la carte, la PAR et le fil ;
  le mot « deviation » n'apparaît plus.

### 9. Fenêtres et pages — 1,5 j

- **Change** : les calques de la carte (`MapControls`), les fiches d'aérodrome et de balise
  (`KeyValues`), le journal (`cl-menu` : six sortes, sons, effacer), la page des réglages
  (`SettingsPage` : Listening, Callsign matching, Display, Account, System), la connexion et le
  premier lancement (`SignIn`, variante « ciel », palette propre de `login.js` retirée), avec la
  console de la station (`StationVitals`) : une route publique qui ne sert que des chiffres
  agrégés, figés par un cache serveur de 10 s (aucune requête SQL par visite), et un réglage
  dans Account pour la couper. Chiffré par la session atc-scribe le 13/10 : ≈ 2 h 30 serveur ;
  trois valeurs sont à fabriquer (l'appareil de calcul dans le `/health` du sidecar, le temps
  moyen par transmission côté Go, la version et le commit à la construction) ; un test vérifie
  qu'aucune position, aucun indicatif, aucune fréquence nommée ne sort.
- **Ne change pas** : les réglages eux-mêmes, leurs routes d'API, l'enregistrement automatique.
- **Référence** : `a-calques.html`, `a-journal.html`, `reglages.html`, `connexion.html`,
  `premier-lancement.html`.

### 10. États et avis — 0,5 j

- **Change** : `ErrorBanner` à la place de l'écran « Connection Lost » (la carte reste utilisable),
  le fil vide quand rien n'est à l'antenne, l'avis « Click anywhere to resume listening ».
- **Référence** : `etats.html`.
- **Vérifier** : couper le flux de l'instance d'essai ; recharger sans cliquer.

### 11. Clavier, focus, mouvement — 1 j

- **Change** : anneau de focus sur `:focus-visible` ; les `<div @click>` deviennent des
  `<button>` ; `aria-label` sur les boutons-icônes ; `prefers-reduced-motion` ; Tab rendu au
  navigateur, J / K et ↓ / ↑ pour passer d'un avion à l'autre (choix du 13/10) ; les raccourcis
  A, G, 0 à 9 vérifient les touches de modification (Cmd-A ne bascule plus le filtre).
- **Vérifier** : un parcours entier sans souris ; « Réduire les animations » de macOS coché.

### 12. Téléphone — 1,5 j

- **Change** : `PhoneShell` : un volet à la fois, onglets Radio (accueil), Sky, Traffic,
  Right now (décision 5) ; tailles tactiles (44 px, lignes de 56 px).
- **Référence** : `telephone.html`.
- **Vérifier** : sonde à 375 px (cibles sous 44 px : 0) ; une soirée sur l'iPhone par Tailscale.

### 13. Nettoyage — 1 j

- **Retire** : les règles de `style.css` et les classes Tailwind que plus rien n'emploie, les
  restes de `text-text/xx` et de `#4ADE80`, les couleurs en dur.
- **Vérifier** : `inventory.mjs` (couleurs distinctes, valeurs en dur, tailles) contre
  `01-inventaire.md` ; la sonde sur les quatorze écrans de l'instance d'essai.

## Effort

| Étapes | Effort |
|---|---:|
| 0 et 1 (préparer, sans rien changer à l'écran) | 1,5 j |
| 2 (carte et PAR) | 1,5 j |
| 3 à 7 (la nouvelle structure) | 6,5 j |
| 8 à 10 (suivre, fenêtres, états) | 4 j |
| 11 à 13 (clavier, téléphone, nettoyage) | 3,5 j |
| **Total** | **≈ 17 j**, plus une soirée d'écoute après chaque étape visible (2 à 12) |

Ce que l'estimation ne couvre pas : une reprise demandée après usage. Elle est probable sur le fil
et la colonne, que le propriétaire regarde le plus ; une demi-journée par étape visible mène à
≈ 22 j au pire. Le serveur ne bouge que pour ≈ 4 h 30 : le « depuis quand » de la piste en
service (étape 5, ≈ 1 h), la valeur des consignes (étape 8, ≈ 1 h) et la route publique de la
console de connexion (étape 9, ≈ 2 h 30). Tout le reste est dans le store depuis le 13/10.

Relu côté code par la session atc-scribe le 13/10 : l'ordre 0 → 13 tient ; ses corrections sont
intégrées (clé `sidebarAircraftTab`, « depuis quand », « Then », message du récepteur, bulles de
la carte, niveau audio, recalcul des fenêtres).

## La revue de design, à chaque étape

La session atc-scribe livre l'étape sur l'instance d'essai et le signale ; la session design
compare à l'écran de référence (capture côte à côte) et aux mesures, puis répond « conforme » ou
nomme les écarts, avant la soirée d'écoute du propriétaire. Un écart volontaire (une contrainte du
code que l'écran ignorait) se consigne dans `09-ecrans.md` et l'écran se met à jour.

## Hors de ce plan

- Les traductions de l'interface (en / fr) : décision à part.
- Les fonds de carte eux-mêmes (IGN, OSM, « None ») : ils restent ceux de la session atc-scribe.
- La forme des volets Track et Nearby : leur contenu est repris tel quel, dans les composants du
  système.
