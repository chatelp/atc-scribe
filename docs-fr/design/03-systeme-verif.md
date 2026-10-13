# Vérification des jetons — générée par tools/design/tokens.mjs

Ne pas modifier à la main : régénérer après tout changement de `jetons.json`. ✗ = exigence non tenue (le script sort alors en erreur).

## Texte sur les surfaces (WCAG 2.1, AA = 4,5:1)

Chaque jeton de texte, sur chaque surface où il peut apparaître (case vide : combinaison non prévue).

| Texte | page | panel | raised | hover | selected | urgent | warn | map |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `text.primary` #e0e0e0 | 15.0 | 13.2 | 12.1 | 10.9 | 10.8 | 13.3 | 11.6 |  |
| `text.secondary` #b8b8b8 | 10.0 | 8.8 | 8.0 | 7.2 | 7.2 | 8.8 | 7.7 |  |
| `text.tertiary` #949494 | 6.5 | 5.7 | 5.2 | 4.7 | 4.7 | 5.8 | 5.0 |  |
| `accent.default` #4caf50 | 7.1 | 6.3 | 5.7 | 5.2 | 5.1 |  |  |  |
| `accent.strong` #66c56a | 9.2 | 8.1 | 7.4 | 6.7 | 6.6 |  |  |  |
| `status.urgent` #ff6e6e | 7.3 | 6.4 | 5.8 | 5.3 |  | 6.4 |  |  |
| `status.warn` #f5b83d | 11.1 | 9.8 | 8.9 | 8.1 |  |  | 8.6 |  |
| `status.info` #6cb6ff | 9.2 | 8.1 | 7.4 | 6.7 |  |  |  |  |
| `phase.ground` #dccb9e | 12.3 | 10.8 | 9.9 | 8.9 | 8.9 |  |  | 13.1 |
| `phase.departure` #ec8b34 | 7.8 | 6.9 | 6.3 | 5.7 | 5.6 |  |  | 8.3 |
| `phase.cruise` #b8d3f2 | 12.9 | 11.3 | 10.3 | 9.3 | 9.2 |  |  | 13.6 |
| `phase.arrival` #4c9ce8 | 6.8 | 6.0 | 5.5 | 4.9 | 4.9 |  |  | 7.2 |
| `phase.unknown` #969696 | 6.7 | 5.9 | 5.4 | 4.9 | 4.8 |  |  | 7.1 |

