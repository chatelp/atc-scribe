# Inventaire brut — généré par tools/design/inventory.mjs

Sources lues : index.html (2446 l.), style.css (482 l.), 33 scripts hors vendor et tests ; CSS compilé : 514 classes.
Utilisations de classes Tailwind reconnues : 3879 (3271 statiques dans class="", 608 conditionnelles : :class ou chaînes des scripts) ; classes distinctes : 507.

## Tailles de texte

| Taille | Utilisations | Classes ou origine |
|---|---:|---|
| 7px | 15 | text-[7px] 15 |
| 8px | 14 | text-[8px] 14 |
| 9px | 26 | text-[9px] 25, style.css:416 |
| 9.6px | 1 | index.html:2063 |
| 10px | 60 | text-[10px] 54, style.css:240, style.css:362, map/core/layer-registry.js:110, map/openlayers-map-manager.js:563, map/openlayers-map-manager.js:950, map/renderers/aircraft-webgl.js:620 |
| 11px | 23 | text-[11px] 13, style.css:227, style.css:234, style.css:330, style.css:370, style.css:372, style.css:422, style.css:446, map/openlayers-map-manager.js:76, map/renderers/aircraft-webgl.js:591, map/renderers/aircraft-webgl.js:605 |
| 11.2px | 2 | login.js:36, login.js:48 |
| 12px | 100 | text-xs 94, text-[12px] 1, style.css:291, style.css:320, style.css:351, login.js:35, login.js:61 |
| 12.8px | 1 | login.js:42 |
| 13px | 2 | style.css:220, style.css:341 |
| 13.6px | 2 | login.js:53, login.js:57 |
| 14px | 61 | text-sm 61 |
| 15px | 1 | style.css:329 |
| 16px | 8 | text-base 7, login.js:34 |
| 18px | 2 | text-lg 2 |
| 20px | 3 | text-xl 3 |
| 36px | 1 | text-4xl 1 |
| 60px | 1 | text-6xl 1 |

Graisses : font-normal 21, font-semibold 18, font-bold 17, font-medium 8, font-black 3.
Interlettrage : tracking-wider 31, tracking-wide 7, tracking-[0.2em] 5, tracking-normal 1.
Familles déclarées dans style.css et les scripts : monospace | inherit ; body : bg-background text-text m-0 p-0 font-mono overflow-hidden h-screen flex flex-col.

## Couleurs

Valeurs distinctes : 215 (dont 131 avec transparence) ; teintes opaques distinctes : 105.
Classes de couleur Tailwind distinctes : texte 35, fond 60, bordure 51, anneau 0, SVG 0.
Valeurs en dur (hors Tailwind) : 284 occurrences, 117 valeurs distinctes ; par fichier : style.css 107, map/openlayers-map-manager.js 73, map/renderers/aircraft-webgl.js 39, par/par-view.js 26, login.js 19, map/features/minimap-feature.js 13, index.html 5, map/core/layer-registry.js 4, audio-client.js 2.

### Par famille de teinte

| Famille | Teintes distinctes | Utilisations |
|---|---:|---:|
| grey | 42 | 444 |
| green | 8 | 267 |
| amber/yellow | 10 | 67 |
| blue | 14 | 66 |
| orange | 11 | 42 |
| red | 8 | 35 |
| teal/cyan | 4 | 21 |
| violet | 4 | 19 |
| lime | 2 | 15 |
| pink | 2 | 15 |

### Regroupement des teintes voisines (ΔE76 < 5 : indiscernables à l'œil côte à côte)

105 teintes opaques → 83 groupes. Groupes de plus d'une valeur :

- #333333, #2a2a2a, #3a3a3a
- #e0e0e0, #e5e7eb, #e8ecef, #e4e4e4
- #0a0a0a, #000000, #0b0b0b, #0b0f0d, #141414, #0f1512, #111111
- #ffffff, #f9fafb, #f3f4f6, #f1f5f9
- #888888, #8a8a8a
- #fbbf24, #eab308
- #fcd34d, #f8d857
- #444444, #4a4a4a
- #d1d5db, #cccccc, #cbd5e1
- #999999, #a3a3a3
- #b0b0b0, #bdbdbd

### Les 60 couleurs les plus utilisées

