#!/usr/bin/env node
// Candidate palettes for the altitude of aircraft icons on the dark map,
// measured after the grey ramp was refused in use (13/10/2026, step 2 of the
// migration: "too hard to tell apart").
//
//   node tools/design/altitude.mjs            # the table, in markdown
//
// For each candidate, bands from very low to cruise:
// - adjacent: the smallest ΔE76 between two neighbouring bands, under normal
//   sight and the three dichromacies (the worst of the four): how well one band
//   is told from the next, the owner's complaint;
// - contrast: the lowest contrast of a band on the black map (icons are marks:
//   3:1 required);
// - urgent: the smallest ΔE between a band and the urgent red (#ff6e6e) under
//   the four visions: a band that looks like an emergency cries wolf;
// - family: the smallest ΔE between a band and a phase family colour (the
//   label of the same aircraft), under the four visions.
import { contrast, deltaE, simulate, CVD } from './color.mjs';

const URGENT = '#ff6e6e';
const FAMILIES = { ground: '#dccb9e', departure: '#ec8b34', cruise: '#b8d3f2', arrival: '#4c9ce8', unknown: '#969696' };
const VISIONS = [['normal', null], ...Object.entries(CVD)];

export const CANDIDATES = {
    'A · before (hue, back since 13/10)': ['#D35A48', '#EA8A4A', '#F4B55A', '#F8D857', '#E8ECEF'],
    'B · grey ramp (refused)': ['#f5f5f5', '#cfcfcf', '#a8a8a8', '#858585', '#6b6b6b'],
    'C · three greys, wide steps': ['#ffffff', '#ffffff', '#a6a6a6', '#a6a6a6', '#616161'],
    'D · warm to cool, no red': ['#fff07a', '#ffb0d8', '#c4a0ff', '#62c8e8', '#7a8699'],
    'E · before, red and orange replaced': ['#ff7fd6', '#ffb35c', '#f4d35e', '#b9f27a', '#e8ecef'],
    'F · warm to cool, wider (chosen 13/10)': ['#fff59a', '#ff9ccf', '#b38cff', '#4fc3e0', '#6e7b8f'],
    'F, adjusted: pink and cyan apart under protanopia (in jetons.json)': ['#fff59a', '#e68cba', '#c2a3ff', '#61c9e3', '#6e7b8f'],
    'G · white, yellow, pink, violet, slate': ['#ffffff', '#ffe066', '#ff8fc8', '#a98bff', '#6f7d94'],
};
export const BANDS = ['very low', 'low', 'mid', 'high', 'cruise'];

const seen = (h, m) => (m ? simulate(h, m) : h);
const worst = (a, b) => Math.min(...VISIONS.map(([, m]) => deltaE(seen(a, m), seen(b, m))));

export function measure(p) {
    const distinct = [...new Set(p)];
    let adjacent = Infinity, normal = Infinity;
    for (let i = 0; i + 1 < distinct.length; i++) {
        adjacent = Math.min(adjacent, worst(distinct[i], distinct[i + 1]));
        normal = Math.min(normal, deltaE(distinct[i], distinct[i + 1]));
    }
    return {
        bands: distinct.length,
        normal,
        adjacent,
        contrast: Math.min(...p.map((h) => contrast(h, '#000000'))),
        urgent: Math.min(...p.map((h) => worst(h, URGENT))),
        family: Math.min(...p.flatMap((h) => Object.values(FAMILIES).map((f) => worst(h, f)))),
    };
}

if (import.meta.url === `file://${process.argv[1]}`) {
    const f = (x) => x.toFixed(1);
    console.log('| Candidate | bands | adjacent ΔE, normal sight | adjacent ΔE, worst vision | contrast on black | ΔE to urgent | ΔE to a phase family |');
    console.log('|---|---:|---:|---:|---:|---:|---:|');
    for (const [name, p] of Object.entries(CANDIDATES)) {
        const m = measure(p);
        console.log(`| ${name} | ${m.bands} | ${f(m.normal)} | ${f(m.adjacent)} | ${f(m.contrast)}:1 | ${f(m.urgent)} | ${f(m.family)} |`);
    }
}
