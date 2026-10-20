# Proposed Contracts: Recognition, Materials and Local Offers

**Status:** T002 readiness recorded 2026-10-13; plaster material scope was added by constitution 1.6.0 on 2026-10-14. Analysis/material calculation types, persistence and estimate provenance have implementation/tests; supplier import/refresh, accuracy evidence and release gates remain pending. Existing payloads stay readable; historical provenance is not invented.

## Wire Conventions

JSON uses existing camelCase names. Timestamps are UTC RFC3339; measurements retain `{value, unit}` with supported `m|cm|ft|in`. Nullable data uses null or omitted optional fields, never fabricated zero. IDs remain opaque strings. Errors retain `{errorCode, message, retryable}` according to the existing server convention; affected IDs/fields are named in messages, with additive `details` only if agreed in T002. New snapshot version numbers are independent from confirmation/estimate revisions.

All project-scoped references must be checked against stored project ownership. Client totals, provider identity/source provenance and confirmation-revision assignment are never trusted. When deployed behind future access protection, the same boundary must cover these routes; CORS is not authorization.

### Implementation Request Bounds (T002)

New analysis-start bodies are limited to 64 KiB. New confirmation bodies are limited to 1 MiB with at most 500 rooms, 500 total openings and 500 informational features. Material-budget bodies are limited to 64 KiB/256 allocations. Comparison preview/save bodies are limited to 64 KiB, at most 20 alternative baskets and 256 total selections per request. Reject oversize bodies with 413 and invalid item counts with 400 before work/version allocation. Cached catalog queries use default limit 50, maximum 100, with an opaque continuation cursor. Operator catalog imports are capped at 10 MiB/1000 products/10000 observations. These are finalized interface limits for pending handler/import tasks, not claims of current enforcement. Original drawing upload limits and existing QS formulas remain unchanged.

## Analysis Extensions

- `Candidate.features?`: array of informational `ArchitecturalFeature` records: stable `id`, `type` (`stair` or `balcony`), label, optional room reference, nullable evidence, confidence, uncertainty, and review disposition.
- `Room.evidence?`, `Opening.evidence?`: per-field source evidence (`length`, `width`, `wallHeight`, `width/height` for openings, identity/association as appropriate). Each entry identifies server-bound `analysisId`, `floorPlanId`, page number, nullable normalized region, optional observed text, confidence and uncertainty. Do not treat observed text as a command or trusted numeric input.
- Region coordinates are finite fractions in [0,1], with positive extent wholly inside the source page. References must match the processed drawing/page. Missing region/measurement remains null; confidence is not a guarantee.
- Original engine candidate is retained. Confirmations store reviewed values, changed fields/dispositions, evidence references, and immutable revision metadata. Clients cannot rewrite provider identity or evidence provenance. Manual additions are visibly manual, not engine evidence.
- Windows still use existing validated measured width/height and room association for deductions. Stairs/balconies never enter the domain confirmed-room/opening calculation conversion. A window evidence region does not establish a 3D wall anchor.
- `Candidate.source?`: server-owned `{floorPlanId, pageNumber, selectionMode, imageDigest, pixelWidth, pixelHeight}`; `selectionMode` is `explicit|legacy-default|image`. The evidence coordinate system is the canonical rotated/cropped PNG. An evidence entry references the same source and original page, even though the provider receives one PNG.
- `Candidate.reviewIssues?`: entries `{entityId, field, code, message, blocking}`. Missing numeric fields are reviewable until confirmation; unusable optional regions are excluded with a named issue. Strict confirmed input validation is unchanged for calculated measurements. Manual entries use separate `manual` origin/disposition, not fabricated provider confidence.
- Analysis-start input includes optional `pageNumber`: new PDF UI requires an explicit positive integer; omitted legacy requests use page 1 and record `legacy-default`. Images require page 1. Validate against actual PDF metadata, rasterize only that page, and persist a digest/image before provider analysis. Reject out-of-range pages; changing page creates a new analysis. Historical source metadata stays unknown, not silently assigned.
- Confirmation adds server-derived `corrections?`: `{entityId, field, originalValue, confirmedValue, origin}` with origin `user|configured-default`; retain the original candidate as the comparison source. UI may submit validated manual source marks, but cannot rewrite engine evidence or analysis source identity. Evidence IDs for deleted/changed entities are checked; revision/digest metadata is copied from stored source.