| Couleur | Famille | Utilisations | Rôles (classe : n) | Fichiers |
|---|---|---:|---|---|
| `#4caf50` | green | 130 | text (text-highlight) 92 ; en dur 26 ; border (border-highlight) 7 ; bg (bg-highlight) 5 | index.html 102, style.css 13, map/openlayers-map-manager.js 5, map/renderers/aircraft-webgl.js 3 |
| `#333333` | grey | 106 | border (border-border) 104 ; en dur 2 | index.html 104, style.css 2 |
| `#9ca3af` | grey | 53 | text (text-gray-400) 31 ; en dur 19 ; bg (bg-gray-400) 2 ; border (border-gray-400) 1 | index.html 32, map/renderers/aircraft-webgl.js 8, map/openlayers-map-manager.js 7, style.css 3 |
| `#e0e0e0` | grey | 43 | text (text-text) 38 ; en dur 5 | index.html 31, store/aircraft-details.js 5, style.css 4, store/sidebar.js 2 |
| `#0a0a0a` | grey | 29 | bg (bg-background) 27 ; en dur 2 | index.html 27, style.css 1, par/par-view.js 1 |
| `#4ade80` | green | 25 | text (text-green-400) 12 ; en dur 7 ; bg (bg-green-400) 4 ; border (border-green-400) 2 | index.html 16, login.js 5, store/aircraft-details.js 1, store/sidebar.js 1 |
| `#60a5fa` | blue | 17 | en dur 10 ; text (text-blue-400) 7 | map/openlayers-map-manager.js 8, index.html 6, store/sidebar.js 1, map/renderers/aircraft-webgl.js 1 |
| `#1a1a1a` | grey | 17 | bg (bg-panel) 17 | index.html 17 |
| `#2a2a2a` | grey | 16 | bg (bg-hover) 16 | index.html 16 |
| `#f87171` | red | 15 | text (text-red-400) 12 ; border (border-red-400) 2 ; en dur 1 | index.html 10, events.js 1, store/aircraft-details.js 1, store/sidebar.js 1 |
| `#ffffff` | grey | 15 | en dur 11 ; text (text-white) 4 | index.html 4, map/openlayers-map-manager.js 4, par/par-view.js 4, map/renderers/aircraft-webgl.js 2 |
| `#fb923c` | orange | 14 | text (text-orange-400) 7 ; en dur 5 ; border (border-orange-400) 2 | index.html 8, map/openlayers-map-manager.js 3, store/sidebar.js 1, map/renderers/aircraft-webgl.js 1 |
| `#000000@0.5` | grey | 13 | bg (bg-black/50) 7 ; en dur 6 | index.html 7, style.css 5, map/core/layer-registry.js 1 |
| `#4caf50@0.15` | green | 11 | bg (bg-highlight/15) 9 ; en dur 2 | index.html 9, style.css 2 |
| `#4caf50@0.3` | green | 11 | en dur 6 ; border (border-highlight/30) 3 ; bg (bg-highlight/30) 2 | style.css 6, index.html 4, store/aircraft-details.js 1 |
| `#facc15` | amber/yellow | 10 | text (text-yellow-400) 8 ; en dur 2 | index.html 7, store/sidebar.js 1, map/renderers/aircraft-webgl.js 1, par/par-view.js 1 |
| `#f44336` | red | 10 | text (text-danger) 5 ; en dur 3 ; bg (bg-danger) 2 | index.html 5, map/features/minimap-feature.js 2, store/aircraft-details.js 1, store/sidebar.js 1 |
| `#4caf50@0.5` | green | 10 | en dur 5 ; border (border-highlight/50) 4 ; text (text-highlight/50) 1 | index.html 5, style.css 5 |
| `#888888` | grey | 10 | en dur 10 | map/openlayers-map-manager.js 6, index.html 2, audio-client.js 1, map/features/minimap-feature.js 1 |
| `#fbbf24` | amber/yellow | 9 | bg (bg-amber-400) 5 ; text (text-amber-400) 3 ; en dur 1 | index.html 8, style.css 1 |
| `#4caf50@0.2` | green | 9 | bg (bg-highlight/20) 9 | index.html 9 |
| `#2dd4bf` | teal/cyan | 8 | text (text-teal-400) 5 ; en dur 3 | index.html 4, store/sidebar.js 1, map/openlayers-map-manager.js 1, map/renderers/aircraft-webgl.js 1 |
| `#6b7280` | grey | 8 | text (text-gray-500) 6 ; bg (bg-gray-500) 1 ; border (border-gray-500) 1 | index.html 8 |
| `#4caf50@0.6` | green | 8 | border (border-highlight/60) 8 | index.html 8 |
| `#c084fc` | violet | 7 | text (text-purple-400) 5 ; en dur 2 | index.html 4, store/sidebar.js 1, map/renderers/aircraft-webgl.js 1, par/par-view.js 1 |
| `#a3e635` | lime | 7 | text (text-lime-400) 5 ; en dur 2 | index.html 4, store/sidebar.js 1, map/renderers/aircraft-webgl.js 1, par/par-view.js 1 |
| `#f9a8d4` | pink | 7 | text (text-pink-300) 5 ; en dur 2 | index.html 4, store/sidebar.js 1, map/renderers/aircraft-webgl.js 1, par/par-view.js 1 |
| `#94a3b8` | blue | 7 | text (text-slate-400) 5 ; en dur 2 | index.html 4, store/sidebar.js 1, map/renderers/aircraft-webgl.js 1, par/par-view.js 1 |
| `#fcd34d` | amber/yellow | 7 | text (text-amber-300) 7 | index.html 5, events.js 1, store/transcripts.js 1 |
| `#ffc107` | amber/yellow | 7 | en dur 5 ; text (text-warning) 1 ; bg (bg-warning) 1 | index.html 3, map/openlayers-map-manager.js 2, map/features/minimap-feature.js 1, par/par-view.js 1 |
| `#f59e0b` | orange | 7 | en dur 5 ; border (border-amber-500) 1 ; border (border-t-amber-500) 1 | map/openlayers-map-manager.js 5, index.html 2 |
| `#000000@0.2` | grey | 7 | en dur 7 | style.css 7 |
| `#7dd3fc` | blue | 6 | text (text-sky-300) 3 ; bg (bg-sky-300) 3 | index.html 5, events.js 1 |
| `#4caf50@0.7` | green | 6 | text (text-highlight/70) 6 | store/aircraft-details.js 6 |
| `#22c55e@0.2` | green | 6 | bg (bg-green-500/20) 6 | index.html 6 |
| `#000000@0.3` | grey | 6 | bg (bg-black/30) 6 | index.html 6 |
| `#4caf50@0.8` | green | 5 | text (text-highlight/80) 4 ; bg (bg-highlight/80) 1 | index.html 5 |
| `#6ee7b7` | green | 5 | text (text-emerald-300) 5 | index.html 4, events.js 1 |
| `#eab308@0.2` | amber/yellow | 5 | bg (bg-yellow-500/20) 5 | index.html 5 |
| `#2a2a2a@0.5` | grey | 5 | bg (bg-hover/50) 5 | index.html 5 |
| `#4caf50@0.1` | green | 5 | bg (bg-highlight/10) 5 | index.html 5 |
| `#22c55e@0.3` | green | 5 | border (border-green-500/30) 5 | index.html 5 |
| `#eab308@0.3` | amber/yellow | 5 | border (border-yellow-500/30) 5 | index.html 5 |
| `#000000` | grey | 4 | en dur 2 ; text (text-black) 1 ; bg (bg-black) 1 | index.html 2, map/openlayers-map-manager.js 1, map/renderers/aircraft-webgl.js 1 |
| `#fbbf24@0.15` | amber/yellow | 4 | bg (bg-amber-400/15) 4 | index.html 4 |
| `#3b82f6@0.2` | blue | 4 | bg (bg-blue-500/20) 4 | index.html 4 |
| `#333333@0.4` | grey | 4 | border (border-border/40) 4 | index.html 4 |
| `#4caf50@0.4` | green | 4 | border (border-highlight/40) 4 | index.html 4 |
| `#333333@0.7` | grey | 4 | border (border-border/70) 4 | index.html 4 |
| `#3b82f6@0.3` | blue | 4 | border (border-blue-500/30) 4 | index.html 4 |
| `#333333@0.5` | grey | 4 | border (border-border/50) 4 | index.html 4 |
| `#000000@0.4` | grey | 4 | en dur 4 | style.css 4 |
| `#444444` | grey | 4 | en dur 4 | style.css 4 |
| `#818cf8` | blue | 4 | en dur 4 | map/openlayers-map-manager.js 4 |
| `#d1d5db` | grey | 3 | en dur 2 ; text (text-gray-300) 1 | style.css 2, index.html 1 |
| `#000000@0.45` | grey | 3 | en dur 2 ; bg (bg-black/45) 1 | index.html 1, style.css 1, map/openlayers-map-manager.js 1 |
| `#f472b6@0.15` | pink | 3 | bg (bg-pink-400/15) 3 | index.html 3 |
| `#a855f7@0.2` | violet | 3 | bg (bg-purple-500/20) 3 | index.html 3 |
| `#f97316@0.2` | orange | 3 | bg (bg-orange-500/20) 3 | index.html 3 |
| `#84cc16@0.2` | lime | 3 | bg (bg-lime-500/20) 3 | index.html 3 |

