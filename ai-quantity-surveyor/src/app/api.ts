export const API = (process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080").replace(/\/$/, "");

export type Measurement = { value: number; unit: string };
export type Bounds = { x: number; y: number; width: number; height: number };
export type Room = {
  id: string;
  name: string;
  length: Measurement | null;
  width: Measurement | null;
  wallHeight: Measurement | null;
  bounds?: Bounds | null;
  confidence?: number;
  uncertainty?: string;
  assumptions?: string[];
};
export type Opening = { id: string; roomId?: string; width: Measurement | null; height: Measurement | null; confidence?: number; uncertainty?: string };
export type Candidate = {
  id?: string;
  floorPlanId?: string;
  analysisState: string;
  drawingAspect?: number;
  engine?: string;
  model?: string;
  startedAt?: string;
  completedAt?: string;
  rooms?: Room[];
  doors?: Opening[];
  windows?: Opening[];
  errorMessage?: string;
  confirmationRevision?: number;
};
export type Confirmation = { analysisId: string; revision: number; confirmedAt?: string; candidate: Candidate & { rooms: Room[] } };
export type Project = { id: string; name: string; client?: string; location?: string; description?: string; createdAt: string; status: string; archived?: boolean };
export type FloorPlan = { id: string; fileName: string; fileSize: number; uploadedAt?: string };
export type ProjectSummary = {
  project: Project;
  stage: "Draft" | "Uploaded" | "Analyzed" | "Confirmed" | "Estimated";
  floorPlan?: FloorPlan;
  analysisId?: string;
  engine?: string;
  confirmedRevision?: number;
  roomCount: number;
  openingCount: number;
  estimateCount: number;
  latestTotal?: number;
  latestCurrency?: string;
  updatedAt: string;
};
export type ProjectDetails = { name: string; client: string; location: string; description: string };
export type RoomFinish = { excludePlaster?: boolean; excludePaint?: boolean; excludeFlooring?: boolean; excludeCeiling?: boolean; excludeSkirting?: boolean; tilePercent?: number };
export type RoomCalculation = {
  roomId: string;
  roomName: string;
  floorArea: number;
  perimeter?: number;
  grossWallArea: number;
  doorDeduction: number;
  windowDeduction: number;
  netWallArea?: number;
  netPlasterArea: number;
  paintArea: number;
  tileArea?: number;
  flooringArea?: number;
  ceilingArea?: number;
  skirtingLength?: number;
  plasterCostUsd: number;
  paintCostUsd: number;
  tilingCostUsd?: number;
  flooringCostUsd?: number;
  ceilingCostUsd?: number;
  skirtingCostUsd?: number;
  totalCostUsd?: number;
};
export type ServerRates = {
  plasterUsdPerSquareMeter: number;
  paintUsdPerSquareMeter: number;
  flooringUsdPerSquareMeter: number;
  ceilingUsdPerSquareMeter: number;
  skirtingUsdPerMeter: number;
  tilingUsdPerSquareMeter: number;
  wastePercent: number;
};
export type EstimateVersion = {
  id: string;
  projectId: string;
  version: number;
  sourceAnalysisId: string;
  targetCurrency: string;
  exchangeRate: number;
  exchangeRateFetchedAt: string;
  rates?: ServerRates;
  finishes?: Record<string, RoomFinish>;
  result: {
    rooms: RoomCalculation[];
    totalPlasterQuantity: number;
    totalPaintQuantity: number;
    totalTilingQuantity?: number;
    totalFlooringQuantity?: number;
    totalCeilingQuantity?: number;
    totalSkirtingQuantity?: number;
    totalPlasterCostUsd: number;
    totalPaintCostUsd: number;
    totalTilingCostUsd?: number;
    totalFlooringCostUsd?: number;
    totalCeilingCostUsd?: number;
    totalSkirtingCostUsd?: number;
    grandTotalUsd: number;
  };
  convertedPlasterCost: number;
  convertedPaintCost: number;
  convertedGrandTotal: number;
  calculatedAt: string;
};

// Trades in display order; quantity/cost keys index EstimateVersion.result and RoomCalculation.
export const trades = [
  { key: "plaster", label: "Plaster", unit: "m²", quantity: "totalPlasterQuantity", cost: "totalPlasterCostUsd", roomQuantity: "netPlasterArea", roomCost: "plasterCostUsd", rate: "plasterUsdPerSquareMeter" },
  { key: "paint", label: "Paint", unit: "m²", quantity: "totalPaintQuantity", cost: "totalPaintCostUsd", roomQuantity: "paintArea", roomCost: "paintCostUsd", rate: "paintUsdPerSquareMeter" },
  { key: "tiling", label: "Wall tiling", unit: "m²", quantity: "totalTilingQuantity", cost: "totalTilingCostUsd", roomQuantity: "tileArea", roomCost: "tilingCostUsd", rate: "tilingUsdPerSquareMeter" },
  { key: "flooring", label: "Flooring", unit: "m²", quantity: "totalFlooringQuantity", cost: "totalFlooringCostUsd", roomQuantity: "flooringArea", roomCost: "flooringCostUsd", rate: "flooringUsdPerSquareMeter" },
  { key: "ceiling", label: "Ceiling", unit: "m²", quantity: "totalCeilingQuantity", cost: "totalCeilingCostUsd", roomQuantity: "ceilingArea", roomCost: "ceilingCostUsd", rate: "ceilingUsdPerSquareMeter" },
  { key: "skirting", label: "Skirting", unit: "m", quantity: "totalSkirtingQuantity", cost: "totalSkirtingCostUsd", roomQuantity: "skirtingLength", roomCost: "skirtingCostUsd", rate: "skirtingUsdPerMeter" },
] as const;
export type Trade = (typeof trades)[number];

export const tradeQuantity = (version: EstimateVersion, trade: Trade) => version.result[trade.quantity] ?? 0;
export const tradeCost = (version: EstimateVersion, trade: Trade) => (version.result[trade.cost] ?? 0) * version.exchangeRate;
export const roomTotal = (room: RoomCalculation) => room.totalCostUsd ?? room.plasterCostUsd + room.paintCostUsd;

// Older versions did not store rates; derive them from cost / quantity.
export function versionRates(version: EstimateVersion): ServerRates {
  if (version.rates) return version.rates;
  const derive = (cost: number, quantity: number) => (quantity > 0 ? cost / quantity : 0);
  return {
    plasterUsdPerSquareMeter: derive(version.result.totalPlasterCostUsd, version.result.totalPlasterQuantity),
    paintUsdPerSquareMeter: derive(version.result.totalPaintCostUsd, version.result.totalPaintQuantity),
    flooringUsdPerSquareMeter: 0, ceilingUsdPerSquareMeter: 0, skirtingUsdPerMeter: 0, tilingUsdPerSquareMeter: 0, wastePercent: 0,
  };
}

export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API}${path}`, init);
  if (!response.ok) {
    let message = `Request failed (${response.status})`;
    try {
      const body = await response.json() as { message?: string };
      if (body.message) message = body.message;
    } catch {}
    throw new Error(message);
  }
  return response.json() as Promise<T>;
}

export function jsonInit(method: string, body: unknown): RequestInit {
  return { method, headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) };
}

export function formatNumber(value: number, digits = 2) {
  return new Intl.NumberFormat(undefined, { minimumFractionDigits: digits, maximumFractionDigits: digits }).format(value);
}

export function formatMoney(value: number, currency: string) {
  try {
    return new Intl.NumberFormat(undefined, { style: "currency", currency }).format(value);
  } catch {
    return `${formatNumber(value)} ${currency}`;
  }
}

export function describeMeasurement(measurement: Measurement | null | undefined) {
  return measurement ? `${measurement.value} ${measurement.unit}` : "—";
}

export function formatDate(value?: string) {
  if (!value) return "—";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) || date.getFullYear() < 2000 ? "—" : date.toLocaleString();
}

export async function fetchDrawing(floorPlan: FloorPlan): Promise<File> {
  const response = await fetch(`${API}/api/floor-plans/${encodeURIComponent(floorPlan.id)}/file`);
  if (!response.ok) throw new Error("Drawing unavailable");
  const blob = await response.blob();
  return new File([blob], floorPlan.fileName, { type: blob.type });
}

export const engineLabels: Record<string, string> = { demo: "Demo", ollama: "Ollama", gemini: "Google Gemini" };

// Starts an analysis and polls until it leaves the Analyzing state.
export async function runAnalysis(floorPlanId: string, engine: string, isCancelled: () => boolean = () => false): Promise<{ id: string; candidate: Candidate } | null> {
  const started = await request<{ id: string }>(`/api/floor-plans/${encodeURIComponent(floorPlanId)}/analysis`, jsonInit("POST", { engine }));
  const deadline = Date.now() + (engine === "gemini" ? 6 : 2) * 60_000;
  let result: Candidate = { analysisState: "Analyzing" };
  while (result.analysisState === "Analyzing") {
    if (isCancelled()) return null;
    if (Date.now() > deadline) throw new Error("The analysis is taking too long. Try again or choose another engine.");
    await new Promise((resolve) => setTimeout(resolve, 1200));
    result = await request<Candidate>(`/api/analyses/${started.id}`);
  }
  if (result.analysisState !== "AnalysisReady") throw new Error(result.errorMessage || "The analysis failed.");
  return { id: started.id, candidate: result };
}