Extend current `GET /api/analyses/{id}` and `PUT/GET /api/analyses/{id}/confirmed-data` additively; no new layout-confirmation API. Existing confirmation/history endpoints preserve legacy reads.

### Drawing and Analysis Routes (Planned)

| Method | Route | Input / result |
| --- | --- | --- |
| GET | `/api/floor-plans/{id}/pages` | Validated metadata `{floorPlanId, pageCount, contentType}`; images count as 1 |
| POST | `/api/floor-plans/{id}/analysis` | Existing `{engine}` plus optional `pageNumber`; pre-acceptance errors use HTTP 400/404/429/503; 202 only after the owned `Analyzing` record is persisted |
| GET | `/api/analyses/{id}/source-image` | Canonical image/png for this analysis only, digest ETag, private caching; 404 if unavailable, never a current-selection replacement |

The existing original-file route remains unchanged. Preparation errors transition the analysis to `AnalysisFailed` with a retryable named message; no unsupported provider receives the entire PDF as fallback. Source-image selection/reading never triggers a new provider call.

### Acceptance Boundary and Polling Failures

Before sending `202 Accepted`, validate the request/references/page and capacity, check provider configuration and persist the owned job. A failure detected before acceptance returns the appropriate HTTP error with `{errorCode, message, retryable}`; quota exhaustion detected at this stage returns 429 with reset-directed `Retry-After`, and configuration/storage unavailability returns 503. An optional preflight quota check is not a reservation and cannot replace atomic quota enforcement before dispatch.

After `202` has been sent, never attempt to change that response or send a later 429/503 for the same POST. Preparation, quota-reservation, provider or storage failures must transition the existing job to `AnalysisFailed` through the ownership-checked persistence path. Successful `GET /api/analyses/{id}` polling returns HTTP 200 with the existing candidate shape plus optional `error: {errorCode, message, retryable, retryAt?}`; `retryAt` is an absolute UTC RFC3339 time when known (next UTC midnight for `ProviderQuotaExceeded`). Retain `errorMessage` for legacy clients. The polling request itself uses 404 for a missing record or 503 if the status store cannot currently be read, not 429 merely because its stored job failed quota checks.

Quota/config/storage rejection after acceptance must never invoke a provider; a preflight-success/quota-race failure is a persisted polling failure, not a second start response. UI stops analysis polling on failed state, shows structured retry/reset information and preserves the analysis ID and saved review data. If a DB outage prevents recording the failure, cancel work without dispatch; polling reports temporary store unavailability until the failure can be conditionally recorded or expired-lease recovery publishes a retryable interrupted job. Do not promise persistence during an unavailable database or bypass lease fencing.

### Job Ownership and Provider Quotas (A4, A6)

Internal job metadata is separate from provider/client candidate fields: `ownerId`, `attemptToken`, `leaseExpiresAt`, `heartbeatAt`. Claim a job atomically for a unique worker/attempt; initial lease is 60 seconds, renewed every 15 seconds using database time. Renewal, source publication, completion and failure writes require matching owner/token, `Analyzing` state and an unexpired lease. A lost/expired lease cancels work and prevents late publication. These are internal controls, not editable confirmation inputs.

Recover only expired leased jobs with a conditional DB update to failed/retryable state. Startup does not fail every `Analyzing` row: another instance may still own it during rollout. Legacy in-progress rows without lease metadata are excluded from automatic reclamation and require explicit operator review or a fresh analysis; never invent ownership. No automatic redispatch/durable queue is introduced. Test deployment overlap, heartbeat loss, expiry and stale completion.

