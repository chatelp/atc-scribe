// node --test www/aircraft-animation.test.js
// The animation engine against a receiver that repeats a fix: a poll without
// a new position must not be taken for a fresh observation (13/10/2026).
const test = require('node:test');
const assert = require('node:assert/strict');
const AircraftAnimationEngine = require('./aircraft-animation.js');

function engine() {
    const poses = [];
    const mapManager = { updateAnimatedAircraftPose(hex, pose) { poses.push({ hex, ...pose }); return true; } };
    return { e: new AircraftAnimationEngine(mapManager, { aircraft: {} }), poses };
}

// ~205 m east per second at 400 kt on latitude 48.5.
const sample = (lat, lon, extra = {}) => ({
    hex: 'abc123', status: 'active',
    adsb: { lat, lon, gs: 400, track: 90, true_heading: 95, alt_baro: 30000, baro_rate: 0, source: 'real', ...extra },
});

test('a real update that repeats the known fix keeps the fix as dated', () => {
    const { e } = engine();
    e.updateAircraft(sample(48.5, 2.0));
    e.updateAircraft(sample(48.5, 2.0028));
    const state = e.aircraftStates.get('abc123');
    const dated = state.lastReal.timestamp;
    // Computing a pose past the correction's end settles it (null from then on).
    const before = e._computePose(state, performance.now() + 1500);
    const correction = state.correction;

    e.updateAircraft(sample(48.5, 2.0028, { gs: 402, alt_baro: 30025 }));

    assert.equal(state.lastReal.timestamp, dated, 'the fix is not re-dated');
    assert.equal(state.correction, correction, 'no new correction back to the fix');
    const after = e._computePose(state, performance.now() + 1500);
    assert.ok(after.lon >= before.lon - 1e-9, 'the pose keeps moving east');
    assert.ok(Math.abs(state.velocity.eastMps - 402 * 0.514444) < 30, 'the velocity follows the new speed');
});

test('a new fix is sampled as before', () => {
    const { e } = engine();
    e.updateAircraft(sample(48.5, 2.0));
    e.updateAircraft(sample(48.5, 2.0028));
    const state = e.aircraftStates.get('abc123');
    const dated = state.lastReal.timestamp;
    e.updateAircraft(sample(48.5, 2.0056));
    assert.ok(state.lastReal.timestamp >= dated);
    assert.deepEqual(state.lastRealFix, { lat: 48.5, lon: 2.0056 });
    assert.ok(state.correction, 'a correction eases toward the new fix');
});

test('a correction neither stops the aircraft nor makes it jump', () => {
    const { e } = engine();
    e.updateAircraft(sample(48.5, 2.0));
    e.updateAircraft(sample(48.5, 2.0028));
    const state = e.aircraftStates.get('abc123');
    // The aircraft was drawn 150 m behind the new fix: a correction starts.
    state.lastRendered = { lat: 48.5, lon: 2.0008, alt: 30000, track: 90, timestamp: performance.now() };
    e.updateAircraft(sample(48.5, 2.0056));
    assert.ok(state.correction, 'a correction is in progress');

    const t0 = performance.now();
    const steps = [];
    let prev = null;
    // 150 m at 205 m/s close over about 2.2 s (three times the gap over the speed).
    for (let ms = 0; ms <= 3000; ms += 33) {
        const p = e._computePose(state, t0 + ms);
        if (prev) steps.push((p.lon - prev.lon) * 111320 * Math.cos(48.5 * Math.PI / 180));
        prev = p;
    }
    const sorted = steps.slice().sort((a, b) => a - b);
    const median = sorted[Math.floor(sorted.length / 2)];
    assert.ok(median > 0, 'the aircraft moves east');
    assert.ok(Math.min(...steps) > median * 0.3, `no frame nearly stops: min ${Math.min(...steps).toFixed(2)} m, median ${median.toFixed(2)} m`);
    assert.ok(Math.max(...steps) < median * 3, `no frame jumps: max ${Math.max(...steps).toFixed(2)} m, median ${median.toFixed(2)} m`);
    assert.equal(state.correction, null, 'the correction has faded out');
});

