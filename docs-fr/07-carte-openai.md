# Où Co-ATC appelle OpenAI — la carte du chantier de remplacement

*Relevé le 15 septembre 2026 par lecture du code, dépôt amont `yegors/co-atc`,
branche `main`, commit `0e4d685` du 3 mai 2026 (« fix(transcription): harden OpenAI
realtime recovery »). Les numéros de ligne renvoient à ce commit.*

Ce document remplace les suppositions de `02-co-atc-amont.md` sur ce point précis.
Tout ce qui suit a été lu dans les sources, pas déduit de la page du dépôt.

---

## 1. La découverte qui change le plan

**L'auteur amont a déjà conçu le remplacement local. Il ne l'a jamais écrit.**

Le dépôt contient `docs/LOCAL-STT.md` (536 lignes) — un document de conception complet
pour un moteur faster-whisper local, avec architecture, API du service annexe, comparatif
de modèles et scripts d'installation. Et `docs/LOCAL-SST.md`, une version antérieure du
même document.

Vérifié : **rien de tout cela n'existe dans le code.**

| Ce que `LOCAL-STT.md` annonce | Réalité mesurée |
|---|---|
| `sidecar/whisper_server.py` | le dossier `sidecar/` n'existe pas |
| `scripts/setup_local_stt_mac.sh` | `scripts/` ne contient que `download_assets.{sh,ps1}` et `get_phases.ps1` |
| `backend = "local"` dans la config | le mot `backend` n'apparaît nulle part dans `internal/config/config.go` sauf pour le type de stockage |
| `LocalProcessor` en Go | `internal/transcription/` ne contient que le chemin OpenAI |

**Conséquence pour nous, et elle est bonne.** L'architecture cible du fork n'est pas à
inventer : elle est écrite, par l'auteur amont, dans son propre dépôt. Un fork qui
*implémente* le plan documenté de l'amont est beaucoup plus facile à faire accepter en
retour qu'un fork qui impose une architecture étrangère. C'est un argument direct pour
la consigne « le fork reste rebasable ».

**Deux réserves, et elles portent sur le cœur du sujet :**

1. Le plan amont garde **le post-traitement sur GPT-4o** (« PostProcessor (GPT-4o) —
   Works with either backend », schéma de `LOCAL-STT.md`). Il supprime le coût de la
   transcription, pas la dépendance à OpenAI. **La décision D3 va plus loin que le plan
   amont** : nous devons aussi remplacer le post-traitement. C'est la question Q6.
2. Le plan est **monolingue**. Il ne dit pas un mot du choix de langue, et recommande
   faster-whisper — dont il note lui-même que « CTranslate2 has no MPS support — CPU
   only » sur Apple Silicon. Sur le M4, ça écarte le moteur qu'il recommande. Q2 reste
   entière.

---

## 2. Les cinq appels sortants, un par un

| # | Fonctionnalité | Point de code | Destination | Sort |
|---|---|---|---|---|
| 1 | Session de transcription | `internal/transcription/openai.go:139` | `POST https://api.openai.com/v1/realtime/transcription_sessions` | **à remplacer** |
| 2 | Flux de transcription | `internal/transcription/openai.go:213` | `wss://api.openai.com/v1/realtime?intent=transcription` | **à remplacer** |
| 3 | Post-traitement, unité | `internal/transcription/openai.go:385` | `POST https://api.openai.com/v1/chat/completions` | **à remplacer** (Q6) |
| 4 | Post-traitement, lot | `internal/transcription/openai.go:503` | `POST https://api.openai.com/v1/chat/completions` | **à remplacer** (Q6) |
| 5 | Assistant vocal | `internal/atcchat/realtime_client.go:145` | `POST https://api.openai.com/v1/realtime/sessions` | **neutralisé** (Q5) |

Et un sixième appel sortant, qui n'est pas OpenAI mais qui sort quand même :