### Couleurs en dur, détail par fichier

**www/style.css** (107)

`#4caf50` ×13 (l. 118, 329, 330, 330, 362, 386…) · `#000000@0.2` ×7 (l. 52, 134, 150, 156, 162, 172…) · `#4caf50@0.3` ×6 (l. 43, 139, 150, 156, 162, 177) · `#000000@0.5` ×5 (l. 4, 72, 276, 372, 446) · `#4caf50@0.5` ×5 (l. 145, 182, 349, 362, 415) · `#000000@0.4` ×4 (l. 37, 37, 43, 48) · `#e0e0e0` ×4 (l. 320, 351, 372, 446) · `#444444` ×4 (l. 434, 436, 438, 439) · `#ffffff@0.05` ×3 (l. 15, 32, 428) · `#9ca3af` ×3 (l. 227, 247, 291) · `#4caf50@0.55` ×3 (l. 446, 462, 462) · `#ffffff@0.03` ×2 (l. 11, 57) · `#4caf50@0.18` ×2 (l. 20, 24) · `#d1d5db` ×2 (l. 234, 255) · `#4caf50@0.15` ×2 (l. 338, 362) · `#8a8a8a` ×2 (l. 339, 341) · `#333333` ×2 (l. 342, 406) · `#0b0b0b` ×2 (l. 413, 414) · `#999999` ×2 (l. 440, 441) · `#0a0a0a@0.88` ×2 (l. 446, 462) · `#000000@0.45` ×1 (l. 4) · `#000000@0.7` ×1 (l. 4) · `#c0ccbf` ×1 (l. 4) · `#ffffff@0.1` ×1 (l. 15) · `#b0b0b0` ×1 (l. 28) · `#000000@0.35` ×1 (l. 37) · `#ffffff@0.04` ×1 (l. 52) · `#4caf50@0.06` ×1 (l. 72) · `#4caf50@0.25` ×1 (l. 72) · `#f9fafb` ×1 (l. 220) · `#4b5563@0.4` ×1 (l. 247) · `#374151@0.3` ×1 (l. 260) · `#93c5fd` ×1 (l. 264) · `#34d399` ×1 (l. 270) · `#111827@0.95` ×1 (l. 276) · `#e5e7eb` ×1 (l. 276) · `#4b5563@0.6` ×1 (l. 276) · `#f3f4f6` ×1 (l. 303) · `#0a0a0a` ×1 (l. 312) · `#555555` ×1 (l. 340) · `#141414` ×1 (l. 342) · `#4caf50@0.35` ×1 (l. 372) · `#000000@0.9` ×1 (l. 372) · `#ffffff@0.12` ×1 (l. 402) · `#4caf50@0.75` ×1 (l. 403) · `#38bdf8@0.8` ×1 (l. 408) · `#ffffff@0.4` ×1 (l. 416) · `#cccccc` ×1 (l. 422) · `#ffffff@0.45` ×1 (l. 425) · `#4caf50@0.12` ×1 (l. 429) · `#fbbf24` ×1 (l. 430) · `#ffffff` ×1 (l. 475)

