import { expect, test, type Page } from "@playwright/test";
import { roofMeshData, validExterior, type ExteriorInput, type ExteriorOption, type RoofForm } from "../src/app/exterior";

const project = { id: "roof-project", name: "Roof sample", createdAt: "2026-10-21T00:00:00Z", status: "Confirmed" };
const candidate = { id: "roof-analysis", floorPlanId: "roof-plan", analysisState: "AnalysisReady", rooms: [{ id: "room", name: "Living room", length: { value: 8, unit: "m" }, width: { value: 6, unit: "m" }, wallHeight: { value: 2.7, unit: "m" } }], doors: [], windows: [] };
const confirmation = { analysisId: candidate.id, revision: 2, confirmedAt: project.createdAt, candidate: { ...candidate, confirmationRevision: 2 } };
const previous = { ...confirmation, revision: 1, candidate: { ...candidate, confirmationRevision: 1 } };
const summary = { project, stage: "Confirmed", floorPlan: { id: "roof-plan", fileName: "plan.png", fileSize: 1 }, analysisId: candidate.id, confirmedRevision: 2, roomCount: 1, openingCount: 0, estimateCount: 0, updatedAt: project.createdAt };
const user = { id: "roof-user", aud: "authenticated", role: "authenticated", email: "roof@example.test", app_metadata: { provider: "email", providers: ["email"] }, user_metadata: {}, identities: [], created_at: project.createdAt };

