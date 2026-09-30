# AI Quantity Surveyor MVP Specification

## Goal

Provide a small web application that analyzes one residential house floor-plan drawing, lets a quantity surveyor verify the extracted data, and calculates transparent wall plaster and paint quantities and costs.

## User Workflow

Create project -> upload one floor plan -> analyze -> review/correct extraction -> confirm data -> calculate -> review estimate -> export Excel.

## User Stories

- As a quantity surveyor, I can create a residential project and upload one supported floor plan.
- As a quantity surveyor, I can see what the AI extracted, including uncertain and missing values.
- As a quantity surveyor, I can correct, add, or remove rooms, doors, and windows before confirmation.
- As a quantity surveyor, I can confirm validated data and receive transparent room-level and project-level quantities.
- As a quantity surveyor, I can set USD rates, select a target currency, and see the captured exchange rate used for the estimate.
- As a quantity surveyor, I can revise confirmed inputs or rates while preserving earlier estimate versions.
- As a quantity surveyor, I can export a selected estimate with values that match the displayed result.

## Functional Requirements

- Create a residential project with a name and basic project information.
- Accept one drawing per analysis. Supported files are PDF, PNG, JPG, and JPEG; reject files larger than 10 MB before storage.
- If a drawing is unreadable or analysis fails, show an understandable error and allow the user to retry.
- Analyze the drawing through an abstract vision provider.
- Extract rooms, rectangular length/width dimensions, wall height, doors, windows, units, and confidence/uncertainty.
- Show original extraction and distinguish user corrections where practical.
- Let the user edit, add, and remove rooms, doors, and windows; edit dimensions, wall heights, associations, and units.
- Require explicit confirmation after validation and before calculation.
- Calculate room-level and project-level plaster quantity, paint quantity, plaster cost, paint cost, and grand total.
- Allow project-specific plaster and paint rates.
- Calculate the estimate in USD, then convert it to the project's selected target currency using the current exchange rate captured for that estimate.
- Allow plaster and paint rates to be edited after estimate creation.
- Preserve prior estimates as immutable versions when confirmed data or rates change; the latest estimate is used by default.
- Export an estimate to Excel with separate Summary, Rooms, and Openings worksheets.

## Workflow and State Requirements

- A project starts in `Created` state.
- A floor plan moves through `Uploaded`, `Analyzing`, `AnalysisReady`, or `AnalysisFailed` state.
- Extracted data is editable until the user explicitly confirms it.
- Confirmation is permitted only when all required room dimensions, wall heights, units, rates, and opening data pass validation.
- A confirmed floor plan is the only valid input to calculation.
- An estimate is created from one confirmed floor-plan version, rate set, target currency, exchange rate, and calculation timestamp.
- Editing confirmed data or rates does not mutate an existing estimate; it creates a new version after the user confirms and recalculates.
- The latest estimate version is shown by default, and prior versions remain viewable and exportable.

## Data Rules

Rooms are rectangular in the MVP. Each room requires positive length, width, wall height, and known units. Doors and windows may have multiple entries per room; deductions apply only to their associated room. Unassociated or uncertain openings block confirmation until corrected or removed.

Supported units are feet, inches, meters, and centimeters. Convert deterministically to meters internally. Decimal and architectural fractional input is accepted when positive and finite. Mixed units are valid when each dimension declares a supported unit. The UI and export show both the original entered value/unit and the converted meter value.

Each room stores an identifier, name, original dimensions, converted meter dimensions, wall height, and confirmation status. Each door and window stores an identifier, associated room identifier when known, original width/height, converted meter width/height, uncertainty status, and confirmation status. User edits must remain distinguishable from the original AI extraction where practical.

Each rate stores a material type, USD amount per square meter, effective value used by the estimate, and confirmation status. Each estimate stores its version number, source confirmed-data version, rates, USD quantities/costs, target currency, exchange rate, exchange-rate timestamp, converted costs, and calculation timestamp.

## Calculation Rules

For each room:

