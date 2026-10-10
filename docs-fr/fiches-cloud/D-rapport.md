# Fiche D — rapport (session cloud du 10/10)
**Branche : `claude/zealous-pascal-2msrsy`** (la session cloud impose sa branche ; poussée aussi sur
`cloud/d-config-reduite`). Partie de `main` (7c585a4), 6 commits, rien sur `main`. Commits signés
Claude Fable 5.1, le modèle réellement servi, plutôt que l'« Opus 5.5 » demandé par le README des fiches.

## Commits

| Commit | Étape | Ce qu'il fait | Chiffres |
|---|---|---|---|
| 4e90124 | 1 | copie figée de l'exemple (`testdata/example-2026-10-10.toml`) et **le** test : exemple figé et exemple livré chargent la même `Config`, champ par champ, chaque écart nommé | +71 test |
| 9614ddc | 2 | `Defaults()` (`defaults.go`), `Load` décode par-dessus ; les 12 clés de phases sans défaut et les 25 déjà remplacées à zéro (l'audit disait 20) puisent au même endroit, vérifié 25/25 ; `Validate` refuse toujours hors bornes, jamais absent | +165 code, +100 tests |
| 46441fe | 3 | une maison par doublon : `[station] airport_range_nm`, `[reference] airlines_dat_path` ; l'ancienne clé lue si écrite (`meta.IsDefined`), avec avertissement ; `Config.Warnings`, journalisé par `main.go` | +55/−5, main +5, +99 tests |
| f853834 | 4 | toute clé non lue signalée par son nom, une section disparue en une ligne avec son nombre de clés ; plus aucun refus, `[adsb]` compris | +47/−14, +55 tests |
| b11ae5c | 5 | l'exemple réduit (les messages de commit disent 148 → 56 clés : un `grep` qui ratait les noms à chiffre ; le vrai compte est ci-dessous) | 538 → 156 lignes, 150 → 57 clés |
| (ce commit) | 6 | README : sous-section Configuration, trois phrases périmées sur OpenAI ; ce rapport | |

## Le tableau des clés (étape 1, mesuré sur l'exemple du 10/10)

Toutes les clés de l'exemple sont lues hors `config.go`, sauf `storage.type` et les deux `backend`
(validées seulement). Retirées : égales au défaut (D), vides (V), déplacées (M).

| Section | Clés | Gardées | Retirées |
|---|---:|---:|---|
| server | 9 | 4 | D `read/write/idle_timeout` ; V `tls_cert`, `tls_key` |
| adsb | 14 | 2 | D URL RapidAPI et OpenSky, `search_radius_nm`, `opensky_*`, `fetch_interval`, `signal_lost` ; V `api_key`, `readsb_*` |
| logging | 5 | 2 | D `format`, `max_size_mb`, `max_files` |
| storage | 3 | 2 | D `type` |
| station | 7 | 4 | D `runway_extension_length_nm`, `airport_range_nm`, `display_range_nm` |
| reference | 6 | 0 | D les six chemins `assets/` |
| frequencies | 4 | 0 | D `buffer_size_kb`, `reconnect_interval_secs`, `ffmpeg_*` |
| frequencies.sources ×4 | 29 | 29 | rien : ce sont les valeurs de la station |
| transcription | 7 | 1 | D `backend`, `language`, `ffmpeg_*` (4) |
| transcription.local | 9 | 1 | D `server_url`, les deux `timeout`, les 5 `segment_*`/`silence_threshold` |
| auth | 4 | 1 | D `session_ttl_hours`, `max_attempts`, `attempt_window_minutes` |
| post_processing | 8 | 1 | D `backend`, `min_score`, `min_digits`, `fleet_last_seen_minutes`, `interval`, `batch_size` ; M `airlines_dat_path` |
| flight_phases | 37 | 7 | D 29 ; M `airport_range_nm` ; gardées : `enabled`, `signal_lost_landing_enabled`, les **5 qui diffèrent** (300/700, 30/60, 1800/60, 2/5, 2/1,5), vérifiées dans le code |
| wx | 8 | 3 | D `refresh_interval`, `api_base_url`, `request_timeout`, `max_retries`, `cache_expiry` |
| **Total** | **150** | **57** | 300 lignes de commentaire → 60 |

Les booléens n'ont pas de défaut autre que faux (un interrupteur se lit dans le fichier) : 7 `enabled`/`fetch_*`
restent. `command = []` et `trusted_proxies = []` aussi : écrit vide et absent ne sont pas la même valeur Go.

## Ce qu'un `config.toml` d'avant dit au démarrage (une ligne chacun, jamais un refus)

- `flight_phases.airport_range_nm repeats station.airport_range_nm (5): one key is enough, in [station]` ; si une
  seule est écrite : `… is read as …: write it in [station]` ; si elles diffèrent, chaque lecteur garde la sienne.
- `post_processing.airlines_dat_path repeats reference.airlines_dat_path (assets/airlines.dat): one key is enough, in [reference]`.
- `<section>.<clé> is not a configuration key of this version and is ignored: remove it, or check its spelling`
  pour les 16 clés OpenAI de `[transcription]` et les 4 de `[post_processing]` (liste dans `A-rapport.md`).
- `[atc_chat] is not a section of this version and is ignored with its 17 keys: remove it`.
- `db_retention_days is no longer used …` (déjà là). Sur `legacy-full.toml` : 23 lignes, testées par nom.

**Pour le propriétaire — à retirer de son `config.toml`** : `[atc_chat]` entière ; dans `[transcription]` :
`openai_api_key`, `model`, `prompt_path`, `noise_reduction`, `chunk_ms`, `buffer_size_kb`, `reconnect_interval_sec`,
`max_retries`, `turn_detection_type`, `prefix_padding_ms`, `silence_duration_ms`, `vad_threshold`, `retry_*` (3),
`timeout_seconds` ; dans `[post_processing]` : `model`, `context_transcriptions`, `timeout_seconds`,
`system_prompt_path`, `airlines_dat_path` ; dans `[flight_phases]` : `airport_range_nm` ; `db_retention_days` s'il
reste. Puis, s'il le veut, toute clé égale au défaut (tableau) : chaque ligne d'avertissement qui disparaît au
relancement est une ligne retirée à bon droit, et le serveur démarre à l'identique (test de l'étape 1).