Désactivé (`text.disabled` #6b6b6b) : panel 3.3, hover 2.7 — exempté par WCAG, voulu sous le seuil.

## Texte sur les aplats

- `text.on-accent` sur `accent.fill` : 6.8
- `text.on-urgent` sur `status.urgent-fill` : 6.5
- `text.on-warn` sur `status.warn` : 11.1
- `text.on-warn` sur `phase.ground` (badge plein de T/O et T/D) : 12.3
- `text.on-warn` sur `phase.departure` (badge plein de T/O et T/D) : 7.8
- `text.on-warn` sur `phase.cruise` (badge plein de T/O et T/D) : 12.9
- `text.on-warn` sur `phase.arrival` (badge plein de T/O et T/D) : 6.8
- `text.on-warn` sur `phase.unknown` (badge plein de T/O et T/D) : 6.7

## Contrastes hors texte (WCAG 1.4.11, 3:1)

- `border.control` #787878 : panel 3.9, raised 3.6, hover 3.3
- `border.focus` #4caf50 : page 7.1, panel 6.3, raised 5.7, hover 5.2, selected 5.1, map 7.6
- `status.ok` #4caf50 : panel 6.3, raised 5.7, map 7.6
- `status.warn` #f5b83d : panel 9.8, raised 8.9, map 11.8
- `status.urgent` #ff6e6e : panel 6.4, raised 5.8, map 7.7
- `status.unknown` #949494 : panel 5.7, raised 5.2
- `status.urgent-fill` #b3262f : panel 2.7 — non exigé : l'aplat est toujours bordé de `status.urgent`, qui porte le contraste.
- `border.default` et `border.subtle` ne sont pas soumis au 3:1 : ce sont des séparateurs, pas des contours de contrôle ; 1.4 et 1.2 sur panel.

## Rampes d'altitude (icône d'avion sur la carte)

- **fond sombre** (sur #000000) : very-low #fff59a (18.8) · low #e68cba (8.8) · mid #c2a3ff (10.0) · high #61c9e3 (11.0) · cruise #6e7b8f (4.9)
  - toutes les paires : ΔE 34.1 en vision normale, 13.6 au pire daltonisme ; au rouge d'urgence 29.0 ; à une famille de phase 5.8 ; le plus bas le plus visible, la croisière la plus discrète : oui
- **fond clair** (sur #ffffff) : very-low #33311f (13.1) · low #452a38 (12.8) · mid #746299 (5.3) · high #27505b (8.8) · cruise #6e7b8f (4.3)
  - toutes les paires : ΔE 22.4 en vision normale, 12.7 au pire daltonisme ; au rouge d'urgence 29.1 ; à une famille de phase 14.5 ; le plus bas le plus visible, la croisière la plus discrète : oui

## Palettes porteuses de sens et daltonisme

ΔE76 dans l'espace Lab, après simulation Machado et al. 2009 (sévérité 1). Repère : sous 10, deux pastilles de quelques pixels se confondent.

### Familles de phase

Inventaire du 12/10 : 1,8 (TAX / CRZ en deutéranopie) avec les dix couleurs actuelles.

| Paire | normale | protanopia | deuteranopia | tritanopia | minimum |
|---|---:|---:|---:|---:|---:|
| ground / departure | 49.3 | 37.5 | 38.0 | 47.2 | 37.5 |
| ground / cruise | 42.9 | 42.1 | 44.0 | 30.0 | 30.0 |
| ground / arrival | 72.8 | 68.7 | 77.5 | 52.6 | 52.6 |
| ground / unknown | 31.7 | 31.1 | 32.5 | 23.8 | 23.8 |
| departure / cruise | 85.9 | 77.6 | 80.9 | 76.4 | 76.4 |
| departure / arrival | 109.5 | 99.2 | 110.4 | 94.2 | 94.2 |
| departure / unknown | 67.0 | 56.8 | 61.5 | 57.7 | 56.8 |
| cruise / arrival | 34.4 | 31.5 | 37.6 | 27.5 | 27.5 |
| cruise / unknown | 28.2 | 28.6 | 28.1 | 27.5 | 27.5 |
| arrival / unknown | 45.5 | 42.5 | 49.2 | 36.7 | 36.7 |

Minimum : **23.8** (exigence ≥ 20).

### Sémantique : voix de l'outil, urgence, attention, information

L'urgence se compare à l'accent par son aplat, qui est sa forme d'apparition ; le texte d'urgence (`status.urgent`) n'apparaît jamais sans son pictogramme.

| Paire | normale | protanopia | deuteranopia | tritanopia | minimum |
|---|---:|---:|---:|---:|---:|
| accent / urgence (aplat) | 107.2 | 45.1 | 18.4 | 110.8 | 18.4 |
| accent / attention | 67.2 | 23.6 | 35.6 | 71.0 | 23.6 |
| accent / information | 95.2 | 87.1 | 82.9 | 18.4 | 18.4 |
| urgence (aplat) / attention | 69.2 | 67.1 | 47.3 | 53.1 | 47.3 |
| urgence (aplat) / information | 99.3 | 71.6 | 86.8 | 120.5 | 71.6 |
| attention / information | 111.3 | 108.3 | 113.8 | 76.8 | 76.8 |

Minimum : **18.4** (exigence ≥ 15).

### Voyants (avec leurs formes : ● ▲ ■ ○)

La couleur n'est pas seule : chaque voyant a sa forme. Le tableau dit ce que la couleur ajoute ; aucune exigence chiffrée.

| Paire | normale | protanopia | deuteranopia | tritanopia | minimum |
|---|---:|---:|---:|---:|---:|
| ok / attention | 67.2 | 23.6 | 35.6 | 71.0 | 23.6 |
| ok / panne | 104.5 | 32.5 | 6.6 | 104.1 | 6.6 |
| ok / inconnu | 62.8 | 48.2 | 36.1 | 35.5 | 35.5 |
| attention / panne | 60.8 | 55.9 | 35.3 | 38.0 | 35.3 |
| attention / inconnu | 70.3 | 71.0 | 69.5 | 39.4 | 39.4 |
| panne / inconnu | 62.0 | 17.1 | 34.4 | 69.8 | 17.1 |

Minimum : **6.6** (exigence ≥ 0).

