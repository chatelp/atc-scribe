# Pilotage — stratégie, chantiers, jalons, décisions attendues

*Le document du chef d'orchestre. Tenu par la session Clairance, mis à jour à chaque livraison
d'une session, à chaque décision du propriétaire et à chaque jalon franchi. Ouvert le 12/10/2026
à la demande du propriétaire. Le journal détaillé reste dans `32-audit-produit.md` (« État ») et
la mémoire des décisions dans `05-decisions.md` ; ici, la vue d'ensemble, courte, à jour.*

## La stratégie, en cinq lignes

1. **Clairance est un produit à part** (D71, 10/10) : plus un fork qu'on rebase, une interface
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
| **Clairance** (cette session) | le produit : code, mesures, documents, coordination | paliers 0 à 5 de l'audit faits ; bulles de dernière transmission ; retours de l'inventaire design traités (palette, 51 règles orphelines) | le renommage, dès le choix du nom | — |
| **Whisper-lab** (`~/Dev/Aero/whisper-lab`, privé) | reconnaissance : modèles, corpus, mesures, GPU | précision par niveau d'indices mesurée (borne haute) ; Kyutai écarté (28/09) | tirage au hasard de la précision quand l'annotation aura avancé (jalon du labo) | la mesure de non-régression du modèle converti, le jour du portage |
| **ATC-scribe communauté** (`~/Dev/Aero/atc-scribe-communaute`, privé, dépôt `chatelp/atc-scribe-communaute`) | cadre légal, existant, communautés françaises, prérequis, recommandation | **étude livrée le 12/10** : `SYNTHESE.md` et fiches 01 à 05 | — (session en veille ; reprend sur question de Pierre) | la décision de Pierre sur le premier pas (consultation d'avocat) ; le P0 de la fiche 04 entre dans le jalon « installable ailleurs » |
| **ATC-scribe design** (branche `design`, index `docs-fr/design/README.md`) | identité, nom, système de design, maquettes, plan de migration | **tout est livré** : inventaire (01), nom (00), principes (02), système et jetons (03), maquettes, migration (04, ≈ 13 j), identité (05, le scope radar, appliquée), esquisses de la structure (07, validées, D77), **système de composants (08, `docs-fr/design/systeme/`, artefact https://claude.ai/artifact/Ut6hWDRRUcWC58HHz6UP2K)** : classes CSS `cl-` avec états pour Alpine, 95 variables de `tokens.css`, chaque fiche nommant ses données (`store.arrivals`, `plainFrench`, `events.recent`, `messages_per_second`, `match_reason`) ; la PAR n'est pas redessinée, seule sa palette de phases change **Quatorze écrans finaux validés le 13/10 (D78, galerie https://claude.ai/artifact/2gZSBGbTkEySxThbNRYhMF)** avec six décisions d'interface (barre du bas supprimée, un seul fil, météo dans « Right now », horloges, téléphone à quatre onglets, « Then » sans verdict) | — | **le plan de migration réécrit le 13/10** (`04-migration.md`, 14 étapes, ≈ 17 j, chacune avec son écran de référence, ce qu'elle change et ses mesures ; relu côté code par cette session : quatre corrections et trois ajouts envoyés ; serveur ≈ 4 h 30 en tout : « depuis quand » de la piste en service, valeurs des consignes, et la **route publique des chiffres avant connexion** choisie par le propriétaire le 13/10 avec la page « le ciel » : agrégats seulement, cache de 10 s, coupable dans Account, trois valeurs à fabriquer : appareil du sidecar, temps moyen par transmission, commit du binaire), **validé par le propriétaire le 13/10** ; référence figée `design-reference-2026-10-13`, branche `design` versée dans `local` le 13/10 (171 fichiers sous `docs-fr/design/` et `tools/design/` ; les captures de la carte réelle restent hors dépôt, D75). **La migration commence par l'étape 0** ; la session design relit chaque étape sur l'instance d'essai avant la soirée d'écoute du propriétaire |
| **Station radio** (session de la station) | la station : réception, chaîne VHF, captations | en veille ; `radio-ctl-sync` rapporte l'état au bandeau | — | prévenue à chaque bascule d'écoute |

## Les jalons

### Faits

- Paliers 0 à 5 de l'audit produit (10–12/10) : retraits, réglages, journal d'événements,
  curseur d'altitude QNH/FL, réécoute, fiche de vol, séquence d'arrivée, consignes en clair,
  bandeau d'état, barre radio, bibliothèques locales, Tailwind compilé, `app.js` découpé,
  préréglages d'appariement mesurés (D73), preuve d'appariement (D72). Validé à l'usage par le
  propriétaire le 12/10 (« nette amélioration UX »).
- Bulles de dernière transmission sur la carte (12/10).
- **D75** (12/10) : position de la station à deux décimales, adresses du réseau domestique
  génériques, **historique git réécrit** (499 commits, quatre branches, poussé de force) ;
  sauvegarde complète d'avant sur le Mac ; reste la purge côté GitHub, à demander par le
  propriétaire.
- Garde-fou 3 rebouché (12/10, trouvé par la session design) : la production sert une copie
  figée de `www/` prise à son lancement (`server.static_dir`), plus les fichiers en cours
  d'édition ; l'instance d'essai reste la vérification en direct.
- Trois études lancées en parallèle (12/10) ; cadre légal, existant, communautés, inventaire
  visuel livrés.
- **D79** (13/10) : le mouvement des avions ne pulse plus. Deux défauts de l'amont révélés par
  la cadence de la station (une position toutes les ~1,7 s par avion, sondée chaque seconde) :
  prédiction serveur ancrée sur le relevé et non sur la position, moteur de la page qui
  re-datait le point figé. Mesuré avant/après dans l'onglet du propriétaire : 33 % → 3,3 % de
  pas à reculons. Commit autonome contribuable (70c8eff). Reste la liste Trafic qui vole
  ~100 ms par seconde au fil principal (étape 5 ou commit dédié).
- **Fiches H et I intégrées** (13/10 soir) : serveur sans transcription, Docker amont retiré ;
  CORS et WebSocket fermés aux origines étrangères, rôle lecture seule, écoute par le serveur
  réservée au propriétaire par défaut, purges réglables (à 0 chez nous). Le voyant
  « Transcription » connaît l'état Off.

### En cours

1. **Le nom et le positionnement** (design, `00-nom.md`) → décision du propriétaire → renommage
   en une fois par cette session : dépôt GitHub, lanceur et dossier de configuration sur le Mac,
   titre de page (encore « Co-ATC »), documents, README.
2. **L'identité et le système de design** (design) → validation du propriétaire → migration par
   étapes, chacune livrable et vérifiée (filet `www-check`, styles calculés, captures).
   Demande du propriétaire du 13/10, à traiter à l'étape de la carte avec la session design :
   **à faible zoom, les cartouches d'avion envahissent l'écran** ; il faut un affichage allégé
   (par exemple l'indicatif seul sous un seuil de zoom, la cartouche complète en zoomant). Consignée par la session design dans `09-ecrans.md` (§ Demandes ouvertes) : elle mesurera d'abord étiquettes et recouvrements par niveau de zoom sur l'instance, puis proposera une variante compacte de l'étiquette et son seuil.
3. **Les prérequis pour être installable ailleurs** (communauté, question 4) → liste
   hiérarchisée → chantier technique dans cette session.
4. **La recommandation d'un premier pas communautaire** (communauté, question 5) → décision du
   propriétaire ; aucune publication ni prise de contact avant.

### À venir, dans l'ordre probable (à confirmer par le propriétaire à chaque étape)

| Jalon | Dépend de | Décidé ? |
|---|---|---|
| Renommage | choix du nom | cible : oui (D74) ; nom : non |
| ~~Application de la marque~~ — **faite le 13/10** (D76, signe = le scope radar, choisi par le propriétaire sur une planche de huit) : favicon et manifeste, images de barre de menus, README ; puis le **troisième état « à l'écoute »** (favicon allumé quand une fréquence est écoutée, lanceur en couleurs quand la production tourne et l'antenne est à l'antenne). Restent pour la migration du design : la marque en tête de la connexion et dans l'en-tête de l'outil | — | fait |
| **Migration du design** (plan `04-migration.md`, 14 étapes, ≈ 17 j + 4 h 30 serveur) — **en cours depuis le 13/10** : étapes 0 à 4 versées dans `local` (système, familles de phase et palette F, barre du haut, onglet Traffic) ; **étape 5 livrée sur `migration` le 13/10 au soir** (« Right now » : phrase de synthèse, tableaux d'arrivée, événements, météo lue en clair, compteurs ; colonne repliable), **relue conforme par la session design après quatre corrections** (dont le sens des pistes, côté serveur), **versée dans `local` le 13/10 dans la nuit après le coup d'œil du propriétaire** ; **étape 6 (le fil) versée dans `local` le 14/10** ; **étape 7 (la barre radio quitte le grand écran, reste au téléphone jusqu'à l'étape 12) versée dans `local` le 14/10** ; prochaine : l'étape 8, suivre un vol (avec « Then », fiche K) (au passage, la barre du haut au téléphone corrigée sur `main`, 6a816f6) ; à l'étape 8, en branchant « Then » (fiche K) : la vitesse se dit « ground speed » (une consigne donne une vitesse air, `then.js` lit la vitesse sol ; sans le mot, l'écart se lirait comme un verdict), test des formes de K à ajuster ; chaque étape : commit sur `migration`, relecture par la session design sur l'instance d'essai, soirée d'écoute du propriétaire, puis versement dans `local` ; production au lancement suivant | — | **oui (13/10)** |
| **Installable ailleurs** — le P0 de la fiche 04 de l'étude, « avant d'en parler à quiconque » : sidecar porté ; **mode sans transcription** (le serveur démarre sans sidecar) ; avertissement légal ; rediffusion de l'audio coupée par défaut et **rôle lecture seule** ; guide d'installation bilingue ; Docker de l'amont retiré. Mesure de sortie : une installation sur une machine vierge, à partir du seul guide, **en moins d'une heure, par quelqu'un qui n'a pas écrit le code** ; précision d'appariement inchangée après portage (95 %, D73) | portage (ci-dessous) pour la transcription ; le reste n'en dépend pas | principe : oui (mission communauté) ; ordre et calendrier : non |
| Version en/fr de l'interface | après le design (les textes bougent) | principe : oui (10/10) |
| **Portage du sidecar : Linux x86-64 + NVIDIA** (faster-whisper / CTranslate2, repli processeur ; Pi qui reçoit, transcription déportée) ; **Windows au moins pour l'affinage** (les simmers) ; sous-point pour un mode différé (Pi 5, dix minutes de retard) : apparier contre le ciel de l'instant de la transmission relu dans `adsb_targets`, comme le fait déjà l'outil de mesure, au lieu de la flotte vivante (modification bornée au point d'appariement) | question 4 ; format permettant l'affinage ; non-régression mesurée au labo | **cible : oui (12/10) ; pas dans l'immédiat** (Q54) |
| **Refonte de la structure de l'écran** — **esquisses validées le 13/10 (D77)** : A par défaut (fil unifié au centre, ciel à côté, colonne « Right now » / « Traffic », la carte suivant les filtres de la liste), B colonne repliée, C suivre un vol (son histoire dans la colonne, son approche dans le cadre PAR, la séquence = le tableau « Arrivals »). La session design passe au système de composants. Si le fil est retenu, deux chantiers serveur bornés : (1) rapprocher chaque instruction ATC de la réponse du pilote qui la suit, **pour la lecture seulement, jamais un jugement d'erreur** (rien de type SLASH) ; (2) ~~enregistrer la raison d'un non-appariement~~ **fait le 13/10** (D72 complétée : `unmatched: …` dans `match_reason`, sans score). **Panneau « Right now », faisabilité vérifiée le 13/10** : tout tient avec l'existant (piste en service et depuis quand, antenne, compteurs, première détection, transferts entendus, plan de descente, heure de toucher) sauf trois petits chantiers, validés par le propriétaire le 13/10 : ~~le journal d'événements qui survit au rechargement~~ et ~~les messages ADS-B par seconde dans le bandeau~~ **faits le 13/10** ; ~~la séquence d'arrivée hors de la PAR~~ et ~~les gabarits français des consignes~~ **faits le 13/10** (`store.arrivals` pour tous les aérodromes suivis, `nextTouchdown`, la PAR dessine depuis le même calcul ; `text_fr` sur chaque consigne, `plainFrench(t)` dans la page avec le nom de la fréquence écoutée) ; « transcription dans 2 s » n'est pas possible (le serveur découpe à la fin de la transmission), « quelqu'un parle » l'est par le niveau audio | esquisses validées par le propriétaire | non |
| **Premier pas (recommandation révisée le 13/10)** : rendre Clairance installable comme **outil strictement local** — c'est le jalon « installable ailleurs » ci-dessus, même mesure de sortie ; ses garde-fous (rien ne sort de la machine, rediffusion de l'audio coupée par défaut, avertissement) passent du rang de bonne pratique à celui de **condition** : c'est ce qui rend l'outil défendable sans avocat, comme les guides français qui recommandent déjà une clé SDR pour la bande aviation. **Aucune démarche** auprès de la DGAC, de la DSNA ou de l'ANSSI, ni consultation d'avocat (objection du propriétaire : coût, et attirer l'attention « pour rien ») | — | **oui (13/10)** |
| Corpus simulé avec consentement (**voie A, la seule retenue**) : une parole simulée n'est pas une communication réelle (ni 226-15 ni le terrain de la DGAC ; reste le RGPD) ; consentement écrit d'après des modèles universitaires gratuits, relecture gratuite possible (CNIL, clinique juridique) ; pilote avec IVAO France ou un cercle proche, **après** l'étape « installable ». C'est aussi la source naturelle du **jeu de validation partageable** | installable ailleurs ; décision du propriétaire sur le pilote | principe : oui ; pilote : non |
| Affinage communautaire ou fédéré (modèle adapté au français) sur le corpus simulé | corpus ; portage | non |
| ~~Voies B (poids entraînés sur du réel) et C (transcriptions réelles publiées)~~ : **fermées** ; un avocat ne redevient utile que si on veut les rouvrir | — | fermées (13/10) |
| Premier pas public (quoi, où, quand) | installable ailleurs ; identité | non |

Plus tard, à discuter (doc 32) : strips de vol, clairance contre trajectoire, vue d'approche en
plan, procédures du SIA, fréquences comme un pupitre, programmation d'écoute.

## Les sessions cloud, vague 3 (13/10, validée par le propriétaire)

Cinq fiches dans `docs-fr/fiches-cloud/` (prompts à coller dans son README), du serveur et de la
logique pure, sans la station : **G** la route publique des chiffres avant connexion et les trois
valeurs qui manquent ; **H** le serveur sans transcription et le Docker amont retiré ; **I** les
dettes de sécurité (CORS, WebSocket, rôle lecture seule, purges) ; **J** la compilation
Windows ; **K** « Then » sans verdict ; **L** (ajoutée le 13/10 au soir, après D79) le bloc
position, trajectoire, prédiction remis d'équerre : banc de rejeu synthétique avec métriques de
mouvement, un seul prédicteur, la prédiction en direct hors de `service.go`, l'observé séparé
du déduit dans la page, interface écrite du moteur — à métriques égales ou meilleures. Indépendantes, **G, H et I lancées par le propriétaire le 13/10 au soir** (A, B, C relancées par erreur puis arrêtées, aucune branche à intégrer) ; **G intégrée le 13/10** (route `/api/v1/vitals`, `since` de la piste, valeur et unité des consignes, version et commit du binaire) ; H et I terminées, relues vertes, à intégrer ; **J, K et M lancées par le propriétaire le 13/10 dans la nuit, intégrées le 14/10 au matin** (Windows compile ; « Then » prêt pour l'étape 8 ; banc du mouvement et cinq corrections, stratégie A confirmée par le propriétaire, D80) ; **M** (écrite par la session design, relue ici) : le mouvement mesuré dans un vrai navigateur, les trois cas restants (première minute, reprise après un trou, onglet remontré) et la comparaison de stratégies (extrapolation, dessin dans le passé, hybride), le choix par défaut restant au propriétaire ; **L lancée par le propriétaire le 13/10 au soir, intégrée la nuit même** (le mouvement : vitesse dessinée qui varie de 8 à 13 % contre 38 à 48 %, plus aucun pas à reculons hors trous de réception)  ;
chaque branche `cloud/…` revient sur le Mac pour relecture et intégration. La migration de
l'interface, elle, reste sur le Mac.

## Les décisions attendues du propriétaire

0. ~~La stratégie du mouvement~~ — **décidée le 14/10 : A** (D80), prolonger la trajectoire et corriger en douceur ; B et C restent des options du banc.

1. ~~Le nom~~ — **décidé le 13/10 : Clairance** (D74 complétée, Q53 close), renommage fait le
   même jour : dépôt `chatelp/clairance`, page, lanceur `Clairance.app`, dossier de configuration
   `~/.config/clairance`, documents. Reste à faire par le propriétaire, quand toutes les sessions
   seront fermées : déplacer le dossier local `~/Dev/Aero/atc-scribe` (commande fournie le 13/10)
   et, s'il le souhaite, renommer le dépôt privé `atc-scribe-communaute` et les sessions.
2. **Le système de design** — tout est livré (`docs-fr/design/`, branche `design` : principes,
   système, jetons, maquettes, plan de migration ≈ 13 j). Six choix à trancher avant la
   migration : (1) monospace pour les phrases ou police du système (principe 8 ; le monospace
   est 1,26 fois plus large à taille égale) ; (2) cinq familles de phase à la place des dix
   couleurs ; (3) une rampe de clarté sans teinte pour l'altitude des avions ; (4) J / K au lieu
   de Tab pour passer d'un avion à l'autre ; (5) l'interpolation des avions gardée sous
   `prefers-reduced-motion` ; (6) ~~la validation des esquisses~~ **faite le 13/10 (D77)** ;
   ~~les écrans finaux~~ **validés le 13/10 (D78)** sur le système de composants
   (https://claude.ai/artifact/Ut6hWDRRUcWC58HHz6UP2K). **Reste le départ de la migration**,
   sur le plan `04-migration.md` que la session design réécrit (étapes, critères
   d'acceptation) : première étape, les jetons dans `tailwind.config.js` et `style.css` à
   comportement identique.
3. ~~Le premier pas communautaire~~ — **tranché le 13/10** avec la session communauté : pas
   d'avocat ni de démarche auprès d'une autorité ; le premier pas est l'outil strictement local
   installable ailleurs (P0), puis un corpus simulé avec consentement (voie A seule) ; les
   voies sur du réel sont fermées. Reste à décider plus tard : le pilote de corpus simulé.
4. **Demander à GitHub la purge des anciens objets** (*reportée par le propriétaire le 14/10*) (D75, deux réécritures : le 12/10 pour la
   position et les adresses, le 13/10 au soir pour les captures de la carte réelle entrées par
   la fusion du design ; sauvegardes `atc-scribe-sauvegarde-2026-10-12.git` et `-13.git`) :
   les commits d'avant restent servis par GitHub par leur identifiant jusqu'à une demande au
   support, que seul le propriétaire peut faire (texte fourni le 12/10, identifiants à citer :
   `d0b14c3`, `cdbf843`, `a892d77`, `1febe52`, `e786aeb`, `bde4b04`). À vérifier aussi : le réglage de
   précision de la carte de couverture sur FlightAware (« My ADS-B »).
5. Le moment du **portage** (cible fixée, calendrier non).
6. Le **tirage au hasard** de la précision d'appariement (jalon du labo, lié à l'annotation).

## Les risques suivis

- **Brevet US12277862B1** (Aerlogics, 2025) : corrélation transcription ATC / ADS-B pour la
  conformité ; aucun dépôt européen trouvé, revendications non lues. À faire lire avant toute
  diffusion hors de France (Q53).
- **Cadre légal** : audio réel non partageable ; corpus simulé seulement avec sessions dédiées et
  accord écrit (les règles de VATSIM et d'IVAO ne valent pas consentement RGPD).
- **Le nom** : rien de public sous « Clairance » au-delà de l'existant tant que le nom n'est pas
  choisi.
- **Secrets et données** : jamais de clé, de mot de passe, d'audio ni de base dans un dépôt ou
  dans le cloud (règles permanentes).
- **Dettes à solder avant toute installation ailleurs** (vérifiées dans les sources le 12/10 pour
  la fiche 04 de l'étude communauté) : ~~CORS renvoie toute origine avec les identifiants, le
  WebSocket accepte toute origine, aucun rôle (tout compte connecté réécoute l'audio) ; l'archive
  audio et les journaux bruts n'ont pas de purge ; Docker est mort (compile un seul fichier)~~
  — **soldées le 13/10 par les fiches H et I** (CORS et WebSocket limités aux pages du serveur
  et à `allowed_origins`, rôle `viewer`, `audio_replay`, purges réglables, Docker amont retiré,
  serveur sans transcription) ; la compilation Windows est faite (fiche J, 14/10 ; l'exécution sous Windows reste à vérifier). Sans
  gravité tant que D5 tient (réseau local seulement) ; bloquante le jour où quelqu'un expose le
  serveur.

## Comment ce document se tient

À chaque message d'une session qui livre, à chaque décision du propriétaire, à chaque jalon :
la ligne du chantier, le tableau des jalons et la liste des décisions attendues sont mis à jour
dans le même commit que le reste. Le propriétaire peut demander « où en est-on ? » : la réponse
est ce document.
