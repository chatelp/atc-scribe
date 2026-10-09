# Audit UX de l'interface web (09/10/2026)

**Méthode.** Lecture de `www/` (index.html 2 656 l., app.js 6 040, style.css 935, par-view.js 745, audio-client.js 542, atc-chat.js 1 106,
aircraft-animation.js 695, login.js 219, map/ 4 304). Mesures dans un navigateur sur `www/` servi en statique, viewport émulé 390×844 puis
1440×900, avec 7 fréquences injectées dans le store ; base du 04/10 (5 512 transcriptions, 13 306 changements de phase) ; station lue sans rien
changer. **Non mesuré** : fluidité réelle (FPS) sur trafic vivant, audio sur iOS. Le dépôt n'a pas été modifié.

## 0. Chiffres
- index.html statique : 55 `<button>`, 58 `<input>`, 6 `<select>`, 15 interrupteurs ; 7 zones d'écran ; 5 surcouches plein écran ou modales
  (accueil, connexion perdue, connexion, première configuration, avion simulé) + 3 popups météo + 1 popover de transcription par fréquence.
- 45 clés `localStorage` lues (app.js, par-view.js) ; 39 clés dans `settings` (app.js:378-470) ; 22 réglages côté serveur.
- 97 textes de 7 à 10 px dans index.html (15 à 7 px, 14 à 8 px, 29 à 9 px, 39 à 10 px) ; 0 attribut `aria-` ; 0 `@media` dans style.css.
- Chargement : 20 balises `<script>` dans `<head>`, une seule en `defer` (index.html:15-84) ; 32 requêtes propres = 660 Ko non compressés, plus ol.js 857 Ko (235 Ko gzip), Font Awesome
  259 Ko, Tailwind en CDN (compilé dans le navigateur), Alpine ; 4 hôtes externes pour le code, sans quoi la page ne s'affiche pas.

## 1. Inventaire (E essentiel, U utile, N inutilisé chez nous, C cassé ou douteux)
### 1.1 Écran, zone par zone
**Barre radio** (index.html:2093-2550, 458 l., 80 px de haut)
- E Tuile de fréquence (2100-2161) : nom, MHz, haut-parleur, barre de niveau, compteur. 260 px minimum chacune (2107).
- E Popover de transcriptions par fréquence (2163-2255) : 400 px de haut, largeur de sa tuile (app.js:4737), recherche, ligne « New above ». U : `@indicatif` cliquable.
- N/C Tuile « AI Advisory » (2259-2371, 113 l.) + atc-chat.js (1 106 l.) : `[atc_chat] enabled = true` avec `openai_api_key = ""` (config.toml:527-530) ;
  affiche « Disconnected » en permanence et un `setInterval` à 500 ms ; clic non testé, vraisemblablement en échec.
- U METAR / TAF / NOTAM (2373-2472) : Windy fonctionne pour LFPO (3 requêtes réussies toutes les 10 min dans les journaux). Mais placés en bout de barre : voir §2.
- E Pistes en service (2474-2524). U Horloges UTC/locale (2526-2550).
**Colonne droite, 480 px** (index.html:1127)
- U Liste « Tracked Targets » (1127-1493) : 19 contrôles avant la première ligne (recherche, altitude min/max, Air, Ground, « Heard on the radio »,
  10 filtres de phase en 10 px avec chiffre-raccourci en 7 px, onglets Active / Signal Lost, rétention, tri). E « Heard on the radio » (1200). N altitude min/max.
- E Fiche avion (1495-2090) : bandeau, onglet DETAILS avec section Radio (1978, nôtre). U Phase History (1863), Tracks (1692, seconde carte OpenLayers par sélection).
  N Issued Clearances (1920 : seulement décollage/atterrissage chez nous), PROXIMITY (2031), Simulation Controls (1593-1658).
