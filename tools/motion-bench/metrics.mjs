// The bench's metrics: those of the replay bench (www/aircraft-replay.test.js),
// so the two compare, measured here on the poses the map drew, plus the
// screen's own.
//
// A step is the move of one aircraft from one pose the map drew to the next
// different one: a frame the map redraws for another reason (a label, another
// layer) without moving the aircraft changes nothing on screen and is not a
// step. A pose held longer than STEP_MAX_MS while the map kept drawing is a
// step too, a slow one (the aircraft stood still on screen); a time the map
// did not draw at all (the tab hidden) is not.
//   backwardsPct  steps whose move along the true track is negative
//   slowPct       steps slower than 30 % of the aircraft's median step speed
//   jumpsPct      steps faster than twice it
//   speedStdPct   standard deviation of step speed over true speed, the steps
//                 timed by when the map drew them (what the screen shows)
//   speedStdEnginePct  the same, the steps timed by when the engine computed
//                 them: the motion itself, without the drawing's timing
//   maxCorrectionM  largest gap the engine started to fade (its corrections)
//   relocations, maxRelocationM  gaps relocated rather than glided over (the
//                 symbol fades out and in): their steps are not motion steps
//   meanErrorM, p95ErrorM  drawn pose against the true position, each frame
//   moveMedMs, moveP95Ms  time between two poses of an aircraft (its cadence)
//   frameMedMs, frameP95Ms, framesOver50Pct  animation frames (rAF) intervals
//   drawMedMs, drawP95Ms  intervals between frames the map drew
//   longTasksPer10s, blockedMsPer10s  main thread tasks over 50 ms
//   glyphJumpsPct  steps where the symbol turned more than 3 degrees
const STEP_MAX_MS = 250;

function median(v) { const s = v.slice().sort((a, b) => a - b); return s.length ? s[Math.floor(s.length / 2)] : 0; }
function quantile(v, q) { const s = v.slice().sort((a, b) => a - b); return s.length ? s[Math.min(s.length - 1, Math.floor(s.length * q))] : 0; }
const mean = (v) => v.reduce((a, b) => a + b, 0) / Math.max(1, v.length);
const std = (v) => { const m = mean(v); return Math.sqrt(mean(v.map((x) => (x - m) ** 2))); };
const r1 = (x) => Number(x.toFixed(1));
const r2 = (x) => Number(x.toFixed(2));

function metres(lat1, lon1, lat2, lon2) {
    const north = (lat2 - lat1) * 111320;
    const east = (lon2 - lon1) * 111320 * Math.cos(((lat1 + lat2) / 2) * Math.PI / 180);
    return { north, east, d: Math.hypot(north, east) };
}

function truthAt(tr, fromMs, stepMs, t) {
    const i = (t - fromMs) / stepMs;
    const n = tr.points.length;
    const i0 = Math.max(0, Math.min(n - 1, Math.floor(i)));
    const i1 = Math.min(n - 1, i0 + 1);
    const f = Math.max(0, Math.min(1, i - i0));
    const a = tr.points[i0];
    const b = tr.points[i1];
    let dTrack = b[2] - a[2];
    if (dTrack > 180) dTrack -= 360;
    if (dTrack < -180) dTrack += 360;
    return { lat: a[0] + (b[0] - a[0]) * f, lon: a[1] + (b[1] - a[1]) * f, track: a[2] + dTrack * f, speedMps: a[3] + (b[3] - a[3]) * f };
}

function angleDiffDeg(a, b) { let d = ((a - b) % 360 + 360) % 360; if (d > 180) d -= 360; return d; }

// Motion metrics over a set of steps and samples.
function motion(perAircraft, filter = () => true) {
    let steps = 0, backwards = 0, slow = 0, jumps = 0, glyphJumps = 0, glyphSteps = 0;
    const ratios = [];
    const engineRatios = [];
    const errors = [];
    for (const acc of perAircraft.values()) {
        const own = acc.steps.filter(filter);
        const med = median(acc.steps.map((s) => s.speed));
        for (const s of own) {
            steps++;
            if (s.along < 0) backwards++;
            if (s.speed < 0.3 * med) slow++;
            if (s.speed > 2 * med) jumps++;
            if (s.trueSpeed > 1) ratios.push(s.speed / s.trueSpeed);
            if (s.trueSpeed > 1 && s.engineDt > 0) engineRatios.push(s.m / (s.engineDt / 1000) / s.trueSpeed);
            if (Number.isFinite(s.turnDeg)) { glyphSteps++; if (Math.abs(s.turnDeg) > 3) glyphJumps++; }
        }
        for (const e of acc.errors) if (filter(e)) errors.push(e.m);
    }
    const pct = (n, d = steps) => r2(100 * n / Math.max(1, d));
    return {
        steps,
        backwardsPct: pct(backwards), slowPct: pct(slow), jumpsPct: pct(jumps),
        speedStdPct: r2(100 * std(ratios)),
        speedStdEnginePct: r2(100 * std(engineRatios)),
        meanErrorM: r1(mean(errors)), p95ErrorM: r1(quantile(errors, 0.95)),
        glyphJumpsPct: pct(glyphJumps, glyphSteps),
    };
}

