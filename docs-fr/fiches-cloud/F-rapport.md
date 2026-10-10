# Rapport de la fiche F — tests de l'API (10/10)

Branche `cloud/f-tests-api`, partie de `main` (7c585a4) : 9 commits de tests et ce rapport, **uniquement des
fichiers `_test.go` dans `internal/api/`**, aucune ligne de production touchée, `access_test.go` intact.
Go 1.24.7, ffmpeg 6.1.1, 4 cœurs, en root. Même branche poussée aussi sous le nom que la session cloud
impose (`claude/jolly-shannon-k2kbxy`).

**Couverture d'`internal/api` : 10,9 % → 69,9 %** (`go test -race -cover`). 5 tests → 23 tests de premier
niveau (49 avec les sous-tests), 2 345 lignes de test.

## Commits

| Commit | Ce qu'il ajoute | Lignes |
|---|---|---:|
| `a test stack behind the real routes…` | `stack_test.go` : tout le serveur derrière `Router.Routes()` — faux tar1090 en `httptest` (4 avions : croisière, descente, montée, sol), base SQLite du jour, services ADS-B et fréquences réels, hub WebSocket, compte, réglages vivants, faux sidecar, serveur `httptest` sur les mêmes routes pour `/ws` et le flux. Tests d'authentification : 29 routes en 401 sans session et chacune avec, `/auth/status`, cookie (`HttpOnly`, `SameSite=Strict`, `Secure` seulement derrière un proxy déclaré en HTTPS), mauvais mot de passe, corps malformé ou trop gros, 8 essais puis refus du bon mot de passe, autre adresse non bloquée, déconnexion révoquée côté serveur | 637 |
| `tests for /aircraft…` | listing avec comptes, distance, voix, clairances, phase ; filtres (`callsign`, `status`, `last_seen_minutes`, `exclude_other_airports_grounded`, valeurs illisibles) ; requête de proximité triée (par hex, coordonnées, numéro de vol) ; `simple=1` ; un avion ; pistes (historique, prédiction, phases) | 279 |
| `a skipped test for the altitude filters` | défaut 1 (ci-dessous) + aide `skipDefect` | 38 |
| `tests for /frequencies…` | listing ordonné, `stream_url`, ports en alternance, transcriptions par fréquence ; `PUT`/`DELETE /sources/{id}` comme `radio-ctl-sync` les envoie, message `frequencies_changed` reçu sur `/ws` à chaque changement ; 409 sur une source configurée ; 15 corps refusés avec la raison ; libellés | 318 |
| `tests for /stream/{id}…` | vrai ffmpeg sur `lavfi` : `audio/wav`, origine exacte + `Allow-Credentials` (jamais `*`), en-tête WAV champ par champ puis 8 000 octets ; même id pendant l'écoute → `X-Already-Connected` sans corps ; client qui ferme → même id resservi en moins de 5 s ; HEAD, 503, preflight, sans `Origin`, session inventée | 223 |
| `tests for the transcription routes` | `/transcriptions` (champs tels quels dont `callsign_source`, `callsign_evidence`, `content_second`, `created_at` en UTC, pagination), par fréquence, par indicatif avec les valeurs, par locuteur, plage horaire (UTC et +02:00) | 186 |
| `tests for the server state…` | `/server` avec sidecar dégradé, absent, injoignable ; stockage (fichier actif, veille, autres fichiers ignorés, disque), journal, réglages, accès ; `PUT /server/settings` (journal, rétention, règles d'appariement, fichier `runtime-settings.json`, 9 refus sans effet) ; `/setup/status` et `POST /setup` sur un dossier vide, 409 ensuite, 409 avec compte, 409 sur `0.0.0.0` | 250 |
| `tests for /ws…` | poignée de main refusée en 401, `aircraft_bulk_request` → `aircraft_bulk_response` au seul demandeur, filtres (`phases`, `show_air`, `show_ground`, `status`, `exclude_…`), messages inconnus ou non-JSON sans fermeture, page fermée sans effet sur les autres | 149 |
| `one stack per family of tests…` | regroupement par banc partagé : 41 bancs → 20 ; durée du paquet 17,4 s → 14,4–15,1 s | = |

## Défauts trouvés (tests `t.Skip("defect: …")`, lancés par `ATC_DEFECTS=1`)

1. **`min_altitude` et `max_altitude` ignorés par `/aircraft`** — `internal/storage/sqlite/aircraft.go`,
   `GetFiltered` : la requête ne filtre que par `status`, les bornes d'altitude sont reçues et jamais
   utilisées (même chemin pour la requête groupée WebSocket). Mesuré : `?min_altitude=10000` rend les 4
   avions, dont ceux à 3 000 ft et au sol. **Aucun client du dépôt n'envoie ces paramètres** (grep sur `www/`,
   `cmd/`, `tools/`) : la page n'en souffre pas. À corriger ou à retirer (D71.3).

