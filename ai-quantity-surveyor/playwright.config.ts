import { defineConfig, devices } from "@playwright/test";

const webPort = Number(process.env.PLAYWRIGHT_WEB_PORT || 3001);
const webURL = `http://localhost:${webPort}`;

export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  use: {
    baseURL: webURL,
    trace: "on-first-retry",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: [
    { command: `npm run dev -- --port ${webPort}`, url: webURL, reuseExistingServer: true, timeout: 120_000 },
    { command: "go run ./api/cmd/server", url: "http://localhost:8080/health", reuseExistingServer: true, timeout: 120_000 },
  ],
});
