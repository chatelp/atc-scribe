# Fiche I — Les dettes de sécurité à solder avant qu'un autre installe l'outil

*Vague 3 (13/10). P0 de l'étude communauté (doc 33, « Dettes à solder avant toute
installation ailleurs »). Branche : `cloud/i-dettes-securite`. Lis d'abord `README.md` de ce
dossier : ses règles s'appliquent.*

## Le constat, vérifié dans les sources le 12/10

- `internal/api/routes.go` : `defaultCORSAllowedOrigins = []string{"*"}`, et le middleware
  (`internal/api/middleware.go`) renvoie l'origine reçue avec `Access-Control-Allow-Credentials:
  true` : **toute origine, avec les identifiants**.
- `internal/websocket/server.go` : `CheckOrigin` renvoie `true`.
- Tout compte connecté peut tout faire, y compris réécouter l'audio (`GET /api/v1/stream/{id}`,
  `GET /api/v1/transcriptions/{id}/audio`) et changer les réglages : **aucun rôle**
  (`internal/auth` n'en a pas).
- L'archive audio (`internal/audioarchive`) et les journaux bruts (`data/transcriptions/raw/`,
  `internal/transcription/file_logger.go`) **n'ont aucune purge** ; la rétention
  (`cmd/server/retention.go`) ne supprime que les bases journalières.

Sans gravité tant que D5 tient (réseau local seulement) ; bloquant le jour où quelqu'un expose
le serveur. On les solde maintenant, en gardant **tout ce qui marche aujourd'hui** : la page
servie par le serveur lui-même, les clients `radio-ctl-sync`, `sidecar/` et `tools/`, le
compte du propriétaire sans rôle déclaré.

## Étapes, dans cet ordre, un commit chacune

1. **CORS à la même origine.** Par défaut, seules les origines du serveur lui-même (son hôte et
   ses ports, `[server] port` et `additional_ports`, en http et https) et celles listées dans
   une nouvelle clé `[server] allowed_origins = []` sont acceptées ; une origine refusée ne
   reçoit **aucun** en-tête CORS (pas de 403 : la requête même-origine sans `Origin` continue
   de marcher). Les tests existants de `stream_test.go` (`Access-Control-Allow-Origin` = l'origine
   de la page, jamais `*`) doivent passer tels quels ; ajoute : origine étrangère → aucun
   en-tête.
2. **WebSocket, même règle** : `CheckOrigin` accepte une requête sans `Origin` (clients non
   navigateurs) et les mêmes origines que le CORS ; test.
3. **Un rôle lecture seule.** `configs/users.json` (format dans `internal/auth`, jamais commis)
   gagne un champ optionnel `role` : absent ou `"owner"` = tout ; `"viewer"` = lecture
   seulement : `GET` partout **sauf** `/stream/{id}` et `/transcriptions/{id}/audio` (l'audio
   n'est pas rediffusé à un lecteur), et aucun `PUT`/`POST`/`DELETE` hors `/auth/logout`.
   Implémente-le dans `auth.Require` ou un second middleware ; `GET /api/v1/auth/status` dit le
   rôle. Tests : un viewer lit `/aircraft`, est refusé (403) sur `/stream/x` et sur `PUT
   /settings`. `cmd/server -add-user` propose le rôle (défaut `owner`). **Le fichier
   `users.json` du propriétaire n'a pas de champ `role` et doit continuer de marcher.**
4. **Rediffusion de l'audio coupée par défaut pour les lecteurs** : c'est l'étape 3 ; en plus,
   une clé `[server] audio_replay = "owner" | "all"` (défaut `owner`) dit qui peut réécouter ;
   documente-la dans `config.toml.example`.
5. **Purge de l'archive audio et des journaux bruts** : deux clés, `[transcription]
   audio_archive_max_gb` (défaut 2) et `raw_log_max_days` (défaut 30) ; une tâche à côté de
   `runDatabaseRetentionCleanup` (`cmd/server/retention.go`) supprime les clips les plus
   anciens au-delà de la taille, et les journaux bruts au-delà de l'âge, **jamais ceux du jour**,
   et journalise ce qu'elle supprime. Tests avec des fichiers factices datés.

## Ce que tu ne changes pas

Les routes (liste figée `internal/api/testdata/routes.txt` : aucune ligne ne doit changer),
le format des réponses, la page `www/`, les comptes existants, le comportement du propriétaire
connecté depuis la page servie par le serveur.

## Vérifications à chaque commit

`go build ./...`, `go vet ./...`, `go test -race ./...` ; le serveur démarre avec
`configs/config.toml.example` et un `users.json` **sans** champ `role` ; `curl` depuis une
origine étrangère (`-H 'Origin: http://evil.test'`) ne reçoit aucun en-tête CORS.

## Rapport

`docs-fr/fiches-cloud/I-rapport.md` (voir README). En plus : les clés de configuration
ajoutées avec leurs défauts, et ce que le propriétaire doit vérifier sur sa configuration
réelle (son VPN : l'origine de la page y est celle de l'adresse Tailscale, à mettre dans
`allowed_origins` si elle diffère de l'hôte du serveur — explique comment la trouver).