## Vérifié, et comment

- À chaque commit : `go build`, `go vet`, `go test -race ./...` verts, sauf `TestAChoiceThatCannotBeWrittenChangesNothing`
  (`internal/auth`), qui échoue **déjà sur `main`** en root ; rejoué sous `nobody` : PASS.
- Banc de démarrage comme en fiche A (faux tar1090 avec un avion, faux sidecar sur `/health`, ports libres) sur
  l'exemple **avant** et **après** et sur `legacy-full.toml` : `/api/v1/health` 200, `/` 200, l'avion revient par
  `/api/v1/aircraft`, arrêt propre sur SIGINT ; l'exemple réduit sans avertissement, le legacy avec ses 23.

## Pas vérifié, doutes

1. Navigateur, vrais flux, vrai sidecar, station, production, le **vrai `config.toml`** du propriétaire ; `www/` non touché.
2. **Un changement hors des deux exemples** : avec `flight_phases.enabled = false` et des seuils absents, les seuils
   valaient 0 (et `IsFlying` les lit même désactivé) ; ils valent maintenant les défauts. Tous nos fichiers ont `true`.
3. Un fichier sans `[reference] airlines_dat_path` charge désormais `assets/airlines.dat` (avant : aucun nom de compagnie).
4. 156 lignes, pas 120 : 18 sont les deux sources commentées (UDP, carte son) que la fiche garde.

## À regarder en priorité sur le Mac

1. Instance d'essai avec le vrai `config.toml` : la liste exacte des avertissements, puis une transmission.
2. La fiche E part de `main.go` : la boucle `for _, w := range cfg.Warnings` (cinq lignes) doit survivre au découpage.
