# Proposed Contracts: Recognition, Materials and Local Offers

**Status:** Design proposal; names below are not implemented endpoints or types. Final schemas are settled in task T002 before integration. Existing APIs remain backward compatible.

## Analysis Extensions

- `Candidate.features?`: array of informational `ArchitecturalFeature` records: stable `id`, `type` (`stair` or `balcony`), label, optional room reference, nullable evidence, confidence, uncertainty, and review disposition.
- `Room.evidence?`, `Opening.evidence?`: per-field source evidence (`length`, `width`, `wallHeight`, `width/height` for openings, identity/association as appropriate). Each entry identifies server-bound `analysisId`, `floorPlanId`, page number, nullable normalized region, optional observed text, confidence and uncertainty. Do not treat observed text as a command or trusted numeric input.
- Region coordinates are finite fractions in [0,1], with positive extent wholly inside the source page. References must match the processed drawing/page. Missing region/measurement remains null; confidence is not a guarantee.
- Original engine candidate is retained. Confirmations store reviewed values, changed fields/dispositions, evidence references, and immutable revision metadata. Clients cannot rewrite provider identity or evidence provenance. Manual additions are visibly manual, not engine evidence.
- Windows still use existing validated measured width/height and room association for deductions. Stairs/balconies never enter the domain confirmed-room/opening calculation conversion. A window evidence region does not establish a 3D wall anchor.

Extend current `GET /api/analyses/{id}` and `PUT/GET /api/analyses/{id}/confirmed-data` additively; no new layout-confirmation API. Existing confirmation/history endpoints preserve legacy reads.

## Material Requirements

- `MaterialRecipe`: versioned reviewed product inputs, category (`paint` or `tile`), specification/use, original units, source document/URL or manual assumption, coats/coverage where applicable, pack volume/coverage and waste percent. Values must be finite and applicable to the quantity unit.
- `MaterialRequirement`: room/trade allocation, measured source quantity, base material amount, waste-adjusted required amount, number of whole packs, purchased amount, surplus, units and exclusions. Quantities are computed by Go, not accepted from AI/client totals.
- Paint: `requiredLitres = paintAreaM2 * coats / coverageM2PerLitrePerCoat * (1 + wastePercent/100)`.
- Tiles: `requiredM2 = flooringOrWallTileAreaM2 * (1 + wastePercent/100)`, with flooring explicitly selected as tile rather than automatically treating all flooring as tile.
- Packs: `ceil(requiredAmount / packAmount)`; purchased = packs * pack amount; surplus = purchased - required. Aggregate across compatible allocations before rounding; distinct paint finishes/products cannot share packs automatically. Use deterministic numeric rounding rules tested at exact pack boundaries.
- `MaterialBudgetSnapshot`: immutable ID/version, project, estimate ID, source analysis/confirmation revision, frozen quantity result, formula/recipe versions, reviewed inputs, requirements, selected offer snapshots, assumptions and creation timestamp. Source revisions must belong to the same project. Separate material-only result; existing estimate totals do not change.
- Historical estimates lack an explicit revision field: do not guess that today's latest confirmation generated an older estimate. Freeze and verify provenance before producing a new budget; unavailable provenance requires fresh confirmation/estimate or explicit rejection.

## Location, Product and Price

- `LocationQuery`: reviewed country/city/postcode, optional radius; optional consented coarse coordinates. A nearby branch is not necessarily a delivery-capable branch.
- `CatalogProduct`: supplier/product IDs, category, normalized specification/use/grade, pack size/unit, coverage where sourced, source URL, and reviewed equivalence/category grouping. Cheap price alone cannot establish equivalent product quality.
- `PriceObservation`: immutable observation ID, supplier/branch, product ID, permitted source method (`api`, `feed`, `scrape`, `manual`, `fixture`), URL, observed/published time if known, currency, pack price, tax inclusion, known tax, delivery coverage/cost/status, availability status, and validity where stated. Preserve original amounts and units as evidence.
- `RefreshStatus`: last attempt/success, bounded failure code, freshness threshold and source policy version. Failed refresh never rewrites an old observation's timestamp.
- `PriceComparison`: grouped equivalent offers, required whole packs per offer, normalized unit price, material subtotal, known charges, incomplete-charge warnings and optional captured exchange metadata. Only comparable complete delivered totals may be ranked by delivered price. Unknown tax/delivery is nullable, not zero. Basket-level delivery is counted once under documented supplier rules; unknown rules stay incomplete.
- `SelectedOfferSnapshot`: copies selected observation/product values, quantity and charge/conversion assumptions into an immutable material budget. Later cache changes do not alter it. External offers are indicative, not reservations or quotations.

## Proposed API Surface

| Method | Route | Purpose |
| --- | --- | --- |
| GET | `/api/material-products` | Approved cached product specs/categories, no arbitrary URLs |
| POST | `/api/projects/{id}/material-budgets` | Validate reviewed inputs/provenance and create a material-only version |
| GET | `/api/projects/{id}/material-budgets` | List saved versions |
| GET | `/api/material-budgets/{id}` | Retrieve an immutable version |
| GET | `/api/material-offers` | Read cached approved offers filtered by reviewed location/product; never trigger an uncontrolled scrape |
| POST | `/api/material-budgets/{id}/comparisons` | Save a new comparison snapshot, not mutate the budget |
| GET | `/api/material-comparisons/{id}` | Retrieve frozen selected options |
| GET | `/api/material-comparisons/{id}/export` | Separate material-price export with provenance |

Refresh is an operator-only authorized job/command, not an unauthenticated customer endpoint. Ingest and refresh authorization are part of the chosen hosted access boundary.

## Validation and Failure Semantics

Use the existing common API error convention with named affected fields/items. Reject foreign project/revision references and invalid units/recipes; distinguish incomplete source data from calculation errors. Provider/collector failure never confirms data, fabricates an offer or blocks the ordinary QS workflow. Schema migrations and export parsers preserve legacy payloads. Tests must prove unknown tax/delivery/stock states cannot be presented as known values.