**www/index.html** (5)

`#888888` ×2 (l. 1600, 2058) · `#ffc107` ×1 (l. 1604) · `#b45555` ×1 (l. 2058) · `#4caf50` ×1 (l. 2058)

**www/audio-client.js** (2)

`#4caf50` ×1 (l. 360) · `#888888` ×1 (l. 360)

**www/login.js** (19)

`#4ade80` ×5 (l. 21, 21, 21, 21, 21) · `#d6e4dc` ×3 (l. 21, 21, 21) · `#1f2d26` ×3 (l. 21, 21, 21) · `#6b7d74` ×3 (l. 21, 21, 21) · `#0b0f0d` ×2 (l. 21, 21) · `#0f1512` ×1 (l. 21) · `#04160c` ×1 (l. 21) · `#f87171` ×1 (l. 21)

**www/map/core/layer-registry.js** (4)

`#4299e1@0.2` ×1 (l. 89) · `#4299e1@0.9` ×1 (l. 90) · `#e5e7eb` ×1 (l. 95) · `#000000@0.5` ×1 (l. 113)

**www/map/features/minimap-feature.js** (13)

`#f44336` ×2 (l. 147, 148) · `#4caf50` ×2 (l. 147, 148) · `#888888` ×1 (l. 161) · `#888888@0.3` ×1 (l. 172) · `#888888@0.5` ×1 (l. 173) · `#00bcd4` ×1 (l. 191) · `#00bcd4@0.2` ×1 (l. 202) · `#00bcd4@0.4` ×1 (l. 203) · `#ffc107` ×1 (l. 216) · `#ffc107@0.6` ×1 (l. 227) · `#ffc107@0.8` ×1 (l. 228)

**www/map/openlayers-map-manager.js** (73)

`#60a5fa` ×8 (l. 442, 443, 651, 709, 710, 837…) · `#9ca3af` ×7 (l. 450, 560, 717, 776, 785, 792…) · `#888888` ×6 (l. 931, 933, 954, 1471, 1472, 1474) · `#f59e0b` ×5 (l. 445, 446, 712, 713, 1656) · `#4caf50` ×5 (l. 834, 838, 908, 1001, 1656) · `#ffffff` ×4 (l. 28, 604, 615, 1706) · `#818cf8` ×4 (l. 444, 448, 711, 715) · `#a78bfa` ×3 (l. 447, 653, 714) · `#fb923c` ×3 (l. 698, 699, 701) · `#666666` ×3 (l. 1533, 1534, 1536) · `#ffc107` ×2 (l. 836, 1005) · `#e4e4e4` ×1 (l. 27) · `#efe3a0` ×1 (l. 29) · `#bebebe@0.16` ×1 (l. 30) · `#aab6d6@0.3` ×1 (l. 31) · `#000000` ×1 (l. 78) · `#efe3a0@0.25` ×1 (l. 98) · `#000000@0.45` ×1 (l. 566) · `#76c76c` ×1 (l. 633) · `#34d399` ×1 (l. 652) · `#2dd4bf` ×1 (l. 654) · `#737373@0.5` ×1 (l. 818) · `#737373@0.8` ×1 (l. 827) · `#f44336` ×1 (l. 1003) · `#fde047` ×1 (l. 1011) · `#86efac` ×1 (l. 1013) · `#eab308` ×1 (l. 1023) · `#0b2f0b` ×1 (l. 1023) · `#00bcd4` ×1 (l. 1488) · `#4caf50@0.12` ×1 (l. 1663) · `dynamic` ×1 (l. 1680) · `#4caf50@0.95` ×1 (l. 1705) · `#eb8c00` ×1 (l. 1739) · `#eb8c00@0.08` ×1 (l. 1743)

**www/map/renderers/aircraft-webgl.js** (39)

`#9ca3af` ×8 (l. 381, 392, 399, 483, 534, 545…) · `dynamic` ×3 (l. 335, 346, 348) · `#4caf50` ×3 (l. 479, 496, 644) · `#e8ecef` ×2 (l. 325, 542) · `#ffffff` ×2 (l. 496, 545) · `#111827` ×2 (l. 533, 536) · `#0e7490` ×1 (l. 308) · `#b45309` ×1 (l. 309) · `#c2410c` ×1 (l. 310) · `#b91c1c` ×1 (l. 311) · `#1d4ed8` ×1 (l. 314) · `#f8d857` ×1 (l. 319) · `#f4b55a` ×1 (l. 320) · `#ea8a4a` ×1 (l. 321) · `#d35a48` ×1 (l. 322) · `#c084fc` ×1 (l. 382) · `#fb923c` ×1 (l. 383) · `#a3e635` ×1 (l. 384) · `#4ade80` ×1 (l. 385) · `#60a5fa` ×1 (l. 386) · `#f9a8d4` ×1 (l. 387) · `#facc15` ×1 (l. 388) · `#2dd4bf` ×1 (l. 389) · `#94a3b8` ×1 (l. 390) · `#000000` ×1 (l. 537)

