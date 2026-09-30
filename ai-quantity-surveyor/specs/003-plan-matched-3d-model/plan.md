# Implementation Plan: Plan-Matched Single-Floor 3D Model

**Status:** Constitution 1.2.0 approves reviewed layout. Manual rectangle tracing, independent confirmed layout persistence, and positioned wall extrusion are implemented. AI-proposed traces, opening anchors, and irregular polygons remain planned.

## Current Gap

The current analysis contract stores room length, width, wall height, and openings associated with room IDs, but no source-page positions, room outlines, wall paths, or opening anchors. The client scene arranges independent boxes using presentation offsets. Neither confirmed measurements nor that scene can establish a floor-plan layout. The backend stores drawing bytes but has no browser-facing drawing retrieval endpoint.

## Trust and Data Flow

`uploaded drawing -> AI-proposed trace -> 2D overlay correction -> explicit layout confirmation -> plan-matched 3D extrusion`

Confirmed dimensions provide scale and height; confirmed spatial geometry provides position and connections. Both must refer to the same analysis/drawing and compatible revisions. Geometry is a read-only visualization input and cannot flow into `domain.Calculate`, rates, estimates, or Excel. If either confirmation is missing or inconsistent, show the source plan and review state, not reconstructed walls.

## Contracts and Persistence

- Define a spatial candidate with source `floorPlanId`, `analysisId`, page number, image width/height and orientation, normalized room-outline vertices linked to room IDs, wall segments, and opening anchors linked to reviewed walls. Null/absent locations remain explicit. No free-form generated mesh or untraceable image is authoritative.
- Define independent layout revision records: candidate, user corrections, confirmed geometry, source page, dimensions-confirmation revision, and timestamp. Persist immutable confirmed revisions separately from the current numeric confirmation table. Reject cross-analysis or cross-page references and stale dimension revisions.
- Validate coordinates and topology on the server (finite, within drawing bounds, non-self-intersecting rooms, valid wall connections, opening anchor on an existing wall). Use an established polygon-geometry library where appropriate; invalid entries are returned with their room/wall/opening IDs.
- Calibrate a single drawing-to-meter transform using confirmed dimensions and reviewed reference edges. Compare each eligible outline against numeric measurements; surface discrepancies rather than adjusting either dataset without review. Dimension confirmation remains the sole calculation source.
- Extend AI adapters to propose optional geometry only when supported; failures and omitted spatial fields leave a reviewable incomplete layout. Retain the original drawing and candidate so users can correct or manually trace even when AI geometry is absent.

## Drawing and Review Surface

- Preserve the selected source file in browser IndexedDB under its analysis ID and restore it locally after reload; no unauthenticated raw-file endpoint is exposed. The file is not shared across browsers. PDF.js renders page one for review, and the browser does not send provider keys or raw documents to a new service.
- Overlay AI-proposed outlines, wall paths, and opening anchors on the uploaded drawing. Users can select, move, redraw, remove, and associate features with room IDs; distinguish suggested from corrected and confirmed geometry, and show unresolved features in a list.
- Provide a separate "Confirm layout" action after discrepancy and topology validation. Changing a dimension or layout later makes the combined model stale until the relevant confirmations are updated. Do not make layout confirmation a prerequisite to the existing numeric estimate flow.

## 3D Construction

- Replace the standalone conceptual boxes only when a compatible confirmed layout exists. Extrude verified wall paths to confirmed heights using Three.js; place openings only at confirmed anchors. Maintain the 2D drawing overlay and revision labels beside the model so users can assess resemblance.
- Model materials, wall thickness, and surfaces that the drawing does not establish must remain visibly schematic or absent. Keep orbit/zoom/reset, accessible text geometry summaries, mobile framing, and a 2D fallback when WebGL fails.
- Preserve the earlier numeric confirmation and estimate services unchanged; the viewer must not generate or write QS measurements. When no layout exists, show "Layout not verified" instead of independent room boxes implying a matched model.

## Delivery Order and Verification

1. Approve the plan-matched single-floor scope amendment; settle the geometry schema, source-page provenance, retrieval authorization, and independent confirmation semantics.
2. Add drawing page retrieval/rendering and deterministic coordinate mapping; test image and rotated PDF overlays with known landmarks.
3. Add optional AI trace parsing, server-side geometry/topology/scale validation, revisioned persistence, and API tests for missing, invalid, stale, and mismatched geometry.
4. Build 2D trace/correction/confirmation UI against fixtures, then integrate provider candidates and server validations.
5. Build 3D extrusion only from matching confirmed dimension and layout revisions, with no-geometry and renderer-failure fallback.
6. Test against at least two known floor-plan fixtures, comparing annotated 2D trace and 3D relative room positions, shared walls, and verified openings at desktop/mobile sizes. Compare estimates and exported workbook values before and after layout confirmation; verify no layout-derived values enter calculations.

## Open Decisions Before Tasks

- Choose the authentication/authorization boundary for stored drawings before exposing them to a browser route; the current MVP API has no user accounts.
- Set a tolerance and review policy for conflicts between dimension labels and traced outline scale; never silently pick one.
- Decide whether an AI provider can reliably propose spatial geometry for the supported PDF/image types. Manual tracing must remain possible and incomplete proposals must not block estimates.