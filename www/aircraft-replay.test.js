// node --test www/aircraft-replay.test.js
//
// The replay bench, the page's end (docs-fr/fiches-cloud/L-bloc-position.md).
// internal/adsb/replay_test.go flies synthetic aircraft, runs the server's
// chain over them and records the WebSocket stream it sends, with the true
// paths. Here the stream goes through the page's own feed (store/feed.js) and
// animation engine on a simulated clock at 30 frames a second, a stand-in map
// notes every pose, and the motion drawn is measured against the truth.
//
// The metrics must not get worse than the reference kept next to the
// recording (internal/adsb/testdata/replay/reference.json); after a change
// that improves them, rewrite it with
//
//     REPLAY_UPDATE_REFERENCE=1 node --test www/aircraft-replay.test.js
//
// and never worse at all than those of the upstream engine with the D79
// fixes, measured before fiche L changed anything (step1.json, fixed).
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const zlib = require('node:zlib');
const vm = require('node:vm');

const ROOT = path.join(__dirname, '..');
const DATA = path.join(ROOT, 'internal', 'adsb', 'testdata', 'replay');
const FRAME_MS = 1000 / 30;
const WARMUP_MS = 15000; // the first aircraft_added and the server's first forecasts

// The clock every script in the sandbox reads: the replay's milliseconds,
// as wall time (Date) and as page time (performance, which starts earlier,
// as a page loaded before the stream does).
function simClock(startMs) {
    const clock = { ms: 0 };
    const RealDate = Date;
    class SimDate extends RealDate {
        constructor(...args) {
            if (args.length === 0) super(startMs + clock.ms);
            else super(...args);
        }
        static now() { return startMs + clock.ms; }
    }
    const pageOffsetMs = 12345;
    clock.Date = SimDate;
    clock.performance = { now: () => clock.ms + pageOffsetMs, timeOrigin: startMs - pageOffsetMs };
    clock.pageNow = () => clock.ms + pageOffsetMs;
    return clock;
}

function loadPage(clock, station) {
    const timers = [];
    const sandbox = {
        console: { log() {}, info() {}, warn() {}, error() {}, debug() {} },
        Date: clock.Date,
        performance: clock.performance,
        setTimeout: (fn, ms) => { timers.push({ at: clock.ms + (ms || 0), fn }); return timers.length; },
        clearTimeout() {},
        requestAnimationFrame: () => 0,
        cancelAnimationFrame() {},
        module: { exports: {} },
    };
    sandbox.window = sandbox;
    const context = vm.createContext(sandbox);
    for (const f of ['store/feed.js', 'aircraft-animation.js']) {
        vm.runInContext(fs.readFileSync(path.join(__dirname, f), 'utf8'), context, { filename: `www/${f}` });
    }
    const Engine = sandbox.module.exports;

    // The map: where each aircraft's symbol stands. The symbol is placed at the
    // aircraft's position when it is created, then only the engine moves it
    // (the renderer keeps the pose while smoothing is on).
    const drawn = new Map();
    const mapManager = {
        updateSingleAircraft(hex, aircraft) {
            const a = aircraft && aircraft.adsb;
            if (!drawn.has(hex) && a && Number.isFinite(a.lat) && Number.isFinite(a.lon) && (a.lat !== 0 || a.lon !== 0)) {
                drawn.set(hex, { lat: a.lat, lon: a.lon, heading: null });
            }
        },
        updateAnimatedAircraftPose(hex, pose) {
            if (!drawn.has(hex)) return false;
            drawn.set(hex, { lat: pose.lat, lon: pose.lon, heading: pose.heading, fade: Number.isFinite(pose.fade) ? pose.fade : 1 });
            return true;
        },
        removeAircraft(hex) { drawn.delete(hex); },
        updateFlightPaths() {},
    };

    const domains = sandbox.window.AtcStore.map((f) => f());
    const store = Object.assign({}, ...domains, {
        aircraft: {},
        selectedAircraft: null,
        settings: { lastSeenMinutes: 10, showPaths: false, aircraftAnimation: { enabled: true, interpolationFps: 30, viewportCulling: true } },
        stationLatitude: station[0],
        stationLongitude: station[1],
        mapManager,
        requestInitialAircraftData() {},
        setupAircraftDetailsPanel() {},
        _performFiltering() {},
    });
    const engine = new Engine(mapManager, store);
    store.animationEngine = engine;
    return { store, engine, drawn, timers };
}

