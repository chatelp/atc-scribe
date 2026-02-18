# OpenLayers Full Migration & Leaflet Removal RFC

**Status:** Proposed (ready for execution)
**Date:** 2026-02-18
**Project:** co-atc
**Scope:** Full replacement of Leaflet with OpenLayers, including refactor of map-related frontend architecture

---

## 1) Executive Summary

We will remove Leaflet entirely and port all map capabilities to OpenLayers.

Execution index for this migration pack:
- `docs/openlayers-migration-index.md`

This is not a 1:1 library swap. We will use the migration to refactor map code into maintainable modules, move aircraft rendering to a WebGL-first path, and formalize map overlays (aviation layers, radar weather, airspace rings, reference data overlays) behind a consistent layer registry.

At completion, there must be **zero runtime dependency on Leaflet**, full functional parity, and measurable performance improvement or stability under high aircraft load.

---

## 2) Why This Migration

### Key drivers
- Better high-density rendering architecture (WebGL points path)
- First-class overlay model for aviation use-cases (charts, weather, airspace)
- Cleaner long-term maintainability than continuing to optimize deeply Leaflet-specific code
- Opportunity to reduce technical debt in `www/map-manager.js`

### Decision
- **No dual-engine long-term support**
- Temporary feature-flagging may be used only during migration, then removed

---

## 3) Goals / Non-Goals

### Goals
1. Remove Leaflet from UI runtime and source code.
2. Preserve existing user-visible map behavior (selection, trails, labels, filtering, mini-map, overlays).
3. Introduce OpenLayers WebGL aircraft rendering path.
4. Refactor map code into modular architecture.
5. Add structured overlay registry and source policy controls.

### Non-goals
- Rewriting backend APIs
- UI redesign unrelated to map behavior
- New unrelated features outside map migration scope

---

## 4) Current State (Affected Code)

Primary current map integration:
- `www/index.html` (Leaflet CSS/JS includes)
- `www/app.js` (MapManager wiring + map-triggered store interactions)
- `www/map-manager.js` (main map lifecycle, rendering, interactivity, trails, overlays, perf)
- `www/aircraft-animation.js` (interacts with map marker state)
- `www/style.css` (Leaflet-specific selectors, if any)
- Docs mentioning Leaflet in `docs/technical_docs.md` and elsewhere

Current map code is feature-rich but monolithic; migration success requires decomposition first.

---

## 5) Target Architecture

Create map modules under `www/map/`:

- `www/map/core/map-engine.js`
  - OpenLayers map init/dispose, view operations, common helpers
- `www/map/core/layer-registry.js`
  - Central declaration of all layers + metadata (z-index, visibility, opacity, source policy)
- `www/map/renderers/aircraft-webgl.js`
  - Aircraft position rendering via WebGL points/vector source updates
- `www/map/renderers/trails.js`
  - Historical, hindcast, future, selected aircraft path rendering
- `www/map/renderers/labels.js`
  - Label text, state transitions, stale updates, declutter behavior
- `www/map/renderers/reference.js`
  - Airports/heliports/navaids/runways/all-runways/rings
- `www/map/features/proximity.js`
  - Proximity circle + set membership behavior
- `www/map/features/interactions.js`
  - Click/dblclick/selection/station-click mode
- `www/map/features/minimap.js`
  - Aircraft detail mini-map
- `www/map/perf/telemetry.js`
  - Perf counters + reporting API (parity with current map perf panel)
- `www/map/openlayers-map-manager.js`
  - Orchestrator replacing `MapManager` monolith API expected by `app.js`

Keep external app contract stable where practical:
- `initMap()`
- `applyFiltersAndRefreshView()`
- `updateSingleAircraft()`
- `toggleRings()`
- other existing call points from `app.js`

---

## 6) Migration Phases (Execution Plan)

## Phase 0 — Baseline & Freeze (2-3 days)

### Tasks
- Capture baseline metrics using current map perf stats:
  - avg refresh ms
  - refresh/sec
  - marker ops/sec
  - behavior under high aircraft count
