import { expect, test } from "@playwright/test";

const email = "design-user@example.test";
const session = {
  access_token: "design-access-token",
  token_type: "bearer",
  expires_in: 3600,
  refresh_token: "design-refresh-token",
  user: {
    id: "design-user",
    aud: "authenticated",
    role: "authenticated",
    email,
    app_metadata: { provider: "email", providers: ["email"] },
    user_metadata: {},
    identities: [],
    created_at: "2026-10-20T00:00:00.000Z",
  },
};

const room = { id: "kitchen", name: "Kitchen", length: { value: 4, unit: "m" }, width: { value: 3.1999999999999997, unit: "m" }, wallHeight: { value: 2.7, unit: "m" } };
const candidate = { id: "analysis-1", floorPlanId: "plan-1", analysisState: "AnalysisReady", rooms: [room], doors: [], windows: [] };
const confirmation = { analysisId: "analysis-1", revision: 3, confirmedAt: "2026-10-20T00:00:00Z", candidate: { ...candidate, confirmationRevision: 3 } };
const priorConfirmation = { analysisId: "analysis-1", revision: 2, confirmedAt: "2026-10-19T00:00:00Z", candidate: { ...candidate, confirmationRevision: 2, rooms: [{ ...room, length: { value: 3.8, unit: "m" } }] } };
const project = { id: "design-project", name: "Design sample", createdAt: "2026-10-20T00:00:00Z", status: "Confirmed" };
const summary = { project, stage: "Confirmed", floorPlan: { id: "plan-1", fileName: "plan.png", fileSize: 10 }, analysisId: "analysis-1", confirmedRevision: 3, roomCount: 1, openingCount: 0, estimateCount: 0, updatedAt: "2026-10-20T00:00:00Z" };
const concepts = [
  { id: "concept-1", projectId: project.id, analysisId: candidate.id, confirmationRevision: 3, roomId: room.id, roomName: room.name, style: "Contemporary", palette: "Warm neutrals", model: "gpt-image-1", selected: false, createdAt: "2026-10-20T00:00:00Z" },
  { id: "concept-2", projectId: project.id, analysisId: candidate.id, confirmationRevision: 3, roomId: room.id, roomName: room.name, style: "Contemporary", palette: "Warm neutrals", model: "gpt-image-1", selected: false, createdAt: "2026-10-20T00:00:00Z" },
];
const png = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=", "base64");
const studioAccount = { plan: { id: "studio", name: "Studio", priceLabel: "Coming soon", description: "Studio fixture", features: ["aiAnalysis", "model3d", "materials", "exports", "designStudio"] }, assignmentSource: "manual", demoSwitchingEnabled: false };

