// node --test 'www/par/*.test.js'
const test = require('node:test');
const assert = require('node:assert/strict');
const G = require('./par-geometry.js');

// As /api/v1/airports/{code} serves them, values from assets/runways.csv.
const ORLY_06_24 = {
    closed: false,
    le_ident: '06', le_latitude: 48.720001220703125, le_longitude: 2.316920042037964, le_elevation_ft: 283, le_displaced_ft: 984,
    he_ident: '24', he_latitude: 48.73550033569336, he_longitude: 2.360680103302002, he_elevation_ft: 284, he_displaced_ft: 0,
};
const CDG_08L_26R = {
    closed: false,
    le_ident: '08L', le_latitude: 48.99570083618164, le_longitude: 2.5527400970458984, le_elevation_ft: 338, le_displaced_ft: 0,
    he_ident: '26R', he_latitude: 48.99879837036133, he_longitude: 2.610179901123047, he_elevation_ft: 318, he_displaced_ft: 1969,
};
const CDG_08R_26L = {
    closed: false,
    le_ident: '08R', le_latitude: 48.99290084838867, le_longitude: 2.565659999847412, le_elevation_ft: 336, le_displaced_ft: 0,
    he_ident: '26L', he_latitude: 48.99489974975586, he_longitude: 2.6024301052093506, he_elevation_ft: 316, he_displaced_ft: 0,
};

// The position at axis coordinates (s, c) of a frame, the inverse of project().
function at(frame, s, c) {
    const b = frame.bearing * Math.PI / 180;
    const x = s * Math.sin(b) + c * Math.cos(b);
    const y = s * Math.cos(b) - c * Math.sin(b);
    return {
        lat: frame.le.lat + y / 60,
        lon: frame.le.lon + x / (60 * Math.cos(frame.le.lat * Math.PI / 180)),
    };
}

const orly = () => G.buildFrames('LFPO', [ORLY_06_24])[0];

test('a closed runway, or one with an end not located, has no frame', () => {
    const closed = Object.assign({}, ORLY_06_24, { closed: true });
    const unlocated = Object.assign({}, ORLY_06_24, { he_latitude: 0, he_longitude: 0 });
    assert.equal(G.buildFrames('LFPO', [closed, unlocated]).length, 0);
    assert.equal(G.buildFrames('LFPO', [ORLY_06_24]).length, 1);
});

test('a frame runs from the low end to the high end, the length of the runway', () => {
    const f = orly();
    assert.equal(f.id, '06-24');
    // 3650 m in the eAIP: 1.97 NM.
    assert.ok(Math.abs(f.lengthNM - 1.97) < 0.03, f.lengthNM);
    assert.ok(Math.abs(f.bearing - 62) < 1, f.bearing);
    const p = G.project(f, f.he.lat, f.he.lon);
    assert.ok(Math.abs(p.s - f.lengthNM) < 1e-9 && Math.abs(p.c) < 1e-9);
});

test('the displaced threshold moves the glide path in, and it is 3 degrees', () => {
    const f = orly();
    assert.ok(Math.abs(f.le.thresholdS - 984 / G.FT_PER_NM) < 1e-9);
    // At the threshold, the crossing height; 5 NM out, some 1600 ft higher.
    assert.equal(G.glidePathFt(f, 'le', f.le.thresholdS), 283 + 50);
    const fiveOut = G.glidePathFt(f, 'le', f.le.thresholdS - 5);
    assert.ok(Math.abs(fiveOut - (333 + 5 * G.FT_PER_NM * Math.tan(3 * Math.PI / 180))) < 1e-6);
    assert.equal(G.glidePathFt(f, 'le', f.le.thresholdS + 0.1), null);
});

test('a per-runway slope replaces the default', () => {
    const f = G.buildFrames('LFPO', [ORLY_06_24], {}, { 'LFPO 24': { slopeDeg: 3.4 } })[0];
    assert.equal(f.he.slopeDeg, 3.4);
    assert.equal(f.le.slopeDeg, 3);
});

