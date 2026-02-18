/**
 * Module: map/features/interactions-feature
 * Why it exists:
 * - Binds primary user interactions to map state transitions.
 * - Keeps pointer/click logic separate from rendering concerns.
 *
 * Key responsibilities:
 * - Handle aircraft selection, deselection, and hover state updates.
 * - Route clicks to reference-feature handlers when appropriate.
 * - Support station-override map click mode for quick station repositioning.
 *
 * Quirks / contracts:
 * - Pointer hit-testing is throttled to reduce expensive per-frame feature queries.
 * - Hit-test errors are warned once and then suppressed per session to avoid log spam.
 */
(function () {
    function isMapClickTarget(target) {
        if (!target || !target.classList) return false;
         return target.classList.contains('ol-viewport') ||
             target.classList.contains('ol-layer') ||
             target.classList.contains('ol-overlaycontainer-stopevent') ||
             target.classList.contains('ol-unselectable') ||
               target.tagName === 'CANVAS';
    }

    function registerOpenLayersInteractions(manager) {
        if (!manager || !manager.engine) return () => {};
        console.info('[MapInteractionsFeature] Registering OpenLayers core interactions');

        let previousHoveredHex = null;
        let warnedHitTestFailure = false;
        let lastPointerHitTestAt = 0;
        const pointerHitTestIntervalMs = 120;
        const rawMapRef = manager.engine && manager.engine.getMap ? manager.engine.getMap() : null;
        const mapTargetElement = rawMapRef && rawMapRef.getTargetElement ? rawMapRef.getTargetElement() : null;

        function tryGetFeatureHexAtPixel(rawMap, pixel, options) {
            if (!rawMap || typeof rawMap.forEachFeatureAtPixel !== 'function') return null;
            let hex = null;
            try {
                rawMap.forEachFeatureAtPixel(pixel, (feature) => {
                    hex = feature && feature.get ? feature.get('hex') : null;
                    return !!hex;
                }, options || undefined);
            } catch (error) {
                if (!warnedHitTestFailure) {
                    warnedHitTestFailure = true;
                    console.warn('[MapInteractionsFeature] forEachFeatureAtPixel failed; suppressing this interaction frame:', error);
                }
                return null;
            }
            return hex;
        }

        function hasReferenceFeatureAtPixel(rawMap, pixel) {
            if (!rawMap || typeof rawMap.forEachFeatureAtPixel !== 'function') return false;
            let found = false;
            try {
                rawMap.forEachFeatureAtPixel(pixel, (feature) => {
                    const refType = feature && feature.get ? feature.get('refType') : null;
                    if (refType) {
                        found = true;
                        return true;
                    }
                    return false;
                }, { hitTolerance: 6 });
            } catch (error) {
                if (!warnedHitTestFailure) {
                    warnedHitTestFailure = true;
                    console.warn('[MapInteractionsFeature] reference hit-test failed; suppressing this interaction frame:', error);
                }
                return false;
            }
            return found;
        }

        const onSingleClick = (event) => {
            const store = manager.store;
            if (!store) return;

            const rawMap = manager.engine && manager.engine.getMap ? manager.engine.getMap() : null;
            const aircraftLayer = typeof manager._getAircraftHitLayer === 'function'
                ? manager._getAircraftHitLayer()
                : null;
            const clickedHex = tryGetFeatureHexAtPixel(rawMap, event.pixel, aircraftLayer ? {
                layerFilter: (candidateLayer) => candidateLayer === aircraftLayer,
                hitTolerance: 4,
            } : { hitTolerance: 4 });

            if (clickedHex && store.aircraft && store.aircraft[clickedHex]) {
                store.selectedAircraft = store.aircraft[clickedHex];
                if (typeof manager.updateVisualState === 'function') {
                    manager.updateVisualState(clickedHex, true);
                }
                return;
            }

            if (typeof manager.handleReferenceFeatureClick === 'function') {
                const handled = manager.handleReferenceFeatureClick(event);
                if (handled) {
                    return;
                }
            }

            if (store.stationOverride?.mapClickMode) {
                const lonLat = window.ol.proj.toLonLat(event.coordinate);
                store.stationOverride.latitude = lonLat[1];
                store.stationOverride.longitude = lonLat[0];
                store.stationOverride.mapClickMode = false;
                if (typeof store.hideMapClickIndicator === 'function') {
                    store.hideMapClickIndicator();
                }
                return;
            }

            if (store.selectedAircraft) {
                store.selectedAircraft = null;
                if (typeof manager.removeProximityCircle === 'function') {
                    manager.removeProximityCircle();
                }
                if (typeof manager.removeProximityHighlighting === 'function') {
                    manager.removeProximityHighlighting();
                }
                if (typeof manager.applyFiltersAndRefreshView === 'function') {
                    manager.applyFiltersAndRefreshView({ immediate: true });
                }
            }
        };

        const onDoubleClick = () => {
            const store = manager.store;
            if (!store) return;
            if (store.searchTerm && store.searchTerm.length > 0) {
                store.searchTerm = '';
                if (typeof manager.applyFiltersAndRefreshView === 'function') {
                    manager.applyFiltersAndRefreshView({ immediate: true });
                }
            }
        };

        const onPointerMove = (event) => {
            const store = manager.store;
            if (!store) return;
            if (event && event.dragging) return;

            const now = performance.now();
            if ((now - lastPointerHitTestAt) < pointerHitTestIntervalMs) {
                return;
            }
            lastPointerHitTestAt = now;

            const rawMap = manager.engine && manager.engine.getMap ? manager.engine.getMap() : null;
            const aircraftLayer = typeof manager._getAircraftHitLayer === 'function'
                ? manager._getAircraftHitLayer()
                : null;
            const hoveredHex = tryGetFeatureHexAtPixel(rawMap, event.pixel, aircraftLayer ? {
                layerFilter: (candidateLayer) => candidateLayer === aircraftLayer,
                hitTolerance: 4,
            } : { hitTolerance: 4 });
            const hasReferenceHover = hasReferenceFeatureAtPixel(rawMap, event.pixel);

            if (mapTargetElement && mapTargetElement.style) {
                mapTargetElement.style.cursor = (hoveredHex || hasReferenceHover) ? 'pointer' : '';
            }

            if (previousHoveredHex && previousHoveredHex !== hoveredHex) {
                if (typeof manager.updateVisualState === 'function') {
                    manager.updateVisualState(previousHoveredHex, true);
                }
            }

            if (hoveredHex && store.aircraft && store.aircraft[hoveredHex]) {
                store.hoveredAircraft = store.aircraft[hoveredHex];
                if (typeof manager.updateVisualState === 'function') {
                    manager.updateVisualState(hoveredHex, true);
                }
                previousHoveredHex = hoveredHex;
                return;
            }

            store.hoveredAircraft = null;
            previousHoveredHex = null;
        };

        manager.engine.on('singleclick', onSingleClick);
        manager.engine.on('dblclick', onDoubleClick);
        manager.engine.on('pointermove', onPointerMove);

        let targetElement = null;
        const onDomDoubleClick = (event) => {
            event.preventDefault();
            onDoubleClick();
        };

        const rawMap = manager.engine.getMap ? manager.engine.getMap() : null;
        if (rawMap && rawMap.getTargetElement) {
            targetElement = rawMap.getTargetElement();
            if (targetElement) {
                targetElement.addEventListener('dblclick', onDomDoubleClick);
            }
        }

        return () => {
            manager.engine.off('singleclick');
            manager.engine.off('dblclick');
            manager.engine.off('pointermove');
            if (mapTargetElement && mapTargetElement.style) {
                mapTargetElement.style.cursor = '';
            }
            if (targetElement) {
                targetElement.removeEventListener('dblclick', onDomDoubleClick);
            }
            console.info('[MapInteractionsFeature] OpenLayers interactions cleaned up');
        };
    }

    window.MapInteractionsFeature = {
        isMapClickTarget,
        registerOpenLayersInteractions,
    };
})();
