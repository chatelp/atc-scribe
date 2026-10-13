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
| (ce fichier) | rapport | |

## Ce qui change, en une phrase par cas

- **Aucune source à `transcribe_audio = true` et `command = []`** : le sidecar n'est ni lancé ni sondé,
  le serveur démarre. **Sinon, rien ne change** : même sonde, même message fatal, même supervision.
- **Une source ajoutée à l'exécution qui demande la transcription** (`PUT /api/v1/sources/{id}`,
  donc aussi `radio-ctl-sync`) : en mode « off » seulement, le sidecar est sondé à cet instant (3 s
  au plus) ; s'il ne répond pas, **503** et la source est refusée ; la version précédente de cette
  source, s'il y en a une, continue de tourner. Renvoyer la même source inchangée ne sonde rien.
  Quand le sidecar a été trouvé au démarrage, aucune sonde n'est faite par source (`Ensure` rend nil) :
  refuser une source pendant qu'il redémarre lui coûterait aussi son audio.
- Une fois qu'une source a trouvé un sidecar, l'état est celui du sidecar (`ok`, `reachable`…) et le
  reste, même si cette source est retirée ensuite.

## Les messages exacts

- Démarrage sans transcription (info) : `No source transcribes and no sidecar is configured: running
  without transcription`, champs `server_url` et `to_transcribe` (« set transcribe_audio = true on a
  source and run a sidecar at server_url, see sidecar/README.md »).
- Source refusée, corps de la réponse 503 : `transcription unavailable: no local STT sidecar answers
  at http://127.0.0.1:8178/health (<erreur>): start one (see sidecar/README.md), or set
  transcribe_audio to false for this source`. Journal (warn) : `Source refused: it asks for
  transcription and no sidecar answers`, champs `id` et `error`.
- Premier sidecar trouvé ensuite (info) : `Local STT sidecar reached: transcription is on`.

## Ce que le voyant de la page devra montrer

`/api/v1/status` → `transcription: {"status": "off", "reason": "no source transcribes"}`, **sans
`reachable`**. Aujourd'hui `www/store/station.js` (l. 143-151) lit `!t.reachable` en premier : la page
affichera donc **« Down » en rouge**, ce qui est faux. Il faudra, avant ce test, `t.status === 'off'` →
niveau neutre (`unknown` ou un gris), texte « Off », aide « No source transcribes: the map and the
audio work without a sidecar. Set transcribe_audio on a source and run one to transcribe. ».
`/api/v1/server` → `transcription: {"status": "off", "detail": "no source transcribes", "reachable": false}`
(lu par rien dans `www/` d'après un grep).

## Vérifié, et comment

- À chaque commit : `go build ./...`, `go vet ./...`, `go test -race ./...` verts, **sauf
  `internal/auth` `TestAChoiceThatCannotBeWrittenChangesNothing`, qui échoue déjà sur `main`** avant
  toute modification : la session tourne en root, et `chmod 0500` n'empêche pas root d'écrire.
- Binaire et `go run ./cmd/server -config <copie de l'exemple>` (sources à `false`, `command = []`, faux
  tar1090, URL des flux en 127.0.0.1) : « Starting HTTP server » sur les cinq ports, rien sur :8178,
  `/api/v1/health` 200, `/status` `off`. PUT d'une source qui transcrit : 503 ; avec un faux `/health`
  (python) : 200, `Starting transcription…`, `/status` `ok`, `reachable: true`. Arrêt propre sur SIGTERM.
- Tests ajoutés (`cmd/server/startup_test.go`) : le test se relance lui-même comme serveur (`main()`
  dans un enfant, depuis la racine du dépôt) sur l'exemple ; sans transcription il répond à
  `/api/v1/health` ; avec une source qui transcrit et rien qui répond il sort avec `Local transcription
  sidecar unavailable` / `no local STT sidecar answering at`. Les remplacements comptent leurs
  occurrences : un exemple modifié fait échouer le test au lieu de tester autre chose.
- Chaque nouveau test **échoue** quand on neutralise la fonction (mode idle jamais pris ; `Off` toujours
  faux), puis repasse. `routes_test.go` (liste figée) inchangé et vert.
- `git grep -i docker` hors `docs/` et `docs-fr/` : seulement la ligne de `README-upstream.md`, des
  lignes de données dans `assets/` et l'icône docker de Font Awesome.

## Pas vérifié

- Le voyant dans un navigateur (je n'ai pas touché `www/`) ; la station, `radio-ctl-sync` réel et le
  vrai sidecar ; macOS (le test de démarrage utilise `Setpgid`, comme `sidecar.go` : Unix seulement,
  à garder en tête pour la fiche J).
- Go 1.24.7 dans le conteneur (le `go.mod` dit 1.23.2) : pas de Go 1.25 installé.

## Doutes, à trancher sur le Mac

1. **Code HTTP du refus : 503.** 400 et 409 sont déjà pris ; `radio-ctl-sync` traite tout non-2xx comme
   une erreur et réessaie au tour suivant : avec une source qui transcrit et pas de sidecar, il y aurait
   **une ligne warn par tour**. Sans effet chez le propriétaire (son sidecar est lancé par `command`).
2. **La ligne de `README-upstream.md` est dans l'en-tête en commentaire HTML**, pas dans le corps : le
   fichier se dit « preserved verbatim », et son corps ne parlait pas de Docker. Invisible sur GitHub ;
   à déplacer si on la veut visible.
3. **`prompts/atc_chat_prompt.txt`** n'était gardé que parce que `docker/start.sh` l'exigeait (rapport
   A, point 5) : il n'a plus d'utilisateur (seulement une chaîne dans `testdata/legacy-full.toml`).
   Retrait laissé à une étape à part.
4. `docs-fr/02-co-atc-amont.md` l. 35 liste encore `docker` dans l'arborescence : périmé, non corrigé
   (règle du README : pas de `docs-fr/` hors rapport).

## À regarder en priorité sur le Mac

Le voyant « Transcription » avant toute instance d'essai sans sidecar (il dira « Down ») ; puis la
production redémarrée avec sa vraie configuration : le journal doit être identique à avant (pas de
ligne « running without transcription »), puisque `command` y est rempli.
