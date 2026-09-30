# Single-Floor Conceptual 3D Preview Specification

**Status:** Approved as a single-floor extension in constitution 1.1.0; implementation and containerized PostgreSQL migration smoke test complete.

## Goal

Let a quantity surveyor inspect a conceptual 3D preview associated with one analyzed residential floor plan without presenting inferred placement as a measured building model. The preview is for review only and never supplies dimensions, openings, quantities, or costs to estimates.

## User Stories

- As a quantity surveyor, I can open a 3D preview after analysis to understand which extracted rooms have enough verified data to visualize.
- As a quantity surveyor, I can distinguish verified room dimensions from inferred room arrangement and see which positions and opening locations are unknown.
- As a quantity surveyor, I can correct and confirm source data through the existing review workflow and see the preview update from that confirmed version.
- As a quantity surveyor, I can continue reviewing and estimating if the preview cannot be rendered.

## Scope and Assumptions

- One drawing and one residential floor only. The preview is conceptual, not a survey, construction drawing, BIM model, or a statement of real room adjacency.
- Analysis candidates remain unverified. Opening the preview after analysis does not make extracted dimensions eligible for display as verified dimensions.
- The existing confirmation workflow is the only source of verified room dimensions. Missing room coordinates, adjacency, wall placement, orientation, and opening positions remain unknown unless separately supplied and verified in a future feature.
- Rectangular room volume may be visualized from confirmed length, width, and wall height. The screen must always identify the arrangement and unmeasured surfaces as conceptual.
- The preview neither changes the approved calculation formulas nor expands the MVP to multiple floors, structural materials, or autonomous design generation.

## Functional Requirements

- The analysis review screen offers a "3D preview" view once an analysis reaches `AnalysisReady`. While there is no confirmed data for that analysis, the view shows a verification-required state, not measured room shapes built from AI-only values.
- After confirmation, each room with positive confirmed length, width, wall height, and units can appear as an individual rectangular volume. Show its name and confirmed dimensions with units. Keep room volumes visually separate; their order, spacing, orientation, colors, wall thickness, and appearance are schematic and labeled "Conceptual arrangement; room positions not verified."
- A room with missing or invalid dimensions is not rendered as a measured volume. Name the room and missing fields in an accompanying list without substituting default measurements.
- Do not draw a door, window, opening cutout, common wall, circulation path, or building footprint from room association alone. List openings whose positions are unknown as "Location not verified"; verified width/height alone does not establish placement.
- Provide orbit, zoom, and reset-view controls for the scene. Room labels and uncertainty information remain readable on desktop and mobile, and the same information is available in text if 3D rendering is unavailable.
- The preview identifies the analysis and confirmed-data version it represents. Re-analysis or edits to the review form do not silently replace the preview; require a new confirmation before its measured dimensions change.
- Preview data is read-only. No value derived from scene placement, AI imagery, camera state, or rendered geometry is written into confirmed data or passed into QS calculations or export.

## Review and Failure Behavior

- Before confirmation, explain which rooms need review and keep the existing editable analysis visible. Confirming the data uses the existing validation rules; merely opening or interacting with the preview never confirms it.
- When the confirmed version has no eligible rooms, show a clear empty state with the missing inputs; do not synthesize geometry.
- If preview generation or rendering fails, show a retry action and the room/uncertainty list. Analysis results, confirmations, estimates, and Excel export remain available and unchanged.
- If a new analysis fails, the preview for that analysis is unavailable; do not present a previous analysis as its result. A previously confirmed version remains explicitly identifiable when available.

## Out of Scope

- Accurate full-building geometry, inferred adjacency or opening placement, texture generation, construction-ready BIM/CAD export, multi-floor modeling, and AI-generated dimensions or costs.

## Acceptance Criteria

1. Given a completed analysis without confirmed data, opening the 3D preview shows a verification-required state and no measured room volumes.
2. Given one confirmed rectangular room, the preview shows its confirmed name, dimensions, and units; the layout is labeled conceptual and never claims an actual position or adjacency.
3. Given multiple confirmed rooms with unknown positions, they appear as separate conceptual volumes; no common wall or footprint is asserted.
4. Given an opening with confirmed dimensions but no verified position, its location is listed as unknown and no opening or cutout is placed in the scene.
5. Given missing or invalid dimensions, the affected room is listed with the missing fields and no substitute-sized volume is rendered.
6. Given unconfirmed edits or a new analysis, an existing preview remains tied to its labeled confirmed version until new data is confirmed.
7. Given a rendering failure or unavailable 3D support, the user sees the same room and uncertainty information in text and can still confirm data, calculate, and export.
8. Given identical confirmed inputs with and without opening the preview, QS quantities, rates, costs, and Excel export remain identical.
9. Desktop and mobile users can identify every displayed room, see the conceptual warning, and use or bypass the view without losing the review workflow.

## Approval Gate

Constitution 1.1.0 approves this read-only single-floor conceptual visualization while preserving the prohibition on invented dimensions and AI-driven calculations.