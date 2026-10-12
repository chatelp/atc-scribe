# Fiche labo — La précision de l'appariement par niveau d'indices

*De la session atc-scribe pour l'agent du laboratoire (session « Whisper-lab »). Écrite le 12/10
à la demande du propriétaire ; c'est lui qui la lance. Aucun GPU, aucun audio : des secondes de
calcul sur des textes et des bases. Rien à committer dans atc-scribe ; ton script et ton résultat
vont chez toi (`scripts/labo/`, `resultats/labo/`).*

## Pourquoi

Depuis le 11/10, co-atc garde avec chaque transmission **pourquoi** il l'a attribuée à un avion :
le score et les raisons du matcher (`match_score`, `match_reason`, D72). La page en fait trois
barres — indices **forts**, **moyens**, **faibles** — et sa bulle d'aide dit honnêtement que c'est
un **compte d'indices, pas une probabilité** : personne n'a mesuré combien d'appariements sont
justes à chaque niveau. C'est ce qui manque, et seule une oreille peut le fournir : le ciel témoin
(`cmd/phraseology -shuffle`) mesure le hasard global, pas la justesse d'un niveau.

## Ce qu'on a, et ce qu'on n'a pas encore

- **Les annotations de l'atelier** (`resultats/labo/exemples/corrections.json`, ton outil
  `scripts/labo/atelier.py`) : le 12/10 à 11 h 43, **58 clips**, 55 `ok`, 2 `doute`, 1 `en_cours` ;
  journées 30/09 (6), 01/10 (1), 02/10 (12), 03/10 (23), 04/10 (16) ; fréquences 124 350 (39),
  126 425 (18), 125 825 (1). Chaque entrée est indexée par le chemin du clip dans l'archive du
  sidecar (`20261003/124350/t_….wav`), et porte le texte corrigé, ses `tokens`, leurs `etats`, et
  `mots_indicatif`. **Le jeu n'est pas fini : le propriétaire continue d'annoter.** La fiche est donc
  écrite pour être **relancée** ; le premier résultat sera préliminaire, et dira combien il manque.
- **Les bases co-atc** de ces journées : `atc-scribe/data/co-atc-2026-09-30.db` à `…-10-04.db`,
  en lecture seule (ne jamais les ouvrir en écriture : `?mode=ro`). Les transmissions de ces jours
  n'ont **pas** de `match_score`/`match_reason` (antérieures au 11/10) : la preuve se **recalcule**.
- **L'outil** : `atc-scribe/cmd/phraseology` (construis-le depuis atc-scribe à `1e3f12a` ou plus
  récent : `go build -o /chemin/phr ./cmd/phraseology`). En mode `-db … -json sortie.jsonl`, il
  écrit une ligne par transmission avec `id`, `frequency_id`, `created_at`, et `match` :
  `flight`, `hex`, `score`, `reason` (ex. `digits exact + operator named + altitude agrees`),
  `refused` (absent quand attaché). **Les règles de production** (préréglage Mesuré, D73) :
  `-strict -every -second -context 120 -min-digits 3 -partial 0.6 -alnum -fuzzy-operators
  -context-letters -context-digits -sectors secteurs.json`, avec `secteurs.json` =
  `{"124350": {"lat": 49.00896, "lon": 2.554117, "radius_nm": 60, "max_alt_ft": 20000},
  "126425": {…idem…}}` (approche De Gaulle, tailles du 26/09 ; 125 825 sans secteur).
