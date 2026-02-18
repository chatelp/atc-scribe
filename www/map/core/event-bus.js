/**
 * Module: map/core/event-bus
 * Why it exists:
 * - Provides a tiny dependency-free pub/sub primitive for map internals.
 * - Avoids tight coupling between map features that need lightweight signaling.
 *
 * Key responsibilities:
 * - Register (`on`) and unregister (`off`) event handlers.
 * - Emit payloads safely (`emit`) and support full teardown (`clear`).
 *
 * Quirks / contracts:
 * - Handler exceptions are caught and logged so one faulty subscriber does not
 *   break event delivery to others.
 * - Implemented as a global factory on `window.MapEventBus` to match existing
 *   non-bundled frontend module loading.
 */
(function () {
    function createEventBus() {
        const listeners = new Map();

        function on(eventName, handler) {
            if (!listeners.has(eventName)) {
                listeners.set(eventName, new Set());
            }
            listeners.get(eventName).add(handler);
            return () => off(eventName, handler);
        }

        function off(eventName, handler) {
            const handlers = listeners.get(eventName);
            if (!handlers) return;
            handlers.delete(handler);
            if (handlers.size === 0) {
                listeners.delete(eventName);
            }
        }

        function emit(eventName, payload) {
            const handlers = listeners.get(eventName);
            if (!handlers) return;
            handlers.forEach((handler) => {
                try {
                    handler(payload);
                } catch (error) {
                    console.warn('MapEventBus handler error:', error);
                }
            });
        }

        function clear() {
            listeners.clear();
        }

        return { on, off, emit, clear };
    }

    window.MapEventBus = {
        createEventBus,
    };
})();
