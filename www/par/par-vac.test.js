// node --test 'www/par/*.test.js'
const test = require('node:test');
const assert = require('node:assert/strict');
const V = require('./par-vac.js');

const day = (iso) => new Date(iso + 'T00:00:00Z');

test('the cycle in force is the one that began last', () => {
    assert.equal(V.airacFolderDate(day('2026-10-09')), '01_OCT_2026');
    assert.equal(V.airacFolderDate(day('2026-10-01')), '01_OCT_2026');
    assert.equal(V.airacFolderDate(new Date('2026-10-28T23:59:59Z')), '01_OCT_2026');
});

test('a new cycle starts every 28 days', () => {
    assert.equal(V.airacFolderDate(day('2026-10-29')), '29_OCT_2026');
    assert.equal(V.airacFolderDate(day('2026-09-30')), '03_SEP_2026');
});

test('years go by: the cycles of 2027', () => {
    assert.equal(V.airacFolderDate(day('2027-01-15')), '24_DEC_2026');
    assert.equal(V.airacFolderDate(day('2027-01-21')), '21_JAN_2027');
    assert.equal(V.airacFolderDate(day('2027-12-31')), '23_DEC_2027');
});

test('the VAC of a French aerodrome at the SIA', () => {
    const link = V.vacLink('lfpg', day('2026-10-09'));
    assert.equal(link.url,
        'https://www.sia.aviation-civile.gouv.fr/media/dvd/eAIP_01_OCT_2026/Atlas-VAC/PDF_AIPparSSection/VAC/AD/AD-2.LFPG.pdf');
    assert.equal(link.cycle, '01/10/2026');
});

test('no link outside France', () => {
    assert.equal(V.vacLink('EGLL', day('2026-10-09')), null);
    assert.equal(V.vacLink('', day('2026-10-09')), null);
    assert.equal(V.vacLink('LFP', day('2026-10-09')), null);
});
