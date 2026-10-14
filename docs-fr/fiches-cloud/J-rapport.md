# Rapport de la fiche J — le serveur compile pour Windows (13/10)

**Branche : `claude/fervent-galileo-09dct9`** (la session cloud impose sa branche ; mêmes commits poussés aussi
sur `cloud/j-compilation-windows`). Partie de `main` (`c783e88`) : 4 commits et ce rapport, rien sur `main`, pas de
pull request. Go 1.24.7, en root. Total : +278 / −130 lignes dans 12 fichiers ; 96 lignes de tests déplacées telles
quelles, un test de 30 lignes ajouté.

## Commits

| Commit | Ce qu'il fait | Lignes |
|---|---|---:|
| `d2d24ea` transcription: split the sidecar's process group by OS | `sidecar_unix.go` (`!windows`) reprend le code d'avant derrière quatre fonctions : `startInOwnGroup` (`Setpgid`), `terminateGroup` (SIGTERM à `-pid`, repli sur le pid), `killGroup` (SIGKILL à `-pid`), `inGroupOf` (`Getpgid`). `sidecar_windows.go` : pas de groupe, `Stop` attend son délai puis `Process.Kill()`. Les trois tests à `syscall.Kill` passent dans `sidecar_unix_test.go`, inchangés ; un test ajouté pour `inGroupOf` | +214 / −107 |
| `b1c7609` api: read free disk space through a per-OS helper | `diskSpace(dir)` : `Statfs` à l'identique sous Unix, `GetDiskFreeSpaceEx` (`golang.org/x/sys/windows`) sous Windows ; mêmes champs `disk_free_bytes` / `disk_total_bytes`, à zéro si illisible | +39 / −6 |
| `b2847c5` server: read the new account's password through golang.org/x/term | `readPassword` : `term.IsTerminal` + `term.ReadPassword` partout ; dans un tube, même avertissement et même lecture qu'avant | +19 / −17 |
| `5486361` server: keep the startup tests off Windows | `startup_test.go` exclu de Windows (`!windows` + commentaire) | +6 / −0 |

**Dépendances** : `golang.org/x/sys` v0.31.0, déjà là en indirecte, devient directe. `golang.org/x/term` **v0.30.0**
ajoutée, choisie parce qu'elle n'exige ni `x/sys` plus récent ni directive `go` plus haute (`go.mod` : `go 1.23.2`).

## Écarts entre la fiche et le code

- **Un quatrième point** : `syscall.Getpgid` dans `notOurs()` (`sidecar.go`, la reconnaissance d'un sidecar lancé
  par un script enveloppe). Traité dans le commit 1 (`inGroupOf`).
- **Le point 3 n'était pas une erreur de compilation.** `exec.Command("stty")` compile sous Windows ; une fois les
  points 1 et 2 corrigés, `GOOS=windows go build ./...` passait déjà. Le défaut était à l'exécution : pas de `stty`,
  donc l'avertissement, et **le mot de passe tapé en clair**. Corrigé quand même, comme la fiche le demande.
- **`cmd/server/startup_test.go`** (non cité) empêchait `GOOS=windows go vet ./...` : il lance `main()` dans un
  groupe à lui, l'arrête par SIGTERM et le tue par groupe. Ses aides ne servent qu'à lui (vérifié par `grep`) :
  fichier exclu de Windows plutôt qu'adapté, faute de pouvoir l'exécuter sous Windows ici.

## Ce que j'ai vérifié, et comment

- **À chaque commit, dans un arbre de travail séparé** (`git worktree`) : `go build ./...`, `go vet ./...`,
  `GOOS=darwin GOARCH=arm64 go build ./...`, `go test -race ./...`, plus `GOOS=windows go vet` des paquets touchés.
- **Preuve finale** : `GOOS=windows GOARCH=amd64 go build ./...`, `GOOS=linux GOARCH=amd64 go build ./...`,
  `GOOS=windows go vet ./...`, verts ; en plus `windows/arm64` (build) et `darwin/arm64` (build et vet).
  `go build ./cmd/server` pour Windows produit un `PE32+ executable (console) x86-64`, 21 Mo.
  **Pas de CI** : le dépôt n'a pas de `.github/`, je n'en ai pas créé.
- **Sidecar** : les 15 tests d'avant passent (3 déplacés, mêmes corps) ; le nouveau test d'`inGroupOf` échoue
  bien si la fonction répond toujours `false` (mutation essayée, puis retirée).
- **Mot de passe**, avec le binaire avant et après, sous Linux : dans un vrai pseudo-terminal (`pty.fork` de
  Python), pas d'écho, retour à la ligne après chaque saisie, le rôle reste affiché, et le hash imprimé vérifie
  le mot de passe tapé (`auth.VerifyPassword`) ; dans un tube, sortie identique octet pour octet (avertissement,
  refus d'un mot de passe court, « the two entries differ »).

## Un test qui échoue ici, et pas à cause de la fiche

`internal/auth` `TestAChoiceThatCannotBeWrittenChangesNothing` échoue **aussi sur `main`** dans cette session :
elle tourne en root, et un dossier en `0500` n'empêche pas root d'écrire. Le binaire de test du paquet, lancé en
utilisateur `nobody` (`setpriv`), passe en entier. Rien à corriger dans le code ; à savoir pour les prochaines
sessions cloud.

## Les limites du chemin Windows

- **Pas de groupe de processus.** Un petit-fils du sidecar (le `resource_tracker` de Silero, par exemple) survivrait
  à l'arrêt. Il faudrait un *Job Object* Windows (`CreateJobObject`, `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`).
- **Pas d'arrêt propre.** Windows n'a pas de SIGTERM : `Stop` attend ses 5 s pour rien, puis tue. Le journal dit
  alors « did not stop on SIGTERM, killing », ce qui est faux sous Windows. Délai et message gardés pour suivre la
  fiche à la lettre ; à trancher (tuer tout de suite sous Windows ?).
- **Un sidecar lancé par un script enveloppe est refusé** : sans groupe, `notOurs` ne reconnaît que le pid exact
  de l'enfant. Une commande qui lance directement l'interpréteur passe.

## Ce que je n'ai pas pu vérifier

Rien n'a été **exécuté** sous Windows : ni le serveur, ni l'arrêt du sidecar, ni `GetDiskFreeSpaceEx`, ni la saisie
du mot de passe dans une console Windows, ni Ctrl-C (le `signal.NotifyContext` sur SIGINT/SIGTERM/SIGHUP compile,
son comportement n'est pas vérifié). Les autres tests du sidecar lancent `sh -c` : sous Windows ils échoueraient à
l'exécution, ils ne sont pas exclus. Aucune station, aucun flux, aucun navigateur.

## Ce qu'il reste pour exécuter sous Windows (le portage, pas cette fiche)

Le sidecar MLX ne tourne que sur Apple Silicon : il faut un autre moteur derrière le même contrat HTTP. Il faut un `ffmpeg` Windows (ses options d'entrée : `dshow` au lieu d'`avfoundation`
ou `alsa`, la configuration d'exemple ne cite que ces deux-là). Puis le Job Object ci-dessus.

## À regarder en priorité sur le Mac

1. `sidecar.go` / `sidecar_unix.go` : que l'arrêt du sidecar et la reconnaissance par groupe soient bien ceux
   d'avant (le diff de `sidecar.go` est court, `sidecar_unix.go` reprend les lignes retirées).
2. `-add-user` dans le Terminal macOS : pas d'écho, retour à la ligne, comme avant (vérifié sous Linux seulement).
3. `GET /api/v1/server` : `storage.disk_free_bytes` inchangé sur le Mac.
