/**
 * Module: map/openlayers-map-manager
 * Why it exists:
 * - Serves as the central orchestration layer for all OpenLayers-backed map behavior.
 * - Coordinates data-driven rendering, interaction wiring, and map-specific UI state.
 *
 * Key responsibilities:
 * - Own aircraft, trail, overlay, reference, and effect feature lifecycle.
 * - Integrate map engine, renderers, interactions, and feature modules.
 * - Apply filtering/visibility decisions and incremental visual updates.
 * - Manage special map workflows (proximity selection, mini-map, popup overlays).
 *
 * Quirks / contracts:
 * - This class intentionally holds broad cross-cutting state to minimize transient
 *   object churn in hot update paths.
 * - Uses defensive checks around optional feature modules to preserve graceful
 *   degradation when subsets are unavailable.
 */
(function () {
    class OpenLayersMapManager {
        constructor(store, CONFIG) {
            this.store = store;
            this.CONFIG = CONFIG;

            this.engine = null;
            this.map = null;
            this.layers = {};
            this.markers = {};
            this.trails = {};

            this.proximityCircle = null;
            this.proximityHexSet = null;
            this.proximityRefHex = null;
            this.proximityDistanceNM = null;

            this._warnings = new Set();
            this._stats = {
                initCalls: 0,
                refreshCalls: 0,
                singleAircraftUpdates: 0,
                singleAircraftUpdateDurationMs: 0,
            };

            this._olMap = null;
            this._aircraftRenderer = null;
            this._trailSource = null;
            this._trailLayer = null;
            this._trailFeaturesByHex = new Map();
            this._overlaySource = null;
            this._overlayLayer = null;
            this._positionHighlightFeature = null;
            this._proximityCircleFeature = null;
            this._effectFeatures = new Set();
            this._labelRefreshInterval = null;
            this._referenceLayerState = {};
            this._rangeRingLayerState = null;
            this._referenceRenderScheduled = false;
            this.runwayData = null;
            this.airportsData = null;
            this.heliportsData = null;
            this.navaidsData = null;
            this.allRunwaysData = null;
            this.tracksMiniMap = null;
            this.tracksMiniMapLayers = null;
            this._overlayRegistry = null;
            this._overlayStatuses = [];
            this._referencePopupOverlay = null;
            this._referencePopupElement = null;

            this._interactionCleanup = null;
            this._simulationPositionMode = false;
        }

        _toMapCoordinate(lat, lon) {
            return window.ol.proj.fromLonLat([lon, lat]);
        }

        _ensureTrailLayer() {
            if (this._trailLayer || !this._olMap) return;

            this._trailSource = new window.ol.source.Vector();
            this._trailLayer = new window.ol.layer.Vector({
                source: this._trailSource,
                updateWhileAnimating: true,
                updateWhileInteracting: true,
            });
            this._trailLayer.setZIndex(300);
            this._olMap.addLayer(this._trailLayer);
        }

        _ensureOverlayLayer() {
            if (this._overlayLayer || !this._olMap) return;

            this._overlaySource = new window.ol.source.Vector();
            this._overlayLayer = new window.ol.layer.Vector({
                source: this._overlaySource,
                updateWhileAnimating: true,
                updateWhileInteracting: true,
            });
            this._overlayLayer.setZIndex(500);
            this._olMap.addLayer(this._overlayLayer);
        }

        _ensureReferenceLayers() {
            if (!this._olMap) return;
            if (Object.keys(this._referenceLayerState).length > 0 && this._rangeRingLayerState) return;

            const createLayerState = (zIndex, defaultVisible = true) => {
                const source = new window.ol.source.Vector();
                const layer = new window.ol.layer.Vector({ source, updateWhileAnimating: false, updateWhileInteracting: false });
                layer.setZIndex(zIndex);
                layer.setVisible(defaultVisible);
                this._olMap.addLayer(layer);
                return { source, layer };
            };

            this._referenceLayerState = {
                runways: createLayerState(120, true),
                airports: createLayerState(125, this.store?.settings?.showAirports !== false),
                heliports: createLayerState(126, this.store?.settings?.showHeliports !== false),
                navaids: createLayerState(127, this.store?.settings?.showNavaids !== false),
                allRunways: createLayerState(115, this.store?.settings?.showAllRunways === true),
            };

            this._rangeRingLayerState = createLayerState(110, this.store?.settings?.showRings !== false);
        }

        _clearReferenceLayer(layerName) {
            const state = this._referenceLayerState[layerName];
            if (!state || !state.source) return;
            state.source.clear();
        }

        _isLayerVisible(layerName) {
            const state = this._referenceLayerState[layerName];
            if (!state || !state.layer) return false;
            return state.layer.getVisible();
        }

        _isRangeRingVisible() {
            if (!this._rangeRingLayerState || !this._rangeRingLayerState.layer) return false;
            return this._rangeRingLayerState.layer.getVisible();
        }

        _scheduleReferenceRender() {
            if (this._referenceRenderScheduled) return;
            this._referenceRenderScheduled = true;
            window.requestAnimationFrame(() => {
                this._referenceRenderScheduled = false;
                this._renderReferenceLayersForCurrentView();
            });
        }

        _getDefaultOverlayConfigs() {
            return [
                {
                    id: 'aviation-chart',
                    type: 'overlay',
                    sourceType: 'xyz',
                    url: this.CONFIG?.aviationChartOverlayUrl || '',
                    zIndex: 640,
                    defaultVisible: false,
                    minZoom: 6,
                    maxZoom: 18,
                    opacity: 0.65,
                    refreshPolicy: { type: 'none' },
                    attribution: this.CONFIG?.aviationChartOverlayAttribution || '',
                    licenseNotes: 'Disabled by default; configure approved source URL before use.',
                    failureIsolation: { disableOnError: true, maxErrors: 2, retryMaxAttempts: 3, retryBaseMs: 2000 },
                },
                {
                    id: 'nexrad-radar',
                    type: 'overlay',
                    sourceType: 'xyz',
                    url: this.CONFIG?.weatherRadarXyzUrl || 'https://mesonet{1-3}.agron.iastate.edu/cache/tile.py/1.0.0/nexrad-n0q-900913/{z}/{x}/{y}.png',
                    wmsParams: {},
                    zIndex: 645,
                    defaultVisible: false,
                    minZoom: 5,
                    maxZoom: 18,
                    opacity: 0.55,
                    refreshPolicy: { type: 'interval', intervalMs: 120000 },
                    attribution: this.CONFIG?.weatherRadarAttribution || 'NEXRAD courtesy of <a href="https://mesonet.agron.iastate.edu/">IEM</a>',
                    licenseNotes: 'Disabled by default; configure approved NEXRAD source before use.',
                    failureIsolation: { disableOnError: true, maxErrors: 2, retryMaxAttempts: 3, retryBaseMs: 5000 },
                },
                {
                    id: 'noaa-infrared',
                    type: 'overlay',
                    sourceType: 'image-wms',
                    url: this.CONFIG?.noaaInfraredWmsUrl || 'https://nowcoast.noaa.gov/geoserver/satellite/wms',
                    wmsParams: this.CONFIG?.noaaInfraredWmsParams || {
                        LAYERS: 'global_longwave_imagery_mosaic',
                        FORMAT: 'image/png',
                        TRANSPARENT: true,
                    },
                    zIndex: 646,
                    defaultVisible: false,
                    minZoom: 3,
                    maxZoom: 18,
                    opacity: 0.55,
                    refreshPolicy: { type: 'interval', intervalMs: 900000 },
                    attribution: this.CONFIG?.noaaInfraredAttribution || 'NOAA nowCOAST infrared',
                    licenseNotes: 'Disabled by default; configure approved NOAA infrared source before use.',
                    failureIsolation: { disableOnError: true, maxErrors: 2, retryMaxAttempts: 3, retryBaseMs: 5000 },
                },
                {
                    id: 'noaa-radar',
                    type: 'overlay',
                    sourceType: 'wms',
                    url: this.CONFIG?.noaaRadarWmsUrl || 'https://nowcoast.noaa.gov/geoserver/weather_radar/wms',
                    wmsParams: this.CONFIG?.noaaRadarWmsParams || {
                        LAYERS: 'base_reflectivity_mosaic',
                        FORMAT: 'image/png',
                        TRANSPARENT: true,
                    },
                    zIndex: 647,
                    defaultVisible: false,
                    minZoom: 3,
                    maxZoom: 18,
                    opacity: 0.55,
                    refreshPolicy: { type: 'interval', intervalMs: 120000 },
                    attribution: this.CONFIG?.noaaRadarAttribution || 'NOAA nowCOAST radar',
                    licenseNotes: 'Disabled by default; configure approved NOAA radar source before use.',
                    failureIsolation: { disableOnError: true, maxErrors: 2, retryMaxAttempts: 3, retryBaseMs: 5000 },
                },
                {
                    id: 'airspace-polygons',
                    type: 'overlay',
                    sourceType: 'xyz',
                    url: this.CONFIG?.airspaceOverlayTmsUrl || 'https://map.adsbexchange.com/mapproxy/tiles/1.0.0/openaip/ul_grid/{z}/{x}/{y}.png',
                    zIndex: 630,
                    defaultVisible: false,
                    minZoom: 0,
                    maxZoom: 13,
                    opacity: 0.5,
                    refreshPolicy: { type: 'none' },
                    attribution: this.CONFIG?.airspaceOverlayAttribution || 'openAIP.net',
                    licenseNotes: 'Disabled by default; configure approved OpenAIP TMS source before use.',
                    failureIsolation: { disableOnError: true, maxErrors: 2, retryMaxAttempts: 2, retryBaseMs: 3000 },
                },
            ];
        }

        _initOverlayRegistry() {
            if (!this._olMap || !window.MapLayerRegistry || typeof window.MapLayerRegistry.createOpenLayersOverlayRegistry !== 'function') {
                return;
            }

            const configured = Array.isArray(this.CONFIG?.mapOverlays) && this.CONFIG.mapOverlays.length > 0
                ? this.CONFIG.mapOverlays
                : this._getDefaultOverlayConfigs();

            const validConfigs = configured.filter((config) => {
                if (!config || !config.id || !config.sourceType) return false;
                const sourceType = String(config.sourceType).toLowerCase();
                if ((sourceType === 'xyz' || sourceType === 'wms' || sourceType === 'image-wms' || sourceType === 'geojson') && !config.url) {
                    return false;
                }
                return true;
            });

            this._overlayRegistry = window.MapLayerRegistry.createOpenLayersOverlayRegistry(this._olMap, validConfigs);
            if (this._overlayRegistry && typeof this._overlayRegistry.registerAll === 'function') {
                this._overlayStatuses = this._overlayRegistry.registerAll();
                this._overlayRegistry.updateZoomVisibility(this.getZoom());
            }
        }

        _renderReferenceLayersForCurrentView() {
            if (!this._olMap) return;
            this._renderRunways();
            this._renderAirports();
            this._renderHeliports();
            this._renderNavaids();
            this._renderAllRunways();
            this._renderRangeRings();
        }

        _ensureReferencePopup() {
            if (!this._olMap || this._referencePopupOverlay) return;

            const container = document.createElement('div');
            container.className = 'ref-popup-container ol-ref-popup';
            container.style.display = 'none';

            const content = document.createElement('div');
            content.className = 'ref-popup';

            const closeButton = document.createElement('button');
            closeButton.type = 'button';
            closeButton.className = 'ref-popup-close';
            closeButton.textContent = '✕';
            closeButton.addEventListener('click', () => this._closeReferencePopup());

            container.appendChild(closeButton);
            container.appendChild(content);

            const overlay = new window.ol.Overlay({
                element: container,
                positioning: 'bottom-center',
                offset: [0, -12],
                autoPan: { animation: { duration: 150 } },
                stopEvent: true,
            });

            this._referencePopupOverlay = overlay;
            this._referencePopupElement = content;
            this._olMap.addOverlay(overlay);
        }

        _closeReferencePopup() {
            if (!this._referencePopupOverlay) return;
            this._referencePopupOverlay.setPosition(undefined);
            const element = this._referencePopupOverlay.getElement();
            if (element) {
                element.style.display = 'none';
            }
            if (this._referencePopupElement) {
                this._referencePopupElement.innerHTML = '';
            }
        }

        _openReferencePopup(coordinate, html) {
            this._ensureReferencePopup();
            if (!this._referencePopupOverlay || !this._referencePopupElement) return;

            this._referencePopupElement.innerHTML = html || '';
            const element = this._referencePopupOverlay.getElement();
            if (element) {
                element.style.display = 'block';
            }
            this._referencePopupOverlay.setPosition(coordinate);
        }

        _airportPopupHtml(airport) {
            const frequencyRows = (airport?.frequencies || []).map((frequency) => {
                const type = frequency?.type || '';
                const description = frequency?.description || '';
                const mhz = frequency?.frequency_mhz || '';
                return `<tr><td class="ref-popup-type">${type}</td><td>${description}</td><td class="ref-popup-freq">${mhz}</td></tr>`;
            }).join('');

            const frequencyTable = frequencyRows
                ? `<table class="ref-popup-table"><tr><th>Type</th><th>Desc</th><th>MHz</th></tr>${frequencyRows}</table>`
                : '';

            return `<div class="ref-popup-title">${airport?.name || airport?.ident || 'Airport'}</div>
                <div class="ref-popup-subtitle">${airport?.ident || ''} · ${(airport?.type || '').replace(/_/g, ' ')} · ${airport?.elevation_ft ?? '?'} ft</div>
                ${airport?.municipality ? `<div class="ref-popup-detail">${airport.municipality}, ${airport?.iso_country || ''}</div>` : ''}
                ${airport?.iata_code ? `<div class="ref-popup-detail">IATA: ${airport.iata_code}</div>` : ''}
                ${frequencyTable}`;
        }

        _navaidPopupHtml(navaid) {
            const frequencyKHz = Number(navaid?.frequency_khz);
            let frequencyDisplay = '';
            if (Number.isFinite(frequencyKHz)) {
                frequencyDisplay = `${(frequencyKHz / 1000).toFixed(frequencyKHz >= 100000 ? 2 : 1)} ${frequencyKHz >= 100000 ? 'MHz' : 'kHz'}`;
            }

            return `<div class="ref-popup-title">${navaid?.ident || ''} - ${navaid?.name || 'Navaid'}</div>
                <div class="ref-popup-subtitle">${navaid?.type || ''}${frequencyDisplay ? ` · ${frequencyDisplay}` : ''}</div>
                ${navaid?.elevation_ft ? `<div class="ref-popup-detail">Elev: ${navaid.elevation_ft} ft</div>` : ''}
                ${navaid?.associated_airport ? `<div class="ref-popup-detail">Airport: ${navaid.associated_airport}</div>` : ''}
                ${navaid?.usage_type ? `<div class="ref-popup-detail">Usage: ${navaid.usage_type}${navaid?.power ? ` · ${navaid.power}` : ''}</div>` : ''}`;
        }

        _heliportPopupHtml(heliport) {
            const frequencyRows = (heliport?.frequencies || []).map((frequency) => {
                const type = frequency?.type || '';
                const description = frequency?.description || '';
                const mhz = frequency?.frequency_mhz || '';
                return `<tr><td class="ref-popup-type">${type}</td><td>${description}</td><td class="ref-popup-freq">${mhz}</td></tr>`;
            }).join('');

            const frequencyTable = frequencyRows
                ? `<table class="ref-popup-table"><tr><th>Type</th><th>Desc</th><th>MHz</th></tr>${frequencyRows}</table>`
                : '';

            return `<div class="ref-popup-title">${heliport?.name || heliport?.ident || 'Heliport'}</div>
                <div class="ref-popup-subtitle">${heliport?.ident || ''} · heliport · ${heliport?.elevation_ft ?? '?'} ft</div>
                ${heliport?.municipality ? `<div class="ref-popup-detail">${heliport.municipality}, ${heliport?.iso_country || ''}</div>` : ''}
                ${frequencyTable}`;
        }

        _navaidSvgMarkup(type, size = 18) {
            const s = size;
            const h = s / 2;
            const colors = {
                VOR: '#60A5FA',
                'VOR-DME': '#60A5FA',
                VORTAC: '#818CF8',
                NDB: '#F59E0B',
                'NDB-DME': '#F59E0B',
                DME: '#A78BFA',
                TACAN: '#818CF8',
            };
            const color = colors[type] || '#9CA3AF';

            const hex = (radius) => {
                const points = [];
                for (let i = 0; i < 6; i++) {
                    const angle = Math.PI / 6 + i * Math.PI / 3;
                    points.push(`${(h + radius * Math.cos(angle)).toFixed(1)},${(h - radius * Math.sin(angle)).toFixed(1)}`);
                }
                return points.join(' ');
            };

            switch (type) {
                case 'VOR':
                    return `<svg xmlns="http://www.w3.org/2000/svg" width="${s}" height="${s}" viewBox="0 0 ${s} ${s}"><polygon points="${hex(h * 0.8)}" fill="none" stroke="${color}" stroke-width="1.5"/></svg>`;
                case 'VOR-DME':
                    return `<svg xmlns="http://www.w3.org/2000/svg" width="${s}" height="${s}" viewBox="0 0 ${s} ${s}"><polygon points="${hex(h * 0.8)}" fill="none" stroke="${color}" stroke-width="1.5"/><circle cx="${h}" cy="${h}" r="2" fill="${color}"/></svg>`;
                case 'VORTAC': {
                    const spokes = [90, 210, 330].map((angle) => {
                        const radians = angle * Math.PI / 180;
                        return `<line x1="${(h + h * 0.72 * Math.cos(radians)).toFixed(1)}" y1="${(h - h * 0.72 * Math.sin(radians)).toFixed(1)}" x2="${(h + h * 0.95 * Math.cos(radians)).toFixed(1)}" y2="${(h - h * 0.95 * Math.sin(radians)).toFixed(1)}" stroke="${color}" stroke-width="2"/>`;
                    }).join('');
                    return `<svg xmlns="http://www.w3.org/2000/svg" width="${s}" height="${s}" viewBox="0 0 ${s} ${s}"><polygon points="${hex(h * 0.7)}" fill="none" stroke="${color}" stroke-width="1.5"/>${spokes}</svg>`;
                }
                case 'NDB': {
                    const dots = [0, 60, 120, 180, 240, 300].map((angle) => {
                        const radians = angle * Math.PI / 180;
                        return `<circle cx="${(h + h * 0.7 * Math.cos(radians)).toFixed(1)}" cy="${(h - h * 0.7 * Math.sin(radians)).toFixed(1)}" r="1" fill="${color}" opacity="0.6"/>`;
                    }).join('');
                    return `<svg xmlns="http://www.w3.org/2000/svg" width="${s}" height="${s}" viewBox="0 0 ${s} ${s}"><circle cx="${h}" cy="${h}" r="3" fill="${color}"/>${dots}</svg>`;
                }
                case 'NDB-DME': {
                    const dots = [0, 60, 120, 180, 240, 300].map((angle) => {
                        const radians = angle * Math.PI / 180;
                        return `<circle cx="${(h + h * 0.7 * Math.cos(radians)).toFixed(1)}" cy="${(h - h * 0.7 * Math.sin(radians)).toFixed(1)}" r="1" fill="${color}" opacity="0.6"/>`;
                    }).join('');
                    return `<svg xmlns="http://www.w3.org/2000/svg" width="${s}" height="${s}" viewBox="0 0 ${s} ${s}"><circle cx="${h}" cy="${h}" r="3" fill="${color}"/>${dots}<rect x="${h - 2}" y="${h - 2}" width="4" height="4" fill="none" stroke="${color}" stroke-width="0.8"/></svg>`;
                }
                case 'DME':
                    return `<svg xmlns="http://www.w3.org/2000/svg" width="${s}" height="${s}" viewBox="0 0 ${s} ${s}"><rect x="${h - h * 0.5}" y="${h - h * 0.5}" width="${h}" height="${h}" fill="none" stroke="${color}" stroke-width="1.5"/></svg>`;
                case 'TACAN': {
                    const spokes = [90, 210, 330].map((angle) => {
                        const radians = angle * Math.PI / 180;
                        return `<line x1="${(h + h * 0.2 * Math.cos(radians)).toFixed(1)}" y1="${(h - h * 0.2 * Math.sin(radians)).toFixed(1)}" x2="${(h + h * 0.8 * Math.cos(radians)).toFixed(1)}" y2="${(h - h * 0.8 * Math.sin(radians)).toFixed(1)}" stroke="${color}" stroke-width="2"/>`;
                    }).join('');
                    return `<svg xmlns="http://www.w3.org/2000/svg" width="${s}" height="${s}" viewBox="0 0 ${s} ${s}">${spokes}<circle cx="${h}" cy="${h}" r="2" fill="${color}"/></svg>`;
                }
                default:
                    return `<svg xmlns="http://www.w3.org/2000/svg" width="${s}" height="${s}" viewBox="0 0 ${s} ${s}"><circle cx="${h}" cy="${h}" r="4" fill="none" stroke="${color}" stroke-width="1.5"/></svg>`;
            }
        }

        _navaidIconStyle(type, size = 18) {
            const svg = this._navaidSvgMarkup(type, size);
            const url = `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`;
            return new window.ol.style.Icon({
                src: url,
                imgSize: [size, size],
                anchor: [size / 2, size / 2],
                anchorXUnits: 'pixels',
                anchorYUnits: 'pixels',
                rotateWithView: false,
            });
        }

        _handleReferenceFeatureClick(event) {
            if (!this._olMap || !event) return false;

            let clickedFeature = null;
            this._olMap.forEachFeatureAtPixel(event.pixel, (feature, candidateLayer) => {
                const refType = feature?.get ? feature.get('refType') : null;
                if (!refType) return false;

                const isReferenceLayer = Object.values(this._referenceLayerState).some((entry) => entry?.layer === candidateLayer)
                    || candidateLayer === this._rangeRingLayerState?.layer;
                if (!isReferenceLayer) return false;

                clickedFeature = feature;
                return true;
            }, { hitTolerance: 6 });

            if (!clickedFeature) {
                this._closeReferencePopup();
                return false;
            }

            const refType = clickedFeature.get('refType');
            const refData = clickedFeature.get('refData');
            if (!refType || !refData) {
                this._closeReferencePopup();
                return false;
            }

            let html = '';
            if (refType === 'airport') {
                html = this._airportPopupHtml(refData);
            } else if (refType === 'navaid') {
                html = this._navaidPopupHtml(refData);
            } else if (refType === 'heliport') {
                html = this._heliportPopupHtml(refData);
            }

            if (!html) {
                this._closeReferencePopup();
                return false;
            }

            this._openReferencePopup(event.coordinate, html);
            return true;
        }

        _getReferenceLabelStyle(text, color = '#9CA3AF') {
            return new window.ol.style.Text({
                text: text || '',
                font: '10px ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
                offsetY: -10,
                fill: new window.ol.style.Fill({ color }),
                backgroundFill: new window.ol.style.Fill({ color: 'rgba(0,0,0,0.45)' }),
                padding: [1, 3, 1, 3],
            });
        }

        _renderRunways() {
            this._clearReferenceLayer('runways');
            const state = this._referenceLayerState.runways;
            if (!state || !state.source || !this.runwayData || !this._isLayerVisible('runways')) return;

            const zoom = this.getZoom();
            if (zoom < 9) return;
            const thresholdsMap = this.runwayData.runway_thresholds || {};
            const extensionsMap = this.runwayData.runway_extensions || {};
            const showLabels = zoom >= 10;
            const showExtensions = zoom >= 11;
            const lineWeight = Math.max(4, Math.min(8, zoom - 6));

            Object.keys(thresholdsMap).forEach((runwayId) => {
                const thresholds = thresholdsMap[runwayId];
                const ends = Object.keys(thresholds || {});
                if (ends.length !== 2) return;

                const endA = thresholds[ends[0]];
                const endB = thresholds[ends[1]];
                if (!endA || !endB) return;

                const line = new window.ol.Feature({
                    geometry: new window.ol.geom.LineString([
                        this._toMapCoordinate(endA.latitude, endA.longitude),
                        this._toMapCoordinate(endB.latitude, endB.longitude),
                    ]),
                });
                line.setStyle(new window.ol.style.Style({
                    stroke: new window.ol.style.Stroke({ color: '#FFFFFF', width: lineWeight }),
                }));
                state.source.addFeature(line);

                if (showLabels) {
                    [ends[0], ends[1]].forEach((endName) => {
                        const end = thresholds[endName];
                        if (!end) return;
                        const label = new window.ol.Feature({
                            geometry: new window.ol.geom.Point(this._toMapCoordinate(end.latitude, end.longitude)),
                        });
                        label.setStyle(new window.ol.style.Style({ text: this._getReferenceLabelStyle(endName, '#FFFFFF') }));
                        state.source.addFeature(label);
                    });
                }

                if (showExtensions) {
                    const extensionSet = extensionsMap[runwayId] || {};
                    Object.keys(extensionSet).forEach((endName) => {
                        const points = extensionSet[endName];
                        if (!Array.isArray(points) || points.length < 2) return;
                        const lineExt = new window.ol.Feature({
                            geometry: new window.ol.geom.LineString(
                                points
                                    .filter((point) => typeof point?.latitude === 'number' && typeof point?.longitude === 'number')
                                    .map((point) => this._toMapCoordinate(point.latitude, point.longitude))
                            ),
                        });
                        lineExt.setStyle(new window.ol.style.Style({
                            stroke: new window.ol.style.Stroke({ color: '#76C76C', width: 1.5, lineDash: [8, 12] }),
                        }));
                        state.source.addFeature(lineExt);
                    });
                }
            });
        }

        _renderAirports() {
            this._clearReferenceLayer('airports');
            const state = this._referenceLayerState.airports;
            if (!state || !state.source || !Array.isArray(this.airportsData) || !this._isLayerVisible('airports')) return;

            const zoom = this.getZoom();
            if (zoom < 8) return;
            const showLabels = zoom >= 12;
            const typeCfg = {
                large_airport: { color: '#60A5FA', radius: 6, minZoom: 8 },
                medium_airport: { color: '#34D399', radius: 5, minZoom: 9 },
                small_airport: { color: '#A78BFA', radius: 4, minZoom: 10 },
                seaplane_base: { color: '#2DD4BF', radius: 3, minZoom: 11 },
            };

            this.airportsData.forEach((airport) => {
                const cfg = typeCfg[airport?.type] || typeCfg.small_airport;
                if (zoom < cfg.minZoom) return;
                if (typeof airport?.latitude !== 'number' || typeof airport?.longitude !== 'number') return;

                const feature = new window.ol.Feature({
                    geometry: new window.ol.geom.Point(this._toMapCoordinate(airport.latitude, airport.longitude)),
                });
                feature.set('refType', 'airport');
                feature.set('refData', airport);
                feature.setStyle(new window.ol.style.Style({
                    image: new window.ol.style.Circle({
                        radius: cfg.radius,
                        fill: new window.ol.style.Fill({ color: cfg.color }),
                        stroke: new window.ol.style.Stroke({ color: cfg.color, width: 1 }),
                    }),
                    text: showLabels ? this._getReferenceLabelStyle(airport.ident || '', cfg.color) : undefined,
                }));
                state.source.addFeature(feature);
            });
        }

        _renderHeliports() {
            this._clearReferenceLayer('heliports');
            const state = this._referenceLayerState.heliports;
            if (!state || !state.source || !Array.isArray(this.heliportsData) || !this._isLayerVisible('heliports')) return;

            const zoom = this.getZoom();
            if (zoom < 11) return;
            const showLabels = zoom >= 12;

            this.heliportsData.forEach((heliport) => {
                if (typeof heliport?.latitude !== 'number' || typeof heliport?.longitude !== 'number') return;
                const feature = new window.ol.Feature({
                    geometry: new window.ol.geom.Point(this._toMapCoordinate(heliport.latitude, heliport.longitude)),
                });
                feature.set('refType', 'heliport');
                feature.set('refData', heliport);
                feature.setStyle(new window.ol.style.Style({
                    image: new window.ol.style.Circle({
                        radius: 4,
                        fill: new window.ol.style.Fill({ color: '#FB923C' }),
                        stroke: new window.ol.style.Stroke({ color: '#FB923C', width: 1.5 }),
                    }),
                    text: showLabels ? this._getReferenceLabelStyle(heliport.ident || '', '#FB923C') : undefined,
                }));
                state.source.addFeature(feature);
            });
        }

        _navaidColor(type) {
            const map = {
                VOR: '#60A5FA',
                'VOR-DME': '#60A5FA',
                VORTAC: '#818CF8',
                NDB: '#F59E0B',
                'NDB-DME': '#F59E0B',
                DME: '#A78BFA',
                TACAN: '#818CF8',
            };
            return map[type] || '#9CA3AF';
        }

        _renderNavaids() {
            this._clearReferenceLayer('navaids');
            const state = this._referenceLayerState.navaids;
            if (!state || !state.source || !Array.isArray(this.navaidsData) || !this._isLayerVisible('navaids')) return;

            const zoom = this.getZoom();
            if (zoom < 9) return;
            const showLabels = zoom >= 12;
            const useSymbolIcons = zoom >= 11;
            const radius = useSymbolIcons ? 1 : 3;

            this.navaidsData.forEach((navaid) => {
                if (typeof navaid?.latitude !== 'number' || typeof navaid?.longitude !== 'number') return;
                const color = this._navaidColor(navaid.type);
                const feature = new window.ol.Feature({
                    geometry: new window.ol.geom.Point(this._toMapCoordinate(navaid.latitude, navaid.longitude)),
                });
                feature.set('refType', 'navaid');
                feature.set('refData', navaid);
                feature.setStyle(new window.ol.style.Style({
                    image: useSymbolIcons
                        ? this._navaidIconStyle(navaid.type, 18)
                        : new window.ol.style.Circle({
                            radius,
                            fill: new window.ol.style.Fill({ color }),
                            stroke: new window.ol.style.Stroke({ color, width: 1 }),
                        }),
                    text: showLabels ? this._getReferenceLabelStyle(navaid.ident || '', color) : undefined,
                }));
                state.source.addFeature(feature);
            });
        }

        _renderAllRunways() {
            this._clearReferenceLayer('allRunways');
            const state = this._referenceLayerState.allRunways;
            if (!state || !state.source || !Array.isArray(this.allRunwaysData) || !this._isLayerVisible('allRunways')) return;

            const zoom = this.getZoom();
            if (zoom < 10) return;
            const showLabels = zoom >= 12;
            const weight = Math.max(2, Math.min(5, zoom - 8));

            this.allRunwaysData.forEach((runway) => {
                if (
                    typeof runway?.le_latitude !== 'number' || typeof runway?.le_longitude !== 'number' ||
                    typeof runway?.he_latitude !== 'number' || typeof runway?.he_longitude !== 'number'
                ) return;

                const line = new window.ol.Feature({
                    geometry: new window.ol.geom.LineString([
                        this._toMapCoordinate(runway.le_latitude, runway.le_longitude),
                        this._toMapCoordinate(runway.he_latitude, runway.he_longitude),
                    ]),
                });
                line.setStyle(new window.ol.style.Style({
                    stroke: new window.ol.style.Stroke({ color: '#9CA3AF', width: weight }),
                }));
                state.source.addFeature(line);

                if (showLabels) {
                    if (runway.le_ident) {
                        const leLabel = new window.ol.Feature({
                            geometry: new window.ol.geom.Point(this._toMapCoordinate(runway.le_latitude, runway.le_longitude)),
                        });
                        leLabel.setStyle(new window.ol.style.Style({ text: this._getReferenceLabelStyle(runway.le_ident, '#9CA3AF') }));
                        state.source.addFeature(leLabel);
                    }
                    if (runway.he_ident) {
                        const heLabel = new window.ol.Feature({
                            geometry: new window.ol.geom.Point(this._toMapCoordinate(runway.he_latitude, runway.he_longitude)),
                        });
                        heLabel.setStyle(new window.ol.style.Style({ text: this._getReferenceLabelStyle(runway.he_ident, '#9CA3AF') }));
                        state.source.addFeature(heLabel);
                    }
                }
            });
        }

        _renderRangeRings() {
            if (!this._rangeRingLayerState || !this._rangeRingLayerState.source) return;
            this._rangeRingLayerState.source.clear();
            if (!this._isRangeRingVisible()) return;

            const centerLat = this.store?.stationOverride?.active
                ? (this.store.stationOverride.latitude || this.store.stationLatitude || 0)
                : (this.store?.stationLatitude || 0);
            const centerLon = this.store?.stationOverride?.active
                ? (this.store.stationOverride.longitude || this.store.stationLongitude || 0)
                : (this.store?.stationLongitude || 0);
            if (!Number.isFinite(centerLat) || !Number.isFinite(centerLon)) return;

            const center = this._toMapCoordinate(centerLat, centerLon);
            const rings = Array.isArray(this.CONFIG?.rangeRings) ? this.CONFIG.rangeRings : [];
            rings.forEach((radiusNm) => {
                const radiusMeters = Number(radiusNm) * 1852;
                if (!Number.isFinite(radiusMeters) || radiusMeters <= 0) return;

                const ring = new window.ol.Feature({
                    geometry: new window.ol.geom.Circle(center, radiusMeters),
                });
                ring.setStyle(new window.ol.style.Style({
                    stroke: new window.ol.style.Stroke({ color: 'rgba(115, 115, 115, 0.5)', width: 1 }),
                }));
                this._rangeRingLayerState.source.addFeature(ring);

                const labelOffsetMeters = radiusMeters;
                const labelPoint = new window.ol.Feature({
                    geometry: new window.ol.geom.Point([center[0], center[1] + labelOffsetMeters]),
                });
                labelPoint.setStyle(new window.ol.style.Style({
                    text: this._getReferenceLabelStyle(`${radiusNm} NM`, 'rgba(115, 115, 115, 0.8)'),
                }));
                this._rangeRingLayerState.source.addFeature(labelPoint);
            });
        }

        _getTrailColorForAircraft(aircraft) {
            if (!aircraft) return '#4CAF50';
            if (aircraft.status === 'signal_lost') return '#9CA3AF';
            if (aircraft.status === 'stale') return '#FFC107';
            if (aircraft.on_ground) return '#60A5FA';
            return '#4CAF50';
        }

        _updateLiveTrailForAircraft(hex, aircraft) {
            if (!hex || !aircraft || !aircraft.adsb) return;

            const lat = aircraft.adsb.lat;
            const lon = aircraft.adsb.lon;
            const hasPosition = typeof lat === 'number' && typeof lon === 'number' && (lat !== 0 || lon !== 0);
            if (!hasPosition) return;

            const isStalePosition = aircraft.stale_position === true;
            const now = new Date();

            if (!isStalePosition) {
                if (!Array.isArray(this.trails[hex])) {
                    this.trails[hex] = [];
                }
                this.trails[hex].push({
                    lat,
                    lon,
                    alt_baro: (typeof aircraft.adsb?.alt_baro === 'number' && Number.isFinite(aircraft.adsb.alt_baro))
                        ? aircraft.adsb.alt_baro
                        : null,
                    time: now,
                    isHistorical: false,
                });
            }

            if (Array.isArray(this.trails[hex])) {
                const selectedTrailLength = Number(this.store?.settings?.trailLength);
                const retentionMinutes = Number.isFinite(selectedTrailLength) ? selectedTrailLength : 2;
                const cutoffMs = now.getTime() - (retentionMinutes * 60 * 1000);

                const trail = this.trails[hex];
                let pruneIdx = 0;
                while (pruneIdx < trail.length) {
                    const point = trail[pruneIdx];
                    const pointMs = point?.time instanceof Date
                        ? point.time.getTime()
                        : (point?.time ? new Date(point.time).getTime() : NaN);
                    if (!Number.isFinite(pointMs) || pointMs >= cutoffMs) break;
                    pruneIdx++;
                }
                if (pruneIdx > 0) {
                    trail.splice(0, pruneIdx);
                }

                if (trail.length > 500) {
                    trail.splice(0, trail.length - 500);
                }
            }
        }

        _createTrailFeature(hex, latLonPoints, styleOptions) {
            if (!Array.isArray(latLonPoints) || latLonPoints.length < 2) return null;

            const projected = latLonPoints
                .filter((point) => Array.isArray(point) && point.length >= 2)
                .map((point) => this._toMapCoordinate(point[0], point[1]));

            if (projected.length < 2) return null;

            const geometry = new window.ol.geom.LineString(projected);
            const feature = new window.ol.Feature({ geometry });
            feature.set('hex', hex);
            feature.set('trailType', styleOptions?.trailType || 'current');

            feature.setStyle(new window.ol.style.Style({
                stroke: new window.ol.style.Stroke({
                    color: styleOptions?.color || '#4CAF50',
                    width: Number.isFinite(styleOptions?.width) ? styleOptions.width : 2,
                    lineDash: Array.isArray(styleOptions?.lineDash) ? styleOptions.lineDash : undefined,
                }),
            }));

            return feature;
        }

        _createTrailMetadataFeature(hex, latitude, longitude, styleOptions = {}) {
            if (typeof latitude !== 'number' || typeof longitude !== 'number') return [];

            const coordinate = this._toMapCoordinate(latitude, longitude);
            const features = [];

            const pointFeature = new window.ol.Feature({
                geometry: new window.ol.geom.Point(coordinate),
            });
            pointFeature.set('hex', hex);
            pointFeature.set('trailType', styleOptions?.trailType || 'metadata');
            pointFeature.setStyle(new window.ol.style.Style({
                image: new window.ol.style.Circle({
                    radius: Number.isFinite(styleOptions?.radius) ? styleOptions.radius : 3,
                    fill: new window.ol.style.Fill({ color: styleOptions?.fillColor || '#888888' }),
                    stroke: new window.ol.style.Stroke({
                        color: styleOptions?.strokeColor || styleOptions?.fillColor || '#888888',
                        width: Number.isFinite(styleOptions?.strokeWidth) ? styleOptions.strokeWidth : 1,
                    }),
                }),
            }));
            features.push(pointFeature);

            const text = (styleOptions?.labelText || '').toString().trim();
            if (text) {
                const labelFeature = new window.ol.Feature({
                    geometry: new window.ol.geom.Point(coordinate),
                });
                labelFeature.set('hex', hex);
                labelFeature.set('trailType', styleOptions?.trailType || 'metadata');
                labelFeature.setStyle(new window.ol.style.Style({
                    text: new window.ol.style.Text({
                        text,
                        font: '10px ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
                        offsetX: Number.isFinite(styleOptions?.labelOffsetX) ? styleOptions.labelOffsetX : 6,
                        offsetY: Number.isFinite(styleOptions?.labelOffsetY) ? styleOptions.labelOffsetY : 0,
                        textAlign: 'left',
                        fill: new window.ol.style.Fill({ color: styleOptions?.labelColor || '#888888' }),
                    }),
                }));
                features.push(labelFeature);
            }

            return features;
        }

        _replaceTrailFeatures(hex, features) {
            const existing = this._trailFeaturesByHex.get(hex);
            if (existing && this._trailSource) {
                existing.forEach((feature) => this._trailSource.removeFeature(feature));
            }

            if (!Array.isArray(features) || features.length === 0) {
                this._trailFeaturesByHex.delete(hex);
                return;
            }

            this._trailFeaturesByHex.set(hex, features);
            if (this._trailSource) {
                features.forEach((feature) => this._trailSource.addFeature(feature));
            }
        }

        _getAircraftFeature(hex) {
            if (!this._aircraftRenderer || typeof this._aircraftRenderer.getFeature !== 'function') {
                return null;
            }
            return this._aircraftRenderer.getFeature(hex);
        }

        _getAircraftHitLayer() {
            if (!this._aircraftRenderer || typeof this._aircraftRenderer.getLayer !== 'function') {
                return null;
            }
            return this._aircraftRenderer.getLayer();
        }

        _getAircraftStyle(feature) {
            const hex = feature && feature.get ? feature.get('hex') : null;
            const aircraft = (hex && this.store && this.store.aircraft) ? this.store.aircraft[hex] : null;

            const isSelected = !!(this.store && this.store.selectedAircraft && this.store.selectedAircraft.hex === hex);
            const isHovered = !!(this.store && this.store.hoveredAircraft && this.store.hoveredAircraft.hex === hex);

            let fillColor = '#4CAF50';
            if (aircraft && aircraft.status === 'signal_lost') {
                fillColor = '#F44336';
            } else if (aircraft && aircraft.status === 'stale') {
                fillColor = '#FFC107';
            } else if (aircraft && aircraft.on_ground) {
                fillColor = '#60A5FA';
            }

            if (isSelected) {
                fillColor = '#FDE047';
            } else if (isHovered) {
                fillColor = '#86EFAC';
            }

            const radius = isSelected ? 7 : (isHovered ? 6 : 5);

            return new window.ol.style.Style({
                image: new window.ol.style.Circle({
                    radius,
                    fill: new window.ol.style.Fill({ color: fillColor }),
                    stroke: new window.ol.style.Stroke({
                        color: isSelected ? '#EAB308' : '#0B2F0B',
                        width: isSelected ? 2 : 1,
                    }),
                }),
            });
        }

        _setFeatureVisible(hex, aircraft, visible) {
            if (!this._aircraftRenderer) return;
            if (!aircraft || !aircraft.adsb) return;
            const lat = aircraft.adsb.lat;
            const lon = aircraft.adsb.lon;
            const hasPosition = typeof lat === 'number' && typeof lon === 'number' && (lat !== 0 || lon !== 0);
            if (!hasPosition) return;

            if (!visible) {
                this._aircraftRenderer.removeAircraftFeature(hex);
                return;
            }

            const selectedHex = this.store?.selectedAircraft?.hex || null;
            const hoveredHex = this.store?.hoveredAircraft?.hex || null;
            const smoothingEnabled = !!this.store?.settings?.aircraftAnimation?.enabled;
            this._aircraftRenderer.upsertAircraftFeature(aircraft, {
                selected: selectedHex === hex,
                hovered: hoveredHex === hex,
                deemphasized: !!selectedHex && selectedHex !== hex,
            }, {
                preservePose: smoothingEnabled,
            });
        }

        _aircraftMatchesFilters(aircraft) {
            if (!aircraft) return false;
            if (!this.store || !this.store.settings) return true;

            if (window.MapVisibilityRules && window.MapVisibilityRules.shouldShowAircraftOnMap) {
                const now = new Date();
                const lastSeenCutoff = new Date(now.getTime() - ((this.store.settings?.lastSeenMinutes || 10) * 60 * 1000));
                return window.MapVisibilityRules.shouldShowAircraftOnMap(aircraft, this.store, {
                    searchTerm: this.store.searchTerm || '',
                    lastSeenCutoff,
                    isInViewport: true,
                });
            }

            return true;
        }

        _warnOnce(key, message) {
            if (this._warnings.has(key)) return;
            this._warnings.add(key);
            console.warn(message);
        }

        _getInitialCenter() {
            const lat = this.store?.stationOverride?.active
                ? (this.store.stationOverride.latitude || this.store.stationLatitude || 43.6777)
                : (this.store?.stationLatitude || 43.6777);
            const lon = this.store?.stationOverride?.active
                ? (this.store.stationOverride.longitude || this.store.stationLongitude || -79.6248)
                : (this.store?.stationLongitude || -79.6248);
            return { lat, lon };
        }

        _createMapShim() {
            return {
                setView: (position, zoom) => {
                    if (!Array.isArray(position) || position.length < 2) return;
                    this.setView(position[0], position[1], zoom);
                },
                getZoom: () => this.getZoom(),
            };
        }

        initMap() {
            this._stats.initCalls++;
            if (this.engine) return;

            if (!window.MapEngine || !window.MapEngine.createOpenLayersEngine) {
                this._warnOnce('engine-missing', 'MapEngine.createOpenLayersEngine is unavailable');
                return;
            }

            const center = this._getInitialCenter();
            const zoom = Number.isFinite(this.CONFIG?.defaultZoom) ? this.CONFIG.defaultZoom : 10;

            this.engine = window.MapEngine.createOpenLayersEngine({
                targetId: 'map',
                center,
                zoom,
                baseMapStyle: this.store?.settings?.mapStyle || 'dark',
            });
            this.engine.init();
            this._olMap = this.engine.getMap();
            console.info('[OpenLayersMapManager] OpenLayers map initialized');

            if (!window.MapAircraftWebGLRenderer || !window.MapAircraftWebGLRenderer.createAircraftWebGLRenderer) {
                this._warnOnce('renderer-missing', 'MapAircraftWebGLRenderer is unavailable; aircraft rendering cannot start');
            } else {
                this._aircraftRenderer = window.MapAircraftWebGLRenderer.createAircraftWebGLRenderer(this._olMap, {
                    zIndex: 400,
                    store: this.store,
                    preferWebGL: !!this.CONFIG?.mapAircraftWebGL,
                });
                this._aircraftRenderer.init();
            }

            this._ensureTrailLayer();
            this._ensureOverlayLayer();
            this._ensureReferenceLayers();
            this._initOverlayRegistry();
            this._ensureReferencePopup();

            this.map = this._createMapShim();

            if (window.MapInteractionsFeature?.registerOpenLayersInteractions) {
                this._interactionCleanup = window.MapInteractionsFeature.registerOpenLayersInteractions(this);
            }

            this.engine.on('moveend', () => {
                this.updateVisibleAircraftList();
                this._scheduleReferenceRender();
                if (this._overlayRegistry && typeof this._overlayRegistry.updateZoomVisibility === 'function') {
                    this._overlayRegistry.updateZoomVisibility(this.getZoom());
                }
            });
            this.startLabelRefreshTimer();
            this._scheduleReferenceRender();
        }

        startLabelRefreshTimer() {
            this.stopLabelRefreshTimer();
            this._labelRefreshInterval = window.setInterval(() => {
                this.refreshStaleLabels();
            }, 5000);
        }

        stopLabelRefreshTimer() {
            if (this._labelRefreshInterval) {
                clearInterval(this._labelRefreshInterval);
                this._labelRefreshInterval = null;
            }
        }

        refreshStaleLabels() {
            if (!this._aircraftRenderer || typeof this._aircraftRenderer.requestRedraw !== 'function') return;
            this._aircraftRenderer.requestRedraw();
        }

        setView(lat, lon, zoom) {
            if (!this.engine) return;
            this.engine.setView(lat, lon, zoom);
        }

        fitBounds(points) {
            if (!this.engine) return;
            this.engine.fitBounds(points);
        }

        getZoom() {
            if (!this.engine) return 0;
            return this.engine.getZoom();
        }

        setMapStyle(styleId) {
            if (!this.engine || typeof this.engine.setBaseMapStyle !== 'function') return;
            this.engine.setBaseMapStyle(styleId);
        }

        centerOnStation() {
            const center = this._getInitialCenter();
            this.setView(center.lat, center.lon, this.getZoom() || this.CONFIG.defaultZoom || 10);
        }

        centerOnAircraft(aircraft) {
            const lat = aircraft?.adsb?.lat;
            const lon = aircraft?.adsb?.lon;
            if (typeof lat !== 'number' || typeof lon !== 'number') return;
            this.setView(aircraft.adsb.lat, aircraft.adsb.lon, this.getZoom() || this.CONFIG.defaultZoom || 10);
        }

        applyFiltersAndRefreshView() {
            this._stats.refreshCalls++;
            const aircraftByHex = this.store?.aircraft || {};

            if (this._aircraftRenderer && typeof this._aircraftRenderer.bulkSyncAircraft === 'function') {
                this._aircraftRenderer.bulkSyncAircraft(
                    aircraftByHex,
                    (aircraft) => this._aircraftMatchesFilters(aircraft),
                    (hex) => ({
                        selected: this.store?.selectedAircraft?.hex === hex,
                        hovered: this.store?.hoveredAircraft?.hex === hex,
                        deemphasized: !!this.store?.selectedAircraft?.hex && this.store.selectedAircraft.hex !== hex,
                    })
                );
            } else {
                Object.keys(aircraftByHex).forEach((hex) => {
                    const aircraft = aircraftByHex[hex];
                    const visible = this._aircraftMatchesFilters(aircraft);
                    this._setFeatureVisible(hex, aircraft, visible);
                });
            }

            this.updateVisualState();
            this.updateVisibleAircraftList();
            this.updateFlightPaths();
        }

        updateVisibleAircraftList() {
            const nextSet = this._aircraftRenderer && typeof this._aircraftRenderer.getFeatureHexes === 'function'
                ? this._aircraftRenderer.getFeatureHexes()
                : new Set();
            const currentSet = this.store.visibleAircraftOnMap;
            if (currentSet && currentSet.size === nextSet.size) {
                let setsEqual = true;
                for (const hex of nextSet) {
                    if (!currentSet.has(hex)) {
                        setsEqual = false;
                        break;
                    }
                }
                if (setsEqual) return;
            }
            this.store.visibleAircraftOnMap = nextSet;
        }

        updateSingleAircraft(hex, aircraft) {
            this._stats.singleAircraftUpdates++;
            if (!hex || !aircraft) return;
            const startedAt = performance.now();

            this._updateLiveTrailForAircraft(hex, aircraft);

            const visible = this._aircraftMatchesFilters(aircraft);
            this._setFeatureVisible(hex, aircraft, visible);
            this.updateProximityCircle();
            this._stats.singleAircraftUpdateDurationMs += (performance.now() - startedAt);
        }

        updateAnimatedAircraftPose(hex, pose) {
            if (!hex || !pose) return false;

            const lat = Number(pose.lat);
            const lon = Number(pose.lon);
            if (!Number.isFinite(lat) || !Number.isFinite(lon) || (lat === 0 && lon === 0)) {
                return false;
            }

            const feature = this._getAircraftFeature(hex);
            if (!feature) return false;

            const geometry = feature.getGeometry();
            if (!geometry || typeof geometry.setCoordinates !== 'function') {
                return false;
            }

            geometry.setCoordinates(this._toMapCoordinate(lat, lon));

            const headingDeg = Number(pose.heading);
            if (Number.isFinite(headingDeg)) {
                feature.set('heading', headingDeg);
                feature.set('rotationRad', (headingDeg * Math.PI) / 180);
            }

            feature.changed();
            return true;
        }

        toggleRings() {
            this._ensureReferenceLayers();
            if (!this._rangeRingLayerState || !this._rangeRingLayerState.layer) return;
            const currentlyVisible = this._rangeRingLayerState.layer.getVisible();
            this._rangeRingLayerState.layer.setVisible(!currentlyVisible);
            this._renderRangeRings();
        }

        toggleLayerVisibility(layerName, visible) {
            this._ensureReferenceLayers();

            if (layerName === 'rangeRings') {
                if (this._rangeRingLayerState?.layer) {
                    const shouldShow = typeof visible === 'boolean'
                        ? visible
                        : !this._rangeRingLayerState.layer.getVisible();
                    this._rangeRingLayerState.layer.setVisible(shouldShow);
                    this._renderRangeRings();
                }
                return;
            }

            const state = this._referenceLayerState[layerName];
            if (!state || !state.layer) {
                if (this._overlayRegistry && typeof this._overlayRegistry.setVisible === 'function') {
                    const shouldShowOverlay = typeof visible === 'boolean' ? visible : true;
                    this._overlayRegistry.setVisible(layerName, shouldShowOverlay);
                    this._overlayStatuses = this._overlayRegistry.getStatuses();
                    if (this._overlayRegistry.updateZoomVisibility) {
                        this._overlayRegistry.updateZoomVisibility(this.getZoom());
                    }
                    return;
                }

                this._warnOnce(`toggle-layer-${layerName || 'unknown'}`, `toggleLayerVisibility(${layerName || 'unknown'}) is unsupported in OpenLayers manager`);
                return;
            }

            const shouldShow = typeof visible === 'boolean' ? visible : !state.layer.getVisible();
            state.layer.setVisible(shouldShow);
            this._scheduleReferenceRender();
        }
        addRangeRings() {
            this._ensureReferenceLayers();
            this._renderRangeRings();
        }
        updateVisualState(hex) {
            if (!this._aircraftRenderer) return;

            const selectedHex = this.store?.selectedAircraft?.hex || null;
            const hoveredHex = this.store?.hoveredAircraft?.hex || null;

            if (!hex) {
                this._aircraftRenderer.setVisualState(selectedHex, hoveredHex);
                return;
            }

            if (typeof this._aircraftRenderer.refreshAircraftStyle === 'function') {
                this._aircraftRenderer.refreshAircraftStyle(hex, selectedHex, hoveredHex);
            } else {
                this._aircraftRenderer.setVisualState(selectedHex, hoveredHex);
            }
        }

        handleReferenceFeatureClick(event) {
            return this._handleReferenceFeatureClick(event);
        }
        updateFlightPaths() {
            this._ensureTrailLayer();

            if (!this._trailSource) return;

            if (!this.store?.settings?.showPaths) {
                this._trailSource.clear();
                this._trailFeaturesByHex.clear();
                return;
            }

            const aircraftByHex = this.store?.aircraft || {};
            const selectedHex = this.store?.selectedAircraft?.hex || null;
            const currentHexes = new Set(Object.keys(aircraftByHex));

            this._trailFeaturesByHex.forEach((_, hex) => {
                if (!currentHexes.has(hex)) {
                    this._replaceTrailFeatures(hex, []);
                }
            });

            Object.keys(aircraftByHex).forEach((hex) => {
                const aircraft = aircraftByHex[hex];
                if (!aircraft) return;

                const markerVisible = this.store?.visibleAircraftOnMap instanceof Set
                    ? this.store.visibleAircraftOnMap.has(hex)
                    : true;
                const isSelected = selectedHex === hex;
                if (!markerVisible && !isSelected) {
                    this._replaceTrailFeatures(hex, []);
                    return;
                }

                const color = this._getTrailColorForAircraft(aircraft);
                const features = [];

                const trailPoints = (this.trails && Array.isArray(this.trails[hex]))
                    ? this.trails[hex]
                        .filter((point) => {
                            if (typeof point?.lat !== 'number' || typeof point?.lon !== 'number') return false;
                            const trailLengthMinutes = Number(this.store?.settings?.trailLength);
                            const safeTrailMinutes = Number.isFinite(trailLengthMinutes) ? trailLengthMinutes : 2;
                            const cutoffMs = Date.now() - (safeTrailMinutes * 60 * 1000);
                            const pointTime = point?.time instanceof Date
                                ? point.time.getTime()
                                : (point?.time ? new Date(point.time).getTime() : NaN);
                            return !Number.isFinite(pointTime) || pointTime >= cutoffMs;
                        })
                        .map((point) => [point.lat, point.lon])
                    : [];

                const currentTrailFeature = this._createTrailFeature(hex, trailPoints, {
                    trailType: 'current',
                    color,
                    width: 2,
                    lineDash: undefined,
                });
                if (currentTrailFeature) {
                    features.push(currentTrailFeature);
                }

                if (isSelected) {
                    const nowMs = Date.now();
                    const historyPoints = (this.store.aircraftDetailsHistoryData || [])
                        .filter((position) => typeof position?.lat === 'number' && typeof position?.lon === 'number')
                        .map((position) => [position.lat, position.lon]);
                    const historyFeature = this._createTrailFeature(hex, historyPoints, {
                        trailType: 'history',
                        color,
                        width: 2,
                        lineDash: [4, 4],
                    });
                    if (historyFeature) {
                        features.push(historyFeature);
                    }

                    const historyPositions = (this.store.aircraftDetailsHistoryData || [])
                        .filter((position) => typeof position?.lat === 'number' && typeof position?.lon === 'number');
                    if (historyPositions.length > 0) {
                        const shownMinutes = new Set();
                        let historyMetadataCount = 0;
                        const maxHistoryMetadata = 12;

                        for (let index = 0; index < historyPositions.length; index++) {
                            if (historyMetadataCount >= maxHistoryMetadata) break;
                            const position = historyPositions[index];
                            const timestampRaw = position?.timestamp || position?.time;
                            if (!timestampRaw) continue;

                            const timestampMs = new Date(timestampRaw).getTime();
                            if (!Number.isFinite(timestampMs)) continue;

                            const minutesAgo = Math.floor(Math.max(0, nowMs - timestampMs) / 60000);
                            const isEdgePoint = index === 0 || index === historyPositions.length - 1;
                            const shouldShow = isEdgePoint || (minutesAgo > 0 && !shownMinutes.has(minutesAgo));
                            if (!shouldShow) continue;

                            shownMinutes.add(minutesAgo);
                            const altitudeRaw = Number(position?.altitude ?? position?.alt_baro);
                            const altitudeText = Number.isFinite(altitudeRaw)
                                ? (altitudeRaw === 0 ? 'GND' : `${Math.round(altitudeRaw / 100) * 100}`)
                                : '';
                            const labelText = altitudeText ? `-${minutesAgo}m ${altitudeText}` : `-${minutesAgo}m`;
                            const metadataFeatures = this._createTrailMetadataFeature(hex, position.lat, position.lon, {
                                trailType: 'history-metadata',
                                radius: isEdgePoint ? 4 : 3,
                                fillColor: '#888888',
                                strokeColor: '#888888',
                                labelText,
                                labelColor: '#888888',
                                labelOffsetX: 8,
                                labelOffsetY: 0,
                            });
                            features.push(...metadataFeatures);
                            historyMetadataCount++;
                        }
                    }

                    const hindcastPoints = (this.store.aircraftDetailsHindcastData || [])
                        .filter((position) => typeof position?.lat === 'number' && typeof position?.lon === 'number')
                        .map((position) => [position.lat, position.lon]);
                    const hindcastFeature = this._createTrailFeature(hex, hindcastPoints, {
                        trailType: 'hindcast',
                        color: '#00BCD4',
                        width: 2,
                        lineDash: [4, 6],
                    });
                    if (hindcastFeature) {
                        features.push(hindcastFeature);
                    }

                    const futurePoints = (this.store.aircraftDetailsFutureData || [])
                        .filter((position) => typeof position?.lat === 'number' && typeof position?.lon === 'number')
                        .map((position) => [position.lat, position.lon]);
                    const futureFeature = this._createTrailFeature(hex, futurePoints, {
                        trailType: 'future',
                        color,
                        width: 2,
                        lineDash: [3, 7],
                    });
                    if (futureFeature) {
                        features.push(futureFeature);
                    }

                    const futurePositions = (this.store.aircraftDetailsFutureData || [])
                        .filter((position) => typeof position?.lat === 'number' && typeof position?.lon === 'number');
                    if (futurePositions.length > 0) {
                        let futureMetadataCount = 0;
                        const maxFutureMetadata = 12;

                        futurePositions.forEach((position, index) => {
                            if (futureMetadataCount >= maxFutureMetadata) return;
                            const is15SecondInterval = (index + 1) % 3 === 0;
                            const isLast = index === futurePositions.length - 1;
                            if (!is15SecondInterval && !isLast) return;

                            const timeLabel = (typeof this.store?.formatPredictionTime === 'function')
                                ? this.store.formatPredictionTime(position)
                                : `+${(index + 1) * 5}s`;
                            const altitudeRaw = Number(position?.altitude ?? position?.alt_baro);
                            const altitudeText = Number.isFinite(altitudeRaw)
                                ? (altitudeRaw === 0 ? 'GND' : `${Math.round(altitudeRaw / 100) * 100}`)
                                : '';
                            const labelText = altitudeText ? `${timeLabel} ${altitudeText}` : timeLabel;

                            const metadataFeatures = this._createTrailMetadataFeature(hex, position.lat, position.lon, {
                                trailType: 'future-metadata',
                                radius: isLast ? 4 : 3,
                                fillColor: '#666666',
                                strokeColor: '#666666',
                                labelText,
                                labelColor: '#666666',
                                labelOffsetX: 8,
                                labelOffsetY: 0,
                            });
                            features.push(...metadataFeatures);
                            futureMetadataCount++;
                        });
                    }
                }

                this._replaceTrailFeatures(hex, features);
            });

            this._stats.trailFeatureCount = this._trailSource.getFeatures().length;
        }
        updateTracksMiniMap() {
            if (window.MapMinimapFeature && typeof window.MapMinimapFeature.updateTracksMiniMap === 'function') {
                window.MapMinimapFeature.updateTracksMiniMap(this);
            }
        }
        initTracksMiniMap(containerId, retryCount = 0) {
            if (window.MapMinimapFeature && typeof window.MapMinimapFeature.initTracksMiniMap === 'function') {
                window.MapMinimapFeature.initTracksMiniMap(this, containerId, retryCount);
            }
        }
        cleanupTracksMiniMap() {
            if (window.MapMinimapFeature && typeof window.MapMinimapFeature.cleanupTracksMiniMap === 'function') {
                window.MapMinimapFeature.cleanupTracksMiniMap(this);
            }
        }
        clearAircraftTrails(hex, options = {}) {
            if (!hex) {
                if (this._trailSource) {
                    this._trailSource.clear();
                }
                this._trailFeaturesByHex.clear();
                return;
            }

            this._replaceTrailFeatures(hex, []);

            if (options?.clearHistory === true && this.trails) {
                delete this.trails[hex];
            }
        }
        ensureMapObjects(aircraft) {
            if (!aircraft || !aircraft.hex) return;
            this.updateSingleAircraft(aircraft.hex, aircraft);
        }

        removeStaleMarkers(currentAircraftHexes) {
            const keep = currentAircraftHexes instanceof Set
                ? currentAircraftHexes
                : new Set(Array.isArray(currentAircraftHexes) ? currentAircraftHexes : []);

            if (!this._aircraftRenderer || typeof this._aircraftRenderer.getFeatureHexes !== 'function') {
                return;
            }

            const existingHexes = this._aircraftRenderer.getFeatureHexes();
            existingHexes.forEach((hex) => {
                if (!keep.has(hex)) {
                    this.removeAircraft(hex);
                }
            });
        }

        removeAircraft(hex) {
            if (!hex) return;
            if (this._aircraftRenderer && typeof this._aircraftRenderer.removeAircraftFeature === 'function') {
                this._aircraftRenderer.removeAircraftFeature(hex);
            }
            this._replaceTrailFeatures(hex, []);

            if (this.store?.selectedAircraft?.hex === hex) {
                this.store.selectedAircraft = null;
            }

            if (this.store?.hoveredAircraft?.hex === hex) {
                this.store.hoveredAircraft = null;
            }
        }
        drawRunways(runwayData) {
            this._ensureReferenceLayers();
            this.runwayData = runwayData || null;
            this._renderRunways();
        }
        drawAirports(airports) {
            this._ensureReferenceLayers();
            this.airportsData = Array.isArray(airports) ? airports : [];
            this._renderAirports();
        }
        drawHeliports(heliports) {
            this._ensureReferenceLayers();
            this.heliportsData = Array.isArray(heliports) ? heliports : [];
            this._renderHeliports();
        }
        drawNavaids(navaids) {
            this._ensureReferenceLayers();
            this.navaidsData = Array.isArray(navaids) ? navaids : [];
            this._renderNavaids();
        }
        drawAllRunways(runways) {
            this._ensureReferenceLayers();
            this.allRunwaysData = Array.isArray(runways) ? runways : [];
            this._renderAllRunways();
        }
        showTakeoffLandingEffect() {
            const hex = arguments[0];
            const eventType = arguments[1];
            const phase = arguments[2];

            const aircraft = this.store?.aircraft?.[hex];
            const lat = aircraft?.adsb?.lat;
            const lon = aircraft?.adsb?.lon;
            if (typeof lat !== 'number' || typeof lon !== 'number') return;

            this._ensureOverlayLayer();
            if (!this._overlaySource) return;

            const phaseColor = phase === 'T/O' ? '#60A5FA' : (phase === 'APP' || phase === 'ARR' ? '#F59E0B' : '#4CAF50');
            const center = this._toMapCoordinate(lat, lon);
            const feature = new window.ol.Feature({
                geometry: new window.ol.geom.Circle(center, 400),
            });
            feature.setStyle(new window.ol.style.Style({
                stroke: new window.ol.style.Stroke({ color: phaseColor, width: 2 }),
                fill: new window.ol.style.Fill({ color: 'rgba(76, 175, 80, 0.12)' }),
            }));

            this._overlaySource.addFeature(feature);
            this._effectFeatures.add(feature);

            const startedAt = Date.now();
            const durationMs = 1400;
            const animate = () => {
                if (!this._overlaySource || !this._effectFeatures.has(feature)) return;

                const progress = Math.min(1, (Date.now() - startedAt) / durationMs);
                const radius = 400 + (2200 * progress);
                const alpha = Math.max(0, 0.2 * (1 - progress));
                feature.setGeometry(new window.ol.geom.Circle(center, radius));
                feature.setStyle(new window.ol.style.Style({
                    stroke: new window.ol.style.Stroke({ color: phaseColor, width: 2 }),
                    fill: new window.ol.style.Fill({ color: `rgba(76, 175, 80, ${alpha.toFixed(3)})` }),
                }));

                if (progress < 1) {
                    window.requestAnimationFrame(animate);
                } else {
                    this._overlaySource.removeFeature(feature);
                    this._effectFeatures.delete(feature);
                }
            };

            window.requestAnimationFrame(animate);
        }
        showPositionHighlight(lat, lon) {
            if (typeof lat !== 'number' || typeof lon !== 'number') return;
            this._ensureOverlayLayer();
            if (!this._overlaySource) return;

            this.clearPositionHighlight();

            const point = new window.ol.geom.Point(this._toMapCoordinate(lat, lon));
            const feature = new window.ol.Feature({ geometry: point });
            feature.setStyle(new window.ol.style.Style({
                image: new window.ol.style.Circle({
                    radius: 7,
                    fill: new window.ol.style.Fill({ color: 'rgba(76, 175, 80, 0.95)' }),
                    stroke: new window.ol.style.Stroke({ color: '#FFFFFF', width: 2 }),
                }),
            }));

            this._positionHighlightFeature = feature;
            this._overlaySource.addFeature(feature);
        }
        clearPositionHighlight() {
            if (this._overlaySource && this._positionHighlightFeature) {
                this._overlaySource.removeFeature(this._positionHighlightFeature);
            }
            this._positionHighlightFeature = null;
        }
        enableSimulationPositionMode() { this._simulationPositionMode = true; }
        disableSimulationPositionMode() { this._simulationPositionMode = false; }

        drawProximityCircle(position, distanceNM) {
            if (!Array.isArray(position) || position.length < 2) return;
            const lat = position[0];
            const lon = position[1];
            if (typeof lat !== 'number' || typeof lon !== 'number') return;

            this._ensureOverlayLayer();
            this.removeProximityCircle();

            const radiusMeters = Number(distanceNM) * 1852;
            if (!Number.isFinite(radiusMeters) || radiusMeters <= 0 || !this._overlaySource) return;

            this.proximityRefHex = this.store?.selectedAircraft?.hex || null;
            this.proximityDistanceNM = distanceNM;

            const circle = new window.ol.geom.Circle(this._toMapCoordinate(lat, lon), radiusMeters);
            const feature = new window.ol.Feature({ geometry: circle });
            feature.setStyle(new window.ol.style.Style({
                stroke: new window.ol.style.Stroke({
                    color: '#EB8C00',
                    width: 2,
                    lineDash: [5, 5],
                }),
                fill: new window.ol.style.Fill({ color: 'rgba(235, 140, 0, 0.08)' }),
            }));

            this._proximityCircleFeature = feature;
            this._overlaySource.addFeature(feature);
            this.proximityCircle = { position: [lat, lon], distanceNM };
        }

        removeProximityCircle() {
            if (this._overlaySource && this._proximityCircleFeature) {
                this._overlaySource.removeFeature(this._proximityCircleFeature);
            }
            this._proximityCircleFeature = null;
            this.proximityCircle = null;
            this.proximityRefHex = null;
            this.proximityDistanceNM = null;
        }

        updateProximityCircle() {
            if (!this._proximityCircleFeature || !this.proximityRefHex) return;

            const aircraft = this.store?.aircraft?.[this.proximityRefHex];
            const lat = aircraft?.adsb?.lat;
            const lon = aircraft?.adsb?.lon;
            if (typeof lat !== 'number' || typeof lon !== 'number') return;

            const radiusMeters = Number(this.proximityDistanceNM) * 1852;
            if (!Number.isFinite(radiusMeters) || radiusMeters <= 0) return;

            this._proximityCircleFeature.setGeometry(
                new window.ol.geom.Circle(this._toMapCoordinate(lat, lon), radiusMeters)
            );
        }

        highlightProximityAircraft(proximityHexSet) {
            this.proximityHexSet = proximityHexSet || null;
            this.store.proximityHighlightedAircraft = proximityHexSet instanceof Set ? new Set(proximityHexSet) : new Set();
            this.updateVisualState();
            this.refreshStaleLabels();
        }

        removeProximityHighlighting() {
            this.proximityHexSet = null;
            this.store.proximityHighlightedAircraft = new Set();
            this.updateVisualState();
            this.refreshStaleLabels();
        }

        getPerformanceStats() {
            const rendererStats = this._aircraftRenderer && typeof this._aircraftRenderer.getStats === 'function'
                ? this._aircraftRenderer.getStats()
                : null;

            const avgSingleAircraftUpdateMs = this._stats.singleAircraftUpdates > 0
                ? this._stats.singleAircraftUpdateDurationMs / this._stats.singleAircraftUpdates
                : 0;

            return {
                engine: 'openlayers',
                windowSec: 1,
                refreshPerSec: this._stats.refreshCalls,
                avgRefreshMs: 0,
                coalescedPct: 0,
                markerOpsPerSec: 0,
                initCalls: this._stats.initCalls,
                refreshCalls: this._stats.refreshCalls,
                singleUpdatesPerSec: this._stats.singleAircraftUpdates,
                singleAircraftAvgUpdateMs: Number(avgSingleAircraftUpdateMs.toFixed(3)),
                visualSkipPct: 0,
                trailLayerOpsPerSec: 0,
                fullVisibilityPasses: 0,
                rendererBackend: rendererStats?.backend || 'none',
                rendererFeatureCount: rendererStats?.featureCount || 0,
                rendererUpserts: rendererStats?.upserts || 0,
                rendererRemoves: rendererStats?.removes || 0,
                rendererBulkSyncs: rendererStats?.bulkSyncs || 0,
                rendererAvgUpdateMs: rendererStats?.avgUpdateMs || 0,
                trailFeatureCount: this._stats.trailFeatureCount || 0,
                overlayCount: Array.isArray(this._overlayStatuses) ? this._overlayStatuses.length : 0,
                overlayDegradedCount: Array.isArray(this._overlayStatuses)
                    ? this._overlayStatuses.filter((overlay) => overlay.degraded).length
                    : 0,
            };
        }

        setOverlayVisibility(overlayId, visible) {
            if (!this._overlayRegistry || typeof this._overlayRegistry.setVisible !== 'function') return;
            this._overlayRegistry.setVisible(overlayId, !!visible);
            this._overlayStatuses = this._overlayRegistry.getStatuses();
            if (this._overlayRegistry.updateZoomVisibility) {
                this._overlayRegistry.updateZoomVisibility(this.getZoom());
            }
        }

        setOverlayOpacity(overlayId, opacity) {
            if (!this._overlayRegistry || typeof this._overlayRegistry.setOpacity !== 'function') return;
            this._overlayRegistry.setOpacity(overlayId, opacity);
            this._overlayStatuses = this._overlayRegistry.getStatuses();
        }

        setLayerOpacity(layerName, opacity) {
            const value = Math.max(0, Math.min(1, Number.isFinite(opacity) ? opacity : 1));

            if (layerName === 'rangeRings') {
                if (this._rangeRingLayerState?.layer) {
                    this._rangeRingLayerState.layer.setOpacity(value);
                    this._scheduleReferenceRender();
                }
                return;
            }

            const referenceState = this._referenceLayerState?.[layerName];
            if (referenceState?.layer) {
                referenceState.layer.setOpacity(value);
                this._scheduleReferenceRender();
                return;
            }

            if (this._overlayRegistry && typeof this._overlayRegistry.setOpacity === 'function') {
                this._overlayRegistry.setOpacity(layerName, value);
                this._overlayStatuses = this._overlayRegistry.getStatuses();
            }
        }

        getOverlayStatuses() {
            if (!this._overlayRegistry || typeof this._overlayRegistry.getStatuses !== 'function') {
                return [];
            }
            this._overlayStatuses = this._overlayRegistry.getStatuses();
            return this._overlayStatuses;
        }

        cleanup() {
            if (this._interactionCleanup) {
                this._interactionCleanup();
                this._interactionCleanup = null;
            }
            this.stopLabelRefreshTimer();
            this.cleanupTracksMiniMap();
            const mapRef = this._olMap;
            if (this._aircraftRenderer && typeof this._aircraftRenderer.dispose === 'function') {
                this._aircraftRenderer.dispose();
            }
            this._aircraftRenderer = null;
            if (this._trailLayer && mapRef) {
                mapRef.removeLayer(this._trailLayer);
            }
            if (this._trailSource) {
                this._trailSource.clear();
            }
            this._trailSource = null;
            this._trailLayer = null;
            this._trailFeaturesByHex.clear();
            if (this._overlayLayer && mapRef) {
                mapRef.removeLayer(this._overlayLayer);
            }
            if (this._overlaySource) {
                this._overlaySource.clear();
            }
            this._overlaySource = null;
            this._overlayLayer = null;
            this._positionHighlightFeature = null;
            this._proximityCircleFeature = null;
            Object.keys(this._referenceLayerState).forEach((layerName) => {
                const state = this._referenceLayerState[layerName];
                if (state?.layer && mapRef) {
                    mapRef.removeLayer(state.layer);
                }
                if (state?.source) {
                    state.source.clear();
                }
            });
            this._referenceLayerState = {};
            if (this._rangeRingLayerState?.layer && mapRef) {
                mapRef.removeLayer(this._rangeRingLayerState.layer);
            }
            if (this._rangeRingLayerState?.source) {
                this._rangeRingLayerState.source.clear();
            }
            this._rangeRingLayerState = null;
            this._referenceRenderScheduled = false;
            if (this._overlayRegistry && typeof this._overlayRegistry.dispose === 'function') {
                this._overlayRegistry.dispose();
            }
            this._overlayRegistry = null;
            this._overlayStatuses = [];
            if (this._referencePopupOverlay && mapRef) {
                mapRef.removeOverlay(this._referencePopupOverlay);
            }
            this._referencePopupOverlay = null;
            this._referencePopupElement = null;
            if (this.engine) {
                this.engine.dispose();
                this.engine = null;
            }
            this.map = null;
            this._olMap = null;
            this.markers = {};
            this.trails = {};
            this.proximityHexSet = null;
            this.proximityRefHex = null;
            this.proximityDistanceNM = null;
            this.proximityCircle = null;
        }
    }

    function createOpenLayersMapManager(store, CONFIG) {
        return new OpenLayersMapManager(store, CONFIG);
    }

    window.OpenLayersMapManager = {
        createOpenLayersMapManager,
    };
})();
