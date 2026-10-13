# Feature Specification: Next-Demo Recognition and Local Material Prices

**Feature:** 005-next-demo-recognition-and-material-prices
**Created:** 2026-10-13
**Status:** Initial clarification pass complete on 2026-10-13: selected-page scope, recognition targets, dimension preview and Colombo/LKR pilot recorded. Supplier shortlist explicitly deferred; operational approval gates and implementation remain pending under constitution 1.5.0.
**Baseline:** Implemented spec 004; constitution 1.5.0 authorizes bounded P1-P3 extensions. Region/source approvals and private-data release gates remain unresolved.

## Problem and Demo Goal

Customer feedback reports missed stairs, balconies, and windows on some plans. Customers also want material requirements and comparable local prices. Demonstrate better recognition/review first, then a reproducible interior-finish material budget with sourced price options. Do not market estimated placement as verified geometry or indicative catalog prices as live quotations.

## User Scenarios and Testing

### US1 - Review architectural features (Priority P1)

As a surveyor, I can find proposed windows, stairs, and balconies on the source drawing, inspect uncertainty, and correct missing or incorrect detections before confirmation.

**Independent test:** An annotated plan with all three feature types supports visible detections, manual addition/removal, explicit confirmation, and unchanged existing QS outputs when only informational features change.

**Acceptance scenarios:**
1. Given a readable plan containing these features, when analysis completes, then detected windows/stairs/balconies are distinguished and selectable with available evidence and uncertainty.
2. Given a missed or incorrect feature, when the surveyor adds, edits or removes it and confirms the review, then the correction is preserved and distinguished from the original extraction.
3. Given only informational stair/balcony edits, when an estimate is produced from otherwise identical confirmed numeric inputs, then existing QS quantities and costs are unchanged.

### US2 - Understand extraction evidence (Priority P1)

As a surveyor, I can see which source page/region and field uncertainty support a value, distinguish detected geometry from indicative placement, and resolve a window's room association.

**Independent test:** A window with unreadable size remains visible as uncertain; its missing required dimensions/association block deductions until corrected or removed. Missing evidence never becomes a fabricated measurement.

**Acceptance scenarios:**
1. Given a detected window with unreadable size, when the surveyor reviews it, then the window remains visible as uncertain and cannot contribute a deduction until its required data is corrected or the opening is removed.
2. Given an evidence reference from a different drawing/page, when the review is opened, then it is identified as invalid rather than overlaid as current evidence.
3. Given a window evidence region without a reviewed wall anchor, when the model is shown, then its 3D placement remains indicative rather than claimed as verified.

### US3 - Budget finish materials (Priority P2)

As a surveyor, I can use confirmed paint/tile quantities and reviewed product coverage/pack information to calculate material requirements, whole packs, and surplus separately from my finish-rate estimate.

**Independent test:** Fixed quantities/specifications produce exact reproducible requirements and pack rounding, with no alteration to existing labour/composite finish-rate totals.

**Acceptance scenarios:**
1. Given confirmed paint/tile quantities and reviewed consumption/pack data, when a material budget is calculated, then it shows required amount, whole packs, purchased amount, surplus and assumptions separately from the finish-rate estimate.
2. Given missing coverage or pack size, when budgeting is requested, then the affected material is identified as incomplete while the ordinary estimate remains available.
3. Given excluded flooring or a flooring finish that is not tile, when a tile budget is calculated, then that area is not silently included as tile.

### US4 - Compare local prices (Priority P3; source approval gated)

As a customer, I can choose a location, compare equivalent paint/tile offers from approved sources, understand tax/delivery/freshness limitations, and save a comparison snapshot.

**Independent test:** Approved catalog fixtures yield comparable alternatives and transparent totals. Unavailable/stale/incomparable offers are identified rather than ranked as verified cheapest options.

**Acceptance scenarios:**
1. Given a manually selected location and equivalent permitted offers, when the customer compares options, then source, branch, specification, pack quantity, price freshness and known charges are visible.
2. Given missing delivery/tax data, when an offer is displayed, then its subtotal may be shown but it cannot be labeled a complete delivered total.
3. Given a saved comparison and a later price change, when the saved comparison is reopened/exported, then its original prices and assumptions are preserved.

### US5 - Revisit the hosted demo (Priority P1 release gate)

As a customer, I can open the hosted demo after an update and retrieve saved review/budget data, without exposing private drawings or provider secrets.