test("confirmed room design concepts can be generated, selected, and removed", async ({ page }) => {
  let savedConcepts: typeof concepts = [];
  let generationPayload: Record<string, unknown> | null = null;
  let firstImageReads = 0;
  await page.route("**/auth/v1/token**", async (route) => route.fulfill({ status: 200, json: session }));
  await page.route("**/auth/v1/logout**", async (route) => route.fulfill({ status: 204, body: "" }));
  await page.route("**/api/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (path.startsWith("/api/room-designs/")) {
      expect(new URL(request.url()).origin).toBe(new URL(process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080").origin);
      expect(request.headers().authorization).toBe(`Bearer ${session.access_token}`);
    }
    if (path === "/api/account/plan") return route.fulfill({ json: studioAccount });
    if (path === "/api/analysis-engines") return route.fulfill({ json: ["demo"] });
    if (path === "/api/projects" && request.method() === "GET") return route.fulfill({ json: [summary] });
    if (path === `/api/projects/${project.id}/summary`) return route.fulfill({ json: summary });
    if (path === `/api/analyses/${candidate.id}`) return route.fulfill({ json: candidate });
    if (path === `/api/analyses/${candidate.id}/confirmed-data`) return route.fulfill({ json: confirmation });
    if (path === `/api/analyses/${candidate.id}/confirmations`) return route.fulfill({ json: [priorConfirmation, confirmation] });
    if (path === `/api/projects/${project.id}/estimates`) return route.fulfill({ json: [] });
    if (path === "/api/room-design-capabilities") return route.fulfill({ json: { enabled: true, dailyImageLimit: 4, styles: ["Contemporary", "Scandinavian"], palettes: ["Warm neutrals", "Earth tones"] } });
    if (path === `/api/projects/${project.id}/rooms/${room.id}/designs` && request.method() === "GET") return route.fulfill({ json: savedConcepts });
    if (path === `/api/projects/${project.id}/rooms/${room.id}/designs` && request.method() === "POST") {
      generationPayload = request.postDataJSON();
      savedConcepts = concepts;
      return route.fulfill({ status: 201, json: concepts });
    }
    if (path.startsWith("/api/room-designs/") && path.endsWith("/image")) {
      if (path.includes("concept-1") && ++firstImageReads === 1) return route.fulfill({ status: 503, json: { message: "Temporary image outage" } });
      return route.fulfill({ contentType: "image/png", body: png });
    }
    if (path === "/api/room-designs/concept-1/selection") {
      savedConcepts = concepts.map((concept) => ({ ...concept, selected: concept.id === "concept-1" }));
      return route.fulfill({ json: { selected: true } });
    }
    if (path === "/api/room-designs/concept-2" && request.method() === "DELETE") {
      savedConcepts = savedConcepts.filter((concept) => concept.id !== "concept-2");
      return route.fulfill({ status: 204, body: "" });
    }
    if (path.endsWith("/file")) return route.fulfill({ status: 404, json: { message: "Drawing omitted" } });
    return route.fulfill({ status: 404, json: { errorCode: "NotFound", message: "Fixture route not provided" } });
  });

  await page.goto("/");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password", { exact: true }).fill("secure-password-1");
  await page.getByRole("button", { name: "Sign in", exact: true }).last().click();
  await page.getByRole("button", { name: project.name, exact: true }).click();
  await page.locator(".stepper").getByRole("button", { name: /3D model/ }).click();
  const canvas = page.locator(".scene-host canvas");
  await expect(canvas).toBeVisible();
  await page.getByRole("button", { name: "Switch to dark theme" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect.poll(async () => canvas.evaluate((element) => {
    const sample = document.createElement("canvas");
    sample.width = 32; sample.height = 32;
    const context = sample.getContext("2d")!;
    context.drawImage(element as HTMLCanvasElement, 0, 0, 32, 32);
    const pixels = context.getImageData(0, 0, 32, 32).data;
    return pixels[0] < 50 && pixels[1] < 60 && Array.from({ length: 1024 }, (_, index) => index * 4).some((offset) => pixels[offset + 3] > 0 && (pixels[offset] !== pixels[0] || pixels[offset + 1] !== pixels[1] || pixels[offset + 2] !== pixels[2]));
  })).toBe(true);
  await page.screenshot({ path: "test-results/model-dark-desktop.png", fullPage: true, animations: "disabled" });
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.screenshot({ path: "test-results/model-dark-mobile.png", fullPage: true, animations: "disabled" });
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.getByRole("button", { name: "Switch to light theme" }).click();
  await page.getByRole("button", { name: "Design Studio" }).click();
  await expect(page.getByRole("heading", { name: "Room interiors" })).toBeVisible();
  await expect(page.getByText("4 m", { exact: true })).toBeVisible();
  await expect(page.getByText("3.2 m", { exact: true })).toBeVisible();
  await page.getByLabel("Design confirmation revision").selectOption("2");
  await expect(page.getByText("3.8 m", { exact: true })).toBeVisible();
  await page.getByLabel("Design confirmation revision").selectOption("3");

  await page.getByLabel("Preferences Optional · 300 characters max").fill("Reading nook and hidden storage");
  await page.getByRole("button", { name: "Generate two concepts" }).click();
  await expect(page.getByRole("heading", { name: "Option 1" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Option 2" })).toBeVisible();
  await expect(page.getByRole("group", { name: "Concept 1 image error", exact: true })).toContainText("HTTP 503");
  await page.getByRole("button", { name: "Retry image 1", exact: true }).click();
  await expect(page.locator(".design-concept img")).toHaveCount(2);
  for (const image of await page.locator(".design-concept img").all()) {
    await expect.poll(() => image.evaluate((element) => (element as HTMLImageElement).naturalWidth)).toBeGreaterThan(0);
  }
  const downloadEvent = page.waitForEvent("download");
  await page.getByRole("link", { name: "Download concept 1", exact: true }).click();
  expect((await downloadEvent).suggestedFilename()).toBe("kitchen-revision-3-concept-1.png");
  expect(generationPayload).toMatchObject({ roomId: "kitchen", confirmationRevision: 3, style: "Contemporary", palette: "Warm neutrals", preferences: "Reading nook and hidden storage" });
  await page.getByRole("button", { name: "Select design" }).first().click();
  await expect(page.locator(".design-concept--selected .badge")).toContainText("Selected");
  await page.getByRole("button", { name: "Remove" }).last().click();
  await expect(page.getByRole("heading", { name: "Option 2" })).toHaveCount(0);
});

test("NVIDIA Nemotron appears when the API advertises it", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "light" });
  await page.route("**/auth/v1/token**", async (route) => route.fulfill({ status: 200, json: session }));
  await page.route("**/auth/v1/logout**", async (route) => route.fulfill({ status: 204, body: "" }));
  await page.route("**/api/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (path === "/api/account/plan") return route.fulfill({ json: studioAccount });
    if (path === "/api/analysis-engines") return route.fulfill({ json: ["demo", "nvidia"] });
    if (path === "/api/projects" && request.method() === "GET") return route.fulfill({ json: [] });
    if (path === "/api/projects" && request.method() === "POST") return route.fulfill({ status: 201, json: { ...project, id: "nvidia-project" } });
    return route.fulfill({ status: 404, json: { errorCode: "NotFound", message: "Fixture route not provided" } });
  });

  await page.goto("/");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password", { exact: true }).fill("secure-password-1");
  await page.getByRole("button", { name: "Sign in", exact: true }).last().click();
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await page.getByRole("button", { name: "Analyzer", exact: true }).click();
  await page.getByRole("combobox", { name: "Analysis engine", exact: true }).selectOption("nvidia");
  await page.getByRole("button", { name: "Material costing", exact: true }).click();
  await page.getByLabel("Paint coats", { exact: false }).fill("3");
  await page.getByRole("button", { name: "Design studio", exact: true }).click();
  await page.getByLabel("Interior style").selectOption("Scandinavian");
  await page.getByLabel("Color palette").selectOption("Earth tones");
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(page.getByText("Settings saved.")).toBeVisible();
  await page.getByRole("button", { name: "Switch to dark theme" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.screenshot({ path: "test-results/settings-dark-desktop.png", fullPage: true, animations: "disabled" });
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  expect(await page.locator(".app-header").evaluate((header) => {
    const boxes = Array.from(header.children).map((element) => element.getBoundingClientRect()).filter((box) => box.width > 0 && box.height > 0);
    return boxes.every((box, index) => boxes.slice(index + 1).every((other) => box.right <= other.left || other.right <= box.left || box.bottom <= other.top || other.bottom <= box.top));
  })).toBe(true);
  await page.screenshot({ path: "test-results/settings-dark-mobile.png", fullPage: true, animations: "disabled" });
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await page.getByRole("button", { name: "Material costing", exact: true }).click();
  await expect(page.getByLabel("Paint coats", { exact: false })).toHaveValue("3");
  await page.getByRole("button", { name: "Design studio", exact: true }).click();
  await expect(page.getByLabel("Interior style")).toHaveValue("Scandinavian");
  await expect(page.getByLabel("Color palette")).toHaveValue("Earth tones");
  await page.getByRole("button", { name: "Switch to light theme" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  await page.screenshot({ path: "test-results/settings-light-mobile.png", fullPage: true, animations: "disabled" });
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.getByRole("button", { name: "Projects", exact: true }).click();
  await page.getByRole("button", { name: "New project" }).click();
  await page.getByLabel("Project name").fill("NVIDIA engine test");
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page.getByRole("heading", { name: "Upload the floor plan" })).toBeVisible();
  const nvidiaOption = page.getByLabel(/NVIDIA Nemotron 3\.5 Super VL/);
  await expect(nvidiaOption).toBeEnabled();
  await expect(nvidiaOption).toBeChecked();
  await page.getByLabel(/Demo/).check();
  await expect(page.getByLabel(/Demo/)).toBeChecked();
  await nvidiaOption.check();
  await expect(nvidiaOption).toBeChecked();
});

test("material defaults and comparison facts remain readable in both themes", async ({ page }) => {
  const recipe = { category: "paint", specification: "Interior matt", coats: 2, coverageM2PerLitrePerCoat: 10, packAmount: 5, packUnit: "litre", inputOrigin: "manual" };
  const requirement = { id: "paint-requirement", roomIds: [room.id], trade: "paint", recipe, sourceAreaM2: 40, baseAmount: 8, requiredAmount: 8.8, packCount: 2, purchasedAmount: 10, surplus: 1.2, unit: "litre", formulaVersion: "paint-v1" };
  const estimate = { id: "estimate-1", projectId: project.id, version: 1, sourceAnalysisId: candidate.id, sourceConfirmationRevision: 3, sourceFloorPlanId: "plan-1", sourcePageNumber: 1, sourceImageDigest: "fixture-digest", targetCurrency: "USD", exchangeRate: 1, exchangeRateFetchedAt: "2026-10-20T00:00:00Z", convertedPlasterCost: 400, convertedPaintCost: 200, convertedGrandTotal: 600, calculatedAt: "2026-10-20T00:00:00Z", result: { rooms: [{ roomId: room.id, roomName: room.name, floorArea: 12.8, grossWallArea: 40, doorDeduction: 0, windowDeduction: 0, netPlasterArea: 40, paintArea: 40, plasterCostUsd: 400, paintCostUsd: 200 }], totalPlasterQuantity: 40, totalPaintQuantity: 40, totalPlasterCostUsd: 400, totalPaintCostUsd: 200, grandTotalUsd: 600 } };
  const budget = { id: "budget-1", projectId: project.id, version: 1, estimateId: estimate.id, sourceAnalysisId: candidate.id, sourceConfirmationRevision: 3, sourceFloorPlanId: "plan-1", sourcePageNumber: 1, sourceImageDigest: "fixture-digest", materialWastePercent: 10, allocations: [{ roomId: room.id, trade: "paint", recipe }], requirements: [requirement], createdAt: "2026-10-20T00:00:00Z" };
  const offer = { id: "offer-1", supplier: "Sample Colombo supplier", branch: "Colombo", country: "LK", city: "Colombo", currency: "LKR", price: 4500, recipe, origin: "fixture", observedAt: "2026-10-20T00:00:00Z", availability: "Sample stock", taxIncluded: null, delivery: null };
  const basket = { items: [{ requirementId: requirement.id, offer, requirement, subtotal: 9000 }], materialSubtotal: 9000, knownDelivery: 0, deliveredTotal: null, warnings: ["Delivery and tax charges are unknown."] };
  await page.emulateMedia({ colorScheme: "light" });
  await page.route("**/auth/v1/token**", async (route) => route.fulfill({ status: 200, json: session }));
  await page.route("**/api/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/api/account/plan") return route.fulfill({ json: studioAccount });
    if (path === "/api/analysis-engines") return route.fulfill({ json: ["demo"] });
    if (path === "/api/projects") return route.fulfill({ json: [summary] });
    if (path === `/api/projects/${project.id}/summary`) return route.fulfill({ json: summary });
    if (path === `/api/analyses/${candidate.id}`) return route.fulfill({ json: candidate });
    if (path === `/api/analyses/${candidate.id}/confirmed-data`) return route.fulfill({ json: confirmation });
    if (path === `/api/projects/${project.id}/estimates`) return route.fulfill({ json: [estimate] });
    if (path === `/api/projects/${project.id}/material-budgets`) return route.fulfill({ json: [budget] });
    if (path === "/api/material-offers") return route.fulfill({ json: [offer] });
    if (path === "/api/material-capabilities") return route.fulfill({ json: { plasterMaterials: false } });
    if (path === `/api/material-budgets/${budget.id}/comparisons/preview`) return route.fulfill({ json: { budgetId: budget.id, evaluatedAt: "2026-10-20T00:00:00Z", currency: "LKR", alternatives: [basket] } });
    return route.fulfill({ status: 404, json: { message: "Fixture route not provided" } });
  });
  await page.goto("/");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password", { exact: true }).fill("secure-password-1");
  await page.getByRole("button", { name: "Sign in", exact: true }).last().click();
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await page.getByRole("button", { name: "Material costing", exact: true }).click();
  await page.getByLabel("Paint coats", { exact: false }).fill("3");
  await page.getByRole("button", { name: "Save settings" }).click();
  await page.getByRole("button", { name: "Close", exact: true }).click();
  await page.getByRole("button", { name: project.name, exact: true }).click();
  await page.locator(".stepper").getByRole("button", { name: /Estimate/ }).click();
  const panel = page.locator(".material-workspace");
  await expect(panel.getByLabel("Material paint coats")).toHaveValue("3");
  await panel.getByLabel("Supplier comparison location").selectOption("LK/Colombo");
  await panel.getByRole("button", { name: "Preview price alternatives" }).click();
  const comparison = panel.locator(".material-alternative");
  await expect(comparison.locator(".material-offer__label")).toHaveCount(4);
  await expect(comparison).toContainText("unknown charges");
  await comparison.screenshot({ path: "test-results/material-comparison-light-desktop.png", animations: "disabled" });
  await page.getByRole("button", { name: "Switch to dark theme" }).click();
  await comparison.screenshot({ path: "test-results/material-comparison-dark-desktop.png", animations: "disabled" });
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await comparison.screenshot({ path: "test-results/material-comparison-dark-mobile.png", animations: "disabled" });
});