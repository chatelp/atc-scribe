// The selected aircraft's panel: its radio, its timeline, its tracks and phase
// history, the aircraft around it, the details sheet.
//
// One domain of the Alpine store 'atc' (docs-fr/32, palier 5.1): the entries
// below are the store's, moved from app.js as they were, and app.js assembles
// them with the rest. A method's `this` is the store. The entries are built
// when app.js assembles the store, inside alpine:init as they always were: by
// then the module-level names of app.js (generateUUID, API_BASE_URL, CONFIG,
// MAP_STYLES, wsClient, audioClient, mapManager) are defined, and a method
// resolves them when it runs.
(function () {
    window.AtcStore = window.AtcStore || [];
    window.AtcStore.push(() => ({

        // What the radio said about the selected aircraft. Filled from
        // /api/v1/transcriptions/callsign/{callsign}, an endpoint upstream already
        // serves and no page ever called -- it only becomes useful once something
        // fills the callsign column, which the local grammar now does.
        aircraftRadio: { callsign: '', key: '', loading: false, error: '', transcriptions: [], values: {} },
        
        // Request throttling state
        pendingRequests: {
            aircraft: false,
            tracks: new Map(), // Map of hex -> boolean for pending tracks requests
            proximity: false,
        },

        // Clear pending requests for a specific aircraft
        clearPendingRequestsForAircraft(hex) {
            this.pendingRequests.tracks.delete(hex);
        },

        // Internal state for tracking aircraft selection changes
        _previousSelectedHex: null,

        // Request timeout configuration
        REQUEST_TIMEOUT_MS: 5000, // 5 seconds

        // Create a fetch request with timeout
        async fetchWithTimeout(url, options = {}) {
            const controller = new AbortController();
            const timeoutId = setTimeout(() => controller.abort(), this.REQUEST_TIMEOUT_MS);
            
            try {
                const response = await fetch(url, {
                    ...options,
                    signal: controller.signal
                });
                clearTimeout(timeoutId);
                return response;
            } catch (error) {
                clearTimeout(timeoutId);
                if (error.name === 'AbortError') {
                    throw new Error(`Request timeout after ${this.REQUEST_TIMEOUT_MS}ms`);
                }
                throw error;
            }
        },

        getATCDerivedMetrics(aircraft) {
            const adsbData = aircraft?.adsb || {};
            const serverDerived = adsbData.atc_derived || null;
            const num = (v) => (typeof v === 'number' && Number.isFinite(v) ? v : null);
            const normalizeHeading = (v) => ((v % 360) + 360) % 360;
            const signedAngleDiff = (a, b) => {
                let d = normalizeHeading(a) - normalizeHeading(b);
                while (d > 180) d -= 360;
                while (d <= -180) d += 360;
                return d;
            };

            const track = num(adsbData.track);
            const trueHeading = num(adsbData.true_heading);
            const magHeading = num(adsbData.mag_heading);
            const windDir = num(adsbData.wd);
            const windSpeed = num(adsbData.ws);
            const gs = num(adsbData.gs);
            const verticalRate = num(adsbData.baro_rate) ?? num(adsbData.geom_rate);
            const trackRate = num(adsbData.track_rate);

            const headingSource = serverDerived?.heading_source
                || (trueHeading !== null ? 'TRUE' : (magHeading !== null ? 'MAG' : (track !== null ? 'TRACK' : '-')));

            let trackHeadingError = '-';
            if (num(serverDerived?.track_heading_error_deg) !== null) {
                const drift = num(serverDerived.track_heading_error_deg);
                trackHeadingError = `${drift >= 0 ? '+' : ''}${drift.toFixed(1)}°`;
            } else {
                const referenceHeading = trueHeading ?? magHeading ?? track;
                if (track !== null && referenceHeading !== null) {
                    const drift = signedAngleDiff(track, referenceHeading);
                    trackHeadingError = `${drift >= 0 ? '+' : ''}${drift.toFixed(1)}°`;
                }
            }

            let headTailWind = '-';
            let crossWind = '-';
            if (num(serverDerived?.head_tailwind_kt) !== null && num(serverDerived?.crosswind_kt) !== null) {
                const headwind = num(serverDerived.head_tailwind_kt);
                const crosswindVal = num(serverDerived.crosswind_kt);
                const hwLabel = headwind >= 0 ? 'HW' : 'TW';
                const cwLabel = crosswindVal >= 0 ? 'R' : 'L';
                headTailWind = `${hwLabel} ${Math.abs(headwind).toFixed(1)} kt`;
                crossWind = `${cwLabel} ${Math.abs(crosswindVal).toFixed(1)} kt`;
            } else if (track !== null && windDir !== null && windSpeed !== null) {
                const rel = signedAngleDiff(windDir, track) * Math.PI / 180;
                const headwind = windSpeed * Math.cos(rel); // + headwind, - tailwind
                const crosswindVal = windSpeed * Math.sin(rel); // + from right, - from left
                const hwLabel = headwind >= 0 ? 'HW' : 'TW';
                const cwLabel = crosswindVal >= 0 ? 'R' : 'L';
                headTailWind = `${hwLabel} ${Math.abs(headwind).toFixed(1)} kt`;
                crossWind = `${cwLabel} ${Math.abs(crosswindVal).toFixed(1)} kt`;
            }

            let flightPathAngle = '-';
            if (num(serverDerived?.flight_path_angle_deg) !== null) {
                const fpa = num(serverDerived.flight_path_angle_deg);
                flightPathAngle = `${fpa >= 0 ? '+' : ''}${fpa.toFixed(2)}°`;
            } else if (verticalRate !== null && gs !== null && gs > 1) {
                const gsFeetPerMin = gs * 101.2686;
                const fpa = Math.atan2(verticalRate, gsFeetPerMin) * 180 / Math.PI;
                flightPathAngle = `${fpa >= 0 ? '+' : ''}${fpa.toFixed(2)}°`;
            }

            let climbGradient = '-';
            if (num(serverDerived?.climb_gradient_ft_nm) !== null) {
                const gradient = num(serverDerived.climb_gradient_ft_nm);
                climbGradient = `${gradient >= 0 ? '+' : ''}${gradient.toFixed(0)} ft/NM`;
            } else if (verticalRate !== null && gs !== null && gs > 1) {
                const gradient = (verticalRate * 60.0) / gs;
                climbGradient = `${gradient >= 0 ? '+' : ''}${gradient.toFixed(0)} ft/NM`;
            }

            let etaToStation = '-';
            if (num(serverDerived?.eta_station_sec) !== null) {
                const totalSec = Math.max(0, Math.round(num(serverDerived.eta_station_sec)));
                const whole = Math.floor(totalSec / 60);
                const sec = totalSec % 60;
                etaToStation = `${whole}m ${sec}s`;
            } else {
                const distanceNm = num(aircraft.distance);
                if (distanceNm !== null && gs !== null && gs > 30) {
                    const mins = (distanceNm / gs) * 60;
                    const whole = Math.floor(mins);
                    const sec = Math.round((mins - whole) * 60);
                    etaToStation = `${whole}m ${sec}s`;
                }
            }

            const turnRateSource = num(serverDerived?.turn_rate_deg_sec) ?? trackRate;
            const turnRateText = turnRateSource !== null ? `${turnRateSource >= 0 ? '+' : ''}${turnRateSource.toFixed(2)}°/s` : '-';

            return {
                headingSource,
                trackHeadingError,
                headTailWind,
                crossWind,
                flightPathAngle,
                climbGradient,
                etaToStation,
                turnRateText
            };
        },

        // Every fact the panel shows about the selected aircraft, by category:
        // [[category, [[label, value, seconds ago]]]]. The Details tab groups
        // them (store/story.js, aircraftDetailGroups).
        aircraftDetailFields() {
            if (!this.selectedAircraft) return null;

            const aircraft = this.selectedAircraft;
            const adsbData = aircraft.adsb || {};
            
            // Calculate seconds since last seen
            let lastSeenText = '-';
            let lastSeenSeconds = '';
            if (aircraft.last_seen) {
                const secondsAgo = Math.floor((new Date() - new Date(aircraft.last_seen)) / 1000);
                lastSeenText = this.formatDate(aircraft.last_seen, true);
                lastSeenSeconds = `${secondsAgo}s`;
            }

            // Calculate first seen text with seconds ago
            let firstSeenText = '-';
            let firstSeenSeconds = '';
            if (aircraft.created_at) {
                const secondsAgo = Math.floor((new Date() - new Date(aircraft.created_at)) / 1000);
                firstSeenText = this.formatDate(aircraft.created_at, true);
                firstSeenSeconds = `${secondsAgo}s`;
            }

            // Calculate takeoff time text with seconds ago
            let takeoffTimeText = '-';
            let takeoffSeconds = '';
            if (aircraft.DateTookoff || aircraft.date_tookoff) {
                const takeoffTime = aircraft.DateTookoff || aircraft.date_tookoff;
                const secondsAgo = Math.floor((new Date() - new Date(takeoffTime)) / 1000);
                takeoffTimeText = this.formatDate(takeoffTime, true);
                takeoffSeconds = `${secondsAgo}s`;
            }

            // Calculate landing time text with seconds ago
            let landingTimeText = '-';
            let landingSeconds = '';
            if (aircraft.DateLanded || aircraft.date_landed) {
                const landingTime = aircraft.DateLanded || aircraft.date_landed;
                const secondsAgo = Math.floor((new Date() - new Date(landingTime)) / 1000);
                landingTimeText = this.formatDate(landingTime, true);
                landingSeconds = `${secondsAgo}s`;
            }

            // Get BSDB data if available - use it to enrich basic info
            const bsdbData = aircraft.bsdb || {};

            // Prefer BSDB data over ADSB data for type and registration
            const aircraftType = bsdbData.type || adsbData.t || '-';
            const aircraftReg = bsdbData.registration || adsbData.r || '-';
            const aircraftOperator = bsdbData.registered_owners || '-';
            const derived = this.getATCDerivedMetrics(aircraft);

            // Runways, as the server's runway judge has them (D69): the same verdict
            // the PAR view and the runway-in-use summary use.
            const runwayText = (() => {
                const r = aircraft.runway || {};
                const parts = [];
                if (r.departure) parts.push(`Departed ${r.departure.runway} (${r.departure.airport})`);
                if (r.go_around) parts.push(`Went around ${r.go_around.runway} (${r.go_around.airport})`);
                if (r.arrival) {
                    const landed = aircraft.on_ground || (aircraft.date_landed && new Date(aircraft.date_landed) >= new Date(r.arrival.since));
                    parts.push(`${landed ? 'Landed on' : 'Arriving'} ${r.arrival.runway} (${r.arrival.airport})`);
                }
                return parts.length ? parts.join(' · ') : '-';
            })();

            const fields = [
                ['Basic Info', [
                    ['Callsign', aircraft.flight?.trim() || '-'],
                    ['Airline', aircraft.airline || '-'],
                    ['Operator', aircraftOperator],
                    ['Country', aircraft.airline_country || '-'],
                    ['Hex', aircraft.hex],
                    ['Type', aircraftType],
                    ['Registration', aircraftReg],
                    ['Category', adsbData.category || '-'],
                    ['Squawk', adsbData.squawk || '-'],
                    ['First Seen', firstSeenText, firstSeenSeconds],
                    ['Last Seen', lastSeenText, lastSeenSeconds]
                ]],
                ['Status', [
                    ['On Ground', aircraft.on_ground ? 'Yes' : 'No'],
                    ['Phase', this.getCurrentPhase(aircraft)],
                    ['Runway', runwayText],
                    ['Takeoff Time', takeoffTimeText, takeoffSeconds],
                    ['Landing Time', landingTimeText, landingSeconds]
                ]],
                ['Position', [
                    ['Latitude', adsbData.lat?.toFixed(6) || '-'],
                    ['Longitude', adsbData.lon?.toFixed(6) || '-'],
                    ['Distance (NM)', aircraft.distance ?? '-'],
                    ['Altitude', window.AltitudeScale.readingText(this.altitudeReading(aircraft), 10) +
                        (this.altitudeReading(aircraft)?.kind === 'ft' ? ` (QNH ${Math.round(this.altimetry.qnh)}${this.altimetry.estimated ? '' : ', standard'})` : '')],
                    ['Pressure altitude (1013)', `${adsbData.alt_baro ?? '-'} ft`],
                    ['Altitude (Geom)', `${adsbData.alt_geom ?? '-'} ft`],
                    ['Vertical Rate', `${adsbData.baro_rate ?? '-'} ft/min`]
                ]],
                ['Speed & Direction', [
                    ['Ground Speed', `${adsbData.gs ?? '-'} kts`],
                    ['True Airspeed', `${adsbData.tas ?? '-'} kts`],
                    ['IAS', `${adsbData.ias ?? '-'} kts`],
                    ['TAS', `${adsbData.tas ?? '-'} kts`],
                    ['Mach', adsbData.mach ?? '-'],
                    ['Track', `${adsbData.track ?? '-'}°`],
                    ['Mag Heading', `${adsbData.mag_heading ?? '-'}°`],
                    ['True Heading', `${adsbData.true_heading ?? '-'}°`]
                ]],
                ['Navigation', [
                    ['Nav QNH', `${adsbData.nav_qnh ?? '-'} hPa`],
                    ['Nav Altitude MCP', `${adsbData.nav_altitude_mcp ?? '-'} ft`],
                    ['Nav Altitude FMS', `${adsbData.nav_altitude_fms ?? '-'} ft`],
                    ['Nav Heading', `${adsbData.nav_heading ?? '-'}°`]
                ]],
                ['Weather', [
                    ['Wind Direction', `${adsbData.wd ?? '-'}°`],
                    ['Wind Speed', `${adsbData.ws ?? '-'} kts`],
                    ['OAT', `${adsbData.oat ?? '-'}°C`],
                    ['TAT', `${adsbData.tat ?? '-'}°C`]
                ]],
                ['ATC Derived', [
                    ['Heading Source', derived.headingSource],
                    ['Track-Heading Error', derived.trackHeadingError],
                    ['Head/Tail Wind', derived.headTailWind],
                    ['Crosswind (L/R)', derived.crossWind],
                    ['Flight Path Angle', derived.flightPathAngle],
                    ['Climb Gradient', derived.climbGradient],
                    ['Turn Rate', derived.turnRateText],
                    ['ETA to Station', derived.etaToStation]
                ]],
                ['ADSB Info', [
                    ['Version', adsbData.version ?? '-'],
                    ['NIC', adsbData.nic ?? '-'],
                    ['NACp', adsbData.nac_p ?? '-'],
                    ['NACv', adsbData.nac_v ?? '-'],
                    ['SIL', adsbData.sil ?? '-'],
                    ['SIL Type', adsbData.sil_type || '-'],
                    ['GVA', adsbData.gva ?? '-'],
                    ['SDA', adsbData.sda ?? '-']
                ]],
                ['Signal', [
                    ['Messages', adsbData.messages ?? '-'],
                    ['Seen', `${adsbData.seen ?? '-'}s`], 
                    ['RSSI', `${adsbData.rssi ?? '-'} dBm`]
                ]]
            ];

            return fields;
        },

        // Properties for Aircraft Details Panel (moved from x-data in HTML)
        aircraftDetailsShowHistoryView: false,
        aircraftDetailsHistoryData: [],
        aircraftDetailsFutureData: [],
        aircraftDetailsHindcastData: [],
        aircraftDetailsHistoryCount: 0,
        aircraftDetailsHistoryLoading: false,
        aircraftDetailsHistoryRefreshInterval: null,
        aircraftDetailsCurrentAircraftHexForPanel: null, // New property
        
        // Properties for Proximity View
        showProximityView: false,
        showTimelineView: true, // the Story tab, www/timeline/ (audit 3.3): the one a followed flight opens on (step 8)
        proximityDistance: 5, // Default to 5 NM
        proximityAircraft: [],
        proximityLoading: false,
        proximityHighlightedAircraft: new Set(), // Set of aircraft hex codes highlighted in proximity view

        // Properties for Phase History (now always shown in Tracks tab)
        phaseHistoryData: [],
        phaseHistoryLoading: false,
        phaseHistoryAircraftHex: null,
        phaseHistoryRefreshInterval: null,
        highlightedAdsbId: null, // For highlighting specific rows in Tracks tab
        
        // Getter to ensure phaseHistoryData is always an array
        get safePhaseHistoryData() {
            return this.phaseHistoryData || [];
        },
        
        // Current time for reactive time calculations
        currentTimeForPhases: new Date(),

        // Methods for Aircraft Details Panel (moved from x-data in HTML)
        // Fetch the transmissions matched to one aircraft, and the values the
        // grammar recovered from them.
        //
        // Called on every position update of the selected aircraft, about once a
        // second. It asks the server again only when the aircraft changed or
        // something new was heard on it -- the voice count is what says so. It
        // used to ask again whenever the last answer was empty, so every aircraft
        // with nothing matched blinked between "loading" and "nothing" each second.
        async loadAircraftRadio(callsign, heard) {
            const cs = (callsign || '').trim();
            if (!cs) {
                this.aircraftRadio = { callsign: '', key: '', loading: false, error: '', transcriptions: [], values: {} };
                return;
            }
            const key = cs + '|' + (heard || 0);
            if (this.aircraftRadio.key === key) return; // loaded, loading, or failed: nothing new

            // Something new on the same aircraft: keep what is shown while asking.
            const same = this.aircraftRadio.callsign === cs;
            this.aircraftRadio = {
                callsign: cs, key, loading: true, error: '',
                transcriptions: same ? this.aircraftRadio.transcriptions : [],
                values: same ? this.aircraftRadio.values : {}
            };
            try {
                const res = await fetch(`/api/v1/transcriptions/callsign/${encodeURIComponent(cs)}?limit=50`);
                if (!res.ok) throw new Error(`HTTP ${res.status}`);
                const data = await res.json();
                // The aircraft may have changed while the request was in flight.
                if (this.aircraftRadio.key !== key) return;
                this.aircraftRadio = {
                    callsign: cs, key,
                    loading: false, error: '',
                    transcriptions: (data.transcriptions || []).slice().reverse(),
                    values: data.values || {}
                };
            } catch (e) {
                console.warn('Failed to load radio for', cs, e);
                // Said, not shown as "nothing matched": the server failing is not
                // the aircraft being silent, which is how a broken query hid for days.
                if (this.aircraftRadio.key === key) {
                    this.aircraftRadio.loading = false;
                    this.aircraftRadio.error = 'Could not load the transmissions (' + e.message + ')';
                }
            }
        },

        // The selected aircraft's story: phases, runways, what was said to and
        // by it on every frequency, transfers, clearances (www/timeline/).
        get flightTimeline() {
            const a = this.selectedAircraft;
            if (!a || !window.FlightTimeline) return [];
            const sameRadio = this.aircraftRadio.callsign && this.aircraftRadio.callsign === (a.flight || '').trim();
            return window.FlightTimeline.build({
                aircraft: a,
                phases: this.phaseHistoryAircraftHex === a.hex ? this.phaseHistoryData : null,
                transmissions: sameRadio ? this.aircraftRadio.transcriptions : [],
                values: sameRadio ? this.aircraftRadio.values : {},
                frequencies: this.audioFrequencies,
            });
        },

        // A timeline's time: the date too when it is not today's.
        timelineTime(at) {
            const time = this.formatDate(at, true);
            const d = new Date(at);
            const now = new Date();
            const sameDay = this.showLocalDates
                ? d.toDateString() === now.toDateString()
                : d.toISOString().slice(0, 10) === now.toISOString().slice(0, 10);
            if (sameDay) return time;
            const day = this.showLocalDates
                ? d.toLocaleDateString([], { day: '2-digit', month: '2-digit' })
                : d.toISOString().slice(8, 10) + '/' + d.toISOString().slice(5, 7);
            return `${day} ${time}`;
        },

        // The values recovered from one transcription, for the template.
        radioValuesFor(id) {
            return (this.aircraftRadio.values && this.aircraftRadio.values[id]) || [];
        },

        setupAircraftDetailsPanel() {
            const a = this.selectedAircraft;
            this.loadAircraftRadio(a ? a.flight : '', a && a.voice ? a.voice.transmissions : 0);

            if (!this.selectedAircraft) { // No aircraft selected, fully close and reset
                this.aircraftDetailsShowHistoryView = false;
                this.aircraftDetailsHistoryData = [];
                this.aircraftDetailsFutureData = [];
                this.aircraftDetailsHindcastData = [];
                this.aircraftDetailsHistoryCount = 0;
                this.aircraftDetailsStopHistoryRefresh();

                // Clear map trails for the previously selected aircraft
                if (this.mapManager && this.aircraftDetailsCurrentAircraftHexForPanel) {
                    this.mapManager.clearAircraftTrails(this.aircraftDetailsCurrentAircraftHexForPanel);
                }

                this.aircraftDetailsCurrentAircraftHexForPanel = null;
                this.showProximityView = false;
                this.stopProximityRefresh();
                this.clearProximityView();
                this.phaseHistoryData = [];
                this.phaseHistoryAircraftHex = null;
                this.stopPhaseHistoryRefresh();
                return;
            }

            if (this.selectedAircraft.hex !== this.aircraftDetailsCurrentAircraftHexForPanel) {
                // Store current view state
                const wasInHistoryView = this.aircraftDetailsShowHistoryView;
                const wasInProximityView = this.showProximityView;
                
                // Clear data but maintain view state
                this.aircraftDetailsHistoryData = [];
                this.aircraftDetailsHistoryCount = 0;
                this.aircraftDetailsStopHistoryRefresh();
                this.stopProximityRefresh();
                this.clearProximityView();
                this.phaseHistoryData = [];
                this.stopPhaseHistoryRefresh();
                
                // Clear map trails for the previous aircraft immediately (O(1) operation)
                if (this.mapManager && this.aircraftDetailsCurrentAircraftHexForPanel) {
                    this.mapManager.clearAircraftTrails(this.aircraftDetailsCurrentAircraftHexForPanel);
                }
                
                // Clear stale track data from the aircraft object to prevent showing previous aircraft's tracks
                if (this.selectedAircraft.historyData) {
                    delete this.selectedAircraft.historyData;
                }
                if (this.selectedAircraft.futureData) {
                    delete this.selectedAircraft.futureData;
                }
                
                // Clear store track data immediately to prevent showing stale data
                this.aircraftDetailsHistoryData = [];
                this.aircraftDetailsFutureData = [];
                this.aircraftDetailsHindcastData = [];
                
                // Update current aircraft hex
                this.aircraftDetailsCurrentAircraftHexForPanel = this.selectedAircraft.hex;
                
                // Reload data for the new aircraft based on current view
                // Restore the previous view state
                if (wasInHistoryView) {
                    this.aircraftDetailsShowHistoryView = true;
                    this.showProximityView = false;
                } else if (wasInProximityView) {
                    this.showProximityView = true;
                    this.aircraftDetailsShowHistoryView = false;
                    this.loadProximityData();
                    this.startProximityRefresh();
                } else {
                    // Reset to default view if no special view was active
                    this.aircraftDetailsShowHistoryView = false;
                    this.showProximityView = false;
                }
                
                // Phase history is loaded as part of the tracks API response
                this.phaseHistoryAircraftHex = this.selectedAircraft.hex;

                // Load tracks data (includes phase history) for map trails when aircraft is selected
                this.aircraftDetailsLoadTracks();
                
                // Start refresh interval to keep map trails updated
                this.aircraftDetailsStartHistoryRefresh();
            }
        },

        aircraftDetailsStartHistoryRefresh() {
            if (this.aircraftDetailsHistoryRefreshInterval) {
                clearInterval(this.aircraftDetailsHistoryRefreshInterval);
            }
            const refreshRate = 5000; // Fixed 5 second refresh for aircraft details history
            
            this.aircraftDetailsHistoryRefreshInterval = setInterval(() => {
                if (this.selectedAircraft) {
                    this.aircraftDetailsLoadTracks(true); // Pass true for isRefresh
                }
            }, refreshRate);
        },

        aircraftDetailsStopHistoryRefresh() {
            if (this.aircraftDetailsHistoryRefreshInterval) {
                clearInterval(this.aircraftDetailsHistoryRefreshInterval);
                this.aircraftDetailsHistoryRefreshInterval = null;
            }
        },

        async aircraftDetailsLoadTracks(isRefresh = false) {
            if (!this.selectedAircraft) return;

            const hex = this.selectedAircraft.hex;

            // Initialize tracks pending requests if needed
            if (!this.pendingRequests.tracks) {
                this.pendingRequests.tracks = new Map();
            }

            // Check if there's already a pending tracks request for this aircraft
            if (this.pendingRequests.tracks.get(hex)) {
                return;
            }

            // Set pending flag
            this.pendingRequests.tracks.set(hex, true);
            
            if (!isRefresh) {
                this.aircraftDetailsHistoryLoading = true;
            }
            
            try {
                // Add limit parameter to control track length
                const limit = this.settings.tracksLimit || 1000;
                const url = `${API_BASE_URL}/aircraft/${hex}/tracks?limit=${limit}`;

                const response = await this.fetchWithTimeout(url);

                if (!response.ok) {
                    this.aircraftDetailsHistoryData = [];
                    this.aircraftDetailsFutureData = [];
                this.aircraftDetailsHindcastData = [];
                    this.aircraftDetailsHistoryCount = 0;
                    this.aircraftDetailsHistoryLoading = false;
                    return;
                }

                const data = await response.json();

                // Split combined tracks response into history, future, and hindcast
                this.aircraftDetailsHistoryData = data.history || [];
                this.aircraftDetailsFutureData = data.future || [];
                this.aircraftDetailsHindcastData = data.hindcast || [];
                this.aircraftDetailsHistoryCount = this.aircraftDetailsHistoryData.length;

                // Populate phase history from the same response (avoids separate API call)
                const phaseHistory = data.phase_history || [];
                if (phaseHistory.length > 0) {
                    this.phaseHistoryData = phaseHistory.map((phase, index) => ({
                        ...phase,
                        is_current: index === 0,
                        id: phase.id !== undefined ? phase.id : `phase-${index}`
                    }));
                } else {
                    this.phaseHistoryData = [];
                }

                // Update tracks mini-map and main map trails
                if (this.mapManager && this.mapManager.updateTracksMiniMap) {
                    this.mapManager.updateTracksMiniMap();
                }
                if (this.mapManager && this.mapManager.updateFlightPaths) {
                    this.mapManager.updateFlightPaths();
                }
            } catch (error) {
                this.aircraftDetailsHistoryData = [];
                this.aircraftDetailsFutureData = [];
                this.aircraftDetailsHindcastData = [];
                this.aircraftDetailsHistoryCount = 0;
                this.phaseHistoryData = [];
            } finally {
                this.aircraftDetailsHistoryLoading = false;
                this.pendingRequests.tracks.delete(hex);
            }
        },
        
        // Proximity View Methods
        proximityRefreshInterval: null,
        
        startProximityRefresh() {
            // Clear any existing interval
            this.stopProximityRefresh();
            
            // Fixed refresh rate for proximity data
            const refreshRate = 5000; // 5 seconds
            
            // Set up new interval
            this.proximityRefreshInterval = setInterval(() => {
                if (this.showProximityView && this.selectedAircraft) {
                    this.loadProximityData(true); // Pass true for isRefresh
                }
            }, refreshRate);
        },
        
        stopProximityRefresh() {
            if (this.proximityRefreshInterval) {
                clearInterval(this.proximityRefreshInterval);
                this.proximityRefreshInterval = null;
            }
        },
        
        async loadProximityData(isRefresh = false) {
            if (!this.selectedAircraft) return;
            
            // Check if there's already a pending proximity request
            if (this.pendingRequests.proximity) {
                console.log('Proximity request already in progress, skipping...');
                return;
            }

            this.pendingRequests.proximity = true;
            
            if (!isRefresh) {
                this.proximityLoading = true;
            }
            
            try {
                // Build the URL with the distance_nm and ref_hex parameters
                const url = `${API_BASE_URL}/aircraft?distance_nm=${this.proximityDistance}&ref_hex=${this.selectedAircraft.hex}`;
                
                const response = await this.fetchWithTimeout(url);
                if (!response.ok) {
                    console.error(`Error fetching proximity data: ${response.status}`);
                    this.proximityAircraft = [];
                    this.proximityLoading = false;
                    return;
                }
                
                const data = await response.json();
                
                // Filter out the reference aircraft
                this.proximityAircraft = (data.aircraft || []).filter(aircraft =>
                    aircraft.hex !== this.selectedAircraft.hex
                );
                
                // Draw the proximity circle on the map
                this.drawProximityCircle();
                
                // Highlight the aircraft labels
                this.highlightProximityAircraft();
            } catch (error) {
                if (error.message.includes('timeout')) {
                    console.warn('Proximity request timed out after 5 seconds');
                } else {
                    console.error('Error fetching proximity data:', error);
                }
                this.proximityAircraft = [];
            } finally {
                this.proximityLoading = false;
                // Always clear the pending flag
                this.pendingRequests.proximity = false;
            }
        },

        clearProximityView() {
            // Stop the refresh interval
            this.stopProximityRefresh();
            
            // Remove the proximity circle from the map
            if (this.mapManager) {
                this.mapManager.removeProximityCircle();
            }
            
            // Remove the highlighting from aircraft labels
            this.removeProximityHighlighting();
            
            // Reset the proximity data
            this.proximityAircraft = [];
            
            // Clear the highlighted aircraft set
            this.proximityHighlightedAircraft.clear();
        },
        

        drawProximityCircle() {
            if (!this.mapManager || !this.selectedAircraft || !this.selectedAircraft.adsb) return;
            
            const position = [this.selectedAircraft.adsb.lat, this.selectedAircraft.adsb.lon];
            this.mapManager.drawProximityCircle(position, this.proximityDistance);
        },

        highlightProximityAircraft() {
            if (!this.mapManager) return;
            
            // Create a set of hex codes for quick lookup
            const proximityHexSet = new Set(this.proximityAircraft.map(a => a.hex));
            
            // Call the map manager to highlight these aircraft
            this.mapManager.highlightProximityAircraft(proximityHexSet);
        },

        removeProximityHighlighting() {
            if (!this.mapManager) return;
            
            this.mapManager.removeProximityHighlighting();
        },

        // Navigate to Tracks tab and highlight a specific row by ADSB ID
        navigateToTracksWithHighlight(adsbId) {
            if (!adsbId) return;
            
            this.setStoryTab('track');
            
            // Store the ADSB ID to highlight
            this.highlightedAdsbId = adsbId;
            
            // Wait for the view to render, then scroll to the highlighted row
            setTimeout(() => {
                this.scrollToHighlightedRow(adsbId);
            }, 100);
            
            // Clear highlight after 30 seconds
            setTimeout(() => {
                this.highlightedAdsbId = null;
            }, 30000);
        },

        // Scroll to the highlighted row in the Tracks table
        // The row of one received position in the Track tab, brought into view
        // (a phase's point in the Details tab leads there). Each row carries its
        // position's id; the one wanted may be folded among unchanged ones.
        scrollToHighlightedRow(adsbId) {
            if (!adsbId) return;
            setTimeout(() => {
                const row = document.querySelector(`#story-track tr[data-adsb-id="${adsbId}"]`);
                if (row) row.scrollIntoView({ block: 'center', behavior: window.smoothScroll() });
            }, 300);
        },

        // Highlight a specific position on the map when hovering over history row
        highlightPositionOnMap(position) {
            if (!position || typeof position.lat !== 'number' || typeof position.lon !== 'number' || !this.mapManager) return;
            
            // Create a temporary marker for the highlighted position
            this.mapManager.showPositionHighlight(position.lat, position.lon, {
                altitude: position.altitude,
                timestamp: position.timestamp,
                speed_gs: position.speed_gs,
                speed_true: position.speed_true,
                heading: position.true_heading
            });
        },

        // Clear the position highlight from the map
        clearPositionHighlight() {
            if (!this.mapManager) return;
            this.mapManager.clearPositionHighlight();
        },

        // Phase history is now loaded from the tracks API response (aircraftDetailsLoadTracks).
        // No separate fetch or refresh needed.

        // Stop automatic refresh for phase history
        stopPhaseHistoryRefresh() {
            if (this.phaseHistoryRefreshInterval) {
                clearInterval(this.phaseHistoryRefreshInterval);
                this.phaseHistoryRefreshInterval = null;
            }
        },
        
        aircraftDetailsGetAltitudeTrend(position, index, isFuture = false) {
            if (!position || typeof position !== 'object') {
                return 'fas fa-arrows-alt-h';
            }

            // Use vertical_speed if available (from new tracks API)
            if (position.vertical_speed !== undefined && position.vertical_speed !== null) {
                if (position.vertical_speed > 100) return 'fas fa-arrow-up';
                if (position.vertical_speed < -100) return 'fas fa-arrow-down';
                return 'fas fa-arrows-alt-h';
            }
            
            // Fallback to altitude difference calculation for old data
            const dataArray = isFuture ? this.aircraftDetailsFutureData : this.aircraftDetailsHistoryData;
            
            if (!dataArray || index === dataArray.length - 1) return 'fas fa-arrows-alt-h';
            const nextPosition = dataArray[index + 1]; // Next is actually previous in time for history, or next in time for future
            if (!nextPosition) return 'fas fa-arrows-alt-h';
            
            // Check if we're using the new PositionMinimal format or the old Position format
            const currentAlt = position.alt_baro !== undefined ? position.alt_baro : position.altitude;
            const nextAlt = nextPosition.alt_baro !== undefined ? nextPosition.alt_baro : nextPosition.altitude;
            if (typeof currentAlt !== 'number' || typeof nextAlt !== 'number') return 'fas fa-arrows-alt-h';
            
            const altDiff = currentAlt - nextAlt;
            if (altDiff > 100) return 'fas fa-arrow-up';
            if (altDiff < -100) return 'fas fa-arrow-down';
            return 'fas fa-arrows-alt-h';
        },

        aircraftDetailsGetAltitudeTrendClass(position, index, isFuture = false) {
            if (!position || typeof position !== 'object') {
                return isFuture ? 'text-highlight/70' : 'text-text';
            }

            const dataArray = isFuture ? this.aircraftDetailsFutureData : this.aircraftDetailsHistoryData;
            
            if (!dataArray || index === dataArray.length - 1) return isFuture ? 'text-highlight/70' : 'text-text';
            const nextPosition = dataArray[index + 1]; // Next is actually previous in time for history, or next in time for future
            if (!nextPosition) return isFuture ? 'text-highlight/70' : 'text-text';
            
            // Check if we're using the new PositionMinimal format or the old Position format
            const currentAlt = position.alt_baro !== undefined ? position.alt_baro : position.altitude;
            const nextAlt = nextPosition.alt_baro !== undefined ? nextPosition.alt_baro : nextPosition.altitude;
            if (typeof currentAlt !== 'number' || typeof nextAlt !== 'number') return isFuture ? 'text-highlight/70' : 'text-text';
            
            const altDiff = currentAlt - nextAlt;
            if (altDiff > 100) return isFuture ? 'text-green-400' : 'text-highlight';
            if (altDiff < -100) return isFuture ? 'text-red-400' : 'text-danger';
            return isFuture ? 'text-highlight/70' : 'text-text';
        },

        formatTrackSpeed(value) {
            if (typeof value !== 'number' || !Number.isFinite(value)) return '-';
            return Math.round(value);
        },

        formatTrackDistance(value) {
            if (typeof value !== 'number' || !Number.isFinite(value)) return '-';
            return value;
        },

        // Check if there's a significant time gap (>10 minutes) between this position and the previous one
        hasSignificantTimeGap(position, index, isFuture = false) {
            const dataArray = isFuture ? this.aircraftDetailsFutureData : this.aircraftDetailsHistoryData;
            
            if (index === 0) return false; // First item has no previous to compare
            
            const prevPosition = dataArray[index - 1];
            if (!prevPosition || !prevPosition.timestamp || !position.timestamp) {
                return false;
            }
            
            const currentTime = new Date(position.timestamp);
            const prevTime = new Date(prevPosition.timestamp);
            const timeDiffMinutes = Math.abs(currentTime - prevTime) / (1000 * 60); // Convert to minutes
            
            return timeDiffMinutes > 10; // Return true if gap is more than 10 minutes
        },

        // Deselect, and the map with it (store/story.js, stopFollowing, does
        // the rest).
        closeAircraftPanel() {
            this.selectedAircraft = null;
            if (this.mapManager) {
                if (typeof this.mapManager.removeProximityCircle === 'function') this.mapManager.removeProximityCircle();
                if (typeof this.mapManager.removeProximityHighlighting === 'function') this.mapManager.removeProximityHighlighting();
                if (typeof this.mapManager.applyFiltersAndRefreshView === 'function') this.mapManager.applyFiltersAndRefreshView({ immediate: true });
            }
        },
        
        // Select an aircraft by hex ID (for right-clicking on alerts)
        selectAircraftByHex(hex) {
            if (!hex) return;
            
            console.log(`Attempting to select aircraft with hex: ${hex}`);
            
            // Find the aircraft with the matching hex
            const foundAircraft = this.aircraft[hex];
            
            if (foundAircraft) {
                console.log(`Found aircraft with hex ${hex}:`, foundAircraft);
                
                // Check if we need to enable the appropriate filter to show the aircraft
                const needsGroundFilter = foundAircraft.on_ground && !this.settings.showGroundAircraft;
                const needsAirFilter = !foundAircraft.on_ground && !this.settings.showAirAircraft;
                
                if (needsGroundFilter) {
                    this.settings.showGroundAircraft = true;
                    this.saveSettings();
                    console.log('Enabled Ground filter to show selected aircraft');
                    // Apply the new filter
                    if (this.mapManager) {
                        this.mapManager.applyFiltersAndRefreshView();
                    }
                } else if (needsAirFilter) {
                    this.settings.showAirAircraft = true;
                    this.saveSettings();
                    console.log('Enabled Air filter to show selected aircraft');
                    // Apply the new filter
                    if (this.mapManager) {
                        this.mapManager.applyFiltersAndRefreshView();
                    }
                }
                
                // Set as selected aircraft
                this.selectedAircraft = foundAircraft;
                
                // Update aircraft details panel
                this.setupAircraftDetailsPanel();
                
                // Update map to highlight the aircraft
                if (this.mapManager) {
                    this.mapManager.updateVisualState(foundAircraft.hex, true);
                    
                    // Center map on the aircraft if it has coordinates
                    if (foundAircraft.adsb && foundAircraft.adsb.lat && foundAircraft.adsb.lon && this.mapManager.centerOnAircraft) {
                        this.mapManager.centerOnAircraft(foundAircraft);
                    }
                }
            } else {
                console.warn(`Could not find aircraft with hex: ${hex}`);
            }
        },

        formatPredictionTime(position) {
            if (!position || !position.timestamp) return '';
            const secs = Math.max(0, Math.round((new Date(position.timestamp).getTime() - Date.now()) / 1000));
            const m = Math.floor(secs / 60);
            const s = secs % 60;
            if (s === 0 && m > 0) return `+${m} min`;
            if (m === 0) return `+${secs}s`;
            return `+${m}:${String(s).padStart(2, '0')}`;
        },

        getFutureDataForTable() {
            // Show every 3rd point (15s intervals from 5s granularity data)
            return this.aircraftDetailsFutureData.filter((_, i) => (i + 1) % 3 === 0);
        },

        // Build flat array for history table rendering from server-deduplicated data.
        // Server provides skipped_before/skipped_after on positions; we convert to
        // { type: 'row', position, originalIndex, beforeDivider } and { type: 'divider', count } entries.
        getFilteredHistoryData() {
            const data = this.aircraftDetailsHistoryData;
            if (!data || data.length === 0) return [];

            const result = [];
            for (let i = 0; i < data.length; i++) {
                const pos = data[i];

                // Divider before this row (skipped duplicates between prev and this)
                if (pos.skipped_before > 0) {
                    // Mark the previous row as being before a divider (suppress its border)
                    if (result.length > 0 && result[result.length - 1].type === 'row') {
                        result[result.length - 1].beforeDivider = true;
                    }
                    result.push({ type: 'divider', count: pos.skipped_before });
                }

                result.push({ type: 'row', position: pos, originalIndex: i, beforeDivider: false });

                // Trailing divider after last row
                if (pos.skipped_after > 0) {
                    result[result.length - 1].beforeDivider = true;
                    result.push({ type: 'divider', count: pos.skipped_after });
                }
            }

            return result;
        },

        getHeadingSuffix(type) {
            switch (type) {
                case 'magnetic':
                    return 'hdg(m)';
                case 'track':
                    return 'hdg(trk)';
                case 'true':
                    return 'hdg(t)';
                default:
                    return '-';
            }
        },
    }));
})();