export function analyse(data, truth, { from, to, scenario }) {
    const truthByHex = new Map(truth.truth.map((t) => [t.hex, t]));
    // When the engine computed the poses of each drawn frame.
    const engineAt = new Map();
    for (let i = 0; i + 1 < data.renders.length; i += 2) engineAt.set(data.renders[i], data.renders[i + 1]);
    const renderTimes = [];
    for (let i = 0; i < data.renders.length; i += 2) renderTimes.push(data.renders[i]);
    const per = new Map();
    const P = data.poses;
    const W = data.poseStride || 5; // 5 before the fade was read
    for (let i = 0; i < P.length; i += W) {
        const t = P[i];
        if (t < from || t > to) continue;
        const hex = data.hexes[P[i + 1]];
        const tr = truthByHex.get(hex);
        if (!tr) continue;
        let acc = per.get(hex);
        if (!acc) { acc = { hex, kind: tr.kind, gap: tr.gap_ms, samples: [], steps: [], errors: [], pauses: 0, relocationSteps: 0 }; per.set(hex, acc); }
        acc.samples.push({ t, et: engineAt.get(t), lat: P[i + 2], lon: P[i + 3], rot: P[i + 4], fade: W > 5 ? P[i + 5] : 1 });
    }
    for (const acc of per.values()) {
        const tr = truthByHex.get(acc.hex);
        acc.samples.sort((a, b) => a.t - b.t);
        let prev = null; // the previous frame drawn with this aircraft
        let anchor = null; // the last pose that differed
        let paused = false; // a frame without drawing since the anchor
        for (const s of acc.samples) {
            const truthNow = truthAt(tr, truth.from_ms, truth.step_ms, s.t);
            acc.errors.push({ t: s.t, m: metres(truthNow.lat, truthNow.lon, s.lat, s.lon).d });
            if (prev && s.t - prev.t > STEP_MAX_MS) { paused = true; acc.pauses++; }
            prev = s;
            if (!anchor) { anchor = s; continue; }
            const moved = Math.abs(s.lat - anchor.lat) > 1e-9 || Math.abs(s.lon - anchor.lon) > 1e-9 ||
                (Number.isFinite(s.rot) && Number.isFinite(anchor.rot) && Math.abs(s.rot - anchor.rot) > 1e-6);
            const held = s.t - anchor.t > STEP_MAX_MS;
            if (!moved && !held) continue;
            if (anchor.fade < 1 || s.fade < 1) {
                acc.relocationSteps++;
            } else if (!paused) {
                const dt = s.t - anchor.t;
                const step = metres(anchor.lat, anchor.lon, s.lat, s.lon);
                const trackRad = truthNow.track * Math.PI / 180;
                acc.steps.push({
                    t: s.t, dt, m: step.d, engineDt: s.et - anchor.et,
                    speed: step.d / (dt / 1000),
                    along: step.north * Math.cos(trackRad) + step.east * Math.sin(trackRad),
                    trueSpeed: truthNow.speedMps,
                    turnDeg: Number.isFinite(s.rot) && Number.isFinite(anchor.rot) ? angleDiffDeg(s.rot * 180 / Math.PI, anchor.rot * 180 / Math.PI) : NaN,
                });
            }
            anchor = s;
            paused = false;
        }
    }

    const inWindow = (t) => t >= from && t <= to;
    const frames = data.frames.filter(inWindow).sort((a, b) => a - b);
    const frameIntervals = frames.slice(1).map((t, i) => t - frames[i]);
    const renders = renderTimes.filter(inWindow).sort((a, b) => a - b);
    const drawIntervals = renders.slice(1).map((t, i) => t - renders[i]);
    const longTasks = data.longTasks.filter(([t]) => inWindow(t));
    const tenSeconds = (to - from) / 10000;
    const corrections = data.corrections.filter(([t]) => inWindow(t));

    const moves = [];
    for (const acc of per.values()) for (const st of acc.steps) if (st.dt <= STEP_MAX_MS) moves.push(st.dt);
    const summary = {
        aircraft: per.size,
        ...motion(per),
        moveMedMs: r1(median(moves)), moveP95Ms: r1(quantile(moves, 0.95)),
        maxCorrectionM: r1(corrections.reduce((m, c) => Math.max(m, c[2]), 0)),
        relocations: (data.relocations || []).filter(([t]) => inWindow(t)).length,
        maxRelocationM: r1((data.relocations || []).filter(([t]) => inWindow(t)).reduce((m, c) => Math.max(m, c[2]), 0)),
        frameMedMs: r1(median(frameIntervals)), frameP95Ms: r1(quantile(frameIntervals, 0.95)),
        framesOver50Pct: r2(100 * frameIntervals.filter((d) => d > 50).length / Math.max(1, frameIntervals.length)),
        drawMedMs: r1(median(drawIntervals)), drawP95Ms: r1(quantile(drawIntervals, 0.95)),
        longTasksPer10s: r1(longTasks.length / tenSeconds),
        blockedMsPer10s: r1(longTasks.reduce((s, [, d]) => s + d, 0) / tenSeconds),
    };

    const extra = {};
    // By kind of flight: mean error, backwards, slow, jumps, speed std.
    const kinds = {};
    for (const acc of per.values()) (kinds[acc.kind] ||= new Map()).set(acc.hex, acc);
    extra.byKind = Object.fromEntries(Object.entries(kinds).sort().map(([k, m]) => {
        const x = motion(m);
        return [k, [x.meanErrorM, x.backwardsPct, x.slowPct, x.jumpsPct, x.speedStdPct]];
    }));
    // The five aircraft drawn furthest from the truth, on average.
    extra.worst = [...per.values()].map((acc) => {
        const x = motion(new Map([[acc.hex, acc]]));
        return [acc.hex, acc.kind, x.meanErrorM, x.steps, acc.pauses];
    }).sort((a, b) => b[2] - a[2]).slice(0, 5);
    // Second by second (the first minute after a restart): speed std, slow
    // and jump steps, mean error.
    if (scenario === 'a') {
        extra.bySecond = [];
        for (let s = from; s < to; s += 1000) {
            const x = motion(per, (e) => e.t >= s && e.t < s + 1000);
            extra.bySecond.push([Math.round((s - from) / 1000), x.speedStdPct, x.slowPct, x.jumpsPct, x.meanErrorM, x.steps]);
        }
    }
    // After the tab is shown again: the first three seconds, and the largest
    // step any aircraft took in them.
    const shown = data.events.find(([, n]) => n === 'shown' || n === 'visibility-visible');
    if (scenario === 'e' && shown) {
        const t = shown[0];
        const x = motion(per, (e) => e.t >= t && e.t < t + 3000);
        let maxStep = 0;
        let maxStepHex = null;
        for (const acc of per.values()) {
            for (const s of acc.steps) if (s.t >= t && s.t < t + 3000 && s.m > maxStep) { maxStep = s.m; maxStepHex = acc.hex; }
        }
        const before = motion(per, (e) => e.t >= t - 33000 && e.t < t - 30000);
        extra.afterShown = { ...x, maxStepM: r1(maxStep), maxStepHex, errorBeforeHideM: before.meanErrorM };
    }
    // The aircraft back from its long gap: its motion from the fix after the
    // gap to 12 s later, its backward travel and its error.
    if (scenario === 'f') {
        for (const acc of per.values()) {
            if (acc.kind !== 'longgap' || !acc.gap || !acc.gap[1]) continue;
            const [g0, g1] = acc.gap;
            const x = motion(new Map([[acc.hex, acc]]), (e) => e.t >= g1 && e.t < g1 + 12000);
            let backM = 0, run = 0;
            for (const s of acc.steps) {
                if (s.t < g0 || s.t > g1 + 12000) continue;
                if (s.along < 0) { run += -s.along; backM = Math.max(backM, run); } else run = 0;
            }
            const errs = acc.errors.filter((e) => e.t >= g0 && e.t < g1 + 12000).map((e) => e.m);
            extra.longGap = { hex: acc.hex, ...x, maxBackwardRunM: r1(backM), maxErrorM: r1(Math.max(0, ...errs)) };
        }
    }
    return { summary, extra };
}

export function formatTable(results) {
    const cols = [
        ['backwardsPct', 'reculs %'], ['slowPct', 'lentes %'], ['jumpsPct', 'sauts %'], ['speedStdPct', 'é.-t. vitesse %'], ['speedStdEnginePct', 'é.-t. moteur %'],
        ['maxCorrectionM', 'corr. max m'], ['meanErrorM', 'err. moy. m'], ['p95ErrorM', 'err. p95 m'],
        ['moveMedMs', 'pose méd. ms'], ['moveP95Ms', 'pose p95 ms'], ['frameMedMs', 'image méd. ms'], ['frameP95Ms', 'image p95 ms'], ['framesOver50Pct', '> 50 ms %'],
        ['longTasksPer10s', 'tâches longues /10 s'], ['glyphJumpsPct', 'glyphe > 3° %'],
    ];
    const lines = [`| scénario | ${cols.map((c) => c[1]).join(' | ')} |`, `|---|${cols.map(() => '---').join('|')}|`];
    for (const r of results) lines.push(`| ${r.id} ${r.label} | ${cols.map(([k]) => r.metrics.summary[k]).join(' | ')} |`);
    return lines.join('\n');
}
