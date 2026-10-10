# Fiche E — Découper `main()` et `handlers.go`, sans rien changer

*Palier 5.3 de l'audit (doc 32), sous D71. Branche : `cloud/e-decoupage-serveur`. **À lancer
après l'intégration de la fiche D** : les deux touchent `cmd/server/main.go`. Lis d'abord
`README.md` de ce dossier : ses règles s'appliquent.*

## Le but

`cmd/server/main.go` fait 690 lignes, dont `main()` 401 ; `internal/api/handlers.go` 1 636
(mesuré le 10/10). Le but est la lisibilité seule : **aucun changement de comportement, aucune
route, aucun nom public, aucun message de journal modifié.** Gain nul à l'exécution, risque
moyen : c'est pourquoi tout se vérifie par comparaison avant/après.

## Les garde-fous propres à cette fiche, à écrire avant de déplacer quoi que ce soit

1. **La liste des routes** : un test qui monte le routeur (`api.NewRouter` avec des services
   nuls ou minimaux, comme les tests existants d'`internal/api` le font) et en tire, par
   `chi.Walk`, la liste triée « méthode chemin ». Il compare à une liste figée dans
   `testdata/routes.txt`, écrite depuis `main` avant tout déplacement. Elle ne doit pas changer
   d'une ligne.
2. **Le banc de démarrage** de la fiche A (binaire, exemple de configuration, faux tar1090,
   faux sidecar) : « Starting HTTP server », `/api/v1/health` 200, un avion par
   `/api/v1/aircraft`, arrêt propre sur SIGINT ; **et le journal de démarrage** : mêmes
   messages, même ordre (compare les lignes, horodatages retirés).
3. `go build`, `go vet`, `go test -race ./...` à chaque commit.

## Étapes, un commit chacune

1. Les garde-fous ci-dessus.
2. `main()` : extraire par domaine, dans `cmd/server/`, des fonctions qui prennent la
   configuration et le journal et rendent ce qu'elles construisent — par exemple
   `buildStorage`, `buildADSB`, `buildTranscription`, `buildFrequencies`, `buildWeather`,
   `buildRouter`, `serve` (les écoutes HTTP), `shutdown` — dans **l'ordre où `main()` les
   appelle aujourd'hui**, sans changer cet ordre (le câblage par `AttachRuntime` et le
   démarrage du sidecar avant tout sont des choix datés : respecte-les, lis les commentaires).
   `main()` doit tenir en une page.
3. `handlers.go` : un fichier par domaine (`aircraft_handlers.go`, `frequency_handlers.go`,
   `weather_handlers.go`, `stream_handlers.go`…), les méthodes **déplacées telles quelles**,
   commentaires compris. `GetAllAircraft` (≈ 400 lignes, filtres) peut se scinder en
   fonctions privées sans changer ni la requête ni la réponse. `StreamAudio` (176 lignes) :
   ne la touche pas au-delà du déplacement, c'est le chemin du son.
4. Les duplications vivantes signalées par l'audit : `adsb/service.go:1919-1974` (branches
   décollage/atterrissage identiques, ≈ 55 lignes) — **seulement si** la fusion est évidente
   et couverte par un test existant ; sinon, laisse et dis-le.

**Garde** la boucle de `main.go` qui journalise `cfg.Warnings` (fiche D) : elle doit survivre au
découpage, avant la construction des services.

**Ne touche pas** : `internal/transcription/`, `phraseology/`, `www/`, les messages de journal,
les signatures publiques.

## Rapport

`docs-fr/fiches-cloud/E-rapport.md` : les fichiers avant/après avec leurs lignes, la preuve
que `routes.txt` et le journal de démarrage sont identiques, ce que tu n'as pas osé déplacer.
