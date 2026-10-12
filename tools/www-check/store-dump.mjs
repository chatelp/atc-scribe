// Dump the Alpine store 'atc' as the page would build it, from the scripts given
// in the page's order, in the same sandbox as check.mjs: run it before and after
// moving code, and compare the two files entry by entry (a move must leave every
// entry identical, clocks and generated ids aside). Used for palier 5.1 (12/10).
//   node tools/www-check/store-dump.mjs out.json www/tile-cache.js www/store/*.js www/server-settings.js www/app.js
import { readFileSync, writeFileSync } from 'node:fs';
import vm from 'node:vm';

const [out, ...files] = process.argv.slice(2);
class StoreCaptured extends Error {}
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
        getElementById: () => null, querySelector: () => null, querySelectorAll: () => [], createElement: () => element,
    },
    addEventListener: noop,
    setTimeout: () => 0, clearTimeout: noop, setInterval: () => 0, clearInterval: noop,
    WebSocketClient: class { constructor() {} },
    isSecureContext: false,
    matchMedia: () => ({ matches: false }),
};
sandbox.window = sandbox;
let store = null;
sandbox.Alpine = {
    store(name, value) { if (name === 'atc' && value) { store = value; throw new StoreCaptured(); } return store; },
    effect: noop,
};
const context = vm.createContext(sandbox);
for (const f of files) vm.runInContext(readFileSync(f, 'utf8'), context, { filename: f });
for (const cb of listeners['alpine:init'] || []) {
    try { cb(); } catch (e) { if (!(e instanceof StoreCaptured)) throw e; }
    if (store) break;
}
if (!store) throw new Error('no store captured');

const norm = (fn) => fn.toString().split('\n').map((l) => l.trim()).filter(Boolean).join('\n');
const describe = (v) => {
    if (v === null) return 'null';
    if (typeof v === 'function') return 'fn:' + norm(v);
    if (v instanceof Set) return 'Set(' + v.size + ')';
    if (v instanceof Map) return 'Map(' + v.size + ')';
    if (v instanceof Date) return 'Date';
    if (typeof v === 'object') return 'obj:' + JSON.stringify(v, (k, x) => (typeof x === 'function' ? 'fn:' + norm(x) : x));
    return typeof v + ':' + String(v);
};
const dump = {};
for (const [key, d] of Object.entries(Object.getOwnPropertyDescriptors(store))) {
    dump[key] = d.get || d.set
        ? { kind: 'accessor', get: d.get ? norm(d.get) : null, set: d.set ? norm(d.set) : null }
        : { kind: 'value', value: describe(d.value) };
}
writeFileSync(out, JSON.stringify(dump, null, 1));
console.log(`${Object.keys(dump).length} entries -> ${out}`);
