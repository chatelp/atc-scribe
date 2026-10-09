#!/usr/bin/env node
// www-check: a safety net for removing code from www/ without breaking the page.
//
// Run from the repository root, with Node 22 or later and nothing installed:
//
//     node tools/www-check/check.mjs
//
// It fails (exit 1) when:
//   - a `$store.atc.X` read by index.html, or a store access in another www/
//     script (`store.X`, `this.store.X`, `Alpine.store('atc').X`, ...), names
//     something the Alpine store defined in app.js does not have;
//   - a store method reads `this.X` and X is not on the store;
//   - an Alpine expression in index.html calls a bare function that no loaded
//     script, x-data component or browser global defines;
//   - a local <script src> of index.html points to a missing file;
//   - `node --check` fails on a www/ script, or `node --test www/par/` fails.
//
// The store is not parsed: app.js is evaluated in a sandbox with stubbed
// browser globals, and the object handed to Alpine.store('atc', ...) is
// captured before anything else in the alpine:init handler runs. Getters are
// listed, never invoked. `--quick` skips node --check and the PAR tests.

import { readFileSync, existsSync, readdirSync, statSync } from 'node:fs';
import { join, dirname, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
import vm from 'node:vm';

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..');
const WWW = join(ROOT, 'www');
const quick = process.argv.includes('--quick');
const problems = [];

function walk(dir) {
    return readdirSync(dir).flatMap((name) => {
        const p = join(dir, name);
        return statSync(p).isDirectory() ? walk(p) : [p];
    });
}

const jsFiles = walk(WWW).filter((f) => f.endsWith('.js'));
const html = readFileSync(join(WWW, 'index.html'), 'utf8');
const appSource = readFileSync(join(WWW, 'app.js'), 'utf8');

// ---------------------------------------------------------------- the store

class StoreCaptured extends Error {}

function captureStore() {
    const listeners = {};
    const noop = () => {};
    const element = new Proxy(noop, { get: () => element, apply: () => element });
    const sandbox = {
        console: { log: noop, info: noop, warn: noop, error: noop, debug: noop },
        location: { protocol: 'http:', hostname: 'localhost', port: '8000', href: 'http://localhost:8000/' },
        navigator: {},
        localStorage: { getItem: () => null, setItem: noop, removeItem: noop },
        document: {
            addEventListener: (type, cb) => { (listeners[type] ||= []).push(cb); },
            getElementById: () => null,
            querySelector: () => null,
            querySelectorAll: () => [],
            createElement: () => element,
        },
        addEventListener: noop,
        setTimeout: () => 0, clearTimeout: noop, setInterval: () => 0, clearInterval: noop,
        WebSocketClient: class { constructor() {} },
        isSecureContext: false,
    };
    sandbox.window = sandbox;
    let store = null;
    sandbox.Alpine = {
        store(name, value) {
            if (name === 'atc' && value) { store = value; throw new StoreCaptured(); }
            return store;
        },
        effect: noop,
    };
    const context = vm.createContext(sandbox);
    vm.runInContext(appSource, context, { filename: 'www/app.js' });
    for (const cb of listeners['alpine:init'] || []) {
        try { cb(); } catch (e) { if (!(e instanceof StoreCaptured)) throw e; }
        if (store) break;
    }
    if (!store) throw new Error("app.js never called Alpine.store('atc', {...})");
    return { store, context };
}

const { store, context } = captureStore();
const descriptors = Object.getOwnPropertyDescriptors(store);
const storeKeys = new Set(Object.keys(descriptors));

// Fields the code adds to the store after it is created, by assignment
// (`this.X = ...` in a store method, `store.X = ...` anywhere).
const STORE_REF = String.raw`(?:\bthis\.store|\bmanager\.store|\bmapManager\.store|Alpine\.store\(\s*['"]atc['"]\s*\)|(?<![.\w$])store)\s*\??\.\s*([A-Za-z_$][\w$]*)`;
const assigned = new Set();
const methodSources = [];
for (const [key, d] of Object.entries(descriptors)) {
    for (const fn of [d.value, d.get, d.set]) {
        if (typeof fn === 'function') methodSources.push([key, stripStrings(fn.toString(), { comments: true })]);
    }
}
for (const [, src] of methodSources) {
    for (const m of src.matchAll(/\bthis\.([A-Za-z_$][\w$]*)\s*(?:=(?!=)|\+\+|--|[-+*/|&]=)/g)) assigned.add(m[1]);
}
for (const f of jsFiles) {
    const src = stripStrings(readFileSync(f, 'utf8'), { comments: true, strings: false });
    for (const m of src.matchAll(new RegExp(STORE_REF + String.raw`\s*=(?!=)`, 'g'))) assigned.add(m[1]);
}
const known = (name) => storeKeys.has(name) || assigned.has(name);

// ---------------------------------------------------------------- index.html

const lineOf = (text, index) => text.slice(0, index).split('\n').length;

// Alpine attribute values: x-*, :attr, @event.
const attrs = [];
for (const m of html.matchAll(/\s((?:x-[\w:.-]+|:[\w.-]+|@[\w.:-]+))\s*=\s*"([^"]*)"/g)) {
    attrs.push({ name: m[1], value: m[2], line: lineOf(html, m.index) });
}

