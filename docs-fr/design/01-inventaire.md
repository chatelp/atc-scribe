# Inventaire visuel de l'interface — 12/10/2026

*Premier livrable de la mission design (`MISSION.md`, §1). Ce qui existe, mesuré, avant toute
proposition. Branche `design`, sources au commit `6976f04` (= `local` du 12/10, 15 h 30). Les mesures dans le navigateur ont été
faites une heure plus tôt sur `bf5c094` ; entre les deux, seules les bulles vocales de la carte et
le correctif de palette (point 5) ont changé.*

> **Mise à jour, même jour (`e701201`).** La session atc-scribe a retiré les 51 sélecteurs orphelins
> de `style.css` et les trois animations qu'eux seuls appelaient (§ 10) : 957 → 482 lignes, 65 → 18
> `!important`, couleurs en dur 319 → 284 (`style.css` 142 → 107), 225 → 215 valeurs distinctes.
> Ce document décrit l'état de départ ; le brut, régénéré, décrit l'état présent. Rien d'autre ne
> change à l'écran.

## Méthode

Deux mesures complémentaires, rejouables :

1. **Statique**, `node tools/design/inventory.mjs` : lit `www/index.html`, `www/style.css` et les
   33 scripts hors `vendor/` et tests ; chaque classe Tailwind est résolue par le CSS compilé
   (`www/vendor/tailwind.css`), qui dit ce qu'elle fait vraiment ; les couleurs, tailles,
   espacements, rayons, ombres en dur sont relevés avec leur fichier et leur ligne. Sortie
   complète : [`01-inventaire-brut.md`](01-inventaire-brut.md) (régénérable).
   Règles de comptage en tête du script : dans `index.html`, seuls `class`, `:class` et
   `x-bind:class` sont lus ; dans les scripts, seules les chaînes littérales.
2. **Rendu**, `tools/design/contrast-probe.js`, exécuté dans la page de l'instance d'essai
   (trafic réel, 107 à 147 avions, deux fréquences) : chaque texte visible **au premier plan**,
   avec sa couleur effective (transparences et opacités des parents composées) sur ce qu'il y a
   derrière, sa taille et sa graisse, **pondéré par le nombre de caractères**. Contraste WCAG 2.1 ;
   seuil AA 4,5:1 (3:1 au-delà de 24 px ou 18,66 px gras).

**Non mesuré** : le texte dessiné dans le canevas de la carte (étiquettes d'avions, invisibles au
DOM : leurs couleurs et tailles viennent de la lecture statique) ; le fil de transcription avec du
contenu (l'instance n'a pas de sidecar vivant, le fil était vide) ; le rendu réel sur iPhone (le
téléphone est une émulation 375 × 812 du navigateur intégré) ; la perception de nuit, qui dépend
de la luminosité de l'écran.

Captures de référence « avant » : `captures/avant-*.jpg` (1 440 × 900 réduites à 800 × 500, et
375 × 812).

## L'essentiel

1. **La moitié de l'écran principal est écrite trop petit.** 45 % des caractères rendus sont
   sous 11 px (7, 8, 9, 9,6 et 10 px) ; à 7 et 8 px : 12 %. **Quinze tailles** coexistent dans le
   code, neuf à l'écran en même temps.
2. **Le gris fait le travail de la hiérarchie, et il le fait mal.** 134 emplois de `text-text/xx`
   (dix opacités du même gris, de /25 à /90) et 46 `opacity-*` sur du texte : c'est l'essentiel
   des 18 % de caractères sous le seuil AA à l'écran principal (35 % sur la Timeline).
3. **225 couleurs distinctes** (107 teintes opaques, 140 variantes transparentes), dont **319
   occurrences en dur** hors Tailwind, dans neuf fichiers. 23 teintes en doublon d'une autre à
   l'œil nu (ΔE < 5). **Deux verts de marque** : `#4CAF50` (thème) et `#4ADE80` (connexion,
   voyants, pilote).
4. **La palette des phases est juste en cohérence, fausse en lisibilité.** Les dix couleurs sont
   les mêmes sur la carte, la liste et la PAR (bon point). Mais NEW et UNK se confondent même en
   vision normale (ΔE 5,6), et pour un deutéranope TAX et CRZ deviennent la même couleur (ΔE 1,8).
5. **Défaut trouvé, corrigé dans l'heure** : avec le fond de carte par défaut (« None », noir), les
   avions étaient peints avec la palette prévue pour un fond **clair** (croisière `#1D4ED8` sur noir :
   3,1:1). Signalé à la session atc-scribe, corrigé par elle (`46dc425`).
