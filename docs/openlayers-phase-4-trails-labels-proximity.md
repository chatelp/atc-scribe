# OpenLayers Migration — Phase 4 Runbook (Trails, Labels, Selection, Proximity)

**Status:** Ready to execute
**Phase window:** 7–10 days
**Pre-req:** Phase 3 complete (`docs/openlayers-phase-3-aircraft-rendering.md`)
**Parent RFC:** `docs/openlayers-full-migration-rfc.md`

---

## 1) Phase Objective

Port and stabilize map features that are most visible to operators after aircraft rendering:

- live trails
- history/hindcast/future path overlays
- labels and stale-label refresh behavior
- selected-aircraft visual behavior and table linkage
- proximity circle and membership behavior

This phase closes core operational parity for tactical traffic monitoring workflows.

---

## 2) Required Outcomes

By end of Phase 4, all must be true:

1. Trail rendering (including selected-aircraft variants) matches baseline behavior.
2. Label behavior is functionally equivalent (toggle, content, stale updates, fade states).
3. Selection and hover interactions remain consistent with table and map state.
4. Proximity circle + inclusion behavior is parity-equivalent.
5. No P0/P1 regressions in operator workflows.

---

## 3) Scope Boundaries

### In scope
- Trail/history/hindcast/future rendering logic
- Label rendering and refresh timers
- Visual state transitions for selected vs non-selected aircraft
- Proximity feature rendering and filtering behavior
- Map-table selection/hover synchronization

### Out of scope
- Full reference overlays and aviation/weather overlays (Phase 5)
- Full Leaflet removal (Phase 6)
- Post-cutover stabilization/signoff (Phase 7)

---

## 4) Files to Touch

Primary:
- `www/map/renderers/trails.js`
- `www/map/renderers/labels.js`
- `www/map/features/proximity.js`
- `www/map/openlayers-map-manager.js`
- `www/map/core/visibility-rules.js`
- `www/map/perf/telemetry.js`

Potential supporting:
- `www/app.js` (only for contract-safe wiring)
- `www/aircraft-animation.js` (if selected/hover hooks need minor alignment)
- `www/style.css` (label visual classes, highlight classes)

---

## 5) Behavioral Parity Targets

## Trails
- Respect configured trail length and retention behavior.
- Render selected-aircraft extended context paths:
  - historical
  - hindcast
  - future
- Preserve visual differentiation (line style/color/opacity) by trail type.

## Labels
- Respect global label toggle.
- Maintain label content parity (callsign/type/alt/speed/last-seen where applicable).
- Preserve stale-label refresh cadence and text updates.
- Preserve fade/de-emphasis behavior when another aircraft is selected.

## Selection & Hover
- Selected aircraft must remain prominent and visible.
- Non-selected aircraft/labels must de-emphasize consistently.
- Hover interactions should remain synchronized with flight card/table highlights.

## Proximity
- Circle center/radius and visibility rules must match baseline behavior.
- Included-aircraft set should update consistently with current filters/selection context.

---

## 6) Work Plan (Execution Sequence)

## Step 1 — Trail Renderer Scaffold + Layer Wiring (Day 1)

### Tasks
- Initialize dedicated trail layers/sources with ordered z-index.
- Define trail subtype channels: live/history/hindcast/future.
- Connect to map manager lifecycle (init/refresh/cleanup).

### Checkpoint
- Layers create/dispose cleanly.
- No runtime errors with empty trail data.

---

## Step 2 — Live Trail Port + Retention Rules (Day 1–2)

### Tasks
- Port live trail point accumulation and trimming rules.
- Preserve immediate redraw behavior for trail length setting changes.
- Ensure orphan cleanup when aircraft disappears.

### Checkpoint
- Live trails update smoothly with no ghost artifacts.
- Trail length control behaves as baseline.

---

## Step 3 — Selected-Aircraft Context Paths (Day 2–3)

### Tasks
- Port selected-aircraft history, hindcast, and future path rendering.
- Preserve style distinctions and draw order.
- Ensure updates remain coherent while selected aircraft changes.

### Checkpoint
- Selected aircraft context tracks match expected visual behavior.

---

## Step 4 — Label Renderer Port (Day 3–5)

