<!--
Sync Impact Report
Version: 1.5.0 -> 1.6.0 (minor: bounded plaster-material budgeting; existing QS rules preserved)
Amended principles: 1, 12, 14
Synchronized: .github/copilot-instructions.md, README.md, spec 005 spec/plan/tasks/checklist/quickstart
Supplier access, holdout evidence, implementation and release gates remain pending as documented.
Spec Kit templates/command files are absent locally; no template synchronization claimed.
Historical initial ratification date is unknown. Version 1.5.0's bounded P1-P3 scope was owner-approved 2026-10-13; this plaster-material amendment was explicitly approved 2026-10-14.
-->

# AI Quantity Surveyor Constitution

**Version:** 1.6.0
**Status:** Ratified scope for next-demo recognition, paint/tile/plaster material budgets, and local-price comparison; implementation and operational gates remain pending.
**Last Amended:** 2026-10-14
**Amendment Approval:** Owner explicitly approved the bounded plaster-material scope on 2026-10-14; original P1-P3 scope approved 2026-10-13.

**Amendments:** 1.4.0 (spec 004) adds a project workspace, interior finishes quantities (flooring, ceiling, skirting, wall tiling), user-configured defaults listed as assumptions, and indicative opening positions in the model.
1.5.0 (spec 005) permits evidence-backed windows/stairs/balconies review, separate deterministic paint/tile material budgets, and a bounded local supplier-price pilot. 1.6.0 adds separately versioned, reviewed traditional cement/sand plaster and premixed-plaster material budgets calculated only from confirmed net plaster area. Neither amendment approves supplier access or represents implementation completion.

## Core Principle

> AI understands the drawing. The application calculates the quantity. The user makes the final decision.

## Principles

1. **MVP first.** Support only residential house floor plans, a project list and overview, one drawing per project, AI extraction, human verification, a read-only single-floor 3D model from confirmed dimensions and an AI-estimated layout, interior finishes quantities (wall plaster, paint, wall tiling, flooring, ceiling, skirting), configurable rates, costs, estimate display and comparison, and Excel export. The next increment adds source-evidence review of windows and informational stairs/balconies, separate paint/tile/plaster material budgets, and price comparisons for one approved region and two or three approved suppliers.
2. **AI-assisted, not AI-driven calculation.** AI extracts rooms, dimensions, doors, windows, informational stairs/balconies, units, source evidence, and confidence. It must never calculate or determine final quantities or costs, invent prices, or silently select product consumption rules.
3. **Human in the loop.** Extracted data is visible, editable, and explicitly confirmed before calculation. Uncertain data is never treated as verified.
4. **No fabrication.** Missing or unreadable information remains missing or uncertain and requires user review. A user-configured default (such as wall height) may pre-fill a missing value only if it is visibly marked in review and listed as an assumption on every estimate that uses it.
5. **Deterministic QS calculations.** The Go domain layer implements the approved formulas and automated tests:
   - Floor area = length x width
   - Perimeter = 2 x (length + width)
   - Gross wall area = perimeter x wall height
   - Door area = door width x door height
   - Window area = window width x window height
   - Net plaster area = gross wall area - door deductions - window deductions
   - Wall tile area = net plaster area x tiling percentage
   - Paint area = net plaster area - wall tile area
   - Flooring area = ceiling area = floor area
   - Skirting length = max(0, perimeter - sum of door widths)
   - Cost = quantity x (1 + waste percentage) x rate
   - Items excluded for a room have zero quantity and cost
   These existing formulas remain unchanged. Separate material-budget formulas are defined in principle 14; informational stairs/balconies and model coordinates cannot enter QS inputs.
6. **Explicit units.** Every dimension has a supported unit. Conversion is deterministic and uses one internal measurement unit.
7. **Configurable rates.** Finish rates, waste allowance, and exchange rates are input data, not constants in AI prompts or calculation code. Product coverage/coats/pack sizes require sourced specifications or explicit surveyor review. Price comparisons record source currency and any captured conversion metadata; unknown prices or charges remain unknown.
8. **Clear separation.** The flow is AI analysis -> validated data -> confirmed data -> QS calculation -> cost calculation -> presentation/export. A separate material-budget flow uses frozen confirmed quantity results and reviewed product inputs -> deterministic requirements -> sourced offer comparison -> immutable material-only snapshot/export. It never silently adds material costs to existing finish-rate totals.
9. **Provider abstraction.** AI integration is behind `IFloorPlanAnalyzer`-equivalent Go interfaces so providers can be replaced independently of the calculation engine.
10. **Testing is mandatory.** Cover formulas, validation, AI response parsing, API behavior, and the complete workflow. Establish annotated extraction baselines and agreed holdout accuracy targets before tuning. Test material rounding/provenance, equivalent offers, stale/unknown prices, immutable snapshots, and unchanged legacy estimates/exports; use recorded fixtures rather than live scraping in automated tests.
11. **Data integrity and security.** Confirmed data is the calculation source of truth. Validate file type/size, AI output, units, dimensions, source-page references, and API input. Protect both web and API before confidential drawings/location data are used; otherwise use isolated sample data and bounded provider calls. CORS is not authentication. Keep credentials outside source/browser code, minimize location data, require geolocation consent, and prevent arbitrary URL fetching/SSRF in supplier adapters.
12. **Scope control.** Do not introduce concrete/structural mix design, steel, masonry quantities or construction design, roofing, electrical, plumbing, external works, full BOQ, procurement/purchases, supplier administration, application users and roles, RAG assistants, multiple agents, LangGraph, MCP, general scheduling, or multi-floor workflows. Reviewed cement/sand and premixed-plaster consumption budgeting is permitted only within principle 14; it is not structural engineering advice or a recommendation of a construction mix. Stair costing and external balcony finishes remain deferred. The only supplier exception is bounded catalog/price lookup and comparison under principle 15; bounded refresh jobs and managed demo access protection do not authorize new scheduling or user-management products.
13. **Estimated visualization (POC).** A single-floor 3D model may place rooms using AI-extracted boundaries from the drawing, scaled by confirmed dimensions, and must be labeled as an AI-estimated layout. Rooms without boundaries may be arranged approximately and must be labeled so. Doors and windows may be drawn at indicative positions labeled as such. Model geometry is never a source for QS calculations.
   Reviewed stairs/balconies may have source-linked informational annotations or text fallback. Never invent stair rise/run, elevations or structure; window evidence regions are not verified wall anchors. Separate verified-layout confirmation and construction-ready BIM/CAD remain outside scope.
