const CACHE_PREFIX = 'co-atc-tiles-';
const CACHE_VERSION = 'v2';
const TILE_CACHE = `${CACHE_PREFIX}${CACHE_VERSION}`;
const TILE_URL_PATTERN = /^https:\/\/[a-d]\.basemaps\.cartocdn\.com\/dark_all\//i;
const MAX_TILE_ENTRIES = 1500;

const tileCacheStats = {
    hits: 0,
    misses: 0,
    networkFetches: 0,
    cacheWrites: 0,
    lastEventAt: null
};

self.addEventListener('install', (event) => {
    event.waitUntil(self.skipWaiting());
});

self.addEventListener('activate', (event) => {
    event.waitUntil((async () => {
        const keys = await caches.keys();
        await Promise.all(
            keys
                .filter((key) => key.startsWith(CACHE_PREFIX) && key !== TILE_CACHE)
                .map((key) => caches.delete(key))
        );
        await self.clients.claim();
    })());
});

async function enforceCacheLimit(cache, maxEntries) {
    const requests = await cache.keys();
    if (requests.length <= maxEntries) return;

    const surplus = requests.length - maxEntries;
    for (let i = 0; i < surplus; i++) {
        await cache.delete(requests[i]);
    }
}

async function fetchAndCache(request) {
    tileCacheStats.networkFetches += 1;
    tileCacheStats.lastEventAt = Date.now();

    const response = await fetch(request);
    if (!response) return response;

    if (response.ok || response.type === 'opaque') {
        const cache = await caches.open(TILE_CACHE);
        await cache.put(request, response.clone());
        tileCacheStats.cacheWrites += 1;
        tileCacheStats.lastEventAt = Date.now();
        await enforceCacheLimit(cache, MAX_TILE_ENTRIES);
    }

    return response;
}

self.addEventListener('fetch', (event) => {
    const request = event.request;
    if (request.method !== 'GET') return;

    const url = request.url;
    if (!TILE_URL_PATTERN.test(url)) return;

    event.respondWith((async () => {
        const cache = await caches.open(TILE_CACHE);
        const cached = await cache.match(request);

        if (cached) {
            tileCacheStats.hits += 1;
            tileCacheStats.lastEventAt = Date.now();
            return cached;
        }

        tileCacheStats.misses += 1;
        tileCacheStats.lastEventAt = Date.now();

        try {
            return await fetchAndCache(request);
        } catch {
            return cached || Response.error();
        }
    })());
});

self.addEventListener('message', (event) => {
    if (!event.data || event.data.type !== 'tile-cache-stats-request') {
        return;
    }

    event.source?.postMessage({
        type: 'tile-cache-stats',
        data: {
            ...tileCacheStats,
            cacheName: TILE_CACHE
        }
    });
});
