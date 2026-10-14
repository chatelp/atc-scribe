/**
 * The story of one flight, as the station saw and heard it: first seen, the
 * phases, the runways, what was said to and by it on every frequency, the
 * transfers heard ("contact 119.255"), the clearances, and the signal lost.
 * Pure functions, no DOM -- loaded as a script in the page
 * (window.FlightTimeline) and as a module by the tests
 * (node --test 'www/timeline/*.test.js'). Audit 3.3, docs-fr/32.
 *
 * Every item: {at: Date, kind, icon, text, help?, transmission?, values?}.
 * Oldest first, as a story is read.
 */
(function (root, factory) {
    const api = factory();
    if (typeof module === 'object' && module.exports) {
        module.exports = api;
    } else {
        root.FlightTimeline = api;
    }
})(typeof self !== 'undefined' ? self : this, function () {
    'use strict';

    // What each phase of the server's phase detector means, in plain words.
    const PHASES = {
        NEW: ['New', 'just appeared, its phase not known yet'],
        TAX: ['Taxiing', 'moving on the ground'],
        'T/O': ['Takeoff', 'the wheels left the runway'],
        DEP: ['Departure', 'climbing away from the airport'],
        CLB: ['Climb', 'climbing'],
        CRZ: ['Cruise', 'level, en route'],
        APP: ['Approach', 'descending towards an airport'],
        ARR: ['Arrival', 'on the final part of its approach'],
        'T/D': ['Touchdown', 'landed'],
        UNK: ['Unknown', 'the phase detector could not tell'],
    };

    // The clearance types the grammar extracts (internal/transcription).
    const CLEARANCES = {
        takeoff: 'Cleared for takeoff',
        landing: 'Cleared to land',
        approach: 'Cleared for the approach',
    };

    function date(v) {
        if (!v) return null;
        const d = v instanceof Date ? v : new Date(v);
        return Number.isFinite(d.getTime()) ? d : null;
    }

    // A frequency the server no longer listens to: its ID, "126425", read as 126.425.
    function frequencyLabel(id) {
        const s = String(id);
        return /^\d{6}$/.test(s) ? `${s.slice(0, 3)}.${s.slice(3)}` : s;
    }

    function mhzKey(v) {
        const n = Number(v);
        return Number.isFinite(n) ? n.toFixed(3) : '';
    }

    /**
     * input: {
     *   aircraft,           // the store's aircraft: created_at, last_seen, status, phase, runway, clearances
     *   phases,             // the phase history if longer than aircraft.phase.history, else omitted
     *   transmissions,      // matched to its callsign, any order
     *   values,             // transcription id -> [{role, text}] (the grammar's)
     *   frequencies,        // [{id, name, frequency_mhz}] the server listens to
     * }
     */
    function build(input) {
        const a = input.aircraft || {};
        const items = [];
        const push = (at, item) => { const d = date(at); if (d) items.push(Object.assign({ at: d }, item)); };

        const freqById = new Map();
        const freqByMhz = new Map();
        for (const f of input.frequencies || []) {
            freqById.set(String(f.id), f);
            if (f.frequency_mhz) freqByMhz.set(mhzKey(f.frequency_mhz), f);
        }

        push(a.created_at, {
            kind: 'seen', icon: 'fa-eye', text: 'First seen by the receiver',
            help: 'When the station first decoded this aircraft\'s ADS-B messages in this database day.',
        });

        const history = (input.phases && input.phases.length) ? input.phases : ((a.phase && a.phase.history) || []);
        for (const p of history) {
            // NEW repeats "first seen", UNK says nothing: both would only crowd the story.
            if (p.phase === 'NEW' || p.phase === 'UNK') continue;
            const [name, what] = PHASES[p.phase] || [p.phase, ''];
            push(p.timestamp, {
                kind: 'phase', icon: p.phase === 'T/O' ? 'fa-plane-departure' : p.phase === 'T/D' ? 'fa-plane-arrival' : 'fa-chart-line',
                text: `${name}${p.airport ? ' at ' + p.airport : ''}`,
                help: `Flight phase ${p.phase}: ${what}. Judged by the server from altitude, speed and position.`,
                phase: p.phase, airport: p.airport || '', adsbId: p.adsb_id ?? null,
            });
        }

        const rw = a.runway || {};
        if (rw.departure) push(rw.departure.since, {
            kind: 'runway', icon: 'fa-road', text: `Departed runway ${rw.departure.runway} at ${rw.departure.airport}`,
            help: 'The runway judge: which runway the aircraft left, from its track over the runway ends.',
        });
        if (rw.go_around) push(rw.go_around.since, {
            kind: 'go_around', icon: 'fa-rotate-left', text: `Went around, runway ${rw.go_around.runway} at ${rw.go_around.airport}`,
            help: 'A go-around: the aircraft was landing, did not land, and climbed away to try again.',
        });
        if (rw.arrival) {
            const landed = a.on_ground || (a.date_landed && date(a.date_landed) >= date(rw.arrival.since));
            push(rw.arrival.since, {
                kind: 'runway', icon: 'fa-road', text: `${landed ? 'Landed on' : 'Lined up for'} runway ${rw.arrival.runway} at ${rw.arrival.airport}`,
                help: 'The runway judge: which runway the aircraft was aligned with on final, from its track.',
            });
        }

        for (const c of a.clearances || []) {
            const what = CLEARANCES[c.type] || 'Clearance';
            push(c.timestamp, {
                kind: 'clearance', icon: 'fa-clipboard-check',
                text: what + (c.runway ? `, runway ${c.runway}` : ''),
                quote: c.text || '',
                help: 'A clearance the controller gave, as recognised in a transmission.',
            });
        }

        for (const t of input.transmissions || []) {
            const f = freqById.get(String(t.frequency_id));
            const values = (input.values && input.values[t.id]) || [];
            push(t.created_at, {
                kind: 'transmission', icon: t.speaker_type === 'ATC' ? 'fa-headset' : t.speaker_type === 'PILOT' ? 'fa-user' : 'fa-comment',
                text: (t.speaker_type === 'ATC' ? 'Controller' : t.speaker_type === 'PILOT' ? 'Pilot' : 'Heard') +
                    (f ? ` on ${f.name}` : (t.frequency_id ? ` on ${frequencyLabel(t.frequency_id)}` : '')),
                transmission: t,
                values,
            });
            // A transfer: the controller gives it a new frequency.
            for (const v of values) {
                if (v.role !== 'frequency' || t.speaker_type === 'PILOT') continue;
                const to = freqByMhz.get(mhzKey(v.text));
                push(t.created_at, {
                    kind: 'transfer', icon: 'fa-right-left',
                    text: `Told to contact ${v.text}` + (to ? `, ${to.name}, which the station listens to` : ''),
                    frequency: v.text, toName: to ? to.name : '', transmission: t,
                    help: 'A transfer to another controller: the frequency was heard in the transmission. ' +
                        (to ? 'The station listens to it too, so the rest of the flight may be heard there.' : 'The station does not listen to it now.'),
                });
            }
        }

        if (a.status === 'signal_lost') push(a.last_seen, {
            kind: 'lost', icon: 'fa-signal', text: 'Signal lost',
            help: 'No more ADS-B messages from it: out of range of the antenna, or landed beyond it.',
        });

        // Oldest first; at the same second, the order of a flight: seen, phase,
        // runway, transmission, transfer, clearance.
        const order = { seen: 0, phase: 1, runway: 2, go_around: 2, transmission: 3, transfer: 4, clearance: 5, lost: 6 };
        items.sort((x, y) => (x.at - y.at) || ((order[x.kind] ?? 9) - (order[y.kind] ?? 9)));
        return items;
    }

    return { build, PHASES };
});