**www/par/par-view.js** (26)

`#ffffff` ×4 (l. 679, 681, 762, 766) · `#555555` ×2 (l. 601, 646) · `#9ca3af` ×1 (l. 21) · `#c084fc` ×1 (l. 21) · `#fb923c` ×1 (l. 21) · `#a3e635` ×1 (l. 21) · `#4ade80` ×1 (l. 22) · `#60a5fa` ×1 (l. 22) · `#f9a8d4` ×1 (l. 22) · `#facc15` ×1 (l. 22) · `#2dd4bf` ×1 (l. 23) · `#94a3b8` ×1 (l. 23) · `#262626` ×1 (l. 26) · `#3a3a3a` ×1 (l. 26) · `#8a8a8a` ×1 (l. 26) · `#e0e0e0` ×1 (l. 26) · `#4caf50` ×1 (l. 27) · `#ffc107` ×1 (l. 27) · `#bdbdbd` ×1 (l. 27) · `#4a4a4a` ×1 (l. 27) · `#111111` ×1 (l. 636) · `#0a0a0a` ×1 (l. 724)

## Contrastes WCAG (texte sur fond)

Contraste de chaque couleur de texte Tailwind (les 24 plus fréquentes) sur les fonds principaux. AA = 4,5 pour le texte courant, 3 pour le texte ≥ 18,66 px gras ou 24 px ; en dessous de 3, illisible de nuit à distance.

| Texte | Classes | n | sur #0a0a0a (background (thème)) | sur #1a1a1a (panel (thème)) | sur #000000 (carte (fond noir)) | sur #2a2a2a (bg-hover) |
|---|---|---:|---:|---:|---:|---:|
| `#4caf50` | text-highlight | 92 | 7.1 | 6.3 | 7.6 | 5.2 |
| `#e0e0e0` | text-text | 38 | 15.0 | 13.2 | 15.9 | 10.9 |
| `#9ca3af` | text-gray-400 | 31 | 7.8 | 6.9 | 8.3 | 5.7 |
| `#f87171` | text-red-400 | 12 | 7.2 | 6.3 | 7.6 | 5.2 |
| `#4ade80` | text-green-400 | 12 | 11.4 | 10.0 | 12.1 | 8.2 |
| `#facc15` | text-yellow-400 | 8 | 12.9 | 11.4 | 13.7 | 9.4 |
| `#fb923c` | text-orange-400 | 7 | 8.7 | 7.7 | 9.3 | 6.3 |
| `#60a5fa` | text-blue-400 | 7 | 7.8 | 6.8 | 8.3 | 5.6 |
| `#fcd34d` | text-amber-300 | 7 | 13.7 | 12.1 | 14.6 | 10.0 |
| `#6b7280` | text-gray-500 | 6 | 4.1 ~ | 3.6 ~ | 4.3 ~ | 3.0 ✗ |
| `#4caf50@0.7` | text-highlight/70 | 6 | 3.9 ~ | 3.7 ~ | 4.0 ~ | 3.3 ~ |
| `#f44336` | text-danger | 5 | 5.4 | 4.7 | 5.7 | 3.9 ~ |
| `#c084fc` | text-purple-400 | 5 | 7.5 | 6.6 | 7.9 | 5.4 |
| `#a3e635` | text-lime-400 | 5 | 13.1 | 11.5 | 13.9 | 9.5 |
| `#f9a8d4` | text-pink-300 | 5 | 10.9 | 9.6 | 11.6 | 7.9 |
| `#2dd4bf` | text-teal-400 | 5 | 10.6 | 9.3 | 11.3 | 7.7 |
| `#94a3b8` | text-slate-400 | 5 | 7.7 | 6.8 | 8.2 | 5.6 |
| `#6ee7b7` | text-emerald-300 | 5 | 13.0 | 11.4 | 13.8 | 9.4 |
| `#ffffff` | text-white | 4 | 19.8 | 17.4 | 21.0 | 14.4 |
| `#4caf50@0.8` | text-highlight/80 | 4 | 4.9 | 4.4 ~ | 5.0 | 3.8 ~ |
| `#fbbf24` | text-amber-400 | 3 | 11.9 | 10.4 | 12.6 | 8.6 |
| `#7dd3fc` | text-sky-300 | 3 | 11.9 | 10.4 | 12.6 | 8.6 |
| `#ca8a04@0.8` | text-yellow-600/80 | 2 | 4.6 | 4.3 ~ | 4.8 | 3.7 ~ |
| `#d1d5db` | text-gray-300 | 1 | 13.4 | 11.8 | 14.3 | 9.7 |

✗ sous 3:1 ; ~ entre 3 et 4,5:1. Ces paires sont possibles, pas forcément rendues : les paires réelles, pondérées par le nombre de caractères, viennent de la sonde (contrast-probe.js) dans le navigateur.

## Palettes porteuses de sens, et le daltonisme

ΔE76 minimal entre deux couleurs de la palette : sous 10, deux pastilles de quelques pixels se confondent ; sous 5, côte à côte. Simulation de Machado et al. (2009), sévérité 1 ; protanopie et deutéranopie touchent environ 8 % des hommes, la tritanopie est rare.