6. **Le clavier n'a pas de focus visible** : 0 `focus-visible` hors un curseur, deux
   `focus:outline-none` qui retirent l'anneau, Tab détourné pour passer d'un avion à l'autre.
   0 `role`, 0 `tabindex`, 7 attributs `aria-*`. En face : **591 éléments à bulle d'aide**.
7. **Téléphone** : 35 cibles tactiles sur 41 sous 44 px dans la liste, 17 sous 24 px.
8. **Identité** : le titre dit `Co-ATC`, aucun favicon, aucun manifeste ; la page de connexion a
   sa propre palette (7 couleurs que rien d'autre n'emploie).
9. **Dette CSS** : 51 des 123 sélecteurs de `style.css` ne servaient plus (le rendu des avions en
   DOM, remplacé par WebGL) ; 65 `!important`. **Retirés le jour même** (`e701201`).

Ce qui est **juste** et doit rester : le noir et le vert, la page des réglages (96 % du texte à 12
et 14 px, 3 % sous AA), la cohérence des phases entre les trois vues, une échelle d'espacement
déjà resserrée de fait, un seul rayon dominant, le monospace pour les nombres.

---

## 1. Typographie

**Famille.** Monospace partout : `font-mono` sur `body` (pile `ui-monospace, SFMono-Regular, …`,
soit SF Mono sur Mac), 100 % des caractères rendus. Aucune police chargée. Les nombres sont alignés
de fait par la chasse fixe ; `tabular-nums` n'apparaît que 3 fois.

**Tailles dans le code** (classes + `style.css` + scripts, carte comprise) :

| Taille | Emplois | D'où |
|---:|---:|---|
| 7 px | 15 | chiffres-raccourcis des filtres de phase (10), étiquettes de la barre du bas (LFPZ, ALT) |
| 8 px | 14 | type d'avion sous l'indicatif, LP/S, badges de la fiche, « all », séparateur « New above » |
| 9 px | 30 | unités (ft, kt), badges de phase, filtres de style.css |
| 9,6 px | 1 | compteur « --s » de la tuile de fréquence (`index.html:2056`) |
| 10 px | 62 | onglets Active / Signal Lost, en-têtes, étiquettes de la carte (canevas) |
| 11 px | 22 | chips d'altitude, PAR, étiquettes d'avion (canevas) |
| 11,2 px | 2 | connexion (`.7rem`) |
| **12 px** | 100 | `text-xs` : corps de la liste, fils, réglages |
| 13 px | 2 | style.css |
| 13,6 px | 2 | connexion (`.85rem`) |
| **14 px** | 60 | `text-sm` : indicatifs, valeurs de la liste |
| 15 px | 1 | nom d'aérodrome de la PAR |
| 16 px | 8 | titres de panneau, connexion |
| 18, 20 px | 5 | titres de la page des réglages |
| 36, 60 px | 2 | écran « Connection Lost » |

**Quinze tailles**, dont sept entre 7 et 11 px séparées de 1 px : à cette échelle, 9 et 10 px ne
se distinguent pas, elles ne hiérarchisent rien.

**Rendu, part des caractères** (sonde, premier plan seulement) :

| Écran | < 11 px | 7–8 px | 12 px | 14 px | Tailles à l'écran |
|---|---:|---:|---:|---:|---:|
| Principal 1 440 × 900 (carte + liste + barre radio) | **45 %** | 12 % | 23 % | 24 % | 9 |
| Fiche avion, onglet Details | 21 % | 7 % | 66 % | 10 % | 8 |
| Fiche avion, onglet Timeline | 37 % | 10 % | 34 % | 8 % | 8 |
| Vue PAR (LFPG, 4 avions) | 32 % | 7 % | 24 % | 15 % | 10 |
| Journal d'événements ouvert | 38 % | 10 % | 34 % | 22 % | 9 |
| Page des réglages | 4 % | 3 % | 60 % | 34 % | 8 |
| Téléphone 375, liste | **41 %** | 10 % | 28 % | 18 % | 9 |

La page des réglages, réécrite le 10/10, montre qu'un écran dense de cet outil tient à 12 et 14 px.

**Graisses** : 5 (normal 21, medium 8, semibold 18, bold 17, black 3). **Interlettrage** : 4
valeurs, surtout sur des capitales en 7 et 8 px (`tracking-[0.2em]`), qui compensent la petite
taille par de l'espace sans regagner la lisibilité.

