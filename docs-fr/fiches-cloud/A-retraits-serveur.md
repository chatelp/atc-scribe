# Fiche A — Retirer côté serveur ce qui n'a pas de rôle ici

*Palier 1 de l'audit (doc 32, lignes 1.2, 1.3, 1.4), sous D71.3. Branche :
`cloud/a-retraits-serveur`. Lis d'abord `README.md` de ce dossier : ses règles s'appliquent.*

## Le but

Retirer du serveur Go le code amont qui ne sert pas dans atc-scribe et ne servira pas, sans
rien changer à ce qui tourne. Estimation de l'audit : environ 5 500 lignes. Carte détaillée :
`docs-fr/audit-2026-10-09/backend.md`, sections 1 et 3.

## La contrainte qui prime : la configuration du propriétaire doit toujours démarrer

Le `configs/config.toml` du propriétaire, qui n'est pas dans le dépôt, contient encore
`[atc_chat]` (avec `enabled = false`), les clés OpenAI de `[transcription]`
(`openai_api_key`, `model`, `noise_reduction`, `prompt_path`…) et celles de
`[post_processing]`. **Le serveur doit démarrer avec ce fichier tel quel** : une clé qui ne
sert plus doit être ignorée, jamais refusée. Écris d'abord un test qui charge une
configuration contenant toutes ces sections et clés (prends-les dans
`configs/config.toml.example`) et vérifie qu'elle se charge et se valide ; garde-le vert à
chaque étape.

## Étapes, dans cet ordre, un commit chacune

1. **Le test de configuration ci-dessus.**
2. **Le chat vocal « AI Advisory »** (1.3) : `internal/atcchat/`, `internal/api/atc_chat_handlers.go`,
   les gestionnaires de chat dans `handlers.go`, les routes `/atc-chat/*` de `routes.go`, sa
   construction et son arrêt dans `cmd/server/main.go`, le paramètre de `NewRouter` et
   `NewHandler`, `ATCChatConfig` et sa validation dans `internal/config`. La section
   `[atc_chat]` de `config.toml.example` part aussi. *Ne touche pas `www/atc-chat.js` : c'est
   la fiche B.*
3. **Les gabarits** (`internal/templating/`) : l'audit dit qu'ils ne servent qu'au chat et au
   post-traitement OpenAI, et que `main.go` les construit deux fois (lignes ~320 et ~367).
   **Vérifie** qu'aucun chemin local ne les utilise (la météo, la grammaire
   `post_processing.backend = "local"`) avant de les retirer ; si quelque chose de vivant s'en
   sert, ne retire que le mort et dis-le.
4. **La simulation** (1.4) : `internal/simulation/`, ses gestionnaires et ses routes
   `/simulation/*`, son injection dans le service ADS-B (l'audit : « injecté chaque seconde,
   jamais peuplé »). *Côté `www/`, c'est la fiche B.*
5. **Le chemin de transcription OpenAI** (1.2, exécution de D3) : `transcription/openai.go`,
   `processor.go`, `post_processor.go` (OpenAI), `audio/chunker.go`, `StartTranscription`, et
   les clés de configuration qui ne servent qu'à lui. **Garde** tout le chemin local :
   `local.go`, `sidecar.go`, le segmenteur, `grammar_processor.go`, `phraseology/`.
   Si `backend = "openai"` est encore écrit dans une configuration, le serveur doit s'arrêter
   au démarrage avec un message clair (« the OpenAI backend was removed, use local »), pas
   démarrer sans transcription. `config.toml.example` passe à `backend = "local"`, et ses
   commentaires sur les crédits OpenAI partent.
6. **Les 81 fonctions inaccessibles** de l'audit (backend.md, section 3), ce qu'il en reste
   après les étapes 2 à 5 : supprime-les par paquet, un commit par paquet ou deux. Le
   compilateur tranche ; pour une fonction appelée par réflexion ou par une route, vérifie à
   la main.

**Ne retire pas** : les sources ADS-B autres que tar1090 (`internal/adsb/external.go`,
`client.go` : D71.6, elles servent à d'autres stations) ; le prompt
`prompts/transcription_prompt.txt` *sauf* si tu prouves que rien ne le lit plus après
l'étape 5 ; quoi que ce soit dans `cmd/radio-ctl-sync`, `cmd/phraseology`, `sidecar/`.

## Vérifications à chaque commit

`go build ./...`, `go vet ./...`, `go test -race ./...` ; et
`go run ./cmd/server -config configs/config.toml.example` doit démarrer jusqu'à « Starting
HTTP server » (copie l'exemple dans un dossier de travail, avec `[transcription] backend =
"local"` et une `server_url` vers un port vide : le sidecar absent est attendu, le serveur doit
démarrer quand même, comme aujourd'hui). Note dans le rapport les lignes retirées par étape
(`git diff --shortstat`).

## Rapport

`docs-fr/fiches-cloud/A-rapport.md` (voir README). En plus : la liste des routes HTTP retirées
(le Mac vérifiera qu'aucun client ne les appelle), et la liste des clés de configuration
devenues sans effet.
