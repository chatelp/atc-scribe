#!/usr/bin/env node
// Visual inventory of the web interface (docs-fr/design/01-inventaire.md).
//
// Static measure, no browser: reads www/ (index.html, every script outside
// vendor/ and the tests, style.css) and the compiled Tailwind file, which tells
// what each utility class really does. Reports colours, font sizes, spacing,
// radii, shadows, states, icons and contrast, with the file each one comes from.
//
//   node tools/design/inventory.mjs            # markdown report on stdout
//   node tools/design/inventory.mjs --json f   # also the raw numbers in f
//
// Counting rules, so that two runs compare:
// - index.html: class tokens are read inside class="…", :class="…" and
//   x-bind:class="…" only; other attributes and text are not class names.
// - scripts: tokens are read inside string literals only ('…', "…", `…`).
// - a token counts as a Tailwind class when the compiled CSS has a rule for it,
//   which is how Tailwind itself decides; a token shaped like a colour or size
//   utility that the compiled CSS lacks is reported as missing (it renders
//   unstyled).
// - colours are normalised to #rrggbb (plus alpha when below 1).

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const www = path.join(root, 'www');
const rel = (f) => path.relative(root, f);

// ---------------------------------------------------------------- sources

const walk = (d) => fs.readdirSync(d, { withFileTypes: true }).flatMap((e) =>
    e.isDirectory() ? walk(path.join(d, e.name)) : [path.join(d, e.name)]);
const scripts = walk(www).filter((f) => f.endsWith('.js') && !f.includes('/vendor/')
    && !f.endsWith('.test.js') && !f.endsWith('/sw.js')).sort();
const indexHtml = path.join(www, 'index.html');
const styleCss = path.join(www, 'style.css');
const compiledCss = path.join(www, 'vendor/tailwind.css');
const twConfig = path.join(root, 'tools/tailwind/tailwind.config.js');
const read = (f) => fs.readFileSync(f, 'utf8');
const lineOf = (text, idx) => text.slice(0, idx).split('\n').length;

// ---------------------------------------------------------------- colours
import { toHex, parseColor, rgbOf, over, contrast, lab, deltaE, family, CVD, simulate } from './color.mjs';
// ---------------------------------------------------------------- compiled Tailwind

// Splits a CSS text into [selector, body, media] rules, following @media nesting.
function cssRules(text, media = '') {
    const out = [];
    // Blank the comments out, keeping offsets, so that line numbers stay true.
    text = text.replace(/\/\*[\s\S]*?\*\//g, (c) => c.replace(/[^\n]/g, ' '));
    let i = 0;
    while (i < text.length) {
        const open = text.indexOf('{', i);
        if (open < 0) break;
        const head = text.slice(i, open).trim();
        let depth = 1, j = open + 1;
        while (j < text.length && depth) { if (text[j] === '{') depth++; else if (text[j] === '}') depth--; j++; }
        const body = text.slice(open + 1, j - 1);
        if (head.startsWith('@media') || head.startsWith('@supports')) out.push(...cssRules(body, head));
        else if (!head.startsWith('@')) out.push({ selector: head, body, media, start: open });
        i = j;
    }
    return out;
}
function decls(body) {
    return body.split(';').map((d) => d.trim()).filter(Boolean).map((d) => {
        const k = d.indexOf(':');
        return [d.slice(0, k).trim(), d.slice(k + 1).trim()];
    });
}
const unescapeCss = (s) => s.replace(/\\([0-9a-fA-F]{1,6}) ?/g, (_, h) => String.fromCodePoint(parseInt(h, 16)))
    .replace(/\\(.)/g, '$1');

const compiled = new Map(); // class name -> [{prop, value}]
for (const r of cssRules(read(compiledCss))) {
    for (const sel of r.selector.split(/,(?![^(]*\))/)) {
        const classes = [...sel.matchAll(/\.((?:\\[0-9a-fA-F]{1,6} ?|\\.|[A-Za-z0-9_-])+)/g)];
        if (!classes.length) continue;
        const name = unescapeCss(classes.at(-1)[1]);
        const ds = decls(r.body);
        const vars = Object.fromEntries(ds.filter(([p]) => p.startsWith('--tw-') && p.endsWith('opacity')).map(([p, v]) => [p, parseFloat(v)]));
        const list = compiled.get(name) || [];
        for (const [prop, value] of ds) list.push({ prop, value, vars, media: r.media });
        compiled.set(name, list);
    }
}

// ---------------------------------------------------------------- class tokens

const splitVariants = (tok) => { // 'md:hover:bg-x' -> [['md','hover'], 'bg-x']
    const parts = []; let depth = 0, cur = '';
    for (const c of tok) {
        if (c === '[') depth++; else if (c === ']') depth--;
        if (c === ':' && depth === 0) { parts.push(cur); cur = ''; } else cur += c;
    }
    parts.push(cur);
    return [parts.slice(0, -1), parts.at(-1)];
};
// Splits on spaces, quotes, braces, parentheses and commas, except inside
// [...] (arbitrary values such as shadow-[0_5px_15px_rgba(0,0,0,0.5)]).
function tokenise(s) {
    const out = []; let cur = '', depth = 0;
    for (const c of s) {
        if (c === '[') depth++; else if (c === ']') depth = Math.max(0, depth - 1);
        if (depth === 0 && /[\s"'`{}(),;]/.test(c)) { if (cur) out.push(cur); cur = ''; } else cur += c;
    }
    if (cur) out.push(cur);
    return out.map((t) => t.replace(/^[:!]+(?=[a-z])/, '').replace(/:$/, ''));
}

const uses = []; // {token, file, line, conditional}
function scanHtml(file) {
    const text = read(file);
    const re = /\s(class|:class|x-bind:class)\s*=\s*("([^"]*)"|'([^']*)')/g;
    for (const m of text.matchAll(re)) {
        const body = m[3] ?? m[4];
        const conditional = m[1] !== 'class';
        const ln = lineOf(text, m.index);
        for (const t of tokenise(body)) uses.push({ token: t, file, line: ln, conditional });
    }
}
function jsStrings(text) {
    const out = [];
    const re = /\/\/[^\n]*|\/\*[\s\S]*?\*\/|'(?:\\.|[^'\\\n])*'|"(?:\\.|[^"\\\n])*"|`(?:\\.|[^`\\])*`/g;
    for (const m of text.matchAll(re)) {
        if (m[0].startsWith('//') || m[0].startsWith('/*')) continue;
        out.push({ s: m[0].slice(1, -1), index: m.index });
    }
    return out;
}
function scanJs(file) {
    const text = read(file);
    for (const { s, index } of jsStrings(text)) {
        const ln = lineOf(text, index);
        for (const t of tokenise(s.replace(/\$\{[^}]*\}/g, ' '))) uses.push({ token: t, file, line: ln, conditional: true });
    }
}
scanHtml(indexHtml);
scripts.forEach(scanJs);