- C Aucun bouton pour fermer la fiche : Échap ou clic sur une zone vide de la carte (interactions-feature.js:116). Sans clavier et avec une carte à 0 px (§2), impossible.
**Carte et PAR**
- E Carte `#map`, commutateur Radar / PAR / aéroports (828-841), vue PAR (par-view.js, 745 l. ; largeur minimale 480 px ligne 399 ; aide au survol, en anglais).
- U Menu « Map Controls » (843-1020) : 27 contrôles (6 fonds, Labels, Tracks, 9 couches à curseur d'opacité).
- C Fonds `vfr-sectional` (défaut, app.js:380), `ifr-low`, `ifr-high` : tuiles FAA. Mesuré : 404 sur Paris au zoom 8 pour les trois, 200 sur le Colorado.
- N NEXRAD, NOAA Infrared, NOAA Radar (934-968) : couvertures américaines (openlayers-map-manager.js:175-211).
- C Barre d'alertes (801-820) : voir §1.3.
**Surcouches**
- C Accueil « Start Monitoring » (1035-1120) : 86 lignes, 1 clic obligatoire à chaque chargement, son d'accueil, textes de démonstration (« modern air traffic control operations »).
- U Connexion perdue (1022). E Connexion (login.js, 1 compte, 1 champ utilisateur, 1 mot de passe). N Modale « Create Simulated Aircraft » (2552-2650, 6 champs).

### 1.2 Le volet de réglages (roue dentée, `#settings-panel`, index.html:98-797)
Ce n'est pas une modale : un volet de 320 px qui glisse sur la carte, replié au départ (app.js:318), 700 lignes, 13 boutons, 22 champs, 4 listes.
Mesuré : 1 392 px de défilement pour 688 px visibles, **sans** la section Serveur chargée. Ordre actuel, de haut en bas :
1. Display (112) : lissage on/off, FPS 10-60. Amont, N.  2. General (141) : « Tracks Limit » 100-3 600, « Hide Remote Ground Aircraft ». Amont, U/C (sans explication).
3. ADS-B Source (170-266) : 0 réglage, 14 lignes en lecture seule, 7 de plus pour le décodeur. Amont. U : c'est l'état de la station, mais enterré.
4. Debug (269-378) : bouton Pause WebSocket + 25 compteurs de performance ; **542 px de haut, 39 % du défilement**. Amont, N.
5. Station (382-491) : surcharge de position (2 champs placeholder Toronto 43.6777 / -79.6248, 4 boutons, GPS auto, intervalle). Amont, N : station fixe.
6. Simulated Aircraft (494-541) : bouton + liste + corbeille avec `confirm()`. Amont, N.
7. **Server (547-797) : tout ce qui sert, en dernier, à ~1 300 px du haut** : journal, rétention, aéroport de référence, « Also follow » (3 max),
   appariement (9 règles + 8 seuils de secteur = 17 contrôles, presque tous des essais mesurés ; `one_digit_off` est « for experiments only »),
   accès, puis 7 lectures (base, croissance, disque, journal, sidecar, uptime). Nôtre, E. **22 réglages éditables.**
Constats : 5 sections sur 7 viennent de l'amont et ne servent pas ; les réglages d'expérience (17) ont le même poids visuel que l'aéroport de référence ;
les réglages du navigateur (45 clés localStorage) et ceux du serveur (22) ne sont séparés que par un commentaire HTML (543) ; textes de 12 px sur 320 px.
**Page proposée** (`/reglages`, page statique, même Alpine, réutilise `serverSettings()` app.js:5813, une colonne de 640 px, lisible au téléphone) :
Écoute (aéroport de référence, aéroports suivis, fréquences à l'antenne en lecture seule) ; Appariement (préréglage Mesuré / Prudent / Expérimental,
détail replié : une ligne claire par règle avec son effet mesuré, tableau 4×2 des secteurs) ; Affichage (étiquette « ce navigateur » : fond, couches,
traînées, tri, UTC) ; Compte (mode, mot de passe, **Se déconnecter**, absent aujourd'hui : 0 occurrence de logout dans www/) ; Système (lecture seule :
sidecar, ADS-B, base, disque, journal, uptime ; seuls niveau de journal et rétention éditables). Suppression : Display, General, Debug, Station, Simulated.

### 1.3 Les alertes (barre `#alerts-bar`, en haut au centre de la carte)
- Affichage : cadre de 40 px sur la moitié de la largeur de la carte, z-index 10 000, « Alerts: None » en permanence ; pastilles cliquables (clic ferme,
  **clic droit** sélectionne l'avion, sans équivalent tactile), durée de vie 60 s (`setTimeout`), croix pour tout effacer.
- Événements : (a) `phase_change`, une pastille par changement de phase de tout avion (app.js:3559, 4575) ; (b) `status_update` « signal_lost » (hors CRZ et sol, après 60 s,
  app.js:3700, 4499) ; (c) `clearance_issued` (3579) qui appelle `this.addAlert` (3616), **défini nulle part dans www/** (grep) : alerte cassée à chaque clairance, à confirmer en console ;
  (d) mouvements `aircraft` (websocket-client.js:176, app.js:3693, 4392) : aucun émetteur côté Go, code mort. Classes CSS `alert-takeoff|landing|approach|clearance` absentes de style.css.
- Volume (04/10, 18 h) : 13 306 changements de phase, soit 740/h (12 par minute, donc une douzaine de pastilles vivantes). UNK 3 952, CRZ 3 726, NEW 1 939 :
  **72 % de bruit** ; T/O + T/D : 1 079 (8 %, 60/h). Le PAR et la liste montrent déjà la phase en couleur.
- Code : 418 lignes (app.js 299 + 64 + 35, index.html 20). **Proposition : supprimer.** Si un signal est voulu, un compteur T/O et T/D de l'heure près des pistes en service.

### 1.4 Le déplacement des avions (à ne pas trancher)
- Source : readsb publie `aircraft.json` **1 fois par seconde** (mesuré le 09/10 : `now` +1,0 s à chaque lecture ; 84-85 avions, 69-70 positionnés ; âge médian
  de la dernière position 1,2 à 1,8 s). co-atc l'interroge toutes les secondes (config.toml:69) et diffuse en websocket un delta par avion modifié
  (service.go:360-385) **plus** une position extrapolée côté serveur chaque seconde (`aircraft_predicted_state`, service.go:56-57, 392-430, confiance ≥ 0,65).
- Navigateur : `AircraftAnimationEngine` (aircraft-animation.js, instancié app.js:5760), boucle `requestAnimationFrame` plafonnée à 30 ips (ligne 24) ; vitesse déduite des
  deux dernières positions (438) ; extrapolation sur au plus 3 s, **multipliée par 0,72** (25-27, 304-361), donc l'icône n'atteint jamais la position extrapolée
  (retard calculé ≈ 0,28 s, soit ≈ 65 m à 450 kt ; non mesuré dans le navigateur) ; recalage lissé 180-900 ms au-delà de 0,016 NM (≈ 30 m) ;
  600 états maximum, budget 14 ms par trame, qualité adaptative ; rendu WebGL (aircraft-webgl.js, 990 l.).
- À retenir : deux extrapolations empilées (serveur, navigateur) et un lissage de recalage, sur une donnée à 1 Hz déjà âgée d'environ 1,5 s ; à 450 kt, 230 m entre deux positions. Le navigateur n'est pas seul en cause.
  Les compteurs existent (Debug, ligne 269) mais ne servent pas à mesurer la latence de bout en bout (instant d'observation contre instant de dessin).

## 2. Frictions
- **Téléphone, 390 px (mesuré) : inutilisable.** Carte 0 px de large (colonne droite fixe de 480 px, `w-120`, index.html:1127) ; accueil de 482 px dans 390 ;
  document de 514 px (124 px coupés, `overflow-hidden`, pas de défilement horizontal) ; barre radio : 7 tuiles jusqu'à x = 1 980 px, barre de 2 494 px, non défilable
  (`scrollLeft` reste à 0). Aucun `@media` ; la seule règle responsive est `md:` sur l'accueil (1061). Survol seul pour l'aide du PAR (par-view.js:190), les alertes (clic droit)
  et la mise en évidence des lignes. Pas de manifeste (pas d'app sur l'écran d'accueil) ; sw.js ne met en cache que les tuiles Carto.
- **Mac 1440 px : la barre radio déborde (mesuré).** Avec 5 fréquences la barre fait 1 920 px : IA, météo, pistes en service et horloges passent au-delà de 1 440 px (mesure sans METAR chargé : en réel, plus large).
  Avec 7 : 2 tuiles coupées, barre de 2 494 px, **ni pistes en service, ni météo, ni heure visibles** ; `overflow: visible` partout, rien à faire défiler.
  Largeur mesurée 260 à 296 px par tuile avec des noms courts. Le nom des tuiles contient déjà la fréquence (`radio-ctl-sync` : « 124.350 Approche… »), répétée en dessous (« 124.35 MHz »).
- **Accueil, geste quotidien** : charger, attendre « Connected », 1 clic sur « Start Monitoring » (son d'accueil, 86 lignes de présentation). Ce clic relance l'écoute mémorisée
  (app.js:5353, D54), donc il est utile, mais il bloque l'écran et se rejoue à chaque F5. Session perdue au redémarrage du serveur (sessions en mémoire, auth/session.go:18-24) : + 3 gestes de connexion.
- **Le son** : 1 clic par tuile sur un navigateur neuf (le téléphone) ; pas de « tout écouter / tout couper » ; pas de volume (le code le prévoit, `userSetVolumes`, aucune commande).
- **Lire les transcriptions** : 1 clic par fréquence, un popover de 400 px de haut et de la largeur de la tuile (260-296 px), tous refermés au rechargement ; pas de fil unique multi-fréquences. Texte à 12 px, en-tête à 9 px.
- **L'avion identifié est presque invisible.** Le badge `@indicatif` n'existe que si `speaker_type == 'PILOT'` (index.html:2216). Base du 04/10 : 1 696 transmissions avec avion
  apparié sur 5 512 ; **seules 345 (20 %)** en affichent le badge ; 887 lignes ATC et 464 sans locuteur n'ont que « @ATC » ou rien (le gras du texte reste l'unique indice).
- **Densité** : 19 contrôles avant la première ligne de la liste ; filtres de phase de 10 px ; Tab détourné pour passer d'un avion à l'autre (app.js:5488), raccourcis 0-9, A, G, Espace
  (push-to-talk) sans aide visible ; 33 `title=` seulement. Interface entièrement en anglais (`lang="en"`), propriétaire francophone.
- **Défaut d'un navigateur neuf** : fond FAA (404 sur Paris) = carte vide, tout coupé, panneaux repliés.

## 3. Ce qui manque pour notre usage
- **Quelles fréquences sont à l'antenne, et pourquoi.** La station sait (`/radio/etat`, lu le 09/10) : « A l antenne », sélection libre « 124,350 + 126,425 »,
  centre 125,3875, depuis 11:35, dernier groupe `en-route-et-descente-cdg`. co-atc n'en montre rien : deux tuiles, sans titre, sans durée, sans mode libre/groupe, sans langue attendue
  (la langue existe dans la source poussée par `radio-ctl-sync`). Si le récepteur est prêté ou éteint, `radio-ctl-sync` retire toutes les sources et la barre se vide **sans message** (aucun état vide dans index.html).
- **État de la station** sur l'écran principal : source ADS-B, messages/min, SNR existent (index.html:170-266) mais dans la roue dentée. « Connection Lost » ne couvre que le websocket, pas la chaîne station.
- **État du sidecar** : `/api/v1/server` rend `status` (ok / degraded / unknown), `degraded[]`, seconde lecture ok/échecs (server_handlers.go:70-77) ; l'interface n'affiche qu'une ligne au bas du volet
  (la liste `degraded[]` n'est pas affichée). Le code le dit lui-même : un sidecar dégradé = « fewer aircraft identified with no stated cause ».
- **Par fréquence** : dernière transmission (compteur `--s` en 9,6 px dans la barre de niveau), transmissions par heure, part avec avion identifié (30,8 % toutes fréquences le 04/10), langue lue.
- **Écoute** : volume par fréquence, « tout écouter », indicateur d'activité (squelch) lisible de loin.

## 4. Dix changements, par priorité (≈ 50 h au total ; la disposition générale ne change pas)
1. **Supprimer l'inutilisé** : tuile IA + atc-chat.js, simulation (volet, fiche, modale), NEXRAD/NOAA, Debug, Station, PROXIMITY, curseur FPS, altitude min/max.
   Gain : −662 lignes d'index.html (25 %) et −1 106 (atc-chat.js) ; 18 des 55 boutons et 24 des 58 champs (comptés dans les plages supprimées). 5 h. Le code mort d'app.js : voir frontend-code.md.
2. **Barre radio : 7 tuiles visibles sur 1440 px.** Tuile de ~150 px (nom sans la fréquence répétée, MHz, haut-parleur, niveau), barre défilable ; pistes en service et heure fixées à droite.
   Gain : plus rien d'inaccessible. 3 h. Supprime la duplication du nom et 260 px par tuile.
3. **Supprimer la barre d'alertes** (418 lignes, 72 % de bruit, clairance cassée). Gain : carte dégagée, un bogue en moins. 1,5 h.
4. **Page de réglages** (§1.2) : 12 h. Gain : l'aéroport de référence et l'appariement à 2 clics au lieu de 1 300 px de défilement ; supprime 700 lignes de volet.
5. **Accueil : bandeau « Reprendre l'écoute » au premier clic n'importe où** (le geste exigé par le navigateur), sans écran bloquant ni son d'accueil. Gain : 0 clic dédié. 2 h. Supprime 86 lignes + 1 mp3.
6. **Avion identifié sur toutes les lignes appariées** (badge `@indicatif` aussi pour ATC et sans locuteur : 20 % → 100 % des lignes appariées) ; corps ≥ 11 px
   pour transcriptions et liste. Gain : le besoin n°1, lire la radio avec l'avion. 2 h.
7. **Fond de carte par défaut utilisable en France** (sombre ou OSM) ; retirer les 3 fonds FAA. Gain : carte non vide sur un navigateur neuf. 1 h.
8. **Téléphone** (sous 768 px seulement : colonne en pleine largeur, carte / PAR / liste en onglets, barre radio défilable, fiche avec bouton fermer, aide au toucher). Gain : usage par Tailscale. 10 h.
9. **Bandeau d'état** (un point par élément) : sélection à l'antenne et depuis quand, sidecar (statut + `degraded`), ADS-B, message « récepteur prêté ou éteint ». Gain : §3. 7 h
   (4 h interface, 3 h `radio-ctl-sync` pour pousser le libellé de la sélection).
10. **Alléger le chargement et mesurer l'animation** : Tailwind compilé une fois, ol/Alpine/Font Awesome servis par co-atc (plus d'hôte externe), scripts en `defer` ;
    ajouter la mesure de latence de bout en bout (instant d'observation contre dessin) avant toute décision sur le moteur. 6 h.

## Résumé
- Le téléphone est cassé (carte 0 px à 390 px, page de 514 px) et la barre radio déborde dès 5 fréquences sur 1440 px : les pistes en service, la météo et l'heure sortent de l'écran.
- 5 des 7 sections de réglages, la tuile IA, la simulation, trois fonds FAA et trois couches américaines ne servent à rien ici ; les 22 réglages utiles sont au bas d'un volet de 1 300 px.
- Les alertes (418 lignes) sont à 72 % du bruit, et celle des clairances est cassée : à supprimer. L'avion identifié n'est affiché que sur 20 % des lignes appariées.
- Les avions bougent sur une donnée à 1 Hz âgée d'environ 1,5 s, avec deux extrapolations empilées : mesurer la latence de bout en bout avant de juger le navigateur.
- Dix changements, ≈ 50 h, sans toucher à la disposition ; les points 1 et 3 retirent 2 186 lignes (662 + 1 106 + 418), les points 1, 3, 4 et 10 répondent aux quatre griefs du propriétaire (lourdeur, lenteur, réglages, alertes).