const handlers = {
    aircraft_added: 'handleAircraftAdded',
    aircraft_update: 'handleAircraftUpdate',
    aircraft_predicted_state: 'handleAircraftPredictedState',
    aircraft_removed: 'handleAircraftRemoved',
};

function metresBetween(lat1, lon1, lat2, lon2) {
    const north = (lat2 - lat1) * 111320;
    const east = (lon2 - lon1) * 111320 * Math.cos(((lat1 + lat2) / 2) * Math.PI / 180);
    return { north, east, d: Math.hypot(north, east) };
}

function truthAt(tr, ms) {
    const i = ms / tr.step_ms;
    const i0 = Math.max(0, Math.min(tr.points.length - 1, Math.floor(i)));
    const i1 = Math.min(tr.points.length - 1, i0 + 1);
    const f = i - i0;
    const a = tr.points[i0];
    const b = tr.points[i1];
    let dTrack = b[3] - a[3];
    if (dTrack > 180) dTrack -= 360;
    if (dTrack < -180) dTrack += 360;
    return {
        lat: a[0] + (b[0] - a[0]) * f, lon: a[1] + (b[1] - a[1]) * f,
        track: a[3] + dTrack * f, speedMps: a[4] + (b[4] - a[4]) * f,
    };
}

function median(values) {
    const s = values.slice().sort((x, y) => x - y);
    return s.length ? s[Math.floor(s.length / 2)] : 0;
}

// Plays the recording and measures what the map showed.
function replay() {
    const rec = JSON.parse(zlib.gunzipSync(fs.readFileSync(path.join(DATA, 'recording.json.gz'))).toString('utf8'));
    const clock = simClock(Date.parse(rec.start));
    const { store, engine, drawn, timers } = loadPage(clock, rec.station);
    const truth = new Map(rec.truth.map((t) => [t.hex, t]));
    const endMs = Math.max(...rec.truth.map((t) => (t.points.length - 1) * t.step_ms));

    const per = new Map(); // hex -> { steps: [{speed, along, trueSpeed}], errors: [] }
    for (const t of rec.truth) per.set(t.hex, { kind: t.kind, steps: [], errors: [], prev: null });
    let maxCorrectionM = 0;
    let relocations = 0;
    let maxRelocationM = 0;
    const seenCorrections = new WeakSet();

    const noteCorrections = () => {
        for (const [, state] of engine.aircraftStates) {
            const c = state.correction;
            if (c && !seenCorrections.has(c)) {
                seenCorrections.add(c);
                const lat = state.lastReal ? state.lastReal.lat : 0;
                const m = Math.hypot(c.dLat * 111320, c.dLon * 111320 * Math.cos(lat * Math.PI / 180));
                if (clock.ms >= WARMUP_MS && m > maxCorrectionM) maxCorrectionM = m;
            }
        }
    };

    let mi = 0;
    for (let frame = 0; frame * FRAME_MS <= endMs; frame++) {
        const frameMs = frame * FRAME_MS;
        while (mi < rec.messages.length && rec.messages[mi].at_ms <= frameMs) {
            const m = rec.messages[mi++];
            clock.ms = m.at_ms;
            const handler = handlers[m.msg.type];
            if (handler) store[handler](m.msg.data);
            noteCorrections();
        }
        for (let i = timers.length - 1; i >= 0; i--) {
            if (timers[i].at <= frameMs) { const t = timers.splice(i, 1)[0]; t.fn(); }
        }
        clock.ms = frameMs;
        engine._processFrame(clock.pageNow());
        noteCorrections();
        if (frameMs < WARMUP_MS) {
            for (const [hex, p] of drawn) per.get(hex).prev = { ...p };
            continue;
        }
        for (const [hex, p] of drawn) {
            const acc = per.get(hex);
            const tr = truth.get(hex);
            if (!acc || !tr) continue;
            const truthNow = truthAt(tr, frameMs);
            acc.errors.push(metresBetween(truthNow.lat, truthNow.lon, p.lat, p.lon).d);
            // A step taken while the symbol fades out or in is a relocation
            // (a gap too wide to glide over, aircraft-animation.js), not a
            // motion: counted apart, with its distance.
            if (acc.prev && (acc.prev.fade < 1 || p.fade < 1)) {
                const m = metresBetween(acc.prev.lat, acc.prev.lon, p.lat, p.lon).d;
                if (m > 1) { relocations++; maxRelocationM = Math.max(maxRelocationM, m); }
            } else if (acc.prev) {
                const step = metresBetween(acc.prev.lat, acc.prev.lon, p.lat, p.lon);
                const trackRad = truthNow.track * Math.PI / 180;
                acc.steps.push({
                    speed: step.d / (FRAME_MS / 1000),
                    along: step.north * Math.cos(trackRad) + step.east * Math.sin(trackRad),
                    trueSpeed: truthNow.speedMps,
                });
            }
            acc.prev = { ...p };
        }
    }

    let steps = 0, backwards = 0, slow = 0, jumps = 0;
    const ratios = [];
    const errors = [];
    const byKind = {};
    for (const [, acc] of per) {
        const med = median(acc.steps.map((s) => s.speed));
        for (const s of acc.steps) {
            steps++;
            if (s.along < 0) backwards++;
            if (s.speed < 0.3 * med) slow++;
            if (s.speed > 2 * med) jumps++;
            if (s.trueSpeed > 1) ratios.push(s.speed / s.trueSpeed);
        }
        errors.push(...acc.errors);
        const k = (byKind[acc.kind] ||= { errors: [], ratios: [], steps: 0, backwards: 0, slow: 0, jumps: 0 });
        k.errors.push(...acc.errors);
        for (const s of acc.steps) {
            k.steps++;
            if (s.along < 0) k.backwards++;
            if (s.speed < 0.3 * med) k.slow++;
            if (s.speed > 2 * med) k.jumps++;
            if (s.trueSpeed > 1) k.ratios.push(s.speed / s.trueSpeed);
        }
    }
    const mean = (v) => v.reduce((a, b) => a + b, 0) / Math.max(1, v.length);
    const std = (v) => { const m = mean(v); return Math.sqrt(mean(v.map((x) => (x - m) ** 2))); };
    const pct = (n) => Number((100 * n / Math.max(1, steps)).toFixed(2));
    const sortedErrors = errors.slice().sort((a, b) => a - b);
    return {
        steps,
        backwardsPct: pct(backwards),
        slowPct: pct(slow),
        jumpsPct: pct(jumps),
        speedStdPct: Number((100 * std(ratios)).toFixed(2)),
        maxCorrectionM: Number(maxCorrectionM.toFixed(1)),
        // Gaps relocated rather than glided over (fiche M), and the widest.
        relocations,
        maxRelocationM: Number(maxRelocationM.toFixed(1)),
        meanErrorM: Number(mean(errors).toFixed(1)),
        p95ErrorM: Number(sortedErrors[Math.floor(sortedErrors.length * 0.95)].toFixed(1)),
        // By kind of flight: mean error (m); backwards, slow and jump steps,
        // standard deviation of the speed drawn over the true speed (%).
        byKind: Object.fromEntries(Object.entries(byKind).sort().map(([k, v]) => [k, [
            Number(mean(v.errors).toFixed(1)),
            Number((100 * v.backwards / v.steps).toFixed(1)),
            Number((100 * v.slow / v.steps).toFixed(1)),
            Number((100 * v.jumps / v.steps).toFixed(1)),
            Number((100 * std(v.ratios)).toFixed(1)),
        ]])),
    };
}

