#!/usr/bin/env node
// The aircraft family of the interface (docs-fr/design/06-avions.md).
//
//   node tools/design/avions.mjs
//
// Each class is drawn from rounded public dimensions, in metres, seen from
// above, nose up: fuselage, wing, tailplane, engines, propellers, rotor. The
// script scales each drawing into a 64-unit box centred on (32, 32), which is
// the rotation point on the map, and enforces a minimum thickness on small
// parts (propeller, nacelle, tail rotor, glider fuselage) so that they still
// show at 24 px, the size the map draws them. Output: one SVG per class in
// docs-fr/design/avions/famille/, black fill, as the map tints them.
//
// The dimensions are those of a representative type of each class, rounded;
// the drawing is a pictogram of the class, not a portrait of the type.

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const out = path.join(root, 'docs-fr/design/avions/famille');

const BOX = 64;          // viewBox side
const FIT = 58;          // the longest side of the aircraft, in box units
const MIN = 3.0;         // minimum thickness of a part, in box units (≈ 1.1 px at 24 px)
const SOFT = 1.2;        // same-colour stroke that rounds corners, in box units
const deg = (d) => (d * Math.PI) / 180;
const f = (n) => Math.round(n * 1000) / 1000;

// ---------------------------------------------------------------- primitives (metres, x right, y aft, nose at y = 0)

// Fuselage: ogive nose, straight cabin, tapering tail cone.
function fuselage({ L, w, nose, tail, tailW }) {
    const h = w / 2, t = tailW / 2;
    return `M0 0 C${f(h * 0.62)} 0 ${f(h)} ${f(nose * 0.38)} ${f(h)} ${f(nose)} L${f(h)} ${f(L - tail)} C${f(h)} ${f(L - tail * 0.4)} ${f(t + (h - t) * 0.25)} ${f(L)} ${f(t)} ${f(L)} L${f(-t)} ${f(L)} C${f(-t - (h - t) * 0.25)} ${f(L)} ${f(-h)} ${f(L - tail * 0.4)} ${f(-h)} ${f(L - tail)} L${f(-h)} ${f(nose)} C${f(-h)} ${f(nose * 0.38)} ${f(-h * 0.62)} 0 0 0Z`;
}

// A pair of lifting surfaces (wing or tailplane): root at x = ±root, leading edge
// at y = le, swept back by `sweep` degrees, chords cRoot and cTip, semi-span s.
function surfaces({ root, le, cRoot, cTip, s, sweep }) {
    const dy = (s - root) * Math.tan(deg(sweep));
    const side = (k) => `M${f(k * root)} ${f(le)} L${f(k * s)} ${f(le + dy)} L${f(k * s)} ${f(le + dy + cTip)} L${f(k * root)} ${f(le + cRoot)}Z`;
    return side(1) + side(-1);
}
// Leading edge y of a surface at lateral position x.
const leAt = (sf, x) => sf.le + (Math.abs(x) - sf.root) * Math.tan(deg(sf.sweep));

const rect = (cx, cy, w, h, r = 0) => `M${f(cx - w / 2 + r)} ${f(cy - h / 2)} L${f(cx + w / 2 - r)} ${f(cy - h / 2)} Q${f(cx + w / 2)} ${f(cy - h / 2)} ${f(cx + w / 2)} ${f(cy - h / 2 + r)} L${f(cx + w / 2)} ${f(cy + h / 2 - r)} Q${f(cx + w / 2)} ${f(cy + h / 2)} ${f(cx + w / 2 - r)} ${f(cy + h / 2)} L${f(cx - w / 2 + r)} ${f(cy + h / 2)} Q${f(cx - w / 2)} ${f(cy + h / 2)} ${f(cx - w / 2)} ${f(cy + h / 2 - r)} L${f(cx - w / 2)} ${f(cy - h / 2 + r)} Q${f(cx - w / 2)} ${f(cy - h / 2)} ${f(cx - w / 2 + r)} ${f(cy - h / 2)}Z`;

