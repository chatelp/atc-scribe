# Fiche H — rapport (session cloud du 13/10)

**Branche : `claude/quirky-ritchie-mfjscr`** (et non `cloud/h-sans-transcription` : comme pour A, la
session cloud impose sa propre branche). Partie de `main` (37f2fa0), 5 commits, rien sur `main`.
`www/`, `docs/`, `sidecar/`, `tools/`, `cmd/radio-ctl-sync` non touchés ; aucune route ajoutée ni renommée.

## Commits

| Commit | Étape | Lignes (`--shortstat`) |
|---|---|---|
| c16189a | 1 — mode sans transcription : `Sidecar.Idle/Off/Ensure`, `transcriptionWanted`, garde dans `AddSource`, 503 dans `PutSource` | +118 / −2 |
| ce473ee | 2 — `/api/v1/status` et `/api/v1/server` disent `off` | +9 / −3 |
| 7adb32c | 3 — tests : `main()` lancé en processus enfant sur l'exemple, API, sidecar, sources | +404 / −4 |
| c769a2e | 4 — `docker/` retiré (5 fichiers, 803 lignes), une ligne dans `README-upstream.md` | +1 / −808 |

## Ce qui change

- **Aucune source à `transcribe_audio = true` et `command = []`** : le sidecar n'est ni lancé ni sondé,
  le serveur démarre. **Sinon, rien ne change** : même sonde, même message fatal, même supervision.
- **Source ajoutée à l'exécution qui transcrit** (`PUT /api/v1/sources/{id}`, donc `radio-ctl-sync`) :
  en mode « off » seulement, sidecar sondé à cet instant (3 s au plus) ; sans réponse, **503**, et
  l'ancienne version de la source continue de tourner. La même source renvoyée ne sonde rien. Sidecar
  trouvé au démarrage : pas de sonde par source (refuser pendant un redémarrage coûterait l'audio).
- Une fois un sidecar trouvé par une source, l'état est le sien, même si la source part ensuite.

## Les messages exacts

- Démarrage (info) : `No source transcribes and no sidecar is configured: running without
  transcription`, champs `server_url` et `to_transcribe` (que faire pour transcrire).
- Source refusée, corps de la réponse 503 : `transcription unavailable: no local STT sidecar answers
  at http://127.0.0.1:8178/health (<erreur>): start one (see sidecar/README.md), or set
  transcribe_audio to false for this source`. Journal (warn) : `Source refused: it asks for
  transcription and no sidecar answers`, champs `id` et `error`.
- Premier sidecar trouvé ensuite (info) : `Local STT sidecar reached: transcription is on`.

## Ce que le voyant de la page devra montrer

`/api/v1/status` → `transcription: {"status": "off", "reason": "no source transcribes"}`, **sans
`reachable`**. Aujourd'hui `www/store/station.js` (l. 143-151) lit `!t.reachable` en premier : la page
affichera **« Down » en rouge**, ce qui est faux. Il faudra, avant ce test, `t.status === 'off'` →
niveau neutre, texte « Off », aide « No source transcribes: the map and the audio need no sidecar. ».
`/api/v1/server` dit `{"status":"off","detail":"no source transcribes","reachable":false}` (lu par
rien dans `www/`).

## Vérifié, et comment

- À chaque commit : `go build ./...`, `go vet ./...`, `go test -race ./...` verts, **sauf
  `internal/auth` `TestAChoiceThatCannotBeWrittenChangesNothing`, qui échoue déjà sur `main`** avant
  toute modification : la session tourne en root, et `chmod 0500` n'empêche pas root d'écrire.
- `go run ./cmd/server` sur une copie de l'exemple (sources à `false`, `command = []`, faux tar1090,
  flux en 127.0.0.1) : « Starting HTTP server » sur les cinq ports, rien sur :8178, health 200, `off` ;
  PUT qui transcrit : 503, puis 200 et `ok` avec un faux `/health` (python) ; arrêt propre sur SIGTERM.
- `cmd/server/startup_test.go` relance le binaire de test comme serveur (`main()` dans un enfant) sur
  l'exemple : sans transcription, `/api/v1/health` répond ; avec une source qui transcrit et rien qui
  répond, sortie avec `Local transcription sidecar unavailable`. Les remplacements comptent leurs
  occurrences : un exemple modifié fait échouer le test.
- Chaque nouveau test **échoue** quand on neutralise la fonction (mode idle jamais pris ; `Off` toujours
  faux), puis repasse. `routes_test.go` (liste figée) inchangé et vert.
- `git grep -i docker` hors `docs/` et `docs-fr/` : seulement la ligne de `README-upstream.md`, des
  lignes de données dans `assets/` et l'icône docker de Font Awesome.

**Pas vérifié** : le voyant dans un navigateur ; la station, `radio-ctl-sync` réel, le vrai sidecar ;
macOS (le test de démarrage use de `Setpgid`, Unix comme `sidecar.go` : à savoir pour la fiche J) ;
Go 1.25 (le conteneur a 1.24.7, `go.mod` dit 1.23.2).

## Doutes, à trancher sur le Mac

1. **Refus en 503** (400 et 409 sont pris). `radio-ctl-sync` réessaie à chaque tour : sans sidecar,
   **une ligne warn par tour**. Sans effet chez le propriétaire si `command` est rempli chez lui (à confirmer).
2. **La ligne de `README-upstream.md` est dans son en-tête en commentaire HTML** (le corps se dit
   « verbatim » et ne parlait pas de Docker) : invisible sur GitHub, à déplacer si on la veut visible.
3. **`prompts/atc_chat_prompt.txt`** n'était gardé que pour `docker/start.sh` (rapport A, point 5) : plus
   d'utilisateur (une chaîne dans `testdata/legacy-full.toml`). Retrait laissé à une étape à part.
4. `docs-fr/02-co-atc-amont.md` l. 35 liste encore `docker` : périmé, non corrigé (pas de `docs-fr/`).

## À regarder en priorité sur le Mac

Le voyant « Transcription » avant toute instance d'essai sans sidecar (il dira « Down ») ; puis la
production redémarrée sur sa configuration : journal identique à avant, sans « running without
transcription » (vrai dès que `command` y est rempli ou qu'une source configurée transcrit).