**Independent test:** A deploy/restart preserves saved records; access protection or a documented sample-only restriction is verified. Analysis availability and startup/error behavior are observable.

**Acceptance scenarios:**
1. Given saved confirmations/budgets/comparisons, when the application is updated or restarted, then those records remain retrievable without changes to their values.
2. Given confidential customer data, when an unauthorized visitor requests projects, drawings or exports, then access is denied; without verified protection the demo must use isolated sample data only.
3. Given an unavailable analysis provider or supplier source, when the customer opens the workflow, then an understandable unavailable/error state is shown and existing saved estimates remain readable.

### US6 - Preview and confirm dimensions (Priority P1)

As a surveyor, I can compare editable room and opening dimensions with the selected source drawing before explicitly confirming the data used for quantities and costs.

**Independent test:** Select a room/window on the drawing or review list, inspect its dimensions/units and available evidence, correct a value, and confirm a new revision. Opening or editing the preview alone never confirms data or changes prior estimates.

**Acceptance scenarios:**
1. Given extracted dimensions, when the surveyor opens review before confirmation, then the selected drawing page and editable room length/width/wall height and door/window width/height are available together, with zoom/pan and linked selection where evidence exists.
2. Given a changed dimension/unit, when the surveyor edits it, then the pending preview values update and remain distinguishable from the original extraction and last confirmed revision; explicit validation and confirmation are still required.
3. Given missing evidence or an unreadable/defaulted value, when it is selected, then the preview identifies missing evidence/uncertainty or the configured assumption rather than inventing a dimension location or value.
4. Given an unavailable drawing renderer, when review continues, then a readable dimension list and source-page/error status remain available; ordinary correction/validation/confirmation is not blocked solely by the renderer failure.

### Edge Cases

- A feature symbol may resemble another object: show uncertainty and permit correction; absence of a detection does not prove the feature is absent.
- Duplicate detections/shared openings must not create duplicate deductions; resolve IDs and room associations during review.
- A stair/balcony lacks reliable placement: retain its informational list entry without fabricating geometry or blocking ordinary estimates.
- A rotated/cropped drawing or unprocessed PDF page cannot use coordinates from another orientation/page as valid evidence.
- Preview labels may overlap or lack a source region: show dimensions in a linked panel/list rather than drawing misleading dimension lines or hiding values. Zoom/pan must not change physical dimensions.
- Unconfirmed preview edits must not mutate the last confirmed revision, its model, or existing estimates/exports; a new confirmation creates a new revision.
- A historical estimate lacks an exact confirmation revision: do not associate it with today's latest revision; require a new traceable estimate or reject the material budget with an explanation.
- Compatible room allocations may share purchased packs; incompatible finishes/products cannot be merged solely to reduce cost. Exact pack boundaries must not round up unnecessarily.
- A supplier lists a price range, promotional condition or quote-only product: do not select the lowest possible value as an unconditional offer.
- Two branches show different prices/stock: preserve branch-specific offers; nearby does not imply delivery coverage.
- A mixed-supplier basket has incomplete delivery rules: show incomplete totals rather than summing per-item delivery or claiming a cheapest complete basket.
- A customer declines location access, no equivalent local offers exist, or collection fails: manual location and ordinary estimation stay usable; clearly distinguish fixtures, stale observations and missing coverage.

## Functional Requirements

