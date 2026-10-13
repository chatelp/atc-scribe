'use strict';
// node --test www/story/
const test = require('node:test');
const assert = require('node:assert/strict');
const { describe: then, valueOf } = require('./then.js');
const ParGeometry = require('../par/par-geometry.js');

const NNBSP = ' ';
const T0 = Date.parse('2026-10-13T12:00:00Z');
const at = (sec) => new Date(T0 + sec * 1000).toISOString();

// Every text a case produced, for the two tests at the end.
const seen = [];
function run(instruction, track, options) {
    const r = then(Object.assign({ at: at(0) }, instruction), track, options);
    if (r) seen.push(r.text);
    return r;
}

// A track: one point every `step` seconds from `from`, fields by function of
// the second (or constants), as the API serves them.
function track({ from = 5, to = 360, step = 10, alt, hdg, gs, lat, lon, mag = false }) {
    const out = [];
    for (let s = from; s <= to; s += step) {
        const v = (f) => (typeof f === 'function' ? f(s) : f);
        const p = { timestamp: at(s) };
        if (alt !== undefined) p.altitude = v(alt);
        if (hdg !== undefined) p[mag ? 'mag_heading' : 'track'] = v(hdg);
        if (gs !== undefined) p.speed_gs = v(gs);
        if (lat !== undefined) p.lat = v(lat);
        if (lon !== undefined) p.lon = v(lon);
        out.push(p);
    }
    return out;
}

// From `a` to `b` ft at `rate` ft/min starting at second 0, then level.
const ramp = (a, b, rate) => (s) => {
    const d = (rate * s) / 60;
    return b > a ? Math.min(b, a + d) : Math.max(b, a - d);
};

const FL100 = { role: 'flight_level', text: 'Descend to FL100', value: '100', unit: 'FL' };

// ---------------------------------------------------------------- level

test('a level reached and held', () => {
    const r = run(FL100, track({ alt: ramp(12000, 10000, 1000) }));
    assert.equal(r.kind, 'level');
    assert.equal(r.text, 'level at FL 100, 2 min later');
    assert.equal(r.at, T0 + 115000);
});

test('an altitude on the QNH, reached', () => {
    // QNH 1023: 4 000 ft on the QNH is about 3 733 ft on the standard setting.
    const r = run({ role: 'altitude', text: 'Climb to 4000 ft', value: '4000', unit: 'ft' },
        track({ alt: ramp(2000, 3733, 1000) }), { qnh: 1023 });
    assert.equal(r.text, `level at 4${NNBSP}000 ft, 2 min later`);
});

test('the same altitude without a QNH is read on the standard setting', () => {
    // Without the QNH, 3 733 ft is 267 ft short of 4 000: not reached, level.
    const r = run({ role: 'altitude', text: 'Climb to 4000 ft', value: '4000', unit: 'ft' },
        track({ alt: ramp(2000, 3733, 1000) }));
    assert.equal(r.text, `at 3${NNBSP}700 ft, 6 min later`);
});

test('a level not reached at the end of the window: still on the way', () => {
    const r = run({ role: 'altitude', text: 'Descend to 3000 ft', value: '3000', unit: 'ft' },
        track({ alt: ramp(9000, 3000, 900) }), { qnh: 1013 });
    assert.equal(r.text, `descending through 3${NNBSP}700 ft`);
});

test('above the transition altitude the level is said as a flight level', () => {
    const r = run({ role: 'altitude', text: 'Descend to 3000 ft', value: '3000', unit: 'ft' },
        track({ alt: ramp(12000, 3000, 500) }));
    assert.equal(r.text, 'descending through FL 090');
});

test('no movement: the level it was at, without a wait', () => {
    const r = run(FL100, track({ alt: 11000 }));
    assert.equal(r.text, 'at FL 110, 6 min later');
});

test('a maintained level: already there', () => {
    const r = run({ role: 'flight_level', text: 'Maintain FL100', value: '100', unit: 'FL' }, track({ alt: 10025 }));
    assert.equal(r.text, 'at FL 100, 6 min later');
});

