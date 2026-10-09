# Audit du serveur Go — 09/10/2026

Périmètre : `cmd/`, `internal/`, `pkg/`, `configs/config.toml`, `go.mod`. Base : branche `local` (ffa0213), `go 1.25.6`.
Méthode : `go vet`, `go test -cover` (et `-race`, vert), lecture du code, un analyseur maison par AST (`staticcheck` et
`deadcode` absents, non installés), trois micro-mesures par `go test -overlay` (aucun fichier du dépôt touché).
Aucun processus `co-atc` ne tournait pendant l'audit (dernier journal : arrêt propre du 04/10) : **aucune mesure de charge réelle**, voir « Mesures faites et non faites ».

## 1. Paquets (lignes hors tests ; couverture en instructions)

| Paquet | Lignes | Rôle | Dans notre config | Couv. |
|---|---:|---|---|---:|
| cmd/server | 1 011 | point d'entrée et câblage | oui | 10,6 % |
| cmd/radio-ctl-sync | 399 | synchronise les canaux avec la station | oui (à nous) | 29,7 % |
| cmd/phraseology | 1 119 | outil de mesure hors ligne de l'appariement | hors serveur | 0 % |
| internal/adsb | 7 951 | ADS-B, phases de vol, juge de piste | oui (tar1090) ; ≈ 720 lignes d'autres sources inactives | 29,1 % |
| internal/api | 4 302 | REST, WebSocket, auth, réglages | oui ; chat ATC (≈ 850) et simulation (138) inactifs | 8,1 % |
| internal/atcchat | 1 119 | assistant vocal OpenAI temps réel | **non** : `enabled = true` mais clé vide | 0 % |
| internal/audio | 1 360 | ffmpeg/SRT vers flux PCM partagé | oui ; `srt_reader` (208) et `chunker` (64) inactifs | 16,8 % |
| internal/auth | 891 | comptes, sessions, Argon2 | oui (`users.json`, pas `config.toml`) | 81,2 % |
| internal/config | 1 410 | TOML, validation, réglages d'exécution | oui | 28,6 % |
| internal/frequencies | 1 694 | un flux par fréquence, sources dynamiques | oui | 44,9 % |
| internal/physics | 159 | formules atmosphériques | oui (`atc_derived`), 9 fonctions mortes | 0 % |
| internal/reference | 1 033 | CSV aéroports, pistes, compagnies | oui | 50,9 % |
| internal/simulation | 237 | avions simulés | non : injecté chaque seconde, jamais peuplé | 0 % |
| internal/storage/sqlite | 3 259 | base quotidienne, rotation | oui | 14,8 % |
| internal/templating | 1 266 | gabarits de prompt | **non** : seulement chat ATC et post-traitement OpenAI | 0 % |
| internal/transcription | 4 293 | chemin local (sidecar, segmenteur, grammaire) + OpenAI | local oui ; `openai.go`, `processor.go`, `post_processor.go` (1 836) non | 24,4 % |
| internal/transcription/phraseology | 2 013 | grammaire et appariement | oui | 92,9 % |
| internal/weather | 737 | METAR/TAF/NOTAM via node.windy.com | oui (144 relevés/jour au 03/10) | 52,7 % |
| internal/websocket | 400 | diffusion aux navigateurs | oui | 0 % |
| pkg/logger | 424 | zap et rotation | oui | 65,1 % |
| **Total** | **35 077** | + 5 468 lignes de tests (33 fichiers à nous) | | **26,8 %** |

## 2. À nous contre amont

- 263 commits d'avance, 0 de retard (`merge-base` = `upstream/main`, 0e4d685). Dépôt entier : 169 fichiers, +33 941 / −1 312.
- Go (`cmd internal pkg`) : **75 fichiers nouveaux** (8 940 lignes + 5 409 de tests), **27 amont modifiés** (+2 201 / −898), aucun supprimé.
  Amont : 24 834 lignes ; aujourd'hui 35 077. **À nous : ≈ 11 140 lignes (32 %)** ; amont inchangé : ≈ 68 %.
- Part à nous par paquet : phraseology, auth, radio-ctl-sync, cmd/phraseology 100 % ; cmd/server 60 % ; logger 56 % ; transcription 43 % ;
  config 42 % ; api 24 % ; storage 21 % ; adsb 13 % ; audio 8 % ; websocket, atcchat, templating, simulation 0 %.
