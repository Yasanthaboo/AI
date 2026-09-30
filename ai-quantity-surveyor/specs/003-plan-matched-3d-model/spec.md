# Plan-Matched Single-Floor 3D Model Specification

**Status:** Superseded for the proof of concept by constitution 1.3.0: the AI extracts room boundaries and the model is shown automatically after dimension confirmation as an AI-estimated layout. Manual tracing and separate layout confirmation were removed.

## Goal

After a quantity surveyor confirms dimensions and the layout traced from the uploaded drawing, show a single-floor 3D model whose room positions, wall connections, and verified openings match that drawing. Do not present dimensions alone as enough information to reconstruct a floor plan.

## User Stories

- As a quantity surveyor, I can compare a proposed 2D layout trace directly over my uploaded plan and correct misplaced or missing geometry.
- As a quantity surveyor, I can separately confirm dimensions and spatial layout before seeing them combined in a plan-matched 3D model.
- As a quantity surveyor, I can identify unverified walls, rooms, and opening positions rather than seeing guessed geometry.
- As a quantity surveyor, I can still confirm quantities and export estimates if layout tracing or 3D rendering fails.

## Functional Requirements

- Analysis may propose room outlines, wall paths, and door/window anchors in coordinates relative to one source drawing page. Every proposed feature references its source analysis and drawing page. Missing or unreadable positions remain unknown, never filled with a plausible layout.
- Provide a 2D review overlay on the original uploaded drawing. The user can adjust or remove proposed geometry and explicitly confirm its alignment; confirmation of numeric dimensions does not automatically confirm the layout.
- Match every traced room to an existing room ID. Reject invalid, overlapping, out-of-bounds, or self-intersecting room outlines; identify disconnected walls and openings not anchored to a reviewed wall. Do not silently snap, stretch, or substitute geometry to make dimensions fit.
- Derive a single drawing-to-meter scale from confirmed measurements and reviewed geometry. Where room outline measurements conflict with confirmed dimensions, show the discrepancy and require correction or exclusion before layout confirmation. The confirmed numeric values remain the source for quantity calculations.
- Show the plan-matched 3D model only for the same analysis and versions of both confirmed dimensions and confirmed layout. Extrude reviewed wall paths and place only reviewed door/window anchors; preserve their positions relative to the source plan. Do not depict unreviewed portions as an accurate building.
- Without a confirmed layout, replace the separate room boxes with a clear "Layout not verified" state, the source drawing, and the remaining review actions. Do not imply that the conceptual boxes resemble the uploaded plan.
- Label the model as a verified plan interpretation, not a construction-ready BIM/CAD deliverable. Make the source drawing and confirmation versions discoverable beside the model.
- Re-analysis or edits to dimensions or layout do not silently mutate a previously confirmed model. Show the version represented and require another confirmation before displaying the changed geometry as verified.
- Keep confirmed geometry read-only to the QS calculation and export paths. No inferred coordinate, wall thickness, area, or layout-derived length may alter quantities or costs.

## Failure and Review Behavior

- If the AI cannot identify a reliable outline, allow manual tracing over the drawing; do not generate a generic room arrangement.
- If a PDF page cannot be displayed for trace review, show a specific error and retain the ordinary dimension-review and estimate workflow.
- If geometry is inconsistent with confirmed dimensions or a room cannot be aligned to the drawing, block *layout* confirmation for the affected features, not the existing numeric estimate workflow.
- If WebGL is unavailable, retain a readable 2D overlay and textual list of confirmed and unresolved geometry.

## Acceptance Criteria

1. After confirming dimensions alone, a drawing with no confirmed layout shows "Layout not verified" and no plan-matched 3D room/wall model.
2. For a test plan with two rooms at distinct positions sharing a wall, the confirmed 2D trace overlays those same positions, and the 3D view preserves their relative position and shared wall.
3. A door/window is placed in 3D only when its wall anchor and position were reviewed; an unlocated opening remains listed as unresolved.
4. Moving or removing a proposed room/wall on the 2D overlay changes the model only after explicit layout reconfirmation; the previous version remains identifiable.
5. A conflicting room outline and confirmed length/width produce a named discrepancy instead of silently scaling either one to fit.
6. Failed tracing, an unreadable PDF, or a failed 3D renderer does not block confirmation of numeric dimensions, QS calculations, or Excel export.
7. For the same confirmed numeric inputs, estimates and exports are identical with or without confirmed layout and model interaction.
8. Desktop and mobile views make the source drawing, trace, unresolved items, and confirmation status distinguishable without overlapping controls or misleading invented geometry.

## Scope Gate

Constitution 1.2.0 permits separately reviewed single-floor spatial geometry while preserving the no-fabrication rule and independent deterministic QS calculations. The current implementation requires manual rectangular room tracing on an image or PDF page one. Opening anchors, nonrectangular room geometry, and AI-proposed traces are not implemented; do not claim those acceptance criteria until they are delivered. Multi-floor modeling and construction-ready BIM/CAD remain excluded.