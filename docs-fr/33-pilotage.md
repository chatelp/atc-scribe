# Pilotage — stratégie, chantiers, jalons, décisions attendues

*Le document du chef d'orchestre. Tenu par la session atc-scribe, mis à jour à chaque livraison
d'une session, à chaque décision du propriétaire et à chaque jalon franchi. Ouvert le 12/10/2026
à la demande du propriétaire. Le journal détaillé reste dans `32-audit-produit.md` (« État ») et
la mémoire des décisions dans `05-decisions.md` ; ici, la vue d'ensemble, courte, à jour.*

## La stratégie, en cinq lignes

1. **atc-scribe est un produit à part** (D71, 10/10) : plus un fork qu'on rebase, une interface
   à nous, ce qui ne sert pas se retire, ce qui est contribuable reste identifiable.
2. **Ce qu'il fait que les autres ne font pas** : lier ce qui se dit à ce qui vole, en local,
   sans service, avec le français en seconde lecture. Mesuré (95 % de précision d'appariement,
   D73) ; rien d'ouvert, local et apparié n'existe ailleurs (étude communauté, question 2).
3. **Le cap** : un outil installable par d'autres, en France d'abord, pour une communauté de
   radioamateurs, de passionnés et de simmers qui fournisse captations et annotations, vers un
   **modèle de transcription communautaire adapté au français** (étude communauté, 12/10).
4. **Le nom change** (D74) : « Scribe » est l'outil de la DSNA, contesté par le SNCTA. Le nouveau
   nom et le positionnement « espace aérien français » (Q53) sont à l'étude ; le propriétaire
   choisit.
5. **Règles du chantier** : ne rien casser de ce qui marche (sept garde-fous, doc 32), mesurer
   avant de conclure, interface utilisable par un novice, aides contextuelles partout, rien ne
   tourne en permanence sur le Mac sans décision.

## Les chantiers en cours (sessions parallèles)

