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
    const PHASE_COLORS = {
        'NEW': '#9CA3AF', 'TAX': '#C084FC', 'T/O': '#FB923C', 'CLB': '#A3E635',
        'DEP': '#4ADE80', 'CRZ': '#60A5FA', 'ARR': '#F9A8D4', 'APP': '#FACC15',
        'T/D': '#2DD4BF', 'UNK': '#94A3B8',
    };
    const C = {
        grid: '#262626', gridStrong: '#3a3a3a', label: '#8a8a8a', text: '#e0e0e0',
        highlight: '#4CAF50', transition: '#FFC107', runway: '#bdbdbd', cone: '#4a4a4a',
    };

    const ELEV_H = 230;     // elevation band height, px
    const AZ_H = 64;        // azimuth band height, px
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

    const HELP = {
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

    function create(container, store) {
        const runwaysByAirport = new Map();   // code -> Promise<RunwayInfo[]>
        const previousFrame = new Map();      // hex -> frame id, for the hysteresis
        const trails = new Map();             // hex -> [{t, lat, lon, altBaro}]
        let timer = null;
        let airport = '';

        let collapsed = new Set();
        try { collapsed = new Set(JSON.parse(localStorage.getItem('parCollapsed') || '[]')); } catch (e) { /* none kept */ }

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
            const toggle = ev.target.closest('[data-par-toggle]');
            if (toggle) {
                const key = toggle.getAttribute('data-par-toggle');
                if (collapsed.has(key)) collapsed.delete(key); else collapsed.add(key);
                try { localStorage.setItem('parCollapsed', JSON.stringify([...collapsed])); } catch (e) { /* kept for this page only */ }
                render();
                return;
            }
            const hit = ev.target.closest('[data-hex]');
            if (hit) {
                store.selectAircraftByHex(hit.getAttribute('data-hex'));
                return;
            }
            // A click beside the aircraft lets go of the selected one, as on the map.
            if (store.selectedAircraft) {
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

        function draw(apt, runways) {
            const opts = G.DEFAULTS;
            const frames = G.buildFrames(apt.code, runways, opts);
            if (!frames.length) {
                container.innerHTML = message(`${esc(apt.code)} has no runway with both ends located.`);
                return;
            }
            const byIdent = {};
            for (const f of frames) { byIdent[f.le.ident] = f.le; byIdent[f.he.ident] = f.he; }
            const inUse = (apt.runway_in_use || [])
                .map((r) => Object.assign({}, r, { end: endIdent(r) }))
                .filter((r) => byIdent[r.end])
                .map((r) => Object.assign(r, { bearing: byIdent[r.end].landingBearing }))
                .filter((r) => r.probability >= IN_USE_MIN_PROBABILITY);

            const list = visibleAircraft();
            recordTrails(list);
            const est = G.estimateQnh(Object.values(store.aircraft || {}), apt.latitude, apt.longitude, opts);
            const qnh = est ? est.qnh : G.STD_HPA;
            const tl = G.transitionLevel(opts.transitionAltitudeFt, qnh);

            const placed = new Map(frames.map((f) => [f.id, []]));
            for (const a of list) {
                const t = a.adsb;
                // co-atc ties each aircraft to one followed airport (D62): one it
                // gives to another airport is not this one's, whatever its place
                // in the cones -- De Gaulle's finals lie in Orly's northern one.
                const ph = a.phase && a.phase.current && a.phase.current[0];
                if (ph && ph.airport && ph.airport !== apt.code) {
                    previousFrame.delete(a.hex);
                    continue;
                }
                // The server's runway judge (D69) has the last word: an aircraft it
                // puts on another airport's runway is not this one's.
                const rw = a.runway || {};
                const arrival = rw.arrival && rw.arrival.airport === apt.code ? rw.arrival.runway : null;
                const departure = rw.departure && rw.departure.airport === apt.code ? rw.departure.runway : null;
                if (!arrival && !departure && (rw.arrival || rw.departure)) {
                    previousFrame.delete(a.hex);
                    continue;
                }
                const onGround = !!a.on_ground;
                if (typeof t.alt_baro !== 'number' && !onGround) continue;
                const altFt = onGround ? null : G.qnhAltitude(t.alt_baro, qnh);
                const input = { lat: t.lat, lon: t.lon, altFt, track: t.track, onGround };
                let p = G.classify(frames, input, opts, previousFrame.get(a.hex));
                // An aircraft judged on a runway goes to that runway's frame while
                // its cone holds it, even as it crosses the neighbouring axis on
                // its way in: arriving on the approach side, departing past the
                // far end.
                const judged = (ident, departing) => {
                    const f = ident && frames.find((q) => q.le.ident === ident || q.he.ident === ident);
                    if (!f) return null;
                    const key = f.le.ident === ident ? 'le' : 'he';
                    const q = G.placeInFrame(f, input, opts);
                    if (!q) return null;
                    const ok = departing
                        ? q.side === (key === 'le' ? 'he' : 'le') && q.movingAway
                        : (q.side === key && !q.movingAway) || q.side === 'rwy';
                    return ok ? Object.assign(q, { judged: departing ? 'departure' : 'arrival', runway: ident }) : null;
                };
                p = judged(arrival, false) || judged(departure, true) || p;
                if (!p) { previousFrame.delete(a.hex); continue; }
                previousFrame.set(a.hex, p.frame);
                placed.get(p.frame).push({ a, p, altFt });
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
                ? `<a class="par-vac" href="${esc(vac.url)}" target="_blank" rel="noopener"${helpAttr('VAC',
                    `Visual approach chart of ${apt.code}, from the SIA, AIRAC cycle of ${vac.cycle}. Opens in a new tab.`)}>VAC</a>`
                : '';
            const header = `
                <div class="par-header">
                    <span class="par-airport"${HELP.view()}>${esc(apt.code)}</span>
                    <span class="par-name"${HELP.view()}>${esc(apt.name || '')}</span>
                    ${vacLink}
                    <span class="par-sep">·</span>
                    <span${HELP.inUse(inUse.length > 1)}>${inUse.length > 1 ? 'Runways' : 'Runway'} in use <b>${inUse.length
                        ? inUse.map((r) => esc(r.end) + ` <span class="par-dim">${Math.round(r.probability * 100)}%</span>`).join(' · ')
                        : '<span class="par-dim">unknown</span>'}</b></span>
                    <span class="par-sep">·</span>
                    <span${HELP.qnh(est)}>QNH <b>${est ? Math.round(qnh) + ' hPa' : '—'}</b> <span class="par-dim">${est ? `(ADS-B, ${est.count} aircraft)` : '(standard)'}</span></span>
                    <span class="par-sep">·</span>
                    <span${HELP.ta(opts.transitionAltitudeFt)}>TA <b>${opts.transitionAltitudeFt} ft</b></span>
                    <span class="par-sep">·</span>
                    <span${HELP.tl(tl, opts.transitionAltitudeFt)}>TL <b>FL${pad3(tl)}</b> <span class="par-dim">(computed)</span></span>
                </div>`;

            const selected = store.selectedAircraft && store.selectedAircraft.hex;
            const body = ordered.map((f) => {
                const key = `${apt.code} ${f.id}`;
                const isInUse = usage(f) > 0;
                const items = placed.get(f.id);
                const A = arrivalEnd(f, inUse);
                const a = f[A].ident;
                const b = f[A === 'le' ? 'he' : 'le'].ident;
                const head = `
                    <div class="par-frame-head" data-par-toggle="${esc(key)}">
                        <i class="fas ${collapsed.has(key) ? 'fa-chevron-right' : 'fa-chevron-down'}"${HELP.fold()}></i>
                        <span class="par-runway"${HELP.runway(a, b)}>${esc(a + ' → ' + b)}</span>
                        ${isInUse ? `<span class="par-badge"${HELP.inUse(inUse.length > 1)}>IN USE</span>` : ''}
                        <span class="par-dim"${HELP.count()}>${items.length} aircraft</span>
                    </div>`;
                if (collapsed.has(key)) return `<section class="par-frame par-collapsed">${head}</section>`;
                return `<section class="par-frame${isInUse ? ' par-in-use' : ''}">${head}${frameSvg(f, A, items, { width, qnh, tl, opts, selected, isInUse })}</section>`;
            }).join('');

            // Start below the buttons in the map's top left corner.
            container.style.paddingTop = '56px';
            container.innerHTML = header + body;
        }

        // --- One frame --------------------------------------------------------

        function frameSvg(f, A, items, ctx) {
            const { width, qnh, tl, opts, selected, isInUse } = ctx;
            const R = opts.rangeNM;
            const L = f.lengthNM;
            const B = A === 'le' ? 'he' : 'le';
            const plotW = width - LEFT - RIGHT;
            const ground = Math.min(f.le.elevFt, f.he.elevFt);
            const ceil = opts.ceilingFt;
            const elevTop = 18;
            const azTop = elevTop + ELEV_H + 22;
            const height = azTop + AZ_H + 20;

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
                text(at[0], at[1] - 5, `${e.slopeDeg}°`, col,
                    `text-anchor="middle" font-size="10"${strong ? '' : ' fill-opacity="0.7"'}${HELP.glide(e, strong)}`);
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
                const col = PHASE_COLORS[phaseOf(it.a)] || PHASE_COLORS.UNK;
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
            const col = PHASE_COLORS[phaseOf(a)] || PHASE_COLORS.UNK;
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

        return {
            show(code) {
                airport = code || airport;
                container.dataset.parActive = '1';
                render();
                if (!timer) timer = setInterval(render, 1000);
            },
            hide() {
                hideTip();
                container.dataset.parActive = '0';
                if (timer) clearInterval(timer);
                timer = null;
            },
            refresh: render,
        };
    }

    window.ParView = { create };
})();