Daily quota scope is a nonsecret configured budget namespace plus provider, shared by all instances using the same allowance. Configure `GEMINI_DAILY_CALL_LIMIT` and `OLLAMA_DAILY_CALL_LIMIT` as nonnegative integers below the applicable provider allowance and `PROVIDER_QUOTA_SCOPE` consistently across replicas; no API keys appear in quota keys. A missing/zero limit disables dispatch for that provider, rather than granting unlimited calls. PostgreSQL is the hosted quota authority; its availability is required for dispatch.

Reserve one daily call atomically immediately before EACH outbound provider attempt, including explicit retries, after preparation succeeds and while the job still owns a live lease. A reservation records a unique logical attempt ID and increments a counter only when below the configured limit; repeated reservation of the same attempt is idempotent but never authorizes sending that attempt twice. Disable uncounted transport-level retries/redirect replay. Reserved/failed calls remain consumed conservatively; no automatic refunds, including crash between reservation and dispatch. Recovery starts no external call automatically.

Buckets use the database's current UTC date and reset at UTC midnight by starting a new dated bucket; old rows remain for audit. Calls dispatched before midnight belong to their reserved day even if completed later. Before acceptance, quota exhaustion returns 429 `ProviderQuotaExceeded`/reset `Retry-After`; config/store failures return 503 `ProviderQuotaNotConfigured`/`QuotaStoreUnavailable`. After acceptance, persist those codes/retry metadata in `AnalysisFailed`, exposing them via successful HTTP 200 polling with `retryAt` for the UTC quota reset. Never rewrite the accepted POST response. Neither phase can fall back to unmetered dispatch; show a clear limit message without clearing saved review data. Concurrency limits remain additional controls; call quotas are not monetary spending guarantees.

## Dimension-Preview State (FR-015)

Reuse existing review/confirmation APIs; no independent dimension-preview confirmation endpoint. The client holds read-only original extraction, editable pending measurements, and last confirmed snapshot/revision as distinct state. Pending room length/width/wall height and opening width/height retain original units; metre equivalents use supported deterministic conversion factors for display only. Required input/association validation remains server-authoritative at confirmation.

Selection/zoom/pan/evidence highlights are client presentation state and never mutate measurements or revisions. Evidence regions are optional; a missing region yields a list/panel status, not a guessed dimension line. Configured defaults remain visibly assumed. Failed drawing rendering retains list-based editing and explicit validated confirmation. Numeric edits create a new revision only after explicit confirmation and cannot change historical estimates; recalculation is a separate action.

## Material Requirements