| Palette | Couleurs | Contraste min. sur le fond | ΔE min. normal | protanopie | deutéranopie | tritanopie |
|---|---|---:|---|---|---|---|
| Phases de vol (par-view.js PHASE_COLORS = étiquettes de la carte = classes de la liste) | NEW `#9ca3af`, TAX `#c084fc`, T/O `#fb923c`, CLB `#a3e635`, DEP `#4ade80`, CRZ `#60a5fa`, ARR `#f9a8d4`, APP `#facc15`, T/D `#2dd4bf`, UNK `#94a3b8` | 7.9 | 5.6 (NEW/UNK) ✗ | 4.6 (NEW/UNK) ✗ | 1.8 (TAX/CRZ) ✗ | 5.3 (DEP/T/D) ✗ |
| Bandes d'altitude, fond sombre (aircraft-webgl.js getAltitudeBandColor) | high `#f8d857`, mid `#f4b55a`, low `#ea8a4a`, very-low `#d35a48`, cruise/default `#e8ecef` | 5.3 | 21.6 (mid/low) | 15.9 (mid/low) | 10.9 (mid/low) | 17.4 (high/mid) |
| Bandes d'altitude, fond clair (idem) | high `#0e7490`, mid `#b45309`, low `#c2410c`, very-low `#b91c1c`, cruise/default `#1d4ed8` | 5.0 | 13.8 (mid/low) | 3.3 (mid/low) ✗ | 1.1 (mid/low) ✗ | 10.8 (low/very-low) |
| Voyants d'état (index.html, bandeau : ok / warn / bad / unknown) | ok `#4ade80`, warn `#fbbf24`, bad `#ef4444`, unknown `#6b7280` | 3.6 | 72.1 (warn/bad) | 33.6 (bad/unknown) | 22.6 (ok/bad) | 49.8 (warn/unknown) |
| Force des indices d'appariement (weak / medium / strong) | weak `#fbbf24`, medium `#7dd3fc`, strong `#4ade80` | 10.0 | 77.0 (medium/strong) | 35.4 (weak/strong) | 45.4 (weak/strong) | 13.5 (medium/strong) |
| Locuteur (ATC / PILOT / inconnu) | ATC `#fb923c`, PILOT `#4ade80`, unknown `#9ca3af` | 6.9 | 73.8 (PILOT/unknown) | 21.5 (ATC/PILOT) | 31.4 (ATC/PILOT) | 46.0 (PILOT/unknown) |
| Consigne en clair (clairance / autre) | clearance `#6ee7b7`, other `#fcd34d` | 11.4 | 72.1 (clearance/other) | 53.2 (clearance/other) | 58.2 (clearance/other) | 68.0 (clearance/other) |
| Tuile de fréquence, voyant de niveau (failed / on / muted) | failed `#b45555`, on `#4caf50`, muted `#888888` | 3.6 | 43.6 (failed/muted) | 17.4 (failed/muted) | 17.5 (failed/on) | 36.1 (on/muted) |

Paires confondues (ΔE < 10) : 13.

- Phases de vol : NEW/UNK en protanopia (ΔE 4.6)
- Phases de vol : TAX/CRZ en protanopia (ΔE 9.9)
- Phases de vol : CLB/APP en protanopia (ΔE 5.6)
- Phases de vol : ARR/UNK en protanopia (ΔE 8.9)
- Phases de vol : NEW/T/D en deuteranopia (ΔE 8.0)
- Phases de vol : NEW/UNK en deuteranopia (ΔE 6.3)
- Phases de vol : TAX/CRZ en deuteranopia (ΔE 1.8)
- Phases de vol : ARR/T/D en deuteranopia (ΔE 5.3)
- Phases de vol : NEW/UNK en tritanopia (ΔE 5.3)
- Phases de vol : DEP/T/D en tritanopia (ΔE 5.3)
- Bandes d'altitude, fond clair : mid/low en protanopia (ΔE 3.3)
- Bandes d'altitude, fond clair : mid/low en deuteranopia (ΔE 1.1)
- Bandes d'altitude, fond clair : mid/very-low en deuteranopia (ΔE 9.9)

## Espacements (padding, margin, gap, space)

| Valeur | n | Classes |
|---|---:|---|
| `.5rem` | 156 | gap-2 46, px-2 44, mb-2 21, py-2 14, p-2 11, ml-2 5, my-2 4, mt-2 2 |
| `.375rem` | 122 | p-1.5 56, gap-1.5 23, py-1.5 18, px-1.5 9, my-1.5 6, pb-1.5 5, space-y-1.5 2, mb-1.5 2 |
| `.25rem` | 90 | py-1 25, gap-1 21, px-1 8, mt-1 6, mr-1 6, ml-1 6, space-y-1 5, pb-1 5 |
| `.125rem` | 78 | py-0.5 30, mt-0.5 20, px-0.5 11, mb-0.5 8, gap-0.5 4, p-0.5 2, pb-0.5 2, gap-y-0.5 1 |
| `1rem` | 44 | mb-4 15, p-4 11, gap-4 9, px-4 2, pr-4 2, gap-x-4 2, py-4 1, ml-4 1 |
| `.625rem` | 36 | gap-2.5 21, px-2.5 10, space-x-2.5 2, pl-2.5 2, mb-2.5 1 |
| `.75rem` | 32 | px-3 11, p-3 6, gap-3 5, space-y-3 3, mb-3 3, mt-3 2, gap-x-3 1, pr-3 1 |
| `-.125rem` | 10 | -mt-0.5 10 |
| `0` | 6 | m-0 2, mt-0 2, p-0 1, pr-0 1 |
| `2rem` | 3 | gap-8 1, p-8 1, pr-8 1 |
| `1px` | 3 | gap-px 3 |
| `1.25rem` | 2 | py-5 1, pl-5 1 |
| `-.25rem` | 2 | -mt-1 2 |
| `auto` | 1 | mx-auto 1 |

