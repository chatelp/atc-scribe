/**
 * Module: map/core/map-engine
 * Why it exists:
 * - Owns raw OpenLayers map creation and map-level primitives used by higher layers.
 * - Centralizes basemap style mapping so UI style toggles stay deterministic.
 *
 * Key responsibilities:
 * - Initialize `window.ol.Map` and `window.ol.View` for the primary map target.
 * - Create and switch basemap sources (dark/light/osm/VFR/IFR variants).
 * - Manage map listeners and expose engine-level utility methods.
 *
 * Quirks / contracts:
 * - Includes URL fallback transform support for chart tiles with endpoint naming
 *   mismatches (notably terminal chart variants).
 * - Keeps `wrapX` disabled for chart tiles to avoid mirrored-world artifacts.
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
        function createArcGisXyzSource(options) {
            const hasFallbackTransform = typeof options.fallbackUrlTransform === 'function';
            return new window.ol.source.XYZ({
                url: options.url,
                attributions: options.attributions,
                minZoom: options.minZoom,
                maxZoom: options.maxZoom,
                crossOrigin: 'anonymous',
                wrapX: false,
                tileLoadFunction: hasFallbackTransform
                    ? (imageTile, src) => {
                        const image = imageTile.getImage();
                        let fallbackTried = false;
                        image.crossOrigin = 'anonymous';
                        image.onerror = () => {
                            if (!fallbackTried) {
                                fallbackTried = true;
                                const fallbackSrc = options.fallbackUrlTransform(src);
                                if (fallbackSrc && fallbackSrc !== src) {
                                    image.src = fallbackSrc;
                                    return;
                                }
                            }
                            image.onerror = null;
                        };
                        image.src = src;
                    }
                    : undefined,
            });
        }

        function normalizeBaseMapStyle(styleId) {
            if (!styleId || typeof styleId !== 'string') return 'dark';
            const value = styleId.trim().toLowerCase();
            if (value === 'light' || value === 'osm' || value === 'dark' || value === 'vfr-sectional' || value === 'terminal' || value === 'ifr-low' || value === 'ifr-high') return value;
            return 'dark';
        }

        function createBaseMapSource(styleId) {
            const normalized = normalizeBaseMapStyle(styleId);
            if (normalized === 'vfr-sectional') {
                return createArcGisXyzSource({
                    url: 'https://tiles.arcgis.com/tiles/ssFJjBXIUyZDrSYZ/arcgis/rest/services/VFR_Sectional/MapServer/tile/{z}/{y}/{x}',
                    attributions: 'Tiles courtesy of <a href="http://tiles.arcgis.com/">arcgis.com</a>',
                    minZoom: 8,
                    maxZoom: 12,
                });
            }

            if (normalized === 'terminal') {
                return createArcGisXyzSource({
                    url: 'https://tiles.arcgis.com/tiles/ssFJjBXIUyZDrSYZ/arcgis/rest/services/VFR_Terminal/MapServer/tile/{z}/{y}/{x}',
                    attributions: 'Tiles courtesy of <a href="http://tiles.arcgis.com/">arcgis.com</a>',
                    minZoom: 10,
                    maxZoom: 12,
                    fallbackUrlTransform: (src) => {
                        if (typeof src !== 'string') return src;
                        return src.replace('/VFR_Terminal/', '/VFR_Terminals/');
                    },
                });
            }

            if (normalized === 'ifr-low') {
                return createArcGisXyzSource({
                    url: 'https://tiles.arcgis.com/tiles/ssFJjBXIUyZDrSYZ/arcgis/rest/services/IFR_AreaLow/MapServer/tile/{z}/{y}/{x}',
                    attributions: 'Tiles courtesy of <a href="http://tiles.arcgis.com/">arcgis.com</a>',
                    minZoom: 8,
                    maxZoom: 11,
                });
            }

            if (normalized === 'ifr-high') {
                return createArcGisXyzSource({
                    url: 'https://tiles.arcgis.com/tiles/ssFJjBXIUyZDrSYZ/arcgis/rest/services/IFR_High/MapServer/tile/{z}/{y}/{x}',
                    attributions: 'Tiles courtesy of <a href="http://tiles.arcgis.com/">arcgis.com</a>',
                    minZoom: 7,
                    maxZoom: 11,
                });
            }

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