- Plus gros fichiers nouveaux : phraseology/parse.go 762, matcher.go 664, grammar_processor.go 599, cmd/phraseology/main.go 539,
  sidecar.go 512, config/runtime.go 419, auth/service.go 408, radio-ctl-sync 399, transcription/local.go 380, api/server_handlers.go 375.
- Fichiers amont les plus modifiés (ajouts/retraits) : cmd/server/main.go +333/−79 ; adsb/service.go +195/−47 ; config/config.go +178/−11 ;
  adsb/trajectory.go +155/−42 ; api/handlers.go +151/−9 ; reference/service.go +151/−14 ; frequencies/service.go +144/−80 ;
  storage/transcriptions.go +141/−329 ; api/routes.go +97/−58 ; storage/aircraft.go +95/−64. Empreinte sur `adsb/service.go` : 9 %, sur `api/handlers.go` : 6,6 %.

## 3. Code mort ou douteux

- **`go vet`** : 1 alerte, `internal/frequencies/service.go:58` (`procCancel` non appelé si `NewCentralAudioProcessor` échoue, retour ligne 80). Corrigeable en 2 lignes.
- **Jamais référencées** (AST, par nom) : 54 fonctions, **1 077 lignes**. **Inaccessibles depuis `main`** (cascade) : 81 fonctions, **1 485 lignes**, dont 80 amont, 1 à nous
  (`auth/session.go:90 RevokeUser`). Gros morceaux : `api/handlers.go:1228/1308/1388` `fetchMetarData/TAFData/NOTAMData` (3 × 78, triplées) ;
  `storage/sqlite/aircraft.go:719` (98) et `:819` (92), jumelles ; `transcription/manager.go:93 StartTranscription` (90) ; `templating/engine.go` (103) ;
  `physics.go` (9 fonctions, 81) ; `clearances.go` (4 accesseurs, 67) ; `adsb/atc_utils.go` (8, 78) ; `file_logger.go:207 CleanupOldFiles` (28, donc `data/transcriptions` ne se purge jamais : 4,2 Mo).
- **Inactif selon notre configuration** (≈ 6 500 lignes, 18,6 % du Go, tout amont) : atcchat 1 119 ; templating 1 266 ; `api/atc_chat_handlers.go` 653 + handlers chat dans `handlers.go` 194 ;
  simulation 237 + handlers 138 ; chemin OpenAI 1 990 (`openai.go` 591, `processor.go` 728, `post_processor.go` 517, `audio/chunker.go` 64, `StartTranscription` 90) ;
  sources ADS-B hors tar1090 ≈ 720 (`external.go` 286, `client.go:199-253, 272-608, 650-688`) ; `srt_reader.go` 208 (aucune URL `srt://` chez nous ; gosrt et openpgp = 9 paquets Go tiers).
- **Plus de 150 lignes** (commentaires compris) : cmd/server/main.go:66 `main` 463 ; api/handlers.go:124 `GetAllAircraft` 406 ; cmd/phraseology/capture.go:47 311 ;
  cmd/phraseology/main.go:223 299 ; transcription/post_processor.go:169 248 ; adsb/service.go:551 `fetchAndProcess` 245 ; transcription/processor.go:353 224 ;
  phraseology/matcher.go:250 221 ; storage/aircraft.go:61 `initDatabase` 201 ; aircraft.go:301 185 ; handlers.go:1522 `StreamAudio` 176 ; config/config.go:461 `Validate` 171 ;
  cmd/phraseology/main.go:40 156 ; adsb/service.go:1983 153 ; atcchat/realtime_client.go:76 150 ; openai.go:442 150. Quatre sont dans du code inactif.
- **Duplications** (blocs ≥ 12 lignes) : 83 blocs, 691 lignes (2 %), surtout dans du code mort : templating/formatters.go 170, aircraft.go:744-984 166, openai.go 110 ;
  vivant : `adsb/service.go:1919-1974` (branches T/O et T/D identiques, ≈ 55). `main.go:320` et `:367` construisent `templating.Service` deux fois.