`floorArea = length * width`

`grossWallArea = 2 * (length + width) * wallHeight`

`doorDeduction = sum(door.width * door.height)`

`windowDeduction = sum(window.width * window.height)`

`netPlasterArea = grossWallArea - doorDeduction - windowDeduction`

`paintArea = netPlasterArea`

Negative net plaster area is invalid and must prevent calculation. Wall thickness and ceiling painting are excluded. Calculations use full precision internally and present quantities/costs to two decimal places. Quantity rates are stored per square meter in USD, then converted to the selected project currency using the exchange rate captured with the estimate.

## AI Behavior

The provider returns structured JSON, never free-form calculation text. It must not invent objects or dimensions. Missing values are null/absent with an uncertainty reason. Invalid provider responses fail analysis clearly. Re-analysis creates a new extraction candidate; it does not silently overwrite confirmed data.

## Currency Behavior

USD is the calculation base currency. The user selects a target currency for each estimate. The system obtains the current USD-to-target exchange rate when calculating, stores the rate and timestamp with the estimate, and converts plaster and paint costs and the grand total deterministically. Quantities do not change during currency conversion. If the rate cannot be obtained or validated as positive, calculation fails clearly and no converted estimate is created.

## Excel Export

The export represents one selected immutable estimate version and contains calculated values only.

- **Summary:** project information, estimate version, calculation timestamp, target currency, exchange rate and timestamp, total plaster quantity/cost, total paint quantity/cost, and grand total.
- **Rooms:** room name, original dimensions and units, converted meter dimensions, wall height, floor area, gross wall area, door deduction, window deduction, net plaster area, and paint area.
- **Openings:** room association, opening type, identifier, original width/height and units, converted meter dimensions, area deduction, and uncertainty/correction status.

All monetary values and displayed quantities use two decimal places. Export failure must return an understandable error and must not alter the estimate.

## Validation and Errors

Block confirmation/calculation for missing or uncertain required data, unsupported units, non-positive dimensions, invalid openings, missing rates, or unconfirmed edits. An uncertain or unassociated opening may be removed by the user; a room with no openings is allowed only after the user explicitly confirms that no openings are present. Show understandable errors for invalid/oversized files, upload failure, unreadable drawings, provider failure, invalid AI output, unavailable exchange rates, calculation failure, and export failure. A failed exchange-rate lookup must not produce a converted estimate.

Invalid input must identify the affected room, door, window, rate, file, or currency where possible. The system must never silently substitute a missing dimension, default unit, rate, exchange rate, or opening association.

## Feature Dependencies

Project creation is required before upload. A valid upload is required before analysis. A completed analysis is required before review. User confirmation and valid rates are required before calculation. A successful calculation is required before estimate display and export. Editing confirmed data or rates requires a new confirmation and calculation before a new estimate version can be created.

## MVP Boundaries

The MVP excludes ceiling painting, irregular/non-rectangular room geometry, wall thickness deductions, concrete, steel, masonry, roofing, electrical, plumbing, full BOQ generation, procurement, supplier management, advanced roles, multiple AI agents, LangGraph, MCP, scheduling, and advanced multi-floor workflows.

## Acceptance Criteria

1. A user can create a project and upload exactly one PDF, PNG, JPG, or JPEG floor plan no larger than 10 MB.
2. AI output is parsed and displayed as editable structured data without being used as final calculations.
3. Missing or uncertain required values prevent confirmation until resolved.
4. Confirmed sample data produces deterministic quantities matching the formulas.
5. Changing rates creates a new estimate version and changes costs without changing calculation code.
6. Editing confirmed source data preserves the prior estimate version and requires a new calculation.
7. An estimate stores the USD result, target currency, exchange rate, converted costs, and calculation timestamp.
8. Excel contains Summary, Rooms, and Openings worksheets with calculated values only; dimensions include original and meter values.
9. Unit, validation, API, provider parsing, calculation, currency conversion, versioning, export, and end-to-end workflow tests cover the above behavior.
