/**
 * The visual approach chart (VAC) of a French aerodrome, as the SIA publishes it.
 * Pure functions, no DOM -- loaded as a script in the page (window.ParVac) and
 * as a module by the tests (node --test 'www/par/*.test.js').
 *
 * The SIA files its PDFs under the date of the AIRAC cycle in force, so the
 * address changes every 28 days and older ones answer 404. The cycle is computed
 * from a known one (1 October 2026). The link is opened in a new tab and never
 * framed: the SIA's terms allow a direct link, not embedding.
 */
(function (root, factory) {
    const api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;
    } else {
        root.ParVac = api;
    }
})(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    const DAY_MS = 24 * 60 * 60 * 1000;
    const CYCLE_MS = 28 * DAY_MS;
    const REFERENCE_CYCLE = Date.UTC(2026, 9, 1); // AIRAC cycle effective 1 October 2026
    const MONTHS = ['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC'];

    const pad2 = (n) => String(n).padStart(2, '0');

    // The first day (UTC) of the AIRAC cycle in force at this date.
    function airacCycleStart(date) {
        const cycles = Math.floor((date.getTime() - REFERENCE_CYCLE) / CYCLE_MS);
        return new Date(REFERENCE_CYCLE + cycles * CYCLE_MS);
    }

    // The SIA's folder name for that cycle: eAIP_01_OCT_2026 -> '01_OCT_2026'.
    function airacFolderDate(date) {
        const start = airacCycleStart(date);
        return `${pad2(start.getUTCDate())}_${MONTHS[start.getUTCMonth()]}_${start.getUTCFullYear()}`;
    }

    // The VAC of a French civil aerodrome (LF..) at this date, or null for any
    // other code. Military aerodromes (LFPV...) have none at the SIA: their link
    // answers 404, which is accepted.
    function vacLink(icao, date) {
        const code = String(icao || '').toUpperCase();
        if (!/^LF[A-Z]{2}$/.test(code)) return null;
        const start = airacCycleStart(date);
        return {
            url: `https://www.sia.aviation-civile.gouv.fr/media/dvd/eAIP_${airacFolderDate(date)}`
                + `/Atlas-VAC/PDF_AIPparSSection/VAC/AD/AD-2.${code}.pdf`,
            cycle: `${pad2(start.getUTCDate())}/${pad2(start.getUTCMonth() + 1)}/${start.getUTCFullYear()}`,
        };
    }

    return { airacCycleStart, airacFolderDate, vacLink };
});