async function setup(page: Page) {
  let saved: ExteriorOption[] = [];
  const mutations: string[] = [];
  await page.emulateMedia({ colorScheme: "light" });
  await page.route("**/auth/v1/token**", (route) => route.fulfill({ json: { access_token: "roof-fixture-token", token_type: "bearer", expires_in: 3600, refresh_token: "roof-fixture-refresh", user } }));
  await page.route("**/api/**", async (route) => {
    const request = route.request(), url = new URL(request.url()), path = url.pathname;
    if (request.method() !== "GET") mutations.push(`${request.method()} ${path}`);
    if (path === "/api/account/plan") return route.fulfill({ json: { plan: { id: "pro", name: "Pro", features: ["model3d", "aiAnalysis", "materials", "exports"] }, assignmentSource: "manual", demoSwitchingEnabled: false } });
    if (path === "/api/analysis-engines") return route.fulfill({ json: ["demo"] });
    if (path === "/api/projects") return route.fulfill({ json: [summary] });
    if (path.endsWith("/summary")) return route.fulfill({ json: summary });
    if (path === `/api/analyses/${candidate.id}`) return route.fulfill({ json: candidate });
    if (path.endsWith("/confirmed-data")) return route.fulfill({ json: confirmation });
    if (path.endsWith("/confirmations")) return route.fulfill({ json: [confirmation, previous] });
    if (path.endsWith("/estimates")) return route.fulfill({ json: [] });
    if (path.endsWith("/exterior-options")) {
      expect(request.headers().authorization).toBe("Bearer roof-fixture-token");
      if (request.method() === "GET") return route.fulfill({ json: saved.filter((entry) => entry.confirmationRevision === Number(url.searchParams.get("revision"))) });
      const option = { ...request.postDataJSON(), id: `roof-option-${saved.length + 1}`, projectId: project.id, assumptions: "Reviewed conceptual single-storey assumption", createdAt: project.createdAt } as ExteriorOption;
      saved = [option, ...saved]; return route.fulfill({ status: 201, json: option });
    }
    return route.fulfill({ status: 404, json: { message: "Fixture endpoint unavailable" } });
  });
  await page.goto("/");
  await page.getByLabel("Email").fill(user.email);
  await page.getByLabel("Password", { exact: true }).fill("fixture-password-1");
  await page.getByRole("button", { name: "Sign in", exact: true }).last().click();
  await page.getByRole("button", { name: project.name, exact: true }).click();
  await page.getByRole("button", { name: "Exterior", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Exterior and roof" })).toBeVisible();
  return { saved: () => saved, mutations };
}

test("roof geometry honors pitch, overhang and supported bounds", () => {
  const original: ExteriorInput = { analysisId: "analysis", confirmationRevision: 1, roof: "gable", pitchDegrees: 30, overhangM: 0.4, lengthM: 12, widthM: 8, wallHeightM: 3, finish: "metal", assumptionsReviewed: false };
  const snapshot = JSON.stringify(original);
  for (const roof of ["flat", "gable", "hip", "shed"] as RoofForm[]) {
    const input = { ...original, roof, pitchDegrees: roof === "flat" ? 0 : 30 };
    const data = roofMeshData(input);
    expect(data.roof.length).toBeGreaterThan(0);
    expect(data.roof.every(Number.isFinite)).toBe(true);
    expect(data.rise).toBeCloseTo(roof === "flat" ? 0 : Math.tan(Math.PI / 6) * (roof === "shed" ? 8.8 : 4.4));
    const rotated = roofMeshData({ ...input, lengthM: 8, widthM: 12 });
    expect(rotated.roof.every(Number.isFinite)).toBe(true);
  }
  expect(validExterior({ ...original, lengthM: NaN })).toBe(false);
  expect(() => roofMeshData({ ...original, pitchDegrees: 80 })).toThrow();
  expect(JSON.stringify(original)).toBe(snapshot);
});

test("roof options render all forms and reopen exact confirmed snapshots", async ({ page }) => {
  const renderingErrors: string[] = [];
  page.on("pageerror", (error) => renderingErrors.push(error.message));
  page.on("console", (message) => { if (["warning", "error"].includes(message.type()) && message.text().includes("THREE.")) renderingErrors.push(message.text()); });
  const fixture = await setup(page);
  await expect(page.getByText("Facade colors, trim and landscaping are illustrative styling only, not verified finishes or site works. Doors and windows are not shown.", { exact: true })).toBeVisible();
  const canvas = page.locator(".exterior-host canvas");
  await expect(canvas).toBeVisible();
  const samples = new Set<string>();
  for (const roof of ["Flat", "Gable", "Hip", "Shed"]) {
    await page.getByRole("radio", { name: roof, exact: true }).check();
    await expect(page.getByRole("heading", { name: `${roof} roof`, exact: true })).toBeVisible();
    if (roof === "Flat") await expect(page.getByRole("slider", { name: "Roof pitch", exact: true })).toBeDisabled();
    await page.locator(".exterior-preview").screenshot({ path: `test-results/exterior-${roof.toLowerCase()}-desktop.png` });
    await expect.poll(() => canvas.evaluate((element) => {
      const sample = document.createElement("canvas"); sample.width = 64; sample.height = 64;
      const context = sample.getContext("2d")!; context.drawImage(element as HTMLCanvasElement, 0, 0, 64, 64);
      const pixels = context.getImageData(0, 0, 64, 64).data;
      let colored = 0;
      for (let offset = 0; offset < pixels.length; offset += 4) { const channels = [pixels[offset], pixels[offset + 1], pixels[offset + 2]]; const total = channels.reduce((sum, value) => sum + value, 0); if (Math.max(...channels) - Math.min(...channels) > 30 && total > 150 && total < 650) colored++; }
      return colored;
    })).toBeGreaterThan(30);
    samples.add(await canvas.evaluate((element) => (element as HTMLCanvasElement).toDataURL()));
  }
  expect(samples.size).toBe(4);
  expect(fixture.mutations).toEqual([]);
  await page.getByRole("slider", { name: "Roof pitch", exact: true }).focus(); await page.keyboard.press("ArrowRight");
  await page.getByRole("slider", { name: "Roof overhang", exact: true }).focus(); await page.keyboard.press("ArrowRight");
  await expect(page.getByRole("button", { name: "Save exterior option", exact: true })).toBeDisabled();
  await page.getByLabel("I reviewed the exterior assumptions").check();
  await page.getByRole("button", { name: "Save exterior option", exact: true }).click();
  await expect(page.locator(".alert--success")).toContainText("Estimates are unchanged");
  expect(fixture.saved()[0]).toMatchObject({ roof: "shed", pitchDegrees: 26, overhangM: 0.5, analysisId: candidate.id, confirmationRevision: 2 });
  expect(fixture.mutations).toEqual([`POST /api/projects/${project.id}/exterior-options`]);
  await page.getByRole("button", { name: "Switch to dark theme" }).click();
  await page.setViewportSize({ width: 390, height: 844 });
  for (const roof of ["Flat", "Gable", "Hip", "Shed"]) {
    await page.getByRole("radio", { name: roof, exact: true }).check();
    await expect(canvas).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await page.locator(".exterior-preview").screenshot({ path: `test-results/exterior-${roof.toLowerCase()}-mobile.png` });
  }
  const beforeOrbit = await canvas.evaluate((element) => (element as HTMLCanvasElement).toDataURL());
  const bounds = (await canvas.boundingBox())!;
  await page.mouse.move(bounds.x + bounds.width / 2, bounds.y + bounds.height / 2); await page.mouse.down(); await page.mouse.move(bounds.x + bounds.width / 2 + 70, bounds.y + bounds.height / 2 + 20, { steps: 8 }); await page.mouse.up();
  await expect.poll(() => canvas.evaluate((element) => (element as HTMLCanvasElement).toDataURL())).not.toBe(beforeOrbit);
  await page.getByRole("button", { name: "Reset exterior view", exact: true }).click();
  const downloading = page.waitForEvent("download");
  await page.getByRole("button", { name: "Download exterior screenshot", exact: true }).click();
  const download = await downloading;
  expect(download.suggestedFilename()).toBe("conceptual-shed-roof.png");
  expect(await download.failure()).toBeNull();
  await page.reload();
  await page.getByRole("button", { name: "Exterior", exact: true }).click();
  await expect(page.getByRole("radio", { name: "Shed", exact: true })).toBeChecked();
  await expect(page.getByRole("slider", { name: "Roof pitch", exact: true })).toHaveValue("26");
  await page.getByLabel("Exterior source revision").selectOption("1");
  await expect(page.getByLabel("Saved exterior option")).toHaveValue("");
  await page.getByLabel("Exterior source revision").selectOption("2");
  await expect(page.getByRole("radio", { name: "Shed", exact: true })).toBeChecked();
  expect(renderingErrors).toEqual([]);
});

test("roof finish colors are distinct without changing dimensions or saving data", async ({ page }) => {
  const fixture = await setup(page);
  const canvas = page.locator(".exterior-host canvas");
  const appearance = page.getByRole("combobox", { name: "Roof appearance", exact: true });
  for (const finish of ["metal", "tile", "membrane"]) {
    await appearance.selectOption(finish);
    await expect.poll(() => canvas.evaluate((element, selectedFinish) => {
      const sample = document.createElement("canvas"); sample.width = 64; sample.height = 64;
      const context = sample.getContext("2d")!; context.drawImage(element as HTMLCanvasElement, 0, 0, 64, 64);
      const pixels = context.getImageData(0, 0, 64, 32).data;
      let roofPixels = 0;
      for (let offset = 0; offset < pixels.length; offset += 4) {
        const [red, green, blue] = [pixels[offset], pixels[offset + 1], pixels[offset + 2]];
        if (selectedFinish === "metal" ? green > red * 1.5 && blue > red * 1.5 : selectedFinish === "tile" ? red > green * 1.3 && red > blue * 1.3 : blue > red * 1.2 && blue > green * 1.05) roofPixels++;
      }
      return roofPixels;
    }, finish)).toBeGreaterThan(30);
  }
  await expect(page.getByRole("spinbutton", { name: "Envelope length m", exact: true })).toHaveValue("8");
  await expect(page.getByRole("spinbutton", { name: "Envelope width m", exact: true })).toHaveValue("6");
  await expect(page.getByRole("spinbutton", { name: "Exterior wall height m", exact: true })).toHaveValue("2.7");
  expect(fixture.mutations).toEqual([]);
});

test("non-WebGL exterior retains editable and saved roof choices", async ({ page }) => {
  await page.addInitScript(() => {
    const original = HTMLCanvasElement.prototype.getContext;
    Object.defineProperty(HTMLCanvasElement.prototype, "getContext", { value: function(this: HTMLCanvasElement, kind: string, ...args: unknown[]) { if (kind.startsWith("webgl")) return null; return Reflect.apply(original, this, [kind, ...args]); } });
  });
  const fixture = await setup(page);
  await expect(page.locator(".exterior-render .alert--warning")).toContainText("3D rendering is unavailable");
  await page.getByRole("radio", { name: "Hip", exact: true }).check();
  await page.getByLabel("I reviewed the exterior assumptions").check();
  await page.getByRole("button", { name: "Save exterior option", exact: true }).click();
  await expect(page.getByLabel("Saved exterior option")).toHaveValue("roof-option-1");
  expect(fixture.saved()[0].roof).toBe("hip");
});