// The metrics that must not get worse, and by how much they may move without
// counting as worse (rounding, nothing more).
const GUARDED = {
    backwardsPct: 0.05, slowPct: 0.05, jumpsPct: 0.05, speedStdPct: 0.05,
    maxCorrectionM: 0.5, meanErrorM: 0.2, p95ErrorM: 0.5,
};

test('the replayed motion is no worse than the reference', () => {
    const m = replay();
    console.log('replay metrics', JSON.stringify(m).replace(/,"(?=[a-zA-Z])/g, ',\n "'));
    const refPath = path.join(DATA, 'reference.json');
    if (process.env.REPLAY_UPDATE_REFERENCE === '1') {
        fs.writeFileSync(refPath, JSON.stringify(m, null, 2) + '\n');
        return;
    }
    const ref = JSON.parse(fs.readFileSync(refPath, 'utf8'));
    for (const [k, tolerance] of Object.entries(GUARDED)) {
        assert.ok(m[k] <= ref[k] + tolerance, `${k}: ${m[k]}, reference ${ref[k]}`);
    }
});

test('the replayed motion is no worse than before fiche L', () => {
    const m = replay();
    const before = JSON.parse(fs.readFileSync(path.join(DATA, 'step1.json'), 'utf8'));
    for (const k of Object.keys(GUARDED)) {
        assert.ok(m[k] <= before[k], `${k}: ${m[k]}, ${before[k]} before fiche L`);
    }
});
