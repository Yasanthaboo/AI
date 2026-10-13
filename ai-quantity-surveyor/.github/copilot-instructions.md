# AI Quantity Surveyor Development Rules

- Read `.specify/memory/constitution.md` before changing project behavior.
- Follow `specs/001-ai-quantity-surveyor/spec.md` and `plan.md` for the approved MVP.
- Constitution 1.5.0 authorizes the bounded spec 005 recognition, paint/tile material-budget, and local-price comparison scope. Contracts remain draft until clarification; live supplier collection requires documented region/source approval.
- The frontend uses Next.js, React, and TypeScript. The backend uses Go.
- AI may extract structured floor-plan data, but deterministic Go application code performs all QS calculations.
- Never fabricate missing dimensions or construction data. Mark uncertain values and require human confirmation.
- Keep controllers/handlers thin and keep domain rules independent of HTTP, persistence, and AI providers.
- Add or update focused tests with every behavior change.
- Keep stairs/balconies informational; source geometry never determines QS measurements. Preserve uncertainty, source evidence, original candidates and reviewed revisions.
- Calculate paint/tile material requirements and pack rounding deterministically, with waste applied once and frozen source provenance. Keep material-only budgets separate from existing finish-rate totals.
- Supplier offers require approved sources, equivalent specifications, units, currency, timestamps and explicit tax/delivery unknowns. Never bypass access controls, scrape arbitrary URLs or fabricate current prices. Use labeled fixtures until live-source gates are met.
- Protect both web and API before confidential customer use; CORS is not authentication. Use consented/minimized location data and environment-only secrets.
- Do not add concrete, steel, masonry, roofing, electrical, plumbing, procurement, multi-agent, LangGraph, MCP, or advanced multi-floor features.
- RAG assistants, cement/sand/plaster recipes, stair costing, external balcony finishes, supplier administration and full BOQ also remain deferred.