const htmlStoreNames = new Set();
for (const m of html.matchAll(/\$store\.atc\s*\??\.\s*([A-Za-z_$][\w$]*)/g)) {
    htmlStoreNames.add(m[1]);
    if (!known(m[1])) problems.push(`index.html:${lineOf(html, m.index)}: $store.atc.${m[1]} is not on the store`);
}

// Blank out comments and string literal text, keeping ${...} inside template
// literals. Regex literals are not recognised; none in www/ upsets this.
function stripStrings(src, { comments = false, strings = true } = {}) {
    let out = '';
    const stack = []; // open template literals, each with its ${ depth
    for (let i = 0; i < src.length; i++) {
        const c = src[i];
        const top = stack[stack.length - 1];
        if (top && top.depth === 0) { // inside template text
            if (c === '\\') { i++; continue; }
            if (c === '`') { stack.pop(); out += ' '; continue; }
            if (c === '$' && src[i + 1] === '{') { top.depth = 1; out += ' '; i++; continue; }
            if (c === '\n') out += c;
            continue;
        }
        if (top) {
            if (c === '{') top.depth++;
            if (c === '}') { top.depth--; if (top.depth === 0) { out += ' '; continue; } }
        }
        if (comments && c === '/' && src[i + 1] === '/') {
            while (i < src.length && src[i] !== '\n') i++;
            out += '\n'; continue;
        }
        if (comments && c === '/' && src[i + 1] === '*') {
            const end = src.indexOf('*/', i + 2);
            const skipped = src.slice(i, end < 0 ? src.length : end + 2);
            out += skipped.replace(/[^\n]/g, '');
            i = end < 0 ? src.length : end + 1; continue;
        }
        if (c === "'" || c === '"') {
            let j = i + 1;
            while (j < src.length && src[j] !== c && src[j] !== '\n') j += src[j] === '\\' ? 2 : 1;
            out += strings ? ' ' + src.slice(i, j).replace(/[^\n]/g, '') : src.slice(i, j + 1);
            i = j; continue;
        }
        if (c === '`') { stack.push({ depth: 0 }); out += ' '; continue; }
        out += c;
    }
    return out;
}

