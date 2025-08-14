// Base API URL - dynamically use the current port
const API_BASE_URL = `${window.location.protocol}//${window.location.hostname}:${window.location.port}/api/v1`;

// Configuration
const CONFIG = {
    // defaultCenter: [43.6777, -79.6248], // Will be fetched from API
    defaultZoom: 10,
    dataUrl: `${API_BASE_URL}/aircraft`,
    wsUrl: `ws://${window.location.hostname}:${window.location.port}/api/v1/ws`, // WebSocket URL
    useRealData: true,
    useSampleData: true,
    rangeRings: [5, 10, 25, 50, 100],
    selectedFadeOpacity: 0.4, // Opacity for non-selected items when one is selected
    
    // Refresh intervals (in milliseconds)
    stationRefreshInterval: 30 * 60 * 1000,  // 30 minutes for station data
    weatherRefreshInterval: 30 * 60 * 1000,  // 30 minutes for weather data
};

// Initialize WebSocket client
const wsClient = new WebSocketClient(CONFIG.wsUrl);

// Declare Audio client - will be initialized in alpine:init
let audioClient;
// Declare Map Manager - will be initialized in alpine:init
let mapManager;
// Declare Animation Engine - will be initialized in alpine:init
let animationEngine;

// Leaflet map instance and layers
let isAppInitialized = false; // Flag to ensure init() runs only once

