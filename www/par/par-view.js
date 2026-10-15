/**
 * PAR view: the sky in profile, runway by runway, for one followed airport.
 * It takes the map's place in the main screen (the side panels stay), reads
 * the same store and applies the same filters. Geometry is in par-geometry.js;
 * design in docs-fr/31-vue-par.md.
 *
 * One frame per physical runway, the one in use first. A frame is an elevation
 * band (altitude against distance to the thresholds) over a thin azimuth band
 * (offset from the axis), sharing the distance axis, as on a Precision
 * Approach Radar scope. Arrivals of the runway in use always move left to
 * right, towards the runway; departures climb out on the right.
 */
(function () {
    'use strict';

    const G = window.ParGeometry;
    const SVG_NS = 'http://www.w3.org/2000/svg';

    // Same colours as the aircraft labels on the map (aircraft-webgl.js).
    // The five families of the design system (www/phase/phase-families.js,
    // migration step 2): the PAR paints an aircraft in its family's colour.
    const phaseColor = (code) => (window.PhaseFamilies ? window.PhaseFamilies.colorOf(code) : "#969696");
    const C = {
        grid: '#262626', gridStrong: '#3a3a3a', label: '#8a8a8a', text: '#e0e0e0',
        highlight: '#4CAF50', transition: '#FFC107', runway: '#bdbdbd', cone: '#4a4a4a',
    };

    let tertiary = '';
    function tertiaryText() {
        if (!tertiary) {
            tertiary = getComputedStyle(document.documentElement).getPropertyValue('--color-text-tertiary').trim() || '#949494';
        }
        return tertiary;
    }

    const ELEV_H_FULL = 230;     // elevation band height, px
    const AZ_H_FULL = 64;        // azimuth band height, px
    // The frame under the map while a flight is followed (step 8, c-suivre):
    // one runway, lower bands.
    const ELEV_H_SINGLE = 140;
    const AZ_H_SINGLE = 36;
    // The PAR view's frames fit the view's height, down to this elevation band
    // (owner, 14/10); below it they scroll.
    const ELEV_H_MIN = 80;
    const LEFT = 70;        // room for the level labels
    const RIGHT = 14;
    const TRAIL_SECONDS = 60;
    // The runway tracker counts an end as in use from this share of the
    // evidence (activeMinProbability in internal/adsb/runway_tracker.go): at
    // Paris-CDG one runway lands and its neighbour takes off.
    const IN_USE_MIN_PROBABILITY = 0.15;
    const STALE_SECONDS = 60;

    function esc(s) {
        return String(s).replace(/[&<>"]/g, (ch) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[ch]));
    }

    function pad3(n) {
        return String(Math.max(0, Math.round(n))).padStart(3, '0');
    }

    // The runway tracker names an end "07-25/25": its pair, then the end.
    function endIdent(score) {
        return String(score.runway_end || '').split('/').pop();
    }

    // Selected altitudes come in steps of 16 or 32 ft: 3000 is sent as 3008 or 3024.
    function roundedFt(ft) {
        return Math.round(ft / 100) * 100;
    }

    // --- Help on hover, for someone who knows nothing of air traffic control ---

    function helpAttr(title, body) {
        return ` data-help-title="${esc(title)}" data-help="${esc(body)}"`;
    }

    // "09L" is flown at about 090 degrees; L, R and C tell parallel runways apart.
    function aboutRunwayName(ident) {
        const n = parseInt(ident, 10);
        const side = { L: 'left', R: 'right', C: 'centre' }[ident.slice(-1)];
        return `${ident} means about ${n === 0 ? 360 : n * 10}° on the compass` +
            (side ? `, and ${ident.slice(-1)} the ${side}-hand one of parallel runways, as seen when landing` : '');
    }

    // Under this gap on the same final the sequence shows amber: 3 NM is the
    // usual minimum radar separation there (ICAO Doc 4444).
    const SEQUENCE_TIGHT_NM = 3;

    const HELP = {
        sequence: (ident) => helpAttr('Arrival sequence',
            `The aircraft coming in to runway ${ident} in this frame, nearest the threshold first: who lands next, how far, how long at its present ground speed, and how far behind the one ahead. A simple version of the arrival managers controllers use. Click a line to select the aircraft.`),
        seqDist: () => helpAttr('To threshold', 'Distance along the runway axis to the landing threshold, which may sit some way in from the end of the runway.'),
        seqEta: () => helpAttr('Time to the threshold', 'At the present ground speed, and the clock time it gives. Aircraft slow down on final, so the real time is usually a little later.'),
        seqGap: () => helpAttr('Spacing', 'How far behind the aircraft ahead, in distance and in time. Controllers keep at least 3 NM between aircraft on the same final, more behind a heavy one.'),
        tight: () => helpAttr('Close behind', 'Less than 3 NM behind the aircraft ahead, the usual minimum radar spacing on final. Often a projection effect while one of them turns onto the axis.'),
        view: () => helpAttr('PAR view',
            'The sky seen from the side, runway by runway, the way a Precision Approach Radar shows controllers the aircraft coming down to land. ' +
            'In each frame, height goes up the side and distance along the bottom; underneath, the same aircraft seen from above. ' +
            'The buttons at the top right switch between the map and the airports followed in the settings.'),
        inUse: (several) => helpAttr(several ? 'Runways in use' : 'Runway in use',
            'Aircraft take off and land into the wind, so an airport uses its runways in one direction at a time, and a large one may land on one runway while taking off from the next. ' +
            'co-atc works this out from the aircraft it has seen approach, land and take off in the last hour, each aircraft counted once, recent ones weighing more; the percentage is each runway\'s share. ' +
            'Frames with a runway in use come first.'),
        qnh: (est) => helpAttr('QNH, the local air pressure', est
            ? `An altimeter measures air pressure, not height. To read their height above sea level, crews set it to the local pressure, called the QNH. ` +
              `co-atc takes the setting that ${est.count} aircraft flying low near the airport report, and uses it to place every aircraft at its true height. The weather report (METAR) gives the same figure.`
            : 'An altimeter measures air pressure, not height; crews set it to the local pressure, the QNH, to read their height above sea level. ' +
              'No aircraft flying low near the airport reports its setting right now, so heights are read on the standard pressure and may be a few hundred feet off.'),
        ta: (ta) => helpAttr('Transition altitude',
            `Up to ${ta} ft, heights are counted above sea level, on the local pressure. Climbing past it, crews switch their altimeter to the standard pressure (1013 hPa) and use flight levels, ` +
            `so that everyone higher up shares one reference whatever the weather. ${ta} ft is the value published for Paris-Orly and Paris-Charles de Gaulle.`),
        tl: (tl, ta) => helpAttr('Transition level',
            `The lowest flight level in use, FL${pad3(tl)}: descending aircraft switch back to the local pressure as they pass it. ` +
            `It is at least 1000 ft above the transition altitude (${ta} ft), so it moves with the air pressure. This one is worked out from today's QNH; the controllers announce the actual one.`),
        layer: () => helpAttr('Transition layer',
            'The band between the transition altitude and the transition level. Aircraft cross it while they change their altimeter setting; none is told to level off inside it.'),
        runway: (a, b) => helpAttr(`Runway ${a} / ${b}`,
            `A runway is named after its direction, at each end: ${aboutRunwayName(a)}. ` +
            `Aircraft landing on ${a} arrive from the left and touch down at the ${a} end; aircraft taking off on ${a} climb out on the right, past the ${b} end. ` +
            'When the airport turns its runways round, the frame turns round too.'),
        fold: () => helpAttr('Fold', 'Click the bar to fold or unfold this runway.'),
        count: () => helpAttr('Aircraft in this frame',
            'The aircraft lined up with this runway, out to 20 NM, or on the runway itself. ' +
            'A filled symbol is an aircraft co-atc judges to be landing on this runway or to have taken off from it (the same runway as in its details panel); ' +
            'a hollow one is in the coverage without that. Others go with the nearest runway. The map\'s filters apply here too.'),
        scale: (ceil) => helpAttr('Height scale',
            'Heights are stretched near the ground, so that the last miles of a landing and the first of a take-off have room while the levels higher up still fit. ' +
            `Above ${ceil.toLocaleString('en')} ft, aircraft are not shown.`),
        feet: () => helpAttr('Height in feet',
            'Height above sea level, in feet, read on the local pressure (QNH). Used near the ground, below the transition altitude. 1000 ft is about 300 m.'),
        fl: (fl) => helpAttr(`Flight level ${pad3(fl)}`,
            `Higher up, heights are flight levels: hundreds of feet read on the standard pressure instead of the local one. ` +
            `FL${pad3(fl)} is about ${(fl * 100).toLocaleString('en')} ft; the line is drawn where that level actually lies today.`),
        glide: (e, strong) => helpAttr(`Glide path to runway ${e.ident}`,
            `The line an aircraft follows down to land on ${e.ident}: a ${e.slopeDeg}° slope, about ${Math.round(G.FT_PER_NM * Math.tan(e.slopeDeg * Math.PI / 180))} ft lower for each nautical mile, ` +
            `crossing the landing threshold ${e.thresholdCrossingFt} ft up. Instrument landing systems guide aircraft down it, and an arriving aircraft should ride on it.` +
            (strong ? ' Green: this runway is in use.' : '')),
        glideMargin: (tol) => helpAttr('Glide path margins',
            `${tol}° either side of the glide path: about how far above or below it the pilots' instrument can still show them. Between these lines, an aircraft is well set up to land.`),
        coneIn: (deg) => helpAttr('Edge of the coverage, inbound',
            `An aircraft flying towards the runway is shown with it while it is below this line, ${deg}° above the horizon seen from the runway end, as on a real approach radar. ` +
            'Higher, it is not yet lined up to land here.'),
        coneOut: (deg, ceil) => helpAttr('Edge of the coverage, outbound',
            `For aircraft flying away from the runway the limit is ${deg}°, since a take-off climbs much more steeply than a landing comes down. ` +
            `Above ${ceil.toLocaleString('en')} ft no aircraft is shown.`),
        runwayBar: (f) => helpAttr(`Runway ${f.le.ident}-${f.he.ident}`,
            `${Math.round(f.lengthNM * 1852).toLocaleString('en')} m long, drawn at its height above sea level, ${Math.round(Math.min(f.le.elevFt, f.he.elevFt))} to ${Math.round(Math.max(f.le.elevFt, f.he.elevFt))} ft.`),
        threshold: (e) => helpAttr(`Landing threshold ${e.ident}`, e.displacedFt > 0
            ? `Where aircraft landing on ${e.ident} may first touch down: ${Math.round(e.displacedFt * 0.3048)} m in from the end of the runway, a "displaced threshold", usually to keep clear of obstacles or reduce noise. The glide path aims here.`
            : `Where aircraft landing on ${e.ident} may first touch down, at the end of the runway. The glide path aims here.`),
        end: (e) => helpAttr(`Runway end ${e.ident}`,
            `${Math.round(e.elevFt)} ft above sea level. Aircraft landing on ${e.ident} fly towards ${pad3(e.landingBearing)}°.`),
        distance: () => helpAttr('Distance',
            'Nautical miles from the end of the runway; one nautical mile is 1852 m. On a 3° glide path, an arriving aircraft is about 1000 ft up 3 NM out.'),
        azimuth: () => helpAttr('Seen from above',
            'The same aircraft seen from above: how far each one is to the side of the runway\'s centre line, on the same distance scale. ' +
            'Up is to the left of an arriving aircraft. An aircraft lined up sits on the line.'),
        axis: (range) => helpAttr('Centre line', `The runway's centre line, extended ${range} NM out on each side.`),
        azMargin: (tol) => helpAttr('Lined up',
            `${tol}° either side of the centre line: about how far left or right of it the pilots' instrument can still show them. ` +
            'Outside, an aircraft is dimmed in the upper band: near the runway, but not lined up with it.'),
        coneSide: (deg, range) => helpAttr('Sides of the coverage',
            `${deg}° either side of the centre line, out to ${range} NM. Outside, an aircraft is not shown with this runway.`),
        track: () => helpAttr('Track of the selected aircraft',
            'Where it has been: its recorded positions, as far back as the track length set in the settings, drawn along this runway. ' +
            'Parts that lie outside this frame are left out.'),
    };

    function phaseOf(a) {
        return (a.phase && a.phase.current && a.phase.current.length) ? a.phase.current[0].phase : 'UNK';
    }

    // options.single: draw only the frame holding the followed aircraft, with a
    // cl-parframe head and "Open the PAR view" (the frame under the map, step 8).
    function create(container, store, options) {
        const single = !!(options && options.single);
        // The full view is the design system's ParView over the sky.
        if (!single) container.classList.add('cl-root', 'cl-parview');
        const runwaysByAirport = new Map();   // code -> Promise<RunwayInfo[]>
        const previousFrame = new Map();      // hex -> frame id, for the hysteresis
        const trails = new Map();             // hex -> [{t, lat, lon, altBaro}]
        let timer = null;
        let airport = '';
        // The elevation band's height that lets every open frame, with its
        // arrival tables, fit the view's height (owner, 14/10: adapt to what the
        // screen can show); measured after each render, used by the next.
        let fitElevH = null;
        let refitting = false;

        // A frame's fold, as chosen (key -> true folded, false open); without a
        // choice, a runway not in use is folded to its head (ParView, step 9e).
        let foldChoice = {};
        try {
            foldChoice = JSON.parse(localStorage.getItem('parFold') || 'null') || {};
            if (!localStorage.getItem('parFold')) {
                for (const k of JSON.parse(localStorage.getItem('parCollapsed') || '[]')) foldChoice[k] = true;
            }
        } catch (e) { /* none kept */ }
        const isFolded = (key, inUse) => (key in foldChoice ? foldChoice[key] : !inUse);

        // One tooltip for the whole view, outside the part redrawn every second.
        const tip = document.createElement('div');
        tip.className = 'par-tip';
        tip.style.display = 'none';
        document.body.appendChild(tip);
        let tipKey = null;
        let tipTimer = null;
        let tipX = 0;
        let tipY = 0;

        function placeTip() {
            const pad = 14;
            let left = tipX + pad;
            let top = tipY + pad;
            if (left + tip.offsetWidth > window.innerWidth - 8) left = tipX - pad - tip.offsetWidth;
            if (top + tip.offsetHeight > window.innerHeight - 8) top = tipY - pad - tip.offsetHeight;
            tip.style.left = Math.max(8, left) + 'px';
            tip.style.top = Math.max(8, top) + 'px';
        }

        function hideTip() {
            clearTimeout(tipTimer);
            tipTimer = null;
            tipKey = null;
            tip.style.display = 'none';
        }

        // Aircraft have no help of their own: clicking one opens its panel.
        container.addEventListener('mousemove', (ev) => {
            tipX = ev.clientX;
            tipY = ev.clientY;
            const el = ev.target.closest('[data-help]');
            if (!el || ev.target.closest('[data-hex]')) {
                hideTip();
                return;
            }
            const title = el.getAttribute('data-help-title') || '';
            const body = el.getAttribute('data-help') || '';
            const key = title + '\n' + body;
            if (key === tipKey) {
                if (tip.style.display !== 'none') placeTip();
                return;
            }
            hideTip();
            tipKey = key;
            tipTimer = setTimeout(() => {
                tip.innerHTML = `<div class="par-tip-title">${esc(title)}</div><div>${esc(body)}</div>`;
                tip.style.display = 'block';
                placeTip();
            }, 250);
        });
        container.addEventListener('mouseleave', hideTip);
        container.addEventListener('scroll', hideTip);

        container.addEventListener('click', (ev) => {
            hideTip();
            if (ev.target.closest('a[href]')) return;   // the VAC link opens its tab, nothing else
            if (ev.target.closest('[data-par-open]')) {
                store.setMainView('par', airport);
                return;
            }
            const toggle = ev.target.closest('[data-par-toggle]');
            if (toggle) {
                const key = toggle.getAttribute('data-par-toggle');
                foldChoice[key] = toggle.getAttribute('aria-expanded') === 'true';
                try { localStorage.setItem('parFold', JSON.stringify(foldChoice)); } catch (e) { /* kept for this page only */ }
                render();
                return;
            }
            const hit = ev.target.closest('[data-hex]');
            if (hit) {
                store.selectAircraftByHex(hit.getAttribute('data-hex'));
                return;
            }
            // A click beside the aircraft lets go of the selected one, as on the
            // map; not in the frame under the map, which is about that aircraft.
            if (store.selectedAircraft && !single) {
                store.selectedAircraft = null;
                const m = store.mapManager;
                if (m) {
                    if (typeof m.removeProximityCircle === 'function') m.removeProximityCircle();
                    if (typeof m.removeProximityHighlighting === 'function') m.removeProximityHighlighting();
                    if (typeof m.applyFiltersAndRefreshView === 'function') m.applyFiltersAndRefreshView({ immediate: true });
                }
                render();
            }
        });

        function runwaysFor(code) {
            if (!runwaysByAirport.has(code)) {
                const p = fetch(`/api/v1/airports/${encodeURIComponent(code)}`)
                    .then((r) => (r.ok ? r.json() : null))
                    .then((d) => (d && Array.isArray(d.runways) ? d.runways : []))
                    .catch(() => {
                        runwaysByAirport.delete(code);   // try again on the next tick
                        return [];
                    });
                runwaysByAirport.set(code, p);
                p.then(() => render());
            }
            return runwaysByAirport.get(code);
        }

        function visibleAircraft() {
            const rules = window.MapVisibilityRules;
            const now = Date.now();
            const lastSeenCutoff = store.settings && store.settings.lastSeenMinutes
                ? new Date(now - store.settings.lastSeenMinutes * 60000) : null;
            const out = [];
            for (const a of Object.values(store.aircraft || {})) {
                const t = a && a.adsb;
                if (!t || t.lat == null || t.lon == null) continue;
                if (a.last_seen && now - new Date(a.last_seen).getTime() > STALE_SECONDS * 1000) continue;
                if (rules && !rules.shouldShowAircraftOnMap(a, store, { searchTerm: store.searchTerm, lastSeenCutoff })) continue;
                out.push(a);
            }
            return out;
        }

        function recordTrails(list) {
            const now = Date.now();
            const seen = new Set();
            for (const a of list) {
                const t = a.adsb;
                seen.add(a.hex);
                let tr = trails.get(a.hex);
                if (!tr) trails.set(a.hex, (tr = []));
                const last = tr[tr.length - 1];
                if (!last || last.lat !== t.lat || last.lon !== t.lon || last.altBaro !== t.alt_baro) {
                    tr.push({ t: now, lat: t.lat, lon: t.lon, altBaro: typeof t.alt_baro === 'number' ? t.alt_baro : null });
                }
                while (tr.length && now - tr[0].t > TRAIL_SECONDS * 1000) tr.shift();
            }
            for (const hex of trails.keys()) if (!seen.has(hex)) trails.delete(hex);
        }

        // The end whose arrivals are drawn left to right: the end in use in this
        // frame, or else the one facing the same way as the runway in use.
        function arrivalEnd(frame, inUse) {
            const scored = inUse.filter((r) => r.end === frame.le.ident || r.end === frame.he.ident);
            if (scored.length) return scored[0].end === frame.le.ident ? 'le' : 'he';
            const top = inUse[0] && inUse[0].bearing;
            if (top == null) return 'le';
            return G.angleDiff(frame.le.landingBearing, top) <= 90 ? 'le' : 'he';
        }

        function render() {
            if (!timer && container.dataset.parActive !== '1') return;
            const airports = store.followedAirports || [];
            const apt = airports.find((a) => a.code === airport) || airports[0];
            if (!apt) {
                container.innerHTML = message('No airport followed. Choose one in the settings panel.');
                return;
            }
            const cached = runwaysByAirport.get(apt.code);
            if (!cached) {
                runwaysFor(apt.code);
                container.innerHTML = message(`Loading the runways of ${esc(apt.code)}…`);
                return;
            }
            cached.then((runways) => draw(apt, runways));
        }

        function message(text) {
            return `<div class="par-message">${text}</div>`;
        }

        // The bands for an elevation height: the azimuth band in proportion, the
        // empty margin under it only at full height.
        function bandsFor(elevH) {
            if (!Number.isFinite(elevH) || elevH >= ELEV_H_FULL) return {};
            return { elevH, azH: Math.max(24, Math.round(elevH * AZ_H_FULL / ELEV_H_FULL)), bottomPad: 6 };
        }

        // After a render: the elevation height at which the open frames fill
        // the view's height without scrolling, between ELEV_H_MIN and full.
        // Drawn again at once when it moved, then kept for the next renders.
        function fit(openCount) {
            const svgs = [...container.querySelectorAll('.cl-parframe svg.par-svg')];
            if (!openCount || !svgs.length || refitting) return;
            const vb = svgs[0].viewBox && svgs[0].viewBox.baseVal;
            const shown = svgs[0].getBoundingClientRect();
            if (!vb || !vb.height || !shown.height) return;
            const scale = shown.height / vb.height;
            const svgTotal = svgs.reduce((sum, el) => sum + el.getBoundingClientRect().height, 0);
            const other = container.scrollHeight - svgTotal;
            const perFrame = (container.clientHeight - other) / svgs.length / scale;
            const cur = bandsFor(fitElevH);
            const fixed = vb.height - (cur.elevH || ELEV_H_FULL) - (cur.azH || AZ_H_FULL) - (cur.bottomPad != null ? 6 : 20);
            const ratio = 1 + AZ_H_FULL / ELEV_H_FULL;
            let next = Math.floor((perFrame - fixed - 6) / ratio);
            next = Math.max(ELEV_H_MIN, Math.min(ELEV_H_FULL, next));
            if (fitElevH !== null && Math.abs(next - fitElevH) < 4) return;
            if (fitElevH === null && next >= ELEV_H_FULL) { fitElevH = ELEV_H_FULL; return; }
            fitElevH = next;
            refitting = true;
            try { render(); } finally { refitting = false; }
        }

        function draw(apt, runways) {
            // The placement is shared with the store's arrival sequence
            // (par-arrivals.js, 13/10): one logic, drawn here, listed there.
            const placement = window.ParArrivals.place(G, store, apt, runways, { previousFrame },
                { list: visibleAircraft(), inUseMinProbability: IN_USE_MIN_PROBABILITY, staleSeconds: STALE_SECONDS });
            if (!placement) {
                container.innerHTML = message(`${esc(apt.code)} has no runway with both ends located.`);
                return;
            }
            const { opts, frames, inUse, list, est, qnh, tl, placed } = placement;
            recordTrails(list);
            if (single) {
                drawSingle(apt, placement);
                return;
            }

            // Frames carrying a runway in use come first, in the order of the
            // runway names, not of their scores: at De Gaulle 08R and 09L land
            // half the traffic each, the first of them changed 18 times in a
            // night, and the two frames kept swapping places on the screen.
            const usage = (f) => Math.max(0, ...inUse.filter((r) => r.end === f.le.ident || r.end === f.he.ident).map((r) => r.score));
            const ordered = frames.map((f) => ({ f, use: usage(f) > 0 }))
                .sort((p, q) => (q.use - p.use) || p.f.id.localeCompare(q.f.id))
                .map((o) => o.f);

            const width = Math.max(480, container.clientWidth - 34);
            // The SIA's chart, in a new tab: its terms allow a link, not a frame.
            const vac = window.ParVac ? window.ParVac.vacLink(apt.code, new Date()) : null;
            const vacLink = vac
                ? `<a class="cl-link" href="${esc(vac.url)}" target="_blank" rel="noopener"${helpAttr('VAC',
                    `Visual approach chart of ${apt.code}, from the SIA, AIRAC cycle of ${vac.cycle}. Opens in a new tab.`)}>VAC</a>`
                : '';
            // ParHead (step 9e): the words in text, the values in mono.
            const header = `
                <div class="cl-parhead">
                    <span class="cl-parhead__airport"${HELP.view()}>${esc(apt.code)}</span>
                    <span${HELP.view()}>${esc(window.RightNow ? window.RightNow.shortName(apt) : (apt.name || ''))}</span>
                    ${vacLink}
                    <span class="cl-parhead__sep">·</span>
                    <span${HELP.inUse(inUse.length > 1)}>${inUse.length > 1 ? 'Runways' : 'Runway'} in use ${inUse.length
                        ? inUse.map((r) => `<b>${esc(r.end)}</b> ${Math.round(r.probability * 100)}\u202f%`).join(' · ')
                        : '<span class="cl-age">unknown</span>'}</span>
                    <span class="cl-parhead__sep">·</span>
                    <span${HELP.qnh(est)}>QNH <b>${est ? Math.round(qnh) : '—'}</b> hPa <span class="cl-age">${est ? `(ADS-B, ${est.count} aircraft)` : '(standard)'}</span></span>
                    <span class="cl-parhead__sep">·</span>
                    <span${HELP.ta(opts.transitionAltitudeFt)}>TA <b>${opts.transitionAltitudeFt}</b> ft</span>
                    <span class="cl-parhead__sep">·</span>
                    <span${HELP.tl(tl, opts.transitionAltitudeFt)}>TL <b>FL${pad3(tl)}</b> <span class="cl-age">(computed)</span></span>
                </div>`;

            const selected = store.selectedAircraft && store.selectedAircraft.hex;
            // Two runways open or more: each frame is drawn low, as under the
            // map, so that they are seen together on one screen; one alone keeps
            // the full height (owner, 14/10).
            const openCount = ordered.filter((f) => !isFolded(`${apt.code} ${f.id}`, usage(f) > 0)).length;
            const compact = bandsFor(fitElevH);
            const body = ordered.map((f) => {
                const key = `${apt.code} ${f.id}`;
                const isInUse = usage(f) > 0;
                const items = placed.get(f.id);
                const A = arrivalEnd(f, inUse);
                const a = f[A].ident;
                const b = f[A === 'le' ? 'he' : 'le'].ident;
                const folded = isFolded(key, isInUse);
                const head = `
                    <div class="cl-parframe__head" style="cursor: pointer;" data-par-toggle="${esc(key)}" role="button" aria-expanded="${folded ? 'false' : 'true'}">
                        <span class="cl-age"${HELP.fold()}>${folded ? '▸' : '▾'}</span>
                        <b${HELP.runway(a, b)}>${esc(a + ' → ' + b)}</b>
                        ${isInUse ? `<span class="cl-parframe__inuse"${HELP.inUse(inUse.length > 1)}>IN USE</span>` : ''}
                        <span class="cl-age"${HELP.count()}>${items.length} aircraft</span>
                    </div>`;
                const cls = `cl-parframe${isInUse ? ' is-in-use' : ''}`;
                if (folded) return `<section class="${cls}">${head}</section>`;
                return `<section class="${cls}">${head}${frameSvg(f, A, items, { width, qnh, tl, opts, selected, isInUse, ...compact })}${sequenceHtml(f, A, items, selected, true)}${sequenceHtml(f, A === 'le' ? 'he' : 'le', items, selected, false)}</section>`;
            }).join('');

            container.innerHTML = header + body;
            fit(openCount);
        }

        // The frame holding the followed aircraft, alone; nothing when it is in
        // none of them.
        function drawSingle(apt, placement) {
            const { opts, frames, inUse, qnh, tl, placed } = placement;
            const selected = store.selectedAircraft && store.selectedAircraft.hex;
            // Its frame; before it is in a cone, the runway in use whose extended
            // axis passes closest to it (a De Gaulle doublet: an aircraft north
            // goes to 27R even when 26L scores higher), the score only breaking
            // a tie; with no runway in use, the closest axis of all.
            const holding = selected && frames.find((fr) => (placed.get(fr.id) || []).some((it) => it.a.hex === selected));
            const usage = (fr) => Math.max(0, ...inUse.filter((r) => r.end === fr.le.ident || r.end === fr.he.ident).map((r) => r.score));
            const pos = store.selectedAircraft && store.selectedAircraft.adsb;
            const off = (fr) => (pos && pos.lat != null && pos.lon != null ? Math.abs(G.project(fr, pos.lat, pos.lon).c) : 0);
            const used = frames.filter((fr) => usage(fr) > 0);
            const f = holding || (used.length ? used : frames).slice()
                .sort((p, q) => (off(p) - off(q)) || (usage(q) - usage(p)))[0];
            if (!f) {
                container.innerHTML = '';
                return;
            }
            const sel = store.selectedAircraft;
            const away = holding ? '' : `<span class="cl-age"${helpAttr('Off the axis',
                'The aircraft is not in this runway\'s approach cone. It appears in the frame if it turns onto this runway\'s final approach.')}>` +
                `${esc(((sel && sel.flight) || '').trim() || selected)} off the axis</span>`;
            const items = placed.get(f.id);
            const isInUse = inUse.some((r) => r.end === f.le.ident || r.end === f.he.ident);
            const A = arrivalEnd(f, inUse);
            const a = f[A].ident;
            const b = f[A === 'le' ? 'he' : 'le'].ident;
            const width = Math.max(480, container.clientWidth);
            container.innerHTML =
                `<div class="cl-parframe__head"><b${HELP.runway(a, b)}>${esc(a + ' → ' + b)}</b>` +
                (isInUse ? `<span class="cl-parframe__inuse"${HELP.inUse(inUse.length > 1)}>IN USE</span>` : '') +
                `<span class="cl-age"${HELP.count()}>${items.length} aircraft</span>` + away +
                `<button class="cl-btn" style="margin-left: auto;" data-par-open>Open the PAR view</button></div>` +
                frameSvg(f, A, items, { width, qnh, tl, opts, selected, isInUse, elevH: ELEV_H_SINGLE, azH: AZ_H_SINGLE });
        }

        // --- The arrival sequence under a frame (audit 3.4) ---------------------

        // Who lands next on this frame's arrival end: distance to the threshold,
        // time to it at the present ground speed, and the gap to the one ahead.
        // The other end is shown too when aircraft come in to it: the runway in
        // use is not always known, and a contrary-flow arrival is worth seeing.
        function sequenceHtml(f, A, items, selected, always) {
            const seq = G.arrivalSequence(f, A, items);
            const ident = f[A].ident;
            const title = `<span class="cl-label"${HELP.sequence(ident)}>Arrivals to ${esc(ident)}</span>`;
            if (!seq.length) {
                return always ? `<div class="cl-section" style="border: 0; padding: 0;">${title}<span class="cl-plain">None in the approach cone.</span></div>` : '';
            }
            const mmss = (sec) => `${Math.floor(sec / 60)}:${String(Math.round(sec % 60)).padStart(2, '0')}`;
            // ArrivalSequence (step 9e): the table of Right now, here under its frame.
            const rows = seq.map((x, i) => {
                const a = x.a;
                const cs = esc(((a.flight || '').trim()) || a.hex.toUpperCase());
                const type = esc((a.adsb && a.adsb.t) || (a.bsdb && a.bsdb.icao_type_code) || '');
                const eta = x.etaSec != null
                    ? `<span title="${esc(store.formatDate(new Date(Date.now() + x.etaSec * 1000), true))}">${mmss(x.etaSec)}</span>`
                    : '—';
                const tight = x.gapNM != null && x.gapNM < SEQUENCE_TIGHT_NM;
                const gap = x.gapNM == null ? 'first' : `+${x.gapNM.toFixed(1)} NM`;
                const sel = a.hex === selected ? ' style="background: var(--color-surface-selected); cursor: pointer;"' : ' style="cursor: pointer;"';
                return `<tr data-hex="${esc(a.hex)}"${sel}>` +
                    `<td class="is-dim">${i + 1}</td><td><b>${cs}</b> <span class="is-dim">${type}</span></td>` +
                    `<td class="is-num">${x.distNM.toFixed(1)} NM</td><td class="is-num">${eta}</td>` +
                    `<td class="is-num${x.gapNM == null ? ' is-dim' : tight ? ' is-tight' : ''}"${tight ? HELP.tight() : ''}>${gap}</td></tr>`;
            }).join('');
            return `<div class="cl-section" style="border: 0; padding: 0; gap: var(--space-1);">${title}<table class="cl-table" style="margin-top: 0;"><thead><tr>` +
                `<th></th><th>Aircraft</th><th class="is-num"${HELP.seqDist()}>To thr.</th><th class="is-num"${HELP.seqEta()}>Time</th><th class="is-num"${HELP.seqGap()}>Behind</th>` +
                `</tr></thead><tbody>${rows}</tbody></table></div>`;
        }

        // --- One frame --------------------------------------------------------

        function frameSvg(f, A, items, ctx) {
            const { width, qnh, tl, opts, selected, isInUse } = ctx;
            const ELEV_H = ctx.elevH || ELEV_H_FULL;
            const AZ_H = ctx.azH || AZ_H_FULL;
            const R = opts.rangeNM;
            const L = f.lengthNM;
            const B = A === 'le' ? 'he' : 'le';
            const plotW = width - LEFT - RIGHT;
            const ground = Math.min(f.le.elevFt, f.he.elevFt);
            const ceil = opts.ceilingFt;
            const elevTop = 18;
            const azTop = elevTop + ELEV_H + 22;
            const height = azTop + AZ_H + (Number.isFinite(ctx.bottomPad) ? ctx.bottomPad : 20);

            // Display coordinate u: arrivals to A come from u < 0; the runway is [0, L].
            const u = (s) => (A === 'le' ? s : L - s);
            const x = (uu) => LEFT + (uu + R) / (L + 2 * R) * plotW;
            // Square-root altitude scale: the last miles and the climb-out get
            // room, and the flight levels up to the ceiling still fit.
            const y = (alt) => {
                const h = Math.max(0, Math.min(1, (alt - ground) / (ceil - ground)));
                return elevTop + ELEV_H * (1 - Math.sqrt(h));
            };
            const azHalf = R * Math.tan(opts.halfWidthDeg * Math.PI / 180);
            const azMid = azTop + AZ_H / 2;
            // Up in the azimuth band is left of the arrivals' course.
            const yAz = (c) => azMid - Math.max(-1, Math.min(1, (A === 'le' ? -c : c) / azHalf)) * (AZ_H / 2);
            const flAlt = (fl) => G.qnhAltitude(fl * 100, qnh);

            const out = [];
            const pts = (list) => list.map((p) => p[0].toFixed(1) + ',' + p[1].toFixed(1)).join(' ');
            const line = (x1, y1, x2, y2, stroke, extra = '') =>
                out.push(`<line x1="${x1.toFixed(1)}" y1="${y1.toFixed(1)}" x2="${x2.toFixed(1)}" y2="${y2.toFixed(1)}" stroke="${stroke}" ${extra}/>`);
            const text = (tx, ty, s, fill, extra = '') =>
                out.push(`<text x="${tx.toFixed(1)}" y="${ty.toFixed(1)}" fill="${fill}" ${extra}>${s}</text>`);
            const poly = (list, stroke, extra = '') => {
                if (list.length < 2) return;
                out.push(`<polyline fill="none" stroke="${stroke}" points="${pts(list)}" ${extra}/>`);
            };
            // Thin lines are hard to point at: each one that has something to say
            // gets a wider invisible twin carrying its help.
            const hitLine = (x1, y1, x2, y2, help) =>
                out.push(`<line x1="${x1.toFixed(1)}" y1="${y1.toFixed(1)}" x2="${x2.toFixed(1)}" y2="${y2.toFixed(1)}" stroke="transparent" stroke-width="10" pointer-events="stroke"${help}/>`);
            const hitPoly = (list, help) => {
                if (list.length < 2) return;
                out.push(`<polyline fill="none" stroke="transparent" stroke-width="10" pointer-events="stroke" points="${pts(list)}"${help}/>`);
            };

            // The level scale's column, then the transition layer.
            out.push(`<rect x="0" y="${elevTop}" width="${LEFT}" height="${ELEV_H}" fill="transparent"${HELP.scale(ceil)}/>`);
            const ta = opts.transitionAltitudeFt;
            out.push(`<rect x="${LEFT}" y="${y(flAlt(tl)).toFixed(1)}" width="${plotW}" height="${(y(ta) - y(flAlt(tl))).toFixed(1)}" fill="${C.transition}" fill-opacity="0.06"${HELP.layer()}/>`);

            // The level grid: feet QNH below the transition altitude, flight
            // levels from the transition level up. A label is left out when it
            // would touch the one below it.
            let lastLabelY = Infinity;
            const levelLabel = (alt, s, fill, help, extra = '') => {
                if (lastLabelY - y(alt) < 11) return;
                lastLabelY = y(alt);
                text(LEFT - 6, y(alt) + 3, s, fill, `text-anchor="end" ${extra}${help}`);
            };
            for (let alt = 1000; alt < ta; alt += 1000) {
                if (alt <= ground) continue;
                line(LEFT, y(alt), LEFT + plotW, y(alt), C.grid);
                levelLabel(alt, `${alt}`, C.label, HELP.feet());
            }
            line(LEFT, y(ta), LEFT + plotW, y(ta), C.transition, 'stroke-opacity="0.5" stroke-dasharray="4 3"');
            hitLine(LEFT, y(ta), LEFT + plotW, y(ta), HELP.ta(ta));
            levelLabel(ta, `TA ${ta}`, C.transition, HELP.ta(ta), 'fill-opacity="0.8"');
            for (let fl = tl; flAlt(fl) <= ceil + 1; fl += 10) {
                const first = fl === tl;
                line(LEFT, y(flAlt(fl)), LEFT + plotW, y(flAlt(fl)), first ? C.transition : C.grid,
                    first ? 'stroke-opacity="0.5" stroke-dasharray="4 3"' : '');
                if (first) hitLine(LEFT, y(flAlt(fl)), LEFT + plotW, y(flAlt(fl)), HELP.tl(tl, ta));
                levelLabel(flAlt(fl), `FL${pad3(fl)}`, first ? C.transition : C.label,
                    first ? HELP.tl(tl, ta) : HELP.fl(fl), first ? 'fill-opacity="0.8"' : '');
            }

            // The cones' upper edges: 7 degrees towards the runway on both sides,
            // and 15 for aircraft leaving it.
            const coneEdge = (endKey, deg, extra, help) => {
                const e = f[endKey];
                const list = [];
                for (let d = 0; d <= R + 0.01; d += 0.25) {
                    const alt = e.elevFt + d * G.FT_PER_NM * Math.tan(deg * Math.PI / 180);
                    if (alt > ceil) break;
                    const uu = endKey === A ? -d : L + d;
                    list.push([x(uu), y(alt)]);
                }
                poly(list, C.cone, extra);
                hitPoly(list, help);
            };
            for (const endKey of [A, B]) {
                coneEdge(endKey, opts.elevApproachDeg, 'stroke-dasharray="2 4"', HELP.coneIn(opts.elevApproachDeg));
                coneEdge(endKey, opts.elevDepartDeg, 'stroke-dasharray="1 6"', HELP.coneOut(opts.elevDepartDeg, ceil));
            }

            // Glide paths, with their margins: the one of the end in use stands out.
            const glide = (endKey, strong) => {
                const e = f[endKey];
                const path = [], lo = [], hi = [];
                for (let d = 0; d <= R + 0.01; d += 0.25) {
                    const s = endKey === 'le' ? e.thresholdS - d : e.thresholdS + d;
                    const uu = u(s);
                    if (uu < -R || uu > L + R) break;
                    const run = d * G.FT_PER_NM;
                    const base = e.elevFt + e.thresholdCrossingFt;
                    path.push([x(uu), y(base + run * Math.tan(e.slopeDeg * Math.PI / 180))]);
                    lo.push([x(uu), y(base + run * Math.tan((e.slopeDeg - opts.elevToleranceDeg) * Math.PI / 180))]);
                    hi.push([x(uu), y(base + run * Math.tan((e.slopeDeg + opts.elevToleranceDeg) * Math.PI / 180))]);
                }
                const col = strong ? C.highlight : '#555';
                poly(lo, col, 'stroke-opacity="0.25"');
                poly(hi, col, 'stroke-opacity="0.25"');
                hitPoly(lo, HELP.glideMargin(opts.elevToleranceDeg));
                hitPoly(hi, HELP.glideMargin(opts.elevToleranceDeg));
                poly(path, col, `stroke-width="${strong ? 1.5 : 1}" stroke-dasharray="6 4"`);
                hitPoly(path, HELP.glide(e, strong));
                const at = path[Math.min(path.length - 1, 16)];
                // The slope is read, not drawn: its figure takes the
                // tertiary text colour, where the line's #555 gave 2.7:1
                // (step 13 contrast probe).
                text(at[0], at[1] - 5, `${e.slopeDeg}°`, strong ? col : tertiaryText(),
                    `text-anchor="middle" font-size="10"${HELP.glide(e, strong)}`);
            };
            glide(B, false);
            glide(A, isInUse);

            // Ground, runway, thresholds and the distance scale.
            const gy = y(ground);
            line(LEFT, gy, LEFT + plotW, gy, C.gridStrong);
            out.push(`<rect x="${x(0).toFixed(1)}" y="${(gy - 3).toFixed(1)}" width="${(x(L) - x(0)).toFixed(1)}" height="5" fill="${C.runway}" rx="1"${HELP.runwayBar(f)}/>`);
            for (const endKey of ['le', 'he']) {
                const e = f[endKey];
                const tx = x(u(e.thresholdS));
                line(tx, gy - 7, tx, gy + 4, C.text, 'stroke-width="1.5"');
                hitLine(tx, gy - 7, tx, gy + 4, HELP.threshold(e));
                const ex = x(u(endKey === 'le' ? 0 : L));
                text(ex, gy + 15, esc(e.ident), C.text, `text-anchor="${endKey === A ? 'end' : 'start'}" font-weight="bold"${HELP.end(e)}`);
            }
            for (let d = 5; d <= R; d += 5) {
                for (const uu of [-d, L + d]) {
                    line(x(uu), gy, x(uu), gy + 4, C.label);
                    text(x(uu), gy + 15, `${d}`, C.label, `text-anchor="middle" font-size="10"${HELP.distance()}`);
                }
            }
            text(x(-R) + 2, gy + 15, 'NM', C.label, `font-size="10"${HELP.distance()}`);

            // Azimuth band: the axis, the approach margins and the cones' sides.
            out.push(`<rect x="${LEFT}" y="${azTop}" width="${plotW}" height="${AZ_H}" fill="#111" stroke="${C.grid}"${HELP.azimuth()}/>`);
            line(LEFT, azMid, LEFT + plotW, azMid, C.gridStrong);
            hitLine(LEFT, azMid, LEFT + plotW, azMid, HELP.axis(R));
            out.push(`<rect x="${x(0).toFixed(1)}" y="${(azMid - 2).toFixed(1)}" width="${(x(L) - x(0)).toFixed(1)}" height="4" fill="${C.runway}" rx="1"${HELP.runwayBar(f)}/>`);
            for (const endKey of ['le', 'he']) {
                // Both wedges are symmetric about the axis: the approach
                // margin from the threshold, the cone's sides from the end.
                const u0 = u(f[endKey].thresholdS);
                const u1 = endKey === A ? -R : L + R;
                const tol = Math.abs(u1 - u0) * Math.tan(opts.azToleranceDeg * Math.PI / 180);
                const col = endKey === A && isInUse ? C.highlight : '#555';
                for (const side of [tol, -tol]) {
                    line(x(u0), azMid, x(u1), yAz(side), col, 'stroke-opacity="0.35"');
                    hitLine(x(u0), azMid, x(u1), yAz(side), HELP.azMargin(opts.azToleranceDeg));
                }
                const uEnd = endKey === A ? 0 : L;
                for (const side of [azHalf, -azHalf]) {
                    line(x(uEnd), azMid, x(u1), yAz(side), C.cone, 'stroke-dasharray="2 4"');
                    hitLine(x(uEnd), azMid, x(u1), yAz(side), HELP.coneSide(opts.halfWidthDeg, R));
                }
            }
            text(LEFT - 6, azMid + 3, 'AZ', C.label, `text-anchor="end"${HELP.azimuth()}`);

            // The selected aircraft's whole recorded track, in segments where
            // it lies within the frame; then the short trails; aircraft on top.
            const sel = items.find((it) => it.a.hex === selected);
            const history = sel && store.selectedAircraft && store.selectedAircraft.hex === selected
                ? (store.aircraftDetailsHistoryData || []) : [];
            if (history.length) {
                const segE = [[]], segA = [[]];
                for (const pt of [...history].reverse()) {
                    if (pt.lat == null || pt.lon == null || pt.altitude == null) continue;
                    const pr = G.project(f, pt.lat, pt.lon);
                    const uu = u(pr.s);
                    if (uu < -R || uu > L + R) {
                        if (segE[segE.length - 1].length) { segE.push([]); segA.push([]); }
                        continue;
                    }
                    const alt = pt.altitude <= 0 ? ground : G.qnhAltitude(pt.altitude, qnh);
                    segE[segE.length - 1].push([x(uu), y(alt)]);
                    segA[segA.length - 1].push([x(uu), yAz(pr.c)]);
                }
                for (const seg of [...segE, ...segA]) {
                    poly(seg, '#ffffff', 'stroke-opacity="0.75" stroke-width="1.5"');
                    hitPoly(seg, HELP.track());
                    for (const q of seg) out.push(`<circle cx="${q[0].toFixed(1)}" cy="${q[1].toFixed(1)}" r="1.4" fill="#ffffff" fill-opacity="0.75"/>`);
                }
            }
            for (const it of items) {
                const tr = trails.get(it.a.hex) || [];
                const pe = [], pa = [];
                for (const pt of tr) {
                    if (pt.altBaro == null && !it.a.on_ground) continue;
                    const pr = G.project(f, pt.lat, pt.lon);
                    const uu = u(pr.s);
                    if (uu < -R || uu > L + R) continue;
                    const alt = it.a.on_ground ? ground : G.qnhAltitude(pt.altBaro, qnh);
                    pe.push([x(uu), y(alt)]);
                    pa.push([x(uu), yAz(pr.c)]);
                }
                const col = phaseColor(phaseOf(it.a));
                poly(pe, col, 'stroke-opacity="0.35"');
                poly(pa, col, 'stroke-opacity="0.35"');
            }
            for (const it of items) out.push(aircraftGlyph(f, A, it, { x, y, yAz, u, ground, qnh, tl, opts, selected }));

            return `<svg xmlns="${SVG_NS}" class="par-svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}">${out.join('')}</svg>`;
        }

        function aircraftGlyph(f, A, it, g) {
            const { a, p } = it;
            const t = a.adsb;
            const uu = g.u(p.s);
            const px = g.x(uu);
            const alt = a.on_ground ? g.ground : it.altFt;
            const py = g.y(alt);
            const ay = g.yAz(p.c);
            const col = phaseColor(phaseOf(a));
            // Faded when off the axis: in the cone, but not lined up.
            const off = p.side !== 'rwy' && Math.abs(p.azimuthDeg) > g.opts.azToleranceDeg;
            const sel = a.hex === g.selected;
            const opacity = off && !sel ? 0.4 : 1;

            // Which way it moves along the display axis.
            const rightward = f.bearing + (A === 'le' ? 0 : 180);
            const goingRight = Number.isFinite(t.track) ? G.angleDiff(t.track, rightward) < 90 : true;
            // Filled: judged landing on or taking off from this runway; hollow:
            // in its coverage, not judged on it.
            const fillAttr = p.judged ? `fill="${col}"` : `fill="#0a0a0a" stroke="${col}" stroke-width="1.5"`;
            const tri = goingRight
                ? `${px + 6},${py} ${px - 4},${py - 4} ${px - 4},${py + 4}`
                : `${px - 6},${py} ${px + 4},${py - 4} ${px + 4},${py + 4}`;

            const callsign = (a.flight || t.flight || a.hex || '').trim();
            let level = '';
            if (a.on_ground) level = 'GND';
            else if (t.alt_baro >= g.tl * 100) level = 'FL' + pad3(t.alt_baro / 100);
            else level = roundedFt(alt) + ' ft';
            const rate = t.baro_rate > 300 ? ' ↑' : (t.baro_rate < -300 ? ' ↓' : '');

            // The level the crew has selected, when it differs from the current one.
            let target = '';
            let targetMark = '';
            if (!a.on_ground && typeof t.nav_altitude_mcp === 'number' && t.nav_altitude_mcp > 0) {
                const mcp = t.nav_altitude_mcp;
                const isFL = mcp >= g.tl * 100;
                const tAlt = isFL ? G.qnhAltitude(mcp, g.qnh) : mcp;
                if (Math.abs(tAlt - alt) > 250) {
                    target = ' → ' + (isFL ? 'FL' + pad3(mcp / 100) : roundedFt(mcp) + ' ft');
                    const ty = g.y(tAlt);
                    targetMark = `<line x1="${px}" y1="${py}" x2="${px}" y2="${ty.toFixed(1)}" stroke="${col}" stroke-opacity="0.45" stroke-dasharray="2 3"/>` +
                        `<line x1="${px - 5}" y1="${ty.toFixed(1)}" x2="${px + 5}" y2="${ty.toFixed(1)}" stroke="${col}" stroke-width="2"/>`;
                }
            }

            const role = p.judged === 'arrival' ? ` · landing on ${p.runway}` : p.judged === 'departure' ? ` · took off from ${p.runway}` : ' · not judged on this runway';
            const title = `${callsign} · ${phaseOf(a)} · ${level}${rate}${target}${role}` +
                (p.side === 'rwy' ? ' · on the runway' : ` · ${p.dist.toFixed(1)} NM from ${esc(f[p.side].ident)}, ${p.elevationDeg.toFixed(1)}° up, ${Math.abs(p.azimuthDeg).toFixed(1)}° off the axis`);
            const labelX = goingRight ? px - 8 : px + 8;
            const anchor = goingRight ? 'end' : 'start';
            return `<g class="par-ac" data-hex="${esc(a.hex)}" opacity="${opacity}">` +
                `<title>${esc(title)}</title>` +
                // A moving 10-pixel triangle is hard to click: a wider, invisible target.
                `<circle cx="${px}" cy="${py}" r="11" fill="transparent"/>` +
                `<circle cx="${px}" cy="${ay}" r="9" fill="transparent"/>` +
                targetMark +
                (sel ? `<circle cx="${px}" cy="${py}" r="9" fill="none" stroke="#fff" stroke-width="1.5"/>` : '') +
                `<polygon points="${tri}" ${fillAttr}/>` +
                `<text x="${labelX}" y="${py - 7}" fill="${col}" text-anchor="${anchor}" font-weight="bold">${esc(callsign)}</text>` +
                `<text x="${labelX}" y="${py + 13}" fill="${C.text}" text-anchor="${anchor}" font-size="10">${esc(level + rate + target)}</text>` +
                (sel ? `<circle cx="${px}" cy="${ay}" r="7" fill="none" stroke="#fff" stroke-width="1.5"/>` : '') +
                `<circle cx="${px}" cy="${ay}" r="3.5" fill="${col}"/>` +
                `</g>`;
        }

        // The frames take their width from the container at each render. At the
        // first show() the container is still hidden (width 0, so the 480 px
        // floor), and a background tab's timer is throttled: the frames stayed
        // at 480 px in a 949 px column (seen by the owner on 12/10). A render on
        // the next frame, once shown, and one at every resize of the container.
        const resizer = typeof ResizeObserver === 'function'
            ? new ResizeObserver(() => { if (container.dataset.parActive === '1') render(); })
            : null;

        return {
            show(code) {
                airport = code || airport;
                container.dataset.parActive = '1';
                render();
                requestAnimationFrame(() => { if (container.dataset.parActive === '1') render(); });
                if (resizer) resizer.observe(container);
                if (!timer) timer = setInterval(render, 1000);
            },
            hide() {
                hideTip();
                container.dataset.parActive = '0';
                if (resizer) resizer.unobserve(container);
                if (timer) clearInterval(timer);
                timer = null;
            },
            refresh: render,
        };
    }

    window.ParView = { create };
})();
