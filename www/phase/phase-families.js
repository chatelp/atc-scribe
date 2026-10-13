/**
 * The five families of flight phase (design system, docs-fr/design/03-systeme.md,
 * migration step 2): ten phase codes, five colours a dichromat tells apart
 * (minimum distance between families 23.8 under the four visions, against
 * 1.8 for the ten colours of before). The code stays written on the badge:
 * T/O, CLB and DEP are all orange, the badge says which.
 *
 * One source for the map (WebGL labels), the PAR (SVG) and the list (Tailwind
 * classes): the colours come from the tokens (www/tokens.css) when a page is
 * there, with the same hex as fallbacks for node and for a canvas that cannot
 * read a CSS variable.
 */
(function (root, factory) {
    const api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;
    } else {
        root.PhaseFamilies = api;
    }
})(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    // Phase code -> family.
    const FAMILY_OF = Object.freeze({
        'TAX': 'ground',
        'T/O': 'departure', 'CLB': 'departure', 'DEP': 'departure',
        'CRZ': 'cruise',
        'ARR': 'arrival', 'APP': 'arrival', 'T/D': 'arrival',
        'NEW': 'unknown', 'UNK': 'unknown',
    });

    // The tokens' values (jetons.json, color.phase.*), for a canvas and for node.
    const HEX = Object.freeze({
        ground: '#dccb9e',
        departure: '#ec8b34',
        cruise: '#b8d3f2',
        arrival: '#4c9ce8',
        unknown: '#969696',
    });

    // The two instants that count have a solid badge; the others an outlined one:
    // the difference is of shape, not of hue.
    const SOLID = new Set(['T/O', 'T/D']);

    // Tailwind utilities for the list's badge, written in full so that the
    // compiler keeps them (tools/tailwind/tailwind.config.js scans www/**/*.js).
    const BADGE = Object.freeze({
        ground: { outline: 'bg-surface-raised text-phase-ground border border-phase-ground', solid: 'bg-phase-ground text-on-warn border border-phase-ground' },
        departure: { outline: 'bg-surface-raised text-phase-departure border border-phase-departure', solid: 'bg-phase-departure text-on-warn border border-phase-departure' },
        cruise: { outline: 'bg-surface-raised text-phase-cruise border border-phase-cruise', solid: 'bg-phase-cruise text-on-warn border border-phase-cruise' },
        arrival: { outline: 'bg-surface-raised text-phase-arrival border border-phase-arrival', solid: 'bg-phase-arrival text-on-warn border border-phase-arrival' },
        unknown: { outline: 'bg-surface-raised text-phase-unknown border border-phase-unknown', solid: 'bg-phase-unknown text-on-warn border border-phase-unknown' },
    });
    const TEXT = Object.freeze({
        ground: 'text-phase-ground', departure: 'text-phase-departure', cruise: 'text-phase-cruise',
        arrival: 'text-phase-arrival', unknown: 'text-phase-unknown',
    });

    // The short modifier the components use: cl-strip--arr, cl-phase--arr, cl-dot--arr.
    const MODIFIER = Object.freeze({ ground: 'gnd', departure: 'dep', cruise: 'crz', arrival: 'arr', unknown: 'unk' });
    const LABEL = Object.freeze({ ground: 'Ground', departure: 'Departure', cruise: 'Cruise', arrival: 'Arrival', unknown: 'Unknown' });
    // On a filter toggle, where the row must hold the five: the reference's short forms.
    const SHORT = Object.freeze({ ground: 'Gnd', departure: 'Dep', cruise: 'Crz', arrival: 'Arr', unknown: '?' });
    const FAMILIES = Object.freeze(['ground', 'departure', 'cruise', 'arrival', 'unknown']);

    function codesOf(family) {
        return Object.keys(FAMILY_OF).filter((c) => FAMILY_OF[c] === family);
    }

    function modifierOf(phase) {
        return MODIFIER[familyOf(phase)];
    }

    function code(phase) {
        return String(phase || '').trim().toUpperCase();
    }

    function familyOf(phase) {
        return FAMILY_OF[code(phase)] || 'unknown';
    }

    // The family's colour as the page defines it (the token), else the same hex.
    let cache = null;
    function colorOf(phase) {
        const fam = familyOf(phase);
        if (typeof document !== 'undefined' && document.documentElement) {
            if (!cache) {
                cache = {};
                const cs = getComputedStyle(document.documentElement);
                for (const f of Object.keys(HEX)) {
                    const v = cs.getPropertyValue(`--color-phase-${f}`).trim();
                    cache[f] = v || HEX[f];
                }
            }
            return cache[fam];
        }
        return HEX[fam];
    }

    function isSolid(phase) {
        return SOLID.has(code(phase));
    }

    // The classes of the list's phase badge: outlined, or solid for T/O and T/D.
    function badgeClasses(phase) {
        const b = BADGE[familyOf(phase)];
        return isSolid(phase) ? b.solid : b.outline;
    }

    // The text colour class of a phase code, where only the text is coloured.
    function textClass(phase) {
        return TEXT[familyOf(phase)];
    }

    return { FAMILY_OF, HEX, FAMILIES, MODIFIER, LABEL, SHORT, familyOf, codesOf, modifierOf, colorOf, isSolid, badgeClasses, textClass };
});
