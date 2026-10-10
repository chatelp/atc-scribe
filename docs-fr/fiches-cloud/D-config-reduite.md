# Fiche D — Une configuration réduite à ce qui est lu

*Palier 5.2 de l'audit (doc 32), sous D71. Branche : `cloud/d-config-reduite`. Lis d'abord
`README.md` de ce dossier : ses règles s'appliquent.*

## Le but

`configs/config.toml.example` fait 538 lignes pour 148 clés (mesuré le 10/10, après la vague
1), dont une partie n'a plus d'effet et une autre répète le défaut du code. L'audit visait
≈ 110 lignes et ≈ 85 clés. Le but : **un exemple court, où chaque clé a un effet et un
commentaire d'une ligne**, et un code qui a un défaut raisonnable pour tout ce qu'il peut
deviner. Carte : `docs-fr/audit-2026-10-09/backend.md`, section 4 (chiffres d'avant la
vague 1 : remesure).

## La contrainte qui prime

**La configuration du propriétaire, qui n'est pas dans le dépôt, doit toujours démarrer, avec
le même comportement.** Elle contient encore des sections et des clés sans effet
(`[atc_chat]`, les clés OpenAI). `internal/config/testdata/legacy-full.toml` en est une copie
figée (fiche A) : son test doit rester vert, et tu l'étends à chaque étape. **Une clé inconnue
se signale par un avertissement au démarrage (une ligne, avec son nom), jamais par un
refus.** Une clé retirée de l'exemple parce qu'égale au défaut doit donner, absente, exactement
la valeur qu'elle donnait présente : écris un test qui charge l'exemple actuel et l'exemple
réduit et compare les deux `Config` champ par champ (`reflect.DeepEqual` après chargement et
validation) — c'est **le** test de cette fiche.

## Étapes, un commit chacune

1. **Mesure** : pour chaque clé de l'exemple, est-elle lue (grep du champ Go hors
   `config.go`), et sa valeur diffère-t-elle du défaut du code ? Écris le tableau dans ton
   rapport. Les clés que l'audit dit différentes du défaut et à garder explicites :
   `flying_min_alt_ft`, `phase_preservation_seconds`, `phase_transition_timeout_seconds`,
   `runway_in_use_approach_weight`, `runway_in_use_climb_weight` ; vérifie.
2. **Des défauts en code** pour les clés qui n'en ont pas et que `Validate` exige
   (`ValidateFlightPhases` en exige 12 ; `[reference]`) : le défaut est la valeur de l'exemple
   actuel. `Validate` continue de refuser une valeur hors bornes, pas une valeur absente.
3. **Les doublons** : `airport_range_nm` dans `[station]` et `[flight_phases]`,
   `airlines_dat_path` dans `[reference]` et `[post_processing]`. Une seule source chacun, l'autre
   lue si présente (compatibilité) avec un avertissement.
4. **Les clés inconnues** signalées sur toutes les sections (seule `[adsb]` le fait), par un
   avertissement, jamais un refus. `go-toml`/`BurntSushi` : utilise les métadonnées de décodage
   (`Undecoded()` ou équivalent) ; vérifie quelle bibliothèque est utilisée.
5. **L'exemple réduit** : sections dans l'ordre d'importance pour quelqu'un qui installe
   (serveur, station, ADS-B, fréquences, transcription locale, stockage, journal, référence,
   météo, phases), un commentaire d'une ligne par clé, les valeurs mesurées gardées avec leur
   raison en une ligne (les commentaires actuels de `[frequencies]` et `[transcription.local]`
   portent des mesures : garde l'essentiel, renvoie à `docs-fr/` pour le reste). L'exemple de
   sources audio (fiche du 10/10 : Icecast, SRT, UDP brut, carte son) reste. Vise ≈ 120 lignes.
6. **La documentation** : README (section configuration, en anglais) et un paragraphe dans ton
   rapport pour le propriétaire : la liste des clés de son `config.toml` qui ne servent plus,
   pour qu'il le réduise lui-même (la session du Mac s'en chargera).

**Ne change aucun comportement** : même `Config` chargée avant et après (étape 1 du test),
`go test -race ./...` vert, banc de démarrage de la fiche A sur l'exemple réduit et sur
`legacy-full.toml`.

## Rapport

`docs-fr/fiches-cloud/D-rapport.md` : le tableau des clés, l'exemple avant/après en lignes et
en clés, la liste des avertissements qu'un `config.toml` d'avant produira.