test('a level passed through without stopping', () => {
    const r = run({ role: 'flight_level', text: 'Climb to FL100', value: '100', unit: 'FL' },
        track({ alt: ramp(8000, 14000, 2000) }));
    assert.equal(r.text, 'climbing through FL 100, 1 min later');
    assert.equal(r.at, T0 + 60000);
});

test('a climb the other way is told as it is', () => {
    const r = run(FL100, track({ alt: ramp(11000, 16000, 400) }));
    assert.equal(r.text, 'climbing through FL 134');
});

test('the signal lost on the way: the delay says when it was last seen', () => {
    const r = run(FL100, track({ alt: ramp(14000, 10000, 1000), to: 90 }));
    assert.equal(r.text, 'descending through FL 126, 1 min later');
    assert.equal(r.at, T0 + 85000);
});

test('the value read from the text when the server did not send it', () => {
    const r = run({ role: 'flight_level', text: 'Descend to FL100' }, track({ alt: ramp(12000, 10000, 1000) }));
    assert.equal(r.text, 'level at FL 100, 2 min later');
});

test('ADS-B names are read as well (alt_baro, gs, epoch ms)', () => {
    const pts = track({ alt: ramp(12000, 10000, 1000) }).map((p) => ({ timestamp: Date.parse(p.timestamp), alt_baro: p.altitude }));
    assert.equal(run(FL100, pts).text, 'level at FL 100, 2 min later');
});

test('an aircraft that lands on the way down', () => {
    const r = run({ role: 'altitude', text: 'Descend to 3000 ft', value: '3000', unit: 'ft' },
        track({ alt: (s) => (s < 200 ? 1500 - s * 7 : 0) }));
    assert.equal(r.text, 'on the ground, 6 min later');
});

// ---------------------------------------------------------------- nothing to say

test('no point after the instruction: null', () => {
    assert.equal(run(FL100, []), null);
    assert.equal(run(FL100, track({ from: -120, to: -10, alt: 12000 })), null);
});

test('points after the window only: null', () => {
    assert.equal(run(FL100, track({ from: 400, to: 600, alt: 10000 })), null);
});

test('no value to read: null', () => {
    assert.equal(run({ role: 'flight_level', text: 'Descend' }, track({ alt: 10000 })), null);
    assert.equal(run({ role: 'heading', text: 'Turn left' }, track({ hdg: 270 })), null);
    assert.equal(run({ role: 'runway', text: 'Runway' }, track({ alt: 0 })), null);
});

test('what asks nothing: a report, a QNH, a squawk, an unknown role', () => {
    assert.equal(run({ role: 'flight_level', text: 'Passing FL100', value: '100', unit: 'FL' }, track({ alt: 9000 })), null);
    assert.equal(run({ role: 'altitude', text: 'Leaving 3000 ft', value: '3000', unit: 'ft' }, track({ alt: 4000 })), null);
    assert.equal(run({ role: 'qnh', text: 'QNH 1015', value: '1015', unit: 'hPa' }, track({ alt: 4000 })), null);
    assert.equal(run({ role: 'squawk', text: 'Squawk 7000', value: '7000', unit: 'code' }, track({ alt: 4000 })), null);
    assert.equal(run({ role: 'callsign', text: 'AFR123' }, track({ alt: 4000 })), null);
    assert.equal(then(null, []), null);
    assert.equal(then({ role: 'flight_level', text: 'Descend to FL100' }, track({ alt: 10000 })), null, 'no time');
});

test('a track with no altitude says nothing of a level', () => {
    assert.equal(run(FL100, track({ hdg: 270 })), null);
});

// ---------------------------------------------------------------- heading

const H270 = { role: 'heading', text: 'Turn left heading 270', value: '270', unit: 'deg' };

test('a heading turned to', () => {
    const r = run(H270, track({ hdg: (s) => Math.max(268, 330 - s), alt: 5000 }));
    assert.equal(r.kind, 'heading');
    assert.equal(r.text, 'turned to 268°');
});

test('a heading still turning at the end of the window', () => {
    const r = run(H270, track({ hdg: (s) => 360 - s / 5 }));
    assert.equal(r.text, 'heading 289°, still turning');
});