| 6 | Météo | `internal/weather/client.go:32-44` | `https://node.windy.com/airports/{metar,taf,notams}/{OACI}` | **à remplacer** (Q7) |

**Aucune bibliothèque cliente OpenAI dans `go.mod`.** Tous ces appels sont écrits à la
main en `net/http` et `gorilla/websocket`. C'est une bonne nouvelle : il n'y a pas de
dépendance à désinstaller, seulement du code à ne plus appeler.

---

## 3. Le chemin de l'audio, de la fréquence au texte

```
[[frequencies.sources]] url
        │
        ├── http(s)://…  ──► ffmpeg ──┐        internal/audio/central_processor.go:374
        └── srt://…      ──► gosrt  ──┤        internal/audio/srt_reader.go
                                      ▼
                            MultiReader (PCM s16le)   internal/audio/multireader.go
                                      │
                                      ▼
                            AudioChunker (chunk_ms)   internal/audio/chunker.go
                                      │
                                      ▼
                     base64 ──► "input_audio_buffer.append"   processor.go:288
                                      │
                                      ▼
                          OpenAI Realtime (websocket)
                                      │
            ┌─────────────────────────┴─────────────────────────┐
            ▼                                                   ▼
"…transcription.delta"  processor.go:508        "…transcription.completed"  processor.go:521
            │                                                   │
            └─────────────────────────┬─────────────────────────┘
                                      ▼
                        processTranscriptionEvent()   processor.go:572
                                      │
                      ┌───────────────┼───────────────┐
                      ▼               ▼               ▼
                   SQLite       WebSocket UI     fichier log
```

**Les octets que voit OpenAI sont exactement ceux-ci** (`central_processor.go:396-406`) :

```
ffmpeg -loglevel error -fflags nobuffer -flags low_delay \
       -reconnect 1 -reconnect_at_eof 1 -reconnect_streamed 1 \
       -reconnect_delay_max <n> -i <url> \
       -f s16le -acodec pcm_s16le -ac 1 -ar 24000 -flush_packets 1 pipe:1
```

PCM 16 bits signé, petit-boutiste, mono, **24 000 Hz**. Whisper travaille nativement à
16 000 Hz : un rééchantillonnage sera nécessaire quelque part. Le plan amont le met dans
le service annexe Python plutôt que dans Go — c'est le bon choix, il laisse la chaîne
audio Go intacte.

> **Ce que ça veut dire pour la station.** Co-ATC sait consommer une URL HTTP par
> fréquence, et c'est exactement ce que produira le mode de diffusion par canal
> (D2, `/ch-118700.mp3`). Aucun développement Go n'est nécessaire côté ingestion :
> une entrée `[[frequencies.sources]]` par point de montage Icecast suffit.
> **Non mesuré à ce stade** : le comportement de ffmpeg sur un flux Icecast qui se
> tait pendant des minutes — c'est le cas normal d'une fréquence à 3 % d'occupation.

---

## 4. La couture où brancher le remplacement

Elle est déjà là, et elle est propre.

```go
// internal/transcription/interface.go
type ProcessorInterface interface {
    Start() error
    Stop() error
}
```

`NewProcessor()` (`processor.go:51`) **retourne déjà un `ProcessorInterface`**, pas un
`*Processor`. Et sa toute première instruction est :

```go
if config.OpenAIAPIKey == "" {
    return nil, fmt.Errorf("OpenAI API key is required for transcription processor")
}
```

**Le plan d'implantation en découle sans effort** : ajouter `backend` à la configuration,
écrire un `LocalProcessor` qui satisfait la même interface, et aiguiller dans
`NewProcessor`. Rien d'autre dans le code n'a besoin de savoir d'où vient le texte —
`manager.go`, le stockage, le WebSocket et l'interface web sont en aval de la couture.

C'est très exactement l'architecture de `LOCAL-STT.md`. L'amont a fait le travail de
conception ; il reste le travail d'écriture.

---

## 5. Ce que le produit attend du texte, précisément

