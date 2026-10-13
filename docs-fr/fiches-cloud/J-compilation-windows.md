# Fiche J — Le serveur compile pour Windows

*Vague 3 (13/10). Prérequis du portage (Q54 : les machines capables d'affiner sont sous
Windows) et du P0 « installable ailleurs ». Branche : `cloud/j-compilation-windows`. Lis
d'abord `README.md` de ce dossier : ses règles s'appliquent.*

## Le constat, vérifié le 12/10

`GOOS=windows go build ./...` échoue en trois endroits, et nulle part ailleurs :

1. `internal/transcription/sidecar.go` : `syscall.Kill(-pid, …)` sur le groupe de processus du
   sidecar (arrêt propre puis forcé), et le `SysProcAttr{Setpgid: true}` qui crée ce groupe ;
2. `internal/api/server_handlers.go` : `syscall.Statfs` pour l'espace disque ;
3. `cmd/server/add_user.go` : `exec.Command("stty", "-echo")` pour lire un mot de passe sans
   écho.

Les tests de `sidecar_test.go` utilisent aussi `syscall.Kill` pour vérifier qu'un processus
est mort.

## Étapes, dans cet ordre, un commit chacune

1. **Le groupe de processus du sidecar** : un fichier `sidecar_unix.go` (`//go:build !windows`)
   garde exactement le comportement d'aujourd'hui (`Setpgid`, `Kill(-pid, SIGTERM)` puis
   `SIGKILL`) ; un `sidecar_windows.go` arrête le processus par `cmd.Process.Kill()` après le
   délai, sans groupe (documente la limite : un petit-fils resterait). Les tests qui vérifient
   la mort par `syscall.Kill(pid, 0)` passent derrière la même séparation (`!windows`), avec un
   équivalent Windows (`FindProcess` + `Wait`), ou sont exclus de Windows avec un commentaire.
2. **L'espace disque** : `diskfree_unix.go` / `diskfree_windows.go`
   (`GetDiskFreeSpaceExW` par `golang.org/x/sys/windows`, déjà dépendance indirecte ? vérifie
   `go.mod` ; sinon ajoute-la) ; même signature, même champ servi.
3. **Le mot de passe sans écho** : remplace `stty` par `golang.org/x/term.ReadPassword` sur
   tous les systèmes (une seule implémentation, le comportement sur macOS et Linux reste
   identique : pas d'écho, retour à la ligne après).
4. **La preuve** : `GOOS=windows GOARCH=amd64 go build ./...` et `GOOS=linux GOARCH=amd64 go
   build ./...` verts, et un travail CI minimal si le dépôt en a un (regarde `.github/` ;
   sinon n'en crée pas, dis-le). Les tests : `go test -race ./...` sur ta machine (Linux),
   et `GOOS=windows go vet ./...`.

## Ce que tu ne changes pas

Le comportement sur macOS et Linux (c'est la production) : mêmes signaux, mêmes délais,
mêmes messages. Rien dans `sidecar/` (Python), rien dans `www/`.

## Rapport

`docs-fr/fiches-cloud/J-rapport.md` (voir README). En plus : les limites du chemin Windows
(groupe de processus) et ce qu'il reste pour **exécuter** sous Windows (le sidecar MLX n'y
tourne pas : c'est le portage, pas cette fiche).