// What a bare call in an Alpine expression may resolve to.
const callable = new Set([
    // keywords that precede a parenthesis
    'if', 'for', 'while', 'switch', 'catch', 'function', 'return', 'typeof', 'await', 'new', 'in', 'of',
    // browser globals Node lacks
    'fetch', 'alert', 'confirm', 'prompt', 'requestAnimationFrame', 'cancelAnimationFrame',
]);
for (const name of Object.getOwnPropertyNames(globalThis)) callable.add(name);
// Globals defined by the page's own scripts.
for (const f of jsFiles) {
    const src = stripStrings(readFileSync(f, 'utf8'), { comments: true, strings: false });
    for (const m of src.matchAll(/^(?:async\s+)?function\s+([A-Za-z_$][\w$]*)|^(?:const|let|var|class)\s+([A-Za-z_$][\w$]*)|\bwindow\.([A-Za-z_$][\w$]*)\s*=(?!=)/gm)) {
        callable.add(m[1] || m[2] || m[3]);
    }
}
// x-data components: a global factory (its returned object's keys) or an inline object.
for (const a of attrs.filter((a) => a.name === 'x-data' && a.value.trim())) {
    const factory = a.value.trim().match(/^([A-Za-z_$][\w$]*)\s*\(\s*\)$/);
    if (factory) {
        const fn = context[factory[1]];
        if (typeof fn !== 'function') { problems.push(`index.html:${a.line}: x-data calls ${factory[1]}(), which no script defines`); continue; }
        Object.keys(Object.getOwnPropertyDescriptors(fn())).forEach((k) => callable.add(k));
    } else {
        for (const m of stripStrings(a.value).matchAll(/([A-Za-z_$][\w$]*)\s*(?:\(|:)/g)) callable.add(m[1]);
    }
}

const ALPINE_MAGIC = new Set(['$nextTick', '$watch', '$dispatch', '$el', '$refs', '$store', '$event', '$data', '$id', '$root']);
let bareCalls = 0;
for (const a of attrs) {
    if (a.name === 'x-data') continue;
    const expr = stripStrings(a.value);
    for (const m of expr.matchAll(/(?<![.\w$])([A-Za-z_$][\w$]*)\s*\(/g)) {
        const name = m[1];
        bareCalls++;
        if (name.startsWith('$') ? ALPINE_MAGIC.has(name) : callable.has(name)) continue;
        problems.push(`index.html:${a.line}: ${a.name}="..." calls ${name}(), which nothing defines`);
    }
}

let scripts = 0;
for (const m of html.matchAll(/<script\b[^>]*\bsrc\s*=\s*"([^"]+)"/g)) {
    const src = m[1];
    if (/^(?:https?:)?\/\//.test(src)) continue;
    scripts++;
    if (!existsSync(join(WWW, src.replace(/^\//, '').split(/[?#]/)[0]))) {
        problems.push(`index.html:${lineOf(html, m.index)}: <script src="${src}"> points to a missing file`);
    }
}

// ---------------------------------------------------------------- the other scripts

const otherNames = new Set();
for (const f of jsFiles) {
    if (f.endsWith('.test.js')) continue;
    const src = stripStrings(readFileSync(f, 'utf8'), { comments: true, strings: false });
    const rel = relative(ROOT, f);
    for (const m of src.matchAll(new RegExp(STORE_REF, 'g'))) {
        if (f.endsWith(join('www', 'app.js')) && !/Alpine\.store/.test(m[0]) && !/^store/.test(m[0])) continue;
        otherNames.add(m[1]);
        if (!known(m[1])) problems.push(`${rel}:${lineOf(src, m.index)}: store.${m[1]} is not on the store`);
    }
}

// `this.X` read inside a store method must be on the store. The exceptions
// are defects already there on main, left to the rewrite of their area; one
// that disappears must leave this list too, so that the list stays true.
const KNOWN_BROKEN = new Map([
    // Clearance alerts (palier 2.2 of docs-fr/32 rewrites the alerts).
    ['showClearanceAlert reads this.addAlert', 'clearance alerts call a function that does not exist'],
    ['refreshSelectedAircraftDetails reads this.selectAircraft', 'same path, after a clearance'],
]);
let thisReads = 0;
const brokenSeen = new Set();
for (const [key, src] of methodSources) {
    for (const m of src.matchAll(/\bthis\.([A-Za-z_$][\w$]*)/g)) {
        thisReads++;
        if (known(m[1])) continue;
        const id = `${key} reads this.${m[1]}`;
        if (KNOWN_BROKEN.has(id)) { brokenSeen.add(id); continue; }
        problems.push(`www/app.js: store.${id}, which is not on the store`);
    }
}
for (const id of KNOWN_BROKEN.keys()) {
    if (!brokenSeen.has(id)) problems.push(`tools/www-check/check.mjs: KNOWN_BROKEN "${id}" no longer happens; remove it from the list`);
}

// ---------------------------------------------------------------- syntax and tests

if (!quick) {
    for (const f of jsFiles) {
        const r = spawnSync(process.execPath, ['--check', f], { encoding: 'utf8' });
        if (r.status !== 0) problems.push(`node --check ${relative(ROOT, f)}:\n${r.stderr.trim()}`);
    }
    const tests = walk(join(WWW, 'par')).filter((f) => f.endsWith('.test.js'));
    const r = spawnSync(process.execPath, ['--test', ...tests], { encoding: 'utf8', cwd: ROOT });
    if (r.status !== 0) problems.push(`node --test www/par/ failed:\n${(r.stdout + r.stderr).trim().split('\n').slice(-30).join('\n')}`);
}

console.log(`store: ${storeKeys.size} properties defined, ${[...assigned].filter((n) => !storeKeys.has(n)).length} added by assignment`);
console.log(`index.html: ${htmlStoreNames.size} store names, ${bareCalls} bare calls, ${scripts} local scripts`);
console.log(`other scripts: ${otherNames.size} store names; store methods: ${thisReads} this.X reads, ${brokenSeen.size} known broken`);
if (!quick) console.log(`node --check on ${jsFiles.length} files, node --test www/par/`);
if (problems.length) {
    console.error(`\n${problems.length} problem(s):\n` + problems.map((p) => '  ' + p).join('\n'));
    process.exit(1);
}
console.log('OK');