test('a heading kept: the heading it flew, without a wait', () => {
    const r = run(H270, track({ hdg: 254 }));
    assert.equal(r.text, 'heading 254°, 6 min later');
});

test('the magnetic heading is preferred to the ground track', () => {
    const pts = track({ hdg: 270, mag: true }).map((p) => Object.assign(p, { track: 262 }));
    assert.equal(run(H270, pts).text, 'turned to 270°');
});

test('a heading across north reads as one', () => {
    const r = run({ role: 'heading', text: 'Turn right heading 360', value: '360', unit: 'deg' },
        track({ hdg: (s) => (s % 20 === 5 ? 358 : 2) }));
    assert.equal(r.text, 'turned to 360°');
    const z = run({ role: 'heading', text: 'Heading 024', value: '024', unit: 'deg' }, track({ hdg: 24 }));
    assert.equal(z.text, 'turned to 024°');
});

test('a heading turned through', () => {
    const r = run(H270, track({ hdg: (s) => 330 - s / 2 }));
    assert.equal(r.text, 'turning through 270°, 2 min later');
    assert.equal(r.at, T0 + 120000);
});

// ---------------------------------------------------------------- speed

const S220 = { role: 'speed', text: 'Reduce speed to 220 kt', value: '220', unit: 'kt' };

test('a speed reached', () => {
    const r = run(S220, track({ gs: (s) => Math.max(220, 280 - s / 2) }));
    assert.equal(r.kind, 'speed');
    assert.equal(r.text, '225 kt, 2 min later');
});

test('a speed on its way at the end of the window', () => {
    const r = run(S220, track({ gs: (s) => 300 - s / 6 }));
    assert.equal(r.text, 'slowing, 241 kt');
});

test('a speed unchanged', () => {
    const r = run(S220, track({ gs: 250 }));
    assert.equal(r.text, '250 kt, 6 min later');
});

test('a speed passed through', () => {
    const r = run({ role: 'speed', text: 'Increase speed to 250 kt', value: '250', unit: 'kt' }, track({ gs: (s) => 180 + s }));
    assert.equal(r.text, 'accelerating through 250 kt, 1 min later');
    assert.equal(r.at, T0 + 70000);
});

// ---------------------------------------------------------------- frequency

const C119 = { role: 'frequency', text: 'Contact 119.255', value: '119.255', unit: 'MHz' };

test('the next transmission heard on another frequency', () => {
    const r = run(C119, [], { transmissions: [{ at: at(-30), frequency: 120.85 }, { at: at(200), frequency: 119.255 }, { at: at(400), frequency: 119.255 }] });
    assert.equal(r.kind, 'frequency');
    assert.equal(r.text, 'next heard on 119.255, 3 min later');
    assert.equal(r.at, T0 + 200000);
});

test('no later transmission', () => {
    assert.equal(run(C119, [], { transmissions: [{ at: at(-30), frequency: 120.85 }] }).text, 'no later transmission heard here');
    assert.equal(run(C119, [], {}), null, 'without the list, nothing is said');
});

test('a later transmission whose frequency is unknown', () => {
    assert.equal(run(C119, [], { transmissions: [{ at: at(42) }] }).text, 'next heard, 40 s later');
});

// ---------------------------------------------------------------- runway

// A runway 09/27 laid along latitude 48.80, 3 km long, the 27 end east.
const RWY = { le_ident: '09', le_latitude: 48.8, le_longitude: 2.0, he_ident: '27', he_latitude: 48.8, he_longitude: 2.041 };
const FRAMES = ParGeometry.buildFrames('LFXX', [RWY]);

// Westbound at 140 kt, 3 ft per 10 s... down to the ground at second 100.
const landing = (lat = 48.8) => track({
    alt: (s) => (s < 100 ? 1000 - s * 10 : 0),
    hdg: 268,
    lat,
    lon: (s) => 2.08 - s * 0.0004,
    gs: (s) => (s < 100 ? 140 : 60),
});

test('a landing on the runway it was cleared to', () => {
    const r = run({ role: 'clearance', text: 'Cleared to land, runway 27R', value: '27R' }, landing());
    assert.equal(r.kind, 'clearance');
    assert.equal(r.text, 'landed on 27');
});

