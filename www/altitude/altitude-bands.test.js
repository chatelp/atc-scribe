const test = require('node:test');
const assert = require('node:assert/strict');
const B = require('./altitude-bands.js');

test('the bands are the map renderer\'s, the cruise level the station\'s', () => {
    assert.equal(B.bandOf(-50), 'very-low');
    assert.equal(B.bandOf(999), 'very-low');
    assert.equal(B.bandOf(1000), 'low');
    assert.equal(B.bandOf(3000), 'mid');
    assert.equal(B.bandOf(12500), 'mid');
    assert.equal(B.bandOf(12600), 'high');
    assert.equal(B.bandOf(18000), 'cruise');
    assert.equal(B.bandOf(18000, 24000), 'high');
    assert.equal(B.bandOf(NaN), 'cruise');
});

test('each band has its bars, every altitude lands in one', () => {
    const sl = B.slices();
    assert.deepEqual(B.NAMES.map((n) => sl[n].length), [4, 4, 8, 5, 9]);
    assert.deepEqual(sl.cruise[8], [42000, 45000]);
    const l = B.legend([100, 900, 1200, 2900, 5000, 12500, 15000, 35000, 47000, -20]);
    assert.deepEqual(l.bands.map((b) => b.count), [3, 2, 2, 1, 2]);
    assert.equal(l.bands[0].bars[0], 2);            // 100 ft and -20 ft
    assert.equal(l.bands[4].bars[8], 1);            // above FL 450, in the last bar
    assert.equal(l.bands.reduce((s, b) => s + b.bars.reduce((x, y) => x + y, 0), 0), 10);
    assert.equal(l.peak, 2);
});

test('a cruise level at or under FL 125 leaves the high band empty', () => {
    const l = B.legend([13000], 12000);
    assert.equal(B.slices(12000).high.length, 0);
    assert.equal(l.bands[4].count, 1);
});
