#!/usr/bin/env node
// Capture of a live page for the design review of a migration step
// (docs-fr/design/04-migration.md, "Les mesures qui jugent chaque étape").
//
// Headless Chrome driven through the DevTools protocol, in real time: unlike
// `chrome --screenshot`, the page's feeds (WebSocket, polling) run for as long
// as asked before the shot, so the data is the live data.
//
//   node tools/design/capture.mjs URL OUT.png [--size 1440x900] [--wait 8000]
//        [--setup "js"] [--after "js"] [--probe] [--profile DIR]
//
// --setup runs once the page has loaded, then the page is reloaded (to set
// localStorage keys such as columnFolded before the app reads them);
// --after runs after the wait, then a second short wait (to click a tab);
// what it returns is printed as JSON;
// --probe runs tools/design/contrast-probe.js and prints its summary as JSON;
// --profile keeps the browser profile in DIR (default: a fresh one, removed).
// Captures of the test instance go to runs/design/captures/, never into the
// repository (D75).
import { spawn } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const here = path.dirname(fileURLToPath(import.meta.url));

const argv = process.argv.slice(2);
const opt = (name, def) => { const i = argv.indexOf(name); return i >= 0 ? argv.splice(i, 2)[1] : def; };
const flag = (name) => { const i = argv.indexOf(name); return i >= 0 ? (argv.splice(i, 1), true) : false; };
const [w, h] = opt('--size', '1440x900').split('x').map(Number);
const wait = Number(opt('--wait', '8000'));
const setup = opt('--setup', null);
const after = opt('--after', null);
const keep = opt('--profile', null);
const probe = flag('--probe');
// --keys "Tab*12,j,Enter,Escape": real key presses (CDP), after --after; with
// --trace, what has the focus after each one is printed (step 11: a walk
// through the page without a mouse; a synthetic KeyboardEvent moves no focus).
const keys = opt('--keys', null);
const trace = flag('--trace');
const reducedMotion = flag('--reduced-motion');   // as "Reduce motion" ticked in the system
const traceJs = opt('--trace-js', null);   // an expression printed after each key too
const [url, out] = argv;
if (!url || !out) { console.error('usage: capture.mjs URL OUT.png [options]'); process.exit(2); }

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const profile = keep || fs.mkdtempSync(path.join(os.tmpdir(), 'cl-capture-'));
const port = 9300 + Math.floor(Math.random() * 500);
const chrome = spawn(CHROME, ['--headless=new', '--disable-gpu', '--hide-scrollbars', `--remote-debugging-port=${port}`,
    `--user-data-dir=${profile}`, `--window-size=${w},${h}`, 'about:blank'], { stdio: 'ignore' });

let ws, seq = 0;
const pending = new Map();
function send(method, params = {}) {
    const id = ++seq;
    ws.send(JSON.stringify({ id, method, params }));
    return new Promise((resolve, reject) => pending.set(id, { resolve, reject }));
}
async function evaluate(expression) {
    const r = await send('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true });
    if (r.exceptionDetails) throw new Error(r.exceptionDetails.exception?.description || r.exceptionDetails.text);
    return r.result.value;
}
async function loaded(go) {
    const done = new Promise((resolve) => { onLoad = resolve; });
    await go();
    await Promise.race([done, sleep(15000)]);
}
let onLoad = null;

try {
    let target;
    for (let i = 0; i < 50 && !target; i++) {
        await sleep(200);
        try { target = (await (await fetch(`http://127.0.0.1:${port}/json/list`)).json()).find((t) => t.type === 'page'); } catch { /* not up yet */ }
    }
    if (!target) throw new Error('Chrome did not start');
    ws = new WebSocket(target.webSocketDebuggerUrl);
    await new Promise((resolve, reject) => { ws.onopen = resolve; ws.onerror = reject; });
    ws.onmessage = (m) => {
        const msg = JSON.parse(m.data);
        if (msg.id && pending.has(msg.id)) {
            const p = pending.get(msg.id); pending.delete(msg.id);
            msg.error ? p.reject(new Error(msg.error.message)) : p.resolve(msg.result);
        } else if (msg.method === 'Page.loadEventFired' && onLoad) { onLoad(); onLoad = null; }
    };
    await send('Page.enable');
    if (reducedMotion) await send('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-reduced-motion', value: 'reduce' }] });
    await send('Emulation.setDeviceMetricsOverride', { width: w, height: h, deviceScaleFactor: 1, mobile: w < 768 });
    await loaded(() => send('Page.navigate', { url }));
    if (setup) {
        await evaluate(setup);
        await loaded(() => send('Page.reload'));
    }
    await sleep(wait);
    if (after) {
        const said = await evaluate(after);
        if (said !== undefined) console.log(JSON.stringify(said, null, 1));
        await sleep(Math.min(wait, 3000));
    }
    if (keys) {
        const codes = { Tab: 9, Enter: 13, Escape: 27, ' ': 32, Space: 32, ArrowUp: 38, ArrowDown: 40, ArrowLeft: 37, ArrowRight: 39 };
        const steps = keys.split(',').flatMap((k) => { const [name, n] = k.split('*'); return Array(Number(n || 1)).fill(name); });
        for (const name of steps) {
            const mods = { Alt: 1, Ctrl: 2, Meta: 4, Shift: 8 };
            const parts = name.split('+');
            const last = parts.pop();
            const shift = parts.includes('Shift');
            const modifiers = parts.reduce((m, p) => m | (mods[p] || 0), 0);
            const key = last === 'Space' ? ' ' : last;
            const code = codes[key] || key.toUpperCase().charCodeAt(0);
            // Enter carries "\r": without it the browser clicks no real <button>
            // (design review of step 11), only a page's own handler answered.
            const text = key === 'Enter' && !(modifiers & 7) ? '\r' : key.length === 1 && !(modifiers & 7) ? key : undefined;
            const base = { key, windowsVirtualKeyCode: code, nativeVirtualKeyCode: code, modifiers };
            await send('Input.dispatchKeyEvent', { type: text ? 'keyDown' : 'rawKeyDown', ...base, ...(text ? { text } : {}) });
            await send('Input.dispatchKeyEvent', { type: 'keyUp', ...base });
            await sleep(250);
            if (trace) {
                const where = await evaluate(`(() => { const e = document.activeElement; if (!e || e === document.body) return 'body';
                    const name = (e.getAttribute('aria-label') || e.textContent || '').replace(/\\s+/g, ' ').trim().slice(0, 50);
                    return e.tagName.toLowerCase() + (e.getAttribute('role') ? '[' + e.getAttribute('role') + ']' : '') + ' ' + JSON.stringify(name); })()`);
                const extra = traceJs ? '  | ' + JSON.stringify(await evaluate(traceJs)) : '';
                console.log(`${name.padEnd(10)} -> ${where}${extra}`);
            }
        }
        await sleep(500);
    }
    if (probe) {
        const src = fs.readFileSync(path.join(here, 'contrast-probe.js'), 'utf8');
        console.log(JSON.stringify(await evaluate(src), null, 1));
    }
    const shot = await send('Page.captureScreenshot', { format: 'png' });
    fs.mkdirSync(path.dirname(path.resolve(out)), { recursive: true });
    fs.writeFileSync(out, Buffer.from(shot.data, 'base64'));
    console.error(`${out}: ${w}x${h}, ${wait} ms`);
} finally {
    try { ws?.close(); } catch { /* closed */ }
    chrome.kill();
    if (!keep) { await sleep(300); fs.rmSync(profile, { recursive: true, force: true }); }
}