test('with the frames, the runway is named from where it touched down', () => {
    const r = run({ role: 'clearance', text: 'Cleared to land, runway 27', value: '27' }, landing(), { frames: FRAMES });
    assert.equal(r.text, 'landed on 27');
    // Touched down 1 km north of the axis: no frame has it, the heading names it.
    const off = run({ role: 'clearance', text: 'Cleared to land, runway 27', value: '27' }, landing(48.81), { frames: FRAMES });
    assert.equal(off.text, 'landed on 27');
});

test('a landing the other way names nothing', () => {
    const r = run({ role: 'clearance', text: 'Cleared to land, runway 09', value: '09' }, landing());
    assert.equal(r, null);
});

test('a takeoff', () => {
    const pts = track({ alt: (s) => (s < 60 ? 0 : (s - 60) * 20), hdg: 85, lat: 48.8, lon: (s) => 2.001 + s * 0.0003 });
    const r = run({ role: 'clearance', text: 'Cleared for takeoff, runway 09', value: '09' }, pts, { frames: FRAMES });
    assert.equal(r.text, 'took off from 09');
    assert.equal(r.at, T0 + 65000);
    // A landing clearance does not look for a takeoff.
    assert.equal(run({ role: 'clearance', text: 'Cleared to land, runway 09', value: '09' }, pts), null);
});

test('a runway named alone looks for either', () => {
    const r = run({ role: 'runway', text: 'Runway 27', value: '27', unit: 'runway' }, landing());
    assert.equal(r.kind, 'runway');
    assert.equal(r.text, 'landed on 27');
});

test('no touchdown in the track: null', () => {
    assert.equal(run({ role: 'clearance', text: 'Cleared to land, runway 27', value: '27' }, track({ alt: (s) => 3000 - s * 5, hdg: 268 })), null);
    assert.equal(run({ role: 'clearance', text: 'Cleared for the approach' }, landing()), null, 'no runway, no frames');
});

test('without a runway said, the frames still name it', () => {
    const r = run({ role: 'clearance', text: 'Cleared for the approach' }, landing(), { frames: FRAMES });
    assert.equal(r.text, 'landed on 27');
});

// ---------------------------------------------------------------- reading the value

test('the value read from the text, by role', () => {
    assert.deepEqual(valueOf({ role: 'flight_level', text: 'Descend to FL100' }), { n: 100, unit: 'FL' });
    assert.deepEqual(valueOf({ role: 'flight_level', text: 'Climb to flight level 080' }), { n: 80, unit: 'FL' });
    assert.deepEqual(valueOf({ role: 'altitude', text: 'Climb to 4000 ft' }), { n: 4000, unit: 'ft' });
    assert.deepEqual(valueOf({ role: 'altitude', text: `Descend to 2${NNBSP}500 ft` }), { n: 2500, unit: 'ft' });
    assert.deepEqual(valueOf({ role: 'altitude', text: 'Descend to 1,500 feet' }), { n: 1500, unit: 'ft' });
    assert.deepEqual(valueOf({ role: 'altitude', text: 'Maintain 500 ft' }), { n: 500, unit: 'ft' });
    assert.deepEqual(valueOf({ role: 'heading', text: 'Turn left heading 270' }), { n: 270, unit: 'deg' });
    assert.deepEqual(valueOf({ role: 'heading', text: 'Heading 024' }), { n: 24, unit: 'deg' });
    assert.deepEqual(valueOf({ role: 'speed', text: 'Reduce speed to 220 kt' }), { n: 220, unit: 'kt' });
    assert.deepEqual(valueOf({ role: 'frequency', text: 'Contact 119.255' }), { s: '119.255', unit: 'MHz' });
    assert.deepEqual(valueOf({ role: 'frequency', text: 'Contact 120.85' }), { s: '120.850', unit: 'MHz' });
    assert.deepEqual(valueOf({ role: 'clearance', text: 'Cleared to land, runway 27R' }), { s: '27R', unit: 'runway' });
    assert.deepEqual(valueOf({ role: 'runway', text: 'Runway 09' }), { s: '09', unit: 'runway' });
    assert.deepEqual(valueOf({ role: 'clearance', text: 'Cleared for takeoff' }), { s: '', unit: 'runway' });
    assert.equal(valueOf({ role: 'frequency', text: 'Contact 99.5' }), null);
    assert.equal(valueOf({ role: 'heading', text: 'Heading 400' }), null);
});

