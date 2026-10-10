# Fiche F — Des tests pour l'API, qui sert le navigateur tous les jours

*Palier 5.4 de l'audit (doc 32), suite de la fiche C. Branche : `cloud/f-tests-api`. Lis
d'abord `README.md` de ce dossier : ses règles s'appliquent. Peut tourner en même temps que D ;
en même temps que E seulement si tu n'ajoutes que des fichiers `_test.go` qui passent par le
routeur, jamais par le fichier où vit un gestionnaire.*

## Le but

`internal/api` est couvert à 10,9 % (10/10) alors que c'est ce que le navigateur appelle :
35 routes. Écrire des tests **qui passent par le routeur** (`api.NewRouter`, `httptest`), comme
`access_test.go` le fait déjà : ils survivent au découpage de la fiche E. **Tu n'ajoutes que
des fichiers `_test.go`** (et des données dans `testdata/`).

## Périmètre, par priorité

1. **Authentification et accès** (compléter `access_test.go`) : chaque route de données
   répond 401 sans session et 200 avec ; `/auth/status`, login (bon et mauvais mot de passe,
   limite de 8 essais par quart d'heure), logout, cookie `SameSite=Strict`, `HttpOnly`.
2. **Avions** : `/aircraft` (filtres d'altitude, de statut, de phase ; tri), `/aircraft/{id}`,
   `/aircraft/{id}/tracks` ; avec un service ADS-B alimenté par un faux `aircraft.json` (regarde
   comment la fiche A a monté son banc de démarrage : faux tar1090 en `httptest`).
3. **Fréquences et sources** : `/frequencies`, `PUT`/`DELETE /sources/{id}` (validation des
   adresses réseau, options d'entrée ffmpeg, 409 sur une source de la configuration), le
   message WebSocket `frequencies_changed` qui en découle.
4. **Flux audio** : `/stream/{id}` répond `audio/wav`, les en-têtes CORS attendus par la page
   (origine exacte, `Allow-Credentials`, **pas** d'étoile : c'est le défaut corrigé le 02/10),
   l'en-tête WAV puis des octets ; un client qui ferme libère son lecteur. Vrai ffmpeg sur
   `lavfi`, comme en fiche C.
5. **Transcriptions** : `/transcriptions`, `/transcriptions/frequency/{id}`,
   `/transcriptions/callsign/{cs}` (champs `callsign_source`, `callsign_evidence`,
   `content_second` lus tels quels), `PUT /frequencies/{id}/label`.
6. **État et réglages** : `/server` (base, disque, rétention, sidecar `degraded`),
   `PUT` des réglages d'exécution (journal, rétention, aéroport de référence, appariement),
   `/setup/status` et `POST /setup` sur un dossier vide.
7. **WebSocket** : connexion sur `/ws` avec session, refus sans ; réception d'un
   `aircraft_bulk_response` après `aircraft_bulk_request`.

## Si un test révèle un défaut

Comme en fiche C : **ne corrige pas**, écris le test, marque-le `t.Skip("defect: …")`,
commite-le à part, décris-le dans le rapport.

## Vérifications

`go vet ./...`, `go test -race -count=1 ./...` verts ; couverture d'`internal/api` avant et
après ; pas de réseau, pas d'horloge réelle dans les assertions, moins de 15 s pour le paquet.

## Rapport

`docs-fr/fiches-cloud/F-rapport.md` : couverture avant/après, défauts trouvés, routes restées
sans test et pourquoi.
