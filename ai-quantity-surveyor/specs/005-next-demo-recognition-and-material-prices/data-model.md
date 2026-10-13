# Data Model: Feature 005

**Status:** Planned additive model; no migrations or types implemented. Contract names are defined in `contracts.md`.

## Existing Data and Additive Fields

- `floor_plans`: keep original validated bytes unchanged; its existing project ownership remains authoritative.
- `analysis_candidates.payload`: add server-owned `source`, optional `features`, field `evidence` and `reviewIssues`. Candidate is unconfirmed; missing numeric fields remain reviewable. Failed preparation/provider calls use current failed state with named retryable error metadata.
- `analysis_candidates` job-control columns: nullable `owner_id`, `attempt_token`, `lease_expires_at`, `heartbeat_at`; new running jobs claim all four atomically. Controls are internal, not provider/client payload fields. Conditional writes match owner/token/state and unexpired lease; historical missing metadata is not fabricated.
- `analysis_confirmations.payload`: copy canonical source identity, reviewed values/dispositions and server-derived corrections; `(analysis_id, revision)` remains immutable primary key. Original candidate/source evidence is retained independently.
- `estimate_versions.payload`: add `sourceConfirmationRevision`, `sourceFloorPlanId`, and optional `sourcePageNumber`/digest from the exact immutable snapshot used in calculation. No historical revision is guessed/backfilled. Existing quantities/costs and exported values are unchanged.

## New Tables

| Table | Keys/indexes | Content and invariant |
| --- | --- | --- |
| `analysis_source_images` | `analysis_id` primary key/FK to candidate | Original floor-plan ID, page, selection origin, digest, canonical width/height, image/png bytes, prepared timestamp. Written once; candidate completion cannot silently switch source. |
| `provider_daily_usage` | primary key `(quota_scope, provider, utc_date)` | Persisted nonnegative reserved-call count; increment only below configured daily limit using DB UTC time. New day gets a new bucket; old audit rows persist. Shared across replicas. |
| `provider_call_reservations` | opaque reservation ID; unique `(quota_scope, provider, analysis_id, attempt_id)` | Durable logical-attempt reservation, its UTC bucket, owner/token and dispatch-claim status. Repeated reservation is idempotent; a unique reservation never permits two outbound sends. No secret key or refund/automatic replay. |
| `material_products` | product ID primary key; category/equivalence key indexes | Versioned reviewed specifications, pack/coverage inputs and source method. Referenced inputs are copied into budgets/comparisons, so catalog edits cannot mutate history. |
| `material_price_observations` | observation ID primary key; product/branch/observed-time indexes | Immutable original price/currency, branch, location, source/method/time, availability and charge states. Numeric null remains unknown. Fixture origin persists. |
| `material_refresh_status` | source/branch key | Last attempt/success/error and policy version. Separate from old observation times; initial sources are fixture/import only. |
| `material_budget_versions` | ID primary key; unique `(project_id, version)`; estimate ID index | Immutable reviewed allocation/recipe/formula-version and frozen source result/provenance JSONB. FK project/estimate; source analysis/revision must match. |
| `material_comparison_versions` | ID primary key; unique `(budget_id, version)` | Immutable selected offer copies, location/requirements/charges/conversions, warnings and known/null totals. FK budget; no update of parent budget on new selection. |

Use typed indexed columns for identity/reference/version/time and JSONB for reproducible payloads, following existing repository conventions. Define explicit SQL checks for positive integer pages/versions and known status enum values where stored as columns; application validation covers payload fields and ownership. Finite numeric bounds and product/unit compatibility are validated before persistence. No generic unbounded URL-fetch record exists.

## Relationships and Immutable Boundaries

Project -> original drawing -> many analysis candidates -> many confirmations.

An analysis has at most one canonical source image. A new source page implies a new analysis, never an update to an old source. One immutable confirmation snapshot -> one estimate version's quantities -> many material budget versions -> many comparison versions. A comparison references observation IDs for traceability and also copies their selected values, so later catalog retention/refresh does not destroy reproducibility.

