import { expect, test, type Download, type Page } from "@playwright/test";
import type { Candidate, Room } from "../src/app/api";
import { buildModel, buildWalls } from "../src/app/preview";

const planPng = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
  "base64",
);
const meters = (value: number) => ({ value, unit: "m" });

async function bytes(download: Download) {
  const chunks: Buffer[] = [];
  for await (const chunk of await download.createReadStream()) chunks.push(Buffer.from(chunk));
  return Buffer.concat(chunks);
}

async function createProject(page: Page, name = "Smith residence") {
  await page.goto("/");
  await page.getByRole("button", { name: "New project" }).click();
  await page.getByLabel("Project name").fill(name);
  await page.getByLabel("Client").fill("Smith family");
  await page.getByLabel("Site location").fill("12 Harbour Rd");
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page.getByRole("heading", { name: "Upload the floor plan" })).toBeVisible();
}

async function analyzeDemo(page: Page) {
  await page.getByLabel(/Demo/).check();
  await page.locator('input[type="file"]').setInputFiles({ name: "plan.png", mimeType: "image/png", buffer: planPng });
  await page.getByRole("button", { name: "Analyze drawing" }).click();
  await expect(page.getByRole("heading", { name: "Review the extraction" })).toBeVisible();
}

async function confirmDemo(page: Page) {
  await analyzeDemo(page);
  await page.getByRole("button", { name: "Looks right" }).click();
  await page.getByRole("button", { name: "Confirm and build model" }).click();
  await expect(page.getByRole("heading", { name: "3D model" })).toBeVisible();
}

const unique = (name: string) => `${name} ${Date.now().toString(36)}`;

test("indicative openings are spaced on room walls without exceeding them", () => {
  const model = buildModel([{ id: "a", name: "Hall", length: meters(4), width: meters(3), wallHeight: meters(2.4), bounds: { x: 0, y: 0, width: 0.4, height: 0.3 } }], 1, {
    doors: [{ id: "d1", roomId: "a", width: meters(0.9), height: meters(2.1) }, { id: "d2", roomId: "other", width: meters(0.9), height: meters(2.1) }],
    windows: [{ id: "w1", roomId: "a", width: meters(1.2), height: meters(1.2) }, { id: "w2", roomId: "a", width: meters(9), height: meters(3) }],
  });
  expect(model.rooms[0].openings.map((opening) => opening.id)).toEqual(["d1", "w1", "w2"]);
  const walls = buildWalls(model.rooms);
  expect(walls).toHaveLength(4);
  const placed = walls.flatMap((wall) => wall.openings.map((opening) => ({ wall, opening })));
  expect(placed).toHaveLength(3);
  for (const { wall, opening } of placed) {
    const length = Math.hypot(wall.x2 - wall.x1, wall.z2 - wall.z1);
    expect(opening.center - opening.width / 2).toBeGreaterThanOrEqual(0);
    expect(opening.center + opening.width / 2).toBeLessThanOrEqual(length);
    expect(opening.sill + opening.height).toBeLessThanOrEqual(wall.height);
  }
  expect(placed.find(({ opening }) => opening.id === "w1")!.opening.sill).toBeCloseTo(0.9);
});

