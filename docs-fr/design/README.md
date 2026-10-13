# Design — identité et système de l'outil

Dossier de la mission design (`MISSION.md`), ouvert le 12/10/2026, branche `design`. Rien ici
n'est appliqué à `www/` : la migration se fait après validation du propriétaire, avec la session
atc-scribe (`04-migration.md`).

| Fichier | Contenu | État |
|---|---|---|
| `MISSION.md` | la fiche de mission | — |
| `00-nom.md` | la recherche du nom en trois tours et la place du français (D74, Q53) | **Clairance**, décidé le 13/10 |
| `01-inventaire.md` | l'interface du 12/10, mesurée : tailles, couleurs, contrastes, états, icônes | fait |
| `01-inventaire-brut.md` | la sortie complète de `tools/design/inventory.mjs`, régénérable | fait |
| `02-principes.md` | dix principes, chacun justifié par un usage et une mesure | fait ; principe 8 **à trancher** |
| `03-systeme.md` | le système : couleurs, typographie, espace, composants, mouvement, densité, accessibilité | fait ; cinq choix **à trancher** (fin du document) |
| `jetons.json` | les 91 jetons, source unique des valeurs | fait |
| `03-systeme-verif.md` | chaque contraste et chaque distance pour les daltoniens, produit par `tools/design/tokens.mjs` | régénérable |
| `maquettes/` | écrans en HTML statique sur les jetons (`index.html` pour commencer) | fait |
| `captures/` | l'interface « avant », 12/10 | référence |
| `04-migration.md` | le plan par étapes, vérifiables, avec l'effort (≈ 13 j) | fait |
| `05-identite.md`, `identite/` | la marque Clairance : signe, nom, favicon, barre de menus, connexion, README (`identite/index.html` pour la planche) | fait, nom décidé le 13/10 |
| `06-avions.md`, `avions/` | l'inventaire des avions dessinés dans l'outil, et notre famille de neuf silhouettes (`tools/design/avions.mjs`) | fait |
| `07-esquisses.md` | le fil au centre : A et sa colonne à deux onglets, B replié, suivre un vol ; d'où vient chaque donnée | esquisses ; « suivre un vol » **à valider** |

Outils, dans `tools/design/` :

```bash
node tools/design/inventory.mjs > docs-fr/design/01-inventaire-brut.md   # l'inventaire
node tools/design/tokens.mjs                                              # vérifier les jetons, écrire tokens.css
```

`contrast-probe.js` se colle dans la console d'une page (l'instance d'essai ou une maquette)
pour mesurer le texte rendu ; `color.mjs` porte l'arithmétique des couleurs commune aux deux
scripts.

Pour voir les maquettes avec leurs feuilles de style, servir ce dossier :

```bash
python3 -m http.server 8791 --bind 127.0.0.1 --directory docs-fr/design
```

puis ouvrir http://127.0.0.1:8791/maquettes/ (et arrêter le serveur ensuite).