- `MaterialRecipe`: versioned reviewed product inputs for paint/tile and ratified plaster systems, specification/use, original units, source document/URL or explicit manual assumption, consumption/coats/coverage where applicable, pack size and waste percent. Values must be finite and applicable to the quantity unit. Plaster inputs are either reviewed traditional cement/sand specifications, volume parts, dry-volume factor, cement density, thickness and separate pack sizes, or a premix specification, thickness, kg/m² consumption and kg pack size. No recipe defaults are permitted.
- `MaterialRequirement`: room/trade allocation, measured source quantity, base material amount, waste-adjusted required amount, number of whole packs, purchased amount, surplus, units and exclusions. Quantities are computed by Go, not accepted from AI/client totals.
- Paint: `requiredLitres = paintAreaM2 * coats / coverageM2PerLitrePerCoat * (1 + wastePercent/100)`.
- Tiles: `requiredM2 = flooringOrWallTileAreaM2 * (1 + wastePercent/100)`, with flooring explicitly selected as tile rather than automatically treating all flooring as tile.
- Traditional plaster base volume: `dryM3 = confirmedNetPlasterAreaM2 * (thicknessMm/1000) * dryVolumeFactor`. Base cement is `dryM3 * cementParts/(cementParts+sandParts) * cementDensityKgPerM3`; base sand is `dryM3 * sandParts/(cementParts+sandParts)`. Premix base quantity is `confirmedNetPlasterAreaM2 * reviewedConsumptionKgPerM2`; record thickness without multiplying it a second time.
- Plaster uses only frozen server-side `NetPlasterArea`; traditional and premix systems are exclusive per room. Apply material waste once to each base amount. Reviewed ratios, density, thickness, consumption and pack sizes are required; do not provide construction defaults or mix-design advice.
- Packs: `ceil(requiredAmount / packAmount)`; purchased = packs * pack amount; surplus = purchased - required. Aggregate across compatible allocations before rounding; distinct products cannot share packs automatically. Use deterministic numeric rounding rules tested at exact pack boundaries.
- Formula `plaster-materials-v1` records the ratified plaster-specific consumption formulas and input provenance. It uses the same explicit six-decimal normalization and exact pack-ceiling policy; snapshots retain the reviewed recipe fields, component units, source area, waste and formula version.
- `MaterialBudgetSnapshot`: immutable ID/version, project, estimate ID, source analysis/confirmation revision, frozen quantity result, formula/recipe versions, reviewed inputs, requirements, assumptions and creation timestamp. Source revisions must belong to the same project. Selected offers belong to separate comparison snapshots referencing this budget. Separate material-only result; existing estimate totals do not change.
- New estimates add `sourceConfirmationRevision`, `sourceFloorPlanId` and optional source page/digest captured from the exact snapshot used for calculation. A budget-create input uses `{estimateId, materialWastePercent, allocations}`; each allocation identifies room, trade (`paint|wallTile|floorTile`), reviewed recipe/specification and compatibility group. All source quantities/provenance are loaded by the server, not supplied as client amounts. Material waste is finite 0-50; coats are positive integers and coverage/pack amounts positive finite values. Flooring is tile only when explicitly allocated.
- Budget output retains requirements and calculation-ready immutable inputs, not selected mutable catalog references. Comparison snapshots store selected offers separately. Unknown historical revision produces 409 `SourceRevisionUnavailable`; user must create a traceable new estimate, not attach a new/latest revision to the old one.
- **Allocation uniqueness (A2):** Within one budget, each `(roomId, trade)` is unique (`paint|wallTile|floorTile`). Reject repeats with 400 `DuplicateMaterialAllocation`, naming the room/trade, before aggregation or persistence; reject even if recipes/group labels differ. Client areas/totals cannot replace frozen server quantities. Compatible distinct-room allocations may share packs only after validation of identical reviewed recipe/product inputs, not merely the submitted `compatibilityGroup`. Split-area/multiple-product requests within one room/trade are not supported in this demo.
- Formula `finish-materials-v1` normalizes material-only area/recipe inputs to six decimal places, round-half-away-from-zero, then exact decimal-rational math and exact pack ceiling. This does not change existing QS result precision. Export/display rounding does not feed calculations.
- **Post-normalization validation (A3):** Validate finite values before normalization and all field constraints again afterward. Normalized coverage and pack amounts must be strictly positive; a positive raw value such as `0.0000004` that becomes zero yields 400 `InvalidMaterialRecipe` naming the field, never a substituted minimum or division by zero. Test `0.00000049`, `0.00000050` and `0.00000051`: half-away normalization gives zero, `0.000001`, `0.000001` respectively. The smallest supported normalized positive value is `0.000001` of the declared unit. A genuinely excluded/zero source quantity remains zero; a positive source area that normalizes to zero yields a named `MaterialPrecisionUnsupported` error rather than zero packs. Validate coverage/pack inputs from catalog offers on both import and comparison paths too.
- Historical estimates lack an explicit revision field: do not guess that today's latest confirmation generated an older estimate. Freeze and verify provenance before producing a new budget; unavailable provenance requires fresh confirmation/estimate or explicit rejection.

## Location, Product and Price

