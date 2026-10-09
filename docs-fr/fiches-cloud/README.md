# Fiches pour les sessions cloud

*Ouvert le 10/10. Le propriétaire a un crédit réservé aux sessions cloud, qui expire le
5 novembre 2026 ; ces fiches lui envoient le travail du chantier de l'audit (doc 32) qui se
vérifie sans la station. Chaque fiche est autonome : une session cloud la lit et l'exécute
seule.*

## Pour le propriétaire : lancer une fiche

1. Ouvrir une **session cloud** (claude.ai/code, ou l'application en choisissant
   l'environnement cloud) sur le dépôt **`chatelp/atc-scribe`**, branche `main`, modèle Opus.
2. Coller le prompt de la fiche, tel qu'il est écrit dans le tableau ci-dessous.
3. Laisser travailler. La session pousse une branche `cloud/…` et écrit son rapport.
   **Elle ne touche jamais `main`.**
4. Le dire à la session atc-scribe du Mac, qui relit la branche, la vérifie (instance
   d'essai, navigateur, garde-fous) et l'intègre.

| Fiche | Vague | Prompt à coller | Branche | Parallèle avec |
|---|---|---|---|---|
| [A — retraits côté serveur](A-retraits-serveur.md) | 1 | `Lis docs-fr/fiches-cloud/README.md puis docs-fr/fiches-cloud/A-retraits-serveur.md, et exécute la fiche A en entier.` | `cloud/a-retraits-serveur` | B, C |
| [B — retraits côté interface, et la carte réparée](B-retraits-interface.md) | 1 | `Lis docs-fr/fiches-cloud/README.md puis docs-fr/fiches-cloud/B-retraits-interface.md, et exécute la fiche B en entier.` | `cloud/b-retraits-interface` | A, C |
| [C — tests de ce qui tourne sans filet](C-tests.md) | 1 | `Lis docs-fr/fiches-cloud/README.md puis docs-fr/fiches-cloud/C-tests.md, et exécute la fiche C en entier.` | `cloud/c-tests` | A, B |

La vague 2 (configuration réduite, découpage de `main()` et de `handlers.go`, tests de l'API)
s'écrira quand la vague 1 sera intégrée : elle touche les mêmes fichiers que A.

## Pour la session cloud : règles communes, à respecter sans exception

**Lis d'abord** `CLAUDE.md` (racine), puis dans `docs-fr/` : `32-audit-produit.md` (sections
« Ce que l'audit doit respecter », « Alignement » et la liste classée) et D71 dans
`05-decisions.md`. Les rapports d'audit bruts sont dans `docs-fr/audit-2026-10-09/`. **Ce
sont des cartes, pas des vérités** : vérifie chaque affirmation dans le code avant d'agir.

**Ce que tu n'as pas, et ne dois pas chercher à avoir** : la station de réception (pas de
`ssh`, aucune adresse `192.168.*`, aucun `*.lan`), la production, le GPU, les bases de
co-atc, l'audio. **Rien de tout cela ne doit jamais entrer dans le cloud.** Le fichier
`configs/config.toml` du propriétaire n'est pas dans le dépôt : tu travailles avec
`configs/config.toml.example`. Tu peux installer des outils dans ta machine (Go ≥ 1.25,
Node ≥ 22, ffmpeg, sqlite3).

**Langue** : code, commentaires, messages de commit en anglais ; ton rapport en français.

**Git** :
- Une branche `cloud/<nom>` partie de `main`, poussée sur `origin`. **Jamais de push sur
  `main`, jamais de fusion, jamais de rebase de `main`, pas de pull request.**
- **Un commit par étape**, qui compile et dont les tests passent ; un retrait = un commit,
  pour qu'on puisse en défaire un seul. Signature en dernière ligne :
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Le message de commit dit ce qui est retiré ou ajouté, **combien de lignes**, et comment tu
  as vérifié que rien ne s'en servait.

**Ne rien casser** (règle du propriétaire) :
- À chaque commit : `go build ./...`, `go vet ./...`, `go test -race ./...` verts ; pour
  `www/` : `node --check` sur chaque fichier JS touché et `node --test www/par/`.
- **Ne renomme rien** de ce qui est enregistré ou appelé ailleurs : les clés `localStorage`
  de `www/`, les noms du store Alpine lus par `index.html` et les autres JS, les routes
  `/api/v1/…` utilisées par `cmd/radio-ctl-sync`, `sidecar/`, `tools/` et `www/`.
- Un doute, une dépendance inattendue, un test qui casse sans raison claire : **arrête
  cette étape**, garde ce qui est fait, et écris-le dans le rapport. Ne devine pas.
- Ne touche pas à `docs/` (à l'amont), ni aux documents de `docs-fr/`, sauf ton rapport.

**Rapport** : à la fin, un fichier `docs-fr/fiches-cloud/<lettre>-rapport.md`, commité sur
ta branche, en français, 80 lignes au plus : la liste des commits avec ce que chacun fait
et ses chiffres ; ce que tu as vérifié et comment ; ce que tu n'as **pas** pu vérifier
(tout ce qui demande un navigateur avec de vrais flux, la station, la production) ; les
doutes ; ce qu'il faut regarder en priorité sur le Mac.
