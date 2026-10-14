// The aircraft story (design system, AircraftStory; migration step 8): what
// the context column shows while a flight is followed. Its head (phase,
// callsign, operator, four numbers), the tabs Story, Details, Track and
// Nearby -- the panel's former Timeline, Details, Tracks and Proximity, whose
// contents and refreshes are unchanged -- and the instructions heard with
// what the aircraft did next ("Then", www/story/then.js, never a verdict).
//
// One domain of the Alpine store 'atc' (docs-fr/32, palier 5.1); see
// store/station.js for how the domains are assembled. Nothing new is fetched:
// the instructions are the `plain` values of the controller's transmissions
// matched to the aircraft (aircraftRadio), the track is the one the panel
// already refreshes every 5 s (aircraftDetailsHistoryData).
(function () {
    // A narrow no-break space between thousands, as the design system writes
    // numbers ("4 000 ft").
    const THIN = ' ';
    const group = (n) => String(Math.round(n)).replace(/\B(?=(\d{3})+(?!\d))/g, THIN);
    // Faster than this, the altitude's arrow says it climbs or descends.
    const TREND_FPM = 300;
    // Out of the store, so that reading them in a getter does not make Alpine
    // render again: the last instructions table and the runway frames.
    const memo = { key: null, rows: [] };
    const framesByAirport = {};

    window.AtcStore = window.AtcStore || [];
    window.AtcStore.push(() => ({
        // Which sections of the Details tab are open (c-details): the tables
        // and the basic facts open, the long lists closed.
        storyOpen: {
            phases: true, instructions: true, radio: false, basic: true,
            position: false, speed: false, nav: false, adsb: false,
        },

        // 'story', 'details', 'track' or 'nearby', from the panel's three flags.
        get storyTab() {
            if (this.showTimelineView) return 'story';
            if (this.showProximityView) return 'nearby';
            if (this.aircraftDetailsShowHistoryView) return 'track';
            return 'details';
        },

        // What the panel's four buttons did, each with its refreshes.
        setStoryTab(tab) {
            this.showTimelineView = tab === 'story';
            this.aircraftDetailsShowHistoryView = tab === 'track';
            this.showProximityView = tab === 'nearby';
            if (tab === 'nearby') {
                this.stopPhaseHistoryRefresh();
                this.loadProximityData();
                this.startProximityRefresh();
            } else {
                this.stopProximityRefresh();
                this.clearProximityView();
                if (tab === 'details') this.stopPhaseHistoryRefresh();
            }
        },

        toggleStorySection(key) {
            this.storyOpen[key] = !this.storyOpen[key];
        },

        // "‹ Back to Traffic": back to the column tab the aircraft was chosen
        // from, which is still the selected one.
        get storyBackLabel() {
            return this.columnTab === 'now' ? 'Back to Right now' : 'Back to Traffic';
        },

        // ‹, ✕ and Escape: stop following, every refresh and trail with it.
        stopFollowing() {
            const previousHex = this.selectedAircraft && this.selectedAircraft.hex;
            this.closeAircraftPanel();
            this.aircraftDetailsShowHistoryView = false;
            this.showProximityView = false;
            this.aircraftDetailsHistoryData = [];
            this.aircraftDetailsFutureData = [];
            this.aircraftDetailsHindcastData = [];
            this.aircraftDetailsHistoryCount = 0;
            this.phaseHistoryData = [];
            this.phaseHistoryAircraftHex = null;
            this.aircraftDetailsStopHistoryRefresh();
            this.stopProximityRefresh();
            this.stopPhaseHistoryRefresh();
            this.clearProximityView();
            if (this.mapManager && previousHex) this.mapManager.clearAircraftTrails(previousHex);
        },

        // Where the followed aircraft stands in an arrival sequence of a
        // followed airport: { airport, runway, distNM, etaSec }, or null.
        storyArrival(a) {
            if (!a) return null;
            for (const [code, seqs] of Object.entries(this.arrivals || {})) {
                for (const seq of seqs || []) {
                    const item = (seq.items || []).find((x) => x.hex === a.hex);
                    if (item) return { airport: code, runway: seq.runway, distNM: item.distNM, etaSec: item.etaSec };
                }
            }
            return null;
        },

        // The head of the story: who, and four numbers.
        get storyHead() {
            const a = this.selectedAircraft;
            if (!a) return null;
            const adsb = a.adsb || {};
            const bsdb = a.bsdb || {};
            const phase = this.getCurrentPhase(a);
            const who = [a.airline || bsdb.registered_owners, bsdb.type || adsb.t, bsdb.registration || adsb.r]
                .map((x) => String(x || '').trim()).filter((x) => x && x !== '-');

            const r = this.altitudeReading(a);
            let alt = { value: '–', unit: '' };
            if (r && r.kind === 'gnd') alt = { value: 'Ground', unit: '' };
            else if (r && r.kind === 'ft') alt = { value: group(Math.round(r.ft / 100) * 100), unit: 'ft' };
            else if (r) alt = { value: window.AltitudeScale.readingText(r), unit: '' };
            const rate = adsb.baro_rate;
            const arrow = typeof rate === 'number' && !(r && r.kind === 'gnd')
                ? (rate > TREND_FPM ? ' ↑' : rate < -TREND_FPM ? ' ↓' : '') : '';

            const arr = this.storyArrival(a);
            let second;
            if (arr && Number.isFinite(arr.distNM)) {
                const eta = Number.isFinite(arr.etaSec)
                    ? ` · ${Math.floor(arr.etaSec / 60)}:${String(Math.round(arr.etaSec % 60)).padStart(2, '0')}` : '';
                second = {
                    label: `To ${arr.runway}`, value: arr.distNM.toFixed(1), unit: 'NM', extra: eta,
                    help: `Distance along the approach to runway ${arr.runway} at ${arr.airport}, and the time to touchdown at its present speed: the same estimate as the PAR.`,
                };
            } else {
                const d = a.distance;
                second = {
                    label: 'Distance', value: d === undefined || d === null ? '–' : String(d), unit: d == null ? '' : 'NM', extra: '',
                    help: 'From the receiving station, in nautical miles (1 NM = 1.852 km).',
                };
            }

            const h = this.getHeadingWithType(a);
            const hType = { magnetic: 'mag', track: 'trk', true: 'true' }[h.type] || '';
            const gs = adsb.gs;
            return {
                callsign: (a.flight || '').trim() || a.hex,
                phase: phase || 'NEW',
                airport: this.getPhaseAirport(a),
                who: who.join(' · '),
                cells: [
                    { label: 'Altitude', value: alt.value, unit: alt.unit, extra: arrow, help: this.altitudeHelp(a) },
                    second,
                    {
                        label: 'Ground speed', value: typeof gs === 'number' ? String(Math.round(gs)) : '–', unit: typeof gs === 'number' ? 'kt' : '', extra: '',
                        help: 'Its speed over the ground, in knots. The wind sits between it and the airspeed a controller gives.',
                    },
                    {
                        label: 'Heading', value: h.value === null ? '–' : `${Math.round(h.value)}°`, unit: h.value === null ? '' : hType, extra: '',
                        help: this.headingHelp(h.type),
                    },
                ],
            };
        },

        // The map's note while a flight is followed.
        get storySkyNote() {
            const a = this.selectedAircraft;
            if (!a) return '';
            const pts = this.aircraftDetailsHistoryData || [];
            let first = null;
            for (const p of pts) {
                const t = Date.parse(p.timestamp);
                if (Number.isFinite(t) && (first === null || t < first)) first = t;
            }
            const cs = (a.flight || '').trim() || a.hex;
            return first === null ? `Following ${cs}` : `Following ${cs} · its track since ${this.storyClock(new Date(first))}`;
        },

        // "13:12Z" or "13:12": a timeline's time without its seconds.
        storyClock(at) {
            return this.timelineTime(at).replace(/(\d{1,2}:\d{2}):\d{2}/, '$1');
        },

        // The Story tab: the flight's timeline (www/timeline/) as a list of
        // facts, the last one marked as now, then the expected touchdown when
        // the aircraft is in an arrival sequence.
        get storyEvents() {
            const list = this.flightTimeline.map((ev, i, all) => ({
                key: i + ev.kind + ev.at.getTime(),
                time: this.storyClock(ev.at),
                dot: i === all.length - 1 ? 'is-now' : (ev.kind === 'transmission' || ev.kind === 'transfer') ? 'is-radio' : '',
                next: false,
                ev,
            }));
            const arr = this.storyArrival(this.selectedAircraft);
            if (arr && Number.isFinite(arr.etaSec) && !(this.selectedAircraft && this.selectedAircraft.on_ground)) {
                const at = new Date(Date.now() + arr.etaSec * 1000);
                list.push({
                    key: 'next',
                    time: '≈' + this.storyClock(at),
                    dot: '',
                    next: true,
                    ev: { text: `Expected touchdown on ${arr.runway}, from its distance and speed.` },
                });
            }
            return list;
        },

        // The instructions heard: every value the grammar read in what the
        // controller said to this aircraft, newest first, with what the
        // aircraft did next. Kept until the transmissions or the track change:
        // each row reads the whole track.
        get instructionsHeard() {
            const a = this.selectedAircraft;
            const R = this.aircraftRadio;
            if (!a || !R || R.callsign !== (a.flight || '').trim()) return [];
            const said = (R.transcriptions || []).filter((t) => t.speaker_type === 'ATC' && Array.isArray(t.plain) && t.plain.length);
            const track = this.aircraftDetailsHistoryData || [];
            const last = track.length ? track[track.length - 1].timestamp : '';
            const frames = this.storyFrames();
            const key = `${a.hex}|${said.length}|${said.length ? said[0].id : ''}|${track.length}|${last}|${Math.round(this.altimetry.qnh || 0)}|${frames ? frames.length : 0}`;
            if (memo.key === key) return memo.rows;

            const T = window.StoryThen;
            const options = {
                qnh: this.altimetry.qnh,
                ta: this.altimetry.ta,
                frames,
                transmissions: (R.transcriptions || []).map((t) => ({ at: t.created_at, frequency: this.freqMhz(t.frequency_id) })),
            };
            const rows = [];
            for (const t of said) {
                t.plain.forEach((p, i) => {
                    const then = T ? T.describe(Object.assign({}, p, { at: t.created_at }), track, options) : null;
                    rows.push({
                        key: `${t.id}-${i}`,
                        at: Date.parse(t.created_at),
                        time: this.formatDate(t.created_at, true),
                        text: p.text,
                        then: then ? then.text : '—',
                        known: !!then,
                    });
                });
            }
            rows.sort((x, y) => y.at - x.at);
            memo.key = key;
            memo.rows = rows;
            return rows;
        },

        freqMhz(freqId) {
            const f = (this.audioFrequencies || []).find((x) => x.id === freqId);
            return f && Number(f.frequency_mhz) > 0 ? Number(f.frequency_mhz) : null;
        },

        // The runway frames of the followed airports, to name the runway an
        // aircraft landed on (built once per airport, when its runways came).
        storyFrames() {
            const G = window.ParGeometry;
            if (!G) return null;
            const out = [];
            for (const apt of this.followedAirports || []) {
                const frames = framesByAirport[apt.code];
                if (frames) { out.push(...frames); continue; }
                if (frames === null || typeof this.runwaysOf !== 'function') continue;
                framesByAirport[apt.code] = null;
                this.runwaysOf(apt.code).then((runways) => {
                    framesByAirport[apt.code] = runways && runways.length ? G.buildFrames(apt.code, runways, G.DEFAULTS) : [];
                });
            }
            return out.length ? out : null;
        },

        // The Details tab's key-value sections (formerly one HTML string):
        // the same facts, grouped as c-details has them.
        get aircraftDetailGroups() {
            const groups = this.aircraftDetailFields();
            if (!groups) return [];
            const by = Object.fromEntries(groups);
            const merge = (...names) => names.flatMap((n) => by[n] || []);
            const rows = (items) => items.map(([k, v, extra]) => {
                const text = String(v === undefined || v === null ? '-' : v).trim();
                const none = text === '-' || /^-\s*(ft|kts|hPa|°|°C|s|dBm|ft\/min)?$/.test(text);
                return { k, v: none ? '—' : (extra ? `${text} · ${extra} ago` : text), none };
            });
            return [
                { key: 'basic', name: 'Basic info', rows: rows(merge('Basic Info', 'Status')) },
                { key: 'position', name: 'Position', rows: rows(merge('Position')) },
                { key: 'speed', name: 'Speed and direction', rows: rows(merge('Speed & Direction')) },
                { key: 'nav', name: 'Navigation and weather', rows: rows(merge('Navigation', 'Weather', 'ATC Derived')) },
                { key: 'adsb', name: 'ADS-B and signal', rows: rows(merge('ADSB Info', 'Signal')) },
            ];
        },
    }));
})();
