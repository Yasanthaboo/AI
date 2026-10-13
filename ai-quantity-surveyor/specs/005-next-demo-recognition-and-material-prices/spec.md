# Feature Specification: Next-Demo Recognition and Local Material Prices

**Feature:** 005-next-demo-recognition-and-material-prices
**Created:** 2026-10-13
**Status:** Proposed; no implementation claimed
**Baseline:** Constitution 1.4.0 and implemented spec 004. Proposed amendment 1.5.0 requires ratification before scope-expanding implementation.

## Problem and Demo Goal

Customer feedback reports missed stairs, balconies, and windows on some plans. Customers also want material requirements and comparable local prices. Demonstrate better recognition/review first, then a reproducible interior-finish material budget with sourced price options. Do not market estimated placement as verified geometry or indicative catalog prices as live quotations.

## User Scenarios and Testing

### US1 - Review architectural features (Priority P1)

As a surveyor, I can find proposed windows, stairs, and balconies on the source drawing, inspect uncertainty, and correct missing or incorrect detections before confirmation.

**Independent test:** An annotated plan with all three feature types supports visible detections, manual addition/removal, explicit confirmation, and unchanged existing QS outputs when only informational features change.

### US2 - Understand extraction evidence (Priority P1)

As a surveyor, I can see which source page/region and field uncertainty support a value, distinguish detected geometry from indicative placement, and resolve a window's room association.

**Independent test:** A window with unreadable size remains visible as uncertain; its missing required dimensions/association block deductions until corrected or removed. Missing evidence never becomes a fabricated measurement.

### US3 - Budget finish materials (Priority P2)

As a surveyor, I can use confirmed paint/tile quantities and reviewed product coverage/pack information to calculate material requirements, whole packs, and surplus separately from my finish-rate estimate.

**Independent test:** Fixed quantities/specifications produce exact reproducible requirements and pack rounding, with no alteration to existing labour/composite finish-rate totals.

### US4 - Compare local prices (Priority P3; source approval gated)

As a customer, I can choose a location, compare equivalent paint/tile offers from approved sources, understand tax/delivery/freshness limitations, and save a comparison snapshot.

**Independent test:** Approved catalog fixtures yield comparable alternatives and transparent totals. Unavailable/stale/incomparable offers are identified rather than ranked as verified cheapest options.

### US5 - Revisit the hosted demo (Priority P1 release gate)

As a customer, I can open the hosted demo after an update and retrieve saved review/budget data, without exposing private drawings or provider secrets.

**Independent test:** A deploy/restart preserves PostgreSQL records; web/API access protection or a documented sample-only restriction is verified. Both provider configuration and cold-start/error behavior are observable.

## Functional Requirements

- **FR-001 Feature extraction.** Add optional stairs/balconies candidates and improve window detection across configured vision adapters. Distinguish feature types; do not coerce stairs/balconies into priced rectangular rooms. Keep unsupported/missing fields explicit.
- **FR-002 Evidence.** Record source drawing/analysis identity, source page, optional normalized region, field-level confidence and uncertainty, and original candidate values. Image page is 1; PDF evidence must refer to the page actually processed/rendered. Omitted evidence is unknown, not proof of absence.
- **FR-003 Review.** Users can select features on an overlay/list, edit/add/remove them, assign a window to a room, and explicitly acknowledge informational items. Preserve original extraction and user corrections in immutable confirmation revisions. Legacy confirmations remain readable without new fields.
- **FR-004 Validation boundary.** Stairs/balconies are informational, excluded from all QS formulas. Invalid informational geometry produces review warnings or requires correction/exclusion, not a new dependency for estimates. Required window dimensions/room associations keep existing confirmation validation. Never use image bounds as measured dimensions.
- **FR-005 Model annotations.** Show reviewed stair/balcony annotations only where source geometry is usable; otherwise provide text. No invented stair rise/run, elevation, structure, or railing. Window evidence regions are not verified wall anchors. Existing indicative 3D openings stay labeled indicative; geometry changes never change QS results.
- **FR-006 Paint/tile requirements.** Deterministically calculate paint litres and tile square metres from a frozen confirmed quantity result, reviewed coverage/coats or pack coverage, and explicit waste. Round purchasing requirements up to whole packs; show measured quantity, required material, purchased quantity, and surplus. Missing yield/pack data blocks the affected budget item, not the ordinary estimate.
- **FR-007 Separate budgets.** Store versioned material-only budgets referencing estimate ID and exact confirmation revision, reviewed recipe/product inputs, formula version, selections, timestamps, and assumptions. Do not silently add material totals to existing finish-rate estimates. Waste is applied once to material requirements; do not reuse waste-adjusted finish costs as quantities. Existing estimates/exports are unchanged.
- **FR-008 Location.** Manual country, city/postcode, and optional radius are primary inputs. Browser geolocation requires consent and has a manual fallback. Do not infer delivery coverage from distance alone or persist exact coordinates without need/consent. Initial pilot is one approved region.
- **FR-009 Offers.** Support two or three approved suppliers via authorized feeds/APIs, permitted scraping, or maintained imports. Record source URL/method, supplier/branch, product specification, pack size/unit, currency, observed time, availability if known, tax and delivery inclusion. Manual/imported/demo prices are clearly labeled.
- **FR-010 Fair comparison.** Match specification/grade/use as well as units. Show unit price and total whole-pack cost, known taxes/delivery, and unresolved charges. Different currencies require explicit captured conversion or separate comparison groups. Budget/standard/premium labels require reviewed category criteria, not AI quality claims. Mixed-supplier baskets do not sum repeated delivery charges per item.
- **FR-011 Collection policy.** Before any live scrape, review terms/permission and robots policy per domain; robots compliance alone is not permission. Use allowlisted public product URLs, bounded scheduled/manual refresh, rate limits, timeouts, backoff, caching, and freshness thresholds. Never bypass controls; no arbitrary customer-supplied fetch URLs. No live web search/scrape on every customer page load.
- **FR-012 Failure/freshness.** Show missing, stale, quote-only, out-of-stock, and unparseable offers explicitly. Do not substitute AI prices, assume unknown charges are zero, or imply availability from missing data. Failed refresh retains the last observed offer with its old timestamp and an error state; it never changes saved comparisons.
- **FR-013 Saved comparisons.** Save immutable selected offers and all pricing/quantity/conversion assumptions. Later catalog changes do not mutate prior budgets. A new selection/refresh creates a new snapshot. Provide a material-only comparison export with matching values and source references, separate from existing QS export.
- **FR-014 Hosted release.** Retain Render web/API and Neon persistence, Git-triggered deployments, environment-only secrets, configured web origin/API URL, and provider availability checks. Protect both web and API before confidential use; otherwise use isolated sample data and provider-call limits. Do not claim CORS is authentication.

