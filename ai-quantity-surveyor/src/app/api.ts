export const API = "http://localhost:8080";

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
};
export type Opening = { id: string; roomId?: string; width: Measurement | null; height: Measurement | null; confidence?: number; uncertainty?: string };
export type Candidate = {
  id?: string;
  analysisState: string;
  drawingAspect?: number;
  rooms?: Room[];
  doors?: Opening[];
  windows?: Opening[];
  errorMessage?: string;
  confirmationRevision?: number;
};
export type Confirmation = { analysisId: string; revision: number; candidate: Candidate & { rooms: Room[] } };
export type Project = { id: string; name: string; client?: string; location?: string; description?: string; createdAt: string; status: string };
export type ProjectDetails = { name: string; client: string; location: string; description: string };
export type RoomCalculation = {
  roomId: string;
  roomName: string;
  floorArea: number;
  grossWallArea: number;
  doorDeduction: number;
  windowDeduction: number;
  netPlasterArea: number;
  paintArea: number;
  plasterCostUsd: number;
  paintCostUsd: number;
};
export type EstimateVersion = {
  id: string;
  projectId: string;
  version: number;
  sourceAnalysisId: string;
  targetCurrency: string;
  exchangeRate: number;
  exchangeRateFetchedAt: string;
  result: {
    rooms: RoomCalculation[];
    totalPlasterQuantity: number;
    totalPaintQuantity: number;
    totalPlasterCostUsd: number;
    totalPaintCostUsd: number;
    grandTotalUsd: number;
  };
  convertedPlasterCost: number;
  convertedPaintCost: number;
  convertedGrandTotal: number;
  calculatedAt: string;
};
export type Rates = { plaster: number; paint: number; currency: string; exchangeRate: number };

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