test("dashboard lists, searches, duplicates, archives and opens projects", async ({ page }) => {
  const name = unique("Dashboard house");
  await createProject(page, name);
  await confirmDemo(page);
  await page.getByRole("button", { name: "Continue to estimate" }).click();
  await page.getByRole("button", { name: "Calculate estimate" }).click();
  await expect(page.locator(".metric-card--total")).toContainText("$");

  await page.getByRole("button", { name: "Projects", exact: true }).click();
  await page.getByLabel("Search projects").fill(name);
  const row = page.locator(".project-table tbody tr").filter({ hasText: name }).filter({ hasNotText: "(copy)" });
  await expect(row).toHaveCount(1);
  await expect(row).toContainText("Estimated");
  await expect(row).toContainText("$");

  await page.getByRole("button", { name: `Duplicate ${name}` }).click();
  await expect(page.locator(".project-table tbody tr").filter({ hasText: `${name} (copy)` })).toContainText("Uploaded");
  await page.getByRole("button", { name: `Archive ${name} (copy)` }).click();
  await expect(page.locator(".project-table tbody tr").filter({ hasText: `${name} (copy)` })).toHaveCount(0);
  await page.getByLabel(/Show archived/).check();
  await expect(page.locator(".project-table tbody tr").filter({ hasText: `${name} (copy)` })).toContainText("Archived");
  await page.getByLabel("Search projects").fill("zz-no-such-project-zz");
  await expect(page.getByText("No matching projects")).toBeVisible();

  await page.getByLabel("Search projects").fill(name);
  await row.getByRole("button", { name: "Open" }).click();
  await expect(page.getByRole("heading", { name })).toBeVisible();
  await expect(page.getByText("Estimate v1 calculated")).toBeVisible();
  await expect(page.getByText("Dimensions confirmed (revision 1)")).toBeVisible();
  await expect(page.locator(".info-panel summary").first()).toContainText("Demo (demo)");
  await expect(page.getByAltText("Uploaded floor plan")).toBeVisible();
});

test("selecting a room highlights it in the table, drawing and model", async ({ page }) => {
  await createProject(page, unique("Selection house"));
  await confirmDemo(page);
  await page.getByRole("button", { name: "Kitchen" }).first().click();
  await expect(page.locator("tr.row--selected")).toContainText("Kitchen");
  await expect(page.locator(".drawing-label--selected")).toHaveText("Kitchen");
  await expect(page.getByText("Selected: Kitchen")).toBeVisible();
  await page.locator(".drawing-label").filter({ hasText: "Bedroom" }).getByRole("button").click();
  await expect(page.locator("tr.row--selected")).toContainText("Bedroom");
  await expect(page.locator(".drawing-room--selected")).toHaveCount(1);
  await page.locator("tr.row--selected").click();
  await expect(page.getByText("No room selected")).toBeVisible();
});

test("model offers top view, hover details and a screenshot", async ({ page }) => {
  await createProject(page, unique("Model tools house"));
  await confirmDemo(page);
  await expect(page.getByText("indicative positions", { exact: false })).toBeVisible();
  const topView = page.getByRole("button", { name: "Top view" });
  await topView.click();
  await expect(page.getByRole("button", { name: "3D view" })).toHaveAttribute("aria-pressed", "true");
  const canvas = page.locator(".scene-host canvas");
  const box = (await canvas.boundingBox())!;
  let hovered = false;
  for (const [fx, fy] of [[0.4, 0.4], [0.6, 0.4], [0.4, 0.6], [0.6, 0.6], [0.5, 0.5], [0.3, 0.5], [0.7, 0.5]]) {
    await page.mouse.move(box.x + box.width * fx, box.y + box.height * fy);
    if (await page.locator(".scene-tooltip").isVisible()) { hovered = true; break; }
  }
  expect(hovered).toBe(true);
  await expect(page.locator(".scene-tooltip")).toContainText("m²");
  const shot = page.waitForEvent("download");
  await page.getByRole("button", { name: "Screenshot" }).click();
  const image = await shot;
  expect(image.suggestedFilename()).toBe("3d-model.png");
  expect((await bytes(image)).subarray(1, 4).toString()).toBe("PNG");
});