test('an arrival on the glide path is in the cone of the end it lands on', () => {
    const f = orly();
    const s = f.lengthNM + 8;
    const alt = G.glidePathFt(f, 'he', s);
    const p = G.placeInFrame(f, Object.assign(at(f, s, 0), { altFt: alt, track: 242 }));
    assert.equal(p.side, 'he');
    assert.ok(Math.abs(p.dist - 8) < 0.01);
    assert.equal(p.movingAway, false);
    assert.ok(Math.abs(p.elevationDeg - 3) < 0.1, p.elevationDeg);
});

test('a departure climbing at 10 degrees stays in; the same point flown inbound does not', () => {
    const f = orly();
    // Departing 24: it leaves over the 06 end, flying 242.
    const pos = at(f, -3, 0);
    const alt = 283 + 3 * G.FT_PER_NM * Math.tan(10 * Math.PI / 180);
    const out = G.placeInFrame(f, Object.assign({}, pos, { altFt: alt, track: 242 }));
    assert.equal(out.side, 'le');
    assert.equal(out.movingAway, true);
    assert.equal(G.placeInFrame(f, Object.assign({}, pos, { altFt: alt, track: 62 })), null);
});

test('past the range, the half-width or the ceiling, an aircraft is out', () => {
    const f = orly();
    const inCone = { altFt: 4000, track: 242 };
    assert.ok(G.placeInFrame(f, Object.assign(at(f, f.lengthNM + 15, 0), inCone)));
    assert.equal(G.placeInFrame(f, Object.assign(at(f, f.lengthNM + 21, 0), inCone)), null);
    // 25 degrees off the axis.
    assert.equal(G.placeInFrame(f, Object.assign(at(f, f.lengthNM + 10, 10 * Math.tan(25 * Math.PI / 180)), inCone)), null);
    // Moving away at 20 NM, 15 degrees would allow 32 000 ft; the ceiling does not.
    assert.equal(G.placeInFrame(f, Object.assign(at(f, -20, 0), { altFt: 16000, track: 242 })), null);
});

test('an aircraft crossing the axis far out is passing through, not flying the runway', () => {
    const f = orly();
    // 15 NM past the 24 end, near the axis, low: aligned it is shown; crossing
    // at 68 degrees, as De Gaulle's finals cross Orly's 02-20 axis, it is not.
    const pos = at(f, f.lengthNM + 15, 0.3);
    assert.ok(G.placeInFrame(f, Object.assign({}, pos, { altFt: 1400, track: 242 })));
    assert.equal(G.placeInFrame(f, Object.assign({}, pos, { altFt: 1400, track: 242 + 68 })), null);
    // Within 2 NM of the runway, turning onto final or away after take-off.
    assert.ok(G.placeInFrame(f, Object.assign(at(f, f.lengthNM + 1.5, 0.2), { altFt: 800, track: 242 + 80 })));
});

test('an aircraft flying beside the runway, never onto it, is not on it', () => {
    const f = orly();
    // Parallel to the axis, 1.25 NM to the side, 4 NM out, heading for the
    // runway: it will pass abeam, as Le Bourget's departures pass De Gaulle.
    assert.equal(G.placeInFrame(f, Object.assign(at(f, -4, 1.25), { altFt: 1800, track: f.bearing })), null);
    // Converging on the axis at 20 degrees from 1.25 NM out, 8 NM out: an
    // intercept, it joins the axis well before the runway.
    assert.ok(G.placeInFrame(f, Object.assign(at(f, -8, 1.25), { altFt: 3000, track: f.bearing - 20 })));
    // A departure that turned 15 degrees after take-off, 5 NM out: behind it,
    // its track leads back to the runway end.
    const turned = at(f, -5, -5 * Math.tan(15 * Math.PI / 180) * 0.9);
    assert.ok(G.placeInFrame(f, Object.assign(turned, { altFt: 4500, track: f.bearing + 180 + 15 })));
});