- **Douteux, vérifié** : (a) `config.toml` met `atc_chat.enabled = true` sans clé : le serveur crée le service, la page affiche le bouton de chat (`www/atc-chat.js:127`) et chaque démarrage écrit un `WARN` ;
  (b) `prompts/transcription_prompt.txt` est lu à chaque démarrage (`frequencies/service.go:567`) et jamais utilisé par le backend local ; (c) 6 routes sans appelant dans `www/`, `tools/`, `sidecar/`, `radio-ctl-sync` :
  `/transcriptions/time-range`, `/transcriptions/speaker/{type}`, `/atc-chat/sessions`, `/atc-chat/airspace-status`, `/atc-chat/session/{id}/status`, `/simulation/aircraft/{hex}/controls` ;
  (d) `go.mod` : `golang.org/x/crypto` est importé directement mais marqué `// indirect` (`go mod tidy -diff`) ; (e) `config.go:890-902` écrit ses avertissements par `fmt.Printf`, pas par le journal.
- **Défaut mesuré, `internal/websocket/server.go`** : `writePump` tient `c.mu` pendant l'écriture réseau, sans échéance ni ping, et `broadcastImmediate` prend ce même verrou pour chaque client.
  Test (overlay) : un client qui cesse de lire (téléphone en veille, Tailscale qui bascule) **fige `Broadcast` après 68 messages de 8 Ko (≈ 0,5 Mo)** et le client sain ne reçoit plus rien.
  `Broadcast` est appelé en ligne par la boucle ADS-B (`adsb/service.go:784`) : relevé et écritures s'arrêtent avec lui. À débit réel (318 octets le message) cela fait ≈ 1 700 messages, soit une quinzaine de secondes.
  Le gel dure jusqu'à la fermeture TCP par le noyau. Dans les journaux 02-04/10 le seul trou diurne (9 min le 02/10, 13:41:41 à 13:51:15) se termine par un démarrage sans ligne d'arrêt : cause non établie. **Risque démontré, pas incident prouvé.**

## 4. `config.toml` : lignes contre clés lues

| Section | Lignes | Clés | Lues chez nous | À retirer ou défauts du code |
|---|---:|---:|---:|---|
| server | 23 | 6 | 6 | — |
| adsb | 44 | 14 | 4 | 10 : rapidapi (4), opensky (4), readsb (2) |
| logging, storage, station, reference, wx | 106 | 29 | 29 | — (`storage.type` ne vaut que `sqlite`) |
| frequencies | 107 | 4 | 4 | 103 lignes sont des exemples en commentaire |
| transcription | 49 | 23 | 7 | 16 : clé OpenAI, modèle, VAD, retry, `noise_reduction`, `prompt_path`… |
| transcription.local | 51 | 9 | 9 | — |
| post_processing | 50 | 12 | 8 | 4 : `model`, `context_transcriptions`, `timeout_seconds`, `system_prompt_path` |
| flight_phases | 87 | 37 | 37 | 20 égales au défaut du code ; 5 diffèrent (garder) ; 12 sans défaut |
| atc_chat | 35 | 17 | 1 | 16 sans effet sans clé |
| **Total** | **559** (301 commentaires, 84 vides) | **151** | **105** | **46 mortes, 20 redondantes** |

Doublons : `airport_range_nm` dans `[station]` et `[flight_phases]` (5 et 5) ; `airlines_dat_path` dans `[reference]` et `[post_processing]`.
**Configuration réduite** : 85 clés, **109 lignes sans commentaire** (contre 559), plus `atc_chat.enabled = false`. Les 5 clés qui diffèrent du défaut doivent rester explicites
(`flying_min_alt_ft` 300 contre 700, `phase_preservation_seconds` 30 contre 60, `phase_transition_timeout_seconds` 1800 contre 60, `runway_in_use_approach_weight` 2 contre 5, `runway_in_use_climb_weight` 2 contre 1,5).
Pour descendre sous 80 clés il faut des défauts en code pour les 12 clés sans défaut et pour `[reference]` ; aujourd'hui `ValidateFlightPhases` les exige.

## 5. Dix simplifications, par priorité

