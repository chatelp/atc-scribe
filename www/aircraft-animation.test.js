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

test('a predicted sample never counts as the known fix', () => {
    const { e } = engine();
    e.updateAircraft(sample(48.5, 2.0));
    e.updateAircraft(sample(48.5, 2.0028));
    const state = e.aircraftStates.get('abc123');
    e.updateAircraft(sample(48.5, 2.0040, { source: 'predicted' }));
    assert.deepEqual(state.lastRealFix, { lat: 48.5, lon: 2.0028 });
    assert.equal(state.lastSampleSource, 'predicted');
});
