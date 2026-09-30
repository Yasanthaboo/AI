# Implementation Plan: Single-Floor Conceptual 3D Preview

**Status:** Implemented under constitution 1.1.0; live PostgreSQL backfill and API restart verified in a disposable Compose stack.

## Technical Context

The Next.js client currently holds an unverified analysis candidate and editable room fields. The Go analysis service validates confirmations and makes the latest confirmed `Candidate` available to estimates. PostgreSQL currently upserts confirmation into the same analysis row, so it does not preserve a distinct confirmed revision; `GET /api/analyses/{id}` serves candidates, not confirmed snapshots. Estimates read only confirmed data through `analysis.Confirmed` and must keep that boundary. Existing room records include length, width, and wall height but no coordinates or opening placement.

## Constitution Check

- Preserve the existing AI -> review -> confirmation -> deterministic QS calculation flow. AI produces candidates only; the scene is a read-only presentation of confirmed measurements.
- No room layout, opening placement, adjacency, materials, or measurements may be fabricated or fed back into confirmed data, estimates, or Excel.
- Constitution 1.1.0 approves the single-floor conceptual visualization. Multi-floor and construction-ready modeling remain excluded.

## Architecture and Contracts

`analysis candidate -> human confirmation -> immutable confirmed snapshot -> read-only scene projection -> Three.js preview`

The scene projection is derived from confirmed room measurements and is never a source for the existing `domain.Calculate` path. Do not call a separate generative-image or 3D API: generative AI has already contributed the reviewable extraction candidate. Rendering a model from its unverified output would violate the spec.

- Add an immutable confirmation revision per analysis, with stable `analysisId` and a new revision identifier or monotonically increasing revision number. Persist the snapshot on each successful confirmation; preserve the current `Confirmed(analysisId)` behavior for estimates. Use additive PostgreSQL storage and a corresponding in-memory repository behavior for local development. Do not repurpose estimate version numbers as confirmation revisions.
- Expose the confirmed snapshot and its revision through a read-only API (for example, `GET /api/analyses/{id}/confirmed-data` for latest and `GET /api/analyses/{id}/confirmed-data/{revision}` if older revisions need retrieval after reload). Return 404 when no confirmation exists; require a matching analysis ID. Continue to use the current `PUT /api/analyses/{id}/confirmed-data` for validation and confirmation, returning revision metadata without breaking existing candidate fields.
- Define a client-only scene projection of `analysisId`, confirmed revision, and eligible rooms with confirmed dimensions/units. Convert supported units to meters for rendering using the same conversion factors as Go; preserve original values and units for labels. Reject nonpositive, missing, nonfinite, or unsupported values; never substitute a display dimension. List ineligible rooms and missing fields instead. Openings remain text-only with "Location not verified."
- Keep layout offsets, colors, schematic wall surfaces, and camera state inside the renderer; they are explicitly inferred display choices. Space room volumes apart to avoid implied shared walls or a building footprint. Label the view "Conceptual arrangement; room positions not verified" whenever it shows geometry.

## UI and Interaction

- Add a `Review | 3D preview` view within the existing analysis workspace, available after `AnalysisReady`; keep the review form and estimate workflow accessible without a scene.
- Until a matching confirmed snapshot is loaded, show verification required and the room/opening uncertainty list. After confirmation, load the snapshot returned by the API, and display its analysis ID and revision. If the user edits unconfirmed form data, keep the last confirmed preview visibly tied to its earlier revision. A newly started analysis cannot silently reuse an earlier analysis's preview.
- Use Three.js in a client-only viewer with an unframed, responsive scene, orbit/zoom/reset controls, and text equivalents for every room and uncertainty. Lazy-load the renderer; keep labels and a meaningful text fallback visible if WebGL fails, loading fails, or the browser lacks support.
- Keep rendering failures local to the preview and provide retry; do not block analysis review, confirmation, estimates, or export. Do not write anything from scene state to the analysis or estimate APIs.

## Delivery Sequence

1. Obtain scope approval and update the constitution to allow this limited visualization; keep other exclusions intact.
2. Agree on the confirmed-revision API contract and the behavior of old confirmations when deploying the additive persistence change. Ensure latest-confirmed lookup used by estimates remains compatible.
3. Implement immutable confirmation snapshots and read-only retrieval, with validation, absence, revision, and persistence tests. Avoid altering calculation contracts or provider prompts.
4. Implement the pure scene projection and its invalid/missing-data cases against typed confirmed snapshots; verify source version changes and unit conversion.
5. Implement the client-only Three.js viewer, controls, accessible text list, conceptual warnings, and review-tab integration using mocked confirmation responses first, then connect to the API.
6. Run backend tests, UI typecheck/lint, and end-to-end workflows. Verify nonblank canvas pixels, framing, labels, interaction, and no overlaps at desktop and mobile sizes; compare estimates and exports before and after opening the preview.

## Test and Acceptance Mapping

- Before confirmation, no measured volumes (spec criterion 1); confirmed dimensions and units, conceptual arrangement, and separate rooms (2-3); unplaced openings and invalid rooms shown in text only (4-5).
- Confirmation revision tests and a browser re-analysis/edit scenario prove no stale or silent replacement (6). API 404, WebGL failure, retry, and text fallback preserve review and estimate actions (7).
- Golden confirmed inputs produce identical Go calculation and Excel output regardless of preview navigation (8). Desktop/mobile screenshots and canvas-pixel checks cover readable labels, fit, controls, and fallback (9).

## Decisions and Remaining Verification

- Confirmations now use an additive `analysis_confirmations` table and backfill one revision for previously confirmed rows. Historical revisions that were never stored cannot be reconstructed. Service and HTTP tests pass; the legacy-row backfill and persisted project retrieval were verified after restarting the API against a disposable PostgreSQL container.
- Confirmation updates the preview snapshot before estimate creation, so an estimate failure does not mislabel successfully confirmed geometry as unconfirmed.
- The browser derives camera bounds from confirmed room dimensions and keeps a text-only fallback. Desktop/mobile canvas and viewport checks cover the normal case; very large drawings should still be observed under realistic data loads.