test('the server value wins over the text', () => {
    assert.deepEqual(valueOf({ role: 'flight_level', text: 'Descend to FL100', value: '090', unit: 'FL' }), { n: 90, unit: 'FL' });
    assert.deepEqual(valueOf({ role: 'altitude', text: '', value: '4000', unit: 'ft' }), { n: 4000, unit: 'ft' });
    assert.deepEqual(valueOf({ role: 'clearance', text: 'Cleared to land', value: '26L' }), { s: '26L', unit: 'runway' });
});

// ---------------------------------------------------------------- every text

// Every form a text can take, as the report lists them for AircraftStory.
const L = `(?:\\d{1,2}${NNBSP})?\\d{1,3} ft|FL \\d{3}`;
const D = ', \\d+ (?:s|min) later';
const FORMS = [
    `level at (?:${L})${D}`,
    `at (?:${L})${D}`,
    `(?:climbing|descending) through (?:${L})(?:${D})?`,
    `on the ground${D}`,
    'turned to \\d{3}°',
    `turning through \\d{3}°${D}`,
    'heading \\d{3}°, still turning',
    `heading \\d{3}°${D}`,
    `\\d{2,3} kt${D}`,
    `(?:accelerating|slowing) through \\d{2,3} kt${D}`,
    `(?:accelerating|slowing), \\d{2,3} kt(?:${D})?`,
    'no later transmission heard here',
    `next heard on 1\\d\\d\\.\\d{3}${D}`,
    `next heard${D}`,
    'landed on \\d{2}[LRC]?',
    'took off from \\d{2}[LRC]?',
].map((f) => new RegExp(`^${f}$`));

test('every text takes one of the listed forms, and every form was produced', () => {
    for (const text of seen) assert.ok(FORMS.some((f) => f.test(text)), `${text}: not a listed form`);
    for (const f of FORMS) assert.ok(seen.some((t) => f.test(t)), `${f}: never produced`);
});

// Constater, ne pas juger (D78): no word that grades what the aircraft did.
test('no output passes a verdict', () => {
    assert.ok(seen.length >= 30, `${seen.length} texts`);
    for (const text of seen) {
        assert.doesNotMatch(text, /\b(deviat\w*|compl(y|ied|iance)\w*|error\w*|violat\w*|wrong\w*)\b/i, text);
        assert.doesNotMatch(text.replace(/\bstill turning\b/g, ''), /\bstill\b/i, text);
        assert.doesNotMatch(text, /[!]/, text);
    }
    // And on the forms themselves, whatever the numbers.
    for (const f of FORMS) {
        const words = f.source.replace(/\\d|\\.|\[[^\]]*\]|[(){}?:|^$*+]/g, ' ');
        assert.doesNotMatch(words, /\b(deviat\w*|compl(y|ied|iance)\w*|error\w*|violat\w*|wrong\w*)\b/i, f.source);
        assert.doesNotMatch(words.replace(/\bstill turning\b/g, ''), /\bstill\b/i, f.source);
    }
});

// The design system's numbers: a narrow no-break space between thousands,
// "FL 110", a comma before the delay.
test('every output follows the number format', () => {
    for (const text of seen) {
        assert.doesNotMatch(text, /\d,\d{3}/, `${text}: a comma between thousands`);
        assert.doesNotMatch(text, /\d \d/, `${text}: a plain space between thousands`);
        assert.doesNotMatch(text, /\d{4,}/, `${text}: thousands without their space`);
        assert.doesNotMatch(text, /FL(?! \d{3}\b)/, `${text}: FL without its space and three digits`);
        if (/\d (s|min) later/.test(text)) assert.match(text, /, \d+ (s|min) later$/, text);
        if (/ ft\b/.test(text)) assert.match(text, /\d ft\b/, text);
    }
    assert.ok(seen.some((t) => t.includes(NNBSP)), 'a thousands separator was exercised');
});
