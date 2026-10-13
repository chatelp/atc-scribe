# Fiche G — rapport (13/10)

Branche `cloud/g-chiffres-avant-connexion`, partie de `main` (37f2fa0). Un commit par étape ; rien dans `www/` ni `docs/`.

| Commit | Étape | Lignes |
|---|---|---|
| 6c9dc4b | 1 — `GET /api/v1/vitals` publique, cache de 10 s sous mutex, réglage `vitals.enabled` (404 coupé), comptage du jour en une requête, nombre de pistes en service | +645 / −3 (297 de tests) |
| 3add80c | 2 — `/health` du sidecar dit `backend` (nom du GPU si `mlx.core` le donne, sinon `mlx`) ; servi en `transcription_backend` | +126 / −4 (79) |
| 5fdf8f2 | 3 — moyenne glissante exponentielle (α = 0,1) du temps par transmission dans `local.go`, servie à une décimale | +155 (79) |
| 69b2075 | 4 — `main.version` / `main.commit` par `-ldflags` (« dev », « unknown » sinon), servis par `/server` et `/vitals` ; commande dans `README.md` et `tools/menubar/README.md` | +84 / −4 (27) |
| 362616f | 5 — `since` (RFC 3339, `null` avant le premier changement) dans chaque entrée de `runway_in_use` de `/api/v1/station` | +53 (38) |
| 3152f5c | 6 — `Instruction` gagne `Value` et `Unit`, recopiés par `PlainInstruction` et `plainOf` | +88 / −27 (52) |

**Liste figée des routes** : une ligne de plus, `GET /api/v1/vitals (*Handler).GetVitals open`.

## Exemple de réponse (valeurs fictives)

```json
{"aircraft":87,"messages_per_second":1240,"farthest_nm":212,"channels_on_air":4,
 "transmissions_today":1312,"matched_today":871,"matched_share":66,
 "transcription_model":"Whisper large-v3 ATCO2","transcription_backend":"Apple M2 Pro",
 "seconds_per_transmission":1.8,"airports_followed":2,"runways_in_use":3,
 "uptime_seconds":86400,"version":"v0.2.0-14-g3152f5c","commit":"3152f5c"}
```

Une valeur inconnue est absente, pas à zéro (`farthest_nm`, `transcription_*`, `seconds_per_transmission`).

## Règle de raccourcissement des noms de modèles (`api.ModelFamily`)

Garder le dernier segment du chemin (le propriétaire Hugging Face et tout dossier local tombent : un chemin peut
porter un nom d'utilisateur) ; couper sur `-` et `_` ; retirer les mots d'emballage (mlx, asr, hf, ct2, ggml, gguf,
fp16, fp32, int8, 4bit, 8bit, q4, q8) ; souder une taille à la version qui suit (`large` `v3` → `large-v3`) ;
majuscule au premier mot et aux langues ; en capitales un mot de lettres terminé par des chiffres qui n'est pas une
version (`atco2` → `ATCO2`) ; le reste tel quel ; 48 caractères au plus. Testé : `sfabriece/whisper-large-v3-atco2-asr-mlx`
→ « Whisper large-v3 ATCO2 », `mlx-community/whisper-large-v3-turbo` → « Whisper large-v3 turbo »,
`/Volumes/Models/whisper-large-v3-french-mlx` → « Whisper large-v3 French », `openai/whisper-small` → « Whisper small ».

## Vérifié, et comment

- À chaque commit : `go build`, `go vet`, `go test -race ./...` verts, sauf `TestAChoiceThatCannotBeWrittenChangesNothing`
  (`internal/auth`), rouge **déjà sur `main`** en root, PASS rejoué sous `nobody` ; `py_compile` du sidecar,
  `unittest` de `sidecar/test_device.py` et `test_loops.py` ; `node --test www/par/` vert.
- Banc de démarrage (comme A et D) à chaque étape : copie de l'exemple, faux tar1090 (un avion), faux sidecar
  (`/health` avec `model_en` et `backend: "fake"`, `/transcribe` en 150 ms), source tonale `lavfi` coupée en
  segments de 2 s : `/api/v1/health` et `/` 200, la route sans session rend `seconds_per_transmission: 0.2`,
  `transmissions_today: 2`, `transcription_backend: "fake"`, `farthest_nm: 17` ; construit avec les `-ldflags`, le
  journal, `/server` et `/vitals` disent le commit ; arrêt propre sur SIGINT.
- Tests de la route : deux visites dans la même seconde = un calcul, 9,9 s = toujours un, 10,1 s = deux ; clés
  énumérées ; aucun nombre à plus de deux décimales ; aucune chaîne à trois décimales ni indicatif ; ni la
  position, ni les indicatifs, ni « LFPG », « 125.825 », le nom ou l'URL de la fréquence, ni le propriétaire du
  modèle dans le corps ; 404 coupé, réglage gardé par un PUT qui ne le nomme pas, persistant au redémarrage.

## Écarts à la fiche, à regarder

1. **`COUNT(callsign != '')` compte faux** : mesuré dans SQLite sur (« AFR1 », « », NULL) → 2 au lieu de 1 (la
   comparaison vaut 0, pas NULL). J'utilise `COUNT(NULLIF(callsign, ''))`, toujours une seule requête.
2. **La regex `[A-Z]{3}[0-9]` refuse le nom de modèle de la fiche** : « ATCO2 » contient « TCO2 ». Le test l'applique
   à une frontière de mot, `\b[A-Z]{3}[0-9]` ; un indicatif est un mot à lui seul.
3. **`GET/PUT /api/v1/settings` n'existe pas** : les réglages du panneau passent par `GET /api/v1/server`
   (`settings.vitals.enabled`) et `PUT /api/v1/server/settings` ; `vitals` y est rangé comme `matching`.
4. **Le sidecar n'est pas appelé par la route** : elle lit sa dernière santé (`LastHealth`), relue à chaque
   démarrage et redémarrage ; plus strict que « un appel par fenêtre ».
5. **`runways_in_use`** = pistes de l'ensemble actif de chaque tracker (probabilité ≥ 15 % + parallèles), une piste
   comptée une fois dans les deux sens : De Gaulle face à l'est = 4. À confronter à ce que veut la session design.
6. **`since` reste `null` après la première détection** (pas un changement, comme le dit la fiche).

## Pas vérifié, doutes

- Ni station, ni production, ni navigateur (la page viendra avec la migration).
- **Le vrai `mlx.core`** : la clé `device_name` de `mx.device_info()` / `mx.metal.device_info()` n'est pas vérifiée
  sur le Mac ; si elle manque, `backend` vaut « mlx ». À lire : `curl 127.0.0.1:8178/health`.
- `channels_on_air` compte le statut `connected`, posé quand ffmpeg démarre, pas quand l'audio coule : une source
  morte compte quelques secondes, jusqu'à l'échec de ffmpeg.
- Le temps par transmission inclut l'attente dans la file du sidecar (délai vécu, pas décodage seul) ;
  `transmissions_today` lit la base ouverte, qui tourne à minuit à la minute près.

## À regarder en priorité sur le Mac
1. `/health` du vrai sidecar : `backend` et `model_en`, puis `/api/v1/vitals` sur l'instance d'essai.
2. Les scripts de lancement : ajouter les `-ldflags` (README), sinon la route dit « dev ».
3. Le chiffre `runways_in_use` sur une journée d'Orly + De Gaulle, et `since` après un changement de configuration.