| Session | Mission | État au 12/10 | Prochain livrable | Ce que j'en attends |
|---|---|---|---|---|
| **atc-scribe** (cette session) | le produit : code, mesures, documents, coordination | paliers 0 à 5 de l'audit faits ; bulles de dernière transmission ; retours de l'inventaire design traités (palette, 51 règles orphelines) | le renommage, dès le choix du nom | — |
| **Whisper-lab** (`~/Dev/Aero/whisper-lab`, privé) | reconnaissance : modèles, corpus, mesures, GPU | précision par niveau d'indices mesurée (borne haute) ; Kyutai écarté (28/09) | tirage au hasard de la précision quand l'annotation aura avancé (jalon du labo) | la mesure de non-régression du modèle converti, le jour du portage |
| **ATC-scribe communauté** (`~/Dev/Aero/atc-scribe-communaute`, privé) | cadre légal, existant, communautés françaises, prérequis, recommandation | questions 1, 2, 3 livrées | question 4 (prérequis techniques : Wyoming, Pi en différé, Windows pour l'affinage) puis 5 (le premier pas) et la synthèse | la liste hiérarchisée de ce que l'outil doit changer avant qu'on en parle à quiconque |
| **ATC-scribe design** (branche `design`, index `docs-fr/design/README.md`) | identité, nom, système de design, maquettes, plan de migration | **tout est livré sauf la marque** : inventaire (01), nom et positionnement (00), principes (02), système et 91 jetons (03, `jetons.json`), maquettes HTML, plan de migration (04, ≈ 13 j, par étapes vérifiables) | identité (marque, favicon, icône), dès le choix du nom | **les choix du propriétaire** : le nom, puis les six points du système listés ci-dessous ; après validation, la migration étape par étape avec cette session |
| **Station radio** (session de la station) | la station : réception, chaîne VHF, captations | en veille ; `radio-ctl-sync` rapporte l'état au bandeau | — | prévenue à chaque bascule d'écoute |

## Les jalons

### Faits

- Paliers 0 à 5 de l'audit produit (10–12/10) : retraits, réglages, journal d'événements,
  curseur d'altitude QNH/FL, réécoute, fiche de vol, séquence d'arrivée, consignes en clair,
  bandeau d'état, barre radio, bibliothèques locales, Tailwind compilé, `app.js` découpé,
  préréglages d'appariement mesurés (D73), preuve d'appariement (D72). Validé à l'usage par le
  propriétaire le 12/10 (« nette amélioration UX »).
- Bulles de dernière transmission sur la carte (12/10).
- Garde-fou 3 rebouché (12/10, trouvé par la session design) : la production sert une copie
  figée de `www/` prise à son lancement (`server.static_dir`), plus les fichiers en cours
  d'édition ; l'instance d'essai reste la vérification en direct.
- Trois études lancées en parallèle (12/10) ; cadre légal, existant, communautés, inventaire
  visuel livrés.

### En cours

1. **Le nom et le positionnement** (design, `00-nom.md`) → décision du propriétaire → renommage
   en une fois par cette session : dépôt GitHub, lanceur et dossier de configuration sur le Mac,
   titre de page (encore « Co-ATC »), documents, README.
2. **L'identité et le système de design** (design) → validation du propriétaire → migration par
   étapes, chacune livrable et vérifiée (filet `www-check`, styles calculés, captures).
3. **Les prérequis pour être installable ailleurs** (communauté, question 4) → liste
   hiérarchisée → chantier technique dans cette session.
4. **La recommandation d'un premier pas communautaire** (communauté, question 5) → décision du
   propriétaire ; aucune publication ni prise de contact avant.

### À venir, dans l'ordre probable (à confirmer par le propriétaire à chaque étape)

| Jalon | Dépend de | Décidé ? |
|---|---|---|
| Renommage | choix du nom | cible : oui (D74) ; nom : non |
| Migration du design | validation du système | non |
| Installation sans la station du propriétaire (sources audio et ADS-B génériques documentées, configuration d'exemple, README) | question 4 | principe : oui (mission communauté) |
| Version en/fr de l'interface | après le design (les textes bougent) | principe : oui (10/10) |
| **Portage du sidecar : Linux x86-64 + NVIDIA** (faster-whisper / CTranslate2, repli processeur ; Pi qui reçoit, transcription déportée) ; **Windows au moins pour l'affinage** (les simmers) ; sous-point pour un mode différé (Pi 5, dix minutes de retard) : apparier contre le ciel de l'instant de la transmission relu dans `adsb_targets`, comme le fait déjà l'outil de mesure, au lieu de la flotte vivante (modification bornée au point d'appariement) | question 4 ; format permettant l'affinage ; non-régression mesurée au labo | **cible : oui (12/10) ; pas dans l'immédiat** (Q54) |
| Corpus simulé (VATSIM / IVAO, sessions dédiées, accord écrit) et annotation communautaire | cadre légal (audio réel exclu) ; premier pas décidé | non |
| Affinage communautaire ou fédéré (modèle adapté au français) | corpus ; portage | non |
| Premier pas public (quoi, où, quand) | question 5 ; renommage ; installable ailleurs | non |

Plus tard, à discuter (doc 32) : strips de vol, clairance contre trajectoire, vue d'approche en
plan, procédures du SIA, fréquences comme un pupitre, programmation d'écoute.

## Les décisions attendues du propriétaire

1. **Le nom** — propositions prêtes le 12/10 (`docs-fr/design/00-nom.md`, branche `design`) :
   `5/5` (recommandé, générique, dépôt `five-by-five`), Entendu, En clair, Verticale, Lisible ;
   positionnement recommandé : le français porté comme une capacité, pas comme un territoire
   (même conclusion que l'étude communauté) (Q53).
2. **Le système de design** — tout est livré (`docs-fr/design/`, branche `design` : principes,
   système, jetons, maquettes, plan de migration ≈ 13 j). Six choix à trancher avant la
   migration : (1) monospace pour les phrases ou police du système (principe 8 ; le monospace
   est 1,26 fois plus large à taille égale) ; (2) cinq familles de phase à la place des dix
   couleurs ; (3) une rampe de clarté sans teinte pour l'altitude des avions ; (4) J / K au lieu
   de Tab pour passer d'un avion à l'autre ; (5) l'interpolation des avions gardée sous
   `prefers-reduced-motion` ; (6) la validation d'ensemble, maquettes à l'appui, qui ouvre la
   migration.
3. **Le premier pas communautaire**, sur la recommandation de l'étude (question 5).
4. **La position de la station dans le dépôt public** (alerte de la session communauté,
   12/10) : D11 (15/09) la garde à 5 décimales, soit le mètre, parce qu'elle est publique via
   FlightAware ; vue d'un projet communautaire, c'est une adresse. Présente dans cinq fichiers
   suivis (01-station, D11, une fiche cloud, un test Go, un script d'outil) depuis le premier
   commit du 15/09, 483 commits, aucun fork ni étoile. L'usage de l'écosystème, vérifié dans
   les sources le 12/10 : dump1090-fa arrondit à **2 décimales** en mode approximatif (« for
   privacy reasons »), mlat-server à 1/20 de degré avec décalage aléatoire, ADS-B Exchange à une
   grille de 5 milles ; FlightAware a un réglage de précision de la carte de couverture, que seul
   le propriétaire peut vérifier dans « My ADS-B » ; aucun outil ne publie à 5 décimales par
   défaut. À trancher : arrondir à 2 décimales, retirer IP privées et nom d'hôte des documents
   publics, réécrire l'historique ou non.
5. Le moment du **portage** (cible fixée, calendrier non).
6. Le **tirage au hasard** de la précision d'appariement (jalon du labo, lié à l'annotation).

## Les risques suivis

- **Brevet US12277862B1** (Aerlogics, 2025) : corrélation transcription ATC / ADS-B pour la
  conformité ; aucun dépôt européen trouvé, revendications non lues. À faire lire avant toute
  diffusion hors de France (Q53).
- **Cadre légal** : audio réel non partageable ; corpus simulé seulement avec sessions dédiées et
  accord écrit (les règles de VATSIM et d'IVAO ne valent pas consentement RGPD).
- **Le nom** : rien de public sous « atc-scribe » au-delà de l'existant tant que le nom n'est pas
  choisi.
- **Secrets et données** : jamais de clé, de mot de passe, d'audio ni de base dans un dépôt ou
  dans le cloud (règles permanentes).
- **Dettes à solder avant toute installation ailleurs** (vérifiées dans les sources le 12/10 pour
  la fiche 04 de l'étude communauté) : CORS renvoie toute origine avec les identifiants, le
  WebSocket accepte toute origine, aucun rôle (tout compte connecté réécoute l'audio) ; l'archive
  audio et les journaux bruts n'ont pas de purge ; Docker est mort (compile un seul fichier) ;
  pas de compilation Windows (trois sites). Sans gravité tant que D5 tient (réseau local
  seulement) ; bloquants le jour où quelqu'un expose le serveur.

## Comment ce document se tient

À chaque message d'une session qui livre, à chaque décision du propriétaire, à chaque jalon :
la ligne du chantier, le tableau des jalons et la liste des décisions attendues sont mis à jour
dans le même commit que le reste. Le propriétaire peut demander « où en est-on ? » : la réponse
est ce document.
