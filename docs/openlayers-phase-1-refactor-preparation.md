# OpenLayers Migration — Phase 1 Runbook (Refactor Preparation)

**Status:** Ready to execute
**Phase window:** 4–6 days
**Pre-req:** Baseline captured in `docs/openlayers-migration-baseline.md`
**Parent RFC:** `docs/openlayers-full-migration-rfc.md`

---

## 1) Phase Objective

Refactor current map code into modular boundaries **without changing user-visible behavior**.

This phase does **not** introduce OpenLayers yet. It prepares the codebase so Phase 2+ can be implemented safely.

---

## 2) Required Outcomes

By end of Phase 1, all must be true:

1. `www/map-manager.js` is decomposed into focused modules.
2. State derivation is separated from rendering side-effects.
3. A layer registry abstraction exists and is used for map layers.
4. A map event bus exists for map/store interaction wiring.
5. Existing Leaflet runtime behavior is unchanged.

---

## 3) Scope Boundaries

### In scope
- Frontend map code structure and internal APIs
- Moving logic into new files under `www/map/`
- Introducing compatibility wrappers to preserve current app contract

### Out of scope
- Replacing Leaflet with OpenLayers (Phase 2)
- Net-new user features
- Backend API changes

---

## 4) Proposed Module Structure (Phase 1 version)

Create these folders/files and migrate logic incrementally:

- `www/map/core/layer-registry.js`
- `www/map/core/event-bus.js`
- `www/map/core/visibility-rules.js`
- `www/map/renderers/aircraft-renderer.js` (Leaflet-backed, temporary)
- `www/map/renderers/trails-renderer.js` (Leaflet-backed, temporary)
- `www/map/renderers/reference-renderer.js` (Leaflet-backed, temporary)
- `www/map/features/proximity-feature.js`
- `www/map/features/interactions-feature.js`
- `www/map/features/minimap-feature.js`
- `www/map/perf/telemetry.js`
- `www/map/leaflet-map-manager.js` (orchestrator, keeps current public methods)

During this phase, `www/map-manager.js` may become a thin compatibility facade that forwards to `leaflet-map-manager.js`.

---

## 5) Public Contract to Preserve

The app currently depends on the map manager API. Keep these methods stable:

- `initMap()`
- `applyFiltersAndRefreshView(options)`
- `updateSingleAircraft(hex, aircraft)`
- `toggleRings()`
- `getPerformanceStats()`
- selection/proximity helpers used by `www/app.js`

No signature changes in Phase 1 unless strictly necessary.

---

## 6) Work Plan (Execution Sequence)

## Step 1 — Introduce Scaffolding (Day 1)

### Tasks
- Create `www/map/` folders and empty modules.
- Add minimal exports/imports and wire bundling/loading order.
- Keep existing behavior fully in `www/map-manager.js` initially.

### Checkpoint
- App boots normally.
- No runtime errors.

---

## Step 2 — Extract Perf + Utility Layers (Day 1–2)

### Tasks
- Move perf counters/stats logic to `www/map/perf/telemetry.js`.
- Move pure helper logic (phase derivation, filter predicates) to `visibility-rules.js`.
- Keep render calls in manager.

### Checkpoint
- `getPerformanceStats()` output matches prior behavior.
- Visibility logic parity confirmed.

---

## Step 3 — Extract Layer Registry + Event Bus (Day 2)

### Tasks
- Add layer registry with one source of truth for:
  - aircraft
   - trails
  - rangeRings
  - runways
  - airports
  - heliports
  - navaids
  - allRunways
- Add event bus for map interaction events and store-triggered refresh requests.

### Checkpoint
- Layer add/remove/toggle behavior unchanged.
- No duplicate map event handlers.

---

## Step 4 — Extract Feature Modules (Day 3–4)

### Tasks
- Move proximity logic to `proximity-feature.js`.
- Move click/dblclick/station-click interaction wiring to `interactions-feature.js`.
- Move mini-map logic to `minimap-feature.js`.

### Checkpoint
- Proximity workflow unchanged.
- Deselect and search-clear behavior unchanged.
- Mini-map still initializes/updates/cleans up correctly.

---

## Step 5 — Extract Renderer Modules (Day 4–5)

### Tasks
- Move aircraft rendering/update logic to `aircraft-renderer.js`.
- Move trail rendering to `trails-renderer.js`.
- Move airports/heliports/navaids/runways/rings rendering to `reference-renderer.js`.
- Keep these renderers Leaflet-backed in Phase 1.

### Checkpoint
- Visual output parity under normal and high traffic.
- No regression in selected aircraft behavior.

---

## Step 6 — Finalize Leaflet Orchestrator (Day 5–6)

### Tasks
- Build `leaflet-map-manager.js` that composes all modules.
- Reduce `www/map-manager.js` to compatibility facade (or replace imports in app directly if safe).
- Remove dead code from monolith after parity confirmation.

### Checkpoint
- App passes functional parity checklist.
- No user-visible changes from baseline.

---

## 7) PR Slicing Strategy (Recommended)

Use small, reviewable PRs:

1. PR-A: scaffolding + no-op wiring
2. PR-B: telemetry extraction
3. PR-C: layer registry + event bus
4. PR-D: interactions + proximity extraction
5. PR-E: mini-map extraction
6. PR-F: aircraft/trails/reference renderer extraction
7. PR-G: orchestrator + compatibility facade cleanup

Each PR must leave app runnable.

---

## 8) Regression Checklist (Phase 1)

After each PR, verify:

- [ ] Map initializes and centers correctly
- [ ] Aircraft markers update continuously
- [ ] Labels show/hide and stale updates still work
- [ ] Trails render and respect trail length setting
- [ ] Selection/deselection behavior unchanged
- [ ] Proximity circle behavior unchanged
- [ ] Airports/heliports/navaids/runways/all-runways toggles work
- [ ] Range rings toggle works
- [ ] Mini-map works in details view
- [ ] No new console errors

---

## 9) Exit Criteria (Hard Gate)

Phase 1 is complete only if:

1. Refactor modules are in place and used.
2. Current UI behavior is baseline-equivalent.
3. No unresolved P0/P1 map regressions.
4. Team can start Phase 2 by swapping only the map engine implementation path.

---

## 10) Risks in This Phase

1. **Behavior drift during extraction**
   - Mitigation: move code in small slices + immediate parity checks.

2. **Import/order issues in browser runtime**
   - Mitigation: preserve deterministic script loading order and compatibility facade.

3. **Hidden coupling with `www/app.js`**
   - Mitigation: maintain public contract and grep usages before every method move.

---

## 11) Ownership & Tracking

Fill before starting:

- Phase owner:
- PR reviewers:
- QA owner:
- Start date:
- Target completion date:

Tracking table:

| Task | Owner | Status | PR | Notes |
|---|---|---|---|---|
| Scaffolding |  |  |  |  |
| Telemetry extraction |  |  |  |  |
| Layer registry + event bus |  |  |  |  |
| Feature extraction |  |  |  |  |
| Renderer extraction |  |  |  |  |
| Orchestrator finalize |  |  |  |  |

---

## 12) Handoff to Phase 2

At Phase 1 completion, open Phase 2 kickoff with:

- Confirmed module boundaries
- Stable public manager contract
- Known behavior parity report
- List of Leaflet-specific surfaces to replace with OpenLayers in each module