test('on the ground, only the runway itself counts, not the taxiway beside it', () => {
    const f = orly();
    assert.equal(G.placeInFrame(f, Object.assign(at(f, 1, 0.01), { altFt: 283, onGround: true })).side, 'rwy');
    assert.equal(G.placeInFrame(f, Object.assign(at(f, 1, 0.1), { altFt: 283, onGround: true })), null);
    assert.equal(G.placeInFrame(f, Object.assign(at(f, -2, 0), { altFt: 283, onGround: true })), null);
});

test('between two parallel runways the nearest axis wins, and a close call keeps the previous one', () => {
    const frames = G.buildFrames('LFPG', [CDG_08L_26R, CDG_08R_26L]);
    const r = frames.find((f) => f.id === '08R-26L');
    const l = frames.find((f) => f.id === '08L-26R');
    // On the 08R extended axis, 6 NM out: nearer 08R.
    const onR = Object.assign(at(r, -6, 0), { altFt: 2200, track: 82 });
    assert.equal(G.classify(frames, onR).frame, '08R-26L');
    // Just off midway between the axes, 0.02 NM nearer 08L's: 08L, unless the
    // aircraft was already in 08R's frame.
    const cl = G.project(r, l.le.lat, l.le.lon).c;   // where 08L's axis lies in 08R's frame
    const mid = Object.assign(at(r, -6, cl / 2 + Math.sign(cl) * 0.01), { altFt: 2200, track: 82 });
    assert.equal(G.classify(frames, mid).frame, '08L-26R');
    assert.equal(G.classify(frames, mid, {}, '08R-26L').frame, '08R-26L');
});

test('an aircraft established on one parallel does not stay in the other one\'s frame', () => {
    const frames = G.buildFrames('LFPG', [CDG_08L_26R, CDG_08R_26L]);
    const l = frames.find((f) => f.id === '08L-26R');
    // On the 08L axis, 4 NM out, having crossed 08R's axis on the way.
    const onL = Object.assign(at(l, -4, 0), { altFt: 1600, track: 85 });
    assert.equal(G.classify(frames, onL, {}, '08R-26L').frame, '08L-26R');
});

test('a departure stays with the runway it took off from', () => {
    const frames = G.buildFrames('LFPG', [CDG_08L_26R, CDG_08R_26L]);
    const r = frames.find((f) => f.id === '08R-26L');
    const l = frames.find((f) => f.id === '08L-26R');
    // Climbing out of 08R past the 26L end, drifted most of the way to 08L's axis.
    const cl = G.project(r, l.le.lat, l.le.lon).c;
    const drifted = Object.assign(at(r, r.lengthNM + 5, cl * 0.7), { altFt: 4000, track: 85 });
    assert.equal(G.classify(frames, drifted).frame, '08L-26R');
    assert.equal(G.classify(frames, drifted, {}, '08R-26L').frame, '08R-26L');
});

test('QNH altitude and the transition level follow the pressure', () => {
    assert.equal(G.qnhAltitude(5000, G.STD_HPA), 5000);
    assert.ok(Math.abs(G.qnhAltitude(5000, 1023.25) - 5273) < 0.01);
    assert.equal(G.transitionLevel(5000, 1015), 60);
    assert.equal(G.transitionLevel(5000, 1013.25), 60);
    // FL60 is at about 5640 ft with 1000 hPa, less than 1000 ft above 5000.
    assert.equal(G.transitionLevel(5000, 1000), 70);
});

test('the QNH is the median of what the low aircraft near the airport have set', () => {
    const ac = (alt, qnh, lat = 48.73, lon = 2.37, onGround = false) =>
        ({ on_ground: onGround, adsb: { alt_baro: alt, nav_qnh: qnh, lat, lon } });
    const list = [
        ac(3000, 1015.2), ac(2000, 1016), ac(4000, 1014.4),
        ac(12000, 1013.2),              // above the transition: standard setting
        ac(0, 1013.2, 48.73, 2.37, true),
        ac(3000, 1030, 50.5, 2.37),     // 100 NM away
    ];
    assert.deepEqual(G.estimateQnh(list, 48.72, 2.36), { qnh: 1015.2, count: 3 });
    assert.equal(G.estimateQnh([ac(12000, 1013.2)], 48.72, 2.36), null);
});
