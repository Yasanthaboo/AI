# AI Quantity Surveyor Development Rules

- Read `.specify/memory/constitution.md` before changing project behavior.
- Follow `specs/001-ai-quantity-surveyor/spec.md` and `plan.md` for the approved MVP.
- Constitution 1.6.0 authorizes bounded spec 005 recognition, separate paint/tile/plaster material budgets, and local-price comparison. Plaster formulas require confirmed net plaster area and explicitly reviewed inputs; never supply construction defaults or imply structural-engineering advice. Price fixtures currently cover paint/tile only; do not invent plaster offers. Live supplier collection still requires documented region/source approval.
- The frontend uses Next.js, React, and TypeScript. The backend uses Go.
- AI may extract structured floor-plan data, but deterministic Go application code performs all QS calculations.
- Never fabricate missing dimensions or construction data. Mark uncertain values and require human confirmation.
- Keep controllers/handlers thin and keep domain rules independent of HTTP, persistence, and AI providers.
- Add or update focused tests with every behavior change.
- Keep stairs/balconies informational; source geometry never determines QS measurements. Preserve uncertainty, source evidence, original candidates and reviewed revisions.
- Calculate paint/tile/plaster material requirements and pack rounding deterministically, with waste applied once and frozen source provenance. Plaster uses the confirmed net plaster area; traditional mix and premix are mutually exclusive per room and require reviewed ratios/density/thickness/consumption/packs. Keep material-only budgets separate from existing finish-rate totals and never prescribe a construction mix.
- Supplier offers require approved sources, equivalent specifications, units, currency, timestamps and explicit tax/delivery unknowns. Never bypass access controls, scrape arbitrary URLs or fabricate current prices. Use labeled fixtures until live-source gates are met.
- Protect both web and API before confidential customer use; CORS is not authentication. Use consented/minimized location data and environment-only secrets.
- Do not add concrete, steel, masonry, roofing, electrical, plumbing, procurement, multi-agent, LangGraph, MCP, or advanced multi-floor features.
- Structural/concrete mix design, stair costing, external balcony finishes, supplier administration and full BOQ remain deferred. Plaster material consumption budgeting is authorized only under constitution principle 14; no default recipe or engineering recommendation is authorized.