- **La jointure** clip → transmission : le manifeste du jour de l'archive
  (`/Volumes/Crucial X8/whisper-corpus/sidecar-audio/<AAAAMMJJ>/manifeste.jsonl`, une ligne par
  clip, champ `fichier` = la clé de l'annotation, `frequency_id`, `created_at`) ; dans la base,
  la transmission est celle de même `frequency_id` et de même instant `created_at` (compare en
  instants : avant le 11/10 le manifeste écrit l'heure locale avec décalage, la base l'UTC).
  `internal/audioarchive/archive.go` fait exactement cette jointure côté Go, si tu veux la copier.

## Ce qui est demandé

Pour chaque transmission annotée (`statut` = `ok` ; les `doute` à part), dire si **l'avion que la
production lui a attribué est le bon**, puis compter par niveau d'indices.

1. **Le verdict de la production** : la ligne `-json` de la transmission. Trois cas : attachée
   (`match` présent, pas de `refused`), refusée (`refused`), aucune (`match` absent).
2. **La vérité** : ce que l'oreille du propriétaire a gardé ou corrigé sur l'indicatif. C'est ton
   format (`mots_indicatif`, `etats`, le texte corrigé, `flight` réécrit d'après l'ADS-B dans la
   phrase présentée) : **à toi de dire précisément comment tu la lis**, et de l'écrire dans le
   résultat. Trois verdicts par transmission : **juste** (l'avion attribué est celui que l'oreille
   nomme), **faux** (l'oreille nomme un autre avion, ou dit que l'indicatif attribué n'a pas été
   prononcé), **indéterminé** (l'oreille n'a pas d'indicatif, ou un indicatif qu'aucun avion du
   ciel ne porte). Ton `oreille-adsb.py` (29/09) a déjà fait passer les annotations par le juge
   ADS-B : repars de là si c'est plus simple.
3. **Le niveau d'indices**, calculé sur `reason` **exactement comme la page** le fait
   (`www/store/transcripts.js`, `matchStrength`), pour que la mesure parle du chiffre affiché :
   - `exact` = la raison contient `digits exact` ou `flight part exact` ;
   - `named` = `exact`, ou l'une de : `digits suffix`, `digits off by one`, `first digits`,
     `flight part, last letter missing`, `last letters of an aircraft just heard`,
     `only aircraft of the named airline just heard`, `last two digits of an aircraft just heard` ;
   - `support` = nombre de raisons parmi `operator named`, `altitude agrees`, `digits repeated`,
     `letters` ;
   - `against` = présence de `altitude contradicts`, `cruising, not landing` ou
     `airborne, not departing` ;
   - **fort** si `exact` et `support ≥ 1` et non `against` ; **moyen** si (`exact`, ou `named` et
     `support ≥ 1`) et non `against` ; **faible** sinon. `in sector` ne compte pas.
   Les raisons sont séparées par ` + `.
4. **Le résultat**, un fichier `resultats/labo/<date>-precision-par-indices.txt` :
   - par niveau (fort, moyen, faible) et au total : n attachées, justes, fausses, indéterminées,
     **précision = justes / (justes + fausses)** avec son intervalle de Wilson à 95 % ;
   - les refusées et les sans-avion : combien, et parmi elles combien l'oreille aurait nommées
     (ce que la production rate) ;
   - les `doute` à part, mêmes colonnes ;
   - **combien d'annotations manquent** pour que chaque niveau tienne à ± 10 points (n tel que
     l'intervalle de Wilson soit ≤ 0,2 de large au taux observé) ;
   - la règle de lecture de la vérité (point 2), et la liste des cas **faux** avec le texte, pour
     que le propriétaire puisse les vérifier à l'oreille : c'est là que la règle du matcher
     apprendra quelque chose.
5. **Relançable** : un script sous `scripts/labo/` qui refait tout depuis `corrections.json`
   courant, sans rien demander ; le propriétaire annote, tu relances, le fichier de résultat se
   date. Le premier passage, sur ~55 transmissions, donnera sans doute moins de 20 cas par niveau :
   dis-le tel quel, avec les intervalles, plutôt que d'arrondir.

## Ce que j'en ferai

Si un niveau tient (intervalle ≤ 20 points), la bulle de la page dira la précision mesurée au lieu
de « compte d'indices, pas une probabilité », avec la date et le n ; sinon elle reste comme elle
est. Si des cas **faux** se concentrent sur une raison (par exemple « two last digits » ou
« operator named » seul), c'est une règle à remesurer dans `cmd/phraseology` et à inscrire dans
`05-decisions.md` — dis-le dans le résultat, je m'en charge côté co-atc.

## Les garde-fous

- Lecture seule sur `atc-scribe/data/*.db` et sur l'archive ; pas de commit dans atc-scribe ; pas
  d'audio envoyé nulle part (les textes de `corrections.json` sont déjà dans ton dépôt privé).
- Pas de GPU ni de séance à inscrire : si ton script dépasse la minute, c'est qu'il refait quelque
  chose d'inutile.
- Le résultat revient par un fichier dans `echanges-atc-scribe/` (« whisper-lab → atc-scribe »),
  comme le 30/09, avec le chemin du fichier de résultat et une ligne par niveau.