Legacy source-image/page metadata is unknown. Legacy estimate revision is unknown and cannot authorize a new budget. New estimates may budget without a rendered image when numeric revision provenance is exact, but missing source evidence remains visible; no evidence is fabricated.

## State and Failure Handling

- Candidate: persisted/owned `Analyzing` -> prepared canonical source -> provider output -> `AnalysisReady` or `AnalysisFailed`. Initial lease 60 seconds, heartbeat every 15 seconds using DB time. Renewal/publication/terminal writes require matching owner/token, active state and unexpired lease. Recovery conditionally fails only expired leases; rollout does not reclaim another instance's live job. Lost lease cancels local work and rejects stale completion. Legacy unleased jobs require explicit review/fresh analysis and are excluded from automatic recovery.
- Provider call: preparation succeeds -> atomic daily quota reservation with live-lease check -> single conditional dispatch claim -> outbound request. Failure/crash never refunds a reserved call or automatically replays an old reservation. Every explicit retry gets a new attempt/reservation. A new DB UTC date gives a new quota bucket at midnight; storage failure cannot permit unmetered calls. Capacity/quota rejections never invoke the provider.
- HTTP acceptance: preflight failures return an immediate HTTP error; 202 follows successful owned-job persistence. Later failures store `AnalysisFailed`, legacy `errorMessage` and additive `error` with code/message/retryable/UTC retryAt when known. HTTP 200 status reads expose stored failures; a status-store outage uses 503 until recoverable. If a terminal write cannot be persisted during DB outage, dispatch remains blocked and ownership/expiry recovery must publish retryable interruption once possible; no successful/fabricated persistence claim.
- Pending review exists only in client editable state until confirmation. A successful immutable confirmation insert precedes the returned revision and preview update. Original candidate is not replaced by pending edits.
- Budget creation validates exact estimate/source ownership, inputs and formulas, allocates a unique version transactionally, and inserts a complete immutable snapshot. Failed creation produces no partial published version.
- Budget allocation lists must have unique `(roomId, trade)` keys before aggregation/version insertion. Reject duplicates regardless of recipe/group; copied areas come only from the frozen server result. Cross-room pack aggregation follows validated recipe/product compatibility. Split-area/multiple-product allocation within one room/trade is unsupported; no partial snapshot is persisted for an invalid request.
- Comparison creation validates stored budget/observation equivalence, calculates pack requirements/charges/conversion, allocates a version and copies values transactionally. No selection mutates the old budget/comparison.
- Comparison preview uses the same pricing function against a read-consistent set of frozen budget/observation inputs, with no insert/version allocation/stock reservation. Explicit Save recalculates against selected immutable observation IDs and current eligibility/freshness; expired selections are rejected, not swapped silently. Preview response is transient and needs no new table.
- Refresh appends new observations and updates separate attempt status; failed refresh cannot alter observation timestamp. Read-time freshness/status flags do not rewrite historical observations or saved results.

## Numeric and Privacy Policy

Material-only normalization uses six decimal places followed by exact rational computation; preserve original reviewed input alongside normalized inputs/formula version. Pack counts are nonnegative integers, missing charges null, currency explicit, totals currency-rounded at defined basket boundary. Current QS float precision remains untouched.

Validate finite/range constraints before normalization and again afterward. Coverage/pack divisors must stay strictly positive; positive source quantities rounded to zero are precision errors, while excluded/genuine zero quantities remain valid zeros. Never persist a recipe whose divisor normalized to zero or substitute a hidden minimum. Record field errors and verify values immediately below/at/above `0.0000005`. Provider quota dates/lease times use database UTC/time authority, not per-process clocks; quota scopes/ownership tokens contain no provider secrets and are not client-editable.

Use coarse reviewed Colombo location fields. Do not store exact browser coordinates in budget/comparison snapshots unless a later approved need/consent policy requires them. Provider credentials are never records. Original drawing/image APIs and every new route require the same future private-access boundary; until verified, only isolated sample data is allowed.