- `LocationQuery`: reviewed country/city/postcode, optional radius; optional consented coarse coordinates. A nearby branch is not necessarily a delivery-capable branch.
- `CatalogProduct`: supplier/product IDs, category, normalized specification/use/grade, pack size/unit, coverage where sourced, source URL, and reviewed equivalence/category grouping. Cheap price alone cannot establish equivalent product quality.
- `PriceObservation`: immutable observation ID, supplier/branch, product ID, permitted source method (`api`, `feed`, `scrape`, `manual`, `fixture`), URL, observed/published time if known, currency, pack price, tax inclusion, known tax, delivery coverage/cost/status, availability status, and validity where stated. Preserve original amounts and units as evidence.
- `RefreshStatus`: last attempt/success, bounded failure code, freshness threshold and source policy version. Failed refresh never rewrites an old observation's timestamp.
- `PriceComparison`: grouped equivalent offers, required whole packs per offer, normalized unit price, material subtotal, known charges, incomplete-charge warnings and optional captured exchange metadata. Only comparable complete delivered totals may be ranked by delivered price. Unknown tax/delivery is nullable, not zero. Basket-level delivery is counted once under documented supplier rules; unknown rules stay incomplete.
- `SelectedOfferSnapshot`: copies selected observation/product values, quantity and charge/conversion assumptions into an immutable comparison referencing the budget. Later cache changes do not alter it. External offers are indicative, not reservations or quotations. The budget quantity/recipe snapshot remains unchanged when new comparisons are created.
- Pilot `LocationQuery`: country code `LK`, city `Colombo`, optional postcode/radius, display comparison currency `LKR`. Other location coverage is absent, not inferred. Cross-currency comparison requires `{baseCurrency, quoteCurrency, rate, capturedAt, source: "manual"}` with explicit reviewed positive rate; unavailable conversion keeps offers in separate groups.
- Freshness default is 24 hours for fixtures/imports, evaluated from observation time; fixtures always retain `fixture` origin regardless of freshness. Availability is `inStock|outOfStock|quoteOnly|unknown`; charge coverage is `included|knownExtra|unknown|notApplicable` with null monetary amounts for unknowns. `deliveredTotal` is null unless all necessary branch-basket taxes/delivery are established. Fixture parties/products use fictional identifiers and no fabricated real quotation/source URLs.

### Budget Request Example (Planned, Not an Existing API)

```json
{
	"estimateId": "estimate-traceable",
	"materialWastePercent": 10,
	"allocations": [
		{
			"roomId": "room-living",
			"trade": "paint",
			"compatibilityGroup": "interior-matt-a",
			"recipe": {
				"category": "paint",
				"specification": "Reviewed interior matt",
				"coats": 2,
				"coverageM2PerLitrePerCoat": 10,
				"packAmount": 5,
				"packUnit": "litre",
				"inputOrigin": "manual"
			}
		}
	]
}
```

## Proposed API Surface

`GET /api/material-capabilities` returns `{plasterMaterials: boolean}` to identify deployments where the ratified calculation feature is explicitly enabled. Disabled requests receive 409 `PlasterMaterialsDisabled`; default runtime configuration remains off until operators enable the already-tested feature.

| Method | Route | Purpose |
| --- | --- | --- |
| GET | `/api/material-products` | Approved cached product specs/categories, no arbitrary URLs |
| POST | `/api/projects/{id}/material-budgets` | Validate reviewed inputs/provenance and create a material-only version |
| GET | `/api/projects/{id}/material-budgets` | List saved versions |
| GET | `/api/material-budgets/{id}` | Retrieve an immutable version |
| GET | `/api/material-offers` | Read cached approved offers filtered by reviewed location/product; never trigger an uncontrolled scrape |
| POST | `/api/material-budgets/{id}/comparisons/preview` | Calculate budget-aware offer/basket alternatives without persisting a comparison or reserving stock |
| POST | `/api/material-budgets/{id}/comparisons` | Save a new comparison snapshot, not mutate the budget |
| GET | `/api/material-comparisons/{id}` | Retrieve frozen selected options |
| GET | `/api/material-comparisons/{id}/export` | Separate material-price export with provenance |

Comparison-create input is `{location, selections, conversion?}`; each selection identifies a budget requirement and a stored observation ID. Server validates equivalent specs and recalculates per-offer packs/coverage and branch-level charges. Return 201 with immutable ID/version, totals and warnings; no client unit-price or subtotal is authoritative. Creation routes accept neither arbitrary URLs nor requests to refresh suppliers. Proposed query fields are `country`, `city`, `postcode`, `category`, `equivalenceKey`; results expose freshness/status and original units/currency. Pagination/response limits are bounded during T002 implementation review.