test("finishes, assumptions, charts and version comparison", async ({ page }) => {
  await page.route("**/api/analyses/*", async (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    const response = await route.fetch();
    const candidate = await response.json() as Candidate;
    if (candidate.rooms?.[0]) candidate.rooms[0].wallHeight = null;
    await route.fulfill({ response, json: candidate });
  });
  await createProject(page, unique("Finishes house"));
  await analyzeDemo(page);
  await expect(page.getByLabel("Living room wall height")).toHaveValue("1.83");
  await page.getByRole("button", { name: "Looks right" }).click();
  await page.getByRole("button", { name: "Confirm and build model" }).click();
  await expect(page.locator("tr").filter({ hasText: "Living room" }).getByText("Default")).toBeVisible();
  await page.getByRole("button", { name: "Continue to estimate" }).click();

  await page.getByLabel("Flooring rate").fill("20");
  await page.getByRole("button", { name: "Calculate estimate" }).click();
  await expect(page.locator(".assumptions")).toContainText("Living room: wall height 1.83 m is the configured default");
  await expect(page.locator(".assumptions")).toContainText("Not priced (rate 0): wall tiling, ceiling, skirting");
  await expect(page.locator(".chart").filter({ hasText: "Cost by trade" }).locator(".bar-row")).toHaveCount(3);
  await expect(page.locator(".chart").filter({ hasText: "Cost by room" }).locator(".bar-row")).toHaveCount(4);

  await page.getByText("Finishes per room").click();
  await page.getByLabel("Bathroom flooring").uncheck();
  await page.getByLabel("Bathroom wall tiling percent").fill("50");
  await page.getByLabel("Wall tiling rate").fill("30");
  await page.getByRole("button", { name: "Recalculate" }).click();
  await expect(page.locator(".version-item")).toHaveCount(2);
  await expect(page.locator(".assumptions")).toContainText("Bathroom: no flooring, 50% wall tiling");
  const bathroom = page.locator(".data-table--numeric tbody tr").filter({ hasText: "Bathroom" });
  await expect(bathroom.locator("td").nth(6)).not.toHaveText("—");

  await page.getByText("Compare versions").click();
  const compare = page.locator(".compare-table");
  await expect(compare.locator("thead")).toContainText("v1");
  await expect(compare.locator("thead")).toContainText("v2");
  await expect(compare.locator("tr").filter({ hasText: "Wall tiling cost" }).locator(".delta--up")).toBeVisible();
  await expect(compare.locator("tr").filter({ hasText: "Flooring cost" }).locator(".delta--down")).toBeVisible();
  await expect(compare.locator("tfoot .delta")).toBeVisible();
});

test("settings pre-fill estimate rates and the default wall height", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await page.getByLabel("Flooring rate").fill("25");
  await page.getByLabel("Waste allowance").fill("5");
  await page.getByLabel("Default wall height").fill("8");
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(page.getByText("Settings saved.")).toBeVisible();
  await page.reload();

  const candidate = { id: "st", analysisState: "AnalysisReady", rooms: [{ id: "r1", name: "Hall", length: { value: 12, unit: "ft" }, width: { value: 10, unit: "ft" }, wallHeight: null, confidence: 1 }], doors: [], windows: [] };
  await page.route("**/api/floor-plans/*/analysis", (route) => route.fulfill({ status: 202, json: { id: "st", analysisState: "Analyzing" } }));
  await page.route("**/api/analyses/st", (route) => route.fulfill({ json: candidate }));
  await page.route("**/api/analyses/st/confirmed-data", (route) => route.fulfill({ json: { ...route.request().postDataJSON(), confirmationRevision: 1 } }));
  await createProject(page, unique("Settings house"));
  await analyzeDemo(page);
  await expect(page.getByLabel("Hall wall height")).toHaveValue("8");
  await expect(page.getByText("Default (8 ft)")).toBeVisible();
  await page.getByRole("button", { name: "Confirm and build model" }).click();
  await page.getByRole("button", { name: "Continue to estimate" }).click();
  await expect(page.getByLabel("Flooring rate")).toHaveValue("25");
  await expect(page.getByLabel("Waste allowance")).toHaveValue("5");
});