Valeurs d'espacement distinctes : 14. En dur dans style.css : 24 valeurs distinctes sur 35 déclarations (2px ×5, 6px ×3, 2px 4px ×2, 10px ×2, 0 7px ×2, 0 ×2, 1px 16px 1px 0 ×2, 4px ×1, 10px 10px 8px 10px ×1, 76px 16px 24px ×1, 4px 8px ×1, 0 5px ×1, 24px ×1, 6px 0 2px ×1, 8px ×1).

## Rayons

| Valeur | n | Classes |
|---|---:|---|
| `.25rem` | 67 | rounded 66, rounded-r 1 |
| `.375rem` | 17 | rounded-md 17 |
| `9999px` | 15 | rounded-full 15 |
| `4px` | 10 | rounded-[4px] 10 |
| `.5rem` | 8 | rounded-lg 8 |
| `.125rem` | 4 | rounded-sm 3, rounded-r-sm 1 |

En dur : 0 2px 2px 0 (.audio-visualizer-bar) ; 3px (::-webkit-scrollbar-track) ; 3px (::-webkit-scrollbar-thumb) ; 2px (.custom-scrollbar::-webkit-scrollbar-track) ; 2px (.custom-scrollbar::-webkit-scrollbar-thumb) ; 6px (.ol-ref-popup) ; 3px (.par-vac) ; 6px (.par-frame) ; 3px (.par-badge) ; 4px (.par-tip) ; 1px 1px 0 0 (.alt-bin) ; 2px (.alt-track, .alt-track-band) ; 50% (.alt-range::-webkit-slider-thumb) ; 50% (.alt-range::-moz-range-thumb) ; 2px (.radio-strip::-webkit-scrollbar-thumb) ; 1px (.vol-range::-webkit-slider-runnable-track) ; 1px (.vol-range::-moz-range-track) ; 50% (.vol-range::-webkit-slider-thumb) ; 50% (.vol-range::-moz-range-thumb) ; 6px (.voice-bubble).

## Ombres

| Valeur | n | Classes |
|---|---:|---|
| `0 25px 50px -12px rgba(0,0,0,.25)` | 5 | shadow-2xl 5 |
| `0 2px 8px rgba(0,0,0,.35)` | 2 | shadow-[0_2px_8px_rgba(0,0,0,0.35)] 2 |
| `0 5px 15px rgba(0,0,0,.5)` | 1 | shadow-[0_5px_15px_rgba(0,0,0,0.5)] 1 |
| `0 8px 20px rgba(0,0,0,.45)` | 1 | shadow-[0_8px_20px_rgba(0,0,0,0.45)] 1 |
| `0 1px 3px 0 rgba(0,0,0,.1),0 1px 2px -1px rgba(0,0,0,.1)` | 1 | shadow 1 |
| `0 1px 2px 0 rgba(0,0,0,.05)` | 1 | shadow-sm 1 |
| `0 -5px 10px rgba(0,0,0,.3)` | 1 | shadow-[0_-5px_10px_rgba(0,0,0,0.3)] 1 |

