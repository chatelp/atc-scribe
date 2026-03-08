/**
 * Module: map/renderers/aircraft-webgl
 * Why it exists:
 * - Implements high-volume aircraft rendering and styling for OpenLayers maps.
 * - Encapsulates label composition, visual state styling, and icon colorization.
 *
 * Key responsibilities:
 * - Upsert aircraft features with geometry/style updates.
 * - Build callsign/type/detail labels and phase-aware visual prefixes.
 * - Manage WebGL-capable rendering paths with vector fallback behavior.
 *
 * Quirks / contracts:
 * - Runtime adapts between WebGL and vector rendering based on capability/errors.
 * - Icon colorization is cached by `iconSrc|colorHex` and notifies listeners on completion.
 * - Label density/readability settings are expected to be driven by external store state.
 */
(function () {
    const colorizedAircraftIconCache = new Map();
    const pendingAircraftIconColorizations = new Set();
    const aircraftIconColorizationListeners = new Set();

    function addAircraftIconColorizationListener(listener) {
        if (typeof listener === 'function') {
            aircraftIconColorizationListeners.add(listener);
        }
    }

    function removeAircraftIconColorizationListener(listener) {
        if (typeof listener === 'function') {
            aircraftIconColorizationListeners.delete(listener);
        }
    }

    function notifyAircraftIconColorized() {
        aircraftIconColorizationListeners.forEach((listener) => {
            try {
                listener();
            } catch (error) {
                // ignore listener failures
            }
        });
    }

    function createColorizedAircraftIcon(iconSrc, colorHex) {
        if (!iconSrc || !colorHex) return;

        const cacheKey = `${iconSrc}|${colorHex}`;
        if (colorizedAircraftIconCache.has(cacheKey) || pendingAircraftIconColorizations.has(cacheKey)) {
            return;
        }

        pendingAircraftIconColorizations.add(cacheKey);

        const image = new Image();
        image.decoding = 'async';
        image.onload = () => {
            try {
                const width = image.naturalWidth || image.width || 64;
                const height = image.naturalHeight || image.height || 64;
                const canvas = document.createElement('canvas');
                canvas.width = width;
                canvas.height = height;

                const context = canvas.getContext('2d');
                if (!context) return;

                context.clearRect(0, 0, width, height);
                context.drawImage(image, 0, 0, width, height);
                context.globalCompositeOperation = 'source-atop';
                context.fillStyle = colorHex;
                context.fillRect(0, 0, width, height);
                context.globalCompositeOperation = 'source-over';

                colorizedAircraftIconCache.set(cacheKey, canvas.toDataURL('image/png'));
                notifyAircraftIconColorized();
            } finally {
                pendingAircraftIconColorizations.delete(cacheKey);
            }
        };

        image.onerror = () => {
            pendingAircraftIconColorizations.delete(cacheKey);
        };

        image.src = iconSrc;
    }

    function getColorizedAircraftIconSrc(iconSrc, colorHex) {
        if (!iconSrc || !colorHex) return iconSrc || 'assets/a0.svg';

        const cacheKey = `${iconSrc}|${colorHex}`;
        const cached = colorizedAircraftIconCache.get(cacheKey);
        if (cached) return cached;

        createColorizedAircraftIcon(iconSrc, colorHex);
        return iconSrc;
    }

    function hasWebGLPointsSupport() {
        return !!(window.ol && window.ol.layer && window.ol.layer.WebGLPoints);
    }

    function getAircraftHeading(aircraft, store) {
        if (store && typeof store.getHeadingWithFallback === 'function') {
            const heading = store.getHeadingWithFallback(aircraft);
            if (Number.isFinite(heading)) return heading;
        }

        const adsb = aircraft?.adsb || {};
        if (Number.isFinite(adsb.true_heading)) return adsb.true_heading;
        if (Number.isFinite(adsb.mag_heading)) return adsb.mag_heading;
        if (Number.isFinite(adsb.track)) return adsb.track;
        return 0;
    }

    function getAircraftPhase(aircraft) {
        return window.MapVisibilityRules && window.MapVisibilityRules.getCurrentPhase
            ? window.MapVisibilityRules.getCurrentPhase(aircraft)
            : ((aircraft && aircraft.phase && aircraft.phase.current && aircraft.phase.current.length > 0)
                ? aircraft.phase.current[0].phase
                : 'NEW');
    }

    function getAircraftWakeCategory(aircraft) {
        const category = (aircraft?.adsb?.category || '').toString().toUpperCase().trim();
        return category;
    }

    function normalizeAircraftTypeText(value) {
        const text = (value || '').toString().toUpperCase();
        return text
            .replace(/[\/_.,-]+/g, ' ')
            .replace(/\s+/g, ' ')
            .trim();
    }

    function compactAircraftTypeText(value) {
        return normalizeAircraftTypeText(value).replace(/[^A-Z0-9]/g, '');
    }

    function getAircraftTypeCandidates(aircraft) {
        const rawCandidates = [
            aircraft?.bsdb?.type,
            aircraft?.adsb?.type,
            aircraft?.adsb?.t,
            aircraft?.type,
            aircraft?.aircraft_type,
            aircraft?.model,
        ];

        if (aircraft?.bsdb?.manufacturer && aircraft?.bsdb?.type) {
            rawCandidates.push(`${aircraft.bsdb.manufacturer} ${aircraft.bsdb.type}`);
        }

        const unique = new Set();
        const candidates = [];

        for (const value of rawCandidates) {
            const normalized = normalizeAircraftTypeText(value);
            if (!normalized || unique.has(normalized)) continue;

            unique.add(normalized);
            candidates.push({
                normalized,
                compact: compactAircraftTypeText(normalized),
            });
        }

        return candidates;
    }

    function matchesAircraftTypePattern(candidates, patterns) {
        return candidates.some((candidate) => {
            return patterns.some((pattern) => {
                if (pattern.normalized && pattern.normalized.test(candidate.normalized)) return true;
                if (pattern.compact && pattern.compact.test(candidate.compact)) return true;
                return false;
            });
        });
    }

    function getSpecificAircraftIconClass(aircraft) {
        const candidates = getAircraftTypeCandidates(aircraft);
        if (!candidates.length) return null;
        const match = (patterns) => matchesAircraftTypePattern(candidates, patterns);

        if (match([{ normalized: /\bA\s*3\s*8\s*0\b|\bAIRBUS\s+A\s*380\b/, compact: /A380/ }])) return 'a380';
        if (match([{ normalized: /\bA\s*3\s*4\s*0\b|\bAIRBUS\s+A\s*340\b/, compact: /A340/ }])) return 'a340';
        if (match([{ normalized: /\bA\s*3\s*3\s*0\b|\bAIRBUS\s+A\s*330\b/, compact: /A330/ }])) return 'a330';
        if (match([{ normalized: /\bA\s*3\s*(1[89]|2[01])\b|\bAIRBUS\s+A\s*3\s*(1[89]|2[01])\b|\bA20N\b|\bA21N\b/, compact: /A31[89]|A320|A321|A20N|A21N/ }])) return 'a320';
        if (match([{ normalized: /\bB\s*7\s*8\s*7\b|\bBOEING\s+787\b|\b78[89]\b/, compact: /B787|78[89]|78X/ }])) return 'b787';
        if (match([{ normalized: /\bB\s*7\s*7\s*7\b|\bBOEING\s+777\b|\b77[0-9W]\b/, compact: /B777|77[0-9W]/ }])) return 'b777';
        if (match([{ normalized: /\bB\s*7\s*6\s*7\b|\bBOEING\s+767\b|\b76[0-9]\b/, compact: /B767|76[0-9]/ }])) return 'b767';
        if (match([{ normalized: /\bB\s*7\s*4\s*7\b|\bBOEING\s+747\b|\b74[0-9]\b/, compact: /B747|74[0-9]/ }])) return 'b747';
        if (match([{ normalized: /\bB\s*7\s*3\s*7\b|\bBOEING\s+737\b|\b73[0-9]\b/, compact: /B737|73[0-9]/ }])) return 'b737';
        if (match([{ normalized: /\bC\s*1\s*3\s*0\b|\bHERCULES\b/, compact: /C130|HERCULES/ }])) return 'c130';
        if (match([{ normalized: /\bCRJ\b|\bCANADAIR\s+REGIONAL\s+JET\b|\bCL\s*[- ]?65\b/, compact: /CRJ|CL65/ }])) return 'crjx';
        if (match([{ normalized: /\bDH\s*8\s*A\b|\bDHC\s*8\b|\bDASH\s*8\b|\bQ\s*400\b/, compact: /DH8A|DHC8|DASH8|Q400/ }])) return 'dh8a';
        if (match([{ normalized: /\bE\s*1\s*9\s*5\b/, compact: /E195/ }])) return 'e195';
        if (match([{ normalized: /\bERJ\b|\bEMBRAER\s+REGIONAL\s+JET\b|\bE\s*1\s*7\s*[05]\b/, compact: /ERJ|E170|E175/ }])) return 'erj';
        if (match([{ normalized: /\bF\s*1\s*0\s*0\b|\bFOKKER\s*100\b/, compact: /F100|FOKKER100/ }])) return 'f100';
        if (match([{ normalized: /\bFA\s*7\s*X\b|\bFALCON\s*7\s*X\b/, compact: /FA7X|FALCON7X/ }])) return 'fa7x';
        if (match([{ normalized: /\bGLF\s*5\b|\bGULFSTREAM\s*(V|5|G\s*5\s*0\s*0)\b/, compact: /GLF5|GULFSTREAMV|GULFSTREAM5|G500|GV/ }])) return 'glf5';
        if (match([{ normalized: /\bLEAR\s*JET\b|\bLEARJET\b|\bLJ\s*\d{2}\b/, compact: /LEARJET|LJ\d{2}/ }])) return 'learjet';
        if (match([{ normalized: /\bMD\s*1\s*1\b|\bMCDONNELL\s+DOUGLAS\s+11\b/, compact: /MD11/ }])) return 'md11';
        if (match([{ normalized: /\bCESSNA\b|\bC\s*1\s*5\s*2\b|\bC\s*1\s*7\s*2\b|\bC\s*2\s*0\s*8\b/, compact: /CESSNA|C152|C172|C208/ }])) return 'cessna';

        return null;
    }

    function getAircraftIconClass(aircraft) {
        const specificTypeClass = getSpecificAircraftIconClass(aircraft);
        if (specificTypeClass) return specificTypeClass;

        const category = getAircraftWakeCategory(aircraft);
        if (category === 'A1') return 'a1';
        if (category === 'A2') return 'a2';
        if (category === 'A3') return 'a3';
        if (category === 'A4') return 'a4';
        if (category === 'A5') return 'a5';
        if (category === 'A6') return 'a6';
        if (category === 'A7') return 'a7';
        return 'unknown';
    }

    function getAircraftAssetPath(iconClass) {
        switch (iconClass) {
            case 'a1': return 'assets/a1.svg';
            case 'a2': return 'assets/a2.svg';
            case 'a3': return 'assets/a3.svg';
            case 'a4': return 'assets/a4.svg';
            case 'a5': return 'assets/a5.svg';
            case 'a6': return 'assets/a6.svg';
            case 'a7': return 'assets/a7.svg';
            case 'a320': return 'assets/aircraft/a320.svg';
            case 'a330': return 'assets/aircraft/a330.svg';
            case 'a340': return 'assets/aircraft/a340.svg';
            case 'a380': return 'assets/aircraft/a380.svg';
            case 'b737': return 'assets/aircraft/b737.svg';
            case 'b747': return 'assets/aircraft/b747.svg';
            case 'b767': return 'assets/aircraft/b767.svg';
            case 'b777': return 'assets/aircraft/b777.svg';
            case 'b787': return 'assets/aircraft/b787.svg';
            case 'c130': return 'assets/aircraft/c130.svg';
            case 'cessna': return 'assets/aircraft/cessna.svg';
            case 'crjx': return 'assets/aircraft/crjx.svg';
            case 'dh8a': return 'assets/aircraft/dh8a.svg';
            case 'e195': return 'assets/aircraft/e195.svg';
            case 'erj': return 'assets/aircraft/erj.svg';
            case 'f100': return 'assets/aircraft/f100.svg';
            case 'fa7x': return 'assets/aircraft/fa7x.svg';
            case 'glf5': return 'assets/aircraft/glf5.svg';
            case 'learjet': return 'assets/aircraft/learjet.svg';
            case 'md11': return 'assets/aircraft/md11.svg';
            case 'unknown':
            default:
                return 'assets/a0.svg';
        }
    }

    function getAircraftIconSize(iconClass) {
        const scale = 0.805;
        const scaled = (size) => Math.max(18, Math.round(size * scale));
        switch (iconClass) {
            case 'a1': return scaled(26);
            case 'a2': return scaled(30);
            case 'a3': return scaled(33);
            case 'a4': return scaled(36);
            case 'a5': return scaled(38);
            case 'a6':
            case 'a7': return scaled(31);
            case 'unknown': return scaled(30);
            default: return scaled(34);
        }
    }

    function getAltitudeColorBand(aircraft, store) {
        const altitude = Number(aircraft?.adsb?.alt_baro);
        if (!Number.isFinite(altitude)) return 'cruise';

        const configuredCruise = Number(store?.stationCruiseAltitudeFt);
        const cruiseAltitudeFt = Number.isFinite(configuredCruise) && configuredCruise > 0
            ? configuredCruise
            : 18000;

        if (altitude >= cruiseAltitudeFt) return 'cruise';
        if (altitude > 12500) return 'high';
        if (altitude >= 3000) return 'mid';
        if (altitude >= 1000) return 'low';
        return 'very-low';
    }

    function getAltitudeBandColor(aircraft, store) {
        const band = getAltitudeColorBand(aircraft, store);
        const isBrightMap = (store?.settings?.mapStyle || 'dark').toLowerCase() !== 'dark';

        if (isBrightMap) {
            switch (band) {
                case 'high': return '#0E7490';
                case 'mid': return '#B45309';
                case 'low': return '#C2410C';
                case 'very-low': return '#B91C1C';
                case 'cruise':
                default:
                    return '#1D4ED8';
            }
        }

        switch (band) {
            case 'high': return '#F8D857';
            case 'mid': return '#F4B55A';
            case 'low': return '#EA8A4A';
            case 'very-low': return '#D35A48';
            case 'cruise':
            default:
                return '#E8ECEF';
        }
    }

    function getAircraftSymbolColor(aircraft, visualState, store) {
        return getAltitudeBandColor(aircraft, store);
    }

    function hexToRgba(hexColor, alpha) {
        if (typeof hexColor !== 'string' || !hexColor.startsWith('#')) {
            return `rgba(76, 175, 80, ${alpha})`;
        }

        const normalized = hexColor.length === 4
            ? `#${hexColor[1]}${hexColor[1]}${hexColor[2]}${hexColor[2]}${hexColor[3]}${hexColor[3]}`
            : hexColor;

        const r = parseInt(normalized.slice(1, 3), 16);
        const g = parseInt(normalized.slice(3, 5), 16);
        const b = parseInt(normalized.slice(5, 7), 16);
        if ([r, g, b].some((value) => Number.isNaN(value))) {
            return `rgba(76, 175, 80, ${alpha})`;
        }
        return `rgba(${r}, ${g}, ${b}, ${alpha})`;
    }

    function getAircraftShortIcaoType(aircraft) {
        const candidates = [
            aircraft?.bsdb?.icao_type_code,
            aircraft?.adsb?.t,
        ];

        for (const candidate of candidates) {
            const raw = (candidate || '').toString().toUpperCase().trim();
            if (!raw) continue;

            const compact = raw.replace(/[^A-Z0-9]/g, '');
            if (!compact) continue;

            if (compact.length <= 6) return compact;
            return compact.slice(0, 6);
        }

        return '';
    }

    function getAltitudeTrendArrow(aircraft) {
        const baroRate = Number(aircraft?.adsb?.baro_rate);
        if (!Number.isFinite(baroRate)) return '↔';
        if (baroRate > 100) return '↑';
        if (baroRate < -100) return '↓';
        return '↔';
    }

    function getPhaseLabelColor(phase) {
        const phaseColorMap = {
            'NEW': '#9CA3AF',
            'TAX': '#C084FC',
            'T/O': '#FB923C',
            'CLB': '#A3E635',
            'DEP': '#4ADE80',
            'CRZ': '#60A5FA',
            'ARR': '#F9A8D4',
            'APP': '#FACC15',
            'T/D': '#2DD4BF',
            'UNK': '#94A3B8',
        };
        return phaseColorMap[phase] || '#9CA3AF';
    }

    function getAircraftLabelParts(aircraft, store) {
        if (!aircraft) {
            return {
                phaseText: '',
                phaseColor: '#9CA3AF',
                callsignText: '',
                typeText: '',
                detailsText: '',
                ageText: '',
            };
        }

        const callsign = (aircraft.flight || aircraft.hex || '').trim();
        const phase = (aircraft?.phase?.current?.[0]?.phase || '').toString().trim().toUpperCase();
        const phaseText = phase ? `[${phase}] ` : '';
        const phaseColor = getPhaseLabelColor(phase || 'NEW');
        const shortType = getAircraftShortIcaoType(aircraft);
        const altitude = Number.isFinite(aircraft?.adsb?.alt_baro)
            ? (aircraft.adsb.alt_baro === 0 ? 'GND' : `${Math.round(aircraft.adsb.alt_baro)}ft`)
            : '-';
        const trendArrow = getAltitudeTrendArrow(aircraft);
        const speed = Number.isFinite(aircraft?.adsb?.gs)
            ? `${Math.round(aircraft.adsb.gs)}kt`
            : '--kt';

        let ageSeconds = null;
        if (store && typeof store.getSecondsSinceLastSeen === 'function') {
            const fromStore = store.getSecondsSinceLastSeen(aircraft);
            if (Number.isFinite(fromStore)) {
                ageSeconds = fromStore;
            }
        }

        if (ageSeconds === null && aircraft.last_seen) {
            const lastSeenMs = new Date(aircraft.last_seen).getTime();
            if (Number.isFinite(lastSeenMs) && lastSeenMs > 0) {
                ageSeconds = Math.floor((Date.now() - lastSeenMs) / 1000);
            }
        }

        return {
            phaseText,
            phaseColor,
            callsignText: callsign,
            typeText: shortType ? ` ${shortType}` : '',
            detailsText: `${altitude} ${trendArrow} ${speed}`.trim(),
            ageText: (ageSeconds === null || !Number.isFinite(ageSeconds) || ageSeconds < 5)
                ? ''
                : `-${Math.max(0, ageSeconds)}s`,
        };
    }

    function createAircraftWebGLRenderer(map, options) {
        const zIndex = Number.isFinite(options?.zIndex) ? options.zIndex : 400;
        const store = options?.store || null;
        const WEBGL_DISABLE_SESSION_KEY = 'coatc.aircraftRenderer.webglDisabled';
        const preferWebGL = options?.preferWebGL === true;

        const source = new window.ol.source.Vector();
        const featureIndex = new Map();
        const styleCache = new Map();

        let layer = null;
        let backend = 'vector';
        let runtimeErrorHandler = null;
        let runtimeRejectionHandler = null;
        let iconColorizationListener = null;

        const stats = {
            upserts: 0,
            removes: 0,
            bulkSyncs: 0,
            updateDurationMs: 0,
            backend: 'vector',
        };

        function getStyleKey(properties) {
            const selected = properties.selected ? 1 : 0;
            const hovered = properties.hovered ? 1 : 0;
            const color = properties.color || '#4CAF50';
            const rotationRad = Number.isFinite(properties.rotationRad) ? properties.rotationRad : 0;
            const iconSrc = properties.iconSrc || 'assets/a0.svg';
            const iconSize = Number.isFinite(properties.iconSize) ? properties.iconSize : 24;
            const labelColor = properties.labelColor || '#9CA3AF';
            const deemphasized = properties.deemphasized ? 1 : 0;
            return `${selected}_${hovered}_${deemphasized}_${color}_${labelColor}_${iconSrc}_${iconSize}_${rotationRad.toFixed(3)}`;
        }

        function getVectorStyle(feature) {
            const properties = feature.getProperties();
            const aircraft = store?.aircraft?.[properties.hex] || null;
            const labelParts = (store?.settings?.showLabels === false)
                ? { callsignText: '', typeText: '', detailsText: '', ageText: '' }
                : getAircraftLabelParts(aircraft, store);

            const isActiveAircraft = aircraft?.status === 'active';
            let labelColor = isActiveAircraft ? '#4CAF50' : '#FFFFFF';
            let staleAgeSeconds = 0;
            let staleAgeFromStore = false;
            if (store && typeof store.getSecondsSinceLastSeen === 'function') {
                const fromStore = store.getSecondsSinceLastSeen(aircraft);
                if (Number.isFinite(fromStore)) {
                    staleAgeSeconds = Math.max(0, fromStore);
                    staleAgeFromStore = true;
                }
            }
            if (!staleAgeFromStore) {
                staleAgeSeconds = aircraft?.last_seen
                    ? Math.max(0, Math.floor((Date.now() - new Date(aircraft.last_seen).getTime()) / 1000))
                    : 0;
            }

            const labelAgeFade = (() => {
                if (!Number.isFinite(staleAgeSeconds) || staleAgeSeconds <= 10) return 1;
                if (staleAgeSeconds >= 60) return 0.5;
                const progress = (staleAgeSeconds - 10) / 50;
                return 1 - (progress * 0.5);
            })();

            const key = getStyleKey(properties);
            const isSelected = !!properties.selected;
            const isHovered = !!properties.hovered;
            const deemphasized = !!properties.deemphasized;
            const currentMapStyle = (store?.settings?.mapStyle || 'dark').toLowerCase();
            const isBrightMapStyle = currentMapStyle !== 'dark';
            const baseTextOpacity = deemphasized
                ? (isBrightMapStyle ? 0.72 : 0.5)
                : (isBrightMapStyle ? 1 : 0.9);
            const textOpacity = baseTextOpacity * labelAgeFade;
            const labelLineOne = `${labelParts.phaseText || ''}${labelParts.callsignText || ''}${labelParts.typeText || ''}`;
            const labelLineTwo = `${labelParts.detailsText || ''}`;
            const longestLabelLineLength = Math.max(labelLineOne.length, labelLineTwo.length);
            const estimatedLabelWidthPx = Math.max(72, Math.round(longestLabelLineLength * 6.4) + 12);
            const labelBorderColor = isBrightMapStyle
                ? hexToRgba('#111827', (deemphasized ? 0.7 : 0.85) * labelAgeFade)
                : hexToRgba('#9CA3AF', 0.45 * labelAgeFade);
            const labelBackgroundColor = isBrightMapStyle
                ? hexToRgba('#111827', (deemphasized ? 0.72 : 0.82) * labelAgeFade)
                : hexToRgba('#000000', 0.45 * labelAgeFade);

            const iconSize = Number.isFinite(properties.iconSize) ? properties.iconSize : 24;
            const rotationRad = Number.isFinite(properties.rotationRad) ? properties.rotationRad : 0;
            const markerAlpha = deemphasized ? 0.5 : 1;
            const iconColor = properties.color || '#E8ECEF';
            const iconSrc = getColorizedAircraftIconSrc(properties.iconSrc || 'assets/a0.svg', iconColor);
            const showHalo = isSelected || isHovered;
            const haloColor = isSelected ? '#FFFFFF' : '#9CA3AF';
            const haloFillAlpha = deemphasized ? 0.12 : 0.2;
            const haloStrokeAlpha = deemphasized ? 0.4 : 0.85;
            const haloRadius = Math.max(12, Math.round(iconSize * (isSelected ? 1.08 : 0.96)));

            let symbolStyles = styleCache.get(key);
            if (!symbolStyles) {
                const iconStyle = new window.ol.style.Style({
                    image: new window.ol.style.Icon({
                        src: iconSrc,
                        width: iconSize,
                        height: iconSize,
                        anchor: [0.5, 0.5],
                        anchorXUnits: 'fraction',
                        anchorYUnits: 'fraction',
                        rotation: rotationRad,
                        rotateWithView: false,
                        opacity: markerAlpha,
                    }),
                });

                const haloStyle = showHalo
                    ? new window.ol.style.Style({
                        image: new window.ol.style.Circle({
                            radius: haloRadius,
                            fill: new window.ol.style.Fill({ color: hexToRgba(haloColor, haloFillAlpha) }),
                            stroke: new window.ol.style.Stroke({
                                color: hexToRgba(haloColor, haloStrokeAlpha),
                                width: isSelected ? 2.2 : 1.8,
                                lineDash: isSelected ? null : [5, 5],
                            }),
                        }),
                    })
                    : null;

                symbolStyles = haloStyle
                    ? [haloStyle, iconStyle]
                    : [iconStyle];
                styleCache.set(key, symbolStyles);
            }

            const labelMainStyle = new window.ol.style.Style({
                text: new window.ol.style.Text({
                    text: labelParts.callsignText
                        ? `${labelParts.phaseText || ''}${labelParts.callsignText}${labelParts.typeText}\n${labelParts.detailsText}`
                        : '',
                    font: '11px ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
                    offsetY: -(iconSize / 2 + 8),
                    textAlign: 'left',
                    fill: new window.ol.style.Fill({ color: hexToRgba(labelColor, textOpacity) }),
                    backgroundFill: new window.ol.style.Fill({ color: labelBackgroundColor }),
                    backgroundStroke: new window.ol.style.Stroke({ color: labelBorderColor, width: 1 }),
                    padding: [2, 6, 2, 6],
                }),
            });

            const phasePrefixStyle = labelParts.phaseText
                ? new window.ol.style.Style({
                    text: new window.ol.style.Text({
                        text: labelParts.phaseText,
                        font: '700 11px ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
                        offsetY: -(iconSize / 2 + 14),
                        textAlign: 'left',
                        fill: new window.ol.style.Fill({ color: hexToRgba(labelParts.phaseColor || '#9CA3AF', textOpacity) }),
                    }),
                })
                : null;

            const ageTextOffsetX = Math.max(36, estimatedLabelWidthPx - 2);
            const ageTextOffsetY = Math.max(3, Math.round(iconSize * 0.18));

            const labelAgeStyle = labelParts.ageText
                ? new window.ol.style.Style({
                    text: new window.ol.style.Text({
                        text: labelParts.ageText,
                        font: '700 10px ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
                        offsetY: ageTextOffsetY,
                        offsetX: ageTextOffsetX,
                        textAlign: 'right',
                        fill: new window.ol.style.Fill({ color: hexToRgba('#9CA3AF', (deemphasized ? 0.68 : 0.9) * labelAgeFade) }),
                    }),
                })
                : null;

            const textStyles = [labelMainStyle];
            if (phasePrefixStyle) textStyles.push(phasePrefixStyle);

            return labelAgeStyle
                ? [...symbolStyles, ...textStyles, labelAgeStyle]
                : [...symbolStyles, ...textStyles];
        }

        function createWebGLLayer() {
            return new window.ol.layer.WebGLPoints({
                source,
                style: {
                    symbol: {
                        symbolType: 'circle',
                        size: 12,
                        color: '#4CAF50',
                        rotateWithView: false,
                        rotation: 0,
                        opacity: 1,
                    },
                },
                disableHitDetection: false,
            });
        }

        function createVectorLayer() {
            return new window.ol.layer.Vector({
                source,
                style: getVectorStyle,
                declutter: false,
                updateWhileAnimating: true,
                updateWhileInteracting: true,
            });
        }

        function isShaderFailureMessage(message) {
            if (!message || typeof message !== 'string') return false;
            const lower = message.toLowerCase();
            return lower.includes('fragment shader compilation failed') ||
                lower.includes('vertex shader compilation failed') ||
                lower.includes('shader compilation failed');
        }

        function isWebGLRuntimeFailureMessage(message) {
            if (!message || typeof message !== 'string') return false;
            const lower = message.toLowerCase();
            return isShaderFailureMessage(message) ||
                lower.includes("cannot read properties of undefined (reading 'ol_uid')") ||
                lower.includes("cannot read properties of undefined (reading 'readpixel')") ||
                lower.includes('pointslayer.js') ||
                lower.includes('getuniformlocation') ||
                lower.includes('prepareframeinternal');
        }

        function markWebGLDisabledForSession() {
            try {
                if (window.sessionStorage) {
                    window.sessionStorage.setItem(WEBGL_DISABLE_SESSION_KEY, '1');
                }
            } catch (error) {
                // ignore storage failures
            }
        }

        function isWebGLDisabledForSession() {
            try {
                return !!(window.sessionStorage && window.sessionStorage.getItem(WEBGL_DISABLE_SESSION_KEY) === '1');
            } catch (error) {
                return false;
            }
        }

        function fallbackToVector(reason) {
            if (!map || backend !== 'webgl') return;

            console.warn('[MapAircraftWebGLRenderer] Falling back to vector renderer:', reason || 'unknown reason');

            if (layer) {
                map.removeLayer(layer);
            }

            layer = createVectorLayer();
            backend = 'vector';
            stats.backend = backend;
            markWebGLDisabledForSession();
            layer.setZIndex(zIndex);
            map.addLayer(layer);
            layer.changed();
        }

        function attachRuntimeWebGLErrorGuard() {
            runtimeErrorHandler = (event) => {
                const message = event && event.message ? event.message : '';
                if (isWebGLRuntimeFailureMessage(message)) {
                    fallbackToVector(message);
                }
            };

            runtimeRejectionHandler = (event) => {
                const reason = event && event.reason ? event.reason : null;
                const message = typeof reason === 'string'
                    ? reason
                    : (reason && reason.message ? reason.message : '');
                if (isWebGLRuntimeFailureMessage(message)) {
                    fallbackToVector(message);
                }
            };

            window.addEventListener('error', runtimeErrorHandler);
            window.addEventListener('unhandledrejection', runtimeRejectionHandler);
        }

        function detachRuntimeWebGLErrorGuard() {
            if (runtimeErrorHandler) {
                window.removeEventListener('error', runtimeErrorHandler);
                runtimeErrorHandler = null;
            }
            if (runtimeRejectionHandler) {
                window.removeEventListener('unhandledrejection', runtimeRejectionHandler);
                runtimeRejectionHandler = null;
            }
        }

        function init() {
            if (!map) return;

            iconColorizationListener = () => {
                styleCache.clear();
                if (layer) {
                    layer.changed();
                }
            };
            addAircraftIconColorizationListener(iconColorizationListener);

            if (isWebGLDisabledForSession()) {
                layer = createVectorLayer();
                backend = 'vector';
            } else if (preferWebGL && hasWebGLPointsSupport()) {
                try {
                    layer = createWebGLLayer();
                    backend = 'webgl';
                    attachRuntimeWebGLErrorGuard();
                } catch (error) {
                    console.warn('[MapAircraftWebGLRenderer] WebGLPoints init failed, falling back to vector layer:', error);
                    layer = createVectorLayer();
                    backend = 'vector';
                    markWebGLDisabledForSession();
                }
            } else {
                layer = createVectorLayer();
                backend = 'vector';
            }

            stats.backend = backend;
            layer.setZIndex(zIndex);
            map.addLayer(layer);
        }

        function dispose() {
            if (!map || !layer) return;
            if (iconColorizationListener) {
                removeAircraftIconColorizationListener(iconColorizationListener);
                iconColorizationListener = null;
            }
            detachRuntimeWebGLErrorGuard();
            map.removeLayer(layer);
            layer = null;
            featureIndex.clear();
            styleCache.clear();
            source.clear();
        }

        function createFeatureProps(aircraft, visualState) {
            const headingDeg = getAircraftHeading(aircraft, store);
            const headingRad = (headingDeg * Math.PI) / 180;
            const phase = getAircraftPhase(aircraft);
            const color = getAircraftSymbolColor(aircraft, visualState, store);
            const iconClass = getAircraftIconClass(aircraft);
            const iconSrc = getAircraftAssetPath(iconClass);
            const iconSize = getAircraftIconSize(iconClass);

            return {
                hex: aircraft.hex,
                lat: aircraft.adsb.lat,
                lon: aircraft.adsb.lon,
                heading: headingDeg,
                rotationRad: headingRad,
                on_ground: !!aircraft.on_ground,
                status: aircraft.status || 'active',
                phase,
                selected: !!visualState.selected,
                hovered: !!visualState.hovered,
                proximity: !!visualState.proximity,
                deemphasized: !!visualState.deemphasized,
                visible: true,
                color,
                iconClass,
                iconSrc,
                iconSize,
            };
        }

        function upsertAircraftFeature(aircraft, visualState, options = {}) {
            if (!aircraft || !aircraft.hex || !aircraft.adsb) return;
            const lat = aircraft.adsb.lat;
            const lon = aircraft.adsb.lon;
            const hasPosition = typeof lat === 'number' && typeof lon === 'number' && (lat !== 0 || lon !== 0);
            if (!hasPosition) return;

            const startedAt = performance.now();

            let feature = featureIndex.get(aircraft.hex);
            const geometry = new window.ol.geom.Point(window.ol.proj.fromLonLat([lon, lat]));
            const props = createFeatureProps(aircraft, visualState || { selected: false, hovered: false });
            const preservePose = options?.preservePose === true;

            if (!feature) {
                feature = new window.ol.Feature({ geometry });
                feature.setProperties(props);
                source.addFeature(feature);
                featureIndex.set(aircraft.hex, feature);
            } else {
                if (!preservePose) {
                    feature.setGeometry(geometry);
                } else {
                    const currentHeading = Number(feature.get('heading'));
                    const currentRotationRad = Number(feature.get('rotationRad'));
                    if (Number.isFinite(currentHeading)) {
                        props.heading = currentHeading;
                    }
                    if (Number.isFinite(currentRotationRad)) {
                        props.rotationRad = currentRotationRad;
                    }
                }
                feature.setProperties(props);
                feature.changed();
            }

            stats.upserts++;
            stats.updateDurationMs += (performance.now() - startedAt);
        }

        function removeAircraftFeature(hex) {
            if (!hex) return;
            const feature = featureIndex.get(hex);
            if (!feature) return;
            source.removeFeature(feature);
            featureIndex.delete(hex);
            stats.removes++;
        }

        function bulkSyncAircraft(aircraftMap, shouldRender, visualResolver) {
            const activeHexes = new Set();
            const collection = aircraftMap || {};

            Object.keys(collection).forEach((hex) => {
                const aircraft = collection[hex];
                if (!aircraft || !aircraft.hex) return;

                const visible = typeof shouldRender === 'function' ? !!shouldRender(aircraft) : true;
                if (!visible) {
                    removeAircraftFeature(aircraft.hex);
                    return;
                }

                const visualState = typeof visualResolver === 'function'
                    ? (visualResolver(aircraft.hex) || { selected: false, hovered: false })
                    : { selected: false, hovered: false };

                upsertAircraftFeature(aircraft, visualState);
                activeHexes.add(aircraft.hex);
            });

            Array.from(featureIndex.keys()).forEach((hex) => {
                if (!activeHexes.has(hex)) {
                    removeAircraftFeature(hex);
                }
            });

            stats.bulkSyncs++;
        }

        function setVisualState(selectedHex, hoveredHex) {
            featureIndex.forEach((feature, hex) => {
                const selected = !!selectedHex && selectedHex === hex;
                const hovered = !!hoveredHex && hoveredHex === hex;
                const proximity = !!(store?.proximityHighlightedAircraft && store.proximityHighlightedAircraft.has(hex));
                const deemphasized = !!selectedHex && !selected;
                feature.set('selected', selected);
                feature.set('hovered', hovered);
                feature.set('proximity', proximity);
                feature.set('deemphasized', deemphasized);

                const aircraft = store?.aircraft?.[hex] || null;
                feature.set('color', getAircraftSymbolColor(aircraft, { selected, hovered, proximity }, store));
                feature.changed();
            });

            if (layer) {
                layer.changed();
            }
        }

        function refreshAircraftStyle(hex, selectedHex, hoveredHex) {
            const feature = featureIndex.get(hex);
            if (!feature) return;

            const selected = !!selectedHex && selectedHex === hex;
            const hovered = !!hoveredHex && hoveredHex === hex;
            const proximity = !!(store?.proximityHighlightedAircraft && store.proximityHighlightedAircraft.has(hex));
            const deemphasized = !!selectedHex && !selected;
            feature.set('selected', selected);
            feature.set('hovered', hovered);
            feature.set('proximity', proximity);
            feature.set('deemphasized', deemphasized);

            const aircraft = store?.aircraft?.[hex] || null;
            feature.set('color', getAircraftSymbolColor(aircraft, { selected, hovered, proximity }, store));
            feature.changed();

            if (layer) {
                layer.changed();
            }
        }

        function getFeature(hex) {
            return featureIndex.get(hex) || null;
        }

        function getFeatureHexes() {
            return new Set(featureIndex.keys());
        }

        function getLayer() {
            return layer;
        }

        function getStats() {
            const avgUpdateMs = stats.upserts > 0 ? stats.updateDurationMs / stats.upserts : 0;
            return {
                backend: stats.backend,
                featureCount: featureIndex.size,
                upserts: stats.upserts,
                removes: stats.removes,
                bulkSyncs: stats.bulkSyncs,
                avgUpdateMs: Number(avgUpdateMs.toFixed(3)),
            };
        }

        function requestRedraw() {
            if (layer) {
                layer.changed();
            }
        }

        return {
            init,
            dispose,
            upsertAircraftFeature,
            removeAircraftFeature,
            bulkSyncAircraft,
            setVisualState,
            refreshAircraftStyle,
            requestRedraw,
            getFeature,
            getFeatureHexes,
            getLayer,
            getStats,
        };
    }

    window.MapAircraftWebGLRenderer = {
        createAircraftWebGLRenderer,
    };
})();