**Non-saving preview (A1):** Preview input is `{location, alternatives, conversion?}`; each alternative is a bounded list of `{requirementId, observationId}` selections for one candidate basket. The route's budget ID loads frozen quantities/recipes and validates ownership. Return 200 with `evaluatedAt`, `budgetId`, formula version and per-alternative selected observation IDs, required amounts/packs/surplus, material subtotal, known tax/delivery, nullable delivered total and warnings. Do not insert budgets/comparisons, allocate versions, refresh catalogs, reserve stock or call providers. Cached offer listing alone never supplies an authoritative budget-specific total.

Preview and Save share one server pricing function. Save explicitly receives the user's selected immutable observation IDs and recalculates against those observations and current eligibility/freshness under a single transaction; client/preview totals are not accepted. Newly observed prices do not silently substitute different observation IDs. If selected offers are no longer eligible, return a named 409 error and require a new preview/selection; never save a different price without user review. Successful Save freezes exactly the recomputed selections/totals. Preview expiration is not a new persisted state: freshness is rechecked at Save. Snapshot history remains immutable.

Refresh is an operator-only authorized job/command, not an unauthenticated customer endpoint. Ingest and refresh authorization are part of the chosen hosted access boundary.

## Validation and Failure Semantics

Use the existing common API error convention with named affected fields/items. Reject foreign project/revision references and invalid units/recipes; distinguish incomplete source data from calculation errors. Provider/collector failure never confirms data, fabricates an offer or blocks the ordinary QS workflow. Schema migrations and export parsers preserve legacy payloads. Tests must prove unknown tax/delivery/stock states cannot be presented as known values.

New error cases: `InvalidSourcePage`, `SourceImageUnavailable`, `PreparationFailed`, `SourceRevisionUnavailable`, `InvalidMaterialRecipe`, `IncomparableOffer`, `OfferUnavailable`, `MissingConversion`, and retryable `CapacityExceeded`. Malformed bodies use 400, missing IDs 404, invalid requirements 400 and provenance/conflict states 409; unavailable current offers remain visible via GET with status/warnings rather than being silently substituted. Monetary unknowns and incomplete totals must remain null through storage and export.

Additional remediation errors: 400 `MaterialPrecisionUnsupported`, 409 `OfferSelectionExpired`, pre-acceptance 429 `ProviderQuotaExceeded`, and pre-acceptance 503 `ProviderQuotaNotConfigured`/`QuotaStoreUnavailable`. After 202, analysis failures instead store these codes in the candidate's structured `error` for HTTP 200 status polling; transport/status-store failures still use appropriate HTTP errors. Immediate 429 quota replies include `Retry-After`; stored quota failures include UTC `retryAt`. Internal stale-owner writes are rejected without exposing worker tokens or turning an expired job into ready state.

A2 adds 400 `DuplicateMaterialAllocation`, with affected room/trade identified and no new budget version written.

## Evaluation Report Contract (A5; Test Artifact Only)

An evaluation report identifies frozen dataset/matching-rule versions, provider/model/prompt version, source page/input representation and `outputStage: raw`. It contains separate `detection`, `dimensions`, and `localization` results; each declares support status `supported|notSupported`, denominators and appropriate TP/FP/FN/accuracy values. Unsupported metrics have null scores, never invented zero detections. Human-corrected confirmation results are separate workflow reports, not raw-AI evaluation input. No new customer API is required for this test artifact.

Detection identity/association matching is frozen before tuning without requiring location boxes. Localization uses one-to-one same-type IoU >=0.5 and counts missing regions as failures only for supported new output. Dimensions include all legible labels with correct entity/unit and <=2% error. Duplicate/unmatched/ambiguous predictions are counted explicitly. The approved feature precision/recall and dimension thresholds are unchanged; legacy baseline comparisons use only jointly supported metrics and disclose unsupported feature/localization capabilities.