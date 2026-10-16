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

test('the first sample flies at its own speed (fiche M, scenario a)', () => {
    const { e } = engine();
    const now = Date.now();
    e.observe('abc123', fix(2.0, now - 1000, now));
    const state = e.aircraftStates.get('abc123');
    assert.ok(state.velocity, 'a velocity from the first sample');
    const a = e._computePose(state, performance.now());
    const b = e._computePose(state, performance.now() + 1000);
    const metres = (b.lon - a.lon) * 111320 * Math.cos(48.5 * Math.PI / 180);
    assert.ok(Math.abs(metres - 400 * 0.514444) < 5, `${metres.toFixed(1)} m in a second, want about 206`);
});

test("the bulk load's aircraft is dated by its last_seen, as the stream's are", () => {
    const { e } = engine();
    const lastSeen = '2026-10-13T12:00:00Z';
    const aircraft = { hex: 'abc123', last_seen: lastSeen, adsb: { lat: 48.5, lon: 2.0, gs: 400, track: 90, seen: 0.4, seen_pos: 1.7 } };
    const s = e.sampleFromAircraft(aircraft, Date.parse(aircraft.last_seen));
    assert.equal(s.at, Date.parse(lastSeen) - 1700);
});

test('after a pause of the frames, the aircraft goes on from where its samples put it (fiche M, scenario e)', () => {
    const { e, poses } = engine();
    const now = Date.now();
    e.observe('abc123', fix(2.0, now - 1000, now));
    e.observe('abc123', fix(2.0028, now - 500, now));
    const state = e.aircraftStates.get('abc123');
    // Drawn 30 s ago, far behind, with a gap fading from there.
    const t = performance.now();
    state.lastRendered = { lat: 48.5, lon: 1.9, alt: 30000, track: 90, timestamp: t - 30000 };
    state.correction = { dLat: 0, dLon: -0.1, dAlt: 0, startAt: t, endAt: t + 2500 };
    globalThis.requestAnimationFrame = () => 0;
    e.isRunning = true;
    e.lastFrameAt = t - 30000;
    e.rafLoop(t);
    assert.equal(state.correction, null, 'no sweep from the pose drawn before the pause');
    const drawn = poses[poses.length - 1];
    const fresh = e._predictedPose(state, state.lastReal, t, null);
    assert.ok(Math.abs(drawn.lon - fresh.lon) < 1e-6, 'drawn where the samples put it now');
});

test('a gap the glide cannot close without outrunning the aircraft relocates it (fiche M, scenario f)', () => {
    // At 400 kt (206 m/s) the capped glide (2.5 s) outruns the aircraft
    // backwards past 1.67 s of flight (343 m), at three times its speed past
    // 3.33 s (686 m).
    const at = (drawnLon) => {
        const { e, poses } = engine();
        const now = Date.now();
        e.observe('abc123', fix(2.0, now, now));
        const state = e.aircraftStates.get('abc123');
        state.lastRendered = { lat: 48.5, lon: drawnLon, alt: 30000, track: 90, timestamp: performance.now() };
        e.observe('abc123', fix(2.0028, now, now));
        return { e, state, poses };
    };
    const metresPerDeg = 111320 * Math.cos(48.5 * Math.PI / 180);
    // Drawn 300 m ahead: glided (no relocation), never backwards.
    let { e, state } = at(2.0028 + 300 / metresPerDeg);
    assert.ok(state.correction && !state.relocation, 'glided');
    // Drawn 400 m ahead: relocated, and never drawn moving west.
    ({ e, state } = at(2.0028 + 400 / metresPerDeg));
    assert.ok(state.relocation && !state.correction, 'relocated');
    const t0 = performance.now();
    let prev = null;
    for (let ms = 0; ms <= 400; ms += 33) {
        const p = e._computePose(state, t0 + ms);
        if (prev && p.fade >= 1 && prev.fade >= 1) assert.ok(p.lon >= prev.lon, 'no step west once shown');
        if (ms < 150) assert.ok(p.fade < 1, 'fading out');
        prev = p;
    }
    assert.equal(state.relocation, null, 'done after two fades');
    // Drawn 600 m behind: glided; 800 m behind: relocated.
    ({ state } = at(2.0028 - 600 / metresPerDeg));
    assert.ok(state.correction && !state.relocation, '600 m behind: glided');
    ({ state } = at(2.0028 - 800 / metresPerDeg));
    assert.ok(state.relocation, '800 m behind: relocated');
});

test('strategy B draws the aircraft in the past, between two observed fixes (fiche M, step 3)', () => {
    const { e } = engine();
    e.updateConfig({ strategy: 'B', renderDelayMs: 1000 });
    const now = Date.now();
    e.observe('abc123', fix(2.0, now - 2000, now));
    e.observe('abc123', fix(2.0028, now - 1000, now));
    e.observe('abc123', fix(2.0056, now, now));
    const state = e.aircraftStates.get('abc123');
    e.estimate('abc123', { lat: 48.5, lon: 2.01, gs: 400, track: 90, alt: 30000, confidence: 0.9, at: now, sentAt: now, basedOn: now });
    assert.equal(state.lastSampleSource, 'real', 'estimates are left out');
    const p = e._pastPose(state, performance.now() - 500, null);
    // 1.5 s ago: half-way between the first two fixes.
    assert.ok(Math.abs(p.lon - 2.0014) < 0.0002, `drawn at ${p.lon.toFixed(5)}, want about 2.0014`);
    assert.equal(new AircraftAnimationEngine({}, {}).config.strategy, 'A', 'A stays the default');
});

