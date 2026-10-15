/**
 * The five altitude bands an aircraft's colour comes from on the map (the
 * design system's palette F, color-altitude-*), and the altitude legend's
 * bars inside each band (AltitudeLegend, owner 15/10). Pure functions, no
 * DOM -- loaded as a script in the page (window.AltitudeBands) and as a
 * module by the tests. The map's renderer, the legend and the Traffic
 * slider's histogram read the bands here, so that they cannot drift apart.
 *
 * The bands are on the aircraft's reported pressure altitude (alt_baro), as
 * the glyph's colour always was: under 1 000 ft, 1 000 to 3 000, 3 000 to
 * 12 500, above 12 500, and from the station's cruise level up.
 */
(function (root, factory) {
    const api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;
    } else {
        root.AltitudeBands = api;
    }
})(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    const NAMES = ['very-low', 'low', 'mid', 'high', 'cruise'];
    const DEFAULT_CRUISE_FT = 18000;
    // The legend's last bar holds what flies above it.
    const LEGEND_TOP_FT = 45000;

    function cruiseOf(cruiseFt) {
        const c = Number(cruiseFt);
        return Number.isFinite(c) && c > 0 ? c : DEFAULT_CRUISE_FT;
    }

    function bandOf(ft, cruiseFt) {
        if (!Number.isFinite(ft)) return 'cruise';
        if (ft >= cruiseOf(cruiseFt)) return 'cruise';
        if (ft > 12500) return 'high';
        if (ft >= 3000) return 'mid';
        if (ft >= 1000) return 'low';
        return 'very-low';
    }

    function even(from, to, n) {
        return Array.from({ length: n }, (_, i) => [from + (to - from) * i / n, from + (to - from) * (i + 1) / n]);
    }

    // Each band's bars, as [from, to) in feet: equal boxes on the legend, the
    // scale differing from one band to the next.
    function slices(cruiseFt) {
        const cruise = cruiseOf(cruiseFt);
        return {
            'very-low': even(0, 1000, 4),
            low: even(1000, 3000, 4),
            mid: even(3000, 12500, 8),
            high: cruise > 12500 ? even(12500, cruise, 5) : [],
            cruise: even(cruise, Math.max(cruise + 3000, LEGEND_TOP_FT), 9),
        };
    }

    // The legend of a set of altitudes: per band its bars and its count, and
    // the busiest bar of all, which the bars' heights are a share of.
    function legend(altitudes, cruiseFt) {
        const sl = slices(cruiseFt);
        const bands = NAMES.map((name) => ({ name, bars: sl[name].map(() => 0), count: 0 }));
        for (const ft of altitudes) {
            if (!Number.isFinite(ft)) continue;
            const name = bandOf(ft, cruiseFt);
            const band = bands[NAMES.indexOf(name)];
            band.count++;
            const edges = sl[name];
            if (!edges.length) continue;
            let i = edges.findIndex(([, to]) => ft < to);
            if (i < 0) i = edges.length - 1;
            band.bars[i]++;
        }
        const peak = Math.max(1, ...bands.flatMap((b) => b.bars));
        return { bands, peak };
    }

    // The four bounds between the bands, in feet.
    function bounds(cruiseFt) {
        return [1000, 3000, 12500, cruiseOf(cruiseFt)];
    }

    return { NAMES, DEFAULT_CRUISE_FT, bandOf, slices, legend, bounds };
});
