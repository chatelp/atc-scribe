// node --test 'www/par/*.test.js'
const test = require('node:test');
const assert = require('node:assert/strict');
const G = require('./par-geometry.js');
const A = require('./par-arrivals.js');

// As /api/v1/airports/{code} serves them, values from assets/runways.csv.
const ORLY_06_24 = {
    closed: false,
    le_ident: '06', le_latitude: 48.720001220703125, le_longitude: 2.316920042037964, le_elevation_ft: 283, le_displaced_ft: 984,
    he_ident: '24', he_latitude: 48.73550033569336, he_longitude: 2.360680103302002, he_elevation_ft: 284, he_displaced_ft: 0,
};
const ORLY = { code: 'LFPO', latitude: 48.7262, longitude: 2.3652, runway_in_use: [{ runway_end: '06', probability: 0.8, score: 4 }] };

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

// An aircraft on final to 06, distNM before the threshold, on the glide path.
function onFinal(frame, hex, flight, distNM, gs) {
    const s = frame.le.thresholdS - distNM;
    const pos = at(frame, s, 0);
    const altFt = G.glidePathFt(frame, 'le', s) + 50;
    return {
        hex, flight, last_seen: new Date().toISOString(),
        adsb: { lat: pos.lat, lon: pos.lon, alt_baro: altFt, track: frame.le.landingBearing, gs, t: 'A320' },
    };
}

test('the sequence lists the aircraft on final, nearest first, with gaps and the tight flag', () => {
    const frame = G.buildFrames('LFPO', [ORLY_06_24])[0];
    const store = { aircraft: {}, settings: {} };
    for (const a of [onFinal(frame, 'a1', 'AFR123', 7.5, 140), onFinal(frame, 'a2', 'EZY45', 4, 140), onFinal(frame, 'a3', 'RYR7', 5.5, 140)]) {
        store.aircraft[a.hex] = a;
    }
    const state = {};
    const placement = A.place(G, store, ORLY, [ORLY_06_24], state);
    assert.ok(placement, 'a placement');
    assert.equal(placement.inUse.length, 1);
    assert.equal(placement.inUse[0].end, '06');

    const seqs = A.sequences(G, placement);
    const s06 = seqs.find((s) => s.runway === '06');
    assert.ok(s06 && s06.inUse, 'the end in use is listed');
    assert.deepEqual(s06.items.map((i) => i.callsign), ['EZY45', 'RYR7', 'AFR123']);
    assert.ok(Math.abs(s06.items[0].distNM - 4) < 0.05, s06.items[0].distNM);
    assert.equal(s06.items[0].gapNM, null);
    assert.ok(Math.abs(s06.items[1].gapNM - 1.5) < 0.05 && s06.items[1].tight, 'RYR7 is 1.5 NM behind: tight');
    assert.ok(Math.abs(s06.items[2].gapNM - 2) < 0.05 && s06.items[2].tight, 'AFR123 is 2 NM behind: tight');
    // 4 NM at 140 kt: 103 s.
    assert.ok(Math.abs(s06.items[0].etaSec - 4 / 140 * 3600) < 1, s06.items[0].etaSec);
    assert.ok(s06.items[0].eta instanceof Date);

    const next = A.nextTouchdown(seqs);
    assert.equal(next.callsign, 'EZY45');
    assert.equal(next.runway, '06');
    // The hysteresis remembers the frame of each aircraft placed.
    assert.equal(state.previousFrame.get('a2'), frame.id);
});

test('an end in use with nobody on final is still listed, empty; an unused end without arrivals is not', () => {
    const store = { aircraft: {}, settings: {} };
    const seqs = A.sequences(G, A.place(G, store, ORLY, [ORLY_06_24], {}));
    assert.deepEqual(seqs.map((s) => [s.runway, s.inUse, s.items.length]), [['06', true, 0]]);
    assert.equal(A.nextTouchdown(seqs), null);
});

test('an aircraft the server gives to another airport is not placed here', () => {
    const frame = G.buildFrames('LFPO', [ORLY_06_24])[0];
    const a = onFinal(frame, 'b1', 'AFR9', 4, 140);
    a.runway = { arrival: { airport: 'LFPG', runway: '26L' } };
    const store = { aircraft: { b1: a }, settings: {} };
    const seqs = A.sequences(G, A.place(G, store, ORLY, [ORLY_06_24], {}));
    assert.equal(seqs.find((s) => s.runway === '06').items.length, 0);
});
