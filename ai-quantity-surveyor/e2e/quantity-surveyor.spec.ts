import { expect, test, type Download, type Page } from "@playwright/test";
import type { Room } from "../src/app/api";
import { buildModel } from "../src/app/preview";

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