// Alpine.js data store
document.addEventListener('alpine:init', () => {
    Alpine.store('atc', {
        // State
        aircraft: [],
        filteredAircraft: [],
        searchTerm: '',
        count_active: 0,
        count_stale: 0,
        count_signal_lost: 0,
        counts: {
            ground_active: 0,
            ground_total: 0,
            air_active: 0,
            air_total: 0
        },
        visibleAircraftOnMap: new Set(), // Track aircraft visible on map for UI indicators
        audioFrequencies: [],
        clientID: crypto.randomUUID(), // Unique client ID for audio streams
        unmutedFrequencies: new Set(), // Set of unmuted frequency IDs
        audioElements: {}, // Map of frequency ID to audio element
        audioAnalysers: {}, // Map of frequency ID to analyser node
        audioDataArrays: {}, // Map of frequency ID to data array
        visualizationFrameIds: {}, // Map of frequency ID to animation frame ID
        sourceNodes: {},
        
        // Request throttling state
        pendingRequests: {
            aircraft: false,
            tracks: new Map(), // Map of hex -> boolean for pending tracks requests
            proximity: false,
            phaseHistory: new Map(), // Map of hex -> boolean for pending phase history requests
        },

        // Clear pending requests for a specific aircraft
        clearPendingRequestsForAircraft(hex) {
            this.pendingRequests.tracks.delete(hex);
            this.pendingRequests.phaseHistory.delete(hex);
        },

        // Clear all pending requests
        clearAllPendingRequests() {
            this.pendingRequests.aircraft = false;
            this.pendingRequests.tracks.clear();
            this.pendingRequests.proximity = false;
            this.pendingRequests.phaseHistory.clear();
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
        radiosStarted: false, // Initialize radiosStarted to false
        wsConnection: null, // WebSocket connection
        transcriptions: [], // Array of transcription messages
        aircraftAlerts: [], // Array of aircraft movement alerts
        audioApiUrl: `${API_BASE_URL}/frequencies`,
        transcriptionSearchTerm: '', // For searching transcriptions
        showLostAircraftOnly: false, // Toggle for showing only lost aircraft
        originalTranscriptions: {}, // Store original transcriptions before filtering
        stationApiUrl: `${API_BASE_URL}/station`, // API URL for station data
        wxApiUrl: `${API_BASE_URL}/wx`, // API URL for weather data
        stationLatitude: null,
        stationLongitude: null,
        stationElevationFeet: null,
        stationAirportCode: null,
        // Station override state
        stationOverride: {
            latitude: null,
            longitude: null,
            active: false,
            mapClickMode: false,
            autoUpdate: false,
            updateInterval: 60, // seconds
            geolocationStatus: null,
            geolocationWatchId: null
        },
        // Weather configuration flags from station config
        stationFetchMETAR: false,
        stationFetchTAF: false,
        stationFetchNOTAMs: false,
        metar: null,
        taf: null,
        notams: null,
        weatherLastUpdated: null,
        weatherFetchErrors: [],
        runwayData: null, // Store runway data
        metarDetailsVisible: false,
        tafDetailsVisible: false,
        notamDetailsVisible: false,
        stationRefreshInterval: null,
        weatherRefreshInterval: null,
        initialDataLoaded: false,
        connected: null, // null = initial state, true = connected, false = connection lost
        lastUpdate: null,
        settingsCollapsed: true, // Hide settings panel by default
        selectedAircraft: null,
        showSplashScreen: true, // Show splash screen by default
        splashScreenAudioPlayed: false, // Track if the welcome sound has been played
        connectionLostSoundPlayed: false, // Track if the connection lost sound has been played
        // coordinates removed as they're not needed
        currentTime: new Date().toLocaleTimeString(),
        zuluTime: new Date().toUTCString().match(/(\d{2}:\d{2}:\d{2})/)[0] + 'Z', // Initial Zulu Time
        showLocalDates: localStorage.getItem('showLocalDates') === 'true' || false, // Default to UTC (false)
        hoveredAircraft: null,
        sortColumn: 'callsign',
        sortDirection: 'asc',
        lastUpdateSeconds: 0, // For footer status
        timeUpdateIntervalId: null, // Store ID for the time update interval
        userSetVolumes: {}, // Initialize as empty object
        lastSignificantAudioTime: {}, // Stores timestamp for each freqId
        secondsSinceLastAudio: {},  // Stores formatted string for display (e.g., "5s")
        lastAudioUpdateIntervalId: null, // Interval ID for updating secondsSinceLastAudio
        frequencyTranscriptions: {}, // Stores transcriptions per frequency_id
        transcriptionViewerVisible: {}, // Stores visibility state for each frequency's viewer
        isReconnecting: false, // Flag to prevent duplicate reconnection attempts

        // Settings
        settings: {
            showLabels: JSON.parse(localStorage.getItem('showLabels')) ?? true,
            showPaths: JSON.parse(localStorage.getItem('showPaths')) ?? true,
            showRings: JSON.parse(localStorage.getItem('showRings')) ?? true,
            minAltitude: parseInt(localStorage.getItem('minAltitude')) || 0,
            maxAltitude: parseInt(localStorage.getItem('maxAltitude')) || 60000,
            trailLength: parseInt(localStorage.getItem('trailLength')) || 2,
            tracksLimit: parseInt(localStorage.getItem('tracksLimit')) || 1000,
            lastSeenMinutes: parseInt(localStorage.getItem('lastSeenMinutes')) || 10, // Default to 10 minutes for aircraft disappearance
            statusFilters: JSON.parse(localStorage.getItem('statusFilters')) || { active: true, stale: true, signal_lost: true },
            showAirAircraft: JSON.parse(localStorage.getItem('showAirAircraft')) ?? true,
            showGroundAircraft: JSON.parse(localStorage.getItem('showGroundAircraft')) ?? true,
            showLocalDates: JSON.parse(localStorage.getItem('showLocalDates')) ?? false, // Default to UTC (false)
            phaseFilters: JSON.parse(localStorage.getItem('phaseFilters')) || { CRZ: true, DEP: true, APP: true, ARR: true, TAX: true, 'T/O': true, 'T/D': true, NEW: true },
            excludeOtherAirportsGrounded: JSON.parse(localStorage.getItem('excludeOtherAirportsGrounded')) ?? false, // Default to false (show all grounded aircraft)
            // Aircraft animation settings
            aircraftAnimation: {
                enabled: JSON.parse(localStorage.getItem('aircraftAnimationEnabled')) ?? true,
                interpolationFps: parseInt(localStorage.getItem('aircraftAnimationFps')) || 10,
                viewportCulling: JSON.parse(localStorage.getItem('aircraftAnimationViewportCulling')) ?? true,
                adaptivePerformance: JSON.parse(localStorage.getItem('aircraftAnimationAdaptivePerformance')) ?? true
            }
        },

        // Add property to track previous settings for change detection
        previousSettings: {},
        needsFullReload: false,
        
        // Simulation state
        showCreateSimulatedAircraft: false,
        simulationModal: {
            lat: 43.6777, // Default to CYYZ area
            lon: -79.6248,
            altitude: 5000,
            heading: Math.floor(Math.random() * 360), // Random heading
            speed: 250,
            verticalRate: 0,
            mapClickMode: false
        },

        // Caching properties for filteredAircraft performance optimization
        _filteredAircraftCache: null,
        _lastFilterHash: null,

        // Enhanced settings save with change detection for WebSocket
        saveSettings() {
            const previousSettings = { ...this.previousSettings };
            
            // Save to localStorage
            localStorage.setItem('showLabels', this.settings.showLabels);
            localStorage.setItem('showPaths', this.settings.showPaths);
            localStorage.setItem('showRings', this.settings.showRings);
            localStorage.setItem('minAltitude', this.settings.minAltitude);
            localStorage.setItem('maxAltitude', this.settings.maxAltitude);
            localStorage.setItem('trailLength', this.settings.trailLength);
            localStorage.setItem('tracksLimit', this.settings.tracksLimit);
            localStorage.setItem('lastSeenMinutes', this.settings.lastSeenMinutes);
            localStorage.setItem('statusFilters', JSON.stringify(this.settings.statusFilters));
            localStorage.setItem('showAirAircraft', this.settings.showAirAircraft);
            localStorage.setItem('showGroundAircraft', this.settings.showGroundAircraft);
            localStorage.setItem('showLocalDates', this.settings.showLocalDates);
            localStorage.setItem('phaseFilters', JSON.stringify(this.settings.phaseFilters));
            localStorage.setItem('excludeOtherAirportsGrounded', this.settings.excludeOtherAirportsGrounded);
            
            // Save aircraft animation settings
            localStorage.setItem('aircraftAnimationEnabled', this.settings.aircraftAnimation.enabled);
            localStorage.setItem('aircraftAnimationFps', this.settings.aircraftAnimation.interpolationFps);
            localStorage.setItem('aircraftAnimationViewportCulling', this.settings.aircraftAnimation.viewportCulling);
            localStorage.setItem('aircraftAnimationAdaptivePerformance', this.settings.aircraftAnimation.adaptivePerformance);
            
            // Update animation engine configuration if it exists
            if (this.animationEngine) {
                this.animationEngine.updateConfig(this.settings.aircraftAnimation);
            }
            
            // Detect if server-side filter change occurred
            const serverSideChanged = (
                previousSettings.minAltitude !== this.settings.minAltitude ||
                previousSettings.maxAltitude !== this.settings.maxAltitude ||
                previousSettings.lastSeenMinutes !== this.settings.lastSeenMinutes ||
                previousSettings.excludeOtherAirportsGrounded !== this.settings.excludeOtherAirportsGrounded
            );
            
            if (serverSideChanged && Object.keys(previousSettings).length > 0) {
                this.needsFullReload = true;
            }
            
            // Store current settings for next comparison
            this.previousSettings = { ...this.settings };
        },

        // Enhanced settings change handlers for WebSocket
        onFilterChange() {
            this.saveSettings();
            
            // Check if this is a server-side filter change that requires bulk reload
            if (this.needsFullReload) {
                console.log('Server-side filter change detected, requesting bulk reload via WebSocket...');
                this.requestInitialAircraftData(); // Use WebSocket bulk request
                this.needsFullReload = false;
            } else {
                // Simple filters like search, phase, air/ground can be handled client-side
                this._lastFilterHash = null;
                
                if (this.mapManager) {
                    this.mapManager.applyFiltersAndRefreshView();
                }
            }
        },

        // Helper method to check if a frequency is unmuted
        isUnmuted(frequencyId) {
            return this.unmutedFrequencies.has(frequencyId);
        },

        // MapManager instance
        mapManager: null, // Will be set in alpine:init

        // Computed
        get aircraftCount() {
            return Object.keys(this.aircraft).length;
        },

        getStatusColor(aircraft) {
            if (!aircraft || !aircraft.status) return 'bg-highlight'; // Default green
            
            // If aircraft is on ground, use a neutral gray color
            if (aircraft.on_ground) return 'bg-gray-400';
            
            switch (aircraft.status) {
                case 'active':
                    return 'bg-highlight'; // Green
                case 'stale':
                    return 'bg-warning';   // Yellow
                case 'signal_lost':
                    return 'bg-gray-500';  // Grey
                default:
                    return 'bg-highlight'; // Default green
            }
        },

        // Helper function to safely get current phase
        getCurrentPhase(aircraft) {
            return (aircraft && aircraft.phase && aircraft.phase.current && aircraft.phase.current.length > 0)
                ? aircraft.phase.current[0].phase
                : 'NEW';
        },

        get filteredAircraft() {
            // CRITICAL FIX: Maintain stable array reference to prevent full table re-renders
            if (!this._filteredAircraftCache) {
                this._filteredAircraftCache = [];
            }
            
            const now = Date.now();
            
            // Only recalculate every 500ms to prevent constant recomputation during WebSocket updates
            if (this._lastFilterTime && (now - this._lastFilterTime) < 500 && this._lastFilterHash) {
                return this._filteredAircraftCache;
            }
            
            // If no filtering has been done yet, do it immediately (synchronously for first load)
            if (!this._lastFilterHash) {
                this._performFiltering();
                return this._filteredAircraftCache;
            }
            
            // Schedule async filtering to prevent main thread blocking
            if (!this._filteringScheduled) {
                this._filteringScheduled = true;
                setTimeout(() => {
                    this._performFiltering();
                    this._filteringScheduled = false;
                }, 0);
            }
            
            // Return stable array reference
            return this._filteredAircraftCache;
        },
        
        // Perform heavy filtering computation asynchronously
        _performFiltering() {
            // Create lightweight hash of current filter state (avoid expensive JSON.stringify)
            const filterHash = `${this.searchTerm}|${this.settings.showGroundAircraft}|${this.settings.showAirAircraft}|${this.settings.minAltitude}|${this.settings.maxAltitude}|${Object.keys(this.aircraft).length}|${this.selectedAircraft?.hex}|${this.sortColumn}|${this.sortDirection}`;
            
            // Return cached result if nothing changed (but always filter if no cache exists)
            if (this._lastFilterHash === filterHash && this._filteredAircraftCache) {
                return;
            }
            
            console.log('[PERFORMANCE] Recalculating filtered aircraft list');
            const searchLower = this.searchTerm.toLowerCase();
            const filtered = Object.values(this.aircraft).filter(aircraft => {
                // Filter by search term - now includes callsign, type, and category
                if (searchLower) {
                    const callsign = (aircraft.flight || aircraft.hex).toLowerCase();
                    const type = (aircraft.adsb?.type || '').toLowerCase();
                    const category = (aircraft.adsb?.category || '').toLowerCase();
                    
                    const matchesSearch = callsign.includes(searchLower) ||
                                        type.includes(searchLower) ||
                                        category.includes(searchLower);
                    
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
                if (!aircraft.on_ground && (!aircraft.adsb || aircraft.adsb.alt_baro < this.settings.minAltitude || aircraft.adsb.alt_baro > this.settings.maxAltitude)) {
                    return false;
                }
                
                return true;
            });

            // Sort the aircraft with status priority: active first, then signal_lost
            const sorted = filtered.sort((a, b) => {
                // Primary sort: Status priority (active first, signal_lost second)
                const aStatus = a.status === 'signal_lost' ? 1 : 0;
                const bStatus = b.status === 'signal_lost' ? 1 : 0;
                
                if (aStatus !== bStatus) {
                    return aStatus - bStatus; // Active (0) comes before signal_lost (1)
                }
                
                // Secondary sort: User-selected column
                const aValue = this.getSortValue(a, this.sortColumn);
                const bValue = this.getSortValue(b, this.sortColumn);
                
                const direction = this.sortDirection === 'asc' ? 1 : -1;
                
                if (aValue < bValue) return -1 * direction;
                if (aValue > bValue) return 1 * direction;
                return 0;
            });
            
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
            const currentMap = new Map(this._filteredAircraftCache.map(aircraft => [aircraft.hex, aircraft]));
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
                const existingIndex = this._filteredAircraftCache.findIndex(a => a.hex === newAircraft.hex);
                
                if (existingIndex >= 0) {
                    // Update existing aircraft in-place
                    Object.assign(this._filteredAircraftCache[existingIndex], newAircraft);
                    finalArray.push(this._filteredAircraftCache[existingIndex]);
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
                case 'callsign':
                    return (aircraft.flight || aircraft.hex).toLowerCase();
                case 'phase':
                    return this.getCurrentPhase(aircraft);
                case 'altitude':
                    return aircraft.adsb ? aircraft.adsb.alt_baro : 0;
                case 'heading':
                    return aircraft.adsb ? (aircraft.adsb.mag_heading || 0) : 0;
                case 'speed':
                    return aircraft.adsb ? aircraft.adsb.tas : 0;
                case 'gs':
                    return aircraft.adsb ? aircraft.adsb.gs : 0;
                case 'distance':
                    return aircraft.distance || 999999; // Sort undefined distances to the end
                default:
                    return '';
            }
        },

        // Helper function to check if we should show signal lost divider before this aircraft
        shouldShowSignalLostDivider(aircraft, index) {
            if (aircraft.status !== 'signal_lost') return false;
            
            // Show divider if this is the first signal_lost aircraft
            if (index === 0) return true;
            
            // Show divider if the previous aircraft was not signal_lost
            const previousAircraft = this.filteredAircraft[index - 1];
            return previousAircraft && previousAircraft.status !== 'signal_lost';
        },

        // RESTORING createLabelContent
        createLabelContent(aircraft, callsign, altitude, verticalTrend) {
            const altitudeColorClass = verticalTrend === 'climbing' ? 'text-highlight' : verticalTrend === 'descending' ? 'text-danger' : 'text-text';
            // Use alt_baro consistently across all components (same as details panel and flight strip)
            const altitudeDisplay = aircraft.adsb && aircraft.adsb.alt_baro !== undefined ?
                `${Math.round(aircraft.adsb.alt_baro/100)*100}` : '0';
            
            // Speed logic: TAS if available, GS if TAS is 0 or null
            const tasValue = aircraft.adsb && aircraft.adsb.tas ? Math.round(aircraft.adsb.tas) : null;
            const gsValue = aircraft.adsb && aircraft.adsb.gs ? Math.round(aircraft.adsb.gs) : null;
            const speedValue = tasValue || gsValue || 0;
            const speedLabel = tasValue ? 'TAS' : 'GS';

            const statusColorClass = this.getStatusColor(aircraft);
            let lastSeenText = '';
            if (aircraft.last_seen) {
                const secondsAgo = Math.floor((new Date() - new Date(aircraft.last_seen)) / 1000);
                lastSeenText = `${secondsAgo}s`;
            }

            const altitudeTrendIconClass = this.getAltitudeTrendIcon(aircraft);

            // Determine callsign color based on aircraft status
            let callsignColorClass = 'text-highlight'; // Default green for active aircraft
            if (aircraft.status === 'signal_lost') {
                callsignColorClass = 'text-red-400'; // Red for signal lost (matching table)
            } else if (aircraft.on_ground) {
                callsignColorClass = 'text-white'; // White for grounded aircraft
            }

            // Create phase badge (identical to table formatting)
            let phaseBadge = '';
            const currentPhase = this.getCurrentPhase(aircraft);
            if (currentPhase) {
                const phaseClasses = {
                    'CRZ': 'bg-blue-500/20 text-blue-400 border border-blue-500/30',
                    'DEP': 'bg-green-500/20 text-green-400 border border-green-500/30',
                    'APP': 'bg-yellow-500/20 text-yellow-400 border border-yellow-500/30',
                    'ARR': 'bg-red-500/20 text-red-400 border border-red-500/30',
                    'TAX': 'bg-purple-500/20 text-purple-400 border border-purple-500/30',
                    'T/O': 'bg-orange-500/20 text-orange-400 border border-orange-500/30',
                    'T/D': 'bg-teal-500/20 text-teal-400 border border-teal-500/30',
                    'NEW': 'bg-gray-500/20 text-gray-400 border border-gray-500/30'
                };
                const phaseClass = phaseClasses[currentPhase] || phaseClasses['NEW'];
                phaseBadge = `<span class="text-[8px] font-bold uppercase px-1.5 py-0.5 rounded ${phaseClass}">${currentPhase}</span>`;
            }

            // Create airline and type display on same line
            const aircraftType = aircraft.adsb?.t || 'N/A';
            const airlineTypeDisplay = aircraft.airline ? `${aircraft.airline} (${aircraftType})` : aircraftType;

            if (aircraft.on_ground) {
                return `
                    <div class="bg-black/80 backdrop-blur-sm border border-white/10 p-1.5 rounded text-[11px] whitespace-nowrap min-w-[140px] flex flex-col gap-1 transition-all duration-200 group cursor-pointer
                                hover:bg-black/90 hover:border-highlight/50 hover:shadow-[0_0_10px_rgba(76,175,80,0.1)]">
                        <div class="flex justify-between items-center">
                            <div class="flex items-center">
                                ${phaseBadge}
                                <span class="font-bold ${callsignColorClass} text-xs group-hover:text-white/90 ${phaseBadge ? 'ml-1.5' : ''}">${callsign}</span>
                            </div>
                            <span class="text-text/70 text-[10px]">${lastSeenText}</span>
                        </div>
                        <div class="text-[10px] text-text/80 -mt-0.5">${airlineTypeDisplay}</div>
                        <div class="text-[10px]">${speedLabel} ${speedValue}</div>
                    </div>
                `;
            }

            return `
                <div class="bg-black/80 backdrop-blur-sm border border-white/10 p-1.5 rounded text-[11px] whitespace-nowrap min-w-[140px] flex flex-col gap-1 transition-all duration-200 group cursor-pointer
                            hover:bg-black/90 hover:border-highlight/50 hover:shadow-[0_0_10px_rgba(76,175,80,0.1)]">
                    <div class="flex justify-between items-center">
                        <div class="flex items-center">
                            ${phaseBadge}
                            <span class="font-bold ${callsignColorClass} text-xs ${phaseBadge ? 'ml-1.5' : ''}">${callsign}</span>
                        </div>
                        <span class="text-text/70 text-[10px]">${lastSeenText}</span>
                    </div>
                    <div class="text-[10px] text-text/80 -mt-0.5">${airlineTypeDisplay}</div>
                    <div class="grid grid-cols-2 gap-1 text-[10px]">
                        <div class="${altitudeColorClass}">ALT ${altitudeDisplay} <span class="${altitudeTrendIconClass}"></span></div>
                        <div>${speedLabel} ${speedValue}</div>
                    </div>
                </div>
            `;
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

        // RESTORING formatAircraftDetails
        formatAircraftDetails() {
            if (!this.selectedAircraft) return '';

            const aircraft = this.selectedAircraft;
            const adsbData = aircraft.adsb || {};
            
            // Calculate seconds since last seen
            let lastSeenText = 'N/A';
            let lastSeenSeconds = '';
            if (aircraft.last_seen) {
                const secondsAgo = Math.floor((new Date() - new Date(aircraft.last_seen)) / 1000);
                lastSeenText = this.formatDate(aircraft.last_seen, true);
                lastSeenSeconds = `${secondsAgo}s`;
            }

            // Calculate first seen text with seconds ago
            let firstSeenText = 'N/A';
            let firstSeenSeconds = '';
            if (aircraft.created_at) {
                const secondsAgo = Math.floor((new Date() - new Date(aircraft.created_at)) / 1000);
                firstSeenText = this.formatDate(aircraft.created_at, true);
                firstSeenSeconds = `${secondsAgo}s`;
            }

            // Calculate takeoff time text with seconds ago
            let takeoffTimeText = 'N/A';
            let takeoffSeconds = '';
            if (aircraft.DateTookoff || aircraft.date_tookoff) {
                const takeoffTime = aircraft.DateTookoff || aircraft.date_tookoff;
                const secondsAgo = Math.floor((new Date() - new Date(takeoffTime)) / 1000);
                takeoffTimeText = this.formatDate(takeoffTime, true);
                takeoffSeconds = `${secondsAgo}s`;
            }

            // Calculate landing time text with seconds ago
            let landingTimeText = 'N/A';
            let landingSeconds = '';
            if (aircraft.DateLanded || aircraft.date_landed) {
                const landingTime = aircraft.DateLanded || aircraft.date_landed;
                const secondsAgo = Math.floor((new Date() - new Date(landingTime)) / 1000);
                landingTimeText = this.formatDate(landingTime, true);
                landingSeconds = `${secondsAgo}s`;
            }

            const fields = [
                ['Basic Info', [
                    ['Callsign', aircraft.flight?.trim() || 'N/A'],
                    ['Airline', aircraft.airline || 'N/A'],
                    ['Hex', aircraft.hex],
                    ['Type', adsbData.t || 'N/A'],
                    ['Registration', adsbData.r || 'N/A'],
                    ['Category', adsbData.category || 'N/A'],
                    ['Squawk', adsbData.squawk || 'N/A'],
                    ['First Seen', firstSeenText, firstSeenSeconds],
                    ['Last Seen', lastSeenText, lastSeenSeconds]
                ]],
                ['Status', [
                    ['On Ground', aircraft.on_ground ? 'Yes' : 'No'],
                    ['Phase', this.getCurrentPhase(aircraft)],
                    ['Takeoff Time', takeoffTimeText, takeoffSeconds],
                    ['Landing Time', landingTimeText, landingSeconds]
                ]],
                ['Position', [
                    ['Latitude', adsbData.lat?.toFixed(6) || 'N/A'],
                    ['Longitude', adsbData.lon?.toFixed(6) || 'N/A'],
                    ['Distance (NM)', aircraft.distance || 'N/A'],
                    ['Altitude (Baro)', `${adsbData.alt_baro ?? 'N/A'} ft`],
                    ['Altitude (Geom)', `${adsbData.alt_geom ?? 'N/A'} ft`],
                    ['Vertical Rate', `${adsbData.baro_rate ?? 0} ft/min`]
                ]],
                ['Speed & Direction', [
                    ['Ground Speed', `${adsbData.gs ?? 'N/A'} kts`],
                    ['True Airspeed', `${adsbData.tas ?? 'N/A'} kts`],
                    ['IAS', `${adsbData.ias ?? 'N/A'} kts`],
                    ['TAS', `${adsbData.tas ?? 'N/A'} kts`],
                    ['Mach', adsbData.mach ?? 'N/A'],
                    ['Track', `${adsbData.track ?? 'N/A'}°`],
                    ['Mag Heading', `${adsbData.mag_heading ?? 'N/A'}°`],
                    ['True Heading', `${adsbData.true_heading ?? 'N/A'}°`]
                ]],
                ['Navigation', [
                    ['Nav QNH', `${adsbData.nav_qnh ?? 'N/A'} hPa`],
                    ['Nav Altitude MCP', `${adsbData.nav_altitude_mcp ?? 'N/A'} ft`],
                    ['Nav Altitude FMS', `${adsbData.nav_altitude_fms ?? 'N/A'} ft`],
                    ['Nav Heading', `${adsbData.nav_heading ?? 'N/A'}°`]
                ]],
                ['Weather', [
                    ['Wind Direction', `${adsbData.wd ?? 'N/A'}°`],
                    ['Wind Speed', `${adsbData.ws ?? 'N/A'} kts`],
                    ['OAT', `${adsbData.oat ?? 'N/A'}°C`],
                    ['TAT', `${adsbData.tat ?? 'N/A'}°C`]
                ]],
                ['ADSB Info', [
                    ['Version', adsbData.version ?? 'N/A'],
                    ['NIC', adsbData.nic ?? 'N/A'],
                    ['NACp', adsbData.nac_p ?? 'N/A'],
                    ['NACv', adsbData.nac_v ?? 'N/A'],
                    ['SIL', adsbData.sil ?? 'N/A'],
                    ['SIL Type', adsbData.sil_type || 'N/A'],
                    ['GVA', adsbData.gva ?? 'N/A'],
                    ['SDA', adsbData.sda ?? 'N/A']
                ]],
                ['Signal', [
                    ['Messages', adsbData.messages ?? 'N/A'],
                    ['Seen', `${adsbData.seen ?? 'N/A'}s`], 
                    ['RSSI', `${adsbData.rssi ?? 'N/A'} dBm`]
                ]]
            ];

            // Initialize collapsible sections state if not already done
            if (!this.collapsibleSections) {
                this.collapsibleSections = {};
                fields.forEach(([category]) => {
                    // Default to expanded, except for ADSB Info and Signal which are collapsed by default
                    this.collapsibleSections[category] = !['ADSB Info', 'Signal'].includes(category);
                });
            }
            
            return `
                <div class="grid grid-cols-2 gap-x-4 gap-y-2 text-xs">
                    ${fields.map(([category, items]) => `
                        <div class="col-span-2 mt-2 first:mt-0">
                            <div class="flex justify-between items-center mb-1 cursor-pointer relative pb-1"
                                 onclick="Alpine.store('atc').toggleSection('${category}')">
                                <h4 class="text-highlight font-bold text-[11px] uppercase tracking-wider">${category}</h4>
                                <i class="fas fa-chevron-${this.collapsibleSections[category] ? 'down' : 'right'} text-highlight/70 text-xs"></i>
                                <!-- Subtle green underline -->
                                <div class="absolute bottom-0 left-0 right-0 h-px bg-highlight/30"></div>
                            </div>
                            <div class="grid grid-cols-2 gap-x-4 gap-y-0.5 transition-all duration-300 overflow-hidden"
                                 style="${this.collapsibleSections[category] ? '' : 'max-height: 0; opacity: 0; margin: 0; padding: 0;'}">
                                ${items.map(([label, value, seconds]) => `
                                    <div class="contents">
                                        <div class="text-text/70">${label}:</div>
                                        <div class="font-mono">
                                            ${seconds ? `
                                                <span>${value}</span>
                                                <span class="text-text/70 text-[10px] ml-1">${seconds}</span>
                                            ` : value}
                                        </div>
                                    </div>
                                `).join('')}
                            </div>
                        </div>
                    `).join('')}
                </div>
            `;
        },

        // RESTORING toggleSort
        toggleSort(column) {
            if (this.sortColumn === column) {
                this.sortDirection = this.sortDirection === 'asc' ? 'desc' : 'asc';
            } else {
                this.sortColumn = column;
                this.sortDirection = 'asc';
            }
            // No need to explicitly call applyFilters here, as the sorted list is a computed property
            // that will react to changes in sortColumn or sortDirection.
            // However, if applyFilters also handles map updates, it might be needed if sorting should affect map directly.
            // For now, assuming filteredAircraft computed property handles table refresh.
        },

        // RESTORING toggleStatusFilter and toggleGroundedAircraft
        toggleStatusFilter(statusKey) {
            if (this.settings.statusFilters.hasOwnProperty(statusKey)) {
                this.settings.statusFilters[statusKey] = !this.settings.statusFilters[statusKey];
                this.saveSettings();
                // this.applyFilters(); // applyFilters calls mapManager.applyFiltersAndRefreshView()
                if (this.mapManager) this.mapManager.applyFiltersAndRefreshView();
            }
        },
        
        // Toggle collapsible sections in aircraft details
        toggleSection(category) {
            if (!this.collapsibleSections) {
                this.collapsibleSections = {};
            }
            this.collapsibleSections[category] = !this.collapsibleSections[category];
        },

        // Toggle Air aircraft visibility
        toggleAirAircraft() {
            console.log('[Alpine Store] toggleAirAircraft called, current state:', this.settings.showAirAircraft);
            this.settings.showAirAircraft = !this.settings.showAirAircraft;

            this.saveSettings();
            this.applyFilters();
            this.refreshAlertsDisplay(); // Refresh alerts based on new filter

            // Update map visibility including tracks
            if (this.mapManager) {
                this.mapManager.applyFiltersAndRefreshView();
            }

            // Send filter update to server immediately (this will return filtered aircraft data)
            this.sendFilterUpdate();
        },

        // Toggle Ground aircraft visibility
        toggleGroundAircraft() {
            //console.log('[Alpine Store] toggleGroundAircraft called, current state:', this.settings.showGroundAircraft);
            this.settings.showGroundAircraft = !this.settings.showGroundAircraft;

            this.saveSettings();
            this.applyFilters();
            this.refreshAlertsDisplay(); // Refresh alerts based on new filter

            // Update map visibility including tracks
            if (this.mapManager) {
                this.mapManager.applyFiltersAndRefreshView();
            }

            // Send filter update to server immediately (this will return filtered aircraft data)
            this.sendFilterUpdate();
        },

        // Toggle flight phase filter
        togglePhaseFilter(phase) {
            if (!this.settings.phaseFilters) {
                this.settings.phaseFilters = { CRZ: true, DEP: true, APP: true, ARR: true, TAX: true, 'T/O': true, 'T/D': true, NEW: true };
            }
            this.settings.phaseFilters[phase] = !this.settings.phaseFilters[phase];
            this.saveSettings();
            this.applyFilters();
            this.refreshAlertsDisplay(); // Refresh alerts based on new filter
            
            // Update map visibility including tracks
            if (this.mapManager) {
                this.mapManager.applyFiltersAndRefreshView();
            }
            
            // Send filter update to server immediately
            this.sendFilterUpdate();
        },

        // Simulation methods
        async createSimulatedAircraft(lat, lon, altitude, heading, speed, verticalRate) {
            try {
                const response = await this.fetchWithTimeout(`${API_BASE_URL}/simulation/aircraft`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        lat: lat,
                        lon: lon,
                        altitude: altitude,
                        heading: heading,
                        speed: speed,
                        vertical_rate: verticalRate
                    })
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    throw new Error(`Failed to create simulated aircraft: ${errorText}`);
                }

                const result = await response.json();
                console.log('Created simulated aircraft:', result.aircraft);
                
                // Close the modal
                this.showCreateSimulatedAircraft = false;
                this.simulationModal.mapClickMode = false;
                
                return result.aircraft;
            } catch (error) {
                console.error('Error creating simulated aircraft:', error);
                throw error;
            }
        },

        async updateSimulationControls(hex, controlType, value) {
            try {
                // Find the aircraft to get current controls
                const aircraft = this.aircraft.find(a => a.hex === hex);
                if (!aircraft || !aircraft.simulation_controls) {
                    console.error('Aircraft not found or not simulated:', hex);
                    return;
                }

                // Update the local value immediately for responsive UI
                switch (controlType) {
                    case 'heading':
                        aircraft.simulation_controls.target_heading = parseFloat(value);
                        break;
                    case 'speed':
                        aircraft.simulation_controls.target_speed = parseFloat(value);
                        break;
                    case 'vertical_rate':
                        aircraft.simulation_controls.target_vertical_rate = parseFloat(value);
                        break;
                }

                // Send update to server
                const response = await this.fetchWithTimeout(`${API_BASE_URL}/simulation/aircraft/${hex}/controls`, {
                    method: 'PUT',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        heading: aircraft.simulation_controls.target_heading,
                        speed: aircraft.simulation_controls.target_speed,
                        vertical_rate: aircraft.simulation_controls.target_vertical_rate
                    })
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Failed to update simulation controls: ${errorText}`);
                    throw new Error(`Failed to update simulation controls: ${errorText}`);
                }
                
                console.log(`Updated simulation controls: ${controlType}=${value} for aircraft ${hex}`);
            } catch (error) {
                console.error('Error updating simulation controls:', error);
                throw error;
            }
        },

        async removeSimulatedAircraft(hex) {
            try {
                const response = await this.fetchWithTimeout(`${API_BASE_URL}/simulation/aircraft/${hex}`, {
                    method: 'DELETE'
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    throw new Error(`Failed to remove simulated aircraft: ${errorText}`);
                }

                console.log('Removed simulated aircraft:', hex);
            } catch (error) {
                console.error('Error removing simulated aircraft:', error);
                throw error;
            }
        },

        setSimulationPositionFromMap() {
            this.simulationModal.mapClickMode = !this.simulationModal.mapClickMode;
            if (this.simulationModal.mapClickMode) {
                console.log('Click on map to set simulated aircraft position');
                // Add map click handler
                if (this.mapManager && this.mapManager.map) {
                    this.mapManager.enableSimulationPositionMode();
                }
            } else {
                // Disable map click handler
                if (this.mapManager && this.mapManager.map) {
                    this.mapManager.disableSimulationPositionMode();
                }
            }
        },

        generateRandomPosition() {
            // Generate random position within 50 nautical miles of airport
            const centerLat = this.stationLatitude || 43.6777; // CYYZ default
            const centerLon = this.stationLongitude || -79.6248;
            
            // 50 nautical miles = ~0.833 degrees latitude
            const maxDistanceDeg = 0.833;
            
            // Random angle and distance
            const angle = Math.random() * 2 * Math.PI;
            const distance = Math.random() * maxDistanceDeg;
            
            // Calculate new position
            const latOffset = distance * Math.cos(angle);
            const lonOffset = distance * Math.sin(angle) / Math.cos(centerLat * Math.PI / 180);
            
            this.simulationModal.lat = centerLat + latOffset;
            this.simulationModal.lon = centerLon + lonOffset;
            
            console.log(`Generated random position: ${this.simulationModal.lat.toFixed(6)}, ${this.simulationModal.lon.toFixed(6)}`);
        },

        onMapClickForSimulation(lat, lon) {
            if (this.simulationModal.mapClickMode) {
                this.simulationModal.lat = lat;
                this.simulationModal.lon = lon;
                this.simulationModal.mapClickMode = false;
                
                // Disable map click mode
                if (this.mapManager && this.mapManager.map) {
                    this.mapManager.disableSimulationPositionMode();
                }
                
                console.log(`Set simulation position from map: ${lat.toFixed(6)}, ${lon.toFixed(6)}`);
            }
        },

        // Get phase color class (matches navigation bar colors)
        getPhaseColorClass(phase) {
            const phaseColorMap = {
                'NEW': 'text-gray-400',
                'TAX': 'text-purple-400',
                'T/O': 'text-orange-400',
                'DEP': 'text-green-400',
                'CRZ': 'text-blue-400',
                'ARR': 'text-red-400',
                'APP': 'text-yellow-400',
                'T/D': 'text-teal-400'
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

        // Check if phase alert should be shown based on phase filter settings
        shouldShowPhaseAlert(phase) {
            // Only show alerts for phases that are currently enabled in filters
            // phaseFilters[phase] === false means phase is filtered OUT
            // phaseFilters[phase] !== false means phase is enabled (true or undefined)
            return this.settings.phaseFilters && this.settings.phaseFilters[phase] !== false;
        },
        
        // Get count of grounded aircraft
        getGroundedAircraftCount() {
            return Object.values(this.aircraft).filter(aircraft => aircraft.on_ground).length;
        },
        
        // Get seconds since last seen for an aircraft
        getSecondsSinceLastSeen(aircraft) {
            if (!aircraft.last_seen) return 'Unknown';
            const lastSeen = new Date(aircraft.last_seen);
            const now = new Date();
            return Math.floor((now - lastSeen) / 1000);
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

        // Cycle to next aircraft in the filtered list
        cycleToNextAircraft() {
            const filtered = this.filteredAircraft;
            if (filtered.length === 0) return;
            
            if (!this.selectedAircraft) {
                // Select first aircraft
                this.selectedAircraft = filtered[0];
                if (this.mapManager) {
                    this.mapManager.updateVisualState(filtered[0].hex, true);
                    this.mapManager.centerOnAircraft(filtered[0]);
                }
                return;
            }
            
            // Find current aircraft index
            const currentIndex = filtered.findIndex(aircraft => aircraft.hex === this.selectedAircraft.hex);
            if (currentIndex === -1) {
                // Current aircraft not in filtered list, select first
                this.selectedAircraft = filtered[0];
            } else {
                // Select next aircraft (wrap around to beginning)
                const nextIndex = (currentIndex + 1) % filtered.length;
                this.selectedAircraft = filtered[nextIndex];
            }
            
            if (this.mapManager) {
                this.mapManager.updateVisualState(this.selectedAircraft.hex, true);
                this.mapManager.centerOnAircraft(this.selectedAircraft);
            }
        },

        // Cycle to previous aircraft in the filtered list
        cycleToPreviousAircraft() {
            const filtered = this.filteredAircraft;
            if (filtered.length === 0) return;
            
            if (!this.selectedAircraft) {
                // Select last aircraft
                this.selectedAircraft = filtered[filtered.length - 1];
                if (this.mapManager) {
                    this.mapManager.updateVisualState(filtered[filtered.length - 1].hex, true);
                    this.mapManager.centerOnAircraft(filtered[filtered.length - 1]);
                }
                return;
            }
            
            // Find current aircraft index
            const currentIndex = filtered.findIndex(aircraft => aircraft.hex === this.selectedAircraft.hex);
            if (currentIndex === -1) {
                // Current aircraft not in filtered list, select last
                this.selectedAircraft = filtered[filtered.length - 1];
            } else {
                // Select previous aircraft (wrap around to end)
                const prevIndex = currentIndex === 0 ? filtered.length - 1 : currentIndex - 1;
                this.selectedAircraft = filtered[prevIndex];
            }
            
            if (this.mapManager) {
                this.mapManager.updateVisualState(this.selectedAircraft.hex, true);
                this.mapManager.centerOnAircraft(this.selectedAircraft);
            }
        },

        // Properties for Aircraft Details Panel (moved from x-data in HTML)
        aircraftDetailsShowHistoryView: false,
        aircraftDetailsHistoryData: [],
        aircraftDetailsFutureData: [],
        aircraftDetailsHistoryCount: 0,
        aircraftDetailsHistoryLoading: false,
        aircraftDetailsHistoryRefreshInterval: null,
        aircraftDetailsCurrentAircraftHexForPanel: null, // New property
        
        // Properties for Proximity View
        showProximityView: false,
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
        setupAircraftDetailsPanel() {
            if (!this.selectedAircraft) { // No aircraft selected, fully close and reset
                this.aircraftDetailsShowHistoryView = false;
                this.aircraftDetailsHistoryData = [];
                this.aircraftDetailsHistoryCount = 0;
                this.aircraftDetailsStopHistoryRefresh();
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
                
                // CRITICAL FIX: Clear map trails for the previous aircraft immediately to prevent visual glitch
                if (this.mapManager && this.aircraftDetailsCurrentAircraftHexForPanel) {
                    // Remove trails for the previously selected aircraft
                    this.mapManager.layers.trails.eachLayer(layer => {
                        if (layer.options.aircraftHex === this.aircraftDetailsCurrentAircraftHexForPanel) {
                            this.mapManager.layers.trails.removeLayer(layer);
                        }
                    });
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
                
                // Always load phase history data for Details tab
                this.phaseHistoryAircraftHex = this.selectedAircraft.hex;
                this.loadPhaseHistoryData();
                this.startPhaseHistoryRefresh();
                
                // Always load tracks data for map trails when aircraft is selected
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
            const requestId = `${hex}-${Date.now()}`; // Unique request ID for debugging
            
            console.log(`[TRACKS] Starting tracks request for ${hex} (${requestId}), isRefresh: ${isRefresh}`);
            
            // Initialize tracks pending requests if needed
            if (!this.pendingRequests.tracks) {
                this.pendingRequests.tracks = new Map();
            }
            
            // Check if there's already a pending tracks request for this aircraft
            if (this.pendingRequests.tracks.get(hex)) {
                console.log(`[TRACKS] Request already in progress for ${hex}, skipping ${requestId}`);
                return;
            }

            // Set pending flag with timestamp for debugging
            this.pendingRequests.tracks.set(hex, { requestId, startTime: Date.now() });
            
            if (!isRefresh) {
                this.aircraftDetailsHistoryLoading = true;
            }
            
            try {
                // Add limit parameter to control track length
                const limit = this.settings.tracksLimit || 1000;
                const url = `${API_BASE_URL}/aircraft/${hex}/tracks?limit=${limit}`;
                
                console.log(`[TRACKS] Fetching: ${url} (${requestId})`);
                const startTime = Date.now();
                
                const response = await this.fetchWithTimeout(url);
                const fetchTime = Date.now() - startTime;
                
                console.log(`[TRACKS] Fetch completed in ${fetchTime}ms for ${hex} (${requestId})`);
                
                if (!response.ok) {
                    console.error(`[TRACKS] Error fetching tracks: ${response.status} for ${hex} (${requestId})`);
                    this.aircraftDetailsHistoryData = []; // Clear data on error
                    this.aircraftDetailsFutureData = []; // Clear data on error
                    this.aircraftDetailsHistoryCount = 0;
                    this.aircraftDetailsHistoryLoading = false;
                    return;
                }
                
                const data = await response.json();
                const totalTime = Date.now() - startTime;
                
                console.log(`[TRACKS] Data parsed in ${totalTime}ms, history: ${data.history?.length || 0}, future: ${data.future?.length || 0} (${requestId})`);
                
                // Split combined tracks response into history and future
                this.aircraftDetailsHistoryData = data.history || [];
                this.aircraftDetailsFutureData = data.future || [];
                this.aircraftDetailsHistoryCount = this.aircraftDetailsHistoryData.length;
                
                // Update tracks mini-map and main map trails
                if (this.mapManager && this.mapManager.updateTracksMiniMap) {
                    this.mapManager.updateTracksMiniMap();
                }
                if (this.mapManager && this.mapManager.updateFlightPaths) {
                    this.mapManager.updateFlightPaths();
                }
                
                console.log(`[TRACKS] Successfully completed tracks request for ${hex} (${requestId})`);
            } catch (error) {
                const elapsedTime = Date.now() - this.pendingRequests.tracks.get(hex)?.startTime;
                if (error.message.includes('timeout')) {
                    console.warn(`[TRACKS] Request timed out after ${elapsedTime}ms for ${hex} (${requestId})`);
                } else {
                    console.error(`[TRACKS] Error after ${elapsedTime}ms for ${hex} (${requestId}):`, error);
                }
                this.aircraftDetailsHistoryData = []; // Clear data on error
                this.aircraftDetailsFutureData = []; // Clear data on error
                this.aircraftDetailsHistoryCount = 0;
            } finally {
                this.aircraftDetailsHistoryLoading = false;
                // Always clear the pending flag
                if (this.pendingRequests.tracks.delete(hex)) {
                    //console.log(`[TRACKS] Cleared pending flag for ${hex} (${requestId})`);
                } else {
                    console.warn(`[TRACKS] Failed to clear pending flag for ${hex} (${requestId})`);
                }
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
        
        // Methods for handling hover effects on proximity aircraft
        highlightProximityAircraftOnHover(hex) {
            if (!this.mapManager) return;
            
            // Find the aircraft label element
            const markers = this.mapManager.markers[hex];
            if (!markers || !markers.label) return;
            
            const labelElement = markers.label.getElement();
            if (!labelElement) return;
            
            const labelDiv = labelElement.querySelector('div');
            if (!labelDiv) return;
            
            // Add the hover class
            labelDiv.classList.add('proximity-highlight-hover');
        },
        
        unhighlightProximityAircraftOnHover() {
            if (!this.mapManager) return;
            
            // Remove hover class from all aircraft labels
            Object.keys(this.mapManager.markers).forEach(hex => {
                const markers = this.mapManager.markers[hex];
                if (!markers || !markers.label) return;
                
                const labelElement = markers.label.getElement();
                if (!labelElement) return;
                
                const labelDiv = labelElement.querySelector('div');
                if (!labelDiv) return;
                
                // Remove the hover class but keep the proximity highlight class
                labelDiv.classList.remove('proximity-highlight-hover');
            });
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

        // Show phase history for an aircraft (now always shown in Tracks tab)
        showPhaseHistory(hex) {
            if (!hex) return;
            
            // Set the aircraft hex for phase history
            this.phaseHistoryAircraftHex = hex;
            
            // Switch to tracks view to show phase history
            this.aircraftDetailsShowHistoryView = true;
            this.showProximityView = false;
            
            // Load phase history data
            this.loadPhaseHistoryData();
            this.startPhaseHistoryRefresh();
        },

        // Navigate to Tracks tab and highlight a specific row by ADSB ID
        navigateToTracksWithHighlight(adsbId) {
            if (!adsbId) return;
            
            // Switch to tracks view
            this.aircraftDetailsShowHistoryView = true;
            this.showProximityView = false;
            this.stopProximityRefresh();
            this.clearProximityView();
            
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
        scrollToHighlightedRow(adsbId) {
            if (!adsbId) return;
            
            // Wait a bit more for Alpine.js to render the highlighted row
            setTimeout(() => {
                // Find the history data and locate the index of the matching ADSB ID
                const historyData = this.aircraftDetailsHistoryData;
                if (!historyData || historyData.length === 0) return;
                
                let targetIndex = -1;
                for (let i = 0; i < historyData.length; i++) {
                    if (historyData[i].id && historyData[i].id == adsbId) {
                        targetIndex = i;
                        break;
                    }
                }
                
                if (targetIndex === -1) return;
                
                // Find the tracks view container and scroll to the appropriate position
                const tracksContainer = document.querySelector('[x-show="$store.atc.aircraftDetailsShowHistoryView"] .overflow-x-auto');
                if (!tracksContainer) return;
                
                // Calculate approximate row height and scroll position
                // Account for header rows (Future Predictions header + Historical Positions header)
                const futureDataLength = this.aircraftDetailsFutureData ? this.aircraftDetailsFutureData.length : 0;
                const headerRows = (futureDataLength > 0 ? 1 : 0) + 1; // Future header (if exists) + Historical header
                const totalRowsBeforeTarget = futureDataLength + headerRows + targetIndex;
                
                // Estimate row height (approximately 32px per row)
                const estimatedRowHeight = 32;
                const scrollPosition = totalRowsBeforeTarget * estimatedRowHeight;
                
                // Scroll to the calculated position
                tracksContainer.scrollTo({
                    top: scrollPosition,
                    behavior: 'smooth'
                });
            }, 300);
        },

        // Highlight a specific position on the map when hovering over history row
        highlightPositionOnMap(position) {
            if (!position || !position.lat || !position.lon || !this.mapManager) return;
            
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

        // Load phase history data for the selected aircraft
        async loadPhaseHistoryData() {
            if (!this.phaseHistoryAircraftHex) {
                console.log('No aircraft hex for phase history');
                return;
            }
            
            const hex = this.phaseHistoryAircraftHex;
            
            // Check if there's already a pending phase history request for this aircraft
            if (this.pendingRequests.phaseHistory.get(hex)) {
                console.log(`Phase history request already in progress for ${hex}, skipping...`);
                return;
            }

            this.pendingRequests.phaseHistory.set(hex, true);
            this.phaseHistoryLoading = true;
            console.log('Loading phase history for aircraft:', hex);
            
            try {
                // Use the existing aircraft phase data if available
                const aircraft = this.aircraft[hex];
                console.log('Aircraft data:', aircraft);
                console.log('Aircraft phase data:', aircraft?.phase);
                
                if (aircraft && aircraft.phase) {
                    // Use only the history array since it already includes the current phase
                    if (aircraft.phase.history && aircraft.phase.history.length > 0) {
                        console.log('Loading history phases:', aircraft.phase.history.length);
                        
                        // Mark the first (most recent) phase as current
                        const historyWithCurrent = aircraft.phase.history.map((phase, index) => ({
                            ...phase,
                            is_current: index === 0, // First item is the current phase
                            id: phase.id !== undefined ? phase.id : `phase-${index}` // Use real ID if available, fallback only if undefined
                        }));
                        
                        console.log('Final phase history data:', historyWithCurrent);
                        this.phaseHistoryData = historyWithCurrent;
                    } else {
                        console.log('No history phases found');
                        this.phaseHistoryData = [];
                    }
                } else {
                    console.log('No aircraft or phase data found');
                    // Fallback: could fetch from API if needed
                    this.phaseHistoryData = [];
                }
            } catch (error) {
                console.error('Error loading phase history:', error);
                this.phaseHistoryData = [];
            } finally {
                this.phaseHistoryLoading = false;
                // Always clear the pending flag
                this.pendingRequests.phaseHistory.delete(hex);
            }
        },

        // Close phase history view (now just clears data since it's always in Tracks tab)
        closePhaseHistory() {
            this.phaseHistoryData = [];
            this.phaseHistoryAircraftHex = null;
            this.stopPhaseHistoryRefresh();
        },

        // Start automatic refresh for phase history
        startPhaseHistoryRefresh() {
            if (this.phaseHistoryRefreshInterval) {
                clearInterval(this.phaseHistoryRefreshInterval);
            }

            // Refresh every 5 seconds like other views
            this.phaseHistoryRefreshInterval = setInterval(() => {
                if (this.phaseHistoryAircraftHex) {
                    this.loadPhaseHistoryData();
                }
            }, 5000);
        },

        // Stop automatic refresh for phase history
        stopPhaseHistoryRefresh() {
            if (this.phaseHistoryRefreshInterval) {
                clearInterval(this.phaseHistoryRefreshInterval);
                this.phaseHistoryRefreshInterval = null;
            }
        },
        
        aircraftDetailsGetAltitudeTrend(position, index, isFuture = false) {
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
            
            const altDiff = currentAlt - nextAlt;
            if (altDiff > 100) return 'fas fa-arrow-up';
            if (altDiff < -100) return 'fas fa-arrow-down';
            return 'fas fa-arrows-alt-h';
        },

        aircraftDetailsGetAltitudeTrendClass(position, index, isFuture = false) {
            const dataArray = isFuture ? this.aircraftDetailsFutureData : this.aircraftDetailsHistoryData;
            
            if (!dataArray || index === dataArray.length - 1) return isFuture ? 'text-highlight/70' : 'text-text';
            const nextPosition = dataArray[index + 1]; // Next is actually previous in time for history, or next in time for future
            if (!nextPosition) return isFuture ? 'text-highlight/70' : 'text-text';
            
            // Check if we're using the new PositionMinimal format or the old Position format
            const currentAlt = position.alt_baro !== undefined ? position.alt_baro : position.altitude;
            const nextAlt = nextPosition.alt_baro !== undefined ? nextPosition.alt_baro : nextPosition.altitude;
            
            const altDiff = currentAlt - nextAlt;
            if (altDiff > 100) return isFuture ? 'text-green-400' : 'text-highlight';
            if (altDiff < -100) return isFuture ? 'text-red-400' : 'text-danger';
            return isFuture ? 'text-highlight/70' : 'text-text';
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

        // Methods
        async init() {
            if (isAppInitialized) {
                console.warn("Application already initialized. Skipping init() call.");
                return;
            }

            if (!document.getElementById('map')) {
                console.error("Map container #map not found in DOM. Aborting initialization.");
                return;
            }
            
            if (!audioClient || !this.mapManager) { // Ensure mapManager is also ready
                return; 
            }
            console.log("Alpine store init() invoked.");

            try {
                await this.fetchStationData();
                await this.fetchWeatherData();
                // Initialize map using MapManager
                if (this.mapManager && !this.mapManager.map) {
                    this.mapManager.initMap();
                    
                    // Draw range rings after map is initialized
                    this.updateStationRings();
                    
                    // Draw runways if data is available
                    if (this.runwayData) {
                        this.mapManager.drawRunways(this.runwayData);
                    }
                } else if (this.mapManager && this.mapManager.map) {
                     console.warn("Alpine store init: MapManager's map already initialized.");
                } else {
                    console.error("Alpine store init: mapManager not available for map initialization.");
                }

                this.fetchAudioFrequencies();
                
                // CRITICAL FIX: Check server config to determine if WebSocket streaming is enabled
                await this.initAircraftDataSource();

                // Initialize previous settings for change detection
                this.previousSettings = { ...this.settings };

                // Manage the current time and Zulu time update interval
                if (this.timeUpdateIntervalId) {
                    clearInterval(this.timeUpdateIntervalId);
                }
                this.updateCurrentTime(); // Initial call
                this.timeUpdateIntervalId = setInterval(() => {
                    this.updateCurrentTime();
                    this.currentTimeForPhases = new Date(); // Update time for phase history
                    if (this.connected && this.lastUpdate) {
                        this.lastUpdateSeconds = Math.floor((new Date() - this.lastUpdate) / 1000);
                    }
                }, 1000);

                // New interval for updating "seconds since last audio"
                if (this.lastAudioUpdateIntervalId) {
                    clearInterval(this.lastAudioUpdateIntervalId);
                }
                this.updateSecondsSinceLastAudio(); // Initial call
                this.lastAudioUpdateIntervalId = setInterval(() => {
                    this.updateSecondsSinceLastAudio();
                }, 1000);

                // Watch for hover changes and update map visual state
                let previousHoveredHex = null;
                Alpine.effect(() => {
                    const currentHoveredAircraft = Alpine.store('atc').hoveredAircraft;
                    if (previousHoveredHex && (!currentHoveredAircraft || previousHoveredHex !== currentHoveredAircraft.hex)) {
                        if (this.mapManager) this.mapManager.updateVisualState(previousHoveredHex, true);
                    }
                    if (currentHoveredAircraft) {
                        if (this.mapManager) this.mapManager.updateVisualState(currentHoveredAircraft.hex, true);
                        previousHoveredHex = currentHoveredAircraft.hex;
                    } else {
                        previousHoveredHex = null;
                    }
                });

                let previousSelectedHex = null; // Keep this to know if an aircraft was just selected from null
                Alpine.effect(() => {
                    const currentSelectedAircraft = this.selectedAircraft;
                    const currentHex = currentSelectedAircraft ? currentSelectedAircraft.hex : null;

                    this.setupAircraftDetailsPanel(); 

                    if (this.mapManager) {
                        this.mapManager.applyFiltersAndRefreshView();
                    }
                    
                    previousSelectedHex = currentHex;
                });
                
                // Watch for changes to showLocalDates
                Alpine.effect(() => {
                    const showLocalDates = this.showLocalDates;
                    this.settings.showLocalDates = showLocalDates;
                    this.saveSettings();
                });

                // Watch for changes to settings
                Alpine.effect(() => {
                    // Create a copy of the settings to trigger the effect when any setting changes
                    const settingsCopy = JSON.parse(JSON.stringify(this.settings));
                    this.saveSettings();
                });
                
                isAppInitialized = true;
                console.log("Application initialization successful.");

                // Setup keyboard event listeners
                this.setupKeyboardEvents();
                
                audioClient.initAudioContext();

            } catch (error) {
                console.error("Error during application initialization:", error);
            }
        },


        processAircraftData(data) {
            // Store the current proximity highlighted aircraft before processing new data
            const proximityHexSet = this.mapManager ? this.mapManager.proximityHexSet : null;
            
            const now = new Date();
            const currentAircraftHexes = new Set();
            const newAircraftData = {};

            data.aircraft.forEach(aircraft => {
                if (!aircraft.adsb || !aircraft.adsb.lat || !aircraft.adsb.lon) return;
                currentAircraftHexes.add(aircraft.hex);
                newAircraftData[aircraft.hex] = aircraft;

                if (this.mapManager) {
                    this.mapManager._ensureLeafletObjects(aircraft);
                }
            });
            
            this.aircraft = newAircraftData;

            // Invalidate filter cache when aircraft data changes
            this._lastFilterHash = null;

            if (this.mapManager) {
                this.mapManager.removeStaleMarkers(currentAircraftHexes);
            }
            
            if (!this.initialDataLoaded) {
                this.initialDataLoaded = true;
            }

            if (this.selectedAircraft && (!this.aircraft[this.selectedAircraft.hex])) {
                this.selectedAircraft = null;
            } else if (this.selectedAircraft && this.aircraft[this.selectedAircraft.hex]) {
                this.selectedAircraft = this.aircraft[this.selectedAircraft.hex];
            }

            // Apply filters and refresh the view
            if (this.mapManager) {
                this.mapManager.applyFiltersAndRefreshView();
                
                // Re-apply proximity highlighting if it was active
                if (proximityHexSet && proximityHexSet.size > 0) {
                    // Wait a tiny bit for the DOM to update
                    setTimeout(() => {
                        this.mapManager.highlightProximityAircraft(proximityHexSet);
                    }, 50);
                }
            }
            
            // Refresh alerts display to ensure filtering is applied continuously
            this.refreshAlertsDisplay();
        },

        updateCurrentTime() {
            const now = new Date();
            this.currentTime = now.toLocaleTimeString();
            this.zuluTime = now.toUTCString().match(/(\d{2}:\d{2}:\d{2})/)[0] + 'Z';
        },
        
        // Toggle between local and UTC date display
        toggleDateFormat() {
            this.showLocalDates = !this.showLocalDates;
            this.settings.showLocalDates = this.showLocalDates;
            this.saveSettings();
        },
        
        // Format a date based on user preference (local or UTC)
        formatDate(dateString, timeOnly = false) {
            if (!dateString) return 'N/A';
            
            const date = new Date(dateString);
            if (isNaN(date.getTime())) return 'Invalid Date';
            
            if (timeOnly) {
                // Only show hours, minutes, and seconds
                if (this.showLocalDates) {
                    return date.toLocaleTimeString();
                } else {
                    return date.toISOString().substring(11, 19) + 'Z';
                }
            } else {
                // Show full date and time
                if (this.showLocalDates) {
                    return date.toLocaleString();
                } else {
                    return date.toISOString().replace('T', ' ').substring(0, 19) + 'Z';
                }
            }
        },

        processSampleData() {
            this.processAircraftData({ 
                aircraft: [], 
                counts: {
                    ground_active: 0,
                    ground_total: 0,
                    air_active: 0,
                    air_total: 0
                },
                count_active: 0, 
                count_stale: 0, 
                count_signal_lost: 0 
            }); // Ensure counts are reset for sample data
        },

        // Settings methods
        toggleLabels() {
            this.saveSettings();
            if (this.mapManager) this.mapManager.applyFiltersAndRefreshView();
        },

        togglePaths() {
            this.saveSettings();
            if (this.mapManager) this.mapManager.applyFiltersAndRefreshView();
        },

        toggleRings() {
            this.saveSettings();
            if (this.mapManager) {
                this.mapManager.toggleRings();
            }
        },

        toggleAnimation() {
            this.saveSettings();
            if (this.animationEngine) {
                if (this.settings.aircraftAnimation.enabled) {
                    this.animationEngine.start();
                    console.log('Aircraft animation enabled');
                } else {
                    this.animationEngine.stop();
                    console.log('Aircraft animation disabled');
                }
            }
        },

        // Station override methods
        setStationFromMap() {
            this.stationOverride.mapClickMode = true;
            this.showMapClickIndicator();
        },

        async applyStationOverride() {
            if (!this.stationOverride.latitude || !this.stationOverride.longitude) {
                alert('Please enter valid coordinates');
                return;
            }

            try {
                const response = await fetch('/api/v1/station', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        latitude: this.stationOverride.latitude,
                        longitude: this.stationOverride.longitude
                    })
                });

                if (response.ok) {
                    this.stationOverride.active = true;
                    this.updateStationRings();
                    if (this.mapManager) {
                        this.mapManager.centerOnStation();
                    }
                    console.log('Station override applied');
                } else {
                    const error = await response.text();
                    alert('Failed to apply station override: ' + error);
                }
            } catch (error) {
                console.error('Failed to apply station override:', error);
                alert('Failed to apply station override: ' + error.message);
            }
        },

        async clearStationOverride() {
            try {
                const response = await fetch('/api/v1/station', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        latitude: null,
                        longitude: null
                    })
                });

                if (response.ok) {
                    this.stationOverride.latitude = null;
                    this.stationOverride.longitude = null;
                    this.stationOverride.active = false;
                    this.stationOverride.autoUpdate = false;
                    this.stopGeolocationWatch();
                    this.updateStationRings();
                    if (this.mapManager) {
                        this.mapManager.centerOnStation();
                    }
                    console.log('Station override cleared');
                } else {
                    const error = await response.text();
                    alert('Failed to clear station override: ' + error);
                }
            } catch (error) {
                console.error('Failed to clear station override:', error);
                alert('Failed to clear station override: ' + error.message);
            }
        },

        updateStationRings() {
            if (this.mapManager) {
                // Update store coordinates for range rings
                if (this.stationOverride.active) {
                    // Use override coordinates temporarily for range rings display
                    // (but don't overwrite the original station coordinates)
                } else {
                    // Use the original station coordinates (already fetched from API)
                    // No need to refetch - just use current stationLatitude/stationLongitude
                }

                // Trigger range rings update - addRangeRings() will use the correct coordinates
                this.mapManager.addRangeRings();
            }
        },

        showMapClickIndicator() {
            // Change cursor, show instruction overlay
            const mapElement = document.getElementById('map');
            if (mapElement) {
                mapElement.style.cursor = 'crosshair';
            }
        },

        hideMapClickIndicator() {
            const mapElement = document.getElementById('map');
            if (mapElement) {
                mapElement.style.cursor = '';
            }
        },

        // Geolocation methods
        useGeolocation() {
            if (!navigator.geolocation) {
                alert('Geolocation is not supported by this browser');
                return;
            }

            this.stationOverride.geolocationStatus = 'Getting location...';
            
            navigator.geolocation.getCurrentPosition(
                (position) => {
                    this.stationOverride.latitude = position.coords.latitude;
                    this.stationOverride.longitude = position.coords.longitude;
                    this.stationOverride.geolocationStatus = `Location acquired (±${Math.round(position.coords.accuracy)}m)`;
                    console.log('Geolocation acquired:', position.coords);
                },
                (error) => {
                    let errorMessage = 'Location access denied';
                    switch(error.code) {
                        case error.PERMISSION_DENIED:
                            errorMessage = 'Location access denied';
                            break;
                        case error.POSITION_UNAVAILABLE:
                            errorMessage = 'Location unavailable';
                            break;
                        case error.TIMEOUT:
                            errorMessage = 'Location request timeout';
                            break;
                    }
                    this.stationOverride.geolocationStatus = errorMessage;
                    console.error('Geolocation error:', error);
                },
                {
                    enableHighAccuracy: true,
                    timeout: 10000,
                    maximumAge: 60000
                }
            );
        },

        toggleGeolocationAutoUpdate() {
            if (this.stationOverride.autoUpdate) {
                this.startGeolocationWatch();
            } else {
                this.stopGeolocationWatch();
            }
        },

        startGeolocationWatch() {
            if (!navigator.geolocation) {
                this.stationOverride.autoUpdate = false;
                alert('Geolocation is not supported by this browser');
                return;
            }

            this.stopGeolocationWatch(); // Clear any existing watch

            this.stationOverride.geolocationStatus = 'Starting location tracking...';
            
            this.stationOverride.geolocationWatchId = navigator.geolocation.watchPosition(
                (position) => {
                    const newLat = position.coords.latitude;
                    const newLon = position.coords.longitude;
                    
                    // Only update if coordinates have changed significantly (>10m)
                    if (!this.stationOverride.latitude || !this.stationOverride.longitude ||
                        Math.abs(newLat - this.stationOverride.latitude) > 0.0001 ||
                        Math.abs(newLon - this.stationOverride.longitude) > 0.0001) {
                        
                        this.stationOverride.latitude = newLat;
                        this.stationOverride.longitude = newLon;
                        
                        // Auto-apply if override is already active
                        if (this.stationOverride.active) {
                            this.applyStationOverride();
                        }
                    }
                    
                    this.stationOverride.geolocationStatus = `Tracking location (±${Math.round(position.coords.accuracy)}m)`;
                },
                (error) => {
                    let errorMessage = 'Location tracking failed';
                    switch(error.code) {
                        case error.PERMISSION_DENIED:
                            errorMessage = 'Location access denied';
                            this.stationOverride.autoUpdate = false;
                            break;
                        case error.POSITION_UNAVAILABLE:
                            errorMessage = 'Location unavailable';
                            break;
                        case error.TIMEOUT:
                            errorMessage = 'Location request timeout';
                            break;
                    }
                    this.stationOverride.geolocationStatus = errorMessage;
                    console.error('Geolocation watch error:', error);
                },
                {
                    enableHighAccuracy: true,
                    timeout: 15000,
                    maximumAge: this.stationOverride.updateInterval * 1000
                }
            );
        },

        stopGeolocationWatch() {
            if (this.stationOverride.geolocationWatchId) {
                navigator.geolocation.clearWatch(this.stationOverride.geolocationWatchId);
                this.stationOverride.geolocationWatchId = null;
                this.stationOverride.geolocationStatus = 'Location tracking stopped';
            }
        },

        updateGeolocationInterval() {
            if (this.stationOverride.autoUpdate) {
                // Restart watch with new interval
                this.startGeolocationWatch();
            }
        },

        // Debug function to show animation stats
        getAnimationStats() {
            if (this.animationEngine) {
                const stats = this.animationEngine.getStats();
                console.log('Animation Engine Stats:', stats);
                return stats;
            }
            return null;
        },

        applyFilters() {
            this.onFilterChange();
        },



        updateSecondsSinceLastAudio() {
            if (!audioClient) { console.warn("audioClient not ready in updateSecondsSinceLastAudio (store)"); return; }
            audioClient.updateSecondsSinceLastAudio(this.secondsSinceLastAudio); // Pass the store's object to be updated
        },

// CRITICAL: Choose between WebSocket streaming and HTTP polling based on server config
async initAircraftDataSource() {
    try {
        console.log('[AIRCRAFT DATA] Checking server configuration for data source...');
        
        // Fetch server config to determine if WebSocket streaming is enabled
        const response = await fetch('/api/v1/config');
        if (!response.ok) {
            throw new Error(`Config request failed: ${response.status}`);
        }
        
        const config = await response.json();
        const streamingEnabled = config.adsb?.websocket_aircraft_updates;
        
        // ALWAYS establish WebSocket connection for alerts (phase changes, transcriptions, etc.)
        console.log('[WEBSOCKET] Establishing WebSocket connection for alerts...');
        this.initWebSocket();
        
        if (streamingEnabled) {
            console.log('[AIRCRAFT DATA] WebSocket streaming ENABLED - aircraft updates via WebSocket');
            // Aircraft data will come via WebSocket (handled in initWebSocket)
        } else {
            console.log('[AIRCRAFT DATA] WebSocket streaming DISABLED - aircraft updates via HTTP polling');
            // Disable aircraft streaming in WebSocket but keep alerts
            this.disableAircraftStreamingInWebSocket();
            // Use HTTP polling for aircraft data
            this.initHTTPPolling();
        }
        
    } catch (error) {
        console.error('[AIRCRAFT DATA] Failed to check server config, defaulting to HTTP polling:', error);
        this.initWebSocket(); // Still need WebSocket for alerts
        this.disableAircraftStreamingInWebSocket();
        this.initHTTPPolling();
    }
},

// Disable aircraft streaming in WebSocket but keep other alerts
disableAircraftStreamingInWebSocket() {
    console.log('[WEBSOCKET] Disabling aircraft streaming handlers - keeping alerts only');
    this.aircraftStreamingDisabled = true;
},

        // HTTP Polling fallback (the fast method that worked before WebSocket)
        initHTTPPolling() {
            console.log('[HTTP POLLING] Starting HTTP aircraft data polling...');
            
            // Load initial aircraft data
            this.fetchAircraftData();
            
            // Set up polling interval (use server's fetch interval)
            if (this.aircraftPollingInterval) {
                clearInterval(this.aircraftPollingInterval);
            }
            
            this.aircraftPollingInterval = setInterval(() => {
                this.fetchAircraftData();
            }, 2000); // Poll every 2 seconds for responsive updates
        },

        // Fetch aircraft data via HTTP API (restored from pre-WebSocket implementation)
        async fetchAircraftData() {
            try {
                // Build URL with current filter parameters
                const params = new URLSearchParams();
                
                // Add altitude filters
                params.append('min_altitude', this.settings.minAltitude.toString());
                params.append('max_altitude', this.settings.maxAltitude.toString());
                
                // Add last seen filter
                if (this.settings.lastSeenMinutes > 0) {
                    params.append('last_seen_minutes', this.settings.lastSeenMinutes.toString());
                }
                
                // Add ground traffic filter
                if (this.settings.excludeOtherAirportsGrounded) {
                    params.append('exclude_other_airports_grounded', 'true');
                }
                
                // Add search term if provided
                if (this.searchTerm && this.searchTerm.trim()) {
                    params.append('callsign', this.searchTerm.trim());
                }
                
                const url = `/api/v1/aircraft?${params.toString()}`;
                console.log(`[HTTP POLLING] Fetching: ${url}`);
                
                const response = await fetch(url);
                if (!response.ok) {
                    throw new Error(`HTTP ${response.status}`);
                }
                
                const data = await response.json();
                
                // CRITICAL FIX: Update aircraft individually to prevent table flashing
                const newAircraftMap = {};
                const currentHexes = new Set();
                let hasChanges = false;
                
                if (data.aircraft) {
                    for (const aircraft of data.aircraft) {
                        this.calculateAircraftDistance(aircraft);
                        
                        // Update animation engine with aircraft data (HTTP polling)
                        if (this.animationEngine) {
                            this.animationEngine.updateAircraft(aircraft);
                        }
                        
                        newAircraftMap[aircraft.hex] = aircraft;
                        currentHexes.add(aircraft.hex);
                    }
                }
                
                // Remove aircraft that are no longer present
                const existingHexes = Object.keys(this.aircraft);
                for (const hex of existingHexes) {
                    if (!currentHexes.has(hex)) {
                        delete this.aircraft[hex];
                        hasChanges = true;
                        console.log(`[HTTP POLLING] Removed aircraft: ${hex}`);
                        
                        // Remove from animation engine (HTTP polling)
                        if (this.animationEngine) {
                            this.animationEngine.removeAircraft(hex);
                        }
                        
                        // Update selected aircraft if it was removed
                        if (this.selectedAircraft && this.selectedAircraft.hex === hex) {
                            this.selectedAircraft = null;
                        }
                    }
                }
                
                // Add new aircraft and update existing ones
                for (const [hex, aircraft] of Object.entries(newAircraftMap)) {
                    const isNewAircraft = !this.aircraft[hex];
                    if (isNewAircraft) {
                        //console.log(`[HTTP POLLING] Added aircraft: ${hex}`);
                        hasChanges = true;
                    }
                    
                    // Always update the aircraft data (this updates existing aircraft without rebuilding table)
                    this.aircraft[hex] = aircraft;
                    
                    // Update selected aircraft details if this is the selected one
                    if (this.selectedAircraft && this.selectedAircraft.hex === hex) {
                        this.selectedAircraft = aircraft;
                    }
                }
                
                // Update counts
                this.counts = data.counts || {
                    ground_active: 0,
                    ground_total: 0,
                    air_active: 0,
                    air_total: 0
                };
                
                // Update connection status
                this.connected = true;
                this.lastUpdate = new Date();
                this.lastUpdateSeconds = 0;
                
                // Update map
                if (this.mapManager) {
                    const currentHexes = new Set(Object.keys(this.aircraft));
                    this.mapManager.removeStaleMarkers(currentHexes);
                    
                    for (const aircraft of Object.values(this.aircraft)) {
                        this.mapManager._ensureLeafletObjects(aircraft);
                    }
                    
                    this.mapManager.applyFiltersAndRefreshView();
                }
                
                // CRITICAL FIX: Only invalidate cache if aircraft were added/removed, not on every update
                if (hasChanges) {
                    this._lastFilterHash = null;
                    console.log(`[HTTP POLLING] Cache invalidated due to aircraft changes`);
                }
                
                console.log(`[HTTP POLLING] Updated ${Object.keys(this.aircraft).length} aircraft`);
                
            } catch (error) {
                console.error('[HTTP POLLING] Failed to fetch aircraft data:', error);
                this.connected = false;
            }
        },
        // Initialize WebSocket connection
                initWebSocket() {            if (!wsClient) {                console.error("wsClient not available during initWebSocket. This shouldn't happen.");                return;            }            // Reset reconnection attempts when manually initializing            if (wsClient.resetReconnectAttempts) {                wsClient.resetReconnectAttempts();            }            // Clear any existing listeners from previous initializations, if any            wsClient.listeners.transcription = [];            wsClient.listeners.transcription_update = [];            wsClient.listeners.phase_change = [];            wsClient.listeners.aircraft_event = [];            wsClient.listeners.open = [];            wsClient.listeners.close = [];            wsClient.listeners.error = [];

            // Add event listeners
            wsClient.addEventListener('transcription', (data) => {
                this.handleTranscriptionMessage(data);
            });
            
            wsClient.addEventListener('transcription_update', (data) => {
                this.handleTranscriptionUpdateMessage(data);
            });

            // Add event listeners for phase changes
            wsClient.addEventListener('phase_change', (data) => {
                this.handlePhaseChangeMessage(data);
            });

            // Add event listener for clearance events
            wsClient.addEventListener('clearance_issued', (data) => {
                this.handleClearanceIssued(data);
            });

            // Add new aircraft streaming handlers
            wsClient.addEventListener('aircraft_added', (data) => {
                this.handleAircraftAdded(data);
            });
            
            wsClient.addEventListener('aircraft_update', (data) => {
                this.handleAircraftUpdate(data);
            });
            
            wsClient.addEventListener('aircraft_removed', (data) => {
                this.handleAircraftRemoved(data);
            });
            
            // Add bulk data response handler
            wsClient.addEventListener('aircraft_bulk_response', (data) => {
                this.handleBulkAircraftData(data);
            });

            // Aircraft events are now handled as phase changes (T/O and T/D)
            
            wsClient.addEventListener('open', () => {
                console.log('App.js: WebSocket connection now open.');
                this.connected = true;
                // Reset the connection lost sound flag when connection is re-established
                this.connectionLostSoundPlayed = false;
                
                // Request initial aircraft data via WebSocket (this will send filter update)
                console.log('WebSocket connected, requesting initial aircraft data...');
                this.requestInitialAircraftData();
            });
            
            wsClient.addEventListener('close', (event) => {
                console.log('App.js: WebSocket connection closed.', event);
                
                // Only set connected to false if we previously had a successful connection
                // This prevents the CONNECTION LOST overlay from showing on initial page load
                if (this.connected === true) {
                    console.log('Connection was previously established and is now lost');
                    this.connected = false;
                    
                    // Play connection lost sound once when connection is lost
                    this.playConnectionLostSound();
                }
                
                // Note: Reconnection is now handled by WebSocketClient internally
                // No need to manually initiate reconnection here
            });
            
            wsClient.addEventListener('error', (event) => {
                console.error('App.js: WebSocket error:', event);
            });
            
            // Connect to the WebSocket server if not already connected or trying
            if (wsClient.connection === null || wsClient.connection.readyState === WebSocket.CLOSED) {
                 console.log("App.js: Attempting to connect WebSocket...");
                wsClient.connect();
            } else if (wsClient.connection.readyState === WebSocket.CONNECTING) {
                console.log("App.js: WebSocket is already connecting.");
            } else if (wsClient.connection.readyState === WebSocket.OPEN) {
                console.log("App.js: WebSocket is already open.");
            }
        },
        
        // Handle transcription message
        handleTranscriptionMessage(data) {
            // Add the transcription to the array
            this.transcriptions.push(data);
            
            // Keep only the last 100 transcriptions
            if (this.transcriptions.length > 100) {
                this.transcriptions.shift();
            }
            
            // Store transcription by frequency_id
            const freqId = data.frequency_id;
            
            // Initialize arrays if they don't exist
            if (!this.frequencyTranscriptions[freqId]) {
                this.frequencyTranscriptions[freqId] = [];
            }
            if (!this.originalTranscriptions[freqId]) {
                this.originalTranscriptions[freqId] = [];
            }
            
            // Always add to the original transcriptions array
            this.originalTranscriptions[freqId].unshift(data);
            
            // Limit stored original transcriptions per frequency
            if (this.originalTranscriptions[freqId].length > 99) {
                this.originalTranscriptions[freqId].pop();
            }
            
            // If there's an active search, check if the new transcription matches
            if (this.transcriptionSearchTerm) {
                const searchTerm = this.transcriptionSearchTerm.toLowerCase();
                const shouldInclude =
                    (data.text && data.text.toLowerCase().includes(searchTerm)) ||
                    (data.content_processed && data.content_processed.toLowerCase().includes(searchTerm)) ||
                    (data.callsign && data.callsign.toLowerCase().includes(searchTerm)) ||
                    (data.speaker_type && data.speaker_type.toLowerCase().includes(searchTerm));
                
                // Only add to the visible list if it matches the search
                if (shouldInclude) {
                    this.frequencyTranscriptions[freqId].unshift(data);
                }
            } else {
                // No search active, add normally
                this.frequencyTranscriptions[freqId].unshift(data);
            }

            // Limit stored visible transcriptions per frequency
            if (this.frequencyTranscriptions[freqId].length > 99) {
                this.frequencyTranscriptions[freqId].pop();
            }
            
            console.log('Transcription for', freqId, data);
        },
        
        // Filter transcriptions based on search term
        filterTranscriptions(frequencyId) {
            if (!this.frequencyTranscriptions[frequencyId]) {
                return;
            }
            
            // If search term is empty, restore from original transcriptions
            if (!this.transcriptionSearchTerm || this.transcriptionSearchTerm.trim() === '') {
                if (this.originalTranscriptions[frequencyId]) {
                    // Copy the original transcriptions to the visible array
                    this.frequencyTranscriptions[frequencyId] = [...this.originalTranscriptions[frequencyId]];
                } else {
                    // Fallback to API if original transcriptions aren't available
                    this.fetchTranscriptionsForFrequency(frequencyId);
                }
                return;
            }
            
            const searchTerm = this.transcriptionSearchTerm.toLowerCase();
            
            // Use original transcriptions as the source for filtering
            if (this.originalTranscriptions[frequencyId]) {
                const originalData = [...this.originalTranscriptions[frequencyId]];
                
                // Create a filtered copy of the transcriptions
                const filtered = originalData.filter(transcription => {
                    // Search in the text content
                    if (transcription.text && transcription.text.toLowerCase().includes(searchTerm)) {
                        return true;
                    }
                    
                    // Search in the processed content if available
                    if (transcription.content_processed &&
                        transcription.content_processed.toLowerCase().includes(searchTerm)) {
                        return true;
                    }
                    
                    // Search in the callsign if available
                    if (transcription.callsign &&
                        transcription.callsign.toLowerCase().includes(searchTerm)) {
                        return true;
                    }
                    
                    // Search in the speaker type if available
                    if (transcription.speaker_type &&
                        transcription.speaker_type.toLowerCase().includes(searchTerm)) {
                        return true;
                    }
                    
                    return false;
                });
                
                // Replace the array with the filtered version
                this.frequencyTranscriptions[frequencyId] = filtered;
            } else {
                // Fallback to API if original transcriptions aren't available
                this.fetchTranscriptionsForFrequency(frequencyId).then(data => {
                    // Store as original and then filter
                    this.originalTranscriptions[frequencyId] = [...data];
                    this.filterTranscriptions(frequencyId); // Call again now that we have original data
                });
            }
        },
        
        // Handle transcription update messages (processed transcriptions)
        handleTranscriptionUpdateMessage(data) {
            console.log('Received transcription update:', data);
            
            if (!data.id) {
                console.error('Received transcription update without ID:', data);
                return;
            }
            
            // Find the original transcription in the array by ID only
            const index = this.transcriptions.findIndex(t => t.id === data.id);
                
            if (index !== -1) {
                // Update the existing transcription with processed content
                const updatedTranscription = {
                    ...this.transcriptions[index],
                    content_processed: data.content_processed,
                    speaker_type: data.speaker_type,
                    callsign: data.callsign,
                    is_processed: true
                };
                
                // Replace the transcription in the array
                this.transcriptions.splice(index, 1, updatedTranscription);
                
                // Update the frequency transcriptions array
                if (!this.frequencyTranscriptions[data.frequency_id]) {
                    this.frequencyTranscriptions[data.frequency_id] = [];
                }
                
                // Find the transcription in the frequency-specific array by ID only
                const freqIndex = this.frequencyTranscriptions[data.frequency_id].findIndex(t => t.id === data.id);
                
                if (freqIndex !== -1) {
                    console.log(`Found transcription at index ${freqIndex}, updating...`);
                    // Create a new object with all properties from the original and the update
                    const updatedFreqTranscription = {
                        ...this.frequencyTranscriptions[data.frequency_id][freqIndex],
                        content_processed: data.content_processed,
                        speaker_type: data.speaker_type,
                        callsign: data.callsign,
                        is_processed: true
                    };
                    
                    // Replace the transcription in the array
                    this.frequencyTranscriptions[data.frequency_id].splice(freqIndex, 1, updatedFreqTranscription);
                } else {
                    console.warn(`Could not find transcription with id ${data.id} in frequency ${data.frequency_id} array`);
                }
                
                // Also update in the originalTranscriptions array
                if (this.originalTranscriptions[data.frequency_id]) {
                    const origIndex = this.originalTranscriptions[data.frequency_id].findIndex(t => t.id === data.id);
                    if (origIndex !== -1) {
                        // Create a new object with all properties from the original and the update
                        const updatedOrigTranscription = {
                            ...this.originalTranscriptions[data.frequency_id][origIndex],
                            content_processed: data.content_processed,
                            speaker_type: data.speaker_type,
                            callsign: data.callsign,
                            is_processed: true
                        };
                        
                        // Replace the transcription in the array
                        this.originalTranscriptions[data.frequency_id].splice(origIndex, 1, updatedOrigTranscription);
                    } else {
                        console.warn(`Could not find transcription with id ${data.id} in original transcriptions array for frequency ${data.frequency_id}`);
                    }
                }
            } else {
                console.warn(`Could not find transcription with id ${data.id} in main transcriptions array`);
            }
        },

        // Handle phase change messages
        handlePhaseChangeMessage(data) {
            console.log('Received phase change:', data);
            
            // Create alert message for phase change
            const message = `${data.flight || data.hex} changed phase: ${data.transition}`;
            
            // Add to alerts using existing alert system
            this.addPhaseChangeAlert(data);
            
            // Trigger visual effect on the map for takeoff/landing (T/O and T/D phases)
            if ((data.phase === 'T/O' || data.phase === 'T/D') && this.mapManager) {
                const eventType = data.phase === 'T/O' ? 'takeoff' : 'landing';
                this.mapManager.showTakeoffLandingEffect(data.hex, eventType, data.phase);
            }
            
            // Log to console for debugging
            console.log(`Phase change alert: ${message}`);
        },

        // Handle clearance issued messages
        handleClearanceIssued(data) {
            console.log('Clearance issued:', data);
            
            // Update aircraft clearances if currently selected
            const selectedAircraft = this.selectedAircraft;
            if (selectedAircraft && selectedAircraft.flight === data.callsign) {
                // Refresh aircraft details to show new clearance
                this.refreshSelectedAircraftDetails();
            }
            
            // Show alert for clearance
            this.showClearanceAlert(data);
            
            // Log to console for debugging
            console.log(`Clearance issued: ${data.callsign} → ${data.clearance_type.toUpperCase()} CLEARANCE`);
        },

        // Show clearance alert
        showClearanceAlert(clearanceData) {
            const alertText = `${clearanceData.callsign} → ${clearanceData.clearance_type.toUpperCase()} CLEARANCE`;
            let alertClass;
            
            switch(clearanceData.clearance_type) {
                case 'takeoff':
                    alertClass = 'alert-takeoff';
                    break;
                case 'landing':
                    alertClass = 'alert-landing';
                    break;
                case 'approach':
                    alertClass = 'alert-approach';
                    break;
                default:
                    alertClass = 'alert-clearance';
            }
            
            // Add to alerts system (reusing existing alert infrastructure)
            this.addAlert(alertText, alertClass, clearanceData.callsign);
        },

        // Refresh selected aircraft details
        refreshSelectedAircraftDetails() {
            if (this.selectedAircraft) {
                // Re-fetch aircraft data to get updated clearances
                this.selectAircraft(this.selectedAircraft.hex);
            }
        },

        // Aircraft events are now handled as phase changes (T/O and T/D)
        
        // Highlight search term in text with a subtle red underline
        highlightSearchTerm(text) {
            if (!text || !this.transcriptionSearchTerm || this.transcriptionSearchTerm.trim() === '') {
                return text;
            }
            
            // Escape special characters for regex
            const escapeRegExp = (string) => {
                return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
            };
            
            const searchTerm = escapeRegExp(this.transcriptionSearchTerm.trim());
            const regex = new RegExp(`(${searchTerm})`, 'gi');
            
            // Replace matches with the same text but with a red underline
            return text.replace(regex, '<span class="border-b border-red-400">${1}</span>');
        },
        
        // Handle aircraft movement message
        handleAircraftMessage(data) {
            if (data && data.movement) {
                this.addAircraftAlert(data);
            }
        },
        
        // Handle status update message
        handleStatusUpdateMessage(data) {
            if (data && data.hex) {
                console.log(`Status update for ${data.flight || data.hex}: ${data.new_status}`, data);
                
                // Skip new_aircraft alerts - these are now handled by phase changes
                if (data.new_status === "new_aircraft") {
                    return;
                }
                
                // Skip signal_lost alerts for CRZ phase and grounded aircraft - these are expected/normal and create spam
                // Still alert for signal loss in critical phases like DEP, ARR, APP, T/O, T/D
                if (data.new_status === "signal_lost") {
                    const aircraft = this.aircraft[data.hex];
                    if (aircraft) {
                        // Update the aircraft status in the local store
                        aircraft.status = data.new_status;
                        
                        // Skip alerts for CRZ phase or grounded aircraft (expected signal loss)
                        const currentPhase = aircraft.phase?.current?.[0]?.phase;
                        if (currentPhase === 'CRZ' || data.on_ground) {
                            return;
                        }
                        // For other phases (DEP, ARR, APP, T/O, T/D, etc.), continue to show alert
                    } else {
                        // If aircraft not found in store, skip the alert (likely already removed)
                        return;
                    }
                }
                
                // Update the aircraft status in the local store if it exists
                if (this.aircraft[data.hex]) {
                    this.aircraft[data.hex].status = data.new_status;
                    
                    // Add alert for status change
                    this.addStatusAlert(data);
                }
            }
        },
        
        // NEW: Request initial aircraft data via WebSocket
        requestInitialAircraftData() {
            // CRITICAL: Don't request bulk data if aircraft streaming is disabled
            if (this.aircraftStreamingDisabled) {
                console.log('[AIRCRAFT STREAMING DISABLED] Skipping bulk aircraft request');
                return;
            }
            
            console.log('Sending initial filter update to get aircraft data');
            // Send filter update which will return filtered aircraft data
            this.sendFilterUpdate();
        },

        // NEW: Build current filter object from settings
        buildCurrentFilters() {
            return {
                min_altitude: this.settings.minAltitude,
                max_altitude: this.settings.maxAltitude,
                last_seen_minutes: this.settings.lastSeenMinutes,
                exclude_other_airports_grounded: this.settings.excludeOtherAirportsGrounded,
                show_air: this.settings.showAirAircraft,
                show_ground: this.settings.showGroundAircraft,
                phases: this.settings.phaseFilters || {},
                selected_aircraft_hex: this.selectedAircraft?.hex || null
            };
        },

        // NEW: Handle bulk aircraft data response
        handleBulkAircraftData(data) {
            console.log(`Received bulk aircraft data: ${data.count} aircraft`);
            
            // CRITICAL: Skip bulk aircraft updates if streaming is disabled (using HTTP polling instead)
            if (this.aircraftStreamingDisabled) {
                console.log(`[AIRCRAFT STREAMING DISABLED] Skipping bulk aircraft data`);
                return;
            }
            
            // Clear current aircraft data
            this.aircraft = {};
            
            // Process each aircraft
            if (data.aircraft && Array.isArray(data.aircraft)) {
                for (const aircraft of data.aircraft) {
                    // Calculate distance
                    this.calculateAircraftDistance(aircraft);
                    
                    // Update animation engine with aircraft data
                    if (this.animationEngine) {
                        this.animationEngine.updateAircraft(aircraft);
                    }
                    
                    // Add to aircraft map (all aircraft from bulk response are pre-filtered)
                    this.aircraft[aircraft.hex] = aircraft;
                }
            }
            
            // Update counts
            this.counts = data.counts || {
                ground_active: 0,
                ground_total: 0,
                air_active: 0,
                air_total: 0
            };
            
            // Update connection status
            this.connected = true;
            this.lastUpdate = new Date();
            this.lastUpdateSeconds = 0;
            
            // Update map with all aircraft
            if (this.mapManager) {
                // Clear existing markers that are no longer in the current aircraft set
                const currentHexes = new Set(Object.keys(this.aircraft));
                this.mapManager.removeStaleMarkers(currentHexes);
                
                // Update all aircraft markers
                for (const aircraft of Object.values(this.aircraft)) {
                    this.mapManager._ensureLeafletObjects(aircraft);
                }
                
                // Refresh view and apply filters
                this.mapManager.applyFiltersAndRefreshView();
            }
            
            // Invalidate filtered aircraft cache
            this._lastFilterHash = null;
            
            console.log('Bulk aircraft data processing complete');
        },

        // Throttling state for performance optimization
        pendingMapUpdates: new Set(),
        mapUpdateThrottleId: null,
        cacheInvalidationPending: false,
        
        // Filtering throttling state to prevent main thread blocking
        _filteringScheduled: false,
        _lastFilterTime: null,
        
        // HTTP polling state
        aircraftPollingInterval: null,
        aircraftStreamingDisabled: false,

        // ASYNC Performance-optimized aircraft handlers to prevent main thread blocking
        handleAircraftAdded(data) {
            if (this.wsUpdatesPaused) {
                console.log(`[PAUSED] Skipping aircraft added: ${data.aircraft?.flight || data.hex}`);
                return;
            }
            
            // CRITICAL: Skip aircraft updates if streaming is disabled (using HTTP polling instead)
            if (this.aircraftStreamingDisabled) {
                console.log(`[AIRCRAFT STREAMING DISABLED] Skipping aircraft added: ${data.aircraft?.flight || data.hex}`);
                return;
            }
            
            // CRITICAL FIX: Process asynchronously to prevent main thread blocking
            setTimeout(() => {
                if (data.aircraft) {
                    console.log(`Adding new aircraft: ${data.aircraft.flight || data.hex}`);
                    
                    // Apply distance calculation
                    this.calculateAircraftDistance(data.aircraft);
                    
                    // Update animation engine with new aircraft data
                    if (this.animationEngine) {
                        this.animationEngine.updateAircraft(data.aircraft);
                    }
                    
                    // Check if aircraft passes current filters
                    if (this.aircraftPassesFilters(data.aircraft)) {
                        // Add to aircraft map
                        this.aircraft[data.aircraft.hex] = data.aircraft;
                        
                        // Queue for throttled map update
                        this.queueMapUpdate(data.aircraft.hex);
                        
                        // Queue cache invalidation
                        this.queueCacheInvalidation();
                    }
                }
            }, 0); // Yield to allow other operations
        },

        handleAircraftUpdate(data) {
            if (this.wsUpdatesPaused) {
                console.log(`[PAUSED] Skipping aircraft update: ${data.hex}`);
                return;
            }
            
            // CRITICAL: Skip aircraft updates if streaming is disabled (using HTTP polling instead)
            if (this.aircraftStreamingDisabled) {
                console.log(`[AIRCRAFT STREAMING DISABLED] Skipping aircraft update: ${data.hex}`);
                return;
            }
            
            // CRITICAL FIX: Process asynchronously to prevent main thread blocking
            setTimeout(() => {
                if (data.aircraft) {
                    //console.log(`Updating aircraft: ${data.hex}`);
                    
                    // Replace entire aircraft object (no incremental changes)
                    // This aligns with HTTP API payload structure for consistency
                    this.aircraft[data.aircraft.hex] = data.aircraft;
                    
                    // Apply distance calculation (same as HTTP polling)
                    this.calculateAircraftDistance(data.aircraft);
                    
                    // Update animation engine with new aircraft data
                    if (this.animationEngine) {
                        this.animationEngine.updateAircraft(data.aircraft);
                    }
                    
                    // Check if aircraft passes filters and update map
                    if (this.aircraftPassesFilters(data.aircraft)) {
                        // Queue for throttled map update
                        this.queueMapUpdate(data.hex);
                    } else {
                        // Remove from display if it no longer passes filters
                        delete this.aircraft[data.hex];
                        if (this.mapManager) {
                            this.mapManager.removeAircraft(data.hex);
                        }
                        // Remove from animation engine
                        if (this.animationEngine) {
                            this.animationEngine.removeAircraft(data.hex);
                        }
                    }
                    
                    // CRITICAL FIX: Update aircraft details panel if this is the selected aircraft
                    if (this.selectedAircraft && this.selectedAircraft.hex === data.aircraft.hex) {
                        // Update the selectedAircraft reference to the new data
                        this.selectedAircraft = data.aircraft;
                        // Refresh the aircraft details panel with the updated data
                        this.setupAircraftDetailsPanel();
                    }
                    
                    // Queue cache invalidation
                    this.queueCacheInvalidation();
                }
            }, 0); // Yield to allow other operations
        },

        handleAircraftRemoved(data) {
            if (this.wsUpdatesPaused) {
                console.log(`[PAUSED] Skipping aircraft removed: ${data.hex}`);
                return;
            }
            
            // CRITICAL: Skip aircraft updates if streaming is disabled (using HTTP polling instead)
            if (this.aircraftStreamingDisabled) {
                console.log(`[AIRCRAFT STREAMING DISABLED] Skipping aircraft removed: ${data.hex}`);
                return;
            }
            
            // CRITICAL FIX: Process asynchronously to prevent main thread blocking
            setTimeout(() => {
                console.log(`Removing aircraft: ${data.hex}`);
                
                if (this.aircraft[data.hex]) {
                    delete this.aircraft[data.hex];
                    
                    // Remove from map immediately (removal is fast)
                    if (this.mapManager) {
                        this.mapManager.removeAircraft(data.hex);
                    }
                    
                    // Remove from animation engine
                    if (this.animationEngine) {
                        this.animationEngine.removeAircraft(data.hex);
                    }
                    
                    // Deselect if this was the selected aircraft
                    if (this.selectedAircraft && this.selectedAircraft.hex === data.hex) {
                        this.selectedAircraft = null;
                    }
                    
                    // Queue cache invalidation
                    this.queueCacheInvalidation();
                }
            }, 0); // Yield to allow other operations
        },

        // Removed applyAircraftChanges method - no longer needed since we receive full aircraft objects

        // Queue map updates for throttling
        queueMapUpdate(hex) {
            if (this.wsUpdatesPaused) {
                console.log(`[PAUSED] Skipping map update queue for: ${hex}`);
                return;
            }
            // Removed mapUpdatesDisabled and dataOnlyMode debug checks
            
            
            this.pendingMapUpdates.add(hex);
            
            // Throttling: 500ms
            if (!this.mapUpdateThrottleId) {
                this.mapUpdateThrottleId = setTimeout(() => {
                    this.processPendingMapUpdates();
                    this.mapUpdateThrottleId = null;
                }, 500);
            }
        },


        // Process all pending map updates in a batch
        processPendingMapUpdates() {
            if (this.pendingMapUpdates.size === 0) return;
            
            console.log(`Processing ${this.pendingMapUpdates.size} pending map updates`);
            
            // PERFORMANCE OPTIMIZATION: Limit batch size to prevent DOM overload
            const maxBatchSize = 10; // Process max 10 aircraft per batch
            const aircraftToUpdate = Array.from(this.pendingMapUpdates).slice(0, maxBatchSize);
            
            // Update specific aircraft markers instead of full refresh
            aircraftToUpdate.forEach(hex => {
                const aircraft = this.aircraft[hex];
                if (aircraft && this.mapManager) {
                    this.mapManager.updateSingleAircraft(hex, aircraft);
                }
                this.pendingMapUpdates.delete(hex);
            });
            
            // If more updates remain, schedule another batch
            if (this.pendingMapUpdates.size > 0) {
                console.log(`${this.pendingMapUpdates.size} updates remaining, scheduling next batch...`);
                this.mapUpdateThrottleId = setTimeout(() => {
                    this.processPendingMapUpdates();
                    this.mapUpdateThrottleId = null;
                }, 500); // Shorter delay for remaining batches
            }
        },

        // Queue cache invalidation to reduce frequency - VERY AGGRESSIVE
        queueCacheInvalidation() {
            if (!this.cacheInvalidationPending) {
                this.cacheInvalidationPending = true;
                
                // AGGRESSIVE: Increase delay from 50ms to 1000ms to batch many more changes
                setTimeout(() => {
                    // Don't null the cache, just invalidate the hash to trigger re-filtering
                    // This maintains the stable array reference to prevent full table re-renders
                    this._lastFilterHash = null;
                    this.cacheInvalidationPending = false;
                }, 1000); // Increased from 50ms to 1000ms
            }
        },

        // Method to request new aircraft data when settings change
        requestNewAircraftData() {
            console.log('Requesting new aircraft data due to setting change');
            this.saveSettings();
            
            // CRITICAL: Use different approach based on streaming mode
            if (this.aircraftStreamingDisabled) {
                console.log('[AIRCRAFT STREAMING DISABLED] Refreshing via HTTP polling');
                // Immediately fetch new data via HTTP with updated filters
                this.fetchAircraftData();
            } else {
                console.log('[AIRCRAFT STREAMING ENABLED] Sending filter update');
                // Send filter update which will return filtered aircraft data
                this.sendFilterUpdate();
            }
        },

        // Send filter update to server for real-time filtering
        sendFilterUpdate() {
            // Only send if WebSocket streaming is enabled
            if (this.aircraftStreamingDisabled) {
                console.log('[AIRCRAFT STREAMING DISABLED] Skipping filter update');
                return;
            }

            if (wsClient && wsClient.connection && wsClient.connection.readyState === WebSocket.OPEN) {
                // Send all current filters to server
                const filters = this.buildCurrentFilters();
                wsClient.updateFilters(filters);
                console.log('[FILTER UPDATE] Sent to server:', filters);
            } else {
                console.warn('[FILTER UPDATE] WebSocket not connected, cannot send filter update');
            }
        },

        calculateAircraftDistance(aircraft) {
            if (aircraft.adsb && aircraft.adsb.lat && aircraft.adsb.lon &&
                this.stationLatitude && this.stationLongitude) {
                const distanceNM = this.haversineDistance(
                    aircraft.adsb.lat, aircraft.adsb.lon,
                    this.stationLatitude, this.stationLongitude
                );
                aircraft.distance = Math.round(distanceNM * 10) / 10;
            }
        },

        haversineDistance(lat1, lon1, lat2, lon2) {
            const R = 3440.065; // Earth's radius in nautical miles
            const dLat = this.toRadians(lat2 - lat1);
            const dLon = this.toRadians(lon2 - lon1);
            const a = Math.sin(dLat/2) * Math.sin(dLat/2) +
                      Math.cos(this.toRadians(lat1)) * Math.cos(this.toRadians(lat2)) *
                      Math.sin(dLon/2) * Math.sin(dLon/2);
            const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1-a));
            return R * c;
        },

        toRadians(degrees) {
            return degrees * (Math.PI/180);
        },

        aircraftPassesFilters(aircraft) {
            // Apply all current filters to determine if aircraft should be displayed
            const searchLower = this.searchTerm.toLowerCase();
            
            // Search filter
            if (searchLower) {
                const callsign = (aircraft.flight || aircraft.hex).toLowerCase();
                const type = (aircraft.adsb?.type || '').toLowerCase();
                const category = (aircraft.adsb?.category || '').toLowerCase();
                
                const matchesSearch = callsign.includes(searchLower) ||
                                    type.includes(searchLower) ||
                                    category.includes(searchLower);
                
                if (!matchesSearch) return false;
            }
            
            // Air/Ground filter
            const showThisAircraft = (aircraft.on_ground && this.settings.showGroundAircraft) ||
                                   (!aircraft.on_ground && this.settings.showAirAircraft);
            
            if (!showThisAircraft) return false;
            
            // Phase filter
            const currentPhase = this.getCurrentPhase(aircraft);
            if (this.settings.phaseFilters && this.settings.phaseFilters[currentPhase] === false) {
                return false;
            }
            
            // Altitude filter (for air aircraft)
            if (!aircraft.on_ground && aircraft.adsb &&
                (aircraft.adsb.alt_baro < this.settings.minAltitude ||
                 aircraft.adsb.alt_baro > this.settings.maxAltitude)) {
                return false;
            }
            
            return true;
        },
        
        // Select an aircraft by callsign (for clicking on transcription callsigns)
        selectAircraftByCallsign(callsign) {
            if (!callsign) return;
            
            console.log(`Attempting to select aircraft with callsign: ${callsign}`);
            
            // Normalize the callsign for comparison (trim whitespace, uppercase)
            const normalizedCallsign = callsign.trim().toUpperCase();
            
            // Find the aircraft with the matching callsign
            const foundAircraft = Object.values(this.aircraft).find(aircraft => {
                const aircraftCallsign = (aircraft.flight || '').trim().toUpperCase();
                return aircraftCallsign === normalizedCallsign;
            });
            
            if (foundAircraft) {
                console.log(`Found aircraft with callsign ${callsign}:`, foundAircraft);
                
                // Set as selected aircraft
                this.selectedAircraft = foundAircraft;
                
                // Notify server about selected aircraft change for real-time updates
                this.sendFilterUpdate();
                
                // Update aircraft details panel
                this.setupAircraftDetailsPanel();
                
                // Update map to highlight the aircraft
                if (this.mapManager) {
                    this.mapManager.updateVisualState(foundAircraft.hex, true);
                    
                    // Center map on the aircraft if it has coordinates
                    if (foundAircraft.adsb && foundAircraft.adsb.lat && foundAircraft.adsb.lon && this.mapManager.map) {
                        this.mapManager.map.setView([foundAircraft.adsb.lat, foundAircraft.adsb.lon], this.mapManager.map.getZoom());
                    }
                }
            } else {
                console.warn(`Could not find aircraft with callsign: ${callsign}`);
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
                    if (foundAircraft.adsb && foundAircraft.adsb.lat && foundAircraft.adsb.lon && this.mapManager.map) {
                        this.mapManager.map.setView([foundAircraft.adsb.lat, foundAircraft.adsb.lon], this.mapManager.map.getZoom());
                    }
                }
            } else {
                console.warn(`Could not find aircraft with hex: ${hex}`);
            }
        },
        
        // Add aircraft alert
        addAircraftAlert(data) {
            // Create a unique ID for the alert
            const alertId = Date.now() + '-' + data.hex;
            
            // Add the alert to the array for tracking
            this.aircraftAlerts.push({
                id: alertId,
                hex: data.hex,
                flight: data.flight || data.hex,
                movement: data.movement,
                timestamp: data.timestamp || new Date().toISOString()
            });
            
            // Get the alerts container
            const alertsContainer = document.getElementById('alerts-container');
            if (!alertsContainer) return;
            
            // Hide the "None" text
            const noAlertsText = document.getElementById('no-alerts-text');
            if (noAlertsText) {
                noAlertsText.style.display = 'none';
            }
            
            // Create the alert element
            const alertElement = document.createElement('div');
            alertElement.id = alertId;
            alertElement.className = `inline-flex items-center text-xs px-1.5 py-0.5 rounded ${data.movement === 'tookoff' ? 'text-blue-300' : 'text-green-300'} cursor-pointer hover:bg-black/50`;
            
            // Add left-click event to dismiss the alert
            alertElement.addEventListener('click', () => {
                this.removeAircraftAlert(alertId);
            });
            
            // Add right-click event to select the aircraft and center the map on it
            alertElement.addEventListener('contextmenu', (e) => {
                e.preventDefault(); // Prevent the default context menu
                this.selectAircraftByHex(data.hex);
            });
            
            // Create the icon
            const icon = document.createElement('i');
            icon.className = `fas fa-xs mr-1 ${data.movement === 'tookoff' ? 'fa-plane-departure' : 'fa-plane-arrival'}`;
            alertElement.appendChild(icon);
            
            // Create the text
            const text = document.createElement('span');
            text.textContent = data.flight || data.hex;
            alertElement.appendChild(text);
            
            // Add the alert to the container
            alertsContainer.appendChild(alertElement);
            
            // Remove the alert after 60 seconds
            setTimeout(() => {
                this.removeAircraftAlert(alertId);
            }, 60000);
        },

        // Check if an alert should be shown based on Air/Ground filter settings
        shouldShowAlert(hex) {
            const aircraft = this.aircraft[hex];
            if (!aircraft) return true; // Show alert if we don't have aircraft data yet
            
            // Check if aircraft matches current Air/Ground filter settings
            const showThisAircraft = (aircraft.on_ground && this.settings.showGroundAircraft) ||
                                   (!aircraft.on_ground && this.settings.showAirAircraft);
            
            return showThisAircraft;
        },

        // Refresh alerts display based on current Air/Ground filter settings and phase filters
        refreshAlertsDisplay() {
            const alertsContainer = document.getElementById('alerts-container');
            if (!alertsContainer) return;

            // Check each alert to see if it should be visible
            this.aircraftAlerts.forEach(alert => {
                const alertElement = document.getElementById(alert.id);
                if (alertElement) {
                    let shouldShow = this.shouldShowAlert(alert.hex);
                    
                    // Additional check for phase change alerts
                    if (alert.type === 'phase_change' && alert.data && alert.data.phase) {
                        shouldShow = shouldShow && this.shouldShowPhaseAlert(alert.data.phase);
                    }
                    
                    if (shouldShow) {
                        alertElement.style.display = 'inline-flex';
                    } else {
                        alertElement.style.display = 'none';
                    }
                }
            });

            // Check if any alerts are visible to show/hide "None" text
            const visibleAlerts = this.aircraftAlerts.filter(alert => {
                const alertElement = document.getElementById(alert.id);
                return alertElement && alertElement.style.display !== 'none';
            });

            const noAlertsText = document.getElementById('no-alerts-text');
            if (noAlertsText) {
                noAlertsText.style.display = visibleAlerts.length === 0 ? 'block' : 'none';
            }
        },
        
        // Add status alert
        addStatusAlert(data) {
            // Check if we should show this alert based on Air/Ground filter settings
            if (!this.shouldShowAlert(data.hex)) {
                return; // Don't show alert if aircraft doesn't match current filter
            }
            
            // Create a unique ID for the alert
            const alertId = Date.now() + '-status-' + data.hex;
            
            // Add the alert to the array for tracking
            this.aircraftAlerts.push({
                id: alertId,
                hex: data.hex,
                type: 'status',
                status: data.new_status,
                timestamp: new Date()
            });
            
            // Get the alerts container
            const alertsContainer = document.getElementById('alerts-container');
            if (!alertsContainer) return;
            
            // Hide the "None" text
            const noAlertsText = document.getElementById('no-alerts-text');
            if (noAlertsText) {
                noAlertsText.style.display = 'none';
            }
            
            // Create the alert element
            const alertElement = document.createElement('div');
            alertElement.id = alertId;
            
            // Set color based on status
            let colorClass = 'text-yellow-300'; // Default for stale
            let iconClass = 'fa-exclamation-triangle';
            let statusText = data.new_status.toUpperCase();
            
            // Add left-click event to dismiss the alert
            alertElement.addEventListener('click', () => {
                this.removeAircraftAlert(alertId);
            });
            
            // Add right-click event to select the aircraft and center the map on it
            alertElement.addEventListener('contextmenu', (e) => {
                e.preventDefault(); // Prevent the default context menu
                this.selectAircraftByHex(data.hex);
            });
            
            if (data.new_status === 'signal_lost') {
                colorClass = 'text-gray-400'; // Use grey for signal_lost
                iconClass = 'fa-ban';
                statusText = ''; // No additional text, just callsign
            }
            
            alertElement.className = `inline-flex items-center text-xs px-1.5 py-0.5 rounded ${colorClass} cursor-pointer hover:bg-black/50`;
            
            // Add icon
            const icon = document.createElement('i');
            icon.className = `fas ${iconClass} fa-xs mr-1`;
            alertElement.appendChild(icon);
            
            // Add text
            const text = document.createElement('span');
            text.textContent = `${data.flight || data.hex}${statusText}`;
            alertElement.appendChild(text);
            
            // Add the alert to the container
            alertsContainer.appendChild(alertElement);
            
            // Remove the alert after 60 seconds
            setTimeout(() => {
                this.removeAircraftAlert(alertId);
            }, 60000);
        },

        // Add phase change alert
        addPhaseChangeAlert(data) {
            // Check if we should show this alert based on Air/Ground filter settings
            if (!this.shouldShowAlert(data.hex)) {
                return; // Don't show alert if aircraft doesn't match current filter
            }

            // Check if we should show this alert based on phase filter settings
            if (!this.shouldShowPhaseAlert(data.phase)) {
                return; // Don't show alert if phase is filtered out
            }
            
            // Create a unique ID for the alert
            const alertId = Date.now() + '-phase-' + data.hex;

            // Add the alert to the array for tracking
            this.aircraftAlerts.push({
                id: alertId,
                hex: data.hex,
                type: 'phase_change',
                data: data
            });

            // Get the alerts container
            const alertsContainer = document.getElementById('alerts-container');
            if (!alertsContainer) return;

            // Hide the "None" text
            const noAlertsText = document.getElementById('no-alerts-text');
            if (noAlertsText) {
                noAlertsText.style.display = 'none';
            }

            // Use centralized color and icon mapping
            const colorClass = this.getPhaseColorClass(data.phase);
            const iconClass = this.getPhaseIconClass(data.phase);
            const displayText = `${data.flight || data.hex} → ${data.phase}`;

            // Create the alert element
            const alertElement = document.createElement('div');
            alertElement.id = alertId;
            alertElement.className = `inline-flex items-center text-xs px-1.5 py-0.5 rounded ${colorClass} cursor-pointer hover:bg-black/50`;

            // Add left-click event to dismiss the alert
            alertElement.addEventListener('click', () => {
                this.removeAircraftAlert(alertId);
            });

            // Add right-click event to select the aircraft and center the map on it
            alertElement.addEventListener('contextmenu', (e) => {
                e.preventDefault(); // Prevent the default context menu
                this.selectAircraftByHex(data.hex);
            });

            // Create icon
            const icon = document.createElement('i');
            icon.className = `fas ${iconClass} fa-xs mr-1`;
            alertElement.appendChild(icon);

            // Create text
            const text = document.createElement('span');
            text.textContent = displayText;
            alertElement.appendChild(text);

            // Add the alert to the container
            alertsContainer.appendChild(alertElement);

            // Remove the alert after 60 seconds
            setTimeout(() => {
                this.removeAircraftAlert(alertId);
            }, 60000);
        },

        // Aircraft events are now handled as phase changes (T/O and T/D)

        // Remove aircraft alert
        removeAircraftAlert(alertId) {
            // Remove from the array
            this.aircraftAlerts = this.aircraftAlerts.filter(alert => alert.id !== alertId);
            
            // Remove from the DOM
            const alertElement = document.getElementById(alertId);
            if (alertElement) {
                alertElement.remove();
            }
            
            // Show the "None" text if there are no alerts
            if (this.aircraftAlerts.length === 0) {
                const noAlertsText = document.getElementById('no-alerts-text');
                if (noAlertsText) {
                    noAlertsText.style.display = 'block';
                }
            }
        },

        // Clear all aircraft alerts
        clearAllAircraftAlerts() {
            // Remove all alerts from DOM
            this.aircraftAlerts.forEach(alert => {
                const alertElement = document.getElementById(alert.id);
                if (alertElement) {
                    alertElement.remove();
                }
            });
            
            // Clear the array
            this.aircraftAlerts = [];
            
            // Show the "None" text
            const noAlertsText = document.getElementById('no-alerts-text');
            if (noAlertsText) {
                noAlertsText.style.display = 'block';
            }
        },

        getTranscriptionCount(frequencyId) {
            return this.frequencyTranscriptions[frequencyId]?.length || 0;
        },

        toggleTranscriptionViewer(frequencyId) {
            // Ensure the viewer state for this frequency is initialized
            if (this.transcriptionViewerVisible[frequencyId] === undefined) {
                this.transcriptionViewerVisible[frequencyId] = false;
            }
            
            // Toggle the state
            const newState = !this.transcriptionViewerVisible[frequencyId];
            this.transcriptionViewerVisible[frequencyId] = newState;
            
            // If closing the viewer, reset the search term
            if (!newState && this.transcriptionSearchTerm) {
                this.transcriptionSearchTerm = '';
                // Reload the original transcriptions if needed
                this.fetchTranscriptionsForFrequency(frequencyId);
            }
            
            // Position the transcription viewer correctly if it's being opened
            if (this.transcriptionViewerVisible[frequencyId]) {
                // Execute immediately with no delay
                const freqElement = document.querySelector(`[data-freq-id="${frequencyId}"]`);
                const viewer = document.querySelector(`[data-viewer-id="${frequencyId}"]`);
                
                if (freqElement && viewer) {
                    const rect = freqElement.getBoundingClientRect();
                    viewer.style.left = `${rect.left}px`;
                    viewer.style.width = `${rect.width}px`;
                    viewer.style.bottom = `${window.innerHeight - rect.top + 8}px`;
                    // Ensure no transitions or animations
                    viewer.style.transition = 'none';
                    viewer.style.transform = 'none';
                }
            }
        },
        
        // Fetch transcriptions for a specific frequency
        async fetchTranscriptionsForFrequency(frequencyId) {
            try {
                const response = await fetch(`${API_BASE_URL}/transcriptions/frequency/${frequencyId}`);
                if (response.ok) {
                    const data = await response.json();
                    if (data && data.transcriptions) {
                        // Sort newest first
                        this.frequencyTranscriptions[frequencyId] = data.transcriptions.sort((a, b) =>
                            new Date(b.timestamp) - new Date(a.timestamp)
                        );
                    }
                }
                return this.frequencyTranscriptions[frequencyId] || [];
            } catch (error) {
                console.error('Error fetching transcriptions:', error);
                return this.frequencyTranscriptions[frequencyId] || [];
            }
        },

        async fetchStationData() {
            try {
                const response = await fetch(this.stationApiUrl);
                if (!response.ok) {
                    console.error(`HTTP error fetching station data! Status: ${response.status}`);
                    // Fallback to some default if needed, or handle error appropriately
                    this.stationLatitude = 13.6777; // Default fallback
                    this.stationLongitude = -79.6248; // Default fallback
                    this.stationElevationFeet = 569; // Default fallback
                    return;
                }
                const data = await response.json();
                this.stationLatitude = data.latitude;
                this.stationLongitude = data.longitude;
                this.stationElevationFeet = data.elevation_feet;
                this.stationAirportCode = data.airport_code;
                
                // Check if station override is active and restore state
                if (data.override_active) {
                    this.stationOverride.latitude = data.latitude;
                    this.stationOverride.longitude = data.longitude;
                    this.stationOverride.active = true;
                    console.log('Station override restored from server:', {
                        lat: data.latitude,
                        lon: data.longitude
                    });
                    
                    // Update station rings to the override coordinates
                    if (this.mapManager) {
                        this.updateStationRings();
                    }
                } else {
                    // Ensure override state is cleared if not active
                    this.stationOverride.active = false;
                    this.stationOverride.latitude = null;
                    this.stationOverride.longitude = null;
                    
                    // Update station rings to use the new coordinates from API
                    if (this.mapManager && this.mapManager.map) {
                        this.updateStationRings();
                    }
                }
                
                // Store weather configuration flags
                this.stationFetchMETAR = data.fetch_metar;
                this.stationFetchTAF = data.fetch_taf;
                this.stationFetchNOTAMs = data.fetch_notams;
                
                // Store runway data if available
                if (data.runways) {
                    this.runwayData = data.runways;
                    console.log('Runway data loaded:', this.runwayData);
                    
                    // Draw runways on the map if mapManager is initialized
                    if (this.mapManager) {
                        this.mapManager.drawRunways(this.runwayData);
                    }
                }
                
                console.log('Station data loaded:', data);
                
                // Center map on station coordinates after loading
                if (this.mapManager) {
                    this.mapManager.centerOnStation();
                }
                
                // Setup refresh interval if not already set (less frequent since station data is static)
                if (!this.stationRefreshInterval) {
                    this.stationRefreshInterval = setInterval(() => {
                        console.log('Refreshing station data...');
                        this.fetchStationData();
                    }, CONFIG.stationRefreshInterval);
                }
            } catch (error) {
                console.error('Error fetching station data:', error);
                // Fallback to some default if needed, or handle error appropriately
                this.stationLatitude = 43.6777; // Default fallback on error
                this.stationLongitude = -79.6248; // Default fallback on error
                this.stationElevationFeet = 569;    // Default fallback on error
            }
        },

        async fetchWeatherData() {
            try {
                const response = await fetch(this.wxApiUrl);
                if (!response.ok) {
                    console.error(`HTTP error fetching weather data! Status: ${response.status}`);
                    return;
                }
                
                const data = await response.json();
                
                // Store weather data
                this.metar = data.metar;
                this.taf = data.taf;
                this.notams = data.notams;
                this.weatherLastUpdated = data.last_updated;
                this.weatherFetchErrors = data.fetch_errors || [];
                
                console.log('Weather data loaded:', data);
                
                // Setup weather refresh interval if not already set
                if (!this.weatherRefreshInterval) {
                    this.weatherRefreshInterval = setInterval(() => {
                        console.log('Refreshing weather data...');
                        this.fetchWeatherData();
                    }, CONFIG.weatherRefreshInterval);
                }
            } catch (error) {
                console.error('Error fetching weather data:', error);
            }
        },
        
        // Get the latest METAR data
        getLatestMetar() {
            if (!this.metar || !this.metar.trend || this.metar.trend.length === 0) {
                return null;
            }
            
            // Return the first (latest) METAR in the trend array
            return this.metar.trend[0];
        },
        
        // Toggle METAR details visibility
        toggleMetarDetails() {
            // Initialize if undefined
            if (this.metarDetailsVisible === undefined) {
                this.metarDetailsVisible = false;
            }
            
            // Toggle the state
            this.metarDetailsVisible = !this.metarDetailsVisible;
            
            // Close other popups
            this.tafDetailsVisible = false;
            this.notamDetailsVisible = false;
            
            // Position the popup correctly if it's being opened
            if (this.metarDetailsVisible) {
                setTimeout(() => {
                    const metarElement = document.querySelector('[data-metar-button]');
                    const metarPopup = document.querySelector('[data-metar-popup]');
                    
                    if (metarElement && metarPopup) {
                        const rect = metarElement.getBoundingClientRect();
                        metarPopup.style.left = `${rect.left + (rect.width / 2)}px`;
                        metarPopup.style.bottom = `${window.innerHeight - rect.top + 8}px`;
                        metarPopup.style.transform = 'translateX(-50%)';
                        metarPopup.style.transition = 'none';
                    }
                }, 0);
            }
        },
        
        // Toggle TAF details visibility
        toggleTAFDetails() {
            // Initialize if undefined
            if (this.tafDetailsVisible === undefined) {
                this.tafDetailsVisible = false;
            }
            
            // Toggle the state
            this.tafDetailsVisible = !this.tafDetailsVisible;
            
            // Close other popups
            this.metarDetailsVisible = false;
            this.notamDetailsVisible = false;
            
            // Position the popup correctly if it's being opened
            if (this.tafDetailsVisible) {
                setTimeout(() => {
                    const tafElement = document.querySelector('[data-taf-button]');
                    const tafPopup = document.querySelector('[data-taf-popup]');
                    
                    if (tafElement && tafPopup) {
                        const rect = tafElement.getBoundingClientRect();
                        tafPopup.style.left = `${rect.left + (rect.width / 2)}px`;
                        tafPopup.style.bottom = `${window.innerHeight - rect.top + 8}px`;
                        tafPopup.style.transform = 'translateX(-50%)';
                        tafPopup.style.transition = 'none';
                    }
                }, 0);
            }
        },
        
        // Toggle NOTAM details visibility
        toggleNOTAMDetails() {
            // Initialize if undefined
            if (this.notamDetailsVisible === undefined) {
                this.notamDetailsVisible = false;
            }
            
            // Toggle the state
            this.notamDetailsVisible = !this.notamDetailsVisible;
            
            // Close other popups
            this.metarDetailsVisible = false;
            this.tafDetailsVisible = false;
            
            // Position the popup correctly if it's being opened
            if (this.notamDetailsVisible) {
                setTimeout(() => {
                    const notamElement = document.querySelector('[data-notam-button]');
                    const notamPopup = document.querySelector('[data-notam-popup]');
                    
                    if (notamElement && notamPopup) {
                        const rect = notamElement.getBoundingClientRect();
                        notamPopup.style.left = `${rect.left + (rect.width / 2)}px`;
                        notamPopup.style.bottom = `${window.innerHeight - rect.top + 8}px`;
                        notamPopup.style.transform = 'translateX(-50%)';
                        notamPopup.style.transition = 'none';
                    }
                }, 0);
            }
        },
        
        // Get the TAF data
        getTAF() {
            return this.taf;
        },
        
        // Get the NOTAM data
        getNOTAMs() {
            return this.notams;
        },
        
        // Get the count of NOTAMs
        getNOTAMCount() {
            if (!this.notams || !Array.isArray(this.notams)) {
                return 0;
            }
            return this.notams.length;
        },
        
        // Get the count of TAF decoded items
        getTAFCount() {
            if (!this.taf || !this.taf.decoded || !Array.isArray(this.taf.decoded)) {
                return 0;
            }
            return this.taf.decoded.length;
        },

        async fetchAudioFrequencies() {
            try {
                const response = await fetch(this.audioApiUrl);
                if (!response.ok) {
                    console.error(`HTTP error fetching frequencies! Status: ${response.status}`);
                    this.audioFrequencies = []; 
                    return;
                }
                const data = await response.json();
                if (data && data.frequencies) {
                    this.audioFrequencies = data.frequencies;
                    // DO NOT connect to all frequencies immediately here.
                    // Let the user click "Start Radios"
                    // this.connectToAllFrequencies(); 
                    // Instead, just prepare them (create elements, setup viz graph)
                    this.prepareAllFrequencies(); 
                } else {
                    this.audioFrequencies = [];
                }
            } catch (error) {
                console.error('Error fetching audio frequencies:', error);
                this.audioFrequencies = [];
            }
        },

        // Renamed from connectToAllFrequencies to reflect its new role
        prepareAllFrequencies() {
            // REMOVED: this.initAudioContext(); // Ensure audio context is ready - This was causing the error.
            // The audioClient.prepareFrequency (called below) handles context initialization.
            this.audioFrequencies.forEach(freq => {
                this.prepareFrequency(freq); // This will now also setup visualization graph
            });
        },

        prepareFrequency(frequency) {
            if (!audioClient) { console.warn("audioClient not ready in prepareFrequency"); return; }
            audioClient.prepareFrequency(frequency);
        },

        connectToFrequency(frequency) {
            if (!audioClient) { console.warn("audioClient not ready in connectToFrequency"); return; }
            audioClient.connectToFrequency(frequency);
        },

        startAllRadios() {
            if (!audioClient) { console.warn("audioClient not ready in startAllRadios"); return; }
            audioClient.startAllRadios();
        },

        toggleMute(frequency) {
            if (!audioClient) { console.warn("audioClient not ready in toggleMute"); return; }
            audioClient.toggleMute(frequency);
        },

        cleanupFrequency(frequencyId) {
            if (!audioClient) { console.warn("audioClient not ready in cleanupFrequency"); return; }
            audioClient.cleanupFrequency(frequencyId);
        },
        
        // Play welcome sound
        playWelcomeSound() {
            if (this.splashScreenAudioPlayed) return;
            
            try {
                const audio = new Audio('/sounds/airplane-ding-dong.mp3');
                audio.volume = 0.7; // Set volume to 70%
                audio.play().then(() => {
                    console.log('[Alpine Store] Welcome sound played successfully');
                    this.splashScreenAudioPlayed = true;
                }).catch(err => {
                    console.error('[Alpine Store] Error playing welcome sound:', err);
                });
            } catch (err) {
                console.error('[Alpine Store] Error creating audio element:', err);
            }
        },
        
        // Close splash screen and play sound
        closeSplashScreen() {
            // Play the welcome sound when user clicks the button
            this.playWelcomeSound();
            
            // Hide the splash screen
            this.showSplashScreen = false;
            
            // Now that the splash screen is closed, we can start showing connection lost messages if needed
            // This ensures the connection lost overlay never appears during initial loading
            this.initialDataLoaded = true;
            
            console.log('[Alpine Store] Splash screen closed');
        },
        
        // Setup keyboard event listeners
        setupKeyboardEvents() {
            document.addEventListener('keydown', (e) => {
                // Skip if user is typing in an input field
                const isInInputField = e.target.tagName === 'INPUT' || 
                                     e.target.tagName === 'TEXTAREA' || 
                                     e.target.contentEditable === 'true';
                
                // ESC key to close aircraft details
                if (e.key === 'Escape' && this.selectedAircraft) {
                    this.selectedAircraft = null;
                    this.aircraftDetailsShowHistoryView = false;
                    this.showProximityView = false;
                    this.aircraftDetailsHistoryData = [];
                    this.aircraftDetailsHistoryCount = 0;
                    this.phaseHistoryData = [];
                    this.phaseHistoryAircraftHex = null;
                    this.aircraftDetailsStopHistoryRefresh();
                    this.stopProximityRefresh();
                    this.stopPhaseHistoryRefresh();
                    this.clearProximityView();
                }
                
                // TAB key for aircraft navigation (only when not in input fields)
                if (e.key === 'Tab' && !isInInputField) {
                    e.preventDefault();
                    
                    if (e.shiftKey) {
                        this.cycleToPreviousAircraft();
                    } else {
                        this.cycleToNextAircraft();
                    }
                }
                
                // Skip other hotkeys if user is typing in an input field
                if (isInInputField) return;
                
                // Air/Ground filter hotkeys
                if (e.key.toLowerCase() === 'a') {
                    e.preventDefault();
                    this.toggleAirAircraft();
                    console.log('[Hotkey] Toggled Air filter:', this.settings.showAirAircraft);
                }
                
                if (e.key.toLowerCase() === 'g') {
                    e.preventDefault();
                    this.toggleGroundAircraft();
                    console.log('[Hotkey] Toggled Ground filter:', this.settings.showGroundAircraft);
                }
                
                // Flight phase hotkeys (1-8) - matches UI filter bar order
                const phaseKeys = {
                    '1': 'NEW',   // New
                    '2': 'TAX',   // Taxi
                    '3': 'T/O',   // Takeoff
                    '4': 'DEP',   // Departure
                    '5': 'CRZ',   // Cruise
                    '6': 'ARR',   // Arrival
                    '7': 'APP',   // Approach
                    '8': 'T/D'    // Touchdown
                };
                
                if (phaseKeys[e.key]) {
                    e.preventDefault();
                    const phase = phaseKeys[e.key];
                    this.togglePhaseFilter(phase);
                    console.log(`[Hotkey] Toggled ${phase} phase filter:`, this.settings.phaseFilters[phase]);
                }
            });
        },

        // Play connection lost sound
        playConnectionLostSound() {
            if (this.connectionLostSoundPlayed) return;
            
            try {
                // const audio = new Audio('/sounds/airbus_retard.mp3');
                // audio.volume = 0.8; // Set volume to 80%
                // audio.play().then(() => {
                //     console.log('[Alpine Store] Connection lost sound played successfully');
                //     this.connectionLostSoundPlayed = true;
                // }).catch(err => {
                //     console.error('[Alpine Store] Error playing connection lost sound:', err);
                // });
                if (audioClient) {
                    audioClient.playRetardSound();
                    this.connectionLostSoundPlayed = true; // Assume it plays successfully
                } else {
                    console.error('[Alpine Store] audioClient not available to play connection lost sound.');
                }
            } catch (err) {
                console.error('[Alpine Store] Error initiating connection lost sound:', err);
            }
        },

        // Get heading with fallback priority: mag_heading -> track -> true_heading
        getHeadingWithFallback(aircraft) {
            if (!aircraft.adsb) return 0;
            
            // Priority order: mag_heading -> track -> true_heading
            if (aircraft.adsb.mag_heading !== undefined && aircraft.adsb.mag_heading !== null && aircraft.adsb.mag_heading !== 0) {
                return aircraft.adsb.mag_heading;
            }
            if (aircraft.adsb.track !== undefined && aircraft.adsb.track !== null && aircraft.adsb.track !== 0) {
                return aircraft.adsb.track;
            }
            if (aircraft.adsb.true_heading !== undefined && aircraft.adsb.true_heading !== null && aircraft.adsb.true_heading !== 0) {
                return aircraft.adsb.true_heading;
            }
            return 0; // Default fallback
        },

        getHeadingWithType(aircraft) {
            if (!aircraft.adsb) return { value: 0, type: null };
            
            // Priority order: mag_heading -> track -> true_heading
            if (aircraft.adsb.mag_heading !== undefined && aircraft.adsb.mag_heading !== null && aircraft.adsb.mag_heading !== 0) {
                return { value: aircraft.adsb.mag_heading, type: 'magnetic' };
            }
            if (aircraft.adsb.track !== undefined && aircraft.adsb.track !== null && aircraft.adsb.track !== 0) {
                return { value: aircraft.adsb.track, type: 'track' };
            }
            if (aircraft.adsb.true_heading !== undefined && aircraft.adsb.true_heading !== null && aircraft.adsb.true_heading !== 0) {
                return { value: aircraft.adsb.true_heading, type: 'true' };
            }
            return { value: 0, type: null }; // Default fallback - no suffix when all are zeros
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
                    return 'hdg'; // No suffix when all are zeros
            }
        },

        // WebSocket debugging state
        wsUpdatesPaused: false,
        // Removed mapUpdatesDisabled and dataOnlyMode debug parameters

        // Toggle WebSocket updates for troubleshooting
        toggleWebSocketUpdates() {
            this.wsUpdatesPaused = !this.wsUpdatesPaused;
            console.log(`WebSocket updates ${this.wsUpdatesPaused ? 'PAUSED' : 'RESUMED'}`);
            
            if (this.wsUpdatesPaused) {
                // Clear any pending updates when pausing
                this.cleanupThrottling();
            }
        },

        // Removed toggleMapUpdates() and toggleDataOnlyMode() debug methods

        // Clean up throttling mechanisms to prevent memory leaks
        cleanupThrottling() {
            if (this.mapUpdateThrottleId) {
                clearTimeout(this.mapUpdateThrottleId);
                this.mapUpdateThrottleId = null;
            }
            this.pendingMapUpdates.clear();
            this.cacheInvalidationPending = false;
        }
    });

    // Initialize audioClient AFTER the store is defined
    audioClient = new AudioClient(Alpine.store('atc'));
    
    // Initialize MapManager AFTER store and L/CONFIG are available
    // L (Leaflet) and CONFIG are globally available here
    mapManager = new MapManager(Alpine.store('atc'), L, CONFIG);
    Alpine.store('atc').mapManager = mapManager; // Make mapManager accessible in the store

    // Initialize Aircraft Animation Engine
    animationEngine = new AircraftAnimationEngine(mapManager, Alpine.store('atc'));
    Alpine.store('atc').animationEngine = animationEngine; // Make animation engine accessible in the store
    animationEngine.initialize();

    // Now initialize the store's own logic
    Alpine.store('atc').init();

    // Watch for aircraft selection changes to clear pending requests
    Alpine.effect(() => {
        const selectedAircraft = Alpine.store('atc').selectedAircraft;
        const previousSelectedHex = Alpine.store('atc')._previousSelectedHex;
        
        // If aircraft selection changed, clear pending requests for the previous aircraft
        if (previousSelectedHex && previousSelectedHex !== selectedAircraft?.hex) {
            Alpine.store('atc').clearPendingRequestsForAircraft(previousSelectedHex);
        }
        
        // Store current selection for next comparison
        Alpine.store('atc')._previousSelectedHex = selectedAircraft?.hex;
        
        // Refresh map visibility when selected aircraft changes to show/hide aircraft based on filters
        if (Alpine.store('atc').mapManager) {
            Alpine.store('atc').mapManager.applyFiltersAndRefreshView();
        }
    });
});