test('a predicted sample never counts as the known fix', () => {
    const { e } = engine();
    e.updateAircraft(sample(48.5, 2.0));
    e.updateAircraft(sample(48.5, 2.0028));
    const state = e.aircraftStates.get('abc123');
    // The server's predicted state reaches the engine through estimate(),
    // never through the aircraft's adsb (fiche L, step 4).
    e.estimate('abc123', { lat: 48.5, lon: 2.0040, alt: 30000, gs: 400, track: 90, confidence: 0.9 });
    assert.deepEqual(state.lastRealFix, { lat: 48.5, lon: 2.0028 });
    assert.equal(state.lastSampleSource, 'predicted');
});

// Fiche L, step 5: samples dated by their source.
const metresEast = (lon, lon0) => (lon - lon0) * 111320 * Math.cos(48.5 * Math.PI / 180);
const fix = (lon, at, sentAt = Date.now()) => ({ lat: 48.5, lon, gs: 400, track: 90, heading: 95, alt: 30000, baroRate: 0, at, sentAt });

test('an observation is carried from when its position was decoded, not from its arrival', () => {
    const { e } = engine();
    const now = Date.now();
    e.observe('abc123', fix(2.0, now - 2000));
    e.observe('abc123', fix(2.0028, now - 1000));
    const state = e.aircraftStates.get('abc123');
    const pose = e._predictedPose(state, state.lastReal, performance.now(), 90);
    // Decoded a second ago at ~205 m/s: drawn ~205 m on from the fix now.
    const ahead = metresEast(pose.lon, 2.0028);
    assert.ok(Math.abs(ahead - 400 * 0.514444) < 25, `carried ${ahead.toFixed(0)} m, want about 206 m`);
});

test('a server clock ahead of the page does not freeze the aircraft', () => {
    const { e } = engine();
    const server = Date.now() + 5000; // the server's clock reads 5 s more
    e.observe('abc123', fix(2.0, server - 1000, server));
    const state = e.aircraftStates.get('abc123');
    const age = performance.now() - state.lastReal.timestamp;
    assert.ok(Math.abs(age - 1000) < 50, `the sample is ${age.toFixed(0)} ms old on the page, want about 1000`);
});

test('an estimate based on an older poll than the last observation is left out', () => {
    const { e } = engine();
    const now = Date.now();
    e.observe('abc123', fix(2.0, now - 1500, now - 1000));
    e.observe('abc123', fix(2.0028, now - 500, now));
    const state = e.aircraftStates.get('abc123');
    e.estimate('abc123', { lat: 48.5, lon: 2.0035, gs: 400, track: 90, alt: 30000, confidence: 0.9, at: now, sentAt: now, basedOn: now - 1000 });
    assert.equal(state.lastSampleSource, 'real', 'the observation stands');
    e.estimate('abc123', { lat: 48.5, lon: 2.0035, gs: 400, track: 90, alt: 30000, confidence: 0.5, at: now, sentAt: now, basedOn: now });
    assert.equal(state.lastSampleSource, 'real', 'under the confidence floor');
    e.estimate('abc123', { lat: 48.5, lon: 2.0035, gs: 400, track: 90, alt: 30000, confidence: 0.9, at: now, sentAt: now, basedOn: now });
    assert.equal(state.lastSampleSource, 'predicted', 'a current estimate is taken');
});

test("an aircraft's sample is dated as the server dates its position", () => {
    const { e } = engine();
    const observedAt = Date.parse('2026-10-13T12:00:00Z');
    const s = e.sampleFromAircraft({ hex: 'abc123', adsb: { lat: 48.5, lon: 2.0, gs: 400, track: 90, seen: 1.4, seen_pos: 2.3 } }, observedAt);
    // observed_at is the poll less the whole seconds of seen.
    assert.equal(s.sentAt, observedAt + 1000);
    assert.equal(s.at, observedAt + 1000 - 2300);
    assert.equal(e.sampleFromAircraft({ hex: 'abc123', adsb: { lat: 0, lon: 0 } }), null, 'no position, no sample');
});
