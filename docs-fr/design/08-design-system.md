# Le design system Clairance

*13/10/2026, session design. Le système est publié comme artefact Claude de type « Design System »
(privé, au propriétaire) : https://claude.ai/artifact/Ut6hWDRRUcWC58HHz6UP2K. Sa source est dans
le dépôt, `systeme/project/`, publiée telle quelle. Il est écrit en anglais, comme le code et le
README : c'est ce que lira la session qui migre `www/`.*

## Ce qu'il contient

- **Le livre de marque** (`project/README.md`) : le ton (constater sans juger, dire d'où vient
  un fait déduit, les nombres et leurs unités), la couleur, la typographie, l'espace, la
  disposition (A, B replié, C suivre un vol), les états et le mouvement, l'iconographie,
  l'accessibilité. Ce sont des règles d'emploi qui nomment les jetons, pas des explications :
  les raisons restent dans `02-principes.md` et `03-systeme.md`.
- **95 jetons** (`project/tokens.json`), produits par `tools/design/ds-export.mjs` depuis
  `jetons.json`, qui reste la seule source : couleurs, deux familles, dix styles de texte, espace,
  rayons, ombre, tailles, plans, durées. Le 13/10, quatre jetons de taille ont été ajoutés pour que
  les composants n'aient plus de hauteur en dur : `size.control-large` (32), `size.panel-head`
  (40), `size.tabs-head` (44), `size.bar` (48).
- **42 fiches** (40 composants et 2 compositions), chacune avec ses règles et un aperçu en direct. Le 13/10 au soir, douze se sont ajoutées pour les écrans finaux (`09-ecrans.md`) : AppShell, ThreadHead, ChannelMenu, MapControls, KeyValues, Badge, Weather, Notices, ParView, SettingsPage, SignIn, PhoneShell ; avec la densité tactile et le bouton radio. Les premières : Button,
  FilterToggle, Checkbox, Tabs, SegmentedControl, SearchField, Slider ; StatusLight, Counter,
  MatchStrength, LevelMeter ; PhaseBadge, Chips ; ChannelButton, Exchange ; AircraftStrip,
  ArrivalSequence ; SummarySentence, EventList, AircraftStory ; MapLabel, ParFrame ; Panel,
  HelpBubble ; EmptyState, ErrorBanner ; TopBar ; et les deux onglets de la colonne,
  ContextColumn et TrafficColumn. Chaque fiche dit d'où viennent ses données dans le store ou
  l'API, vérifié avec la session atc-scribe le 13/10.
- **Les ressources** : les douze fichiers du signe (`identite/`) et les neuf avions de la famille
  (`avions/famille/`), copiés tels quels.

## Un choix d'architecture : des classes CSS, pas des composants React

L'outil est écrit en Alpine et en CSS (Tailwind compilé, `style.css`). Des composants React
seraient beaux dans l'artefact mais inutilisables dans `www/`. Les composants sont donc des
**classes CSS** préfixées `cl-` (`components/bundle.css`), avec des parties (`cl-exchange__head`)
et des états en classes (`is-selected`, `is-on`) qu'Alpine pose par `:class`. Le préfixe évite
toute collision avec Tailwind et `style.css` pendant la migration. Les aperçus sont du HTML
statique sur ces classes.

Les variables CSS sont **les mêmes** que celles des maquettes (`--color-surface-panel`,
`--space-2`…), vérifié le 13/10 : les 95 noms coïncident. L'étape 1 de la migration
(`04-migration.md`) les installe dans `www/tokens.css`, après quoi `bundle.css` s'applique sans
retouche.

## Le cadre de la PAR

La PAR existe et marche : elle ne fait pas partie de la feuille. La fiche ParFrame la décrit et en
montre un rendu statique fait avec ses formules ; à la migration, seule sa palette de phases passe
aux cinq familles.

## Tenir le système à jour

1. Changer une valeur dans `jetons.json`.
2. `node tools/design/tokens.mjs` : les contrastes et les distances pour les daltoniens ; le
   script échoue si une exigence recule.
3. `node tools/design/ds-export.mjs` : écrit `project/tokens.json` et `project/tokens.css`.
4. Publier sur l'artefact les fichiers changés, avec `root` = `docs-fr/design/systeme` et chaque
   fichier à son chemin `project/…` (y compris celui passé en `file_path`). L'index
   (`project/design-system.json`) se relit en ligne avant d'être republié : la page le réécrit
   (elle a trié ses clés le 13/10).

Une valeur changée seulement sur la page de l'artefact sera écrasée au prochain export : la
changer dans `jetons.json`.

## Ce qui n'y est pas encore

Tout ce que les quatorze écrans finaux emploient y est (vérifié le 13/10 : aucune classe des écrans
hors du système). Restent hors du système : les volets Track et Nearby de la fiche avion, dont le
contenu ne change pas, et les fiches de balise et d'héliport, de même forme que celle d'un
aérodrome.