- Freeze new map feature work during migration.
- Define parity checklist and acceptance thresholds.

### Exit Criteria
- Baseline report committed in `docs/openlayers-migration-baseline.md`.
- Parity checklist approved.

---

## Phase 1 — Refactor Preparation (4-6 days)

### Tasks
- Split `www/map-manager.js` into modules without behavior changes.
- Separate state derivation from rendering side-effects.
- Introduce layer registry abstraction and map event bus.

### Exit Criteria
- Existing Leaflet behavior unchanged.
- Map code organized into module boundaries.
- No user-visible regressions.

---

## Phase 2 — OpenLayers Foundation (4-5 days)

### Tasks
- Add OpenLayers dependencies and load path in frontend.
- Implement map creation, base layer, view controls, move/zoom event handlers.
- Port map click + dblclick semantics and station override click mode.
- Add compatibility shim so app store still calls manager API unchanged.

### Exit Criteria
- App starts with OpenLayers map active.
- Core interactions work (pan/zoom/click).
- No Leaflet object usage in startup path.

---

## Phase 3 — Aircraft Rendering (WebGL-First) (7-10 days)

### Tasks
- Implement aircraft rendering as OpenLayers WebGL points or equivalent performant vector path.
- Port marker state updates (position/heading/status/selection highlight).
- Port single-aircraft update fast path.
- Preserve viewport logic and filtering semantics.

### Exit Criteria
- Aircraft render/update parity achieved.
- Equal or better high-load responsiveness compared to baseline.

---

## Phase 4 — Trails, Labels, Selection, Proximity (7-10 days)

### Tasks
- Port trails (live + history + hindcast + future).
- Port label rendering/refresh logic and fade/visual state transitions.
- Port selected-aircraft behavior and table hover linkage.
- Port proximity circle + set filtering behavior.

### Exit Criteria
- Selected aircraft UX parity complete.
- Track visualizations match baseline behavior.

---

## Phase 5 — Reference Layers + Aviation Overlays + Mini-Map (7-10 days)

### Tasks
- Port airports, heliports, navaids, runways, all-runways, distance rings.
- Add structured overlay support for:
  - aviation chart layers
  - weather radar/cloud layers
  - airspace polygons/rings
- Port detail mini-map behavior.
- Add independent failure handling per overlay source.

### Exit Criteria
- Overlay toggles operational and isolated.
- Mini-map fully functional.
- Aircraft rendering remains stable when overlays enabled.

---

## Phase 6 — Leaflet Removal & Hard Cleanup (3-4 days)

### Tasks
- Remove Leaflet CSS/JS references from `www/index.html`.
- Remove Leaflet-specific code and dead compatibility branches.
- Remove Leaflet-specific styles/selectors.
- Update docs and architecture references.

### Exit Criteria
- Zero Leaflet imports/usages across repository.
- Build/run works with OpenLayers only.
- Documentation updated.

---

## Phase 7 — Stabilization, Regression, Sign-Off (3-5 days)

### Tasks
- Run full regression checklist.
- Run high-load sessions and compare against baseline.
- Fix migration regressions only.
- Final go/no-go review.

### Exit Criteria
- Parity checklist all green.
- Performance gate passed.
- RFC marked completed.

---

## 7) File-by-File Workboard

## Core frontend
- `www/index.html`
  - Remove Leaflet includes
  - Add OpenLayers includes/init requirements
- `www/app.js`
  - Rewire manager creation/import
  - Keep public map manager contract stable where possible
- `www/map-manager.js`
  - Decompose, then replace by `www/map/openlayers-map-manager.js`
- `www/aircraft-animation.js`
  - Adapt marker references if internal marker representation changes
- `www/style.css`
  - Remove Leaflet-specific selectors/classes

## New modules (to add)
- `www/map/core/map-engine.js`
- `www/map/core/layer-registry.js`
- `www/map/renderers/aircraft-webgl.js`
- `www/map/renderers/trails.js`
- `www/map/renderers/labels.js`
- `www/map/renderers/reference.js`
- `www/map/features/interactions.js`
- `www/map/features/proximity.js`
- `www/map/features/minimap.js`
- `www/map/perf/telemetry.js`
- `www/map/openlayers-map-manager.js`