### Tasks
- Implement label layer/source strategy suitable for OpenLayers.
- Port label content generation + minimal DOM/text churn strategy.
- Port stale-label refresh timer and fade-start thresholds.
- Preserve label visibility culling semantics where applicable.

### Checkpoint
- Label text and stale updates behave as baseline.
- Label toggles and visibility transitions are stable.

---

## Step 5 — Selection & Hover State Integration (Day 5–6)

### Tasks
- Port selected/non-selected visual state transitions for labels/trails.
- Preserve map/table hover linking behavior.
- Ensure no stuck highlight state on rapid selection changes.

### Checkpoint
- Selection/hover workflows pass parity checks.

---

## Step 6 — Proximity Feature Port (Day 6–7)

### Tasks
- Port proximity circle rendering and radius update logic.
- Port proximity membership set updates and cleanup behavior.
- Validate behavior when selected aircraft changes/disappears.

### Checkpoint
- Proximity feature behaves consistently across filter and selection changes.

---

## Step 7 — Performance + Regression Validation (Day 7–10)

### Tasks
- Add telemetry for trail/label/proximity update costs.
- Run baseline scenarios A/B/C and long-session checks for these features.
- Tune update cadence and batching if needed.

### Checkpoint
- No critical regressions.
- Performance remains within defined acceptance thresholds.

---

## 7) PR Slicing Strategy (Recommended)

1. PR-4A: trail layer scaffold + lifecycle
2. PR-4B: live trails + retention
3. PR-4C: selected-aircraft context paths
4. PR-4D: label renderer + stale refresh
5. PR-4E: selection/hover integration
6. PR-4F: proximity feature port
7. PR-4G: performance tuning + regression fixes

Each PR must keep app runnable.

---

## 8) Regression Checklist (Phase 4)

- [ ] Live trails render and update continuously
- [ ] Trail length slider reflects immediately
- [ ] Selected aircraft history/hindcast/future paths render correctly
- [ ] Label toggle on/off works globally
- [ ] Label content remains correct and legible
- [ ] Last-seen stale label refresh updates text correctly
- [ ] Selected aircraft remains visually prioritized
- [ ] Non-selected fade/de-emphasis behavior works
- [ ] Map hover and table hover states stay in sync
- [ ] Proximity circle updates and clears correctly
- [ ] Proximity included-aircraft behavior matches baseline
- [ ] No map-breaking console errors

---

## 9) Performance Gates (Hard)

Phase 4 passes only if:

1. Trail/label/proximity updates do not produce sustained UI stutter under scenario B.
2. Added label/trail work does not regress overall responsiveness beyond accepted threshold.
3. Long-session run shows no unbounded memory growth from trail or label lifecycle.
4. Refresh coalescing remains effective under rapid interaction.

---

## 10) Known Risks in This Phase

1. **Label rendering complexity + readability tradeoffs**
   - Mitigation: isolate label renderer and test with dense traffic cases.

2. **Trail overdraw under high density**
   - Mitigation: enforce retention limits, batching, and style simplification where needed.

3. **Selection state race conditions**
   - Mitigation: centralize selected-aircraft state transitions and clear stale references.

4. **Proximity desync with filter state**
   - Mitigation: compute proximity set from canonical filtered aircraft state.

---

## 11) Deliverables

- OpenLayers trail renderer parity for live + selected context paths
- OpenLayers label renderer parity including stale refresh behavior
- Proximity feature parity
- Updated telemetry and validation notes for Phase 4

---

## 12) Ownership & Tracking

Fill before execution:

- Phase owner:
- Reviewer(s):
- QA/perf owner:
- Start date:
- Target completion date:

Tracking table:

| Task | Owner | Status | PR | Notes |
|---|---|---|---|---|
| Trail scaffold |  |  |  |  |
| Live trail parity |  |  |  |  |
| Selected context paths |  |  |  |  |
| Label renderer parity |  |  |  |  |
| Selection/hover sync |  |  |  |  |
| Proximity parity |  |  |  |  |
| Perf/regression closeout |  |  |  |  |

---

## 13) Handoff to Phase 5

At completion, provide:

- Trail/label/proximity parity summary vs baseline
- Remaining map feature gaps limited to reference overlays and aviation/weather layers
- Known caveats (if any) and mitigation plan
- Explicit “ready for Phase 5” decision