14. **Separate material budgets.** Deterministic Go calculations use frozen confirmed quantities, exact source revision/page/image digest, reviewed product specifications, and versioned formulas. Paint litres = paint area x coats / coverage per litre per coat x (1 + material waste percentage). Tile area required = explicitly selected flooring/wall-tile area x (1 + material waste percentage). For traditional plaster, use confirmed net plaster area $A$ in m² and reviewed thickness $t$ in mm, dry-volume factor $F$, cement:sand volume parts $C:S$, cement density $\rho$ in kg/m³, and reviewed pack sizes: dry volume is $V=A(t/1000)F$; base cement is $V \cdot C/(C+S) \cdot \rho$ kg and base sand is $V \cdot S/(C+S)$ m³. For premixed plaster, use the reviewed product consumption $q$ in kg/m² at the recorded thickness; base premix is $Aq$ kg and thickness is not applied a second time. Apply material waste once to each base amount. Whole packs = ceiling(required amount / pack amount); purchased amount = packs x pack amount; surplus = purchased - required. Aggregate only compatible products before pack rounding, never treat all flooring as tile, and never mix traditional/premix systems in one room allocation. All recipe inputs, ratios, density, coverage, thickness and pack sizes must be explicitly reviewed or sourced; do not provide construction defaults or structural mix recommendations. Missing inputs block only the affected material budget. Keep snapshots/exports immutable and separate from existing QS estimates; do not guess legacy source revisions or alter QS formulas/totals.
15. **Local-price evidence.** Use one approved region and two or three approved sources via authorized feeds/APIs, permitted scraping, or labeled maintained imports. Record product specification/pack units, source URL/method, supplier/branch, currency, observation time, availability when known, and tax/delivery coverage. Compare equivalent products and whole-pack totals; unknown charges are not zero and cannot support a complete delivered-total claim. Capture selected observations immutably; failed refresh retains old timestamps and explicit stale/error states. Require per-source terms/permission/robots review; robots compliance alone is not permission. Use allowlists, bounded rate-limited refresh/caching and no access-control bypasses. Manual location is primary with consented geolocation fallback. No approved live sources means labeled fixture/imported data, not invented live-price claims.

## Architecture Rules

- `src/app/` owns presentation and human verification.
- `api/` exposes HTTP APIs and owns application, domain, and infrastructure boundaries.
- AI providers may produce structured candidates only.
- Only confirmed and validated data may enter the calculation engine.
- Handlers remain thin; business rules live in domain/application packages.
- Supplier adapters and network/parsing logic stay outside deterministic domain calculations; material budgets remain independent of renderer state and existing finish-rate totals.

## Definition of Done

A feature is complete when its behavior, validation, error handling, focused tests, and documentation are implemented without unrelated scope expansion.

## Governance and Next-Demo Gates

The constitution controls scope; a feature document alone cannot override exclusions. Amendments require explicit owner approval, a recorded version/date and synchronized development instructions/spec statuses. Use semantic versioning: major for incompatible principle changes, minor for added bounded scope, patch for wording corrections. Check constitution compliance during planning and before release.

Spec 005 is authorized for P1 recognition, P2 paint/tile/plaster material budgets and P3 local-price comparison. Plaster budgeting is quantity estimation from reviewed inputs, not a construction recommendation. P2 may use explicit surveyor-reviewed recipes; no mix, density, thickness, pack or premix consumption defaults are supplied. Price-comparison fixtures currently cover paint/tile only; plaster pricing requires separately approved equivalent offers and must remain visibly unavailable otherwise. Before P3 live collection, approve region/currency, sources/access policies, freshness/refresh limits and product equivalence. Before recognition claims, agree annotated fixtures/accuracy targets. Before confidential customer release, verify both web/API protection; sample-only demo remains the fallback. Ratification does not finalize schemas, source permissions, credentials, tests or delivery status.