// Content globs of the Tailwind config, to explain a missing class.
const cfgText = read(twConfig);
const contentGlobs = [...cfgText.matchAll(/'\.\.\/\.\.\/(www\/[^']+)'/g)].map((m) => m[1]);
const globRe = (g) => new RegExp('^' + g.replace(/[.+^${}()|[\]\\]/g, '\\$&').replace(/\*\*\//g, '(?:.*/)?').replace(/\*/g, '[^/]*') + '$');
const scannedByTailwind = (f) => contentGlobs.some((g) => globRe(g).test(rel(f)));

const PALETTE = 'slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose|white|black|transparent|current|background|panel|text|highlight|warning|danger|border|hover';
const looksLikeUtility = new RegExp(`^-?(text|bg|border|ring|divide|from|to|via|fill|stroke|placeholder|outline|accent|shadow)-(?:(?:${PALETTE})(?:-\\d{2,3})?(?:\\/\\d+)?|\\[[^\\]]+\\])$|^(?:p|px|py|pt|pb|pl|pr|m|mx|my|mt|mb|ml|mr|gap|space-[xy]|w|h|min-w|max-w|rounded(?:-[trbl]{1,2})?|z|opacity|leading|tracking)-(?:\\[[^\\]]+\\]|\\d+(?:\\.5)?|px|full|auto|none|sm|md|lg|xl|2xl|3xl)$`);

// ---------------------------------------------------------------- classify

function classify(cls) {
    const ds = compiled.get(cls) || [];
    const cat = {};
    for (const { prop, value, vars } of ds) {
        if (prop === 'font-size') cat.size = value;
        else if (prop === 'color') cat.text = parseColor(value, vars);
        else if (prop === 'background-color') cat.bg = parseColor(value, vars);
        else if (/^border(-[a-z]+)?-color$/.test(prop)) cat.border = parseColor(value, vars);
        else if (prop === '--tw-ring-color') cat.ring = parseColor(value, vars);
        else if (prop === 'fill' || prop === 'stroke') cat.svg = parseColor(value, vars);
        else if (prop === 'outline-color' || prop === 'accent-color' || prop === 'caret-color') cat.other_color = parseColor(value, vars);
        else if (prop.startsWith('--tw-gradient')) cat.gradient = value;
        else if (prop === '--tw-shadow') cat.shadow = value;
        else if (prop === 'box-shadow') cat.shadow = cat.shadow || value;
        else if (prop.startsWith('border') && prop.includes('radius')) cat.radius = value;
        else if (/^(padding|margin)/.test(prop) || prop === 'gap' || prop.startsWith('column-gap') || prop.startsWith('row-gap')) cat.spacing = cat.spacing || (value.startsWith('calc(') ? (value.match(/-?[\d.]+(?:rem|px)/) || [value])[0] : value);
        else if (prop === 'opacity') cat.opacity = value;
        else if (/^border(-[a-z]+)?-width$/.test(prop)) cat.borderWidth = value;
        else if (prop === 'font-weight') cat.weight = value;
        else if (prop === 'font-family') cat.family = value;
        else if (prop === 'letter-spacing') cat.tracking = value;
        else if (prop === 'line-height') cat.leading = value;
        else if (prop.startsWith('transition') || prop.startsWith('animation')) cat.motion = value;
        else if (prop === 'z-index') cat.z = value;
        else if (prop === 'cursor') cat.cursor = value;
    }
    return cat;
}
const pxOf = (v) => {
    const m = String(v).match(/^([\d.]+)(px|rem|em)$/);
    if (!m) return null;
    return m[2] === 'px' ? +m[1] : +m[1] * 16;
};

// ---------------------------------------------------------------- aggregate class uses

const inc = (map, key, by = 1) => map.set(key, (map.get(key) || 0) + by);
const where = new Map(); // key -> Map(file -> count)
const addWhere = (key, file) => { const m = where.get(key) || new Map(); inc(m, rel(file)); where.set(key, m); };

const classCount = new Map();
const missing = new Map(); // token -> [{file,line}]
const variantCount = new Map();
const stateClasses = new Map(); // variant -> Map(base -> n)
const byCat = { size: new Map(), text: new Map(), bg: new Map(), border: new Map(), ring: new Map(), svg: new Map(), shadow: new Map(), radius: new Map(), spacing: new Map(), opacity: new Map(), borderWidth: new Map(), weight: new Map(), motion: new Map(), z: new Map(), tracking: new Map() };
let conditionalUses = 0, staticUses = 0;
for (const u of uses) {
    const [variants, base] = splitVariants(u.token);
    const known = compiled.has(u.token);
    if (!known) {
        if (looksLikeUtility.test(base)) {
            const l = missing.get(u.token) || [];
            l.push({ file: rel(u.file), line: u.line, scanned: scannedByTailwind(u.file) });
            missing.set(u.token, l);
        }
        continue;
    }
    inc(classCount, u.token);
    if (u.conditional) conditionalUses++; else staticUses++;
    for (const v of variants) {
        inc(variantCount, v);
        const m = stateClasses.get(v) || new Map(); inc(m, base); stateClasses.set(v, m);
    }
    const cat = classify(u.token);
    for (const k of Object.keys(byCat)) {
        if (cat[k] === undefined || cat[k] === null) continue;
        const key = k === 'size' ? `${pxOf(cat.size) ?? cat.size}px` : String(cat[k]);
        const label = `${key}\u0000${base}`;
        inc(byCat[k], label);
        addWhere(`${k}:${label}`, u.file);
    }
}

// ---------------------------------------------------------------- raw CSS (style.css, inline styles, CSS in scripts)

const rawColors = []; // {color, file, line, context}
const rawSizes = [];  // {px, file, line, context}
const rawDecl = { radius: [], shadow: [], spacing: [], motion: [], z: [] };
const cssStates = new Map();
const COLOR_RE = /#[0-9a-fA-F]{3,8}\b|rgba?\([^)]*\)/g;

function scanCssText(text, file, offsetLine = 0, isStylesheet = true) {
    for (const r of cssRules(text)) {
        const ln = offsetLine + lineOf(text, r.start);
        for (const [prop, value] of decls(r.body)) {
            for (const c of value.matchAll(COLOR_RE)) {
                const color = parseColor(c[0]);
                if (color) rawColors.push({ color, file: rel(file), line: ln, context: `${r.selector.slice(0, 60)} { ${prop} }` });
            }
            if (prop === 'font-size') rawSizes.push({ px: pxOf(value.replace(/\s*!important/, '')) ?? value, file: rel(file), line: ln, context: r.selector.slice(0, 60) });
            if (prop.includes('radius')) rawDecl.radius.push({ value, file: rel(file), line: ln, context: r.selector.slice(0, 50) });
            if (prop === 'box-shadow' || prop === 'text-shadow' || prop === 'filter' && value.includes('shadow')) rawDecl.shadow.push({ value, file: rel(file), line: ln, context: r.selector.slice(0, 50) });
            if (/^(padding|margin|gap)/.test(prop)) rawDecl.spacing.push({ value, file: rel(file), line: ln, context: r.selector.slice(0, 50) });
            if (/^(transition|animation)/.test(prop)) rawDecl.motion.push({ value, file: rel(file), line: ln, context: r.selector.slice(0, 50) });
            if (prop === 'z-index') rawDecl.z.push({ value, file: rel(file), line: ln, context: r.selector.slice(0, 50) });
        }
        if (isStylesheet) {
            for (const s of r.selector.matchAll(/:(hover|focus-visible|focus-within|focus|active|disabled|checked|first-child|last-child|placeholder)|\[(disabled|aria-[\w-]+)[^\]]*\]|\.(active|selected|disabled|is-[\w-]+|error|muted)\b/g)) {
                inc(cssStates, s[1] || s[2] || '.' + s[3]);
            }
        }
    }
}
const styleText = read(styleCss);
scanCssText(styleText, styleCss);
const mediaQueries = [...styleText.matchAll(/@media[^{]+/g)].map((m) => m[0].trim());
const keyframes = [...styleText.matchAll(/@keyframes\s+([\w-]+)/g)].map((m) => m[1]);
const reducedMotion = /prefers-reduced-motion/.test(styleText);

// Inline style="…" in index.html, and colours / sizes in scripts.
const htmlText = read(indexHtml);
for (const m of htmlText.matchAll(/\sstyle\s*=\s*"([^"]*)"/g)) {
    const ln = lineOf(htmlText, m.index);
    scanCssText(`x{${m[1]}}`, indexHtml, ln - 1, false);
}
// Colours elsewhere in index.html (Alpine :style objects, SVG attributes), class
// attributes blanked out: their arbitrary values are read through the compiled CSS.
{
    const t = htmlText.replace(/<!--[\s\S]*?-->/g, (c) => c.replace(/[^\n]/g, ' '))
        .replace(/\s(class|:class|x-bind:class|style)\s*=\s*"[^"]*"/g, (c) => c.replace(/[^\n]/g, ' '));
    for (const c of t.matchAll(COLOR_RE)) {
        const color = parseColor(c[0]);
        const ln = lineOf(t, c.index);
        if (color) rawColors.push({ color, file: rel(indexHtml), line: ln, context: htmlText.split('\n')[ln - 1].trim().slice(0, 90) });
    }
}
for (const m of htmlText.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)) scanCssText(m[1], indexHtml, lineOf(htmlText, m.index) - 1);

