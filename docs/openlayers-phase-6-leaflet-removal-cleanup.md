# OpenLayers Migration — Phase 6 Runbook (Leaflet Removal & Hard Cleanup)

**Status:** Ready to execute
**Phase window:** 3–4 days
**Pre-req:** Phase 5 complete (`docs/openlayers-phase-5-overlays-minimap.md`)
**Parent RFC:** `docs/openlayers-full-migration-rfc.md`

---

## 1) Phase Objective

Remove Leaflet completely from the project and eliminate all compatibility/debt paths left from migration.

This phase is the formal cutover cleanup: OpenLayers-only runtime, source, and docs.

---

## 2) Required Outcomes

By end of Phase 6, all must be true:

1. Zero Leaflet references in frontend runtime path.
2. Zero Leaflet dependency includes/imports in source files.
3. Compatibility shims used during migration are removed.
4. Documentation reflects OpenLayers-only architecture.
5. App runs cleanly with no Leaflet fallback assumptions.

---

## 3) Scope Boundaries

### In scope
- Remove Leaflet CSS/JS includes and code paths
- Delete/retire Leaflet-specific modules and wrappers
- Remove Leaflet-specific styles/selectors
- Update docs (technical + migration references)
- Cleanup dead code created during phased migration

### Out of scope
- New feature work
- Large UI redesign
- Final long-run stabilization signoff (Phase 7)

---

## 4) Files to Touch

Primary:
- `www/index.html`
- `www/app.js`
- `www/map-manager.js` (delete or convert to OpenLayers alias as final)
- `www/style.css`
- `www/map/**` (remove Leaflet-only artifacts)

Documentation:
- `docs/technical_docs.md`
- `README.md`
- `docs/openlayers-full-migration-rfc.md` (status update section if used)

---

## 5) Work Plan (Execution Sequence)

## Step 1 — Remove Runtime Includes/Dependencies (Day 1)

### Tasks
- Remove Leaflet CSS/JS tags from `www/index.html`.
- Ensure OpenLayers includes remain the only map framework runtime dependency.
- Validate script load order still correct.

### Checkpoint
- App starts with no unresolved Leaflet symbols.

---

## Step 2 — Remove Leaflet Integration Points (Day 1–2)

### Tasks
- Remove Leaflet object injection and references from `www/app.js`.
- Remove remaining `L.` usage in map code and temporary wrappers.
- Replace or remove migration compatibility facade where still present.

### Checkpoint
- No code path requires `L` or Leaflet classes.

---

## Step 3 — Delete/Retire Legacy Files (Day 2)

### Tasks
- Delete obsolete Leaflet-only modules/files.
- Remove dead helper methods that existed only for transitional compatibility.
- Keep source tree clean and intentional.

### Checkpoint
- Source tree has only OpenLayers map stack artifacts.

---

## Step 4 — Style Cleanup (Day 2–3)

### Tasks
- Remove Leaflet-specific CSS selectors/classes from `www/style.css`.
- Verify map container and overlay styles remain correct.

### Checkpoint
- No styling regressions from Leaflet class removal.

---

## Step 5 — Repository-wide Verification (Day 3)

### Tasks
- Search repository for residual Leaflet references:
  - `leaflet`
  - `L.`
  - legacy map-manager compatibility markers
- Resolve any remaining references.

### Checkpoint
- Zero intended Leaflet references remain (excluding historical changelog notes if desired).

---

## Step 6 — Documentation Cleanup (Day 3–4)

### Tasks
- Update `docs/technical_docs.md` from Leaflet architecture to OpenLayers architecture.
- Update README frontend stack references.
- Add migration status note that Leaflet removal is complete.

### Checkpoint
- Docs are internally consistent and accurate.

---

## 6) PR Slicing Strategy (Recommended)

1. PR-6A: runtime include + app wiring cleanup
2. PR-6B: Leaflet code path removal and file cleanup
3. PR-6C: CSS and style cleanup
4. PR-6D: docs updates and final repo-wide reference cleanup

Each PR must keep app runnable.

---

## 7) Regression Checklist (Phase 6)

- [ ] App boot succeeds without Leaflet includes
- [ ] Map renders and all migrated features remain operational
- [ ] No console errors related to missing Leaflet symbols
- [ ] Selection, filters, trails, labels, proximity, mini-map still work
- [ ] Overlay toggles still work
- [ ] No visual regression from CSS cleanup
- [ ] Docs no longer claim Leaflet as active map stack

---

## 8) Hard Gate (Definition of Completion)

Phase 6 is complete only if:

1. Leaflet is absent from runtime and source dependencies.
2. Repository scans clean for unintended Leaflet references.
3. Functional parity remains intact after removal.
4. Team can enter Phase 7 with only stabilization/signoff work remaining.

---

## 9) Known Risks in This Phase

1. **Hidden transitive Leaflet assumptions**
   - Mitigation: repo-wide grep and startup smoke tests after each cleanup PR.

2. **Accidental removal of still-used compatibility hooks**
   - Mitigation: remove in small PR slices with focused regression checks.

3. **Documentation drift**
   - Mitigation: pair code cleanup PRs with docs updates immediately.

---

## 10) Deliverables

- OpenLayers-only frontend runtime
- Cleaned source tree without Leaflet compatibility debt
- Updated technical and project documentation
- Phase 7 readiness confirmation

---

## 11) Ownership & Tracking

Fill before execution:

- Phase owner:
- Reviewer(s):
- QA owner:
- Docs owner:
- Start date:
- Target completion date:

Tracking table:

| Task | Owner | Status | PR | Notes |
|---|---|---|---|---|
| Runtime include cleanup |  |  |  |  |
| App/map wiring cleanup |  |  |  |  |
| Legacy file retirement |  |  |  |  |
| CSS cleanup |  |  |  |  |
| Repo scan verification |  |  |  |  |
| Docs update |  |  |  |  |

---

## 12) Handoff to Phase 7

At completion, provide:

- Repo scan report showing Leaflet removal complete
- Regression summary post-cleanup
- Updated docs references
- Explicit “ready for Phase 7” decision