- **FR-001 Feature extraction.** Identify optional stairs/balconies candidates and improve window detection through supported analysis providers. Distinguish feature types; do not coerce stairs/balconies into priced rectangular rooms. Keep unsupported/missing fields explicit.
- **FR-002 Evidence.** Record source drawing/analysis identity, source page, optional normalized region, field-level confidence and uncertainty, and original candidate values. Image page is 1. For PDFs, the user selects exactly one page before analysis; extraction, evidence, drawing review and model annotations must all refer to that selected page. Unselected pages remain unprocessed. Changing the selected page requires a new analysis and cannot relabel an existing confirmed snapshot. Omitted evidence is unknown, not proof of absence.
- **FR-003 Review.** Users can select features on an overlay/list, edit/add/remove them, assign a window to a room, and explicitly acknowledge informational items. Preserve original extraction and user corrections in immutable confirmation revisions. Legacy confirmations remain readable without new fields.
- **FR-004 Validation boundary.** Stairs/balconies are informational, excluded from all QS formulas. Invalid informational geometry produces review warnings or requires correction/exclusion, not a new dependency for estimates. Required window dimensions/room associations keep existing confirmation validation. Never use image bounds as measured dimensions.
- **FR-005 Model annotations.** Show reviewed stair/balcony annotations only where source geometry is usable; otherwise provide text. No invented stair rise/run, elevation, structure, or railing. Window evidence regions are not verified wall anchors. Existing indicative 3D openings stay labeled indicative; geometry changes never change QS results.
- **FR-006 Paint/tile requirements.** Deterministically calculate paint litres and tile square metres from a frozen confirmed quantity result, reviewed coverage/coats or pack coverage, and explicit waste. Round purchasing requirements up to whole packs; show measured quantity, required material, purchased quantity, and surplus. Missing yield/pack data blocks the affected budget item, not the ordinary estimate.
- FR-006 validation clarification (A3): Reject required positive coverage/pack inputs that become zero at the supported material precision, identifying the affected field; never silently substitute a minimum. A positive source quantity below supported precision must not appear as zero required packs. Existing QS estimation remains available when an affected material budget is invalid.
- **FR-007 Separate budgets.** Store versioned material-only budgets referencing estimate ID and exact confirmation revision, reviewed recipe/product inputs, formula version, selections, timestamps, and assumptions. Do not silently add material totals to existing finish-rate estimates. Waste is applied once to material requirements; do not reuse waste-adjusted finish costs as quantities. Existing estimates/exports are unchanged.
- FR-007 allocation clarification (A2): Each room/trade has at most one material allocation per budget; reject duplicates before aggregating quantities, naming the room/trade. Derive areas from confirmed estimate quantities, not submitted areas. Compatible allocations from different rooms may share packs after validation; splitting one room/trade between multiple products/areas is deferred. No additional confirmation screen is required.
- **FR-008 Location.** Manual country, city/postcode, and optional radius are primary inputs. Browser geolocation requires consent and has a manual fallback. Do not infer delivery coverage from distance alone or persist exact coordinates without need/consent. Initial pilot is Colombo, Sri Lanka, with local-price comparisons in Sri Lankan rupees (LKR). Nationwide supplier/delivery coverage is not assumed. Supplier prices in other currencies follow FR-010; this pilot does not change the existing USD-base QS estimate behavior.
- **FR-009 Offers.** Support two or three approved suppliers via authorized feeds/APIs, permitted scraping, or maintained imports. The supplier shortlist is explicitly deferred for this demo; use clearly labeled fixture/imported prices for Colombo/LKR until sources/access policies are approved, with no live/current-price claim. Record source URL/method, supplier/branch, product specification, pack size/unit, currency, observed time, availability if known, tax and delivery inclusion. Manual/imported/demo prices are clearly labeled; fixtures must not impersonate real supplier quotations.
- **FR-010 Fair comparison.** Match specification/grade/use as well as units. Show unit price and total whole-pack cost, known taxes/delivery, and unresolved charges. Different currencies require explicit captured conversion or separate comparison groups. Budget/standard/premium labels require reviewed category criteria, not AI quality claims. Mixed-supplier baskets do not sum repeated delivery charges per item.
- FR-010 preview clarification (A1): Before saving, show a server-calculated, non-saving preview of selected alternative baskets with required packs, material cost, known taxes/delivery and incomplete-total warnings. Explicit Save recalculates against selected price observations; unavailable/expired selections require renewed review rather than silent replacement or trust in preview totals.
- **FR-011 Collection policy.** Before any live scrape, review terms/permission and robots policy per domain; robots compliance alone is not permission. Collect only approved public product sources at approved bounded frequencies. Customer comparison reads stored observations rather than requesting live collection on every page load. Never bypass controls or allow arbitrary customer-supplied fetch URLs; failed collection must not imply current prices.
- **FR-012 Failure/freshness.** Show missing, stale, quote-only, out-of-stock, and unparseable offers explicitly. Do not substitute AI prices, assume unknown charges are zero, or imply availability from missing data. Failed refresh retains the last observed offer with its old timestamp and an error state; it never changes saved comparisons.
- **FR-013 Saved comparisons.** Save immutable selected offers and all pricing/quantity/conversion assumptions. Later catalog changes do not mutate prior budgets. Explicitly saving a new selection or recalculating with refreshed prices creates a new comparison snapshot; background collection does not silently replace a saved comparison. Provide a material-only comparison export with matching values and source references, separate from existing QS export.
- **FR-014 Hosted release.** Provide a remotely accessible customer demo with durable saved records and repeatable deployments after approved changes. Keep provider secrets out of source/browser code, show analysis availability and recoverable startup errors, and protect both web and API before confidential use; otherwise use isolated sample data and provider-call limits. Do not claim CORS is authentication.
- FR-014 reliability/usage clarification (A4, A6): Recovery must not fail analysis work still owned by another live instance; expired/lost ownership prevents stale results from being published. Enforce persistent daily per-provider call limits shared across deployed instances, reserve before each outbound attempt/retry, reset at UTC midnight and show an explicit quota-limit message. Missing configuration or unavailable quota storage must not enable unlimited calls. Call-count limits do not guarantee a monetary spending cap.
- **FR-015 Dimension-confirmation preview.** Before explicit confirmation, provide a source-drawing preview with zoom/pan and a linked editable dimension panel/list: room length/width/wall height and door/window width/height, entered units and converted metre equivalents. Selecting a room/opening or dimension highlights its source evidence when available; missing evidence remains visibly unknown. Distinguish original extraction, pending corrections, configured defaults/assumptions and last confirmed values/revision. Show pending edits immediately and provide a clearly identified confirmation action only after required values/associations pass existing validation. Viewing/selecting/editing/zooming is never confirmation, and pixel distances or schematic dimension lines never supply measurements. Keep labels/panels readable on desktop/mobile, with a dimension-list/error fallback if the drawing cannot render. Existing estimates remain unchanged until a new confirmed revision and explicit recalculation.