## Acceptance Criteria

1. **AC-01 (FR-001-005):** A fixture containing stairs, a balcony, and windows can be reviewed/corrected with source evidence and uncertainty; informational features do not enter QS formulas.
2. **AC-02 (FR-002-004):** Missing/ambiguous window size or association is not fabricated; confirmation for deductions requires correction/removal. Duplicate IDs, invalid references, nonfinite confidence, and out-of-bounds evidence receive named validation results.
3. **AC-03 (FR-003-005):** Reload/re-analysis preserves identifiable original candidates and confirmation revisions. A mismatched drawing/page cannot display stale evidence as current. WebGL failure retains drawing/list review.
4. **AC-04 (FR-006-007):** 100 m2 paint area, two coats, 10 m2/litre/coat, 10% waste, and 5-litre packs yields 22 litres required, 5 packs, 25 litres purchased, 3 litres surplus. 20 m2 tiles, 10% waste, and 1.5 m2/pack yields 22 m2 required, 15 packs, 22.5 m2 purchased, 0.5 m2 surplus.
5. **AC-05 (FR-007,013):** Informational feature/overlay interaction and material-price comparison leave existing estimate and QS workbook quantities/costs unchanged; saved budgets survive restart and include exact source revisions.
6. **AC-06 (FR-008-010):** Manual location works without geolocation consent. Comparable offers display specification, branch, whole-pack cost, currency, freshness, and tax/delivery coverage; unknown charges prevent a complete delivered-total claim.
7. **AC-07 (FR-009-012):** A failed/stale/blocked source remains visibly unavailable or stale, retains provenance, and never fabricates a current offer. Unapproved domains cannot be fetched.
8. **AC-08 (FR-010,013):** Later prices do not mutate saved comparison/export values. Incomparable specifications/currencies cannot be ranked as equivalent or advertised as the cheapest delivered option.
9. **AC-09 (FR-014):** Hosted desktop/mobile workflows, API/web protection or sample-only release restrictions, provider setup, restart persistence, and deploy regression checks are recorded before the customer demo.
10. **AC-10 (FR-001-005):** Report feature precision/recall and dimension accuracy on an annotated holdout set against the current analyzer baseline; targets and fixtures are agreed before tuning. Improvements must reduce the observed window misses without hiding false positives or room/dimension regressions.

## Scope, Assumptions, and Decisions

- P1 recognition/review is the first demo increment. P2 material budgeting can use curated product fixtures. P3 live prices is conditional on region, supplier permissions, and data readiness; use explicitly labeled fixture/imported prices if gates are unresolved.
- Paint and tiles are proposed pilot categories, not confirmed supplier coverage. Product consumption inputs require documented supplier specifications or manual surveyor approval.
- One residential drawing/floor stays the limit. PDF feature evidence initially targets the supported review page; unprocessed pages are explicitly unsupported. Ollama PDF rasterization remains a known dependency, not an assumed delivered capability.
- Deferred: cement/sand/plaster recipes, structural quantities, external balcony finishes, stair costing, full BOQ, purchases/procurement, supplier management, multiple drawings/floors, verified BIM/CAD, autonomous agents, LangGraph/MCP, RAG knowledge assistants, and arbitrary web-wide scraping.
- Region/currency, supplier list/access policy, freshness interval, pack/spec equivalence rules, accuracy targets, and hosted access mechanism are open decisions. No customer date or reliable live-price claim is committed before those decisions.
- RAG may be reconsidered for cited catalog retrieval after trustworthy ingestion; a second-model verification pass may be reconsidered only after measured extraction errors justify its latency/cost. Neither is an initial-demo dependency.

## Success and Release Evidence

Recognition evaluation reports are reproducible with fixture versions and engine/model/prompt identifiers. Material examples match AC-04 exactly. Every presented offer has provenance or a clear fixture label. The demo shows at least two equivalent source options only when such offers actually exist; otherwise it shows honest incomplete coverage. All related tasks remain unchecked until implemented and verified.