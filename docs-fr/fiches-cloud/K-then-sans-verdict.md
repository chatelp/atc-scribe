# Fiche K — « Then » : ce que l'avion a fait après une consigne, sans verdict

*Vague 3 (13/10). Étape 8 du plan de migration (`docs-fr/design/04-migration.md`), décision 6
de D78 : la colonne « Status » des consignes entendues (prévue pour issued / complied /
deviation, qui n'affiche jamais que « issued ») devient « Then », qui décrit ce que l'avion a
fait, **sans jamais qualifier un écart**. Branche : `cloud/k-then-sans-verdict`. Lis d'abord
`README.md` de ce dossier : ses règles s'appliquent.*

## Ce que tu livres

De la **logique pure, testée en node**, dans `www/story/then.js` (nouveau dossier, même forme
que `www/par/par-geometry.js` : un module qui s'exporte pour node et se pose sur `window`),
et rien d'autre dans `www/` : ni `index.html`, ni le store, ni les styles (c'est la migration
qui les branche, étape 8).

```
then(instruction, track, options) -> { text, at, kind }   ou null
```

- `instruction` : `{ role, text, text_fr, value, unit, at }` comme l'API les sert
  (`plain` d'une transcription, `GET /api/v1/transcriptions`, et `value`/`unit` de la fiche G ;
  en leur absence, extrais la valeur de `text` avec une règle explicite et testée, par rôle :
  « Descend to FL100 », « Climb to 4000 ft », « Turn left heading 270 », « Reduce speed to 220
  kt », « Contact 119.255 », « Cleared to land, runway 27R ») ;
- `track` : les points de `GET /api/v1/aircraft/{hex}/tracks` **après** `instruction.at`
  (`{ timestamp, alt_baro, track, gs, lat, lon }`), triés ;
- `options` : fenêtre d'observation (défaut 6 min), tolérances (niveau ± 200 ft, cap ± 10°,
  vitesse ± 15 kt), le QNH pour lire un niveau de vol (`www/altitude/altitude-scale.js` le
  sait : réutilise-le, ne le recopie pas).

Ce qu'il rend, en anglais, une phrase courte et factuelle, et `kind` ∈ `level`, `heading`,
`speed`, `frequency`, `runway`, `clearance` :

- niveau : « level at 4 000 ft, 2 min later », « descending through 5 200 ft » (encore en
  route à la fin de la fenêtre), « at FL 110, 6 min later » (aucun mouvement : le même fait,
  sans attente) ;
- cap : « turned to 270° », « heading 254°, still turning » ;
- vitesse : « 220 kt, 1 min later », « slowing, 245 kt » ;
- fréquence : « no later transmission heard here » ou « next heard on 119.255 3 min later »
  (si le store fournit la liste des transmissions suivantes de l'avion, paramètre optionnel) ;
- piste et clairance : « landed on 27R » / « took off from 26L » si la trajectoire le montre
  (altitude au sol, position : une règle simple et dite), sinon null.

**Interdit** : les mots *deviation*, *complied*, *error*, *violation*, *wrong*, et *still*
(il sous-entend que l'avion aurait dû bouger : un verdict discret), **sauf** dans « still
turning », qui décrit un mouvement en cours ; et toute couleur ou classement. Un test l'affirme
sur chaque sortie possible : c'est le principe « constater, ne pas juger » (D78), que le
propriétaire a confirmé.

**Format des nombres**, celui du système de design (`docs-fr/design/systeme/project/README.md`,
Content › Numbers) : milliers séparés par une **espace fine insécable U+202F** (« 4 000 ft »,
jamais « 4,000 ft »), niveau de vol avec une espace (« FL 110 », jamais « FL110 »), et une
virgule avant le délai (« , 2 min later »). Un test vérifie le format de chaque phrase.

## Tests

`www/story/then.test.js`, `node --test www/story/` : au moins vingt cas construits à la main
(trajectoires synthétiques de quelques points), dont : niveau atteint, non atteint, aucune
donnée après la consigne (→ null), trajectoire qui s'arrête (signal perdu), consigne sans
valeur extractible (→ null), QNH donné et non donné. Ajoute `www/story/` à la ligne de tests de
`tools/www-check/check.mjs` (`node --test www/par/ www/altitude/ www/timeline/`) et vérifie
que le filet passe.

## Rapport

`docs-fr/fiches-cloud/K-rapport.md` (voir README). En plus : la liste des phrases possibles
(c'est ce que la session design mettra dans la fiche `AircraftStory`), et les règles d'extraction
de valeur depuis `text` quand `value` manque.
