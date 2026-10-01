# AI Quantity Surveyor Constitution

**Version:** 1.4.0  
**Status:** Approved MVP baseline with AI-estimated single-floor layout, project workspace, and interior finishes (proof-of-concept)

**Amendments:** 1.4.0 (spec 004) adds a project workspace, interior finishes quantities (flooring, ceiling, skirting, wall tiling), user-configured defaults listed as assumptions, and indicative opening positions in the model.

## Core Principle

> AI understands the drawing. The application calculates the quantity. The user makes the final decision.

## Principles

1. **MVP first.** Support only residential house floor plans, a project list and overview, one drawing per project, AI extraction, human verification, a read-only single-floor 3D model from confirmed dimensions and an AI-estimated layout, interior finishes quantities (wall plaster, paint, wall tiling, flooring, ceiling, skirting), configurable rates, costs, estimate display and comparison, and Excel export.
2. **AI-assisted, not AI-driven calculation.** AI extracts rooms, dimensions, doors, windows, units, and confidence. It must never calculate or determine final quantities or costs.
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
6. **Explicit units.** Every dimension has a supported unit. Conversion is deterministic and uses one internal measurement unit.
7. **Configurable rates.** Finish rates, waste allowance, and exchange rates are input data, not constants in AI prompts or calculation code.
8. **Clear separation.** The flow is AI analysis -> validated data -> confirmed data -> QS calculation -> cost calculation -> presentation/export.
9. **Provider abstraction.** AI integration is behind `IFloorPlanAnalyzer`-equivalent Go interfaces so providers can be replaced independently of the calculation engine.
10. **Testing is mandatory.** Cover formulas, validation, AI response parsing, API behavior, and the complete workflow.
11. **Data integrity and security.** Confirmed data is the calculation source of truth. Validate file type/size, AI output, units, dimensions, and API input.
12. **Scope control.** Do not introduce concrete, steel, masonry, roofing, electrical, plumbing, external works, full BOQ, suppliers, procurement, users and roles, multiple agents, LangGraph, MCP, scheduling, or multi-floor workflows.
13. **Estimated visualization (POC).** A single-floor 3D model may place rooms using AI-extracted boundaries from the drawing, scaled by confirmed dimensions, and must be labeled as an AI-estimated layout. Rooms without boundaries may be arranged approximately and must be labeled so. Doors and windows may be drawn at indicative positions labeled as such. Model geometry is never a source for QS calculations.

## Architecture Rules

- `web/` owns presentation and human verification.
- `api/` exposes HTTP APIs and owns application, domain, and infrastructure boundaries.
- AI providers may produce structured candidates only.
- Only confirmed and validated data may enter the calculation engine.
- Handlers remain thin; business rules live in domain/application packages.

## Definition of Done

A feature is complete when its behavior, validation, error handling, focused tests, and documentation are implemented without unrelated scope expansion.
