#!/usr/bin/env node
// Writes the design system's tokens (docs-fr/design/systeme/project/tokens.json) from
// the one source of values, docs-fr/design/jetons.json.
//
//   node tools/design/ds-export.mjs
//
// The design system artifact reads tokens as lists ({name, value, usage}), not
// as the W3C tree of jetons.json. Names are the CSS custom properties of
// maquettes/tokens.css without their "--", so the artifact's generated
// tokens.css declares the same variables as the mock-ups and, later, www/.
// Edit jetons.json, run tokens.mjs (checks) then this script; never edit the
// output by hand.
//
//   node tools/design/ds-export.mjs --www [dir]     also copy the system into www/
//   node tools/design/ds-export.mjs --check [dir]   only check the copies (exit 1 if they differ)
//
// The interface gets two copies, written here and nowhere else (04-migration.md,
// rule 1): <dir>/tokens.css from systeme/project/tokens.css, and
// <dir>/components.css from systeme/project/components/bundle.css, each under a
// first line that says where to change it. --check is for tools/www-check: it
// regenerates nothing and fails when a copy was edited or is out of date.

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const src = path.join(root, 'docs-fr/design/jetons.json');
const out = path.join(root, 'docs-fr/design/systeme/project/tokens.json');
const tokens = JSON.parse(fs.readFileSync(src, 'utf8'));

// 'color.text.primary' -> { value, description }
const flat = new Map();
(function walk(node, trail) {
    for (const [k, v] of Object.entries(node)) {
        if (k.startsWith('$')) continue;
        if (v && typeof v === 'object' && '$value' in v) flat.set([...trail, k].join('.'), { value: v.$value, description: v.$description || '' });
        else if (v && typeof v === 'object') walk(v, [...trail, k]);
    }
})(tokens, []);

const cssName = (key) => key.replace(/\./g, '-');
const take = (prefix) => [...flat.entries()].filter(([k]) => k.startsWith(prefix + '.'));
const used = new Set();
const entry = (key, value) => {
    used.add(key);
    const t = flat.get(key);
    return { name: cssName(key), value: String(value ?? t.value), usage: t.description || `See 03-systeme.md (${key}).` };
};
const list = (prefix, fmt = (v) => v) => ({ tokens: take(prefix).map(([k, t]) => entry(k, fmt(t.value))) });

const px = (k) => flat.get(k).value;
const style = (name, size, family, extra = {}) => ({
    name,
    fontSize: px(`font.size.${size}`),
    lineHeight: px(`font.line-height.${size}`),
    fontWeight: Number(extra.weight ?? 400),
    ...(extra.letterSpacing ? { letterSpacing: extra.letterSpacing } : {}),
    ...(family ? { family } : {}),
    sample: extra.sample,
    usage: extra.usage,
});
const strong = flat.get('font.weight.strong').value;
const tracking = flat.get('font.tracking.label').value;

