// The tile cache's service worker (www/sw.js) and the figures the settings
// page shows for it. Moved from app.js as it was (docs-fr/32, palier 5.1);
// tileCacheRuntimeStats and requestTileCacheStatsFromSW are read by app.js
// through the page's global bindings, so this script comes before it.
const tileCacheRuntimeStats = {
    supported: 'serviceWorker' in navigator,
    controlled: false,
    cacheName: null,
    hits: 0,
    misses: 0,
    networkFetches: 0,
    cacheWrites: 0,
    hitRatePct: 0,
    lastEventAt: null
};

function requestTileCacheStatsFromSW() {
    if (!('serviceWorker' in navigator)) return;
    if (!navigator.serviceWorker.controller) return;
    navigator.serviceWorker.controller.postMessage({ type: 'tile-cache-stats-request' });
}

function attachTileCacheSWListeners() {
    if (!('serviceWorker' in navigator)) return;

    navigator.serviceWorker.addEventListener('message', (event) => {
        if (!event.data || event.data.type !== 'tile-cache-stats') return;
        const data = event.data.data || {};

        const hits = Number.isFinite(data.hits) ? data.hits : 0;
        const misses = Number.isFinite(data.misses) ? data.misses : 0;
        const totalLookups = hits + misses;

        tileCacheRuntimeStats.controlled = !!navigator.serviceWorker.controller;
        tileCacheRuntimeStats.cacheName = data.cacheName || tileCacheRuntimeStats.cacheName;
        tileCacheRuntimeStats.hits = hits;
        tileCacheRuntimeStats.misses = misses;
        tileCacheRuntimeStats.networkFetches = Number.isFinite(data.networkFetches) ? data.networkFetches : 0;
        tileCacheRuntimeStats.cacheWrites = Number.isFinite(data.cacheWrites) ? data.cacheWrites : 0;
        tileCacheRuntimeStats.hitRatePct = totalLookups > 0 ? Number(((hits / totalLookups) * 100).toFixed(1)) : 0;
        tileCacheRuntimeStats.lastEventAt = Number.isFinite(data.lastEventAt) ? data.lastEventAt : null;

        if (window.Alpine && Alpine.store('atc')) {
            Alpine.store('atc').tileCacheStats = { ...tileCacheRuntimeStats };
        }
    });

    navigator.serviceWorker.addEventListener('controllerchange', () => {
        tileCacheRuntimeStats.controlled = !!navigator.serviceWorker.controller;
        requestTileCacheStatsFromSW();
    });
}

attachTileCacheSWListeners();

async function registerTileCacheServiceWorker() {
    if (!('serviceWorker' in navigator)) {
        return;
    }

    if (!window.isSecureContext && window.location.hostname !== 'localhost' && window.location.hostname !== '127.0.0.1') {
        console.warn('Service worker tile cache skipped: insecure context');
        return;
    }

    try {
        const registration = await navigator.serviceWorker.register('/sw.js', { scope: '/' });
        console.log('Tile cache service worker registered');

        if (navigator.serviceWorker.controller) {
            console.log('Tile cache service worker is controlling this page');
            tileCacheRuntimeStats.controlled = true;
            requestTileCacheStatsFromSW();
        } else {
            console.log('Tile cache service worker installed, will control after next reload');
            tileCacheRuntimeStats.controlled = false;
        }

        if (registration.waiting) {
            console.log('Tile cache service worker update is waiting to activate');
        }
    } catch (error) {
        console.warn('Tile cache service worker registration failed:', error);
    }
}

window.addEventListener('load', () => {
    registerTileCacheServiceWorker();
});
