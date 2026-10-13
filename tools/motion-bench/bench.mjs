#!/usr/bin/env node
// The browser bench of the aircraft motion (docs-fr/fiches-cloud/M-mouvement-ecran.md).
//
// The real server, built from this checkout, reads synthetic traffic from the
// generator (tools/motion-bench/gen, internal/adsb/simtraffic) as it would a
// tar1090; the real page it serves runs in headless Chrome, driven through the
// DevTools protocol, 1440 x 900, tab in front. The probe (probe.js) reads in
// the page, after every frame the map draws, the pose actually set on each
// aircraft's OpenLayers feature; the truth comes from the generator, never
// from the page. Nothing here reaches the station or leaves the machine.
//
//   node tools/motion-bench/bench.mjs [--scenario a,b,c,d,e,f] [--chrome PATH]
//        [--out DIR] [--warmup 60] [--measure 60] [--engine JSON]
//        [--label NAME] [--save-reference] [--restart both|server|page] [--headed]
//
// Scenarios, each 60 s after a minute of warm-up but the first:
//   a  the server restarted (and the page reloaded), measured from its first second;
//   b  60 aircraft, column on Right now;
//   c  150 aircraft, column on Traffic;
//   d  60 aircraft, a zoom from 10 to 50 NM and drags of the map while they fly;
//   e  60 aircraft, the tab hidden 30 s then shown again;
//   f  60 aircraft, one of them 18 s without a position at 45 NM, then a fix.
// --engine sets the animation engine's config in the page before measuring
// (e.g. '{"strategy":"B","renderDelayMs":1800}'); --label names the run;
// --save-reference writes the metrics to tools/motion-bench/reference/<label>.json.
// On the Mac: --chrome "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
// (the default there) and, to see the real screen, --headed.
import { spawn, spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { benchConfig } from './config.mjs';
import { analyse, formatTable } from './metrics.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(here, '..', '..');

const argv = process.argv.slice(2);
const opt = (name, def) => { const i = argv.indexOf(name); return i >= 0 ? argv.splice(i, 2)[1] : def; };
const flag = (name) => { const i = argv.indexOf(name); return i >= 0 ? (argv.splice(i, 1), true) : false; };
const DEFAULT_CHROME = process.platform === 'darwin'
    ? '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
    : (process.env.CHROME || '/opt/pw-browsers/chromium-1194/chrome-linux/chrome');
const CHROME = opt('--chrome', DEFAULT_CHROME);
const scenarios = opt('--scenario', 'a,b,c,d,e,f').split(',').map((s) => s.trim()).filter(Boolean);
const WARMUP = Number(opt('--warmup', '60')) * 1000;
const MEASURE = Number(opt('--measure', '60')) * 1000;
const engineConfig = opt('--engine', null);
const label = opt('--label', engineConfig ? 'custom' : 'A');
const outDir = opt('--out', path.join(os.tmpdir(), 'motion-bench'));
const saveReference = flag('--save-reference');
const headed = flag('--headed');
// a: what restarts -- both (the server, and the page reloaded), server (the
// page kept open, it reconnects) or page (reloaded, the server kept running).
const restart = opt('--restart', 'both');
const genPort = Number(opt('--gen-port', '8790'));
const serverPort = Number(opt('--port', '8791'));
fs.mkdirSync(outDir, { recursive: true });

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const log = (...a) => console.log(new Date().toISOString().slice(11, 19), ...a);

// ------------------------------------------------------------- processes
const children = new Set();
function start(cmd, args, logName, opts = {}) {
    const out = fs.openSync(path.join(outDir, logName), 'a');
    const p = spawn(cmd, args, { stdio: ['ignore', out, out], cwd: ROOT, ...opts });
    children.add(p);
    p.on('exit', () => children.delete(p));
    return p;
}
async function stop(p) {
    if (!p || p.exitCode !== null) return;
    const done = new Promise((r) => p.once('exit', r));
    p.kill('SIGTERM');
    await Promise.race([done, sleep(5000)]);
    if (p.exitCode === null) p.kill('SIGKILL');
}
function cleanup() { for (const p of children) { try { p.kill('SIGKILL'); } catch { /* gone */ } } }
process.on('exit', cleanup);
process.on('SIGINT', () => { cleanup(); process.exit(130); });

function build() {
    for (const [out, pkg] of [['co-atc', './cmd/server'], ['gen', './tools/motion-bench/gen']]) {
        const r = spawnSync('go', ['build', '-o', path.join(outDir, out), pkg], { cwd: ROOT, stdio: 'inherit' });
        if (r.status !== 0) throw new Error(`go build ${pkg} failed`);
    }
}

async function waitHttp(url, ms = 20000) {
    const until = Date.now() + ms;
    while (Date.now() < until) {
        try { const r = await fetch(url); if (r.ok) return; } catch { /* not yet */ }
        await sleep(200);
    }
    throw new Error(`${url} did not answer`);
}

async function startServer(name) {
    const dataDir = path.join(outDir, `data-${name}`);
    fs.rmSync(dataDir, { recursive: true, force: true });
    fs.mkdirSync(dataDir, { recursive: true });
    const cfgPath = path.join(outDir, `config-${name}.toml`);
    fs.writeFileSync(cfgPath, benchConfig(path.join(ROOT, 'configs', 'config.toml.example'), {
        port: serverPort, sourceUrl: `http://127.0.0.1:${genPort}/data/`, dataDir, wwwDir: path.join(ROOT, 'www'),
    }));
    const p = start(path.join(outDir, 'co-atc'), ['-config', cfgPath], `server-${name}.log`);
    await waitHttp(`http://127.0.0.1:${serverPort}/api/v1/aircraft`);
    return p;
}

// ------------------------------------------------------------- DevTools
class Cdp {
    static async launch() {
        const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'motion-bench-chrome-'));
        const port = 9400 + Math.floor(Math.random() * 400);
        const args = [`--remote-debugging-port=${port}`, `--user-data-dir=${profile}`, '--window-size=1440,900',
            '--no-first-run', '--no-default-browser-check', '--enable-unsafe-swiftshader', 'about:blank'];
        if (!headed) args.unshift('--headless=new');
        // Chrome refuses to run as root with its sandbox (a cloud container).
        if (process.getuid && process.getuid() === 0) args.unshift('--no-sandbox');
        const proc = start(CHROME, args, 'chrome.log');
        let version;
        for (let i = 0; i < 100 && !version; i++) {
            try { version = await (await fetch(`http://127.0.0.1:${port}/json/version`)).json(); } catch { await sleep(100); }
        }
        if (!version) throw new Error(`Chrome did not start (${CHROME})`);
        const cdp = new Cdp(proc, profile, version);
        await cdp.connect(version.webSocketDebuggerUrl);
        return cdp;
    }
    constructor(proc, profile, version) {
        this.proc = proc; this.profile = profile; this.version = version;
        this.seq = 0; this.pending = new Map(); this.listeners = [];
    }
    connect(url) {
        return new Promise((resolve, reject) => {
            this.ws = new WebSocket(url);
            this.ws.onopen = resolve;
            this.ws.onerror = reject;
            this.ws.onmessage = (ev) => {
                const m = JSON.parse(ev.data);
                if (m.id && this.pending.has(m.id)) {
                    const { resolve: ok, reject: ko } = this.pending.get(m.id);
                    this.pending.delete(m.id);
                    if (m.error) ko(new Error(`${m.error.message} ${m.error.data || ''}`)); else ok(m.result);
                } else if (m.method) {
                    for (const l of this.listeners) l(m);
                }
            };
        });
    }
    send(method, params = {}, sessionId = undefined) {
        const id = ++this.seq;
        this.ws.send(JSON.stringify({ id, method, params, sessionId }));
        return new Promise((resolve, reject) => this.pending.set(id, { resolve, reject }));
    }
    async close() {
        try { await this.send('Browser.close'); } catch { /* closing */ }
        await stop(this.proc);
        try { fs.rmSync(this.profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 300 }); } catch { /* left in tmp */ }
    }
}