## Documentation
- `docs/technical_docs.md` (replace Leaflet mentions)
- `README.md` (frontend map stack update)
- `docs/openlayers-migration-baseline.md` (new)
- `docs/openlayers-full-migration-rfc.md` (this doc)

---

## 8) Definition of Done (Strict)

All must be true:
1. No Leaflet runtime dependency or source usage remains.
2. All current map features are functionally available.
3. High-load performance is at least baseline-equivalent, preferably improved.
4. Overlay system supports aviation/radar/airspace layers with robust failure isolation.
5. Docs fully reflect OpenLayers architecture.

---

## 9) Acceptance Checklist

### Interaction parity
- [ ] Single click deselect behavior
- [ ] Double-click search clear behavior
- [ ] Station override map click mode
- [ ] Selected aircraft persistence through filtering

### Rendering parity
- [ ] Aircraft markers with heading/state visuals
- [ ] Labels on/off and stale label refresh behavior
- [ ] Trails with configured length and selected aircraft history/future overlays
- [ ] Proximity circle and included aircraft set behavior

### Data overlay parity
- [ ] Airports/heliports/navaids toggles
- [ ] Runways/all runways toggles
- [ ] Distance rings toggle
- [ ] Mini-map tracks rendering

### New overlay capabilities
- [ ] Aviation chart layer(s)
- [ ] Weather radar/cloud overlay(s)
- [ ] Airspace polygons/rings
- [ ] Per-layer opacity/visibility controls

### Performance and reliability
- [ ] Map perf stats functional
- [ ] No progressive memory growth during long run
- [ ] Smooth operation under high aircraft count
- [ ] Overlay source failures do not break aircraft rendering

---

## 10) Risks & Mitigations

1. **Label parity complexity**
   - Mitigation: implement labels as dedicated renderer module with explicit test checklist.

2. **WebGL environment variability**
   - Mitigation: support non-WebGL fallback rendering path in OpenLayers.

3. **Overlay provider outages/rate limits**
   - Mitigation: source-level circuit breaker and independent failure isolation.

4. **Regression risk from big-bang replacement**
   - Mitigation: phased migration with strict phase exit criteria and gated cutover.

5. **Team drift during migration**
   - Mitigation: freeze unrelated map features; enforce RFC scope.

---

## 11) Rollback Strategy

During migration only:
- Keep a short-lived rollback branch before full Leaflet deletion.
- If critical blocker appears before Phase 6 completion, revert to last stable milestone.

After Phase 6:
- No operational rollback to Leaflet; fixes occur in OpenLayers stack.

---

## 12) Recommended Project Cadence

- Week 1: Phase 0-1
- Week 2: Phase 2-3
- Week 3: Phase 4
- Week 4: Phase 5-6
- Week 5 (buffer): Phase 7 stabilization

Expected duration: **4-5 weeks** depending on parity gaps and overlay source integration effort.

---

## 13) Execution Notes

- Keep commits small and phase-scoped.
- Preserve existing public map manager method names until final cleanup.
- Do not mix map migration with unrelated refactors.
- Every phase must end with a runnable app state.

---

## 14) Project Kickoff Tasks (Immediate)

1. Create `migration/openlayers` working branch.
2. Create `docs/openlayers-migration-baseline.md` and capture baseline metrics.
3. Break up `www/map-manager.js` into modules (no behavior changes yet).
4. Implement OpenLayers init in parallel with current manager contract.

---

## 15) Ownership Template (Fill In)

- Migration lead:
- Aircraft renderer owner:
- Overlay/layer owner:
- UI interaction owner:
- QA/perf owner:
- Target cutover date:

---

## 16) Final Success Statement

This migration is complete when co-atc ships with OpenLayers-only map infrastructure, matches existing operational behavior, supports aviation-focused overlays cleanly, and demonstrates stable performance under high aircraft traffic.
