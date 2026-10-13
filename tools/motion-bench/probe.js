// The bench's probe, run in the page before any of its scripts
// (Page.addScriptToEvaluateOnNewDocument, tools/motion-bench/bench.mjs).
//
// What the screen shows, not what the engine computes: after each frame the
// map draws (OpenLayers' postrender), the coordinates and rotation actually
// set on each aircraft's feature; every animation frame's time; the long
// tasks (> 50 ms) of the main thread; the corrections the engine starts.
// Times are wall clock, ms (performance.timeOrigin + performance.now()), the
// generator's clock on the same machine. The bench drains __motionProbe.drain()
// every few seconds; nothing here talks to the server.
(function () {
    const wall = () => performance.timeOrigin + performance.now();
    const probe = {
        hexes: [], // index -> hex
        hexIndex: new Map(),
        poses: [], // flat: t, hexIndex, lat, lon, rotationRad, fade, per aircraft per drawn frame
        renders: [], // flat: t of each drawn frame, and the time the engine computed its poses for
        frames: [], // t of each animation frame
        longTasks: [], // [start, duration]
        corrections: [], // [t, hexIndex, metres]
        relocations: [], // [t, hexIndex, metres]: gaps the engine relocates instead of gliding
        events: [], // [t, name]
        attached: false,
        recording: false,
    };
    window.__motionProbe = probe;

    function hexId(hex) {
        let i = probe.hexIndex.get(hex);
        if (i === undefined) {
            i = probe.hexes.length;
            probe.hexes.push(hex);
            probe.hexIndex.set(hex, i);
        }
        return i;
    }

    try {
        new PerformanceObserver((list) => {
            for (const e of list.getEntries()) {
                if (probe.recording) probe.longTasks.push([performance.timeOrigin + e.startTime, e.duration]);
            }
        }).observe({ type: 'longtask', buffered: true });
    } catch (e) { probe.events.push([wall(), 'no-longtask-observer']); }

    function onFrame() {
        if (probe.recording) probe.frames.push(wall());
        requestAnimationFrame(onFrame);
    }
    requestAnimationFrame(onFrame);

    document.addEventListener('visibilitychange', () => probe.events.push([wall(), 'visibility-' + document.visibilityState]));

    const seenCorrections = new WeakSet();
    function noteCorrections(engine, t) {
        if (!engine || !engine.aircraftStates) return;
        for (const [hex, state] of engine.aircraftStates) {
            const r = state.relocation;
            if (r && !seenCorrections.has(r)) {
                seenCorrections.add(r);
                if (probe.recording) probe.relocations.push([t, hexId(hex), Math.round(r.gapM * 10) / 10]);
            }
            const c = state.correction;
            if (c && !seenCorrections.has(c)) {
                seenCorrections.add(c);
                const lat = state.lastReal ? state.lastReal.lat : 45;
                const m = Math.hypot(c.dLat * 111320, c.dLon * 111320 * Math.cos(lat * Math.PI / 180));
                if (probe.recording) probe.corrections.push([t, hexId(hex), Math.round(m * 10) / 10]);
            }
        }
    }

    function attach() {
        const store = window.Alpine && window.Alpine.store && window.Alpine.store('atc');
        const mm = store && store.mapManager;
        const map = mm && mm._olMap;
        const renderer = mm && mm._aircraftRenderer;
        if (!map || !renderer || typeof renderer.getFeature !== 'function') {
            setTimeout(attach, 200);
            return;
        }
        probe.attached = true;
        probe.backend = renderer.getStats ? renderer.getStats().backend : 'unknown';
        probe.events.push([wall(), 'attached-' + probe.backend]);
        map.on('postrender', () => {
            const t = wall();
            noteCorrections(store.animationEngine, t);
            if (!probe.recording) return;
            const engine = store.animationEngine;
            probe.renders.push(t, engine && engine.lastFrameAt ? performance.timeOrigin + engine.lastFrameAt : NaN);
            for (const hex of renderer.getFeatureHexes()) {
                const f = renderer.getFeature(hex);
                const g = f && f.getGeometry();
                if (!g) continue;
                const [lon, lat] = window.ol.proj.toLonLat(g.getCoordinates());
                const rot = Number(f.get('rotationRad'));
                const fade = Number(f.get('fade'));
                probe.poses.push(t, hexId(hex), lat, lon, Number.isFinite(rot) ? rot : NaN, Number.isFinite(fade) ? fade : 1);
            }
        });
    }
    if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', attach);
    else attach();

    probe.start = () => { probe.recording = true; probe.events.push([wall(), 'start']); };
    probe.stop = () => { probe.recording = false; probe.events.push([wall(), 'stop']); };
    probe.mark = (name) => probe.events.push([wall(), name]);
    // Everything recorded since the last drain, then forgotten.
    probe.drain = () => {
        const out = {
            hexes: probe.hexes.slice(), poses: probe.poses, renders: probe.renders, frames: probe.frames,
            longTasks: probe.longTasks, corrections: probe.corrections, relocations: probe.relocations, events: probe.events,
            attached: probe.attached, backend: probe.backend, poseStride: 6,
        };
        probe.poses = []; probe.renders = []; probe.frames = []; probe.longTasks = [];
        probe.corrections = []; probe.relocations = []; probe.events = [];
        return JSON.stringify(out);
    };
})();
