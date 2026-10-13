// node --test www/now/right-now.test.js
const test = require('node:test');
const assert = require('node:assert/strict');
const R = require('./right-now.js');

const text = (html) => html.replace(/<[^>]+>/g, '').replace(/ /g, ' ');
const clock = (d) => d.toISOString().slice(11, 16);

// The followed airports as /api/v1/station gives them in production (13/10).
const airports = (orly, cdg) => ([
    { code: 'LFPO', name: 'Paris-Orly Airport', principal: true, runway_in_use: orly },
    { code: 'LFPG', name: 'Charles de Gaulle International Airport', principal: false, runway_in_use: cdg },
]);
const seq = (runway, inUse, items) => ({ runway, inUse, items });
const item = (callsign, distNM, etaSec, gapNM) => ({ hex: callsign.toLowerCase(), callsign, type: 'A320', distNM, etaSec, gapNM });

test('the runway in use since a change the server saw, the arrivals and the next touchdown', () => {
    const s = R.summary({
        airports: airports(
            [{ runway_end: '07-25/25', probability: 1, since: '2026-10-13T15:20:00Z' }],
            [{ runway_end: '09L-27R/27R', probability: 0.6, since: '2026-10-13T15:56:43Z' }, { runway_end: '08R-26L/26L', probability: 0.4 }],
        ),
        sequences: { LFPO: [seq('07-25/25', true, [item('AFR47KX', 6.2, 141, null)])], LFPG: [seq('09L-27R/27R', true, [item('EZY82QT', 14, 209, null), item('DLH9KR', 16.5, 215, 2.5)])] },
        next: { airport: 'LFPO', runway: '07-25/25', callsign: 'AFR47KX', etaSec: 141 },
        clock,
    });
    assert.equal(text(s.html),
        'Paris-Orly uses runway 25 since 15:20. Charles de Gaulle uses runway 27R since 15:56, perhaps 26L too. ' +
        'Three aircraft on final; the next, AFR47KX, lands on 25 at Paris-Orly in about 2 min.');
    assert.equal(text(s.short), 'Paris-Orly uses 25, three on final, next in about 2 min.');
    assert.equal(s.source, 'Read from the ADS-B tracks, not announced by the controllers.');
});

test('parallel runways both in use are named together', () => {
    const s = R.summary({ airports: airports([{ runway_end: '06L-24R/06L', probability: 0.55 }, { runway_end: '06R-24L/06R', probability: 0.45 }], null), clock });
    assert.match(text(s.html), /^Paris-Orly uses runways 06L and 06R\./);
});

test('since unknown: "since at least" the server started, never a bare "since"', () => {
    const s = R.summary({ airports: airports([{ runway_end: '07-25/25', probability: 1, since: null }], null), serverStarted: new Date('2026-10-13T15:05:00Z'), clock });
    assert.match(text(s.html), /^Paris-Orly uses runway 25 since at least 15:05\./);
    const t = R.summary({ airports: airports([{ runway_end: '07-25/25', probability: 1, since: null }], null), clock });
    assert.match(text(t.html), /^Paris-Orly uses runway 25\./);
    assert.doesNotMatch(text(t.html), /since/);
});

test('nothing known yet, nobody on final', () => {
    const s = R.summary({ airports: airports(null, null), clock });
    assert.equal(text(s.html), 'No runway in use known yet: it is judged from the takeoffs and landings the receiver sees. Nobody on final.');
    assert.equal(text(s.short), 'Nobody on final.');
});

test('the reference airport unknown, another known', () => {
    const s = R.summary({ airports: airports(null, [{ runway_end: '09L-27R/27R', probability: 1 }]), clock });
    assert.match(text(s.html), /^Paris-Orly: runway in use not known yet\. Charles de Gaulle uses runway 27R\./);
});

test('the folded card carries the latest warning', () => {
    const s = R.summary({
        airports: airports([{ runway_end: '07-25/25', probability: 1 }], null), clock,
        warning: { mark: '▲', cls: 'cl-warn', text: 'A go-around', at: new Date('2026-10-13T13:09:00Z') },
    });
    assert.match(s.short, /<span class="cl-warn">▲<\/span> A go-around at 13:09\.$/);
});

test('server values are escaped', () => {
    const s = R.summary({
        airports: [{ code: 'LFPO', name: '<script>x</script> Airport', principal: true, runway_in_use: [{ runway_end: '07-25/<b>', probability: 1 }] }],
        clock,
    });
    assert.doesNotMatch(s.html, /<script>|<b><b>/);
    assert.match(s.html, /&lt;script&gt;/);
});

test('the arrival tables: ends in use, nearest touchdown first, the PAR\'s words', () => {
    const t = R.tables(airports(null, null), {
        LFPO: [seq('07-25/25', true, [item('AFR47KX', 6.2, 141, null), item('EZY82QT', 8.9, 200, 2.7)]), seq('07-25/07', false, [item('XYZ1', 4, 90, null)])],
        LFPG: [seq('09L-27R/27R', true, [item('DLH9KR', 3, 60, null)]), seq('08R-26L/26L', true, [])],
    });
    assert.deepEqual(t.map((x) => x.title), ['Arrivals to LFPG 27R', 'Arrivals to LFPO 25']);
    const rows = t[1].rows.map((r) => [r.rank, r.callsign, r.toThreshold, r.time, r.behind, r.tight].join('|').replace(/ /g, ' '));
    assert.deepEqual(rows, ['1|AFR47KX|6.2 NM|2:21|first|false', '2|EZY82QT|8.9 NM|3:20|+2.7 NM|true']);
});

test('no end in use has aircraft: the ends that have some are shown', () => {
    const t = R.tables(airports(null, null), { LFPO: [seq('07-25/25', true, []), seq('07-25/07', false, [item('XYZ1', 4, 90, null)])] });
    assert.deepEqual(t.map((x) => x.title), ['Arrivals to LFPO 07']);
});

test('at most five rows, the rest counted', () => {
    const items = Array.from({ length: 7 }, (_, i) => item(`AC${i}`, 5 + i, 100 + i * 60, i ? 1 + i : null));
    const t = R.tables(airports(null, null), { LFPO: [seq('07-25/25', true, items)] });
    assert.equal(t[0].rows.length, 5);
    assert.equal(t[0].more, 2);
});

test('event marks by kind', () => {
    assert.deepEqual(R.eventMark('go_around'), { mark: '▲', cls: 'is-warn' });
    assert.deepEqual(R.eventMark('emergency'), { mark: '■', cls: 'is-urgent' });
    assert.deepEqual(R.eventMark('runway'), { mark: '', cls: '' });
});

test('"since at least" is said for the first airport only', () => {
    const s = R.summary({
        airports: airports([{ runway_end: '07-25/25', probability: 1, since: null }], [{ runway_end: '09L-27R/27R', probability: 1, since: null }]),
        serverStarted: new Date('2026-10-13T15:05:00Z'), clock,
    });
    assert.match(text(s.html), /^Paris-Orly uses runway 25 since at least 15:05\. Charles de Gaulle uses runway 27R\./);
});
