# OpenLayers Migration — Master Index

**Status:** Active execution index
**Date:** 2026-02-18
**Project:** co-atc

This is the single entry point for the OpenLayers migration project.

---

## 1) Start Here

1. Read the master RFC:
   - `docs/openlayers-full-migration-rfc.md`
2. Capture baseline and freeze scope:
   - `docs/openlayers-migration-baseline.md`
3. Execute phases in strict order:
   - Phase 1 → Phase 7 docs listed below

---

## 2) Phase Documents (Execution Order)

## Phase 0 — Baseline & Freeze
- `docs/openlayers-migration-baseline.md`

## Phase 1 — Refactor Preparation
- `docs/openlayers-phase-1-refactor-preparation.md`

## Phase 2 — OpenLayers Foundation
- `docs/openlayers-phase-2-foundation.md`

## Phase 3 — Aircraft Rendering (WebGL-First)
- `docs/openlayers-phase-3-aircraft-rendering.md`

## Phase 4 — Trails, Labels, Selection, Proximity
- `docs/openlayers-phase-4-trails-labels-proximity.md`

## Phase 5 — Reference Overlays, Aviation/Weather Layers, Mini-Map
- `docs/openlayers-phase-5-overlays-minimap.md`

## Phase 6 — Leaflet Removal & Hard Cleanup
- `docs/openlayers-phase-6-leaflet-removal-cleanup.md`

## Phase 7 — Stabilization, Regression, Sign-Off
- `docs/openlayers-phase-7-stabilization-signoff.md`

---

## 3) Execution Rules

- Execute phases sequentially; do not skip gates.
- Keep the app runnable after each PR slice.
- Do not mix unrelated feature work into migration PRs.
- Use baseline scenarios and acceptance thresholds for every performance decision.
- Record regressions in baseline/migration logs as they happen.

---

## 4) Gate Model

Each phase has a hard exit gate. Move forward only when current phase gate passes.

- Phase 0 gate: baseline captured and parity checklist approved.
- Phase 1 gate: modular refactor complete, no behavior drift.
- Phase 2 gate: OpenLayers startup path active and core interactions stable.
- Phase 3 gate: aircraft rendering parity + high-load performance pass.
- Phase 4 gate: trails/labels/proximity parity pass.
- Phase 5 gate: overlays/mini-map parity + failure isolation pass.
- Phase 6 gate: zero Leaflet usage in runtime/source/docs.
- Phase 7 gate: full parity/performance/stability sign-off.

---

## 5) Working Rhythm (Recommended)

For each phase:
1. Kickoff with owners and target dates filled in phase doc.
2. Execute PR slices listed in phase runbook.
3. Run regression checklist after each PR.
4. Update status and decision notes before phase handoff.

---

## 6) Ownership Template

- Migration lead:
- Frontend map lead:
- Performance/QA lead:
- Source policy owner (overlays):
- Operations approver:

---

## 7) Reporting Template

Update at end of each phase:

- Phase completed:
- Date:
- Gate result: PASS / FAIL
- Open P0/P1 items:
- Performance note vs baseline:
- Ready for next phase: YES / NO

---

## 8) Project Closure Condition

Migration is complete only when:
- Phase 7 gate passes
- `docs/openlayers-full-migration-rfc.md` is marked completed
- docs and runtime are OpenLayers-only with no Leaflet remnants
