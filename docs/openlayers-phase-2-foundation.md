# OpenLayers Migration — Phase 2 Runbook (Foundation)

**Status:** Ready to execute
**Phase window:** 4–5 days
**Pre-req:** Phase 1 complete (`docs/openlayers-phase-1-refactor-preparation.md`)
**Parent RFC:** `docs/openlayers-full-migration-rfc.md`

---

## 1) Phase Objective

Introduce OpenLayers as the active map engine foundation while preserving current app-level map manager contract.

This phase establishes map initialization, base map layer, core view controls, and interaction parity hooks. It does **not** complete full rendering parity for aircraft/trails/labels (that is Phase 3 and 4).

---

## 2) Required Outcomes

By end of Phase 2, all must be true:

1. App starts with OpenLayers map active.
2. Core map operations work (pan, zoom, set view, fit bounds).
3. Click/dblclick/station-click interaction semantics are preserved.
4. Existing store ↔ map manager calls continue through compatibility API.
5. No Leaflet usage in startup path.

---

## 3) Scope Boundaries

### In scope
- OpenLayers dependency wiring and startup integration
- OpenLayers map engine lifecycle (`init`, `dispose`, `setView`, `fitBounds`)
- Base map layer setup
- Core interaction event wiring
- Compatibility shim for map manager public methods

### Out of scope
- WebGL aircraft rendering parity (Phase 3)
- Trail/label/proximity parity (Phase 4)
- Full overlay migration (Phase 5)
- Full Leaflet code deletion (Phase 6)

---

## 4) Files to Touch

Primary:
- `www/index.html`
- `www/app.js`
- `www/map/core/map-engine.js`
- `www/map/features/interactions-feature.js`
- `www/map/openlayers-map-manager.js`
- `www/map-manager.js` (temporary compatibility forwarding, if still present)

Supporting:
- `www/style.css` (OpenLayers container/control class adjustments only)
- `docs/technical_docs.md` (optional incremental note)

---

## 5) Public Contract to Preserve

Keep these callable from app/store with no behavioral surprises:

- `initMap()`
- `applyFiltersAndRefreshView(options)`
- `updateSingleAircraft(hex, aircraft)`
- `toggleRings()`
- `getPerformanceStats()`

In Phase 2, methods not yet fully implemented on OpenLayers should fail safe (no crash), emit clear warnings, and keep app operational.

---

## 6) Work Plan (Execution Sequence)

## Step 1 — OpenLayers Dependency Wiring (Day 1)

### Tasks
- Add OpenLayers CSS/JS loading path in `www/index.html`.
- Ensure deterministic script load order for map modules.
- Keep Leaflet includes temporarily only if needed for unfinished code paths.

### Checkpoint
- App loads with no import/runtime errors.
- OpenLayers classes available in browser runtime.

---

## Step 2 — Implement OpenLayers Map Engine Core (Day 1–2)

### Tasks
- Implement `map-engine.js`:
  - create map instance
  - set default center/zoom
  - expose `setView` and `fitBounds`
  - expose `getBounds`, `getZoom`, `on/off` event wrappers
  - clean shutdown/dispose behavior
- Ensure station coordinates are respected on initial center.

### Checkpoint
- Map displays and responds to pan/zoom.
- `setView`/`fitBounds` work from manager calls.

---

## Step 3 — Base Layer + View Semantics (Day 2)

### Tasks
- Add Carto dark base layer equivalent to current behavior.
- Match current zoom defaults and practical bounds behavior.
- Preserve attribution behavior at minimum operational equivalence.

### Checkpoint
- Visual base map is usable and equivalent in intent.
- No map blank/white tile regressions.

---

## Step 4 — Interaction Wiring Parity (Day 2–3)

### Tasks
- Port click behavior:
  - map click deselect selected aircraft
  - station override map-click mode sets coordinates
- Port dblclick behavior:
  - clear search term when populated
- Use `interactions-feature.js` for centralized event registration.

### Checkpoint
- Interaction semantics match baseline behavior.
- No duplicate event handler side effects.

