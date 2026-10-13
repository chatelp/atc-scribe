# Fiche G — La route publique des chiffres avant la connexion, et trois valeurs qui manquent

*Vague 3 (13/10). Étape 9 du plan de migration (`docs-fr/design/04-migration.md`), fiche
`StationVitals` du système de design (`docs-fr/design/systeme/project/components/`). Branche :
`cloud/g-chiffres-avant-connexion`. Lis d'abord `README.md` de ce dossier : ses règles
s'appliquent.*

## Le but

La page de connexion choisie par le propriétaire (« le ciel », `docs-fr/design/09-ecrans.md`)
montre, **avant toute connexion**, des chiffres agrégés sur ce que la station reçoit. Le serveur
doit les servir par une route publique, sans jamais rien dire qui situe la station. Rien côté
`www/` : la page viendra avec la migration ; cette fiche livre la route, ses données et ses tests.

## Ce que la route sert

`GET /api/v1/vitals`, **sans authentification**, JSON, exactement ces champs (voir la fiche
`StationVitals` pour les libellés) :

- `aircraft` : avions actifs en vue (`adsbService.GetAllAircraft()`, statut `active`, comme
  `/api/v1/status`) ;
- `messages_per_second` : `adsbService.MessageRate()` arrondi (existe) ;
- `farthest_nm` : distance au plus lointain avion positionné, depuis la position de la station
  (`cfg.Station.Latitude/Longitude`), en milles nautiques, arrondie à l'unité ; **la position
  elle-même ne sort jamais** ;
- `channels_on_air` : nombre de sources audio actives (`frequenciesService`) ;
- `transmissions_today`, `matched_today`, `matched_share` : `COUNT(*)`,
  `COUNT(callsign != '')` sur la base du jour (`internal/storage/sqlite`, table
  `transcriptions`), **une seule requête**, et la part en pourcentage entier ;
- `transcription_model` : la famille lisible du modèle (`/health` du sidecar donne `model_en`,
  un nom Hugging Face comme `sfabriece/whisper-large-v3-atco2-asr-mlx` → « Whisper large-v3
  ATCO2 » ; écris la règle de raccourcissement et teste-la sur trois noms) ;
- `transcription_backend` : ce que le sidecar déclare (étape 2) ;
- `seconds_per_transmission` : la moyenne glissante de l'étape 3, à une décimale, absent tant
  qu'aucune transmission n'a été transcrite ;
- `airports_followed`, `runways_in_use` : nombres (`/api/v1/station` les connaît) ;
- `uptime_seconds`, `version`, `commit` (étape 4).

**Un cache serveur de 10 s est obligatoire** : la route est publique sur un serveur qui écoute
le réseau local ; une visite ne doit jamais déclencher une requête SQL ni un appel au sidecar.
Un seul calcul par fenêtre de 10 s, partagé entre toutes les visites, protégé par un mutex.

**Un réglage la coupe** : `vitals.enabled` dans les réglages d'exécution
(`internal/config/runtime.go`, comme `MatchingRules`), vrai par défaut ; coupée, la route
répond 404. Expose-le dans `GET/PUT /api/v1/settings` comme les autres réglages du panneau.

## Étapes, dans cet ordre, un commit chacune

1. **La route et son cache**, avec les valeurs déjà disponibles ; test : deux appels dans la
   même seconde font un seul calcul (compteur dans le test), un appel après 10 s en fait un
   second ; la réponse ne contient aucune clé hors de la liste ci-dessus (test qui énumère les
   clés) ; aucun nombre à plus de deux décimales ; `vitals.enabled = false` → 404.
2. **Le sidecar dit son appareil** : `sidecar/whisper_server.py`, `/health` gagne `backend`
   (« mlx » aujourd'hui, le nom du GPU si `mlx.core` le donne, sinon « mlx ») ; le Go le lit
   dans `internal/transcription/sidecar.go` (la structure du `/health` existe) et la route le
   sert. Le faux sidecar des tests (`runs/son-test/fake-sidecar.py` n'est pas dans le dépôt :
   regarde `internal/transcription/sidecar_test.go` pour le serveur de test) renvoie `backend:
   "fake"`.
3. **Le temps moyen par transmission** : dans `internal/transcription/local.go`, autour de
   l'appel HTTP au sidecar (là où « Transcribed » est journalisé), mesure la durée et tiens une
   moyenne glissante exponentielle (α = 0,1) protégée par un mutex, exposée par une méthode du
   processeur ; test unitaire sur la moyenne.
4. **Version et commit du binaire** : `internal/api/server_handlers.go` dit `"0.1.0"` en dur.
   Ajoute dans `cmd/server` deux variables `version` et `commit` renseignées par
   `-ldflags "-X main.version=… -X main.commit=…"`, « dev » et « unknown » sinon ; passe-les
   au gestionnaire ; documente la commande de construction dans `README.md` (section
   installation) et dans `tools/menubar/README.md`. Le Mac ajoutera les `-ldflags` à ses
   scripts de lancement (hors dépôt).
5. **Le « depuis quand » de la piste en service** : `internal/adsb/runway_tracker.go` journalise
   « Active runway changed » sans le garder ; mémorise l'instant du dernier changement par
   aérodrome et sers-le comme `since` (RFC 3339) dans chaque entrée de `runway_in_use` de
   `/api/v1/station`, `null` tant qu'aucun changement n'a été vu depuis le démarrage. Test sur
   le tracker : deux événements qui font basculer la piste → `since` = l'instant du second.
6. **Valeur et unité des consignes** : `internal/transcription/phraseology/explain.go`,
   `Instruction` gagne `Value` (chaîne telle que dite : « 100 », « 4000 », « 270 »,
   « 119.255 », « 27R ») et `Unit` (« FL », « ft », « deg », « kt », « MHz », « hPa », « code »,
   « runway », vide pour une clairance) ; `sqlite.PlainInstruction` et `plainOf` les
   recopient ; étends `explain_test.go` et `explain_fr_test.go`.

## Ce qui ne sort jamais

Aucune position, aucun indicatif, aucune fréquence nommée, aucun nom d'aérodrome, aucune
adresse. Le test de l'étape 1 vérifie la liste des clés ; ajoute un test qui sérialise la
réponse et affirme qu'aucune valeur n'est une chaîne contenant un chiffre à trois décimales ni
une lettre d'indicatif (regex `[A-Z]{3}[0-9]`).

## Vérifications à chaque commit

`go build ./...`, `go vet ./...`, `go test -race ./...` ; le serveur démarre avec
`configs/config.toml.example` (sidecar absent attendu). `python3 -m py_compile
sidecar/whisper_server.py`. La liste figée des routes
(`internal/api/testdata/routes.txt`, `ATC_WRITE_ROUTES=1` pour la régénérer) change d'une ligne,
`GET /api/v1/vitals … open` : dis-le dans le rapport.

## Rapport

`docs-fr/fiches-cloud/G-rapport.md` (voir README). En plus : un exemple de réponse de la route
(valeurs fictives), et la règle de raccourcissement des noms de modèles.
