# Fiche B — Retirer côté interface ce qui n'a pas de rôle ici

*Palier 1 de l'audit (doc 32, ligne 1.1), sous D71.3. Branche : `cloud/b-retraits-interface`.
Lis d'abord `README.md` de ce dossier : ses règles s'appliquent.*

## Le but

Exécuter les **étapes 0 à 5** du plan de découpage de `docs-fr/audit-2026-10-09/frontend-code.md`
(section 5) : poser un filet, puis retirer environ 2 355 lignes de `www/` qui ne servent pas
ici, **sans rien déplacer** (déplacer change l'ordre d'initialisation ; retirer, non).
L'interface doit rester identique pour tout ce qui sert.

## Étapes, dans cet ordre, un commit chacune

0. **Le filet**, avant tout retrait : un script Node sans dépendance (`tools/www-check/check.mjs`,
   par exemple) qui vérifie que chaque `$store.atc.X` lu dans `index.html` (attributs Alpine :
   `x-text`, `x-show`, `:class`, `@click`, etc.) et chaque `store.X` / `Alpine.store('atc').X`
   des autres fichiers de `www/` **existe** dans le store défini par `app.js` ; que chaque
   fonction appelée depuis `index.html` existe ; et qu'aucun `<script src>` d'`index.html` ne
   pointe vers un fichier absent. Il sort en erreur sinon. Il doit passer sur `main` tel quel
   (l'audit : 169 noms, 0 non résolu). La façon de le lancer s'écrit en tête du script et
   dans ton rapport. Ce script tourne ensuite **à chaque commit**, avec `node --check` et
   `node --test www/par/`.
1. **Le chat IA « AI Advisory »** : `www/atc-chat.js`, son `<script>`, la tuile
   d'`index.html` (~2256-2371), sa règle CSS. Aucun nom du store n'en dépend (l'audit : 0
   référence depuis `app.js`) : vérifie-le.
2. **La simulation** : champs et fonctions du store (`app.js` ~465-474, ~1416-1558), les trois
   blocs d'`index.html` (volet ~493-541, fiche ~1593-1657, modale ~2552-2649), les lignes de
   `map/openlayers-map-manager.js` qui s'y rapportent.
3. **Les fonctions mortes** : les 21 fonctions jamais appelées et les 3 champs jamais lus de
   l'audit (section 3), moins celles déjà parties aux étapes 1 et 2. Corrige au passage le
   défaut latent signalé : `cleanup()` lit `window.animationEngine`, jamais assigné.
4. **Les fonds et couches américains** : VFR et IFR de la FAA, NEXRAD, NOAA (index.html,
   `app.js`, `map/map-engine.js`, `map/openlayers-map-manager.js`). **Le fond par défaut
   devient le fond sombre Carto déjà présent** (choix du propriétaire : le sombre) ; ne touche
   pas aux autres fonds. **Attention aux préférences enregistrées** : un navigateur dont la clé
   `localStorage` du fond vaut un fond retiré doit retomber sur le sombre, pas sur une carte
   vide ; ne renomme pas la clé.
5. **Les clones** (section 4 de l'audit) : les `set*Opacity`, les `toggle*` de couches, les
   trois `toggle*Details` METAR/TAF/NOTAM, `cycleToNext/PreviousAircraft`, fusionnés en
   fonctions paramétrées. **Garde les anciens noms** comme enveloppes d'une ligne s'ils sont lus
   par `index.html` ou les autres JS (le filet te le dira), ou mets à jour `index.html` en même
   temps, dans le même commit.

**Ne touche pas** : la météo (METAR, TAF, NOTAM : active), la vue PAR (`www/par/`), le
diagnostic de performance (volet Debug : il servira à mesurer la latence, palier 4.3), le volet
Station, l'accueil, les réglages, les alertes (paliers 1.5, 2.1, 2.2 : ils seront réécrits sur
le Mac), `login.js`, `audio-client.js`. Les alertes et les réglages, même s'ils contiennent du
mort, attendent leur refonte.

## Vérifications

À chaque commit : le filet, `node --check` sur chaque JS touché, `node --test www/par/`. Si tu
peux, sers `www/` en statique et charge `index.html` dans un navigateur sans tête pour
vérifier qu'aucune erreur JavaScript n'apparaît au chargement (le serveur absent donnera des
erreurs réseau, attendues ; une `ReferenceError` ou `TypeError` ne l'est pas). Mesure les
lignes retirées par étape.

## Rapport

`docs-fr/fiches-cloud/B-rapport.md` (voir README). En plus : la liste des gestes à refaire
sur le Mac dans un vrai navigateur, avec les vrais flux, pour valider (écoute, sélection
d'un avion, PAR, panneau Serveur, changement de fond, réglages des couches restantes).