const ds = {
    name: 'Clairance',
    version: 1,
    meta: {
        source: 'docs-fr/design/jetons.json',
        generator: 'tools/design/ds-export.mjs',
        note: 'Generated: edit jetons.json, check with tokens.mjs, export again.',
    },
    color: {
        themes: [{ id: 'dark', name: 'Dark' }],
        tokens: take('color').map(([k]) => entry(k)),
    },
    type: {
        fonts: [],
        families: {
            'family-text': flat.get('font.family.text').value.replace(/'/g, '"'),
            'family-mono': flat.get('font.family.mono').value.replace(/'/g, '"'),
        },
        groups: [
            {
                name: 'Read (system font)',
                family: 'family-text',
                styles: [
                    style('text-label', 'caption', null, { letterSpacing: tracking, sample: 'ALTITUDE · TO 26L · HEARD', usage: 'Field labels, in capitals, text.secondary.' }),
                    style('text-small', 'small', null, { sample: 'Pilot replied, 4 s later', usage: 'Dense prose: plain-language explanations, help bubbles, captions.' }),
                    style('text-body', 'body', null, { sample: 'Air France quatre sept Kilo X-ray, autorisé ILS piste deux six gauche.', usage: 'Transcriptions and anything read as a sentence.' }),
                    style('text-title', 'title', null, { weight: strong, sample: 'Radio', usage: 'Panel and section titles.' }),
                    style('text-display', 'display', null, { weight: strong, sample: 'Settings', usage: 'Page titles.' }),
                ],
            },
            {
                name: 'Compared (monospace)',
                family: 'family-mono',
                styles: [
                    style('mono-caption', 'caption', null, { sample: '13:12:31 · 124.350', usage: 'Times, frequencies in a header, units, badges, counters.' }),
                    style('mono-small', 'small', null, { sample: 'A320 · 6.2 NM · 2:21', usage: 'Table cells, a row\'s second line.' }),
                    style('mono-body', 'body', null, { sample: '2 400 ft ↓  158 kt  270°', usage: 'Values: level, speed, heading, distance.' }),
                    style('mono-callsign', 'body', null, { weight: strong, sample: 'AFR47KX', usage: 'A callsign in a row or a chip.' }),
                    style('mono-display', 'display', null, { weight: strong, sample: 'AFR47KX', usage: 'The followed aircraft\'s callsign; the brand name.' }),
                ],
            },
        ],
    },
    spacing: list('space'),
    radius: list('radius'),
    shadow: list('shadow'),
    fontSize: list('font.size'),
    lineHeight: list('font.line-height'),
    fontWeight: list('font.weight'),
    letterSpacing: list('font.tracking'),
    borderWidth: list('border-width'),
    zIndex: list('z'),
    size: list('size'),
    duration: list('motion.duration'),
    easing: list('motion.easing', (v) => Array.isArray(v) ? `cubic-bezier(${v.join(', ')})` : v),
};

// Every token of jetons.json must land somewhere; font families went to type.
for (const k of flat.keys()) if (!used.has(k) && !k.startsWith('font.family.')) throw new Error(`token not exported: ${k}`);
const names = new Set();
for (const [fam, v] of Object.entries(ds)) {
    if (!v || !Array.isArray(v.tokens)) continue;
    for (const t of v.tokens) {
        if (!/^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$/.test(t.name)) throw new Error(`bad name ${t.name}`);
        if (names.has(t.name)) throw new Error(`duplicate name ${t.name} (${fam})`);
        names.add(t.name);
    }
}

// --check compares the copies with the sources as they stand: it writes nothing
const args = process.argv.slice(2);
const checkOnly = args[0] === '--check';
if (!checkOnly) {
    fs.mkdirSync(path.dirname(out), { recursive: true });
    fs.writeFileSync(out, JSON.stringify(ds, null, 2) + '\n');
    console.log(`${path.relative(root, out)}: ${names.size} tokens, ${ds.type.groups.reduce((n, g) => n + g.styles.length, 0)} type styles`);
}

// The artifact compiles tokens.css itself when someone saves on its page; until
// then the previews need one. Same layout as the page writes (format.md), and
// the first line the page recognises as its own, so it takes the file over.
const cssOut = path.join(path.dirname(out), 'tokens.css');
const decl = (t) => `  --${t.name.replace(/\./g, '\\.')}: ${t.value};`;
const others = Object.entries(ds).filter(([k, v]) => v && Array.isArray(v.tokens) && !['color', 'shadow'].includes(k));
const css = [
    '/* Clairance — generated from tokens.json */',
    `:root, [data-theme="${ds.color.themes[0].id}"] {`,
    ...ds.color.tokens.map(decl),
    ...ds.shadow.tokens.map(decl),
    '}',
    ':root {',
    ...others.flatMap(([, v]) => v.tokens.map(decl)),
    ...Object.entries(ds.type.families).map(([k, v]) => `  --font-${k}: ${v};`),
    '}',
    ...ds.type.groups.flatMap((g) => g.styles.map((s) => `.${s.name} { font-family: var(--font-${s.family || g.family}); font-size: ${s.fontSize}; line-height: ${s.lineHeight}; font-weight: ${s.fontWeight};${s.letterSpacing ? ` letter-spacing: ${s.letterSpacing};` : ''} }`)),
].join('\n') + '\n';
if (!checkOnly) {
    fs.writeFileSync(cssOut, css);
    console.log(`${path.relative(root, cssOut)} written`);
}

// ---------------------------------------------------------------- copies for www/
if (args[0] === '--www' || checkOnly) {
    const dir = path.resolve(root, args[1] || 'www');
    const copies = [
        ['tokens.css', path.join(path.dirname(out), 'tokens.css')],
        ['components.css', path.join(path.dirname(out), 'components', 'bundle.css')],
    ];
    let bad = 0;
    for (const [name, from] of copies) {
        const head = `/* Copied from ${path.relative(root, from)} by tools/design/ds-export.mjs --www. Change jetons.json or the system there, never this file. */\n`;
        const want = head + fs.readFileSync(from, 'utf8');
        const to = path.join(dir, name);
        if (checkOnly) {
            const have = fs.existsSync(to) ? fs.readFileSync(to, 'utf8') : null;
            if (have !== want) { bad++; console.error(`${path.relative(root, to)}: ${have === null ? 'missing' : 'differs from its source'} (run ds-export.mjs --www)`); }
        } else {
            fs.writeFileSync(to, want);
            console.log(`${path.relative(root, to)} written`);
        }
    }
    if (checkOnly) {
        if (bad) process.exit(1);
        console.log(`copies of the design system in ${path.relative(root, dir) || '.'}: identical to their sources`);
    }
}