test("a symbol just created starts where the engine draws its aircraft (fiche M, step 4)", () => {
    const { e } = engine();
    const now = Date.now();
    e.observe('abc123', fix(2.0, now - 2000, now));
    const p = e.currentPose('abc123');
    const metres = (p.lon - 2.0) * 111320 * Math.cos(48.5 * Math.PI / 180);
    assert.ok(metres > 350 && metres < 480, `${metres.toFixed(0)} m on from the fix decoded 2 s ago, want about 412`);
    assert.equal(e.currentPose('nobody'), null);
});

test('a silent aircraft fades out where it was drawn and in at its last observed position (14/10)', () => {
    const { e } = engine();
    e.updateAircraft(sample(48.5, 2.0, { alt_baro: 1600 }));
    e.updateAircraft(sample(48.5, 2.0028, { alt_baro: 1600 }));
    const state = e.aircraftStates.get('abc123');
    const at = state.lastFixAt;
    const ahead = e._computePose(state, at + 3000);
    assert.ok(ahead.lon > 2.0028 + 1e-5, 'predicted ahead of the fix');
    state.lastRendered = { lat: ahead.lat, lon: ahead.lon, alt: ahead.alt, timestamp: at + 3000 };
    const fadeMs = e.config.relocateFadeMs;
    const silent = at + e.config.holdAfterSilenceMs + 1;
    e._computePose(state, silent);   // the hold starts at its first frame
    const out = e._computePose(state, silent + fadeMs / 2);
    assert.ok(out.held && out.lon === ahead.lon && out.fade > 0 && out.fade < 1, 'fading out where it was drawn');
    const back = e._computePose(state, silent + fadeMs * 1.5);
    assert.equal(back.lon, 2.0028, 'fading in at the last fix, never between');
    assert.ok(back.fade > 0 && back.fade < 1);
    const later = e._computePose(state, silent + 60000);
    assert.equal(later.lon, 2.0028, 'it waits at the last fix');
    assert.equal(later.fade, 1);
    assert.ok(state.holdSettled);
});

test('a silent aircraft takes no estimate, and a new fix lets it go (14/10)', () => {
    const { e } = engine();
    e.updateAircraft(sample(48.5, 2.0, { alt_baro: 1600 }));
    e.updateAircraft(sample(48.5, 2.0028, { alt_baro: 1600 }));
    const state = e.aircraftStates.get('abc123');
    state.lastFixAt = performance.now() - e.config.holdAfterSilenceMs - 1000;
    e.estimate('abc123', { lat: 48.5, lon: 2.02, track: 90, gs: 400, confidence: 1 });
    assert.equal(state.lastReal.lon, 2.0028, 'the estimate is left out');
    e.updateAircraft(sample(48.5, 2.0056, { alt_baro: 1600 }));
    assert.equal(state.hold, null);
    assert.ok(!e._isSilent(state, performance.now()), 'flying again');
});

test('high up, a silence is flown ahead until the aircraft is declared lost (14/10)', () => {
    const { e } = engine();
    e.updateAircraft(sample(48.5, 2.0));
    e.updateAircraft(sample(48.5, 2.0028));
    const state = e.aircraftStates.get('abc123');
    const at = state.lastFixAt;
    assert.ok(!e._computePose(state, at + 20000).held, 'a 20 s hole at FL 300 is flown ahead');
    assert.ok(e._computePose(state, at + e.config.holdAfterSilenceHighMs + 1).held, 'held once a minute has passed');
    state.status = 'signal_lost';
    assert.ok(e._isSilent(state, at + 1000), 'and as soon as it is declared lost');
});

// The owner, 16/10: aircraft crossing the map at full speed, diagonally. Two
// fixes without a track, dated a few milliseconds apart, gave a velocity of
// hundreds of thousands of knots (the gap over a dt floored at 1 ms); blended
// at 35 % a sample, it flew the aircraft across the map for twenty seconds,
// and made every gap look glidable (measured on the real traffic: 18
// aircraft in 10 min, one drawn 4 388 NM away).
test('two fixes without a track a few ms apart do not make an aircraft fly at 100 000 kt (16/10)', () => {
    const { e } = engine();
    e.updateAircraft(sample(48.5, 2.0));
    e.updateAircraft(sample(48.5, 2.0028));
    const state = e.aircraftStates.get('abc123');
    const kts = () => Math.hypot(state.velocity.eastMps, state.velocity.northMps) / 0.514444;
    assert.ok(Math.abs(kts() - 400) < 30);
    // The next fix has no track, lies 2 km off and is dated as the last one.
    const at = Date.now();
    e.observe('abc123', { lat: 48.5, lon: 2.03, alt: 30000, gs: null, track: null, heading: null, at, sentAt: at });
    e.observe('abc123', { lat: 48.518, lon: 2.03, alt: 30000, gs: null, track: null, heading: null, at: at + 3, sentAt: at + 3 });
    assert.ok(kts() < 1000, `the velocity stays an aircraft's: ${Math.round(kts())} kt`);
});

test('a velocity no aircraft flies is never taken (16/10)', () => {
    const { e } = engine();
    e.updateAircraft(sample(48.5, 2.0));
    const state = e.aircraftStates.get('abc123');
    const before = state.velocity;
    state.velocity = e._blendVelocity(state.velocity, { eastMps: 50000, northMps: 0, verticalMps: 0, headingDeg: 90 });
    assert.deepEqual(state.velocity, before);
});