test("analysis details show the engine and a re-run replaces the analysis", async ({ page }) => {
  await page.route("**/api/analysis-engines", (route) => route.fulfill({ json: ["demo", "gemini"] }));
  await createProject(page, unique("Rerun house"));
  await analyzeDemo(page);
  await expect(page.locator(".info-panel summary")).toContainText("Demo (demo)");

  await page.route("**/api/floor-plans/*/analysis", async (route) => {
    expect(route.request().postDataJSON()).toEqual({ engine: "gemini" });
    await route.fulfill({ status: 202, json: { id: "rerun", analysisState: "Analyzing" } });
  });
  await page.route("**/api/analyses/rerun", (route) => route.fulfill({ json: { id: "rerun", analysisState: "AnalysisReady", engine: "gemini", model: "gemini-test", startedAt: new Date(Date.now() - 4000).toISOString(), completedAt: new Date().toISOString(),
    rooms: [{ id: "r1", name: "Studio", length: meters(5), width: meters(4), wallHeight: null, confidence: 0.6, uncertainty: "Blurred" }, { id: "r2", name: "Bath", length: meters(2), width: meters(2), wallHeight: meters(2.4), confidence: 1 }], doors: [], windows: [] } }));
  page.once("dialog", (dialog) => dialog.accept());
  await page.getByText("Analysis details").click();
  await expect(page.getByLabel("Re-run engine")).toHaveValue("gemini");
  await page.getByRole("button", { name: "Re-run analysis" }).click();
  await expect(page.locator(".info-panel summary")).toContainText("Google Gemini (gemini-test)");
  await expect(page.locator(".data-table").first().locator("tbody tr")).toHaveCount(2);
  await page.getByText("Analysis details").click();
  await expect(page.locator(".info-panel")).toContainText("1 wall height");
  await expect(page.locator(".info-panel")).toContainText("1 room");
});

test("confirmation history records each revision and its changes", async ({ page }) => {
  await createProject(page, unique("History house"));
  await confirmDemo(page);
  await page.getByRole("button", { name: "Back to review" }).click();
  await page.getByLabel("Room name").first().fill("Lounge");
  await page.getByRole("button", { name: "Confirm and build model" }).click();
  await expect(page.getByText("Revision 2")).toBeVisible();
  await page.getByRole("button", { name: "Overview" }).click();
  const history = page.getByRole("list", { name: "Confirmation history" });
  await expect(history.locator(".history-item")).toHaveCount(2);
  await expect(history).toContainText("Renamed Living room to Lounge");
  await expect(history).toContainText("Initial confirmation");
});

test("model follows extracted room bounds and one fitted scale", () => {
  const rooms: Room[] = [
    { id: "a", name: "Living", length: meters(4), width: meters(5), wallHeight: meters(2.8), bounds: { x: 0, y: 0, width: 0.4, height: 0.5 } },
    { id: "b", name: "Kitchen", length: meters(4), width: meters(5), wallHeight: meters(2.8), bounds: { x: 0.4, y: 0, width: 0.4, height: 0.5 } },
  ];
  const model = buildModel(rooms, 1);
  expect(model.rooms).toHaveLength(2);
  expect(model.rooms[0].length).toBeCloseTo(4);
  expect(model.rooms[0].width).toBeCloseTo(5);
  expect(model.rooms[0].x + model.rooms[0].length).toBeCloseTo(model.rooms[1].x);
  expect(model.approximate).toEqual([]);
});

test("rooms without bounds are packed approximately and incomplete rooms skipped", () => {
  const model = buildModel([
    { id: "a", name: "Living", length: meters(4), width: meters(5), wallHeight: meters(2.8), bounds: { x: 0, y: 0, width: 0.5, height: 0.5 } },
    { id: "b", name: "Store", length: meters(2), width: meters(2), wallHeight: null },
    { id: "c", name: "Unknown", length: null, width: meters(2), wallHeight: meters(2.8) },
  ], 1);
  expect(model.approximate).toEqual(["Store"]);
  expect(model.skipped).toEqual(["Unknown"]);
  const store = model.rooms.find((room) => room.id === "b")!;
  expect(store.approximate).toBe(true);
  expect(store.height).toBeCloseTo(2.8);
  expect(store.z).toBeGreaterThanOrEqual(model.rooms[0].z + model.rooms[0].width - 1e-9);
});

test("project name is required before the workflow continues", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "New project" }).click();
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page.locator(".alert--error")).toHaveText("Enter a project name.");
  await expect(page.getByRole("button", { name: /Floor plan/ })).toBeDisabled();
});

