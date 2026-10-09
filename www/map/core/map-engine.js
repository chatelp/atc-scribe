/**
 * Module: map/core/map-engine
 * Why it exists:
 * - Owns raw OpenLayers map creation and map-level primitives used by higher layers.
 * - Centralizes basemap style mapping so UI style toggles stay deterministic.
 *
 * Key responsibilities:
 * - Initialize `window.ol.Map` and `window.ol.View` for the primary map target.
 * - Create and switch basemap sources (Plan IGN v2 and OSM, each light or darkened).
 * - Manage map listeners and expose engine-level utility methods.
 *
 * Quirks / contracts:
 * - An unknown style id (such as a removed FAA or Carto map) resolves to osm-dark.
 */
(function () {
    function createOpenLayersEngine(options) {
        const targetId = options?.targetId || 'map';
        const initialCenter = options?.center || { lat: 43.6777, lon: -79.6248 };
        const initialZoom = Number.isFinite(options?.zoom) ? options.zoom : 10;
        let activeBaseMapStyle = typeof options?.baseMapStyle === 'string' ? options.baseMapStyle : 'osm-dark';

        let map = null;
        let baseLayer = null;
        const listenerKeys = new Map();
        // ign and osm are light tiles; their -dark variants are the same tiles
        // with a CSS filter (style.css, .basemap-dark) on the base layer's own
        // canvas, which the aircraft, trails and other layers do not share.
        function normalizeBaseMapStyle(styleId) {
            if (!styleId || typeof styleId !== 'string') return 'osm-dark';
            const value = styleId.trim().toLowerCase();
            if (value === 'ign' || value === 'ign-dark' || value === 'osm' || value === 'osm-dark' || value === 'none') return value;
            return 'osm-dark';
        }

        function isDarkBaseMapStyle(styleId) {
            return normalizeBaseMapStyle(styleId).endsWith('-dark');
        }

        // The Géoplateforme answers about one tile in twenty with a 400 "layer
        // unknown" that a later request does not get (measured 10/10: 4 of 60
        // at z13, 4 of 80 on one tile, in runs of one or two requests). A failed
        // tile stayed empty, a black square under the dark filter. Refusals come
        // in spells that can hold one tile for several seconds (one z15 tile was
        // refused five times over 8 s while its neighbours loaded; 25 parallel
        // requests from curl were all served, so it is not a limit on
        // concurrency). Tiles are fetched with up to seven tries, spaced 0.5 s
        // to 16 s with some jitter, about 30 s in all. Both servers
        // allow it (Access-Control-Allow-Origin: *), and the browser still
        // caches by their headers.
        function loadTileWithRetry(tile, src) {
            const image = tile.getImage();
            const attempt = (n) => {
                fetch(src)
                    .then((response) => {
                        if (!response.ok) throw new Error(String(response.status));
                        return response.blob();
                    })
                    .then((blob) => {
                        const url = URL.createObjectURL(blob);
                        image.addEventListener('load', () => URL.revokeObjectURL(url), { once: true });
                        image.src = url;
                    })
                    .catch(() => {
                        if (n < 7) setTimeout(() => attempt(n + 1), 500 * 2 ** (n - 1) + Math.random() * 300);
                        else image.src = src; // let OpenLayers see the failure
                    });
            };
            attempt(1);
        }

        function createBaseMapSource(styleId) {
            const normalized = normalizeBaseMapStyle(styleId);
            if (normalized === 'ign' || normalized === 'ign-dark') {
                // Plan IGN v2 from the Géoplateforme: Licence Ouverte, no key, France only.
                return new window.ol.source.XYZ({
                    url: 'https://data.geopf.fr/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0'
                        + '&LAYER=GEOGRAPHICALGRIDSYSTEMS.PLANIGNV2&STYLE=normal&FORMAT=image/png'
                        + '&TILEMATRIXSET=PM&TILEMATRIX={z}&TILEROW={y}&TILECOL={x}',
                    attributions: '&copy; IGN &ndash; Plan IGN v2',
                    maxZoom: 19,
                    tileLoadFunction: loadTileWithRetry,
                });
            }

            return new window.ol.source.OSM({
                attributions: '&copy; OpenStreetMap contributors',
                tileLoadFunction: loadTileWithRetry,
            });
        }

        // A base layer for this style. Its class gives it its own canvas, so that
        // the dark filter applies to it alone.
        // 'none' hides the base layer: aircraft and the airport layout on black.
        function createBaseMapLayer(styleId) {
            return new window.ol.layer.Tile({
                className: 'ol-layer atc-basemap',
                source: createBaseMapSource(styleId),
                visible: normalizeBaseMapStyle(styleId) !== 'none',
            });
        }

        // Darkening is a class on the map's element, so it follows the style
        // without the layer being rebuilt.
        function applyBaseMapTheme(targetElement, styleId) {
            if (targetElement && targetElement.classList) {
                targetElement.classList.toggle('basemap-dark', isDarkBaseMapStyle(styleId));
            }
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
                        baseLayer = createBaseMapLayer(normalizedStyle);
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
            applyBaseMapTheme(map.getTargetElement(), activeBaseMapStyle);

            return map;
        }

        function setBaseMapStyle(styleId) {
            const normalizedStyle = normalizeBaseMapStyle(styleId);
            activeBaseMapStyle = normalizedStyle;

            if (!baseLayer) return;

            baseLayer.setVisible(normalizedStyle !== 'none');
            if (normalizedStyle === 'none') return;
            const source = createBaseMapSource(normalizedStyle);
            baseLayer.setSource(source);
            applyBaseMapTheme(map && map.getTargetElement(), normalizedStyle);
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
            createBaseMapLayer: () => createBaseMapLayer(activeBaseMapStyle),
            applyBaseMapTheme: (targetElement) => applyBaseMapTheme(targetElement, activeBaseMapStyle),
            on,
            off,
            dispose,
        };
    }

    window.MapEngine = {
        createOpenLayersEngine,
    };
})();