### 5.1 La transcription brute

Le prompt envoyé au modèle est `prompts/transcription_prompt.txt`, **un seul paragraphe,
en dur, entièrement tourné vers Toronto** : il se termine par « This is Toronto (CYYZ)
Tower frequency, and these are the runways in use: 05, 23, 06 Right… ».

Il demande trois choses qui nous concernent directement :

- « Spell out all numbers (9 -> nine) » — **les chiffres en toutes lettres** ;
- « NATO phonetic alphabet is extensively used » ;
- « Use [unclear] or [inaudible] for unclear phrases ».

Whisper ne fait spontanément aucune des trois. En français, la deuxième est même
douteuse : l'alphabet OACI est utilisé en français aussi, mais prononcé à la française.
**C'est un point à mesurer sur le corpus, pas à supposer.**

Le texte produit est stocké tel quel dans `transcriptions.content`
(`internal/storage/sqlite/transcriptions.go:55-65`) :

```sql
CREATE TABLE transcriptions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    frequency_id      TEXT NOT NULL,
    created_at        TIMESTAMP NOT NULL,
    content           TEXT NOT NULL,   -- brut, du modèle
    is_complete       BOOLEAN NOT NULL,
    is_processed      BOOLEAN NOT NULL,
    content_processed TEXT,            -- corrigé, du post-traitement
    speaker_type      TEXT,            -- "ATC" | "PILOT"
    callsign          TEXT             -- indicatif OACI, ex. "AFR1234"
)
```

**Le contrat de la transcription brute est donc minimal : du texte.** Pas de segments
horodatés, pas de JSON, pas de score de confiance. Le champ `Type` de
`TranscriptionEvent` (`models.go`) ne vaut que `"delta"` ou `"completed"`. Un moteur
local qui rend une phrase par transmission remplit ce contrat sans adaptation.

C'est plus simple que ce que `03-transcription.md` redoutait. **En revanche le modèle de
données ne prévoit aucun champ pour la langue détectée** — sur une station bilingue,
c'est une information qu'on voudra garder. Une colonne `language` est à ajouter : c'est
une divergence assumée avec l'amont, à signaler comme telle.

### 5.2 Le post-traitement, qui en demande beaucoup plus

`prompts/post_processing_prompt.txt` fait 90 lignes et réclame **cinq choses** d'un coup :

1. corriger les erreurs de transcription à partir du contexte ;
2. écrire les nombres en chiffres — l'inverse exact de la consigne donnée au
   transcripteur ;
3. décider si le locuteur est `ATC` ou `PILOT` ;
4. rattacher chaque transmission à un indicatif **pris dans la liste des avions
   réellement présents dans l'espace aérien** ;
5. extraire les clairances de décollage, d'atterrissage et d'approche, avec la piste.

L'entrée est un lot JSON (`post_processor.go:220`, `json.MarshalIndent`) de transcriptions
non traitées plus quelques traitées pour le contexte. Le prompt système est **un gabarit
Go** rendu à chaque appel par `internal/templating/` : `{{.Aircraft}}`, `{{.Runways}}`,
`{{.Weather}}`, `{{.Airport}}`, `{{.Time}}` y sont remplacés par l'état courant de
l'espace aérien.

> **C'est l'astuce centrale du produit, et il faut la mesurer avant de la remplacer.**
> Le modèle ne devine pas les indicatifs : on lui donne la liste des avions que l'ADS-B
> voit en ce moment, et on lui interdit d'en inventer d'autres. Sur notre station, cette
> liste fait **128 avions**, mesurés aujourd'hui à 11 h.

**Ce que ça dit de Q6.** Les points 2 et 5 sont des règles : une grammaire de
phraséologie les couvre. Le point 4 est un **appariement approximatif** entre des chiffres
mal transcrits et une liste courte et connue — c'est de la distance d'édition pondérée,
pas un modèle de langue. Le point 3 se devine largement des tournures. **Le point 1, la
correction libre, est le seul qui demande vraiment un modèle de langue** — et c'est aussi
celui dont on peut se passer, puisque `content` brut reste stocké à côté.

