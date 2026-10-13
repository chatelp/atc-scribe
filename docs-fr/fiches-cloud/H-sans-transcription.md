# Fiche H — Le serveur démarre sans transcription, et le Docker de l'amont s'en va

*Vague 3 (13/10). Premier point du P0 « installable ailleurs » de l'étude communauté (doc 33,
jalon « Installable ailleurs »). Branche : `cloud/h-sans-transcription`. Lis d'abord
`README.md` de ce dossier : ses règles s'appliquent.*

## Le constat, vérifié le 12/10

`cmd/server/main.go` appelle `startSidecar` sans condition, et `startSidecar`
(`cmd/server/startup.go`) arrête le serveur si rien ne répond à `/health` à `server_url`, même
quand **aucune source n'a `transcribe_audio = true`**. Quelqu'un qui n'a pas encore de sidecar
(aucun Apple Silicon, aucun GPU) ne peut donc ni voir sa carte ni écouter sa radio. Et
`docker/Dockerfile` compile `cmd/server/main.go` seul, ce qui ne compile plus (le paquet a dix
fichiers) ; `docker/start.sh` exige des fichiers qui n'existent plus ; `docker-compose.yml` est
en fuseau de Toronto.

## Étapes, dans cet ordre, un commit chacune

1. **Mode sans transcription.** Le sidecar n'est lancé, ou sondé, que si au moins une source de
   `[[frequencies.sources]]` a `transcribe_audio = true` **ou** si `[transcription.local]
   command` n'est pas vide (l'opérateur l'a demandé). Sinon : journal d'information clair
   (« No source transcribes and no sidecar is configured: running without transcription »),
   le serveur démarre, la carte, l'audio et l'ADS-B marchent. Quand une source passe à
   `transcribe_audio = true` à l'exécution (l'API des sources le permet : `PUT /api/v1/sources`
   et `radio-ctl-sync`), le serveur **sonde alors** le sidecar à cet instant et, s'il ne répond
   pas, refuse cette source avec un message net plutôt que de tomber. Regarde comment
   `internal/transcription` et `cmd/server/serve.go` démarrent la transcription locale pour une
   source ; ne réécris pas plus que nécessaire.
2. **L'état le dit.** `GET /api/v1/status` → `transcription` vaut `{"status": "off",
   "reason": "no source transcribes"}` dans ce mode (aujourd'hui `nil` quand le sidecar est
   absent) ; la page a déjà un voyant « Transcription » (`www/store/station.js`, bandeau) :
   **ne touche pas `www/`**, dis seulement dans le rapport ce que le voyant devra afficher.
3. **Tests** : un serveur construit avec la configuration d'exemple où aucune source ne
   transcrit et `command = []` démarre sans sidecar et répond à `/api/v1/health` ; le même avec
   une source qui transcrit et rien qui répond s'arrête avec le message d'aujourd'hui. Le test
   de démarrage existant (`internal/api/stack_test.go` et `cmd/server` ont des tests de
   démarrage : regarde) te donne le modèle.
4. **Le Docker de l'amont s'en va** : `docker/` en entier (Dockerfile, docker-compose.yml,
   start.sh, ce que le dossier contient), et toute mention dans `README.md` et
   `README-upstream.md` (celui-ci garde une ligne : « upstream shipped a Docker setup; it was
   removed on 13/10, it did not build »). Vérifie par `git grep -i docker` qu'il ne reste
   rien qui le promette. **Ne crée pas de nouveau Dockerfile** : ce sera une étape à part,
   après le portage du sidecar.

## Vérifications à chaque commit

`go build ./...`, `go vet ./...`, `go test -race ./...` ; `go run ./cmd/server -config
<copie de configs/config.toml.example avec toutes les sources à transcribe_audio = false et
command = []>` démarre jusqu'à « Starting HTTP server » **sans** sidecar ; la liste figée des
routes ne change pas.

## Rapport

`docs-fr/fiches-cloud/H-rapport.md` (voir README). En plus : le texte exact des deux messages
(sans transcription ; source refusée faute de sidecar), et ce que le voyant de la page devra
montrer.