| # | Action | Gain | Effort | Risque prod |
|---|---|---|---:|---|
| 1 | WebSocket : échéance d'écriture, ping/pong, verrou relâché pendant l'écriture ; le test overlay devient le test de non-régression | supprime un gel démontré ; piste possible pour la lenteur ressentie sur téléphone | 4 h | moyen-faible : chemin de diffusion, tester avec un client bloqué |
| 2 | `atc_chat.enabled = false` dans `config.toml` | bouton de chat trompeur, `WARN`, goroutine de nettoyage | 0,2 h | nul |
| 3 | Supprimer `idx_adsb_targets_unique_check` (copie exacte de l'index du `UNIQUE`) et `idx_adsb_targets_aircraft_hex` (préfixe de `hex_timestamp`), `aircraft.go:198,242` | **−14,8 %** mesurés (136 à 116 Mo sur une copie du 01/10), moins d'écritures d'index ; ≈ +17 % de jours gardés | 1 h | faible : vérifier `EXPLAIN QUERY PLAN` des requêtes 669/724/827/921 ; effet à la base du lendemain |
| 4 | Retirer les 81 fonctions inaccessibles + corriger `vet` + `go mod tidy` | −1 485 lignes (4,2 %) | 4 h | nul : le compilateur tranche |
| 5 | Retirer le chemin OpenAI (`openai.go`, `processor.go`, `post_processor.go`, `chunker`, `StartTranscription`, 20 clés) ; D3 le dit déjà retiré | −1 990 lignes (5,7 %), −20 clés | 6 h | faible : le backend local ne les atteint pas |
| 6 | Config réduite plus défauts en code, détection des clés inconnues sur toutes les sections (seule `[adsb]` l'a) | 559 à ≈ 110 lignes, 151 à 85 clés | 3 h | moyen-faible : un défaut mal copié change le comportement (5 clés ci-dessus) |
| 7 | Retirer `atcchat`, `templating`, handlers et routes chat, `ATCChatConfig` | −3 250 lignes (9,3 %) ; **décision du propriétaire** (mission : « à rediscuter ») | 8 h | faible-moyen : routeur et constructeur `Handler` ; coordonner avec `www/atc-chat.js` |
| 8 | Journaux : `Aircraft status updated` et `Phase change detected` en debug | 27 306 de 53 676 lignes/jour (51 %), 6,4 Mo/jour au 03/10 | 1 h | nul |
| 9 | Retirer les sources ADS-B hors tar1090, SRT, simulation | ≈ −1 300 lignes (720 + 208 + 375), −10 clés, −9 paquets ; **décision** (généralité du dépôt public) | 7 h | faible |
| 10 | Découper `main()` (463) et `handlers.go` (2 299), supprimer la double construction de `templating` | lisibilité seule | 6 h | moyen, gain nul à l'exécution : en dernier |

Recouvrements : 4, 5, 7, 9 se chevauchent d'environ 430 lignes ; l'ensemble 4+5+7+9 vaut ≈ 7 600 lignes (≈ 21 %) pour ≈ 25 h.

## Mesures faites et non faites

- Faites (machine de dev, M4 Pro, base neuve) : `Upsert` de 100 avions = **12,3 ms par cycle** (une transaction par avion, 1,2 % du budget d'une seconde) : pas un goulot, ne pas y courir ;
  sérialisation d'un message de prédiction = 2,1 µs et 318 octets, 31 allocations : le JSON n'est pas non plus un goulot ;
  `adsb_targets` = 3 565 316 lignes sur 1,72 Go au 03/10, soit 99,9 % de la base (483 octets la ligne, 57 colonnes) ; `-race` vert.
- Non mesurées : coût CPU/mémoire du serveur en production, cause de la lenteur d'affichage des avions (le serveur diffuse ≈ 1 message par avion et par seconde, soit 80 à 110 messages/s par client, estimé),
  effet réel du point 1 sur le téléphone, durée du gel TCP, plan de requêtes après suppression des index.

## Résumé

1. Le Go fait 35 077 lignes ; 32 % sont à nous, mais ≈ 6 500 lignes d'amont (18,6 %) ne servent pas dans notre configuration, dont 1 485 mortes quoi qu'on configure (≈ 430 en commun).
2. `config.toml` : 151 clés dont 46 sans effet et 20 égales au défaut ; 559 lignes tiendraient en ≈ 110 ; `atc_chat.enabled = true` sans clé affiche un bouton inutilisable.
3. Seul défaut grave trouvé : un client WebSocket qui cesse de lire fige la diffusion, et avec elle la boucle ADS-B, après ≈ 0,5 Mo (démontré en test, jamais vu dans les journaux).
4. Gains mesurés bon marché : −14,8 % de base en retirant deux index redondants, −51 % de lignes de journal en rétrogradant deux messages ; l'écriture SQLite (12,3 ms/cycle) et le JSON ne sont pas des goulots.
5. Couverture 26,8 % ; websocket (0 %), api (8,1 %), storage (14,8 %) et audio (16,8 %) portent la production sans filet : le point 1 doit livrer son test.