## Key Entities

- **Architectural feature:** a window, stair or balcony with identity/type, source evidence, uncertainty and review disposition. Windows remain measured openings when confirmed; stairs/balconies remain informational.
- **Source evidence:** drawing/analysis/page identity, optional region and observed text, field confidence, and uncertainty supporting an extracted value or feature.
- **Confirmation revision:** an immutable reviewed snapshot preserving corrected values, original extraction references and manual dispositions.
- **Material specification:** reviewed category/use, coverage/coats, pack size/unit, source or explicit assumption, and compatibility criteria.
- **Material requirement:** frozen confirmed source quantity and computed required/purchased amounts, whole packs, surplus, exclusions and waste assumptions.
- **Material budget:** an immutable material-only version linking exact estimate/confirmation provenance, reviewed specifications and calculated requirements; distinct from the existing finish-rate estimate.
- **Location selection:** manually supplied country/city/postcode and optional radius or consented coordinates; not proof of supplier delivery coverage.
- **Supplier offer:** branch-specific observed product/pack price with specification, currency, source/method/time, availability and known/unknown tax/delivery conditions.
- **Saved comparison:** immutable selected offer values and quantity/charge/conversion assumptions, with a matching export and complete/incomplete-total status.

## Acceptance Criteria

1. **AC-01 (FR-001-005):** A fixture containing stairs, a balcony, and windows can be reviewed/corrected with source evidence and uncertainty; informational features do not enter QS formulas.
2. **AC-02 (FR-002-004):** Missing/ambiguous window size or association is not fabricated; confirmation for deductions requires correction/removal. Duplicate IDs, invalid references, nonfinite confidence, and out-of-bounds evidence receive named validation results.
3. **AC-03 (FR-002-005):** A PDF page selected before analysis is used consistently for extraction/evidence/review. Changing the selected page starts a new analysis; reload/re-analysis preserves identifiable original candidates, source page and confirmation revisions. Unselected pages are marked unprocessed, and a mismatched drawing/page cannot display stale evidence as current. WebGL failure retains drawing/list review.
4. **AC-04 (FR-006-007):** 100 m2 paint area, two coats, 10 m2/litre/coat, 10% waste, and 5-litre packs yields 22 litres required, 5 packs, 25 litres purchased, 3 litres surplus. 20 m2 tiles, 10% waste, and 1.5 m2/pack yields 22 m2 required, 15 packs, 22.5 m2 purchased, 0.5 m2 surplus.
5. **AC-05 (FR-007,013):** Informational feature/overlay interaction and material-price comparison leave existing estimate and QS workbook quantities/costs unchanged; saved budgets survive restart and include exact source revisions.
6. **AC-06 (FR-008-010):** Manual location works without geolocation consent. Comparable offers display specification, branch, whole-pack cost, currency, freshness, and tax/delivery coverage; unknown charges prevent a complete delivered-total claim.
7. **AC-07 (FR-009-012):** A failed/stale/blocked source remains visibly unavailable or stale, retains provenance, and never fabricates a current offer. Unapproved domains cannot be fetched.
8. **AC-08 (FR-010,013):** Later prices do not mutate saved comparison/export values. Incomparable specifications/currencies cannot be ranked as equivalent or advertised as the cheapest delivered option.
9. **AC-09 (FR-014):** Hosted desktop/mobile workflows, API/web protection or sample-only release restrictions, provider setup, restart persistence, and deploy regression checks are recorded before the customer demo.
10. **AC-10 (FR-001-005):** Evaluate at least 10 consented holdout plan pages, with at least 20 annotated instances of each feature type across the set, excluded from tuning. Windows require at least 90% precision and 90% recall; stairs and balconies each require at least 85% precision and 85% recall. At least 95% of legible labeled room/opening dimensions must be extracted with correct units and within 2% of annotated values; missing predictions count as failures. Report results separately per provider/model, reduce window misses versus the recorded baseline, and do not worsen baseline room/dimension accuracy. Fixture acquisition/annotation and matching/scoring rules must be completed before tuning; no claim that these targets are already achieved.
11. **AC-11 (FR-002-004, FR-015):** Before confirmation, the selected source page, editable dimension panel, original values, pending corrections, units/metre equivalents and assumptions are distinguishable. Selecting a room/window links to available evidence; missing evidence and renderer failure yield explicit status and a usable list fallback. Editing a value updates pending preview data but not the last confirmed revision or prior estimate/export. Invalid required dimensions/associations block confirmation; valid explicit confirmation produces a new immutable revision. Preview use never derives QS measurements from pixels or confirms automatically. Verify readable desktop/mobile layout and zoom/pan behavior.