class Page {
    static async open(cdp, url) {
        const { targetId } = await cdp.send('Target.createTarget', { url: 'about:blank' });
        const { sessionId } = await cdp.send('Target.attachToTarget', { targetId, flatten: true });
        const page = new Page(cdp, targetId, sessionId);
        await page.send('Page.enable');
        await page.send('Runtime.enable');
        await page.send('Page.addScriptToEvaluateOnNewDocument', { source: fs.readFileSync(path.join(here, 'probe.js'), 'utf8') });
        await page.send('Emulation.setFocusEmulationEnabled', { enabled: true });
        await page.goto(url);
        return page;
    }
    constructor(cdp, targetId, sessionId) { this.cdp = cdp; this.targetId = targetId; this.sessionId = sessionId; }
    send(method, params = {}) { return this.cdp.send(method, params, this.sessionId); }
    async goto(url) {
        const loaded = new Promise((resolve) => {
            const l = (m) => { if (m.sessionId === this.sessionId && m.method === 'Page.loadEventFired') { this.cdp.listeners.splice(this.cdp.listeners.indexOf(l), 1); resolve(); } };
            this.cdp.listeners.push(l);
        });
        await this.send('Page.navigate', { url });
        await Promise.race([loaded, sleep(30000)]);
    }
    async reload() {
        const loaded = new Promise((resolve) => {
            const l = (m) => { if (m.sessionId === this.sessionId && m.method === 'Page.loadEventFired') { this.cdp.listeners.splice(this.cdp.listeners.indexOf(l), 1); resolve(); } };
            this.cdp.listeners.push(l);
        });
        await this.send('Page.reload', { ignoreCache: true });
        await Promise.race([loaded, sleep(30000)]);
    }
    async eval(expression) {
        const r = await this.send('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true });
        if (r.exceptionDetails) throw new Error(r.exceptionDetails.exception?.description || r.exceptionDetails.text);
        return r.result.value;
    }
    async waitFor(expression, ms = 30000) {
        const until = Date.now() + ms;
        while (Date.now() < until) {
            try { if (await this.eval(expression)) return; } catch { /* page not ready */ }
            await sleep(200);
        }
        throw new Error(`timed out waiting for ${expression}`);
    }
    async mouse(type, x, y, extra = {}) {
        await this.send('Input.dispatchMouseEvent', { type, x, y, button: 'left', ...extra });
    }
}

// ------------------------------------------------------------- the page
async function readyPage(page) {
    await page.waitFor('window.__motionProbe && window.__motionProbe.attached');
    await page.waitFor("Object.keys(Alpine.store('atc').aircraft || {}).length > 0");
    if (engineConfig) {
        await page.eval(`Alpine.store('atc').animationEngine.updateConfig(${engineConfig}), true`);
    }
}

// The view, set to show `nm` nautical miles from the station to the top edge.
function viewRangeJs(nm) {
    return `(() => {
        const mm = Alpine.store('atc').mapManager, map = mm._olMap, view = map.getView();
        const c = ol.proj.fromLonLat([4.67, 45.0]);
        const h = map.getSize()[1];
        const metres = ${nm} * 1852 / Math.cos(45 * Math.PI / 180);
        view.setCenter(c);
        view.setResolution(2 * metres / h);
        return view.getZoom();
    })()`;
}

function mapCentreJs() {
    return `(() => { const r = Alpine.store('atc').mapManager._olMap.getTargetElement().getBoundingClientRect();
        return [Math.round(r.left + r.width / 2), Math.round(r.top + r.height / 2)]; })()`;
}

class Recorder {
    constructor(page) { this.page = page; this.parts = []; this.timer = null; }
    async start() {
        await this.page.eval('__motionProbe.start(), true');
        this.timer = setInterval(() => this.drain().catch(() => {}), 3000);
    }
    async drain() {
        const s = await this.page.eval('__motionProbe.drain()');
        if (s) this.parts.push(JSON.parse(s));
    }
    async stop() {
        clearInterval(this.timer);
        await this.page.eval('__motionProbe.stop(), true');
        await this.drain();
    }
    // The parts, joined; hex indexes are stable across drains of one page.
    joined() {
        const out = { hexes: [], poses: [], renders: [], frames: [], longTasks: [], corrections: [], events: [], backend: null };
        for (const p of this.parts) {
            if (p.hexes.length > out.hexes.length) out.hexes = p.hexes;
            for (const k of ['poses', 'renders', 'frames', 'longTasks', 'corrections', 'events']) {
                for (let i = 0; i < p[k].length; i++) out[k].push(p[k][i]);
            }
            out.backend = p.backend || out.backend;
        }
        return out;
    }
}

async function truth(fromMs, toMs) {
    const r = await fetch(`http://127.0.0.1:${genPort}/truth?from=${Math.floor(fromMs) - 1000}&to=${Math.ceil(toMs) + 1000}&step=100`);
    return r.json();
}

// ------------------------------------------------------------- scenarios
const SPEC = {
    a: { aircraft: 60, title: 'server restarted, from its first second' },
    b: { aircraft: 60, title: '60 aircraft, Right now' },
    c: { aircraft: 150, title: '150 aircraft, Traffic open' },
    d: { aircraft: 60, title: 'zoom 10 to 50 NM and drags' },
    e: { aircraft: 60, title: 'tab hidden 30 s, shown again' },
    f: { aircraft: 60, title: 'long gap: 18 s without a position, at 45 NM' },
};

async function runScenario(id) {
    const spec = SPEC[id];
    log(`scenario ${id}: ${spec.title} (${label})`);
    // The long gap starts 25 s into the measurement of f, counted from the
    // generator's start (the page loads within a few seconds of it).
    const genArgs = ['-addr', `127.0.0.1:${genPort}`, '-aircraft', String(spec.aircraft),
        '-long-gap-from', id === 'f' ? String(Math.round((WARMUP + 25000) / 1000) + 5) : '0'];
    const gen = start(path.join(outDir, 'gen'), genArgs, `gen-${id}.log`);
    await waitHttp(`http://127.0.0.1:${genPort}/data/aircraft.json`);
    let server = await startServer(id);
    const cdp = await Cdp.launch();
    const page = await Page.open(cdp, `http://127.0.0.1:${serverPort}/`);
    const result = { id, title: spec.title, label, aircraft: spec.aircraft, chrome: cdp.version.Browser };
    try {
        await readyPage(page);
        result.visibility = await page.eval('document.visibilityState + "/" + document.hasFocus()');
        result.gl = await page.eval(`(() => { try { const c = document.createElement('canvas').getContext('webgl');
            const d = c.getExtension('WEBGL_debug_renderer_info'); return d ? c.getParameter(d.UNMASKED_RENDERER_WEBGL) : 'webgl'; } catch (e) { return 'none'; } })()`);
        if (id === 'c') {
            await page.eval(`(() => { const b = [...document.querySelectorAll('.cl-tabs--column [role=tab]')].find((x) => x.textContent.trim().startsWith('Traffic')); b.click(); return true; })()`);
        }
        if (id === 'd') await page.eval(viewRangeJs(10));
        await sleep(WARMUP);

        const rec = new Recorder(page);
        let t0;
        if (id === 'a') {
            if (restart !== 'page') {
                await stop(server);
                server = await startServer('a2');
            }
            t0 = Date.now();
            if (restart !== 'server') {
                await page.reload();
                await page.waitFor('window.__motionProbe && window.__motionProbe.attached');
            }
            result.restart = restart;
            if (engineConfig) await page.waitFor('!!Alpine.store("atc").animationEngine').then(() => page.eval(`Alpine.store('atc').animationEngine.updateConfig(${engineConfig}), true`));
            await rec.start();
            await sleep(Math.max(0, MEASURE - (Date.now() - t0)));
        } else {
            t0 = Date.now();
            await rec.start();
            if (id === 'd') await zoomAndDrag(page, t0);
            else if (id === 'e') await hideAndShow(page, cdp, t0, result);
            await sleep(Math.max(0, MEASURE - (Date.now() - t0)));
        }
        await rec.stop();
        const t1 = Date.now();
        result.engine = await page.eval(`(() => { const s = Alpine.store('atc'), e = s.animationEngine, now = performance.now();
            const feats = s.mapManager._aircraftRenderer.getFeatureHexes(), vis = s.visibleAircraftOnMap;
            const idle = [];
            for (const [h, st] of e.aircraftStates) if (!st.lastRendered || now - st.lastRendered.timestamp > 2000) idle.push([h, vis.has(h), feats.has(h), st.status, st.lastRendered && Math.round(now - st.lastRendered.timestamp)]);
            return { states: e.aircraftStates.size, features: feats.size, visible: vis.size, idle, stats: e.getStats() }; })()`);
        const data = rec.joined();
        const tr = await truth(t0, t1);
        result.window = [t0, t1];
        // The raw poses, to measure again with tools/motion-bench/analyse.mjs.
        fs.writeFileSync(path.join(outDir, `raw-${label}-${id}.json`), JSON.stringify({ data, truth: tr, t0, t1 }));
        result.metrics = analyse(data, tr, { from: t0, to: t1, scenario: id });
        result.backend = data.backend;
        result.events = data.events.map(([t, n]) => [Math.round(t - t0), n]);
    } finally {
        await cdp.close();
        await stop(server);
        await stop(gen);
    }
    return result;
}

// d: at 10 s the view zooms out from 10 to 50 NM with the wheel, at 25 s the
// map is dragged, at 40 s it zooms back in, at 50 s dragged again.
async function zoomAndDrag(page, t0) {
    const [cx, cy] = await page.eval(mapCentreJs());
    const at = async (ms) => sleep(Math.max(0, t0 + ms - Date.now()));
    const wheel = async (dy, n) => {
        for (let i = 0; i < n; i++) {
            await page.send('Input.dispatchMouseEvent', { type: 'mouseWheel', x: cx, y: cy, deltaX: 0, deltaY: dy });
            await sleep(120);
        }
    };
    const drag = async (dx, dy) => {
        await page.mouse('mouseMoved', cx, cy);
        await page.mouse('mousePressed', cx, cy, { clickCount: 1 });
        for (let i = 1; i <= 30; i++) {
            await page.mouse('mouseMoved', cx + dx * i / 30, cy + dy * i / 30, { buttons: 1 });
            await sleep(33);
        }
        await page.mouse('mouseReleased', cx + dx, cy + dy, { clickCount: 1 });
    };
    await at(10000);
    await page.eval("__motionProbe.mark('zoom-out'), true");
    const z0 = await page.eval('Alpine.store("atc").mapManager._olMap.getView().getZoom()');
    // Out to 50 NM: 2.32 zoom levels; the wheel steps until there.
    for (let i = 0; i < 20; i++) {
        const z = await page.eval('Alpine.store("atc").mapManager._olMap.getView().getZoom()');
        if (z <= z0 - Math.log2(5) + 0.05) break;
        await wheel(100, 1);
    }
    await at(25000);
    await page.eval("__motionProbe.mark('drag'), true");
    await drag(-300, 120);
    await at(40000);
    await page.eval("__motionProbe.mark('zoom-in'), true");
    for (let i = 0; i < 20; i++) {
        const z = await page.eval('Alpine.store("atc").mapManager._olMap.getView().getZoom()');
        if (z >= z0 - 0.05) break;
        await wheel(-100, 1);
    }
    await at(50000);
    await page.eval("__motionProbe.mark('drag'), true");
    await drag(250, -150);
}

// e: hidden from 15 s to 45 s. The window is minimised, as a tab put behind
// another or a window sent to the dock: the page is hidden, animation frames
// stop, its timers slow down, the WebSocket keeps delivering.
async function hideAndShow(page, cdp, t0, result) {
    const at = async (ms) => sleep(Math.max(0, t0 + ms - Date.now()));
    const { windowId } = await cdp.send('Browser.getWindowForTarget', { targetId: page.targetId });
    await at(15000);
    // The focus emulation set at opening would keep the page visible.
    await page.send('Emulation.setFocusEmulationEnabled', { enabled: false });
    await cdp.send('Browser.setWindowBounds', { windowId, bounds: { windowState: 'minimized' } });
    await sleep(500);
    result.hiddenState = await page.eval('document.visibilityState');
    await at(45000);
    await cdp.send('Browser.setWindowBounds', { windowId, bounds: { windowState: 'normal' } });
    await page.send('Emulation.setFocusEmulationEnabled', { enabled: true });
    await page.eval("__motionProbe.mark('shown'), true");
}

// JSON indented, its arrays of numbers and strings on one line.
function compactJson(v) {
    return JSON.stringify(v, null, 1).replace(/\[[^\[\]{}]*\]/g, (a) => a.replace(/\s+/g, ' ').replace(/\[ /, '[').replace(/ \]/, ']'));
}

// ------------------------------------------------------------- main
(async () => {
    build();
    const results = [];
    for (const id of scenarios) {
        if (!SPEC[id]) throw new Error(`unknown scenario ${id}`);
        const r = await runScenario(id);
        results.push(r);
        fs.writeFileSync(path.join(outDir, `result-${label}-${id}.json`), JSON.stringify(r, null, 1));
        log(`scenario ${id}`, JSON.stringify(r.metrics.summary));
    }
    console.log(formatTable(results));
    const machine = { cpus: os.cpus().length, cpu: os.cpus()[0]?.model, platform: `${os.platform()} ${os.arch()}`, chrome: results[0]?.chrome, gl: results[0]?.gl };
    if (saveReference) {
        const dir = path.join(here, 'reference');
        fs.mkdirSync(dir, { recursive: true });
        const file = path.join(dir, `${label}.json`);
        const prev = fs.existsSync(file) ? JSON.parse(fs.readFileSync(file, 'utf8')) : { scenarios: {} };
        prev.machine = machine;
        prev.label = label;
        prev.engine = engineConfig ? JSON.parse(engineConfig) : null;
        for (const r of results) prev.scenarios[r.id] = { title: r.title, aircraft: r.aircraft, backend: r.backend, ...r.metrics.summary, extra: r.metrics.extra };
        fs.writeFileSync(file, compactJson(prev) + '\n');
        log(`reference written: ${path.relative(ROOT, file)}`);
    }
    cleanup();
    process.exit(0);
})().catch((e) => { console.error(e); cleanup(); process.exit(1); });