## Observations, non transformées en défauts

- `?distance_nm=…&ref_hex=<inconnu>` : la référence introuvable est journalisée et **le filtre est abandonné**,
  la réponse contient tout le ciel. La page n'envoie que l'hex de l'avion sélectionné.
- `/transcriptions/frequency/{id}` sans transmission répond `"transcriptions":null` ; `app.js:4114` teste le
  champ avant de le parcourir, donc sans effet. Testé tel quel.
- `Access-Control-Allow-Methods` ne cite ni `PUT` ni `HEAD` ; la page fait ses `PUT` en même origine, sans
  preflight. Sans effet aujourd'hui, à savoir si un client distant apparaît.
- Un hex français sans indicatif (`39b4c5`) reste sans `flight` : la dérivation d'immatriculation ne couvre
  pas la France. Comportement amont.

## Ce que j'ai vérifié, et comment

- À chaque commit : `go vet ./internal/api/`, `go test -race -count=1 ./internal/api/` ; à la fin,
  `go vet ./...` et `go test -race -count=1 ./...` sur tout le dépôt, verts **sauf `internal/auth`
  `TestAChoiceThatCannotBeWrittenChangesNothing`, qui échoue déjà sur `main` en root** (fiches A et C) ;
  le même binaire de test passe sous l'utilisateur `nobody`.
- Le défaut : lancé avec `ATC_DEFECTS=1`, il échoue sur les deux bornes ; sans, il est sauté.
- Pas de réseau : faux tar1090, faux sidecar et serveur « 404 » en `httptest` ; ffmpeg lit `lavfi`.
  Pas d'horloge réelle dans les assertions : transmissions et phases datées en dur ; les seules attentes
  sont des délais plafonds (5 s pour un message, un flux).
- Durée : 14,4 à 15,1 s sur cette machine, **à la limite de la fiche** : 6,9 s sont les cinq tests existants
  d'`access_test.go` (Argon2id de production, 0,65 s par calcul sous `-race`), 6,6 s les nouveaux. Les
  nouveaux comptes de test portent une empreinte Argon2id à paramètres minuscules, lus dans l'empreinte
  (`VerifyPassword`), ce qui rend chaque connexion gratuite. Les passer en parallèle n'a rien gagné (4 cœurs,
  64 Mio et deux fils par calcul). Sur le Mac ce sera plus court ; à mesurer.
- Stabilité : le paquet passé 4 fois de suite en `-race`, dont 3 après le regroupement.

## Routes restées sans test de fond, et pourquoi

Couvertes seulement par le tableau 401/200 : `/airports`, `/airports/{ident}`, `/heliports`, `/navaids`,
`/navaids/{ident}`, `/runways` (pas de données de référence dans le banc : listes vides, 404 par ident),
`/wx` (pas de service météo : `fetch_errors`), `/station` (pas de référence : pas de pistes), `/config`,
`/adsb/source`, `/transcriptions/time-range` au-delà des bornes. Le fichier statique `/*` n'est pas testé
(`www/` relatif au dossier courant). `PUT /access` et le premier lancement avec compte restent à
`access_test.go`.

## À regarder en priorité sur le Mac

1. `go test -race -count=1 -cover ./internal/api/` : durée et couverture sur le Mac (ffmpeg 8, Go 1.25).
2. Le défaut 1 : décider entre corriger `GetFiltered` et retirer les deux paramètres.
3. La fiche E : ces tests passent par le routeur, ils doivent survivre au découpage de `handlers.go` sans
   modification ; seul `stack_test.go` construit un `Handler` et un `Router` à la main (champs non exportés).
