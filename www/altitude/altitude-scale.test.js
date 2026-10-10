const test = require('node:test');
const assert = require('node:assert/strict');
const S = require('./altitude-scale.js');

test('the stops give the low levels about half the travel', () => {
    const list = S.stops(5000);
    assert.equal(list[0], 0);
    assert.equal(list[list.length - 1], S.NO_CEILING);
    assert.equal(list[list.length - 2], 45000);
    const ta = list.indexOf(5000);
    assert.ok(ta > 0);
    const share = ta / (list.length - 1);
    assert.ok(share > 0.4 && share < 0.6, `share below TA ${share}`);
    for (let i = 1; i < list.length; i++) assert.ok(list[i] > list[i - 1], `stops in order at ${i}`);
});

test('an altitude goes to its nearest stop, past FL450 to no ceiling', () => {
    const list = S.stops(5000);
    assert.equal(list[S.indexOf(list, 2440)], 2400);
    assert.equal(list[S.indexOf(list, 12300)], 12500);
    assert.equal(list[S.indexOf(list, 31400)], 31000);
    assert.equal(list[S.indexOf(list, 60000)], S.NO_CEILING);
    assert.equal(list[S.indexOf(list, 0)], 0);
});

test('feet up to the transition altitude, flight levels above', () => {
    assert.equal(S.label(0), 'surface');
    assert.equal(S.label(2500), '2 500 ft');
    assert.equal(S.label(5000), '5 000 ft');
    assert.equal(S.label(5500), 'FL055');
    assert.equal(S.label(12000), 'FL120');
    assert.equal(S.label(60000), 'no ceiling');
    assert.equal(S.label(5500, 7000), '5 500 ft');
});

test('a band is said the way it is read', () => {
    assert.equal(S.bandLabel(0, 60000), 'all altitudes');
    assert.equal(S.bandLabel(0, 3000), 'up to 3 000 ft');
    assert.equal(S.bandLabel(19500, 60000), 'FL195 and above');
    assert.equal(S.bandLabel(1500, 12000), '1 500 ft to FL120');
});

test('the histogram counts aircraft along the travel', () => {
    const list = S.stops(5000);
    const counts = S.histogram(list, [0, 100, 37000, 37000, NaN], 10);
    assert.equal(counts.reduce((a, b) => a + b, 0), 4);
    assert.equal(counts[0], 2);
    assert.equal(counts[Math.floor(S.position(list, 37000) * 10)], 2);
});

test('every preset falls exactly on stops', () => {
    const list = S.stops(5000);
    for (const p of S.presets(5000)) {
        assert.equal(list[S.indexOf(list, p.min)], p.min, `${p.id} min`);
        assert.equal(list[S.indexOf(list, p.max)], p.max, `${p.id} max`);
    }
});
