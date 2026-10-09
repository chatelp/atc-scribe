# Fiche C — Des tests pour ce qui tourne sans filet

*Palier 5.4 de l'audit (doc 32), en avance. Branche : `cloud/c-tests`. Lis d'abord
`README.md` de ce dossier : ses règles s'appliquent.*

## Le but

La couverture du serveur est de 26,8 % (`docs-fr/audit-2026-10-09/backend.md`, section 1), et
des paquets qui portent la production tous les jours n'ont presque rien : `storage` 14,8 %,
`audio` 16,8 %, `frequencies` 44,9 %. Écrire les tests qui manquent **pour ce qui reste**,
pour que les retraits et le découpage à venir se fassent sous filet. **Tu n'ajoutes que des
fichiers `_test.go`** (et, si besoin, des données de test dans `testdata/`).

## Périmètre, par ordre de priorité

1. **`internal/storage/sqlite`** : écriture et relecture des positions (`adsb_targets` :
   `INSERT OR IGNORE` et son dédoublonnage par la contrainte `UNIQUE`, les requêtes d'historique
   et de « dernière position »), des transcriptions (dont `callsign_source`,
   `callsign_evidence`, `content_second`), des changements de phase ; la rétention par taille
   (`cleanupOldDailyDatabases` dans `cmd/server`, ou là où elle vit) : elle ne doit jamais
   supprimer la base du jour, et supprime les plus anciennes d'abord. Des tests de rotation
   existent déjà (`rotating_test.go`) : ne les duplique pas.
2. **`internal/audio`** : `MultiReader` (plusieurs lecteurs, un lent ne bloque pas les autres,
   fermeture), le traitement de l'en-tête WAV (`wavreader.go`), la construction de la commande
   ffmpeg pour chaque type de source (HTTP, UDP brut avec options d'entrée, périphérique) —
   deux tests existent déjà dans `central_processor_test.go`.
3. **`internal/frequencies`** : les sources ajoutées à chaud (des tests existent :
   `runtime_sources_test.go`), le service de flux par client (`StreamProcessor` : ajout et
   retrait de clients, nettoyage des inactifs).

**Hors périmètre** (la fiche A les retire en parallèle) : `internal/atcchat`,
`internal/templating`, `internal/simulation`, le chemin de transcription OpenAI
(`openai.go`, `processor.go`, `post_processor.go`, `chunker.go`). **Hors périmètre aussi**,
pour la vague 2 : `internal/api` (la fiche A change la signature de `NewHandler`).

## Si un test révèle un défaut

**Ne corrige pas le code dans cette fiche.** Écris le test qui le montre, marque-le
`t.Skip("defect: …")` avec une phrase qui décrit le défaut, commite-le à part, et décris-le
dans le rapport (fichier, ligne, ce qui se passe, ce qui devrait se passer). La correction se
fera sur le Mac, avec la production à l'esprit.

## Vérifications

À chaque commit : `go vet ./...`, `go test -race -count=1 ./...` verts (les tests marqués
`Skip` exceptés), et la couverture des paquets visés avant et après
(`go test -cover ./internal/storage/... ./internal/audio/ ./internal/frequencies/`). Les tests
ne dépendent ni du réseau ni de l'heure (pas de `time.Now()` sans contrôle), et durent moins
de 10 s par paquet. ffmpeg est requis par certains tests de `frequencies` : installe-le.

## Rapport

`docs-fr/fiches-cloud/C-rapport.md` (voir README). En plus : la couverture avant et après par
paquet, et la liste des défauts trouvés, s'il y en a.