Q6 est donc plus prometteuse que prévu, mais **rien de tout cela n'est mesuré** : il faut
le jeu de test annoté de `04-corpus.md` avant d'affirmer quoi que ce soit.

### 5.3 Ce que le post-traitement couvre exactement, et ce qui tombe sans lui

Vérifié le 15/09 en suivant les quatre champs jusqu'à l'interface. **Ce n'est pas un
correcteur de texte : c'est ce qui transforme un flux de texte anonyme en événements
structurés rattachés à des avions.**

| Sortie | Ce que ça alimente | Sans post-traitement |
|---|---|---|
| `content_processed` | le texte affiché | **dégradation propre** : `index.html:1868` retombe sur le texte brut, avec le corrigé en infobulle quand il existe |
| `speaker_type` | couleur du liseré (orange = ATC, vert = pilote), pastille, route `GET /transcriptions/speaker/{type}` | liseré gris, pastille absente |
| `callsign` | **la clé de jointure transmission ↔ avion**, route `GET /transcriptions/callsign/{callsign}` | **plus aucune transmission n'est rattachée à un avion** |
| `clearances` | table SQLite dédiée, événement WebSocket `clearance_issued`, alerte dans l'interface, champ `Clearances[]` dans la charge utile de l'avion (`internal/adsb/models.go:262`) | **la fonctionnalité disparaît entièrement** |

**Le champ qui compte est `callsign`.** Les trois autres sont du confort ou une
fonctionnalité isolée ; celui-là est ce qui fait que le produit croise vraiment la radio
et l'ADS-B. C'est la promesse de Co-ATC.

**Le mécanisme.** Le prompt système n'est pas figé : c'est un **gabarit Go re-rendu à
chaque lot** par `internal/templating/`, où `{{.Aircraft}}`, `{{.Runways}}`, `{{.Weather}}`,
`{{.Airport}}` et `{{.Time}}` sont remplacés par l'état courant de l'espace aérien.
`PostProcessorFormattingOptions()` plafonne à **100 avions**, et chaque avion occupe une
ligne dense : indicatif, compagnie, exploitant, type, catégorie de turbulence, cap, vitesses,
altitude, taux, transpondeur, distance et relèvement, ETA, phase, état de la télémétrie.

**Le coût.** Le lot part toutes les 10 s, 20 transcriptions au maximum, plus 3 traitées
pour le contexte. **Mais `processNextBatch()` sort immédiatement s'il n'y a rien à
traiter** (`post_processor.go:156`) : la dépense suit le volume de parole, pas le temps
qui passe. En revanche, quand ça parle, **tout l'espace aérien repart dans le prompt à
chaque lot**. Ordre de grandeur non mesuré : une centaine d'avions à environ 50 jetons la
ligne, plus 90 lignes d'instructions, soit **6 à 7 000 jetons de prompt système par
appel**. C'est ce que le README amont appelle « this will eat up your API credits ».

> **Conséquence pour le fork.** Remplacer le post-traitement n'est pas un raffinement
> tardif : sans lui, on a une carte d'un côté et un flux de texte de l'autre, sans lien.
> Mais la dégradation étant propre, **on peut livrer la transcription locale d'abord et
> le post-traitement ensuite** — le produit reste utilisable entre les deux. C'est la
> bonne découpe.

---

## 6. Suite immédiate

1. **Q2** — conversion du modèle de jacktol vers MLX. *(en cours)*
2. Annoter 120 transmissions du corpus. Rien de sérieux ne se décide avant.
3. Écrire le service annexe et le `LocalProcessor`, en suivant `LOCAL-STT.md` amont.
4. Post-traitement par règles : prototyper l'appariement d'indicatifs contre la liste
   ADS-B réelle et **mesurer** son taux de bonne association.