En dur : inset 0 2px 6px rgba(0, 0, 0, 0.7) (.flight-strip) ; inset 0 1px 3px rgba(0, 0, 0, 0.4) (.filter-input) ; inset 0 1px 3px rgba(0, 0, 0, 0.4) !important (.filter-input:focus) ; inset 0 1px 3px rgba(0, 0, 0, 0.4) (.filter-btn-active) ; inset 0 1px 3px rgba(0, 0, 0, 0.5) !important (.flight-card-map-hover) ; 0 0 8px theme('colors.highlight') (.audio-visualizer-bar) ; 0 2px 8px rgba(0, 0, 0, 0.2) (#radio-bar .rounded-lg:hover) ; 0 4px 12px rgba(0, 0, 0, 0.5) (.ol-ref-popup) ; 0 4px 12px rgba(0, 0, 0, 0.5) (.par-tip) ; 0 0 0 1px #4CAF50 (.alt-range::-webkit-slider-thumb) ; 0 0 0 3px rgba(76, 175, 80, 0.5) (.alt-range:focus-visible::-webkit-slider-thumb) ; 0 2px 6px rgba(0, 0, 0, 0.5) (.voice-bubble).

## Épaisseurs de bordure

| Valeur | n | Classes |
|---|---:|---|
| `1px` | 195 | border 113, border-b 63, border-t 9, border-l 8, border-r 2 |
| `2px` | 9 | border-t-2 4, border-b-2 3, border-l-2 2 |

## Opacités

| Valeur | n | Classes |
|---|---:|---|
| `.6` | 22 | opacity-60 22 |
| `.5` | 12 | opacity-50 12 |
| `1` | 4 | opacity-100 4 |
| `.7` | 4 | opacity-70 4 |
| `.8` | 4 | opacity-80 4 |
| `.4` | 4 | opacity-40 4 |
| `.3` | 1 | opacity-30 1 |
| `0` | 1 | opacity-0 1 |
| `.35` | 1 | opacity-35 1 |

## Plans (z-index)

| Valeur | n | Classes |
|---|---:|---|
| `50` | 4 | z-50 4 |
| `2000` | 3 | z-[2000] 3 |
| `10` | 2 | z-10 2 |
| `10001` | 2 | z-[10001] 2 |
| `70` | 1 | z-[70] 1 |
| `9000` | 1 | z-[9000] 1 |
| `40` | 1 | z-40 1 |
| `20` | 1 | z-20 1 |
| `1000` | 1 | z-[1000] 1 |
| `1001` | 1 | z-[1001] 1 |

En dur : 1200 (.ol-ref-popup) ; 9000 (#par-view) ; 20000 (.par-tip) ; 10000 (x).

## Mouvement (transitions, animations)

| Valeur | n | Classes |
|---|---:|---|
| `.15s` | 69 | transition-all 25, transition-colors 21, duration-150 13, transition 8, transition-transform 2 |
| `.2s` | 13 | duration-200 13 |
| `spin 1s linear infinite` | 3 | animate-spin 3 |
| `.5s` | 1 | duration-500 1 |
| `pulse 2s cubic-bezier(.4,0,.6,1) infinite` | 1 | animate-pulse 1 |
| `0s` | 1 | duration-0 1 |
| `.3s` | 1 | duration-300 1 |

En dur : 5 déclarations ; @keyframes : tx-flash ; prefers-reduced-motion : non.

## États

Variantes Tailwind (préfixes) :

| Variante | n | Classes les plus fréquentes |
|---|---:|---|
| hover: | 67 | bg-hover 16, text-text 16, bg-highlight/15 5, text-highlight 5, bg-hover/50 5, text-highlight/80 4, opacity-100 3, border-highlight/30 3 |
| md: | 18 | block 4, flex 3, hidden 2, flex-1 2, min-w-0 2, w-120 1, min-w-120 1, border-l 1 |
| disabled: | 6 | opacity-40 4, opacity-30 1, cursor-not-allowed 1 |
| focus: | 2 | outline-none 2 |
| first: | 1 | mt-0 1 |

Sélecteurs d'état dans style.css : hover 13, focus 2, checked 2, active 1, focus-visible 1, first-child 1, last-child 1.

## Icônes

Font Awesome : 54 icônes distinctes, 90 occurrences ; styles : fas 60, far 9.

arrows-alt-h 8 · times 7 · volume-high 4 · plane-arrival 4 · volume-xmark 3 · road 3 · arrow-up 3 · arrow-down 3 · plane 3 · play 2 · chevron-down 2 · route 2 · exclamation-triangle 2 · tower-broadcast 2 · rotate-left 2 · plane-departure 2 · rotate 1 · xmark 1 · right-from-bracket 1 · pause 1 · cog 1 · bell 1 · trash-can 1 · chevron-up 1 · wifi-slash 1 · arrows-up-down 1 · arrow-left 1 · timeline 1 · info-circle 1 · broadcast-tower 1 · map 1 · list 1 · comments 1 · search 1 · cloud-sun 1 · cloud 1 · triangle-exclamation 1 · star 1 · plug-circle-exclamation 1 · chevron-right 1 · chevron 1 · spinner 1 · stop 1 · taxi 1 · plane-up 1 · plane-down 1 · eye 1 · chart-line 1 · clipboard-check 1 · headset 1 · user 1 · comment 1 · right-left 1 · signal 1

Glyphes Unicode servant d'icône ou de signe (hors commentaires) : → 4 (par/par-view.js 2, index.html 1, store/transcripts.js 1) · ↔ 2 (map/renderers/aircraft-webgl.js 2) · ↑ 2 (map/renderers/aircraft-webgl.js 1, par/par-view.js 1) · ↓ 2 (map/renderers/aircraft-webgl.js 1, par/par-view.js 1) · ✕ 1 (map/openlayers-map-manager.js 1).

SVG dans www/assets : a0.svg, a1.svg, a2.svg, a3.svg, a4.svg, a5.svg, a6.svg, a7.svg.

## Accessibilité (compteurs)

- data-help (index.html, static) : 22
- :data-help (index.html, bound) : 36
- data-help (scripts) : 16
- aria-* attributes : 7
- role= attributes : 0
- title= attributes : 29
- tabindex : 0
- alt= : 0
- <button> : 57
- <input> : 43
- <select> : 5
- focus/focus-visible variants (Tailwind) : 2
- :focus rules (style.css) : 3
- @media in style.css : 0
- prefers-reduced-motion : no
- @media de style.css : aucune

## style.css : règles orphelines

Sélecteurs de classe ou d'identifiant définis : 70 ; sans aucune occurrence dans index.html ni les scripts : 0.


`!important` dans style.css : 18 ; classes Tailwind préfixées par ! : 0.

## Identité de la page

- <title> : « Co-ATC »
- favicon (<link rel="icon">) : aucun ; manifeste d'application : aucun ; theme-color : aucun
- occurrences de « Co-ATC » / « co-atc » dans index.html : 3 ; dans les scripts : 12
- tabular-nums (chiffres à chasse fixe hors monospace) : 3

## Classes absentes du CSS compilé (rendues sans style)

Aucune.