## 2. Couleurs

### Volume

| | |
|---|---:|
| Valeurs distinctes (avec transparences) | 225 |
| Teintes opaques distinctes | 107 |
| Groupes indiscernables à l'œil (ΔE76 < 5) | 84 groupes : 23 teintes sont le doublon d'une autre |
| Classes de couleur Tailwind distinctes | texte 35, fond 60, bordure 51 |
| Occurrences en dur, hors Tailwind | 319 (130 valeurs) |

Les 319 en dur : `style.css` 142 (60 hexadécimales, 86 `rgba()`, quelques-unes dans des
commentaires que le script ignore : la fiche en comptait 54 hexadécimales), `openlayers-map-manager.js` 73, `aircraft-webgl.js` 39, `par-view.js` 26,
`login.js` 19, `minimap-feature.js` 13, `index.html` 5 (objets `:style` d'Alpine), `layer-registry.js` 4,
`audio-client.js` 2. Le thème Tailwind (8 couleurs nommées) n'est repris par aucun de ces fichiers :
`#4CAF50` est écrit en dur 31 fois.

### Par famille (utilisations)

gris 460 · vert 270 · ambre/jaune 67 · bleu 67 · orange 47 · rouge 35 · sarcelle 21 · violet 20 ·
citron 15 · rose 15. **Trois familles de gris** Tailwind (`gray`, `slate`, `neutral`) plus les gris
du thème (`#0a0a0a`, `#1a1a1a`, `#2a2a2a`, `#333`, `#e0e0e0`) et une quinzaine de gris en dur
(`#888`, `#8a8a8a`, `#999`, `#555`, `#444`, `#4a4a4a`, `#666`, `#ccc`, `#b0b0b0`, `#bdbdbd`…).

### Doublons à l'œil nu (ΔE < 5, extrait)

- fond : `#0a0a0a`, `#000`, `#0b0b0b`, `#0b0f0d`, `#141414`, `#0f1512`, `#111` ;
- texte clair : `#e0e0e0`, `#e5e7eb`, `#e8ecef`, `#e5e5e5`, `#e4e4e4` ;
- bordure : `#333`, `#2a2a2a`, `#3a3a3a` ;
- gris moyen : `#888`, `#8a8a8a` ; `#999`, `#a3a3a3` ; `#d1d5db`, `#ccc`, `#cbd5e1` ;
- ambre : `#fbbf24`, `#eab308` ; `#fcd34d`, `#f8d857`.

### Les verts

| | Où | Rôle |
|---|---|---|
| `#4CAF50` (`highlight`) | 92 `text-highlight`, 30 en dur, 23 variantes transparentes (/10 à /95) | marque, actif, sélection, heure |
| `#4ADE80` (`green-400`) | connexion (5), voyant « ok », locuteur PILOT, indice « strong » | succès, pilote |
| `#22C55E` (`green-500`) | fonds /20, bordures /30 | badges |
| `#6EE7B7` (`emerald-300`), `#34D399` | consigne en clair « clairance » | clairance |
| `#86EFAC`, `#76C76C`, `#0b2f0b` | carte (plan d'aérodrome, pistes) | carte |

Le vert dit à la fois « c'est la marque », « c'est actif », « ça va bien », « c'est le pilote » et
« c'est une clairance ». Quand tout est vert, rien ne l'est.

### Palettes porteuses de sens, et le daltonisme

ΔE minimal entre deux couleurs de la palette (sous 10, deux pastilles de quelques pixels se
confondent). Simulation Machado et al. 2009, sévérité 1.

| Palette | Couleurs | Normal | Protanopie | Deutéranopie | Tritanopie |
|---|---|---|---|---|---|
| Phases de vol (10) | `#9CA3AF` NEW, `#C084FC` TAX, `#FB923C` T/O, `#A3E635` CLB, `#4ADE80` DEP, `#60A5FA` CRZ, `#F9A8D4` ARR, `#FACC15` APP, `#2DD4BF` T/D, `#94A3B8` UNK | **5,6** NEW/UNK | **4,6** NEW/UNK | **1,8** TAX/CRZ | **5,3** DEP/T/D |
| Bandes d'altitude, fond sombre | `#F8D857` haut, `#F4B55A` moyen, `#EA8A4A` bas, `#D35A48` très bas, `#E8ECEF` croisière | 21,6 | 15,9 | 10,9 | 17,4 |
| Bandes d'altitude, fond clair | `#0E7490`, `#B45309`, `#C2410C`, `#B91C1C`, `#1D4ED8` | 13,8 | **3,3** moyen/bas | **1,1** moyen/bas | 10,8 |
| Voyants (bandeau d'état) | `#4ADE80` ok, `#FBBF24` warn, `#EF4444` bad, `#6B7280` inconnu | 72 | 34 | 23 | 50 |
| Force des indices | `#FBBF24` faible, `#7DD3FC` moyen, `#4ADE80` fort | 77 | 35 | 45 | 14 |
| Locuteur | `#FB923C` ATC, `#4ADE80` PILOT, `#9CA3AF` inconnu | 74 | 22 | 31 | 46 |
| Consigne en clair | `#6EE7B7` clairance, `#FCD34D` autre | 72 | 53 | 58 | 68 |
| Voyant de la tuile | `#B45555` échec, `#4CAF50` écoute, `#888` coupée | 44 | 17 | 18 | 36 |

**13 paires confondues**, dont 10 dans les phases (liste complète dans le brut). Les voyants
vert / rouge, que la fiche de mission croyait indiscernables, se séparent en fait par la
**luminosité** (vert clair, rouge sombre), pas par la teinte : un deutéranope les distingue comme
« clair » et « foncé », ce qui tient sur une pastille de 6 px mais pas sur un trait fin. Ils ne
portent aucune forme ni aucun mot en plus de la couleur.

**Le défaut du fond noir (corrigé, `46dc425`).** `getAltitudeBandColor` (`aircraft-webgl.js`)
choisissait la palette claire quand le nom du fond ne finissait pas par `dark` ; le fond « None »
(noir, défaut choisi par le propriétaire) s'appelle `none`. Mesuré sur l'instance :
`settings.mapStyle === 'none'`, avions en palette claire. Contraste sur noir : croisière 3,1:1, très
bas 3,2, haut 3,9 — au lieu de 17,7 / 5,3 / 15,0 avec la palette sombre. Depuis le correctif, les
fonds clairs sont nommés (`ign`, `osm`) et tout le reste tombe dans la palette sombre. La palette
claire garde son défaut propre : moyen / bas confondus pour un deutéranope (ΔE 1,1).

## 3. Contrastes rendus

| Écran | Caractères | Sous AA | Pires cas (ratio, taille, texte) |
|---|---:|---:|---|
| Principal 1 440 | 707 | **18 %** | 3,2 · 8 px « Antenna » ; 3,2 · 7 px « LFPZ » ; 3,8 · 9 px « SFC » ; 4,2 · 12 px « QNH 1018 » ; 2,0 · 14 px « --- » ; 2,5–3,4 · 7 px chiffres des filtres |
| Fiche, Details | 766 | 17 % | 4,2 · 12 px tirets des valeurs absentes (×60) ; 4,2 · 10 px « 0 transmission(s) » |
| Fiche, Timeline | 453 | **35 %** | 3,2 · 10 px texte explicatif (×84) ; 4,2 · 11 px heures |
| Vue PAR | 1 298 | 13 % | 3,2 · 10 px « 6100 ft ↓ → FL070 » ; 2,5 · 10 px « 3° » ; 2,6 · 11 px avion estompé |
| Journal ouvert | 855 | 30 % | 4,2 · 12 px message vide (×148) |
| Réglages | 1 359 | 3 % | les mêmes « Antenna », « LFPZ » de la barre du bas |
| Téléphone, liste | 427 | 18 % | comme l'écran principal |

Les échecs ont une cause unique : **le texte secondaire est obtenu en baissant l'opacité du texte
principal** (`text-text/40` = `#696969` sur `#1a1a1a`, 3,2:1 ; `/50` = 4,2:1 ; `/25` = 2,0:1), et
il est en même temps le plus petit. Les couleurs pleines, elles, passent toutes : sur `#1a1a1a`,
les 24 couleurs de texte les plus employées sont entre 6,3 et 17,4:1, sauf `gray-500` (3,6) et
`highlight/70` (3,7).

## 4. Espacements, rayons, bordures, ombres, plans

**Espacements** (padding, margin, gap, space) : 14 valeurs Tailwind, mais **4 couvrent 77 %**
des emplois : 2 px (78), 4 px (94), 6 px (122), 8 px (156). 40 % des emplois sont des demi-pas
(`1.5`, `2.5`, `0.5`), signe que l'échelle de 4 px est trop grosse pour cette densité et qu'on
l'a affinée au cas par cas. `style.css` en ajoute 25 valeurs en dur (`1px 3px`, `2px 4px`,
`10px 10px 8px 10px`, `76px 16px 24px`…).

**Rayons** : 4 px domine (77 emplois, écrit deux façons : `rounded` 67 et `rounded-[4px]` 10) ;
puis 6 px (17), plein (15), 8 px (8), 2 px (4) ; `style.css` ajoute 1, 2, 3 et 6 px.

**Bordures** : 1 px partout (195), 2 px pour les accents (9). Couleur dominante `border-border`
`#333` (104), puis 9 variantes transparentes du même gris et 23 bordures colorées.

**Ombres** : 7 valeurs Tailwind (dont 4 arbitraires) et 21 en dur, dont des `drop-shadow`
d'éléments qui n'existent plus. Sur fond noir une ombre portée ne se voit pas ; ce qui sépare un
popover de la carte, c'est sa bordure et son fond.

**Plans (z-index)** : 10, 20, 40, 50, 70, 1 000, 1 001, 1 200, 1 500, 2 000, 9 000, 10 000,
10 001, 20 000, 99 999 (connexion) : quinze étages sans échelle.

## 5. États

| État | Comment il est rendu aujourd'hui | Emplois |
|---|---|---:|
| Survol | `hover:` (fond `bg-hover`, texte plein, vert /15) + 14 règles `:hover` | 81 |
| Actif / sélectionné | classes conditionnelles `:class` : `bg-highlight/15–20 text-highlight` (onglets, filtres, Radar/PAR), `filter-btn-active` (ombre intérieure), bordure verte | ~40 |
| Désactivé | `disabled:opacity-40` (4), `/30` (1), `cursor-not-allowed` (1) | 6 |
| Focus clavier | `focus:outline-none` (2, **retire** l'anneau), `.filter-input:focus` (bordure verte /30), un `focus-visible` sur le curseur d'altitude | 5 |
| Erreur | rouge sur la tuile en échec (`border-red-400/30 bg-red-950/30 text-red-300/60 opacity-80` : l'erreur est **estompée**), compteur `!` | — |
| Chargement | `animate-spin` (3), `animate-pulse` (1, « Connection Lost ») | 4 |

Trois façons de dire « actif » (fond vert transparent, ombre intérieure, bordure verte), aucune de
dire « focus ». Une erreur rendue à 60 % d'opacité est moins visible qu'un état normal.

## 6. Icônes

**Font Awesome 6** (solid 60, regular 9) : **54 icônes distinctes, 90 emplois** ; les plus
fréquentes : `arrows-alt-h` 8, `times` 7, `volume-high` 4, `plane-arrival` 4. Noms des versions 5
et 6 mélangés pour la même icône : `times` (7) et `xmark` (1), `exclamation-triangle` (2) et
`triangle-exclamation` (1), `broadcast-tower` (1) et `tower-broadcast` (2), `cog`, `info-circle`.
Glyphes Unicode servant de signe : → (4), ↔ ↑ ↓ (canevas de la carte), ✕ (1). Huit SVG
d'avions dans `www/assets/` (a0 à a7). Aucun signe propre à l'outil.

## 7. Mouvement

Transitions : 150 ms (69 emplois), 200 ms (13), 300 et 500 ms (2). Animations : `animate-spin`,
`animate-pulse` (« Connection Lost » clignote en 36 px rouge), 4 `@keyframes` dans `style.css`
(`subtle-pulse`, `proximityPulse`, `takeoffLandingGlow`, `takeoffLandingLabelPulse`), dont les
trois dernières n'étaient plus appelées (retirées depuis ; `tx-flash` est arrivée avec les
bulles vocales de la carte). Les avions bougent par `requestAnimationFrame` (moteur
d'animation, 30 i/s). **`prefers-reduced-motion` : absent.**

## 8. Accessibilité

| | |
|---|---:|
| Bulles d'aide `data-help` (rendues, écran principal) | **591** (77 statiques et 37 liées dans `index.html`, le reste par `x-for`) |
| `aria-*` | 7 |
| `role=` · `tabindex` · `alt=` | 0 · 0 · 0 |
| `title=` | 29 |
| `@media` dans `style.css` | 0 (le téléphone passe par 18 classes `md:`) |
| `prefers-reduced-motion`, `prefers-color-scheme` | absents |
| Cibles visibles à 1 440, écran principal | 55, dont 22 sous 24 px (curseurs de volume : 10 px de haut) |
| Cibles visibles au téléphone, liste | 41, dont **17 sous 24 px** et **35 sous 44 px** |
| Clavier | raccourcis 0-9, A, G, Espace ; **Tab détourné** (`app.js:791`) pour l'avion suivant |

Les aides contextuelles sont abondantes et bien faites ; l'accessibilité au sens strict (lecteur
d'écran, clavier, mouvement réduit) n'a pas été un objectif, et le système devra en fixer le
minimum sans alourdir.

## 9. Identité

- `<title>` : « Co-ATC » ; « co-atc » apparaît 3 fois dans `index.html`, 12 fois dans les scripts.
- Ni favicon, ni manifeste d'application, ni `theme-color` (le téléphone l'ouvre comme un site).
- La **page de connexion** (`login.js`, CSS en ligne) a sa palette : fond `#0b0f0d` (noir verdi),
  cadre `#0f1512`, bordure `#1f2d26`, texte `#d6e4dc`, secondaire `#6b7d74`, accent `#4ADE80` (le
  second vert), bouton à texte `#04160c`. Aucune n'est employée ailleurs. Taille du titre 16 px,
  le reste entre 11,2 et 13,6 px.
- L'icône de la barre de menus (`tools/menubar/AtcScribe.swift:262`) est un symbole SF :
  `dot.radiowaves.left.and.right` en marche, `antenna.radiowaves.left.and.right.slash` à l'arrêt.
- Aucun nom ni signe visible dans l'interface elle-même : le seul texte de marque à l'écran est
  « Tracked Targets ».

## 10. Dette CSS

- **51 sélecteurs orphelins sur 123** dans `style.css` (42 %) : étiquettes, traînées, pistes,
  survols et bandes d'altitude de l'ancien rendu des avions en DOM, remplacé par WebGL ; aucune
  occurrence dans le HTML ni les scripts, y compris construite par concaténation (vérifié par
  `grep` sur les racines). **Retirés le 12/10 par la session atc-scribe (`e701201`)**, avec les
  trois `@keyframes` qu'ils appelaient : il reste 70 sélecteurs, tous employés.
- **65 `!important`** dans `style.css` au départ, **18** après ce retrait ; ceux qui restent tiennent
  à l'ordre des feuilles (Tailwind compilé passe après `style.css`, voulu le 11/10 pour reproduire
  le CDN).
- `:root { --highlight-color: #4CAF50; }` : la seule variable CSS de couleur, que rien n'utilise.

## Ce qui est incohérent, ce qui est juste

**Incohérent**
- quinze tailles de texte, sept entre 7 et 11 px ;
- la hiérarchie par opacité (dix nuances de `text-text/xx`) au lieu de niveaux de gris nommés ;
- 225 couleurs, trois familles de gris, deux verts de marque, 319 valeurs en dur hors thème ;
- le vert surchargé de sens (marque, actif, succès, pilote, clairance) ;
- NEW / UNK identiques ; TAX / CRZ identiques pour un deutéranope ;
- la palette claire des avions sur le fond noir par défaut (corrigé le jour même) ;
- trois rendus de « actif », aucun de « focus », une erreur estompée ;
- quinze plans z-index sans échelle ; ombres invisibles sur fond noir ;
- icônes FA5 et FA6 mélangées ; aucun signe propre ;
- une page de connexion qui n'appartient pas au reste.

**Juste, à garder**
- le noir et le vert, lus de nuit, code de l'outil professionnel ;
- le monospace pour tout ce qui est nombre, indicatif, fréquence, heure (alignement sans effort) ;
- les mêmes couleurs de phase sur la carte, la liste et la PAR ;
- la page des réglages : 12–14 px, 3 % sous AA, une colonne ;
- de fait, une échelle d'espacement serrée (2, 4, 6, 8 px) et un rayon de 4 px ;
- une bordure de 1 px `#333` comme séparateur principal, qui marche sur le noir ;
- les aides contextuelles partout ;
- les couleurs pleines, toutes au-dessus de 6:1 sur le fond des panneaux.

## Rejouer

```bash
node tools/design/inventory.mjs > docs-fr/design/01-inventaire-brut.md
```

Pour le rendu : ouvrir l'instance d'essai, coller `tools/design/contrast-probe.js` dans la
console (ou l'exécuter par l'outil de navigateur), à 1 440 × 900 puis 375 × 812, sur chaque écran.
