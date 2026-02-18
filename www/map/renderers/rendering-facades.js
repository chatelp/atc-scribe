/**
 * Module: map/renderers/rendering-facades
 * Why it exists:
 * - Consolidates small renderer facade entry points into one script to reduce file churn
 *   and script-tag overhead while preserving existing global contracts.
 *
 * Key responsibilities:
 * - Expose `window.MapAircraftRenderer.updateAircraft`.
 * - Expose `window.MapTrailsRenderer.refreshTrails`.
 * - Expose `window.MapReferenceRenderer.refreshReferenceLayers`.
 *
 * Quirks / contracts:
 * - This module intentionally delegates all work to `OpenLayersMapManager` methods.
 * - Public global names are preserved to avoid breaking existing call sites.
 */
(function () {
    function updateAircraft(manager, hex, aircraft) {
        manager.updateSingleAircraft(hex, aircraft);
    }

    function refreshTrails(manager) {
        manager.updateFlightPaths();
    }

    function refreshReferenceLayers(manager) {
        if (manager.store.settings.showAirports) manager.renderAirports();
        if (manager.store.settings.showHeliports) manager.renderHeliports();
        if (manager.store.settings.showNavaids) manager.renderNavaids();
        if (manager.store.settings.showAllRunways) manager.renderAllRunways();
        manager.renderRunways();
    }

    window.MapAircraftRenderer = {
        updateAircraft,
    };

    window.MapTrailsRenderer = {
        refreshTrails,
    };

    window.MapReferenceRenderer = {
        refreshReferenceLayers,
    };
})();
