# Fiche E — rapport (session cloud du 10/10)

**Branche : `claude/tender-franklin-xk90nr`** (imposée par la session cloud ; poussée aussi sur
`cloud/e-decoupage-serveur`). Partie de `main` (8503e84, D et F intégrées), 5 commits et ce rapport,
rien sur `main`. Aucun comportement changé : ni route, ni nom public, ni message de journal.
`www/`, `docs/`, `internal/transcription/`, `phraseology/` non touchés.

## Commits

| Commit | Étape | Ce qu'il fait | `--shortstat` |
|---|---|---|---|
| 06cce12 | 1 | garde-fous : `internal/api/routes_test.go` + `testdata/routes.txt` (45 lignes : méthode, chemin, fonction, `open`/`locked`) ; `tools/startup-bench/` (`run.sh`, `fake.py`, `compare.py`) | +396 |
| b5b1a5e | 2a | rétention (Q26) → `retention.go`, `-add-user` → `add_user.go`, déclarations recopiées à l'octet | +243 / −222 |
| 77f0df4 | 2b | `main()` en étapes nommées : `startup.go` (loadConfig, newLogger, buildADSBClient, startSidecar, openStorage, buildADSBService, loadReference, startWeather, startFrequencies, buildRouter), `serve.go` (serve, shutdown) | +514 / −379 |
| fd31868 | 3 | `handlers.go` en sept fichiers par domaine, 41 déclarations recopiées à l'octet | +1 526 / −1 470 |
| 6677ec7 | 3 | `GetAllAircraft` scindée en dix fonctions privées, même requête, même réponse | +315 / −249 |

## Fichiers avant / après (lignes, hors tests)

| Avant | | Après |
|---|---:|---|
| `cmd/server/main.go` (`main()` 406) | 695 | `main.go` 112 (`main()` 93, dont 49 de code) · `startup.go` 352 · `serve.go` 144 · `retention.go` 159 · `add_user.go` 84 |
| `internal/api/handlers.go` (`GetAllAircraft` 407) | 1 636 | `handlers.go` 166 · `aircraft_handlers.go` 1 002 (`GetAllAircraft` 80) · `station_handlers.go` 217 · `stream_handlers.go` 190 · `reference_handlers.go` 91 · `frequency_handlers.go` 63 · `weather_handlers.go` 29 |

Total : `cmd/server` 966 → 1 122 (+156), `handlers.go` 1 636 → 1 758 (+122) : signatures et commentaires de tête.
`StreamAudio` (176 lignes) déplacée telle quelle. Les `defer` restent dans `main()`, posés au même point,
donc exécutés dans le même ordre ; la boucle `cfg.Warnings` (fiche D) y reste, avant toute construction ;
`router.Routes()` est toujours appelée une fois par port d'écoute.

## La preuve

- **Routes** : `routes.txt` écrit depuis `main` au commit 1, **inchangé** jusqu'à la tête
  (`git diff 06cce12 HEAD -- internal/api/testdata/routes.txt` vide) ; le test passe à chaque commit. Vérifié
  qu'il échoue si une route est branchée sur une autre fonction. 36 routes `/api/v1` (30 derrière la
  connexion, `HEAD /stream` compris ; 6 ouvertes) et les 9 méthodes des fichiers statiques sur `/*`.
- **Journal de démarrage** : le serveur journalise depuis plusieurs goroutines, deux passes du même binaire
  ne donnent jamais le même fichier. `compare.py` compare trois vues, **stables sur six passes de `main`** :
  les lignes de `main()` dans l'ordre avec leurs champs (34), l'ordre de construction des services (28 étapes),
  l'ensemble des messages distincts (87). Vérifié qu'elle signale un démarrage réordonné (météo avant ADS-B).
  **Identique à `main` après chaque commit** (une à cinq passes chacun, deux sur la tête).
- **Banc** (repris de la fiche A, désormais dans le dépôt) : « Starting HTTP server », `/api/v1/health` 200,
  `/` 200, l'avion `c0ffee` revient par `/api/v1/aircraft`, arrêt propre sur SIGINT (code 0), à chaque commit.
- **Déplacements** : par script, chaque déclaration comparée à sa copie d'avant ; pour `main()`, chaque
  littéral de chaîne et chaque commentaire de l'ancienne fonction compté dans les nouveaux fichiers.
- **`GetAllAircraft`** : un test jetable (non commité) sur le banc de la fiche F enregistre 22 réponses
  (filtres, proximité par hex, coordonnées et vol, références introuvables, `simple=1`, `/aircraft/{id}`,
  `/tracks`) ; **identiques octet pour octet avant et après**, trois passes, horodatages masqués et listes non
  ordonnées triées par hex (l'ordre de `/aircraft` varie déjà d'un appel à l'autre sur `main`). Vérifié que la
  comparaison voit un compte faussé. Le défaut 1 de F (`min/max_altitude` ignorés) échoue à l'identique.
- `go build`, `go vet`, `go test -race ./...` verts à chaque commit, sauf
  `TestAChoiceThatCannotBeWrittenChangesNothing` (`internal/auth`), qui échoue **déjà sur `main`** en root ;
  rejoué sous `nobody` : PASS.

## Ce que je n'ai pas osé, ou pas fait

1. **Étape 4, la duplication `adsb/service.go`** (aujourd'hui `sendImmediateGroundTransitionAlerts`,
   l. 1787-1848, l'audit disait 1919-1974 avant la fiche A) : les branches T/O et T/D ne diffèrent que par le
   message (« Aircraft TOOK OFF (IMMEDIATE) » / « LANDED ») ; la fusion est évidente, mais **aucun test
   existant ne passe par cette fonction**. Laissée, comme le veut la fiche.
2. Rien déplacé dans `server_handlers.go`, `setup_handlers.go`, `transcription_handlers.go` (déjà par domaine).
3. `serve()` porte aussi le refus de démarrer sans compte hors boucle locale : même endroit qu'avant, avant les écoutes.
4. Les commentaires amont devenus faux restent tels quels (« Stop any active transcription processors … »,
   « Create SQLite storage with no retention settings ») : la fiche interdisait d'y toucher.

## Pas vérifié

Navigateur, vrais flux, vrai sidecar, la station, la production, le vrai `config.toml` ; le cas TLS
(`tls_cert`/`tls_key`) et le refus sans compte sur `0.0.0.0` ne passent pas par le banc (code déplacé tel quel).
Aucun fichier JS touché : `node --check` et `node --test www/par/` sans objet.

## À regarder en priorité sur le Mac

1. Rejouer le banc entre `main` et la branche :
   `tools/startup-bench/run.sh /tmp/avant` (sur `main`), puis sur la branche `run.sh /tmp/apres`, puis
   `tools/startup-bench/compare.py /tmp/avant /tmp/apres` (Python 3, curl, Go ; ports 18700-18710).
2. L'instance d'essai avec le vrai `config.toml` : même journal de démarrage qu'avant (mêmes lignes, même ordre).
3. Décider de l'étape 4 : un test de `sendImmediateGroundTransitionAlerts` d'abord, la fusion ensuite (≈ 30 lignes).
