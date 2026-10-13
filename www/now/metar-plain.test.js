// node --test www/now/metar-plain.test.js
const test = require('node:test');
const assert = require('node:assert/strict');
const { read, point } = require('./metar-plain.js');

const plain = (s) => read(s).replace(/ /g, ' ');

test('the reference METAR of the design reads as the design says', () => {
    assert.equal(plain('LFPG 131300Z 25012KT 9999 FEW030 17/09 Q1018'),
        'Wind from the west, 12 kt; visibility over 10 km; a few clouds at 3 000 ft; 17 °C; QNH 1018.');
});

test('a real METAR from Orly, 13/10/2026', () => {
    assert.equal(plain('LFPO 131530Z AUTO 27010KT 9999 BKN043 OVC052 23/14 Q1024 NOSIG'),
        'Wind from the west, 10 kt; visibility over 10 km; broken clouds at 4 300 ft, overcast at 5 200 ft; 23 °C; QNH 1024; no significant change expected.');
});

test('calm, gusts, variable and varying wind', () => {
    assert.match(plain('LFPO 131530Z 00000KT CAVOK 12/08 Q1030'), /^Calm wind; visibility over 10 km; no cloud below 5 000 ft;/);
    assert.match(plain('LFPO 131530Z 31018G32KT 9999 Q1001'), /^Wind from the north-west, 18 kt, gusting 32 kt;/);
    assert.match(plain('LFPO 131530Z VRB03KT 9999 Q1001'), /^Variable wind, 3 kt;/);
    assert.match(plain('LFPO 131530Z 24012KT 210V280 9999 Q1001'), /^Wind from the south-west, varying 210° to 280°, 12 kt;/);
});

test('poor visibility, weather and storm clouds', () => {
    assert.equal(plain('LFPG 140600Z 09004KT 0800 FG VV002 08/08 Q1022'),
        'Wind from the east, 4 kt; visibility 800 m; fog; sky hidden, vertical visibility 200 ft; 8 °C; QNH 1022.');
    assert.match(plain('LFPO 131530Z 27015KT 4000 -SHRA BKN025CB 15/12 Q1009'), /visibility 4 000 m; light rain showers; broken clouds at 2 500 ft \(storm clouds\)/);
    assert.match(plain('LFPO 131530Z 27015KT 3000 +TSRA BKN020CB Q1009'), /heavy thunderstorm with rain/);
    assert.match(plain('LFPO 131530Z 27015KT 9999 M02/M05 Q1031'), /-2 °C/);
});

test('trends and remarks stop the reading; unknown groups are left out', () => {
    assert.match(plain('LFPO 131530Z 27010KT 9999 SCT030 15/10 Q1015 TEMPO 4000 SHRA'), /QNH 1015; changes expected\.$/);
    assert.equal(plain('LFPO 131530Z 27010KT 9999 R26/P2000 SCT030 Q1015 RMK AO2'),
        'Wind from the west, 10 kt; visibility over 10 km; scattered clouds at 3 000 ft; QNH 1015.');
});

test('what is not a METAR reads as nothing', () => {
    assert.equal(read(''), '');
    assert.equal(read(null), '');
    assert.equal(read('hello world'), '');
    assert.equal(read('LFPO 131530Z NIL'), '');
});

test('the eight points', () => {
    assert.deepEqual([0, 45, 90, 135, 180, 225, 270, 315, 350].map(point),
        ['north', 'north-east', 'east', 'south-east', 'south', 'south-west', 'west', 'north-west', 'north']);
});