### Additional Checks for Existing Acceptance Criteria

- **AC-04 / A3:** Normalization-boundary inputs that become zero yield named errors without division/hidden substitution; valid boundary and ordinary recipe inputs remain deterministic.
- **AC-04 and AC-05 / A2:** Duplicate `(roomId, trade)` allocations are rejected even when their recipe/group differs, without saving a partial budget. Unique compatible allocations across rooms aggregate correctly; manual confirmation does not authorize client-supplied areas or repeated allocations.
- **AC-06 and AC-08 / A1:** Preview changes no saved record/version and shows authoritative packs/charges/warnings; Save recomputes from the chosen observations. Newer prices do not silently replace selected IDs, and expired selections require renewed review.
- **AC-09 / A4:** During overlapping deployments, recovery ignores live ownership and stale owners cannot publish after expiry/recovery. Heartbeat loss is handled without overwriting confirmed results.
- **AC-09 / A6:** Concurrent instances cannot exceed the configured daily reservation limit; each retry consumes a new reservation, restart preserves usage, UTC rollover creates the next bucket, and quota/config/storage rejection never calls the provider. Saved review data remains usable.
- **AC-10 / A5:** Evaluate raw extraction before human corrections with separate detection, dimensions and localization metrics. The same approved precision/recall thresholds apply to new detection and localization; IoU >=0.5 defines a localization match only. Legacy localization without support is labeled `notSupported`, not treated as zero detection accuracy. Freeze identity/association matching and count duplicates/unmatched/ambiguous predictions. Baseline regression/improvement comparisons use jointly supported raw metrics; human-confirmation success is separate workflow evidence, never AI accuracy. Existing numeric thresholds and dataset size remain unchanged.

## Scope, Assumptions, and Decisions

- P1 recognition/review is the first demo increment. P2 material budgeting can use curated product fixtures. P3 live prices is conditional on region, supplier permissions, and data readiness; use explicitly labeled fixture/imported prices if gates are unresolved.
- Paint and tiles are approved pilot categories, not confirmed supplier coverage. Product consumption inputs require documented supplier specifications or manual surveyor approval.
- Pilot is Colombo, Sri Lanka, with LKR local-price comparisons, selected by the owner. Candidate suppliers remain deliberately undecided; until approved, the price demo uses labeled fixtures/imports rather than live scraping. Location selection is not source permission or proof of delivery coverage.
- One residential drawing/floor stays the limit. PDFs support exactly one user-selected page per analysis, not combined multi-page/floor extraction. Unselected pages remain explicitly unprocessed. Provider-compatible processing of the selected page, including Ollama PDF rasterization, is an implementation dependency, not an assumed delivered capability.
- Deferred: cement/sand/plaster recipes, structural quantities, external balcony finishes, stair costing, full BOQ, purchases/procurement, supplier management, multiple drawings/floors, verified BIM/CAD, autonomous agents, LangGraph/MCP, RAG knowledge assistants, and arbitrary web-wide scraping.
- Supplier list/access policy, freshness interval, pack/spec equivalence rules, annotated fixture availability/scoring rules, and hosted access mechanism remain open operational decisions. Region/currency and recognition thresholds are agreed, but sources, fixtures and measured results are not delivered. No customer date or reliable live-price claim is committed before the relevant gates are resolved.
- RAG may be reconsidered for cited catalog retrieval after trustworthy ingestion; a second-model verification pass may be reconsidered only after measured extraction errors justify its latency/cost. Neither is an initial-demo dependency.

