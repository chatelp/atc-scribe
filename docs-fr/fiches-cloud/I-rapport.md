# Rapport de la fiche I — les dettes de sécurité (13/10)

**Branche : `claude/festive-carson-vpienu`** (la session cloud impose sa branche ; mêmes commits poussés aussi
sur `cloud/i-dettes-securite`). Partie de `main` (`37f2fa0`) : 5 commits et ce rapport, rien sur `main`.
Go 1.24.7, ffmpeg 6.1.1, en root. Total : +1 478 / −46 lignes dans 26 fichiers, dont 738 de tests.

## ⚠ À faire AVANT d'installer ce binaire chez le propriétaire

**`audio_archive_max_gb` vaut 2 par défaut** (demandé par la fiche). L'archive de production est celle
du `--save-audio` du sidecar, sur le disque externe, et le laboratoire la référence (D64, D65). Sans
réglage, **le premier passage, au démarrage, efface tous les clips au-delà de 2 Go**, les plus anciens
d'abord. Mettre dans le `config.toml` réel, section `[transcription]`, **avant** `[transcription.local]` :
`audio_archive_max_gb = 0` (rien n'est purgé) ou la taille voulue ; idem `raw_log_max_days` (30).

## Commits

| Commit | Ce qu'il fait | Lignes (dont tests) |
|---|---|---:|
| `4358e65` api: CORS… | Politique d'origine (`internal/auth/origin.go`) : le serveur lui-même — le nom par lequel le navigateur l'a joint (`Host`), ses noms propres (boucle locale, adresse liée), sur `port` et `additional_ports`, http et https — plus `[server] allowed_origins = []`. Origine refusée : **aucun** en-tête CORS, pas de 403 ; `Vary: Origin` ; refus journalisé une fois par origine (64 au plus). `"*"` ou une origine mal formée refusés au démarrage | +413 / −26 (175) |
| `2fde6bf` websocket… | `CheckOrigin` ne renvoie plus `true` : sans règle donnée, celle de gorilla (pas d'`Origin`, ou l'hôte de la requête) ; le routeur lui donne la politique du CORS **plus** les requêtes sans `Origin` (clients non navigateurs). 403 sinon, journalisé une fois | +182 / −12 (110) |
| `8ee29cf` auth: a read-only role… | Champ `role` optionnel dans `users.json` et `[[auth.users]]` : absent ou `owner` = tout, `viewer` = lecture ; autre valeur = refus au démarrage, compte nommé. `api.ViewerGuard`, second middleware du groupe verrouillé : GET/HEAD partout sauf `/stream/{id}` et `/transcriptions/{id}/audio`, aucune écriture (403). `/auth/status` dit `role`. `-add-user` demande le rôle (défaut `owner`, `role = "viewer"` imprimé seulement pour un lecteur) | +367 / −6 (212) |
| `46eb177` config: server.audio_replay… | `[server] audio_replay = "owner" \| "all"` (défaut `owner`) : `all` donne l'audio (direct et clips) aux lecteurs, rien d'autre | +56 / −7 (21) |
| `1bfede2` server: purge… | `runFilePurge` à côté de `runDatabaseRetentionCleanup`, au démarrage puis toutes les heures. Archive : clips les plus anciens d'abord, seulement `AAAAMMJJ/<fréquence>/*.wav`, jamais ceux du jour, dossiers de fréquence vidés retirés, **manifestes gardés**. Journaux bruts : par la date du nom, jamais ceux du jour. Tout est journalisé (par jour, puis en tout) | +470 / −5 (220) |

**Clés ajoutées et défauts** : `[server] allowed_origins = []`, `[server] audio_replay = "owner"`,
`[transcription] audio_archive_max_gb = 2`, `[transcription] raw_log_max_days = 30` (`0` = garder tout).
Rôle : `role = "viewer"` dans un compte. Toutes documentées dans `config.toml.example` ; l'exemple réduit
se charge toujours exactement comme l'exemple complet figé.

## Vérifié, et comment

- À chaque commit : `go build ./...`, `go vet ./...`, `go test -race ./...`. Seul échec :
  `TestAChoiceThatCannotBeWrittenChangesNothing` (`internal/auth`), **déjà rouge sur `main` en root** ;
  binaire de test rejoué sous `nobody` à la fin : PASS.
- `stream_test.go` passe tel quel ; `internal/api/testdata/routes.txt` inchangé (aucune ligne).
  `www/`, `docs/`, `sidecar/`, `tools/`, `cmd/radio-ctl-sync` : aucun octet modifié.
- **Banc sur le vrai binaire** (`tools/startup-bench/fake.py`, exemple avec `host = "0.0.0.0"`, un
  `users.json` dont le compte **n'a pas de champ `role`**), rejoué après chaque étape : le compte se
  connecte et est `owner` ; `curl -H 'Origin: http://evil.test'` et son preflight : aucun en-tête
  `Access-Control-*` ; la page du port N qui interroge le port N+1 : son origine, avec identifiants ;
  WebSocket : 403 depuis `evil.test`, 101 depuis la page et sans `Origin` ; un lecteur : lit `/aircraft`,
  403 sur `/stream/x` et `PUT /server/settings`, se déconnecte ; avec `audio_replay = "all"` il atteint
  le flux ; purge sur une archive et des journaux datés : les bons fichiers partent au démarrage.

## Pas vérifié

- **Un navigateur réel** (Mac, téléphone, Tailscale), la station, la production, la vraie archive : CORS
  et WebSocket sont prouvés par des requêtes, pas par Safari ou Chrome.
- **La page face à un lecteur** : `www/` ne lit pas encore `role` ; un lecteur verra les boutons de
  réglage et recevra des 403, et le lecteur audio échouera. Hors de la fiche (la page ne change pas).

## Doutes et écarts

1. La fiche dit « `PUT /settings` » : cette route n'existe pas. La vraie est `PUT /api/v1/server/settings`
   (testée, 403) ; un `PUT /api/v1/settings` tombe sur le gestionnaire de fichiers statiques et répond 404.
2. `audio_replay` couvre **les deux** routes audio (direct et réécoute) : la fiche parle de
   « rediffusion » et de « réécouter » ; j'ai lu « tout l'audio ». À trancher si le direct doit différer.
3. `0` = « ne rien purger » est un ajout (porte de sortie pour l'archive du laboratoire, ci-dessus).
4. Chaque WebSocket refusé laisse aussi la ligne `error` existante « Failed to upgrade connection ».
5. Une passe de `go test -race ./...` (étape 2) a vu échouer `internal/transcription` (superviseur du
   sidecar, paquet non touché) ; 12 passes suivantes vertes. Nom du test non capturé : instabilité à surveiller.

## Le VPN du propriétaire : faut-il `allowed_origins` ?

**En principe non.** La page construit l'adresse de l'audio avec le nom par lequel on l'a ouverte
(`www/audio-client.js`) ; ce nom arrive dans l'en-tête `Host`, et une origine qui le porte, sur un port
du serveur, est acceptée. Une adresse Tailscale (`http://100.x.y.z:8000`) ou un nom MagicDNS marchent donc
sans rien déclarer. Il faut une entrée seulement si un intermédiaire réécrit `Host` (un proxy, ou
`tailscale serve` en HTTPS). **Pour la trouver** : ouvrir la page par le VPN ; si le son ou les données
manquent, chercher dans le journal du serveur `Refused a page from another origin` ou `Refused a WebSocket
from another origin` : la valeur `origin` est exactement la ligne à ajouter. Autre voie : dans la console du
navigateur, `location.origin`. Puis `[server] allowed_origins = ["https://mac.tailnet.ts.net"]`.

## À regarder en priorité sur le Mac

1. **Régler `audio_archive_max_gb` (et `raw_log_max_days`) dans le vrai `config.toml` avant tout démarrage.**
2. Instance d'essai, vrai `users.json` sans `role` : connexion, carte, son en direct sur les ports 8001+,
   réécoute d'une transmission, réglages ; depuis le Mac **et** depuis le téléphone par Tailscale.
3. Journal de l'instance : aucune ligne `Refused …` pour les pages du propriétaire.
4. Créer un lecteur (`-add-user`, rôle `viewer`) et décider ce que la page doit lui montrer.
