/**
 * Module: map/core/visibility-rules
 * Why it exists:
 * - Consolidates aircraft visibility predicates used by map refresh/update paths.
 * - Ensures filtering semantics are identical across full passes and incremental updates.
 *
 * Key responsibilities:
 * - Evaluate search, ground/air state, altitude, phase, and recency filters.
 * - Preserve selected-aircraft visibility even when it no longer matches filters.
 *
 * Quirks / contracts:
 * - Phase defaults to `NEW` when unavailable.
 * - Callers can bypass viewport gating via `options.isInViewport = false`.
 */
(function () {
    function getCurrentPhase(aircraft) {
        return (aircraft && aircraft.phase && aircraft.phase.current && aircraft.phase.current.length > 0)
            ? aircraft.phase.current[0].phase
            : 'NEW';
    }

    function matchesSearch(aircraft, searchTerm) {
        const searchLower = (searchTerm || '').toLowerCase();
        if (searchLower === '') return true;

        const callsign = (aircraft.flight || aircraft.hex || '').toLowerCase();
        const type = (aircraft.adsb?.type || '').toLowerCase();
        const category = (aircraft.adsb?.category || '').toLowerCase();

        return callsign.includes(searchLower) || type.includes(searchLower) || category.includes(searchLower);
    }

    function isVisibleByGroundState(aircraft, settings) {
        return (aircraft.on_ground && settings.showGroundAircraft) ||
               (!aircraft.on_ground && settings.showAirAircraft);
    }

    function isVisibleByAltitude(aircraft, settings) {
        return aircraft.on_ground ||
            (aircraft.adsb && aircraft.adsb.alt_baro >= settings.minAltitude && aircraft.adsb.alt_baro <= settings.maxAltitude);
    }

    function isVisibleByPhase(aircraft, settings) {
        const currentPhase = getCurrentPhase(aircraft);
        return !(settings.phaseFilters && settings.phaseFilters[currentPhase] === false);
    }

    function isVisibleByLastSeen(aircraft, lastSeenCutoff) {
        if (!aircraft.last_seen) return true;
        const lastSeenDate = new Date(aircraft.last_seen);
        return lastSeenDate >= lastSeenCutoff;
    }

    function shouldShowAircraftOnMap(aircraft, store, options) {
        const matches = matchesSearch(aircraft, options.searchTerm);
        const groundVisible = isVisibleByGroundState(aircraft, store.settings);
        const altitudeVisible = isVisibleByAltitude(aircraft, store.settings);
        const phaseVisible = isVisibleByPhase(aircraft, store.settings);
        const lastSeenVisible = options.lastSeenCutoff ? isVisibleByLastSeen(aircraft, options.lastSeenCutoff) : true;
        const selected = !!(store.selectedAircraft && store.selectedAircraft.hex === aircraft.hex);
        const inViewport = options.isInViewport !== false;

        return (matches && groundVisible && altitudeVisible && phaseVisible && lastSeenVisible && inViewport) || selected;
    }

    window.MapVisibilityRules = {
        getCurrentPhase,
        matchesSearch,
        isVisibleByGroundState,
        isVisibleByAltitude,
        isVisibleByPhase,
        isVisibleByLastSeen,
        shouldShowAircraftOnMap,
    };
})();