test("guided workflow from project details to exported estimate", async ({ page }) => {
  await createProject(page);
  await expect(page.locator(".header-project")).toContainText("Smith residence");
  await analyzeDemo(page);

  await expect(page.locator(".data-table").first().locator("tbody tr")).toHaveCount(4);
  const confirm = page.getByRole("button", { name: "Confirm and build model" });
  await expect(confirm).toBeDisabled();
  await page.getByRole("button", { name: "Looks right" }).click();
  await confirm.click();

  await expect(page.getByRole("heading", { name: "3D model" })).toBeVisible();
  await expect(page.getByText("AI-estimated layout")).toBeVisible();
  await expect(page.getByAltText("Uploaded floor plan")).toBeVisible();
  await expect(page.locator(".drawing-label")).toHaveCount(4);
  await expect(page.locator(".scene-host canvas")).toBeVisible();
  await expect(page.getByText("From drawing")).toHaveCount(4);
  await page.getByRole("button", { name: "Continue to estimate" }).click();

  await expect(page.getByText("No estimate yet")).toBeVisible();
  await page.getByRole("button", { name: "Calculate estimate" }).click();
  await expect(page.locator(".metric-card--total")).toContainText("$");
  await expect(page.locator(".data-table--numeric tbody tr")).toHaveCount(4);

  const excel = page.waitForEvent("download");
  await page.getByRole("button", { name: "Export Excel" }).click();
  const workbook = await excel;
  expect(workbook.suggestedFilename()).toBe("smith-residence-estimate-v1.xlsx");
  expect((await bytes(workbook)).length).toBeGreaterThan(100);

  const csv = page.waitForEvent("download");
  await page.getByRole("button", { name: "Export CSV" }).click();
  const csvText = (await bytes(await csv)).toString("utf8");
  expect(csvText).toContain("Living room");
  expect(csvText.split("\r\n")).toHaveLength(6);

  await page.getByLabel("Plaster rate").fill("12");
  await page.getByRole("button", { name: "Recalculate" }).click();
  await expect(page.locator(".version-item")).toHaveCount(2);

  await page.reload();
  await expect(page.getByRole("heading", { name: "Estimate" })).toBeVisible();
  await expect(page.locator(".version-item")).toHaveCount(2);
  await page.getByRole("button", { name: /3D model/ }).click();
  await expect(page.getByAltText("Uploaded floor plan")).toBeVisible();
});

test("analysis progress is visible while the AI works", async ({ page }) => {
  let polls = 0;
  await page.route("**/api/floor-plans/*/analysis", (route) => route.fulfill({ status: 202, json: { id: "slow-analysis", analysisState: "Analyzing" } }));
  await page.route("**/api/analyses/slow-analysis", (route) => {
    polls += 1;
    return route.fulfill({ json: polls < 3 ? { analysisState: "Analyzing" } : { id: "slow-analysis", analysisState: "AnalysisReady", rooms: [{ id: "r1", name: "Hall", length: meters(3), width: meters(3), wallHeight: meters(2.7), confidence: 1 }], doors: [], windows: [] } });
  });
  await createProject(page);
  await page.locator('input[type="file"]').setInputFiles({ name: "plan.png", mimeType: "image/png", buffer: planPng });
  await page.getByRole("button", { name: "Analyze drawing" }).click();
  await expect(page.getByRole("button", { name: "Analyzing…" })).toBeDisabled();
  await expect(page.locator(".progress-stage--done")).toContainText("Upload drawing");
  await expect(page.locator(".progress-stage--active")).toContainText("AI extracts rooms");
  await expect(page.getByRole("heading", { name: "Review the extraction" })).toBeVisible();
});

test("model without extracted bounds arranges rooms approximately", async ({ page }) => {
  const candidate = { id: "nb", analysisState: "AnalysisReady", rooms: [
    { id: "r1", name: "Hall", length: meters(3), width: meters(3), wallHeight: meters(2.7), confidence: 1 },
    { id: "r2", name: "Study", length: meters(3), width: meters(2), wallHeight: meters(2.7), confidence: 1 },
  ], doors: [], windows: [] };
  await page.route("**/api/floor-plans/*/analysis", (route) => route.fulfill({ status: 202, json: { id: "nb", analysisState: "Analyzing" } }));
  await page.route("**/api/analyses/nb", (route) => route.fulfill({ json: candidate }));
  await page.route("**/api/analyses/nb/confirmed-data", (route) => route.fulfill({ json: { ...route.request().postDataJSON(), confirmationRevision: 1 } }));
  await createProject(page);
  await analyzeDemo(page);
  await page.getByRole("button", { name: "Confirm and build model" }).click();
  await expect(page.getByText("did not return room boundaries", { exact: false })).toBeVisible();
  await expect(page.getByText("Approximate", { exact: true })).toHaveCount(2);
  await expect(page.locator(".scene-host canvas")).toBeVisible();
});

