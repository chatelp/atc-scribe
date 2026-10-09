# Rapport de la fiche C — tests de ce qui tourne sans filet (09/10)

Branche `cloud/c-tests`, partie de `main` (b561478) : 10 commits de tests et ce rapport, **uniquement
des fichiers `_test.go`**, aucune ligne de production touchée. Go 1.24.7, ffmpeg 6.1.1, en root.

## Commits

| Commit | Ce qu'il ajoute | Lignes | Couverture |
|---|---|---:|---|
| `storage: tests for positions…` | 12 tests : `INSERT OR IGNORE`, dernière position, mise à jour, traînée, historique d'une heure, phases (toutes les lectures), filtre `last_seen`, avions perdus, au sol, statut | 420 | sqlite 14,8 → 63,8 % |
| `storage: a skipped test… tas` | défaut 1 (ci-dessous) + aide `skipDefect` | 35 | = |
| `storage: tests for transcription queries…` | 5 tests : toutes les colonnes, file des non traitées, pages, plage horaire, `VoiceSummaries`, valeurs de phraséologie | 243 | 63,8 → 70,8 % |
| `server: tests for the edges of size-based retention` | 5 tests à côté des 5 existants : taille 0 = 20 Go, chemin actif relatif ou avec `..`, jour courant gardé même si un jour plus récent existe, première passe sans fichier du jour, dossier absent | 95 | cmd/server 10,6 → 11,2 % |
| `storage: a skipped test… zones` | défaut 2 | 38 | = |
| `audio: tests for the shared reader…` | 10 tests : lecteur lent qui ne bloque personne, réveil, retrait et fermeture, réutilisation d'un id, alignement sur l'échantillon ; en-tête WAV champ par champ | 275 | audio 27,8 → 47,8 % |
| `audio: tests for ffmpeg's command line…` | ligne de commande complète par source (Icecast avec/sans délai, montage par canal, UDP brut, carte son) ; vrai ffmpeg sur `lavfi` ; redémarrage après fin de source | 197 | 47,8 → ≈ 80 % |
| `frequencies: tests for per-client streams…` | 7 tests sur `StreamProcessor` et `GetAudioStream` (vrai ffmpeg, tonalité `-re`) | 260 | frequencies 44,8 → 76,5 % |
| `frequencies: … returning listener` | défaut 3 | 76 | = |
| `frequencies: … data race` | défaut 4 | 29 | = |

Couverture globale du serveur (`-coverprofile ./...`, mesurée ici) : **28,0 % → 36,1 %**. Celle
d'`audio` varie de 79,7 à 80,3 % : le chemin de redémarrage dépend du minutage.

## Défauts trouvés (tests marqués `t.Skip("defect: …")`, lancés par `ATC_DEFECTS=1`)

1. **Positions sans `tas` jamais dédoublonnées** — `internal/storage/sqlite/aircraft.go:166` et `:1243`.
   SQLite tient les NULL pour distincts dans un `UNIQUE` : un rapport identique où `tas` (ou `gs`,
   `track`, `lat`…) manque est réinséré à chaque relève. Mesuré : 3 relèves identiques, 3 lignes au lieu
   d'une. Le coût sur une vraie journée **n'est pas mesuré** ; sur le Mac :
   `SELECT COUNT(*) FROM adsb_targets WHERE tas IS NULL` rapporté au total.
2. **Horodatage des transcriptions dans le fuseau local** — `transcriptions.go:191` écrit
   `CreatedAt.Format(RFC3339)` sans `.UTC()`, et `transcription/local.go:170` date par `time.Now()`
   local. Les requêtes comparent du texte : une plage 11:30Z–12:30Z ne trouve pas une transmission
   enregistrée `14:00+02:00`, et le 25/10 l'heure répétée se trie à l'envers. `aircraft.go` convertit
   déjà en UTC. Impact faible aujourd'hui : la route `/transcriptions/time-range` n'a pas d'appelant dans
   le dépôt hors `templating` (retiré par la fiche A) ; reste le tri de la nuit du changement d'heure.
3. **L'auditeur qui revient repart en retard sur le direct** — `frequencies/service.go:422`
   (`NonClosingReader.Close` ne fait rien) et `audio/multireader.go:100`. Le lecteur créé dans le
   `MultiReader` (`service.go:301`) n'est jamais retiré ; la page garde un `clientID` pour sa vie
   (`www/app.js:194`) ; quand elle réécoute une fréquence, `CreateReader` rend l'**ancienne position**.
   Mesuré : nouveau client 2 à 4 Ko dans ses 100 premières ms ; même id revenu après 1,5 s : 64 Ko d'un
   coup (2,0 s d'audio), trois fois sur trois. Retard possible jusqu'au tampon entier (87 s à 24 kHz).
   **Le plus important des quatre.**
4. **Course de données** — `frequencies/service.go:1140` : `buildStreamInfo` écrit `streamPortIndex`
   sous le seul verrou en lecture (appels `:1062`, `:1105`). Révélée par `-race` seulement (11
   rapports). Coût visible au pire : un port donné deux fois de suite.

## Ce que j'ai vérifié, et comment

- À chaque commit : `go build ./...`, `go vet ./...`, `go test -race -count=1 ./...`. Tout vert **sauf
  `internal/auth` `TestAChoiceThatCannotBeWrittenChangesNothing`, qui échoue déjà sur `main`** : en root,
  un `chmod 0500` n'empêche pas d'écrire. Le même binaire de test passe sous l'utilisateur `nobody`.
- Chaque défaut lancé avec `ATC_DEFECTS=1` : il échoue comme décrit ; sans, il est sauté.
- Stabilité : `audio` et `frequencies` passés 10 fois de suite en `-race` ; durée par paquet en `-race` :
  sqlite 3,5 s, audio 2,1 s, frequencies 1,3 s, cmd/server 1,0 s.
- Pas de réseau (ffmpeg lit `lavfi`) ; heure fixe, sauf trois requêtes qui partent de `time.Now()` :
  lignes placées par rapport à l'horloge, à 20 min au moins de la limite.

## Ce que je n'ai pas pu vérifier

- Vrais flux, station, production, navigateur : si le défaut 3 s'entend, et combien de temps la page
  reste absente d'une fréquence en usage réel ; le fuseau de la production (défaut 2) ; le coût du
  défaut 1 sur une base réelle ; ffmpeg 8.0.1 et Go 1.25.6 du Mac.

## Doutes, non transformés en tests

- `MultiReader.Read` : une écriture entre le test `pos == written` et le `Wait` est manquée, le lecteur
  attend la suivante (ou 30 s). Sans effet sur un flux continu ; pas testable sans crochet.
- `central_processor.go:399` : `ProcessState` n'est rempli que par `Wait`, appelé seulement dans
  `stopFFmpeg` ; la surveillance « ffmpeg s'est arrêté » ne se déclenche donc jamais. Le redémarrage
  passe par la fin de lecture (testé). Code mort probable.
- `lastError`/`lastActivity` (`:324`, `:361`) : écrits sans verrou, lus par `GetStatus` (sans appelant).
- La session cloud imposait une autre branche que `cloud/c-tests` : mêmes commits poussés sur les deux.

## À regarder en priorité sur le Mac

1. Le défaut 3, à l'oreille : couper une fréquence 30 s puis la réécouter dans la même page.
2. Le défaut 1, par la requête SQL ci-dessus sur la base du jour.
3. `ATC_DEFECTS=1 go test -race ./internal/storage/... ./internal/frequencies/` : les quatre doivent
   échouer avant correction, et passer après (retirer alors leur `skipDefect`).
