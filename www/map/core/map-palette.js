/**
 * Module: map/core/map-palette
 * Why it exists:
 * - The map's colours are the design system's tokens (palette C, chosen by
 *   the owner on 15/10: the map keeps its hues, and only what clashed with a
 *   colour that means something changed). --color-map-* on a dark base map,
 *   --color-map-light-* where a bright one (IGN, OSM) needs its own value.
 *
 * Key responsibilities:
 * - color(role): the role's colour for the base map in use, read once from
 *   the page's variables, the tokens' hex as fallbacks for a canvas.
 * - withAlpha(hex, alpha): an rgba() for OpenLayers, whose canvas takes no
 *   color-mix().
 *
 * Quirks / contracts:
 * - The table of roles and why each one is what it is: docs-fr/design/03-systeme.md,
 *   « La palette de la carte (15/10) ».
 */
(function () {
    const FALLBACK = {
        airport: '#60a5fa', 'airport-small': '#a78bfa', heliport: '#a78bfa',
        'navaid-vor': '#60a5fa', 'navaid-vortac': '#818cf8', 'navaid-ndb': '#f59e0b',
        'navaid-dme': '#a78bfa', runway: '#e4e4e4', centerline: '#555555', taxiway: '#5c5c5c',
        ring: '#3d3d3d', 'ring-label': '#949494',
        selected: '#66c56a', hovered: '#e0e0e0',
        'trail-followed': '#66c56a', estimate: '#66c56a', 'trail-stale': '#949494',
        proximity: '#66c56a',
        'light-ring': '#b5b5b5', 'light-centerline': '#a8a8a8', 'light-proximity': '#2e7d32',
        'light-taxiway': '#9a9a9a', 'light-selected': '#2e7d32',
        'light-hovered': '#333333', 'light-trail-stale': '#7a7a7a',
        'text-primary': '#e0e0e0', 'text-secondary': '#b8b8b8', 'text-tertiary': '#949494',
        'text-disabled': '#666666',
    };
    // The same list as aircraft-webgl.js: none, ign-dark and osm-dark are dark.
    const BRIGHT_MAP_STYLES = ['ign', 'osm'];
    const cache = new Map();

    function token(name) {
        if (cache.has(name)) return cache.get(name);
        let v = '';
        if (typeof document !== 'undefined' && document.documentElement) {
            const variable = name.startsWith('text-') ? `--color-${name}` : `--color-map-${name}`;
            v = getComputedStyle(document.documentElement).getPropertyValue(variable).trim();
        }
        v = v || FALLBACK[name] || '#949494';
        cache.set(name, v);
        return v;
    }

    function isBright() {
        const style = window.Alpine?.store?.('atc')?.settings?.mapStyle || 'none';
        return BRIGHT_MAP_STYLES.includes(String(style).toLowerCase());
    }

    function color(role) {
        if (isBright() && FALLBACK[`light-${role}`]) return token(`light-${role}`);
        return token(role);
    }

    function withAlpha(hex, alpha) {
        const m = /^#([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i.exec(hex || '');
        if (!m) return hex;
        return `rgba(${parseInt(m[1], 16)}, ${parseInt(m[2], 16)}, ${parseInt(m[3], 16)}, ${alpha})`;
    }

    window.MapPalette = { color, withAlpha, isBright };
})();