// ---------------------------------------------------------------- the classes

// Each class returns { label, extent: {x0, x1, y0, y1} in metres, draw(m) }
// where m is the minimum thickness in metres once the scale is known.
const classes = {
    leger: {
        label: 'Monomoteur léger', type: 'Robin DR400 · Cessna 172 · Tecnam P2002', mapPx: 20,
        // ~DR400 / C172: span 10 m, length 7.6 m, constant-chord wing, propeller 1.9 m
        L: 7.6, B: 10.0,
        build() {
            const wing = { root: 0.55, le: 2.1, cRoot: 1.55, cTip: 1.25, s: 5.0, sweep: 0 };
            const ht = { root: 0.2, le: 6.5, cRoot: 0.95, cTip: 0.75, s: 1.7, sweep: 5 };
            return (m) => [
                fuselage({ L: 7.6, w: 1.15, nose: 1.3, tail: 3.6, tailW: Math.max(0.3, m) }),
                surfaces(wing), surfaces(ht),
                rect(0, -0.05, 1.9, Math.max(0.22, m), Math.max(0.22, m) / 2), // propeller across the nose
            ];
        },
        extent: { x0: -5.0, x1: 5.0, y0: -0.4, y1: 7.6 },
    },
    turboprop: {
        label: 'Turbopropulseur', type: 'ATR 72 · Dash 8 · King Air', mapPx: 24,
        // ~ATR 72: span 27.1 m, length 27.2 m, straight high wing, propellers 3.9 m
        build() {
            const wing = { root: 1.4, le: 10.8, cRoot: 2.6, cTip: 1.55, s: 13.55, sweep: 2 };
            const ht = { root: 0.3, le: 24.6, cRoot: 2.0, cTip: 1.3, s: 3.65, sweep: 12 };
            return (m) => {
                const parts = [
                    fuselage({ L: 27.2, w: 2.85, nose: 3.4, tail: 8.5, tailW: Math.max(0.6, m) }),
                    surfaces(wing), surfaces(ht),
                ];
                for (const x of [-4.1, 4.1]) {
                    const front = leAt(wing, x) - 2.4;
                    parts.push(rect(x, front + 2.6, Math.max(1.8, m), 5.2, 0.8));           // nacelle, to the trailing edge
                    parts.push(rect(x, front - 0.1, 3.9, Math.max(0.3, m), Math.max(0.3, m) / 2)); // propeller
                }
                return parts;
            };
        },
        extent: { x0: -13.55, x1: 13.55, y0: -0.6, y1: 27.2 },
    },
    affaires: {
        label: "Jet d'affaires", type: 'Falcon 2000 · Gulfstream · Global · Citation', mapPx: 22,
        // ~Falcon 2000: span 19.3 m, length 20.2 m, swept wing, two engines on the rear fuselage, T-tail
        build() {
            const wing = { root: 1.1, le: 8.0, cRoot: 4.0, cTip: 1.35, s: 9.65, sweep: 30 };
            const ht = { root: 0.15, le: 17.6, cRoot: 1.9, cTip: 1.0, s: 3.6, sweep: 32 };
            return (m) => {
                const parts = [
                    fuselage({ L: 20.2, w: 2.4, nose: 3.2, tail: 6.0, tailW: Math.max(0.4, m) }),
                    surfaces(wing), surfaces(ht),
                ];
                for (const x of [-2.05, 2.05]) parts.push(rect(x, 14.4, Math.max(1.25, m), 3.4, 0.55)); // rear engines
                return parts;
            };
        },
        extent: { x0: -9.65, x1: 9.65, y0: 0, y1: 20.2 },
    },
    biréacteur: {
        label: 'Biréacteur moyen-courrier', type: 'A320 · B737 · A220 · E190 · CRJ', mapPx: 26,
        // ~A320: span 35.8 m, length 37.6 m, 25° sweep, engines 5.75 m off the axis
        build() {
            const wing = { root: 1.9, le: 13.6, cRoot: 6.6, cTip: 1.6, s: 17.9, sweep: 28 };
            const ht = { root: 0.4, le: 31.6, cRoot: 3.6, cTip: 1.4, s: 6.2, sweep: 32 };
            return (m) => {
                const parts = [
                    fuselage({ L: 37.6, w: 3.95, nose: 4.6, tail: 9.0, tailW: Math.max(0.7, m) }),
                    surfaces(wing), surfaces(ht),
                ];
                for (const x of [-5.75, 5.75]) {
                    const front = leAt(wing, x) - 2.7;
                    parts.push(rect(x, front + 2.2, Math.max(2.1, m), 4.4, 0.9));
                }
                return parts;
            };
        },
        extent: { x0: -17.9, x1: 17.9, y0: 0, y1: 37.6 },
    },
    gros: {
        label: 'Gros-porteur biréacteur', type: 'A350 · B787 · B777 · A330', mapPx: 30,
        // ~A350-900: span 64.8 m, length 66.8 m, 31° sweep, engines 10.7 m off the axis
        build() {
            const wing = { root: 3.0, le: 23.5, cRoot: 11.5, cTip: 2.4, s: 32.4, sweep: 31 };
            const ht = { root: 0.6, le: 57.8, cRoot: 6.0, cTip: 2.0, s: 9.6, sweep: 33 };
            return (m) => {
                const parts = [
                    fuselage({ L: 66.8, w: 5.96, nose: 7.5, tail: 15.0, tailW: Math.max(1.0, m) }),
                    surfaces(wing), surfaces(ht),
                ];
                for (const x of [-10.7, 10.7]) {
                    const front = leAt(wing, x) - 4.4;
                    parts.push(rect(x, front + 3.6, Math.max(3.4, m), 7.2, 1.5));
                }
                return parts;
            };
        },
        extent: { x0: -32.4, x1: 32.4, y0: 0, y1: 66.8 },
    },
    quadri: {
        label: 'Gros-porteur quadriréacteur', type: 'A380 · B747 · A340', mapPx: 32,
        // ~B747-400: span 64.4 m, length 70.7 m, 37.5° sweep, engines 12 and 21 m off the axis
        build() {
            const wing = { root: 3.25, le: 22.5, cRoot: 13.5, cTip: 3.6, s: 32.2, sweep: 37 };
            const ht = { root: 0.6, le: 61.0, cRoot: 7.0, cTip: 2.4, s: 11.0, sweep: 35 };
            return (m) => {
                const parts = [
                    fuselage({ L: 70.7, w: 6.5, nose: 9.0, tail: 15.0, tailW: Math.max(1.0, m) }),
                    surfaces(wing), surfaces(ht),
                ];
                for (const x of [-21.0, -12.0, 12.0, 21.0]) {
                    const front = leAt(wing, x) - 4.0;
                    parts.push(rect(x, front + 3.6, Math.max(2.8, m), 7.2, 1.2));
                }
                return parts;
            };
        },
        extent: { x0: -32.2, x1: 32.2, y0: 0, y1: 70.7 },
    },
    helico: {
        label: 'Hélicoptère', type: 'H125 · EC135 · H145', mapPx: 22,
        // ~H125: rotor 10.7 m, fuselage 10.9 m, cabin 1.9 m wide
        build() {
            return (m) => {
                const t = Math.max(0.32, m);
                const blade = (a) => {
                    const r = 5.35, c = Math.cos(deg(a)), s = Math.sin(deg(a));
                    const hw = t / 2;
                    const pts = [[-r, -hw], [r, -hw], [r, hw], [-r, hw]].map(([x, y]) => [x * c - y * s, 3.3 + x * s + y * c]);
                    return `M${pts.map(([x, y]) => `${f(x)} ${f(y)}`).join(' L')}Z`;
                };
                const boom = Math.max(0.45, m);
                return [
                    // cabin: a teardrop, the rotor mast over its middle
                    `M0 0.5 C1.05 0.5 1.1 2.0 1.1 3.3 C1.1 4.9 0.7 5.8 0.4 6.2 L-0.4 6.2 C-0.7 5.8 -1.1 4.9 -1.1 3.3 C-1.1 2.0 -1.05 0.5 0 0.5Z`,
                    rect(0, 8.6, boom, 5.4, boom / 2),                                  // tail boom
                    rect(-(boom / 2 + Math.max(0.25, m) / 2), 10.7, Math.max(0.25, m), 1.9, Math.max(0.25, m) / 2), // tail rotor, edge-on, beside the boom
                    blade(45), blade(-45),
                ];
            };
        },
        extent: { x0: -5.35, x1: 5.35, y0: -2.05, y1: 11.5 },
    },
    planeur: {
        label: 'Planeur', type: 'ASK 21 · Duo Discus', mapPx: 22,
        // ~ASK 21: span 17 m, length 8.35 m, slender straight wing, T-tail
        build() {
            const wing = { root: 0.35, le: 2.7, cRoot: 1.35, cTip: 0.5, s: 8.5, sweep: 1 };
            const ht = { root: 0.1, le: 7.65, cRoot: 0.7, cTip: 0.45, s: 1.6, sweep: 4 };
            return (m) => [
                fuselage({ L: 8.35, w: Math.max(0.72, m), nose: 1.4, tail: 5.0, tailW: Math.max(0.2, m * 0.8) }),
                surfaces({ ...wing, cTip: Math.max(wing.cTip, m) }), surfaces({ ...ht, cTip: Math.max(ht.cTip, m * 0.9) }),
            ];
        },
        extent: { x0: -8.5, x1: 8.5, y0: 0, y1: 8.35 },
    },
    inconnu: {
        label: 'Inconnu', type: "ni type ni catégorie : un chevron qui donne le cap, pas un avion", mapPx: 18,
        build() {
            return () => ['M0 0 L4.2 8.2 L0 6.1 L-4.2 8.2Z'];
        },
        extent: { x0: -4.2, x1: 4.2, y0: 0, y1: 8.2 },
    },
};