const jsColorFiles = new Map();
for (const f of scripts) {
    const text = read(f);
    for (const { s, index } of jsStrings(text)) {
        const ln = lineOf(text, index);
        for (const c of s.matchAll(COLOR_RE)) {
            const color = parseColor(c[0]);
            const srcLine = text.split('\n')[ln - 1].trim().slice(0, 90);
            if (color) { rawColors.push({ color, file: rel(f), line: ln, context: srcLine }); inc(jsColorFiles, rel(f)); }
            else if (/\$\{/.test(c[0]) || /rgba?\(/.test(c[0])) rawColors.push({ color: 'dynamic', file: rel(f), line: ln, context: srcLine });
        }
        for (const fm of s.matchAll(/(?:^|[\s;'"])(?:font-size\s*:\s*|(?:bold|normal|[1-9]00)?\s*)(\d+(?:\.\d+)?)px(?:\s*\/[\d.]+)?\s+(?:ui-monospace|monospace|sans-serif|system-ui|Inter|-apple-system|"|')/g)) {
            rawSizes.push({ px: +fm[1], file: rel(f), line: ln, context: text.split('\n')[ln - 1].trim().slice(0, 80) });
        }
        for (const fm of s.matchAll(/font-size\s*:\s*(\d*\.?\d+)(px|rem|em)/g)) {
            const px = fm[2] === 'px' ? +fm[1] : Math.round(+fm[1] * 16 * 10) / 10;
            const ln2 = lineOf(text, index + fm.index);
            rawSizes.push({ px, file: rel(f), line: ln2, context: text.split('\n')[ln2 - 1].trim().slice(0, 80) });
        }
    }
}
// Same position found twice by the two font regexes: keep one.
const seenSize = new Set();
const sizes = rawSizes.filter((s) => { const k = `${s.file}:${s.line}:${s.px}`; if (seenSize.has(k)) return false; seenSize.add(k); return true; });

// ---------------------------------------------------------------- icons and glyphs

const FA_MOD = new Set(['fa', 'fas', 'far', 'fab', 'fa-solid', 'fa-regular', 'fa-brands', 'fa-fw', 'fa-spin', 'fa-pulse', 'fa-beat', 'fa-fade', 'fa-xs', 'fa-sm', 'fa-lg', 'fa-xl', 'fa-2xs', 'fa-2xl', 'fa-1x', 'fa-2x', 'fa-3x', 'fa-stack', 'fa-stack-1x', 'fa-stack-2x', 'fa-inverse', 'fa-border', 'fa-flip-horizontal', 'fa-flip-vertical', 'fa-rotate-90', 'fa-rotate-180', 'fa-rotate-270']);
const icons = new Map(), iconStyle = new Map();
const iconSources = [[indexHtml, htmlText], ...scripts.map((f) => [f, read(f)])];
for (const [f, text] of iconSources) {
    for (const m of text.matchAll(/\b(fa-[a-z0-9-]+|fa[srb])\b/g)) {
        if (FA_MOD.has(m[1])) { if (/^(fa-solid|fa-regular|fas|far|fa-brands|fab)$/.test(m[1])) inc(iconStyle, m[1]); continue; }
        inc(icons, m[1]); addWhere(`icon:${m[1]}`, f);
    }
}
const glyphs = new Map();
const GLYPH_RE = /[←-⇿⌀-⏿■-◿☀-➿⬀-⯿\u{1F300}-\u{1FAFF}]/gu;
for (const [f, text] of iconSources) {
    const t = text.replace(/<!--[\s\S]*?-->/g, '').replace(/\/\/[^\n]*/g, '');
    for (const m of t.matchAll(GLYPH_RE)) { inc(glyphs, m[0]); addWhere(`glyph:${m[0]}`, f); }
}
const svgAssets = fs.readdirSync(path.join(www, 'assets')).filter((n) => n.endsWith('.svg'));

// ---------------------------------------------------------------- accessibility counters

const count = (text, re) => (text.match(re) || []).length;
const allJs = scripts.map(read).join('\n');
const a11y = {
    'data-help (index.html, static)': count(htmlText, /\sdata-help\s*=/g),
    ':data-help (index.html, bound)': count(htmlText, /\s(?::|x-bind:)data-help\s*=/g),
    'data-help (scripts)': count(allJs, /data-help/g),
    'aria-* attributes': count(htmlText, /\saria-[\w-]+\s*=/g) + count(htmlText, /\s:aria-[\w-]+\s*=/g) + count(allJs, /aria-[\w-]+=/g),
    'role= attributes': count(htmlText, /\srole\s*=/g),
    'title= attributes': count(htmlText, /\s:?title\s*=/g),
    'tabindex': count(htmlText, /tabindex/g),
    'alt=': count(htmlText, /\salt\s*=/g),
    '<button>': count(htmlText, /<button\b/g),
    '<input>': count(htmlText, /<input\b/g),
    '<select>': count(htmlText, /<select\b/g),
    'focus/focus-visible variants (Tailwind)': (variantCount.get('focus') || 0) + (variantCount.get('focus-visible') || 0),
    ':focus rules (style.css)': (cssStates.get('focus') || 0) + (cssStates.get('focus-visible') || 0),
    '@media in style.css': mediaQueries.length,
    'prefers-reduced-motion': reducedMotion ? 'yes' : 'no',
};

// ---------------------------------------------------------------- report

const out = [];
const p = (s = '') => out.push(s);
const sortDesc = (m) => [...m.entries()].sort((a, b) => b[1] - a[1]);
const top = (m, key) => { const w = where.get(key); return w ? sortDesc(w).map(([f, n]) => `${f.replace(/^www\//, '')} ${n}`).join(', ') : ''; };
const fmt = (x) => (typeof x === 'number' ? (Number.isInteger(x) ? String(x) : x.toFixed(2)) : x);

p(`# Inventaire brut — généré par tools/design/inventory.mjs`);
p();
p(`Sources lues : index.html (${htmlText.split('\n').length} l.), style.css (${styleText.split('\n').length} l.), ${scripts.length} scripts hors vendor et tests ; CSS compilé : ${compiled.size} classes.`);
p(`Utilisations de classes Tailwind reconnues : ${staticUses + conditionalUses} (${staticUses} statiques dans class="", ${conditionalUses} conditionnelles : :class ou chaînes des scripts) ; classes distinctes : ${classCount.size}.`);
p();

// Font sizes
p('## Tailles de texte');
p();
const sizeTotals = new Map();
for (const [label, n] of byCat.size) inc(sizeTotals, label.split('\u0000')[0], n);
for (const s of sizes) inc(sizeTotals, `${s.px}px (CSS/JS)`, 1);
p('| Taille | Utilisations | Classes ou origine |');
p('|---|---:|---|');
const sizeRows = new Map();
for (const [label, n] of byCat.size) {
    const [px, base] = label.split('\u0000');
    const r = sizeRows.get(px) || { n: 0, classes: [] }; r.n += n; r.classes.push(`${base} ${n}`); sizeRows.set(px, r);
}
for (const s of sizes) {
    const px = `${s.px}px`; const r = sizeRows.get(px) || { n: 0, classes: [] }; r.n++; r.classes.push(`${s.file.replace(/^www\//, '')}:${s.line}`); sizeRows.set(px, r);
}
for (const [px, r] of [...sizeRows.entries()].sort((a, b) => parseFloat(a[0]) - parseFloat(b[0]))) {
    p(`| ${px} | ${r.n} | ${r.classes.slice(0, 12).join(', ')}${r.classes.length > 12 ? ` … (+${r.classes.length - 12})` : ''} |`);
}
p();
const weightRows = sortDesc(byCat.weight);
p(`Graisses : ${weightRows.map(([l, n]) => `${l.split('\u0000')[1]} ${n}`).join(', ')}.`);
p(`Interlettrage : ${sortDesc(byCat.tracking).map(([l, n]) => `${l.split('\u0000')[1]} ${n}`).join(', ') || 'aucun'}.`);
p(`Familles déclarées dans style.css et les scripts : ${[...new Set([...styleText.matchAll(/font-family\s*:\s*([^;]+)/g)].map((m) => m[1].trim()))].join(' | ')} ; body : ${/<body[^>]*class="([^"]*)"/.exec(htmlText)?.[1] ?? '?'}.`);
p();

// Colours
p('## Couleurs');
p();
const colorUse = new Map(); // color -> {n, roles:Map, files:Map}
const addColor = (color, role, n, file) => {
    if (!color || color === 'dynamic') return;
    const c = colorUse.get(color) || { n: 0, roles: new Map(), files: new Map() };
    c.n += n; inc(c.roles, role, n); inc(c.files, file.replace(/^www\//, ''), n); colorUse.set(color, c);
};
for (const role of ['text', 'bg', 'border', 'ring', 'svg']) {
    for (const [label, n] of byCat[role]) {
        const [color, base] = label.split('\u0000');
        const w = where.get(`${role}:${label}`);
        for (const [f, k] of w) addColor(color, `${role} (${base})`, k, f);
    }
}
for (const r of rawColors) addColor(r.color, 'en dur', 1, r.file);
const distinct = [...colorUse.keys()];
const opaque = [...new Set(distinct.map((c) => c.split('@')[0]))];
p(`Valeurs distinctes : ${distinct.length} (dont ${distinct.filter((c) => c.includes('@')).length} avec transparence) ; teintes opaques distinctes : ${opaque.length}.`);
p(`Classes de couleur Tailwind distinctes : texte ${byCat.text.size}, fond ${byCat.bg.size}, bordure ${byCat.border.size}, anneau ${byCat.ring.size}, SVG ${byCat.svg.size}.`);
p(`Valeurs en dur (hors Tailwind) : ${rawColors.filter((r) => r.color !== 'dynamic').length} occurrences, ${new Set(rawColors.filter((r) => r.color !== 'dynamic').map((r) => r.color)).size} valeurs distinctes ; par fichier : ${sortDesc(rawColors.reduce((m, r) => (inc(m, r.file), m), new Map())).map(([f, n]) => `${f.replace(/^www\//, '')} ${n}`).join(', ')}.`);
p();
p('### Par famille de teinte');
p();
const fam = new Map();
for (const c of opaque) { const f = family(c); const e = fam.get(f) || { values: new Set(), n: 0 }; e.values.add(c); fam.set(f, e); }
for (const [c, u] of colorUse) fam.get(family(c.split('@')[0])).n += u.n;
p('| Famille | Teintes distinctes | Utilisations |');
p('|---|---:|---:|');
for (const [f, e] of [...fam.entries()].sort((a, b) => b[1].n - a[1].n)) p(`| ${f} | ${e.values.size} | ${e.n} |`);
p();
p('### Regroupement des teintes voisines (ΔE76 < 5 : indiscernables à l\'œil côte à côte)');
p();
const clusters = [];
for (const c of opaque.sort((a, b) => (colorUse.get(b)?.n || 0) - (colorUse.get(a)?.n || 0))) {
    const cl = clusters.find((k) => deltaE(k[0], c) < 5);
    if (cl) cl.push(c); else clusters.push([c]);
}
p(`${opaque.length} teintes opaques → ${clusters.length} groupes. Groupes de plus d'une valeur :`);
p();
for (const cl of clusters.filter((k) => k.length > 1)) p(`- ${cl.join(', ')}`);
p();
p('### Les 60 couleurs les plus utilisées');
p();
p('| Couleur | Famille | Utilisations | Rôles (classe : n) | Fichiers |');
p('|---|---|---:|---|---|');
for (const [c, u] of [...colorUse.entries()].sort((a, b) => b[1].n - a[1].n).slice(0, 60)) {
    p(`| \`${c}\` | ${family(c.split('@')[0])} | ${u.n} | ${sortDesc(u.roles).slice(0, 4).map(([r, n]) => `${r} ${n}`).join(' ; ')} | ${sortDesc(u.files).slice(0, 4).map(([f, n]) => `${f} ${n}`).join(', ')} |`);
}
p();
p('### Couleurs en dur, détail par fichier');
p();
const byFile = new Map();
for (const r of rawColors) { const l = byFile.get(r.file) || []; l.push(r); byFile.set(r.file, l); }
for (const [f, l] of byFile) {
    p(`**${f}** (${l.length})`);
    p();
    const agg = new Map();
    for (const r of l) { const e = agg.get(r.color) || []; e.push(r.line); agg.set(r.color, e); }
    p([...agg.entries()].sort((a, b) => b[1].length - a[1].length).map(([c, ls]) => `\`${c}\` ×${ls.length} (l. ${ls.slice(0, 6).join(', ')}${ls.length > 6 ? '…' : ''})`).join(' · '));
    p();
}

// Contrast
p('## Contrastes WCAG (texte sur fond)');
p();
const bgs = [['#0a0a0a', 'background (thème)'], ['#1a1a1a', 'panel (thème)'], ['#000000', 'carte (fond noir)']];
for (const [label, n] of sortDesc(byCat.bg).slice(0, 8)) {
    const c = label.split('\u0000')[0];
    if (!c.includes('@') && !bgs.some((b) => b[0] === c)) bgs.push([c, label.split('\u0000')[1]]);
}
const textColors = new Map();
for (const [label, n] of byCat.text) { const [c, base] = label.split('\u0000'); const e = textColors.get(c) || { n: 0, names: [] }; e.n += n; e.names.push(base); textColors.set(c, e); }
const topText = [...textColors.entries()].sort((a, b) => b[1].n - a[1].n).slice(0, 24);
p('Contraste de chaque couleur de texte Tailwind (les 24 plus fréquentes) sur les fonds principaux. AA = 4,5 pour le texte courant, 3 pour le texte ≥ 18,66 px gras ou 24 px ; en dessous de 3, illisible de nuit à distance.');
p();
p(`| Texte | Classes | n | ${bgs.map((b) => `sur ${b[0]} (${b[1]})`).join(' | ')} |`);
p(`|---|---|---:|${bgs.map(() => '---:').join('|')}|`);
for (const [c, e] of topText) {
    p(`| \`${c}\` | ${e.names.slice(0, 3).join(', ')} | ${e.n} | ${bgs.map((b) => { const r = contrast(c, b[0]); return `${r.toFixed(1)}${r < 3 ? ' ✗' : r < 4.5 ? ' ~' : ''}`; }).join(' | ')} |`);
}
p();
p('✗ sous 3:1 ; ~ entre 3 et 4,5:1. Ces paires sont possibles, pas forcément rendues : les paires réelles, pondérées par le nombre de caractères, viennent de la sonde (contrast-probe.js) dans le navigateur.');
p();

// Palettes that carry a meaning: can each colour be told from the others,
// with normal sight and with the three dichromacies (Machado et al. 2009,
// severity 1, applied in linear RGB)?
const classColor = (cls, role = 'bg') => { const c = classify(cls); return (c[role] || c.text || c.bg || '').split('@')[0]; };
const parText = read(path.join(www, 'par/par-view.js'));
const phaseBlock = /PHASE_COLORS\s*=\s*\{([\s\S]*?)\}/.exec(parText)[1];
const phases = [...phaseBlock.matchAll(/'([^']+)':\s*'(#[0-9A-Fa-f]{6})'/g)].map((m) => [m[1], m[2].toLowerCase()]);
const webglText = read(path.join(www, 'map/renderers/aircraft-webgl.js'));
const bandSets = [...webglText.matchAll(/case 'high': return '(#\w+)';\s*case 'mid': return '(#\w+)';\s*case 'low': return '(#\w+)';\s*case 'very-low': return '(#\w+)';[\s\S]*?return '(#\w+)';/g)]
    .map((m) => ['high', 'mid', 'low', 'very-low', 'cruise/default'].map((k, i) => [k, m[i + 1].toLowerCase()]));
const palettes = [
    { name: 'Phases de vol (par-view.js PHASE_COLORS = étiquettes de la carte = classes de la liste)', bg: '#000000', colors: phases },
    { name: 'Bandes d\'altitude, fond sombre (aircraft-webgl.js getAltitudeBandColor)', bg: '#000000', colors: bandSets[1] || [] },
    { name: 'Bandes d\'altitude, fond clair (idem)', bg: '#ffffff', colors: bandSets[0] || [] },
    { name: 'Voyants d\'état (index.html, bandeau : ok / warn / bad / unknown)', bg: '#1a1a1a', colors: [['ok', classColor('bg-green-400')], ['warn', classColor('bg-amber-400')], ['bad', classColor('bg-red-500')], ['unknown', classColor('bg-gray-500')]] },
    { name: 'Force des indices d\'appariement (weak / medium / strong)', bg: '#1a1a1a', colors: [['weak', classColor('bg-amber-400')], ['medium', classColor('bg-sky-300')], ['strong', classColor('bg-green-400')]] },
    { name: 'Locuteur (ATC / PILOT / inconnu)', bg: '#1a1a1a', colors: [['ATC', classColor('text-orange-400', 'text')], ['PILOT', classColor('text-green-400', 'text')], ['unknown', classColor('text-gray-400', 'text')]] },
    { name: 'Consigne en clair (clairance / autre)', bg: '#1a1a1a', colors: [['clearance', classColor('text-emerald-300', 'text')], ['other', classColor('text-amber-300', 'text')]] },
    { name: 'Tuile de fréquence, voyant de niveau (failed / on / muted)', bg: '#1a1a1a', colors: [['failed', '#b45555'], ['on', '#4caf50'], ['muted', '#888888']] },
];
p('## Palettes porteuses de sens, et le daltonisme');
p();
p('ΔE76 minimal entre deux couleurs de la palette : sous 10, deux pastilles de quelques pixels se confondent ; sous 5, côte à côte. Simulation de Machado et al. (2009), sévérité 1 ; protanopie et deutéranopie touchent environ 8 % des hommes, la tritanopie est rare.');
p();
p('| Palette | Couleurs | Contraste min. sur le fond | ΔE min. normal | protanopie | deutéranopie | tritanopie |');
p('|---|---|---:|---|---|---|---|');
const minPair = (cols, f) => {
    let best = null;
    for (let i = 0; i < cols.length; i++) for (let j = i + 1; j < cols.length; j++) {
        const d = deltaE(f(cols[i][1]), f(cols[j][1]));
        if (!best || d < best.d) best = { d, a: cols[i][0], b: cols[j][0] };
    }
    return best ? `${best.d.toFixed(1)} (${best.a}/${best.b})${best.d < 10 ? ' ✗' : ''}` : '';
};
const confusable = [];
for (const pal of palettes) {
    const cols = pal.colors.filter(([, c]) => c);
    const minC = Math.min(...cols.map(([, c]) => contrast(c, pal.bg)));
    p(`| ${pal.name} | ${cols.map(([k, c]) => `${k} \`${c}\``).join(', ')} | ${minC.toFixed(1)} | ${minPair(cols, (c) => c)} | ${Object.values(CVD).map((m) => minPair(cols, (c) => simulate(c, m))).join(' | ')} |`);
    for (const [kind, m] of Object.entries(CVD)) {
        for (let i = 0; i < cols.length; i++) for (let j = i + 1; j < cols.length; j++) {
            const d = deltaE(simulate(cols[i][1], m), simulate(cols[j][1], m));
            if (d < 10) confusable.push(`${pal.name.split(' (')[0]} : ${cols[i][0]}/${cols[j][0]} en ${kind} (ΔE ${d.toFixed(1)})`);
        }
    }
}
p();
p(`Paires confondues (ΔE < 10) : ${confusable.length}.`);
p();
for (const c of confusable) p(`- ${c}`);
p();

// Spacing, radius, shadow, border, opacity, z, motion
const catTable = (title, m, n = 30) => {
    p(`## ${title}`);
    p();
    const rows = new Map();
    for (const [label, k] of m) { const [v, base] = label.split('\u0000'); const r = rows.get(v) || { n: 0, cls: [] }; r.n += k; r.cls.push(`${base} ${k}`); rows.set(v, r); }
    p('| Valeur | n | Classes |'); p('|---|---:|---|');
    for (const [v, r] of [...rows.entries()].sort((a, b) => b[1].n - a[1].n).slice(0, n)) p(`| \`${v}\` | ${r.n} | ${r.cls.sort((a, b) => parseInt(b.split(' ').at(-1)) - parseInt(a.split(' ').at(-1))).slice(0, 8).join(', ')} |`);
    p();
    return rows;
};
const spacingRows = catTable('Espacements (padding, margin, gap, space)', byCat.spacing, 40);
p(`Valeurs d'espacement distinctes : ${spacingRows.size}. En dur dans style.css : ${[...new Set(rawDecl.spacing.map((d) => d.value))].length} valeurs distinctes sur ${rawDecl.spacing.length} déclarations (${[...rawDecl.spacing.reduce((m, d) => (inc(m, d.value), m), new Map()).entries()].sort((a, b) => b[1] - a[1]).slice(0, 15).map(([v, n]) => `${v} ×${n}`).join(', ')}).`);
p();
catTable('Rayons', byCat.radius);
p(`En dur : ${rawDecl.radius.map((d) => `${d.value} (${d.context})`).join(' ; ')}.`);
p();
catTable('Ombres', byCat.shadow);
p(`En dur : ${rawDecl.shadow.map((d) => `${d.value} (${d.context})`).join(' ; ') || 'aucune'}.`);
p();
catTable('Épaisseurs de bordure', byCat.borderWidth);
catTable('Opacités', byCat.opacity);
catTable('Plans (z-index)', byCat.z);
p(`En dur : ${rawDecl.z.map((d) => `${d.value} (${d.context})`).join(' ; ')}.`);
p();
catTable('Mouvement (transitions, animations)', byCat.motion);
p(`En dur : ${rawDecl.motion.length} déclarations ; @keyframes : ${keyframes.join(', ') || 'aucune'} ; prefers-reduced-motion : ${reducedMotion ? 'oui' : 'non'}.`);
p();

// States
p('## États');
p();
p('Variantes Tailwind (préfixes) :');
p();
p('| Variante | n | Classes les plus fréquentes |'); p('|---|---:|---|');
for (const [v, n] of sortDesc(variantCount)) p(`| ${v}: | ${n} | ${sortDesc(stateClasses.get(v)).slice(0, 8).map(([b, k]) => `${b} ${k}`).join(', ')} |`);
p();
p(`Sélecteurs d'état dans style.css : ${sortDesc(cssStates).map(([s, n]) => `${s} ${n}`).join(', ')}.`);
p();

// Icons
p('## Icônes');
p();
p(`Font Awesome : ${icons.size} icônes distinctes, ${[...icons.values()].reduce((a, b) => a + b, 0)} occurrences ; styles : ${sortDesc(iconStyle).map(([s, n]) => `${s} ${n}`).join(', ')}.`);
p();
p(sortDesc(icons).map(([i, n]) => `${i.replace(/^fa-/, '')} ${n}`).join(' · '));
p();
p(`Glyphes Unicode servant d'icône ou de signe (hors commentaires) : ${sortDesc(glyphs).map(([g, n]) => `${g} ${n} (${top(null, `glyph:${g}`)})`).join(' · ') || 'aucun'}.`);
p();
p(`SVG dans www/assets : ${svgAssets.join(', ')}.`);
p();

// Accessibility
p('## Accessibilité (compteurs)');
p();
for (const [k, v] of Object.entries(a11y)) p(`- ${k} : ${v}`);
p(`- @media de style.css : ${mediaQueries.join(' ; ') || 'aucune'}`);
p();

// style.css: rules nobody uses any more, and the weight of !important.
p('## style.css : règles orphelines');
p();
{
    const corpus = htmlText + '\n' + allJs;
    const defined = new Map();
    for (const r of cssRules(styleText)) {
        for (const m of r.selector.matchAll(/\.([A-Za-z_][\w-]*)/g)) {
            if (!defined.has(m[1])) defined.set(m[1], lineOf(styleText, r.start));
        }
        for (const m of r.selector.matchAll(/#([A-Za-z_][\w-]*)/g)) {
            if (!defined.has('#' + m[1])) defined.set('#' + m[1], lineOf(styleText, r.start));
        }
    }
    const isUsed = (name) => {
        const bare = name.replace(/^#/, '');
        // OpenLayers generates its own ol-* classes; leave them out.
        if (/^ol-/.test(bare)) return true;
        return new RegExp(`(^|[^\\w-])${bare.replace(/[-]/g, '\\-')}([^\\w-]|$)`).test(corpus);
    };
    const orphans = [...defined.entries()].filter(([n]) => !isUsed(n));
    p(`Sélecteurs de classe ou d'identifiant définis : ${defined.size} ; sans aucune occurrence dans index.html ni les scripts : ${orphans.length}.`);
    p();
    if (orphans.length) p(orphans.map(([n, l]) => `\`${n.startsWith('#') ? n : '.' + n}\` (l. ${l})`).join(' · '));
    p();
    p(`\`!important\` dans style.css : ${count(styleText, /!important/g)} ; classes Tailwind préfixées par ! : ${[...classCount.keys()].filter((c) => c.startsWith('!')).length}.`);
    p();
}

// Identity, as the page shows it.
p('## Identité de la page');
p();
p(`- <title> : « ${/<title>([^<]*)<\/title>/.exec(htmlText)?.[1] ?? '?'} »`);
p(`- favicon (<link rel="icon">) : ${/rel="(shortcut )?icon"/.test(htmlText) ? 'oui' : 'aucun'} ; manifeste d'application : ${/rel="manifest"/.test(htmlText) ? 'oui' : 'aucun'} ; theme-color : ${/name="theme-color"/.test(htmlText) ? 'oui' : 'aucun'}`);
p(`- occurrences de « Co-ATC » / « co-atc » dans index.html : ${count(htmlText, /co-atc/gi)} ; dans les scripts : ${count(allJs, /co-atc/gi)}`);
p(`- tabular-nums (chiffres à chasse fixe hors monospace) : ${classCount.get('tabular-nums') || 0}`);
p();

// Missing classes
p('## Classes absentes du CSS compilé (rendues sans style)');
p();
if (!missing.size) p('Aucune.');
else {
    p(`Contenu analysé par Tailwind (tailwind.config.js) : ${contentGlobs.join(', ')}.`);
    p();
    p('| Classe | Où | Fichier analysé par Tailwind ? |'); p('|---|---|---|');
    for (const [t, l] of [...missing.entries()].sort()) p(`| \`${t}\` | ${l.slice(0, 4).map((x) => `${x.file.replace(/^www\//, '')}:${x.line}`).join(', ')}${l.length > 4 ? '…' : ''} | ${[...new Set(l.map((x) => (x.scanned ? 'oui' : 'non')))].join('/')} |`);
}
p();

process.stdout.write(out.join('\n') + '\n');

const jsonAt = process.argv.indexOf('--json');
if (jsonAt > 0) {
    const obj = (m) => Object.fromEntries([...m.entries()].map(([k, v]) => [k.replace('\u0000', ' '), v]));
    fs.writeFileSync(process.argv[jsonAt + 1], JSON.stringify({
        generated: new Date().toISOString(),
        classes: obj(classCount), sizes: obj(byCat.size), rawSizes: sizes,
        colors: Object.fromEntries([...colorUse.entries()].map(([c, u]) => [c, { n: u.n, family: family(c.split('@')[0]), roles: Object.fromEntries(u.roles), files: Object.fromEntries(u.files) }])),
        rawColors, clusters, spacing: obj(byCat.spacing), radius: obj(byCat.radius), shadow: obj(byCat.shadow),
        variants: Object.fromEntries(variantCount), cssStates: Object.fromEntries(cssStates), icons: Object.fromEntries(icons),
        glyphs: Object.fromEntries(glyphs), a11y, missing: Object.fromEntries(missing),
    }, null, 2));
}