## Clarifications

### Session 2026-10-13

- Q: Which PDF page should the recognition demo analyze and show evidence against? A: One user-selected page; selection occurs before analysis and all extraction/evidence/review use that page. Other pages are unprocessed; a page change requires a new analysis.
- Q: Which annotated fixture set and accuracy thresholds establish the recognition release gate? A: At least 10 consented holdout pages and 20 instances per feature type; windows precision/recall at least 90% each, stairs/balconies at least 85% each, and at least 95% of legible dimensions with correct units and within 2% of annotations. Missing predictions fail; evaluate providers/models separately, reduce baseline window misses and preserve room/dimension accuracy. Fixtures remain to be acquired and annotated.
- Requirement added by owner: Include a preview to confirm dimensions. The P1 review workflow must show the selected drawing with linked editable dimensions before explicit validated confirmation; preview interaction alone never confirms measurements.
- Q: Which country should the local paint/tile price pilot target? A: Sri Lanka. City/region, currency and supplier shortlist have not yet been provided; live-source approvals remain pending.
- Q: Which initial region/currency and suppliers should the Sri Lanka pilot use? A: Colombo and Sri Lankan rupees (LKR). Suppliers remain undecided; proceed with recognition/material budgeting and clearly labeled fixture/imported price comparisons until live sources/access policies are approved.

## Deferred Operational Decisions

Q1-Q3 are answered or explicitly deferred with a defined fallback. Supplier shortlist/permission remains deferred, not approved: T004 must identify and approve two or three sources before live collection. Product equivalence/consumption rules, freshness/refresh limits, fixture scoring and hosted access protection remain planning/release gates, not completed tasks.

Source permissions, freshness/refresh limits, product-equivalence criteria and hosted protection must also be documented before the affected integration/release; ratification does not imply those operational approvals. Detailed schemas and technical choices belong in `contracts.md` and `plan.md`.

## Success Criteria

- **SC-001 Recognition:** Meet all agreed AC-10 holdout thresholds for windows, stairs, balconies and dimensions, reduce baseline window misses, preserve room/dimension accuracy and disclose false positives per provider/model. Self-reported confidence cannot substitute for measured accuracy; no recognition-improvement claim passes before fixture evaluation demonstrates these targets.
- **SC-002 Material correctness:** Both AC-04 examples produce exactly the specified required amounts, pack counts, purchased amounts and surplus; repeat calculations with identical inputs give identical values.
- **SC-003 Price transparency:** Every displayed offer provides its source/method, currency, observed time, pack/specification and explicit availability/tax/delivery states; no offer with unknown charges is labeled a complete delivered total.
- **SC-004 Reproducibility:** All saved confirmation/material/comparison acceptance fixtures retain their values after application restart and subsequent source-price changes; corresponding exports match saved values.
- **SC-005 No QS regression:** Existing golden estimate/workbook fixtures have zero quantity/cost differences when only informational features, model interactions or material-price options change.
- **SC-006 Usable demo:** A surveyor completes recognition review, a paint/tile material budget and a fixture-backed comparison on desktop and mobile without horizontal overflow or overlapping controls. These workflows remain usable without geolocation permission and with unavailable live sources.
- **SC-007 Dimension review:** All AC-11 review fixtures support inspecting/correcting required room/opening dimensions before explicit confirmation on desktop/mobile; zero preview-only interactions alter confirmed revisions or prior estimates. A failed drawing renderer retains list-based review and validation.

## Release Evidence

Recognition evaluation reports must be reproducible with fixture/engine/model/prompt versions; no holdout report is delivered yet. Pure material unit tests match AC-04 exactly. Every future offer must have provenance or a clear fixture label, and comparison coverage cannot be invented. Readiness T002 and bounded foundations T006/T007/T020/T045 are recorded complete alongside governance T001; customer-facing integration and release tasks remain pending.