test("missing wall height defaults to 6 ft or its unit equivalent", async ({ page }) => {
  const feet = (value: number) => ({ value, unit: "ft" });
  const candidate = { id: "wh", analysisState: "AnalysisReady", rooms: [
    { id: "r1", name: "Hall", length: feet(12), width: feet(10), wallHeight: null, confidence: 1 },
  ], doors: [], windows: [] };
  await page.route("**/api/floor-plans/*/analysis", (route) => route.fulfill({ status: 202, json: { id: "wh", analysisState: "Analyzing" } }));
  await page.route("**/api/analyses/wh", (route) => route.fulfill({ json: candidate }));
  await createProject(page);
  await analyzeDemo(page);
  const height = page.getByLabel("Hall wall height");
  await expect(height).toHaveValue("6");
  await expect(page.getByText("Default (6 ft)")).toBeVisible();
  await page.getByLabel("Hall unit").selectOption("m");
  await expect(height).toHaveValue("1.83");
  await height.fill("2.4");
  await expect(page.getByText("Default (6 ft)")).toHaveCount(0);
  await page.getByLabel("Hall unit").selectOption("cm");
  await expect(height).toHaveValue("2.4");
});

test("selected engine is sent and failures offer a retry", async ({ page }) => {
  await page.route("**/api/analysis-engines", (route) => route.fulfill({ json: ["demo", "gemini"] }));
  await page.route("**/api/floor-plans/*/analysis", async (route) => {
    expect(route.request().postDataJSON()).toEqual({ engine: "gemini" });
    await route.fulfill({ status: 400, json: { message: "Gemini is unavailable" } });
  });
  await createProject(page);
  await expect(page.getByLabel(/Google Gemini/)).toBeChecked();
  await page.locator('input[type="file"]').setInputFiles({ name: "plan.png", mimeType: "image/png", buffer: planPng });
  await page.getByRole("button", { name: "Analyze drawing" }).click();
  await expect(page.locator(".alert--error")).toHaveText("Gemini is unavailable");
  await expect(page.getByRole("button", { name: "Retry analysis" })).toBeEnabled();
});

test("unconfigured engines are shown but cannot be selected", async ({ page }) => {
  await page.route("**/api/analysis-engines", (route) => route.fulfill({ json: ["demo"] }));
  await createProject(page);
  await expect(page.getByLabel(/Google Gemini/)).toBeDisabled();
  await expect(page.getByLabel(/Demo/)).toBeChecked();
});

test("model step keeps room details when WebGL is unavailable", async ({ page }) => {
  await page.addInitScript(() => {
    const original = HTMLCanvasElement.prototype.getContext;
    HTMLCanvasElement.prototype.getContext = function (this: HTMLCanvasElement, ...args: Parameters<typeof original>) {
      return String(args[0]).startsWith("webgl") ? null : original.apply(this, args);
    } as typeof original;
  });
  await createProject(page);
  await analyzeDemo(page);
  await page.getByRole("button", { name: "Looks right" }).click();
  await page.getByRole("button", { name: "Confirm and build model" }).click();
  await expect(page.getByText("3D rendering is unavailable", { exact: false })).toBeVisible();
  await expect(page.getByRole("cell", { name: "Kitchen" })).toBeVisible();
  await page.getByRole("button", { name: "Continue to estimate" }).click();
  await expect(page.getByRole("button", { name: "Calculate estimate" })).toBeVisible();
});

test("workflow fits a mobile screen", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await createProject(page, "Mobile house");
  await analyzeDemo(page);
  await page.getByRole("button", { name: "Looks right" }).click();
  await page.getByRole("button", { name: "Confirm and build model" }).click();
  await expect(page.locator(".scene-host canvas")).toBeVisible();
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(overflow).toBeLessThanOrEqual(1);
  await page.locator(".scene-host").scrollIntoViewIfNeeded();
  await page.screenshot({ path: "test-results/model-mobile.png" });
});
