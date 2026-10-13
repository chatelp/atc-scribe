// node --test 'www/phase/*.test.js'
const test = require('node:test');
const assert = require('node:assert/strict');
const P = require('./phase-families.js');

test('the ten codes fall into the five families of the design system', () => {
    assert.deepEqual(
        Object.fromEntries(['NEW', 'TAX', 'T/O', 'CLB', 'DEP', 'CRZ', 'ARR', 'APP', 'T/D', 'UNK'].map((c) => [c, P.familyOf(c)])),
        { NEW: 'unknown', TAX: 'ground', 'T/O': 'departure', CLB: 'departure', DEP: 'departure', CRZ: 'cruise', ARR: 'arrival', APP: 'arrival', 'T/D': 'arrival', UNK: 'unknown' },
    );
    // Case and spaces do not matter; anything else is unknown.
    assert.equal(P.familyOf(' crz '), 'cruise');
    assert.equal(P.familyOf(''), 'unknown');
    assert.equal(P.familyOf(undefined), 'unknown');
    assert.equal(P.familyOf('XYZ'), 'unknown');
});

test('without a page, the colours are the tokens\' hex values', () => {
    assert.equal(P.colorOf('ARR'), '#4c9ce8');
    assert.equal(P.colorOf('T/O'), '#ec8b34');
    assert.equal(P.colorOf('TAX'), '#dccb9e');
    assert.equal(P.colorOf('CRZ'), '#b8d3f2');
    assert.equal(P.colorOf('NEW'), '#969696');
    assert.equal(P.colorOf('nope'), '#969696');
});

test('each family has its short modifier, its label and its codes', () => {
    assert.deepEqual(P.FAMILIES, ['ground', 'departure', 'cruise', 'arrival', 'unknown']);
    assert.equal(P.modifierOf('APP'), 'arr');
    assert.equal(P.modifierOf('TAX'), 'gnd');
    assert.equal(P.modifierOf('???'), 'unk');
    assert.deepEqual(P.codesOf('departure'), ['T/O', 'CLB', 'DEP']);
    assert.deepEqual(P.codesOf('unknown'), ['NEW', 'UNK']);
    assert.equal(P.LABEL.arrival, 'Arrival');
});

test('T/O and T/D have a solid badge, the other codes an outlined one', () => {
    assert.ok(P.isSolid('T/O') && P.isSolid('T/D'));
    for (const c of ['NEW', 'TAX', 'CLB', 'DEP', 'CRZ', 'ARR', 'APP', 'UNK']) assert.ok(!P.isSolid(c), c);
    assert.match(P.badgeClasses('T/D'), /^bg-phase-arrival text-on-warn /);
    assert.match(P.badgeClasses('APP'), /^bg-surface-raised text-phase-arrival /);
    assert.equal(P.textClass('DEP'), 'text-phase-departure');
    assert.equal(P.textClass('whatever'), 'text-phase-unknown');
});
