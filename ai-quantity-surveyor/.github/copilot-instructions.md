# AI Quantity Surveyor Development Rules

- Read `.specify/memory/constitution.md` before changing project behavior.
- Follow `specs/001-ai-quantity-surveyor/spec.md` and `plan.md` for the approved MVP.
- The frontend uses Next.js, React, and TypeScript. The backend uses Go.
- AI may extract structured floor-plan data, but deterministic Go application code performs all QS calculations.
- Never fabricate missing dimensions or construction data. Mark uncertain values and require human confirmation.
- Keep controllers/handlers thin and keep domain rules independent of HTTP, persistence, and AI providers.
- Add or update focused tests with every behavior change.
- Do not add concrete, steel, masonry, roofing, electrical, plumbing, procurement, multi-agent, LangGraph, MCP, or advanced multi-floor features.