// ---------------------------------------------------------------- render

function svgFor(key, c, { fill = '#000000', title = true } = {}) {
    const e = c.extent;
    const w = e.x1 - e.x0, h = e.y1 - e.y0;
    const k = FIT / Math.max(w, h);                 // box units per metre
    const m = MIN / k;                              // minimum thickness, in metres
    const cx = (e.x0 + e.x1) / 2, cy = (e.y0 + e.y1) / 2;
    const parts = c.build()(m);
    const tr = `translate(32 32) scale(${f(k)}) translate(${f(-cx)} ${f(-cy)})`;
    return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${BOX} ${BOX}" width="${BOX}" height="${BOX}">` +
        (title ? `<title>${c.label} (${c.type})</title>` : '') +
        `<g transform="${tr}" fill="${fill}" stroke="${fill}" stroke-width="${f(SOFT / k)}" stroke-linejoin="round">` +
        parts.map((d) => `<path d="${d}"/>`).join('') +
        `</g></svg>\n`;
}

fs.mkdirSync(out, { recursive: true });
const index = [];
for (const [key, c] of Object.entries(classes)) {
    const file = `${key.normalize('NFD').replace(/[̀-ͯ]/g, '')}.svg`;
    fs.writeFileSync(path.join(out, file), svgFor(key, c));
    index.push({ file, key, label: c.label, type: c.type, mapPx: c.mapPx });
}
fs.writeFileSync(path.join(out, 'famille.json'), JSON.stringify(index, null, 2) + '\n');
console.log(`${index.length} classes écrites dans ${path.relative(root, out)}/ : ${index.map((i) => i.file).join(', ')}`);
