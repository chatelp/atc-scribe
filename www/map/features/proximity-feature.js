/**
 * Module: map/features/proximity-feature
 * Why it exists:
 * - Provides a narrow read API for proximity selection state.
 * - Avoids direct coupling to manager internals from UI/debug consumers.
 *
 * Key responsibilities:
 * - Return proximity reference target, configured distance, and active-state flag.
 *
 * Quirks / contracts:
 * - This is a read-only facade; mutation remains owned by the map manager.
 */
(function () {
    function getProximityState(manager) {
        return {
            refHex: manager.proximityRefHex,
            distanceNM: manager.proximityDistanceNM,
            hasSet: !!manager.proximityHexSet,
        };
    }

    window.MapProximityFeature = {
        getProximityState,
    };
})();
