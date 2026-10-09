/**
 * Module: map/core/map-engine
 * Why it exists:
 * - Owns raw OpenLayers map creation and map-level primitives used by higher layers.
 * - Centralizes basemap style mapping so UI style toggles stay deterministic.
 *
 * Key responsibilities:
 * - Initialize `window.ol.Map` and `window.ol.View` for the primary map target.
 * - Create and switch basemap sources (dark/light/osm).
 * - Manage map listeners and expose engine-level utility methods.
 *
 * Quirks / contracts:
 * - An unknown style id (such as a removed FAA chart) resolves to the dark map.
 */
(function () {
    function createOpenLayersEngine(options) {
        const targetId = options?.targetId || 'map';
        const initialCenter = options?.center || { lat: 43.6777, lon: -79.6248 };
        const initialZoom = Number.isFinite(options?.zoom) ? options.zoom : 10;
        let activeBaseMapStyle = typeof options?.baseMapStyle === 'string' ? options.baseMapStyle : 'dark';

        let map = null;
        let baseLayer = null;
        const listenerKeys = new Map();
        function normalizeBaseMapStyle(styleId) {
            if (!styleId || typeof styleId !== 'string') return 'dark';
            const value = styleId.trim().toLowerCase();
            if (value === 'light' || value === 'osm' || value === 'dark') return value;
            return 'dark';
        }

        function createBaseMapSource(styleId) {
            const normalized = normalizeBaseMapStyle(styleId);
            if (normalized === 'light') {
                return new window.ol.source.XYZ({
                    url: 'https://{a-d}.basemaps.cartocdn.com/light_all/{z}/{x}/{y}{r}.png',
                    attributions: '&copy; OpenStreetMap contributors &copy; CARTO',
                });
            }

            if (normalized === 'osm') {
                return new window.ol.source.OSM({
                    attributions: '&copy; OpenStreetMap contributors',
                });
            }

            return new window.ol.source.XYZ({
                url: 'https://{a-d}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png',
                attributions: '&copy; OpenStreetMap contributors &copy; CARTO',
            });
        }

        function ensureOL() {
            return !!(window.ol && window.ol.Map && window.ol.View);
        }

        function init() {
            if (!ensureOL()) {
                throw new Error('OpenLayers runtime is not available on window.ol');
            }

            const interactionDefaultsFactory =
                (window.ol.interaction && typeof window.ol.interaction.defaults === 'function')
                    ? window.ol.interaction.defaults
                    : (window.ol.interaction && window.ol.interaction.defaults && typeof window.ol.interaction.defaults.defaults === 'function')
                        ? window.ol.interaction.defaults.defaults
                        : null;

            const controlDefaultsFactory =
                (window.ol.control && typeof window.ol.control.defaults === 'function')
                    ? window.ol.control.defaults
                    : (window.ol.control && window.ol.control.defaults && typeof window.ol.control.defaults.defaults === 'function')
                        ? window.ol.control.defaults.defaults
                        : null;

            const view = new window.ol.View({
                center: window.ol.proj.fromLonLat([initialCenter.lon, initialCenter.lat]),
                zoom: initialZoom,
            });

            const mapOptions = {
                target: targetId,
                layers: [
                    (() => {
                        const normalizedStyle = normalizeBaseMapStyle(activeBaseMapStyle);
                        activeBaseMapStyle = normalizedStyle;
                        baseLayer = new window.ol.layer.Tile({
                            source: createBaseMapSource(normalizedStyle),
                        });
                        return baseLayer;
                    })(),
                ],
                view,
            };

            if (interactionDefaultsFactory) {
                mapOptions.interactions = interactionDefaultsFactory({ doubleClickZoom: false });
            }

            if (controlDefaultsFactory) {
                mapOptions.controls = controlDefaultsFactory({ attribution: false, zoom: false, rotate: false });
            }

            map = new window.ol.Map(mapOptions);

            return map;
        }

        function setBaseMapStyle(styleId) {
            const normalizedStyle = normalizeBaseMapStyle(styleId);
            activeBaseMapStyle = normalizedStyle;

            if (!baseLayer) return;

            const source = createBaseMapSource(normalizedStyle);
            baseLayer.setSource(source);
        }

        function getBaseMapStyle() {
            return activeBaseMapStyle;
        }

        function getMap() {
            return map;
        }

        function setView(lat, lon, zoom) {
            if (!map) return;
            map.getView().setCenter(window.ol.proj.fromLonLat([lon, lat]));
            if (Number.isFinite(zoom)) {
                map.getView().setZoom(zoom);
            }
        }

        function fitBounds(boundsOrPoints, options = {}) {
            if (!map) return;
            if (!Array.isArray(boundsOrPoints) || boundsOrPoints.length === 0) return;

            let extent;
            if (boundsOrPoints.length === 2 && Array.isArray(boundsOrPoints[0]) && Array.isArray(boundsOrPoints[1])) {
                const sw = window.ol.proj.fromLonLat([boundsOrPoints[0][1], boundsOrPoints[0][0]]);
                const ne = window.ol.proj.fromLonLat([boundsOrPoints[1][1], boundsOrPoints[1][0]]);
                extent = [sw[0], sw[1], ne[0], ne[1]];
            } else {
                const projected = boundsOrPoints
                    .filter((point) => Array.isArray(point) && point.length >= 2)
                    .map((point) => window.ol.proj.fromLonLat([point[1], point[0]]));
                if (projected.length === 0) return;
                extent = window.ol.extent.boundingExtent(projected);
            }

            map.getView().fit(extent, {
                padding: options.padding || [20, 20, 20, 20],
                duration: Number.isFinite(options.duration) ? options.duration : 0,
                maxZoom: Number.isFinite(options.maxZoom) ? options.maxZoom : 14,
            });
        }

        function getBounds() {
            if (!map) return null;
            const size = map.getSize();
            if (!size) return null;
            const extent = map.getView().calculateExtent(size);
            const sw = window.ol.proj.toLonLat([extent[0], extent[1]]);
            const ne = window.ol.proj.toLonLat([extent[2], extent[3]]);
            return {
                south: sw[1],
                west: sw[0],
                north: ne[1],
                east: ne[0],
            };
        }

        function getZoom() {
            if (!map) return 0;
            return map.getView().getZoom() || 0;
        }

        function on(eventName, handler) {
            if (!map) return;
            const key = map.on(eventName, handler);
            if (!listenerKeys.has(eventName)) {
                listenerKeys.set(eventName, new Set());
            }
            listenerKeys.get(eventName).add(key);
        }

        function off(eventName) {
            if (!map) return;
            const keys = listenerKeys.get(eventName);
            if (!keys) return;
            keys.forEach((key) => window.ol.Observable.unByKey(key));
            listenerKeys.delete(eventName);
        }

        function dispose() {
            if (!map) return;
            listenerKeys.forEach((keys) => {
                keys.forEach((key) => window.ol.Observable.unByKey(key));
            });
            listenerKeys.clear();
            map.setTarget(null);
            map = null;
        }

        return {
            init,
            getMap,
            setView,
            fitBounds,
            getBounds,
            getZoom,
            setBaseMapStyle,
            getBaseMapStyle,
            on,
            off,
            dispose,
        };
    }

    window.MapEngine = {
        createOpenLayersEngine,
    };
})();
