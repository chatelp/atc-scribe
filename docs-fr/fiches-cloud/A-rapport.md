# Fiche A — rapport (session cloud du 09/10)

**Branche : `claude/magical-feynman-u2kw15`** (et non `cloud/a-retraits-serveur` : la session cloud
impose sa propre branche de travail). Partie de `main` (b561478), 8 commits, rien sur `main`.
Go hors tests : **34 714 → 27 474 lignes (−7 240, −20,9 %)**. `www/`, `docs/`, `sidecar/`, `tools/`,
`cmd/radio-ctl-sync`, `cmd/phraseology` non touchés.

## Commits

| Commit | Étape | Lignes (`--shortstat`) |
|---|---|---|
| 1103c8e | 1 — test : une config écrite pour les chemins retirés démarre toujours (copie figée de l'exemple, `testdata/legacy-full.toml`) | +709 (66 de test) |
| de1a4a9 | 2 — chat vocal : `atcchat`, `atc_chat_handlers.go`, routes, `ATCChatConfig`, `[atc_chat]` de l'exemple | +8 / −2 161 |
| 358db75 | 3 — gabarits : une seule construction, seul le rendu du post-traitement gardé | +29 / −323 |
| 01f2406 | 4 — simulation : paquet, routes, injection par relevé, champs `is_simulated`/`simulation_controls` | +49 / −565 |
| 0c3fb3e | 5 — chemin OpenAI : `openai.go`, `processor.go`, `post_processor.go`, `chunker.go`, `StartTranscription`, `templating`, deux prompts, 20 clés | +210 / −3 500 |
| ab3b8fc | 6a — fonctions mortes `adsb`, `physics` | −333 |
| 42b32f3 | 6b — `api`, `storage/sqlite`, journal de transcription | −680 |
| af6139d | 6c — `weather`, `audio`, `frequencies`, `reference`, `config`, `logger` | −198 |

## Routes HTTP retirées (à vérifier côté clients sur le Mac)

`/api/v1/atc-chat/` : `POST session`, `DELETE session/{id}`, `GET session/{id}/status`,
`POST session/{id}/update-context`, `GET sessions`, `GET airspace-status`, `GET ws/{id}`.
`/api/v1/simulation/` : `POST aircraft`, `GET aircraft`, `PUT aircraft/{hex}/controls`, `DELETE aircraft/{hex}`.
Aussi : message WebSocket `simulation_control_update` (tombe dans le cas par défaut, une ligne debug) ;
`GET /config` n'a plus `atc_chat` (`atc-chat.js` lit `config.atc_chat?.enabled` → bouton masqué) ;
les avions n'ont plus `is_simulated` ni `simulation_controls` (lus comme faux/absents par `index.html`).
Seuls appelants trouvés (grep) : `www/atc-chat.js` et la modale de simulation d'`app.js`, à retirer par la fiche B.

## Clés de configuration devenues sans effet (ignorées, jamais refusées)

- `[atc_chat]` : toute la section (17 clés).
- `[transcription]` (16) : `openai_api_key`, `model`, `prompt_path`, `noise_reduction`, `chunk_ms`,
  `buffer_size_kb`, `reconnect_interval_sec`, `max_retries`, `turn_detection_type`, `prefix_padding_ms`,
  `silence_duration_ms`, `vad_threshold`, `retry_max_attempts`, `retry_initial_backoff_ms`,
  `retry_max_backoff_ms`, `timeout_seconds`. Toujours lues : `backend`, `language`, `log_dir`, les quatre `ffmpeg_*`.
- `[post_processing]` (4) : `model`, `context_transcriptions`, `system_prompt_path`, `timeout_seconds`.
- **Refusées au démarrage** : `backend = "openai"` dans `[transcription]` *ou* `[post_processing]`
  → `transcription.backend = "openai": the OpenAI backend was removed, use "local" (see sidecar/README.md)`, code 1.

## Vérifié, et comment

- À chaque commit : `go build`, `go vet`, `go test -race ./...` verts, sauf
  `TestAChoiceThatCannotBeWrittenChangesNothing` (`internal/auth`) : il échoue **déjà sur `main`**, parce que
  la session tourne en root (`chmod 0500` n'arrête pas root). Rejoué en utilisateur `nobody` : PASS, sur `main` et à la fin.
- À chaque commit, banc de démarrage : binaire lancé sur une copie de l'exemple (ports libres, faux tar1090
  local avec un avion, faux sidecar qui répond à `/health`) → « Starting HTTP server », `/api/v1/health` 200,
  `/` 200, l'avion revient par `/api/v1/aircraft`, arrêt propre sur SIGINT.
- Étape 5 : même banc sur `legacy-full.toml` (avec `[atc_chat]` et toutes les clés OpenAI) : démarre,
  transcription locale lancée sur les deux fréquences transcrites, grammaire démarrée ; refus de `openai` vérifié.
- Code mort : `deadcode` (x/tools, RTA depuis les trois `main`) + une passe AST par nom ; le compilateur tranche.
  Après 6c, les deux ne signalent plus rien hors `internal/auth` et `HasData` (utilisée par nos tests).

## Pas vérifié

Navigateur, vrais flux, vrai sidecar (MLX), station, production ; mesures de référence (appariement
`cmd/phraseology`, juge des pistes) non rejouées — `phraseology/` et la grammaire ne sont pas modifiés.
Côté `www/`, `node --check` / `node --test` non lancés : aucun fichier JS touché.

## Écarts et doutes

1. **La fiche dit que le serveur démarre sans sidecar « comme aujourd'hui » : faux.** Sur `main`, un sidecar
   absent est fatal, par choix (« refuses to run half deaf »). Comportement gardé ; le banc fournit un faux sidecar.
2. **`backend` vide** valait OpenAI ; il vaut maintenant `local` (testé). Sans effet chez le propriétaire (`local` partout).
3. **`post_processing.backend = "openai"`** est refusé comme celui de `[transcription]` : la fiche ne parlait que du second.
4. **`auth.Store.RevokeUser` et `Count`** (à nous, sans appelant) laissés : `auth` est contribuable (D71.5). À trancher.
5. **`prompts/atc_chat_prompt.txt` gardé** : `docker/start.sh` l'exige. `docker/` (amont, non maintenu ici ?) cite
   encore `[atc_chat]`. Les deux autres prompts sont retirés : plus rien ne les lit (grep sur tout le dépôt).
6. Le dossier `data/transcriptions/processed/` est encore créé au démarrage, plus rien n'y écrit. Laissé.
7. L'étape 3 n'a retiré que le mort : le seul client vivant des gabarits était le post-traitement OpenAI ;
   le paquet est parti avec lui à l'étape 5.

## À regarder en priorité sur le Mac

1. Instance d'essai avec le **vrai** `config.toml` : démarrage, plus de `WARN` chat ni « Loaded transcription prompt ».
2. Une transmission réelle : transcription, appariement, clairances, comme avant.
3. La page : bouton de chat absent ; la modale de simulation (fiche B) appelle désormais des routes en 404.
4. La branche à relire est `claude/magical-feynman-u2kw15`. Un commit par retrait, mais tous touchent
   `main.go` et `config.go` : un `git revert` se fait dans l'ordre inverse, pas isolément.
