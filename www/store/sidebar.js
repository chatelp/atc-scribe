// The list of aircraft: filters, sorting, tabs, the altitude read on the QNH,
// headings, the phase and heading help.
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
        filteredAircraft: [],
        searchTerm: '',
        // Computed aircraft counts — derived from the live aircraft store so they stay
        // in sync as WebSocket adds/updates/removes aircraft at runtime.
        get counts() {
            const now = Date.now();
            const lastSeenCutoff = now - ((this.settings?.lastSeenMinutes || 10) * 60 * 1000);
            let ground_active = 0, ground_total = 0, air_active = 0, air_total = 0;

            for (const ac of Object.values(this.aircraft)) {
                if (ac.last_seen && new Date(ac.last_seen).getTime() < lastSeenCutoff) continue;
                if (ac.on_ground) {
                    ground_total++;
                    if (ac.status === 'active') ground_active++;
                } else {
                    air_total++;
                    if (ac.status === 'active') air_active++;
                }
            }
            return { ground_active, ground_total, air_active, air_total };
        },
        hoveredAircraft: null,
        sidebarTab: localStorage.getItem('sidebarAircraftTab') || 'active',
        sortColumn: 'callsign',
        sortDirection: 'asc',

        // Caching properties for filteredAircraft performance optimization
        _filteredAircraftCache: null,
        _lastFilterHash: null,

        // Computed
        get aircraftCount() {
            return Object.keys(this.aircraft).length;
        },

        isSignalLostForSidebar(aircraft) {
            if (!aircraft) return false;

            // Strict source of truth: classification is time-based from last ADS-B poll.
            const secondsSince = this.getSecondsSinceLastSeen(aircraft);
            return Number.isFinite(secondsSince) && secondsSince >= 60;
        },

        get activeAircraftCount() {
            this.currentTime;
            return this.filteredAircraft.filter((aircraft) => !this.isSignalLostForSidebar(aircraft)).length;
        },

        get lostAircraftCount() {
            this.currentTime;
            return this.filteredAircraft.filter((aircraft) => this.isSignalLostForSidebar(aircraft)).length;
        },

        get visibleSidebarAircraft() {
            this.currentTime;
            if (this.sidebarTab === 'lost') {
                return this.filteredAircraft.filter((aircraft) => this.isSignalLostForSidebar(aircraft));
            }
            return this.filteredAircraft.filter((aircraft) => !this.isSignalLostForSidebar(aircraft));
        },

        setSidebarTab(tab) {
            this.sidebarTab = tab === 'lost' ? 'lost' : 'active';
            localStorage.setItem('sidebarAircraftTab', this.sidebarTab);
        },

        // Helper function to safely get current phase
        getCurrentPhase(aircraft) {
            if (!aircraft) return 'NEW';
            return (aircraft.phase && aircraft.phase.current && aircraft.phase.current.length > 0)
                ? aircraft.phase.current[0].phase
                : 'NEW';
        },

        get filteredAircraft() {
            // CRITICAL FIX: Maintain stable array reference to prevent full table re-renders
            if (!this._filteredAircraftCache) {
                this._filteredAircraftCache = [];
            }

            // If no filtering has been done yet, do it immediately (synchronously for first load)
            if (!this._lastFilterHash) {
                this._performFiltering();
                return this._filteredAircraftCache;
            }
 
            // Return stable array reference
            return this._filteredAircraftCache;
        },
        
        // Perform heavy filtering computation asynchronously
        _performFiltering() {
            // Create lightweight hash of current filter state (include ALL filter settings)
            const phaseFilterHash = this.settings.phaseFilters ? Object.entries(this.settings.phaseFilters).map(([k, v]) => `${k}:${v}`).join(',') : '';
            const filterHash = `${this.settings.voiceOnly}|${this.searchTerm}|${this.settings.showGroundAircraft}|${this.settings.showAirAircraft}|${this.settings.minAltitude}|${this.settings.maxAltitude}|${this.altimetry.qnh}|${this.settings.lastSeenMinutes}|${this.settings.listSort}|${phaseFilterHash}|${Object.keys(this.aircraft).length}|${this.selectedAircraft?.hex}|${this.sortColumn}|${this.sortDirection}`;

            // Return cached result if nothing changed (but always filter if no cache exists)
            if (this._lastFilterHash === filterHash && this._filteredAircraftCache) {
                return;
            }

            const searchLower = this.searchTerm.toLowerCase();
            const now = new Date();
            const lastSeenMinutes = Number(this.settings.lastSeenMinutes) || 10;
            const lastSeenCutoff = new Date(now.getTime() - (lastSeenMinutes * 60 * 1000));

            const filtered = Object.values(this.aircraft).filter(aircraft => {
                // Keep only aircraft the radio has named, when asked. The voice
                // field is attached server-side from the transcription callsigns.
                if (this.settings.voiceOnly && !(aircraft.voice && aircraft.voice.transmissions > 0)) {
                    return false;
                }

                // Filter by last seen time - hide aircraft not seen recently
                if (aircraft.last_seen) {
                    const lastSeenDate = new Date(aircraft.last_seen);
                    if (lastSeenDate < lastSeenCutoff) {
                        return false;
                    }
                }

                // Filter by search term - includes callsign, type, category, manufacturer
                if (searchLower) {
                    const callsign = (aircraft.flight || aircraft.hex).toLowerCase();
                    const type = (aircraft.adsb?.type || '').toLowerCase();
                    const category = (aircraft.adsb?.category || '').toLowerCase();
                    const manufacturer = (aircraft.bsdb?.manufacturer || '').toLowerCase();
                    const bsdbType = (aircraft.bsdb?.type || '').toLowerCase();

                    const matchesSearch = callsign.includes(searchLower) ||
                                        type.includes(searchLower) ||
                                        category.includes(searchLower) ||
                                        manufacturer.includes(searchLower) ||
                                        bsdbType.includes(searchLower);

                    if (!matchesSearch) return false;
                }

                // Filter by air/ground settings - both can be enabled/disabled independently
                const showThisAircraft = (aircraft.on_ground && this.settings.showGroundAircraft) ||
                                        (!aircraft.on_ground && this.settings.showAirAircraft);

                if (!showThisAircraft) {
                    return false;
                }

                // Filter by flight phase
                const currentPhase = this.getCurrentPhase(aircraft);

                if (this.settings.phaseFilters && this.settings.phaseFilters[currentPhase] === false) {
                    return false;
                }

                // Filter by altitude (client-side for immediate table update)
                // Only apply altitude filter to aircraft in the air
                const altFt = aircraft.on_ground ? null : this.altitudeFt(aircraft);
                if (!aircraft.on_ground && (altFt == null || altFt < this.settings.minAltitude || altFt > this.settings.maxAltitude)) {
                    return false;
                }

                return true;
            });

            const sortKey = this.settings.listSort || 'callsign_az';
            const sorted = filtered.sort((a, b) => this.compareAircraftForSidebar(a, b, sortKey));
            
            // CRITICAL FIX: Update array in-place to maintain stable reference and prevent full table re-render
            this._updateFilteredAircraftInPlace(sorted);
            this._lastFilterHash = filterHash;
            this._lastFilterTime = Date.now();
        },

        // Update filtered aircraft array in-place to maintain stable reference
        _updateFilteredAircraftInPlace(newFiltered) {
            if (!this._filteredAircraftCache) {
                this._filteredAircraftCache = [];
            }

            // Create maps for efficient lookups
            const currentMap = new Map(this._filteredAircraftCache.map((aircraft, index) => [aircraft.hex, { aircraft, index }]));
            const newMap = new Map(newFiltered.map(aircraft => [aircraft.hex, aircraft]));

            // Track aircraft that need animations
            const addedAircraft = [];
            const removedAircraft = [];

            // Remove aircraft that are no longer in the filtered list
            for (let i = this._filteredAircraftCache.length - 1; i >= 0; i--) {
                const aircraft = this._filteredAircraftCache[i];
                if (!newMap.has(aircraft.hex)) {
                    removedAircraft.push(aircraft);
                    this._filteredAircraftCache.splice(i, 1);
                }
            }

            // Update existing aircraft and add new ones
            const finalArray = [];
            for (const newAircraft of newFiltered) {
                const existingEntry = currentMap.get(newAircraft.hex);
                
                if (existingEntry) {
                    // Update existing aircraft in-place
                    Object.assign(existingEntry.aircraft, newAircraft);
                    finalArray.push(existingEntry.aircraft);
                } else {
                    // Add new aircraft
                    addedAircraft.push(newAircraft);
                    finalArray.push(newAircraft);
                }
            }

            // Replace array contents while maintaining reference
            this._filteredAircraftCache.length = 0;
            this._filteredAircraftCache.push(...finalArray);

            // Trigger animations for added/removed aircraft
            this._animateAircraftChanges(addedAircraft, removedAircraft);
        },

        // Animate aircraft appearing and disappearing
        _animateAircraftChanges(addedAircraft, removedAircraft) {
            // Animate new aircraft appearing
            addedAircraft.forEach(aircraft => {
                setTimeout(() => {
                    const row = document.querySelector(`tr[data-aircraft-hex="${aircraft.hex}"]`);
                    if (row) {
                        row.style.opacity = '0';
                        row.style.transform = 'translateX(-20px)';
                        row.style.transition = 'opacity 0.3s ease, transform 0.3s ease';
                        
                        // Trigger animation
                        requestAnimationFrame(() => {
                            row.style.opacity = '1';
                            row.style.transform = 'translateX(0)';
                        });
                    }
                }, 50);
            });

            // Animate removed aircraft disappearing
            removedAircraft.forEach(aircraft => {
                const row = document.querySelector(`tr[data-aircraft-hex="${aircraft.hex}"]`);
                if (row) {
                    row.style.transition = 'opacity 0.3s ease, transform 0.3s ease';
                    row.style.opacity = '0';
                    row.style.transform = 'translateX(20px)';
                }
            });
        },

        getSortValue(aircraft, column) {
            switch (column) {
                case 'callsign_az':
                case 'callsign':
                    return (aircraft.flight || aircraft.hex).toLowerCase();
                case 'phase':
                    return this.getCurrentPhase(aircraft);
                case 'altitude':
                    return this.altitudeFt(aircraft) ?? Number.NEGATIVE_INFINITY;
                case 'heading':
                    return this.getHeadingWithType(aircraft).value ?? -1;
                case 'speed':
                    return (typeof aircraft.adsb?.tas === 'number' && Number.isFinite(aircraft.adsb.tas))
                        ? aircraft.adsb.tas
                        : Number.NEGATIVE_INFINITY;
                case 'gs':
                    return (typeof aircraft.adsb?.gs === 'number' && Number.isFinite(aircraft.adsb.gs))
                        ? aircraft.adsb.gs
                        : Number.NEGATIVE_INFINITY;
                case 'distance':
                    return aircraft.distance ?? 999999; // Sort undefined distances to the end
                case 'last_seen': {
                    if (!aircraft.last_seen) return Number.NEGATIVE_INFINITY;
                    const ts = new Date(aircraft.last_seen).getTime();
                    return Number.isFinite(ts) ? ts : Number.NEGATIVE_INFINITY;
                }
                default:
                    return '';
            }
        },

        compareAircraftForSidebar(a, b, column) {
            const aValue = this.getSortValue(a, column);
            const bValue = this.getSortValue(b, column);

            const ascendingColumns = new Set(['distance', 'callsign_az', 'callsign']);
            const direction = ascendingColumns.has(column) ? 1 : -1;

            if (aValue < bValue) return -1 * direction;
            if (aValue > bValue) return 1 * direction;

            const aCallsign = (a.flight || a.hex || '').toLowerCase();
            const bCallsign = (b.flight || b.hex || '').toLowerCase();
            if (aCallsign < bCallsign) return -1;
            if (aCallsign > bCallsign) return 1;
            return 0;
        },

        // RESTORING getAltitudeTrendClasses and getAltitudeTrendIcon
        getAltitudeTrendClasses(aircraft) {
            if (!aircraft || !aircraft.adsb || typeof aircraft.adsb.baro_rate === 'undefined') return 'text-text'; // Default
            if (aircraft.adsb.baro_rate > 100) return 'text-highlight'; // Climbing - green
            if (aircraft.adsb.baro_rate < -100) return 'text-danger'; // Descending - red
            return 'text-text'; // Level
        },

        getAltitudeTrendIcon(aircraft) {
            if (!aircraft || !aircraft.adsb || typeof aircraft.adsb.baro_rate === 'undefined') return 'fas fa-arrows-alt-h'; // Default - level
            if (aircraft.adsb.baro_rate > 100) return 'fas fa-arrow-up'; // Climbing
            if (aircraft.adsb.baro_rate < -100) return 'fas fa-arrow-down'; // Descending
            return 'fas fa-arrows-alt-h'; // Level
        },

        // An aircraft's altitude as it is said: feet on the QNH below the
        // transition altitude, flight level above (null without altitude).
        altitudeReading(aircraft) {
            return window.AltitudeScale.reading(aircraft?.adsb?.alt_baro, this.altimetry);
        },

        // The altitude the filters compare, in feet: on the QNH below the
        // transition altitude, pressure altitude (the flight level) above.
        altitudeFt(aircraft) {
            const r = this.altitudeReading(aircraft);
            return r ? r.ft : null;
        },

        // For the list and the aircraft panel: "2 900" (ft) or "FL120".
        altitudeValueText(aircraft) {
            const r = this.altitudeReading(aircraft);
            if (!r) return '-';
            if (r.kind !== 'ft') return window.AltitudeScale.readingText(r);
            return String(Math.round(r.ft / 100) * 100);
        },

        // A past position's altitude, read the same way with today's QNH.
        altitudeTextFromBaro(altBaro) {
            const r = window.AltitudeScale.reading(altBaro, this.altimetry);
            if (!r) return '-';
            return r.kind !== 'ft' ? window.AltitudeScale.readingText(r) : String(Math.round(r.ft / 100) * 100);
        },

        // What the altitude means, for its help bubble.
        altitudeHelp(aircraft) {
            const r = this.altitudeReading(aircraft);
            const a = this.altimetry;
            const qnh = a.estimated
                ? `QNH ${Math.round(a.qnh)} hPa, the setting ${a.count} aircraft near the station report`
                : 'no aircraft nearby reports its QNH yet, so the standard 1013 hPa is used';
            const ta = `${a.ta} ft`;
            const raw = aircraft?.adsb?.alt_baro;
            if (!r) return 'No altitude received.';
            if (r.kind === 'gnd') return 'The aircraft reports that it is on the ground.';
            if (r.kind === 'fl') {
                return `Flight level ${r.fl}: ${raw} ft on the standard pressure setting (1013 hPa). ` +
                    `Above the transition altitude (${ta}) every aircraft sets 1013 and is given a flight level, ` +
                    'so all of them read the same scale whatever the weather.';
            }
            return `About ${Math.round(r.ft / 10) * 10} ft above sea level, on the local pressure setting (${qnh}). ` +
                `Below the transition altitude (${ta}) altitudes are given in feet on the QNH. ` +
                `The aircraft itself transmits ${raw} ft, measured on 1013 hPa.`;
        },

        getAltitudeUnitLabel(aircraft) {
            const altitude = aircraft?.adsb?.alt_baro;
            if (typeof altitude !== 'number' || !Number.isFinite(altitude)) return '';

            const baroRate = aircraft?.adsb?.baro_rate;
            if (typeof baroRate === 'number' && Number.isFinite(baroRate)) {
                const roundedRate = Math.round(baroRate);
                if (roundedRate > 100) return `+${roundedRate} ft`;
                if (roundedRate < -100) return `${roundedRate} ft`;
            }

            return this.altitudeReading(aircraft)?.kind === 'ft' ? 'ft' : '';
        },

        // Toggle Air aircraft visibility
        toggleAirAircraft() {
            console.log('[Alpine Store] toggleAirAircraft called, current state:', this.settings.showAirAircraft);
            this.settings.showAirAircraft = !this.settings.showAirAircraft;

            this.saveSettings();
            this.applyFilters();

            // Update map visibility including tracks
            if (this.mapManager) {
                this.mapManager.applyFiltersAndRefreshView();
            }
        },

        // Toggle Ground aircraft visibility
        toggleGroundAircraft() {
            //console.log('[Alpine Store] toggleGroundAircraft called, current state:', this.settings.showGroundAircraft);
            this.settings.showGroundAircraft = !this.settings.showGroundAircraft;

            this.saveSettings();
            this.applyFilters();

            // Update map visibility including tracks
            if (this.mapManager) {
                this.mapManager.applyFiltersAndRefreshView();
            }
        },

        // Toggle the "named by the radio" filter
        toggleVoiceOnly() {
            this.settings.voiceOnly = !this.settings.voiceOnly;
            this.saveSettings();
            this.applyFilters();
            if (this.mapManager) {
                this.mapManager.applyFiltersAndRefreshView();
            }
        },

        // How many aircraft the radio has named, of those currently held
        get voiceAircraftCount() {
            return Object.values(this.aircraft).filter(a => a.voice && a.voice.transmissions > 0).length;
        },

        // Toggle flight phase filter
        togglePhaseFilter(phase) {
            if (!this.settings.phaseFilters) {
                this.settings.phaseFilters = { CRZ: true, CLB: true, DEP: true, APP: true, ARR: true, TAX: true, 'T/O': true, 'T/D': true, NEW: true, UNK: true };
            }
            this.settings.phaseFilters[phase] = !this.settings.phaseFilters[phase];
            this.saveSettings();
            this.applyFilters();

            // Update map visibility including tracks
            if (this.mapManager) {
                this.mapManager.applyFiltersAndRefreshView();
            }
        },

        // Get phase color class (matches navigation bar colors)
        getPhaseColorClass(phase) {
            const phaseColorMap = {
                'NEW': 'text-gray-400',
                'TAX': 'text-purple-400',
                'T/O': 'text-orange-400',
                'CLB': 'text-lime-400',
                'DEP': 'text-green-400',
                'CRZ': 'text-blue-400',
                'ARR': 'text-pink-300',
                'APP': 'text-yellow-400',
                'T/D': 'text-teal-400',
                'UNK': 'text-slate-400'
            };
            return phaseColorMap[phase] || 'text-gray-400';
        },

        // Get phase icon class
        getPhaseIconClass(phase) {
            const phaseIconMap = {
                'NEW': 'fa-plane',
                'TAX': 'fa-taxi',
                'T/O': 'fa-plane-departure',
                'DEP': 'fa-plane-up',
                'CRZ': 'fa-plane',
                'ARR': 'fa-plane-down',
                'APP': 'fa-plane-arrival',
                'T/D': 'fa-plane-arrival'
            };
            return phaseIconMap[phase] || 'fa-plane';
        },

        // Get seconds since last seen for an aircraft
        getSecondsSinceLastSeen(aircraft) {
            if (!aircraft.last_seen) return 'Unknown';

            // Reactive dependency so Alpine reevaluates this value every second
            // when the app clock updates.
            this.currentTime;

            const lastSeen = new Date(aircraft.last_seen);
            const now = new Date();
            return Math.floor((now - lastSeen) / 1000);
        },

        shouldShowLastSeenBadge(aircraft) {
            const secondsSince = this.getSecondsSinceLastSeen(aircraft);
            return Number.isFinite(secondsSince) && secondsSince >= 5;
        },

        // Highlight search matches with red underline
        highlightSearchMatch(text) {
            if (!this.searchTerm || !text) return text;
            
            const searchLower = this.searchTerm.toLowerCase();
            const textLower = text.toLowerCase();
            const index = textLower.indexOf(searchLower);
            
            if (index === -1) return text;
            
            const before = text.substring(0, index);
            const match = text.substring(index, index + this.searchTerm.length);
            const after = text.substring(index + this.searchTerm.length);
            
            return `${before}<span class="border-b border-red-400">${match}</span>${after}`;
        },

        // Select the next (step 1) or previous (step -1) aircraft of the filtered
        // list, wrapping around. With none selected, or the selected one filtered
        // out, start from the first (next) or the last (previous).
        cycleAircraft(step) {
            const filtered = this.filteredAircraft;
            if (filtered.length === 0) return;

            const currentIndex = this.selectedAircraft
                ? filtered.findIndex(aircraft => aircraft.hex === this.selectedAircraft.hex)
                : -1;
            const index = currentIndex === -1
                ? (step > 0 ? 0 : filtered.length - 1)
                : (currentIndex + step + filtered.length) % filtered.length;
            this.selectedAircraft = filtered[index];

            if (this.mapManager) {
                this.mapManager.updateVisualState(this.selectedAircraft.hex, true);
                this.mapManager.centerOnAircraft(this.selectedAircraft);
            }
        },
        cycleToNextAircraft() { this.cycleAircraft(1); },
        cycleToPreviousAircraft() { this.cycleAircraft(-1); },

        applyFilters() {
            this.onFilterChange();
        },

        phaseHelp(phase) {
            const P = window.FlightTimeline && window.FlightTimeline.PHASES;
            const [name, what] = (P && P[phase]) || [phase, ''];
            const on = !(this.settings.phaseFilters && this.settings.phaseFilters[phase] === false);
            return `${phase}, ${name}${what ? ': ' + what : ''}. Judged by the server from altitude, speed and position. ` +
                `${on ? 'Shown' : 'Hidden'} now: click to ${on ? 'hide' : 'show'} the aircraft in this phase.`;
        },

        headingHelp(type) {
            if (type === 'magnetic') return 'hdg(m): the heading the aircraft reports, where its nose points, from magnetic north -- the one controllers give.';
            if (type === 'true') return 'hdg(t): where its nose points, from true north.';
            if (type === 'track') return 'hdg(trk): no heading reported, so its track over the ground is shown instead; the wind makes the two differ by a few degrees.';
            return 'No heading received.';
        },

        // Heading helpers for map icon rotation and label suffixes
        normalizeHeading(value) {
            if (!Number.isFinite(value)) return null;
            return ((value % 360) + 360) % 360;
        },

        hasHeadingValue(value) {
            return value !== undefined && value !== null && Number.isFinite(value);
        },

        _isLikelyDefaultZeroHeading(value, adsbLike) {
            if (!this.hasHeadingValue(value)) return false;
            if (Math.abs(value) > 0.0001) return false;

            const track = this.hasHeadingValue(adsbLike?.track) ? this.normalizeHeading(adsbLike.track) : null;
            const trueHeading = this.hasHeadingValue(adsbLike?.true_heading) ? this.normalizeHeading(adsbLike.true_heading) : null;
            const magHeading = this.hasHeadingValue(adsbLike?.mag_heading) ? this.normalizeHeading(adsbLike.mag_heading) : null;

            const hasNonZeroTrack = track !== null && Math.abs(track) > 0.0001;
            const hasNonZeroTrue = trueHeading !== null && Math.abs(trueHeading) > 0.0001;
            const hasNonZeroMag = magHeading !== null && Math.abs(magHeading) > 0.0001;

            return hasNonZeroTrack || hasNonZeroTrue || hasNonZeroMag;
        },

        _getResolvedHeadingCandidates(adsbLike) {
            const magHeading = this.hasHeadingValue(adsbLike?.mag_heading)
                ? this.normalizeHeading(adsbLike.mag_heading)
                : null;
            const track = this.hasHeadingValue(adsbLike?.track)
                ? this.normalizeHeading(adsbLike.track)
                : null;
            const trueHeading = this.hasHeadingValue(adsbLike?.true_heading)
                ? this.normalizeHeading(adsbLike.true_heading)
                : null;

            const magIsUsable = magHeading !== null && !this._isLikelyDefaultZeroHeading(magHeading, adsbLike);
            const trueIsUsable = trueHeading !== null && !this._isLikelyDefaultZeroHeading(trueHeading, adsbLike);
            const trackIsUsable = track !== null;

            return {
                magHeading,
                track,
                trueHeading,
                magIsUsable,
                trackIsUsable,
                trueIsUsable
            };
        },

        // Get heading with fallback priority: magnetic -> track -> true
        getHeadingWithFallback(aircraft) {
            if (!aircraft?.adsb) return null;

            const candidates = this._getResolvedHeadingCandidates(aircraft.adsb);
            if (candidates.magIsUsable) return candidates.magHeading;
            if (candidates.trackIsUsable) return candidates.track;
            if (candidates.trueIsUsable) return candidates.trueHeading;
            return null;
        },

        getHeadingWithTypeFromAdsb(adsbLike) {
            if (!adsbLike) return { value: null, type: null };

            const candidates = this._getResolvedHeadingCandidates(adsbLike);
            if (candidates.magIsUsable) return { value: candidates.magHeading, type: 'magnetic' };
            if (candidates.trackIsUsable) return { value: candidates.track, type: 'track' };
            if (candidates.trueIsUsable) return { value: candidates.trueHeading, type: 'true' };

            return { value: null, type: null };
        },

        getHeadingWithType(aircraft) {
            if (!aircraft?.adsb) return { value: null, type: null };
            return this.getHeadingWithTypeFromAdsb(aircraft.adsb);
        },

        getHeadingDisplayTextFromAdsb(adsbLike) {
            const heading = this.getHeadingWithTypeFromAdsb(adsbLike);
            if (heading.value === null) return '-';
            return `${Math.round(heading.value)}°`;
        },

        getHeadingDisplayText(aircraft) {
            const heading = this.getHeadingWithType(aircraft);
            if (heading.value === null) return '-';
            return `${Math.round(heading.value)}°`;
        },
    }));
})();