---

## Step 5 — Compatibility Manager Shim (Day 3–4)

### Tasks
- Implement `openlayers-map-manager.js` as active manager.
- Keep contract expected by `app.js`.
- For methods not yet fully ported, provide safe no-op or partial implementations with explicit TODO markers for Phase 3/4.

### Checkpoint
- App flow remains stable with OpenLayers manager active.
- No startup calls fail due to missing methods.

---

## Step 6 — App Integration + Startup Path Validation (Day 4)

### Tasks
- Rewire manager instantiation in `www/app.js` to OpenLayers manager.
- Ensure no Leaflet object is required to start app.
- Verify initial data load, map init timing, and store subscriptions.

### Checkpoint
- Startup path works end-to-end with OpenLayers.
- Leaflet no longer required for initial map boot.

---

## Step 7 — Hardening & Handoff Prep (Day 5)

### Tasks
- Add targeted logging around map init and interaction events.
- Record known intentional gaps for Phase 3/4 in a short handoff section.
- Re-run baseline interaction checks and document results.

### Checkpoint
- Phase 2 exit criteria met.
- Team has clean handoff list for aircraft rendering phase.

---

## 7) PR Slicing Strategy (Recommended)

1. PR-2A: OpenLayers dependency + scaffold
2. PR-2B: map engine core lifecycle
3. PR-2C: base layer + view semantics
4. PR-2D: interactions parity wiring
5. PR-2E: openlayers map manager + app integration
6. PR-2F: hardening, docs, and phase handoff notes

Each PR must keep app runnable.

---

## 8) Regression Checklist (Phase 2)

After each PR:

- [ ] App boots without JS errors
- [ ] Map visible and interactive
- [ ] Pan/zoom smooth and stable
- [ ] Initial center uses station coordinates
- [ ] Click deselect behavior works
- [ ] Station override click-to-set works
- [ ] Dblclick clears search behavior works
- [ ] `applyFiltersAndRefreshView` calls do not break app flow
- [ ] `getPerformanceStats` remains callable
- [ ] No startup dependency on Leaflet objects

---

## 9) Exit Criteria (Hard Gate)

Phase 2 is complete only if:

1. OpenLayers map is active in production startup path.
2. Core interactions are parity-equivalent for click/dblclick/station-click.
3. Manager public contract remains operational.
4. No P0/P1 startup or navigation regressions.
5. Phase 3 can begin with aircraft rendering as primary work, not foundation fixes.

---

## 10) Known Risks in This Phase

1. **Script load/order issues in non-bundled setup**
   - Mitigation: explicit load order and startup guards.

2. **Coordinate/projection mismatch bugs**
   - Mitigation: centralize projection conversion helpers in map engine.

3. **Hidden Leaflet assumptions in app store flows**
   - Mitigation: keep contract-compatible methods and fallback-safe behavior.

4. **Event semantic drift (click targets)**
   - Mitigation: parity-test click cases after each PR.

---

## 11) Phase 2 Deliverables

- Working OpenLayers foundation integrated into app startup
- Updated map manager orchestration for OpenLayers path
- Interaction parity for core click flows
- Phase 3 handoff list documenting remaining rendering parity items

---

## 12) Ownership & Tracking

Fill before execution:

- Phase owner:
- Reviewer(s):
- QA owner:
- Start date:
- Target completion date:

Tracking table:

| Task | Owner | Status | PR | Notes |
|---|---|---|---|---|
| Dependency wiring |  |  |  |  |
| Map engine core |  |  |  |  |
| Base layer semantics |  |  |  |  |
| Interaction parity |  |  |  |  |
| Manager integration |  |  |  |  |
| Hardening + handoff |  |  |  |  |

---

## 13) Handoff to Phase 3

At Phase 2 completion, create a short section in PR notes with:

- Methods currently no-op/partial awaiting aircraft renderer
- Any projection/coordinate caveats found
- Remaining high-load behavior gaps
- Final “ready for Phase 3” confirmation
