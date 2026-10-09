// Tile cache retired. This worker used to keep the Carto dark tiles, cache
// first and without expiry, which hid that Carto now wants a key and kept its
// "API KEY REQUIRED" tile. The base maps now in use (Plan IGN v2, OSM) ask that
// their own cache headers be honoured, so nothing is cached here any more: on
// activation the old caches are deleted, and requests go straight to the
// network (no fetch handler). It stays registered so that browsers which have
// the old worker replace it with this one.
const CACHE_PREFIX = 'co-atc-tiles-';

self.addEventListener('install', (event) => {
    event.waitUntil(self.skipWaiting());
});

self.addEventListener('activate', (event) => {
    event.waitUntil((async () => {
        const keys = await caches.keys();
        await Promise.all(keys.filter((key) => key.startsWith(CACHE_PREFIX)).map((key) => caches.delete(key)));
        await self.clients.claim();
    })());
});

// The performance panel still asks for cache statistics: there are none.
self.addEventListener('message', (event) => {
    if (!event.data || event.data.type !== 'tile-cache-stats-request') {
        return;
    }

    event.source?.postMessage({
        type: 'tile-cache-stats',
        data: { hits: 0, misses: 0, networkFetches: 0, cacheWrites: 0, lastEventAt: null, cacheName: null }
    });
});
