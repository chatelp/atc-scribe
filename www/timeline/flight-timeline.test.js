const test = require('node:test');
const assert = require('node:assert/strict');
const T = require('./flight-timeline.js');

const at = (min) => new Date(Date.UTC(2026, 9, 10, 14, min, 0)).toISOString();

const aircraft = {
    hex: '3c6444', flight: 'AFR123', created_at: at(0), last_seen: at(40), status: 'signal_lost', on_ground: false,
    phase: { history: [{ phase: 'APP', timestamp: at(10) }, { phase: 'T/D', timestamp: at(30) }] },
    runway: { arrival: { airport: 'LFPG', runway: '27R', since: at(28) } },
    date_landed: at(30),
    clearances: [{ type: 'landing', text: 'cleared to land runway 27R', runway: '27R', timestamp: at(25) }],
};
const frequencies = [{ id: '124350', name: '124.350 Approach', frequency_mhz: 124.35 }, { id: '126425', name: '126.425 De Gaulle', frequency_mhz: 126.425 }];
const transmissions = [
    { id: 7, frequency_id: '124350', created_at: at(20), speaker_type: 'ATC', content_processed: 'air france one two three contact de gaulle one two six four two five' },
    { id: 8, frequency_id: '126425', created_at: at(22), speaker_type: 'PILOT', content_processed: 'de gaulle air france one two three' },
];
const values = { 7: [{ role: 'frequency', text: '126.425' }], 8: [{ role: 'frequency', text: '126.425' }] };

test('the story is told oldest first, from every source', () => {
    const items = T.build({ aircraft, transmissions, values, frequencies });
    assert.deepEqual(items.map((i) => i.kind),
        ['seen', 'phase', 'transmission', 'transfer', 'transmission', 'clearance', 'runway', 'phase', 'lost']);
    for (let i = 1; i < items.length; i++) assert.ok(items[i].at >= items[i - 1].at);
});

test('a transfer heard names the frequency, and says when the station listens to it', () => {
    const items = T.build({ aircraft, transmissions, values, frequencies });
    const transfers = items.filter((i) => i.kind === 'transfer');
    assert.equal(transfers.length, 1, 'the pilot reading back the frequency is no second transfer');
    assert.equal(transfers[0].text, 'Told to contact 126.425, 126.425 De Gaulle, which the station listens to');
    const elsewhere = T.build({ aircraft: {}, transmissions: [transmissions[0]], values: { 7: [{ role: 'frequency', text: '119.255' }] }, frequencies });
    assert.equal(elsewhere.find((i) => i.kind === 'transfer').text, 'Told to contact 119.255');
});

test('transmissions say who spoke and on which frequency', () => {
    const items = T.build({ aircraft: {}, transmissions, values, frequencies }).filter((i) => i.kind === 'transmission');
    assert.equal(items[0].text, 'Controller on 124.350 Approach');
    assert.equal(items[1].text, 'Pilot on 126.425 De Gaulle');
    assert.equal(items[0].transmission.id, 7);
});

test('a landing runway reads as landed once the aircraft has landed', () => {
    const items = T.build({ aircraft });
    assert.equal(items.find((i) => i.kind === 'runway').text, 'Landed on runway 27R at LFPG');
    const onFinal = T.build({ aircraft: { ...aircraft, date_landed: null } });
    assert.equal(onFinal.find((i) => i.kind === 'runway').text, 'Lined up for runway 27R at LFPG');
});

test('a longer phase history replaces the one the aircraft carries', () => {
    const phases = [{ phase: 'CRZ', timestamp: at(1) }, { phase: 'APP', timestamp: at(10) }, { phase: 'T/D', timestamp: at(30) }];
    assert.equal(T.build({ aircraft, phases }).filter((i) => i.kind === 'phase').length, 3);
});

test('missing or bad dates are left out, not shown at the epoch', () => {
    const items = T.build({ aircraft: { created_at: 'nonsense', clearances: [{ type: 'takeoff' }] } });
    assert.equal(items.length, 0);
});

test('NEW and UNK phases are left out, and an unknown frequency reads in MHz', () => {
    const items = T.build({
        aircraft: { phase: { history: [{ phase: 'NEW', timestamp: at(0) }, { phase: 'UNK', timestamp: at(1) }, { phase: 'CLB', timestamp: at(2) }] } },
        transmissions: [{ id: 1, frequency_id: '119255', created_at: at(3), speaker_type: 'ATC' }],
    });
    assert.deepEqual(items.map((i) => i.text), ['Climb', 'Controller on 119.255']);
});
