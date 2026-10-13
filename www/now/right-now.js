/**
 * Right now: what the sky is doing, in one sentence and one table (design
 * system, SummarySentence and ArrivalSequence; migration step 5).
 *
 * Built from templates, never generated: the runway each followed airport
 * uses and since when, how many aircraft are on final and the next
 * touchdown. Everything here is inferred from the ADS-B tracks, which the
 * source line says. Pure functions over what the store already holds; the
 * HTML they return escapes every value that comes from the server.
 */
(function (root, factory) {
    const api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;
    } else {
        root.RightNow = api;
    }
})(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    const NBSP = ' ';
    // Under 3 NM behind the one ahead: the usual radar spacing on final, the
    // PAR's amber (www/par/par-view.js, SEQUENCE_TIGHT_NM).
    const TIGHT_NM = 3;
    // A second runway is said when its likelihood passes this, as the bottom
    // bar did (alternate shown above 15 %).
    const SECOND_RUNWAY_PROBABILITY = 0.15;
    const MAX_ROWS = 5;
    const MAX_ROWS_SEVERAL = 3;

    const SOURCE = 'Read from the ADS-B tracks, not announced by the controllers.';

    const WORDS = ['no', 'one', 'two', 'three', 'four', 'five', 'six', 'seven', 'eight', 'nine', 'ten'];

    function esc(s) {
        return String(s == null ? '' : s)
            .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
    }

    function count(n) {
        return n >= 0 && n < WORDS.length ? WORDS[n] : String(n);
    }

    function capital(s) {
        return s.charAt(0).toUpperCase() + s.slice(1);
    }

    // "Charles de Gaulle International Airport" -> "Charles de Gaulle";
    // "Paris-Orly Airport" -> "Paris-Orly". The code when there is no name.
    function shortName(airport) {
        if (!airport) return '';
        const name = String(airport.name || '').trim();
        if (!name) return airport.code || '';
        return name.replace(/\s+(International\s+)?(Airport|Aerodrome|Airfield)$/i, '') || airport.code;
    }

    // "09L-27R/27R" -> "27R": the end in use, as the runway is named on its threshold.
    function runwayEnd(id) {
        if (!id) return '';
        const parts = String(id).split('/');
        return parts.length === 2 ? parts[1] : String(id);
    }

    // Two runway ends flown the same way: numbers one apart at most, around
    // the compass (De Gaulle's 26L and 27R land together). The server and the
    // event log use the same rule (internal/adsb/runway_tracker.go, events.js).
    function number(end) {
        const n = parseInt(String(end || '').replace(/[LRC]$/, ''), 10);
        return n >= 1 && n <= 36 ? n : null;
    }

    function sameDirection(a, b) {
        const na = number(runwayEnd(a)), nb = number(runwayEnd(b));
        if (na == null || nb == null) return false;
        let d = Math.abs(na - nb);
        if (d > 18) d = 36 - d;
        return d <= 1;
    }

    // What runway_in_use says for one airport: the ends in use (two when they
    // are parallel and both likely), a less likely other end, and since when.
    function runwaysInUse(entries) {
        if (!Array.isArray(entries) || !entries.length) return null;
        const first = runwayEnd(entries[0].runway_end);
        if (!first) return null;
        const out = { ends: [first], maybe: null, since: null };
        const second = entries[1];
        if (second && second.probability > SECOND_RUNWAY_PROBABILITY) {
            const end = runwayEnd(second.runway_end);
            if (sameDirection(first, end)) out.ends.push(end);
            else if (end && end !== first) out.maybe = end;
        }
        const since = entries[0].since ? new Date(entries[0].since) : null;
        out.since = since && !isNaN(since.getTime()) ? since : null;
        return out;
    }

    function ends(r) {
        return r.ends.map((e) => `<b>${esc(e)}</b>`).join(' and ');
    }

    // "in about 2 min", "in under a minute".
    function inAbout(sec) {
        if (sec == null || !isFinite(sec)) return '';
        if (sec < 60) return 'in under a minute';
        return `in about ${Math.round(sec / 60)}${NBSP}min`;
    }

    function mmss(sec) {
        const s = Math.max(0, Math.round(sec));
        return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
    }

    // The airports in the order the sentence names them: the reference airport
    // first, then the others as the server lists them.
    function ordered(airports) {
        const list = (airports || []).filter((a) => a && a.code);
        return list.filter((a) => a.principal).concat(list.filter((a) => !a.principal));
    }

    // The sequences shown, as the tables show them: each runway end in use
    // with aircraft coming in; if no end in use has any, the ends that have
    // some. The counts of the sentence read the same ones, so the sentence,
    // the folded card and the tables say the same number.
    function shown(airports, sequences) {
        const out = [];
        for (const apt of ordered(airports)) {
            for (const s of (sequences || {})[apt.code] || []) {
                if (s.items && s.items.length) out.push({ apt, s });
            }
        }
        const inUse = out.filter((t) => t.s.inUse);
        return inUse.length ? inUse : out;
    }

    function onFinal(list) {
        return list.reduce((n, t) => n + t.s.items.length, 0);
    }

    // summary builds the sentence. input:
    //   airports       the followed airports, each with runway_in_use
    //   sequences      store.arrivals: { code: [sequence] }
    //   next           store.nextTouchdown: { airport, runway, callsign, etaSec } or null
    //   serverStarted  a Date, for "since at least", or null
    //   clock          (Date) -> "17:20", in the page's chosen time
    //   warning        { mark, cls, text, at } of the latest notable event, or null
    // Returns { html, short, source }: html for the column, short for the card
    // over the map when the column is folded.
    function summary(input) {
        const o = input || {};
        const clock = typeof o.clock === 'function' ? o.clock : (d) => d.toISOString().slice(11, 16);
        const list = ordered(o.airports);
        const sentences = [];

        list.forEach((apt, i) => {
            const r = runwaysInUse(apt.runway_in_use);
            const name = esc(shortName(apt));
            if (!r) {
                if (i === 0) sentences.push(`${name}: runway in use not known yet.`);
                return;
            }
            // Since when, for every airport whose change the server saw; "since
            // at least" the server's start for the first one only, to keep the
            // sentence short. With "since", the present perfect.
            const rw = `${r.ends.length > 1 ? 'runways' : 'runway'} ${ends(r)}`;
            let s;
            if (r.since) s = `${name} has been using ${rw} since ${esc(clock(r.since))}`;
            else if (i === 0 && o.serverStarted) s = `${name} has been using ${rw} since at least ${esc(clock(o.serverStarted))}`;
            else s = `${name} uses ${rw}`;
            if (r.maybe) s += `, perhaps <b>${esc(r.maybe)}</b> too`;
            sentences.push(s + '.');
        });

        if (!list.some((a) => runwaysInUse(a.runway_in_use))) {
            sentences.length = 0;
            sentences.push('No runway in use known yet: it is judged from the takeoffs and landings the receiver sees.');
        }

        const finals = shown(o.airports, o.sequences);
        const n = onFinal(finals);
        const next = o.next;
        let arrivals = '';
        if (n > 0 && next) {
            const apt = list.find((a) => a.code === next.airport);
            const where = apt && list.length > 1 ? ` at ${esc(shortName(apt))}` : '';
            arrivals = `${capital(count(n))} aircraft on final; the next, ` +
                `${esc(next.callsign)}, lands on <b>${esc(runwayEnd(next.runway))}</b>${where} ${inAbout(next.etaSec)}.`;
        } else if (n > 0) {
            arrivals = `${capital(count(n))} aircraft on final.`;
        } else {
            arrivals = 'Nobody on final.';
        }
        sentences.push(arrivals);

        // Folded, one airport: the one of the next touchdown, with the count
        // on final there; with no arrival, the reference airport.
        let short;
        const nextApt = next && list.find((a) => a.code === next.airport);
        if (nextApt) {
            const there = onFinal(finals.filter((t) => t.apt.code === nextApt.code));
            short = `${esc(shortName(nextApt))} lands on <b>${esc(runwayEnd(next.runway))}</b>, ` +
                `${count(Math.max(1, there))} on final, next ${inAbout(next.etaSec)}.`;
        } else {
            const ref = list[0];
            const r = ref && runwaysInUse(ref.runway_in_use);
            const lead = ref && r ? `${esc(shortName(ref))} uses ${ends(r)}` : '';
            const said = n > 0 ? `${count(n)} on final` : 'nobody on final';
            short = lead ? `${lead}, ${said}.` : `${capital(said)}.`;
        }
        if (o.warning) {
            short += ` <span class="${esc(o.warning.cls || 'cl-warn')}">${esc(o.warning.mark)}</span> ` +
                `${esc(o.warning.text)} at ${esc(clock(o.warning.at))}.`;
        }

        return { html: sentences.join(' '), short, source: SOURCE };
    }

    // tables lists the arrival tables to show: each runway end in use with
    // aircraft coming in, nearest touchdown first; if no end in use has any,
    // the ends that have aircraft. Rows as the PAR writes them.
    function tables(airports, sequences) {
        const list = shown(airports, sequences);
        const firstEta = (t) => {
            const e = t.s.items.map((x) => x.etaSec).filter((x) => x != null);
            return e.length ? Math.min(...e) : Infinity;
        };
        list.sort((a, b) => firstEta(a) - firstEta(b));
        // Five rows when alone, three when there are several: three tables of
        // five pushed "Worth knowing" below the fold at 900 px.
        const rowsEach = list.length > 1 ? MAX_ROWS_SEVERAL : MAX_ROWS;
        return list.map(({ apt, s }) => ({
            key: `${apt.code}-${s.runway}`,
            title: `Arrivals to ${apt.code} ${runwayEnd(s.runway)}`,
            rows: s.items.slice(0, rowsEach).map((x, i) => ({
                rank: i + 1,
                hex: x.hex,
                callsign: x.callsign,
                type: x.type || '',
                toThreshold: x.distNM != null ? `${x.distNM.toFixed(1)}${NBSP}NM` : '—',
                time: x.etaSec != null ? mmss(x.etaSec) : '—',
                behind: x.gapNM == null ? 'first' : `+${x.gapNM.toFixed(1)}${NBSP}NM`,
                first: x.gapNM == null,
                tight: x.gapNM != null && x.gapNM < TIGHT_NM,
            })),
            more: Math.max(0, s.items.length - rowsEach),
        }));
    }

    // The mark and the class of an event in the list, by kind (EventList).
    function eventMark(kind) {
        switch (kind) {
        case 'emergency': return { mark: '■', cls: 'is-urgent' };
        case 'go_around': return { mark: '▲', cls: 'is-warn' };
        case 'interest': return { mark: '●', cls: 'is-info' };
        default: return { mark: '', cls: '' };
        }
    }

    return { summary, tables, runwaysInUse, runwayEnd, sameDirection, shortName, eventMark, inAbout, mmss, SOURCE, TIGHT_NM };
});
