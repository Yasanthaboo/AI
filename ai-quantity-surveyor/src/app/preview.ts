import type { Measurement, Room } from "./api";

export type ModelRoom = {
  id: string;
  name: string;
  x: number;
  z: number;
  length: number;
  width: number;
  height: number;
  approximate: boolean;
};

export type Model = { rooms: ModelRoom[]; approximate: string[]; skipped: string[]; extent: { width: number; depth: number } };

const metersPerUnit: Record<string, number> = { m: 1, cm: 0.01, ft: 0.3048, in: 0.0254 };
const defaultWallHeight = 2.7;
const defaultDrawingSpan = 15;

export function toMeters(measurement: Measurement | null | undefined): number | null {
  if (!measurement || !Number.isFinite(measurement.value) || measurement.value <= 0) return null;
  const factor = Object.hasOwn(metersPerUnit, measurement.unit) ? metersPerUnit[measurement.unit] : undefined;
  return factor ? measurement.value * factor : null;
}

function median(values: number[]) {
  const sorted = [...values].sort((left, right) => left - right);
  return sorted.length ? sorted[Math.floor(sorted.length / 2)] : 0;
}

// Footprints follow the AI-extracted drawing bounds; one scale is fitted from confirmed room areas.
export function buildModel(rooms: Room[], drawingAspect?: number): Model {
  const aspect = drawingAspect && drawingAspect > 0 ? drawingAspect : 1;
  const heights = rooms.map((room) => toMeters(room.wallHeight)).filter((value): value is number => value !== null);
  const fallbackHeight = heights.length ? median(heights) : defaultWallHeight;
  const bounded = rooms.filter((room) => room.bounds);
  const scales = bounded.flatMap((room) => {
    const length = toMeters(room.length);
    const width = toMeters(room.width);
    const planArea = room.bounds!.width * room.bounds!.height * aspect;
    return length && width && planArea > 0 ? [Math.sqrt((length * width) / planArea)] : [];
  });
  const scale = scales.length ? median(scales) : defaultDrawingSpan;
  const result: ModelRoom[] = [];
  const approximate: string[] = [];
  const skipped: string[] = [];
  let maxX = 0;
  let maxZ = 0;
  for (const room of bounded) {
    const bounds = room.bounds!;
    const placed = {
      id: room.id,
      name: room.name,
      x: bounds.x * scale,
      z: bounds.y * aspect * scale,
      length: bounds.width * scale,
      width: bounds.height * aspect * scale,
      height: toMeters(room.wallHeight) ?? fallbackHeight,
      approximate: false,
    };
    result.push(placed);
    maxX = Math.max(maxX, placed.x + placed.length);
    maxZ = Math.max(maxZ, placed.z + placed.width);
  }
  const unplaced = rooms.filter((room) => !room.bounds);
  const rowLimit = Math.max(maxX, Math.sqrt(unplaced.reduce((sum, room) => sum + (toMeters(room.length) ?? 0) * (toMeters(room.width) ?? 0), 0)) * 1.4);
  let cursorX = 0;
  let cursorZ = result.length ? maxZ : 0;
  let rowDepth = 0;
  for (const room of unplaced) {
    const length = toMeters(room.length);
    const width = toMeters(room.width);
    if (!length || !width) {
      skipped.push(room.name);
      continue;
    }
    if (cursorX > 0 && cursorX + length > rowLimit) {
      cursorX = 0;
      cursorZ += rowDepth;
      rowDepth = 0;
    }
    result.push({ id: room.id, name: room.name, x: cursorX, z: cursorZ, length, width, height: toMeters(room.wallHeight) ?? fallbackHeight, approximate: true });
    approximate.push(room.name);
    cursorX += length;
    rowDepth = Math.max(rowDepth, width);
    maxX = Math.max(maxX, cursorX);
    maxZ = Math.max(maxZ, cursorZ + width);
  }
  return { rooms: result, approximate, skipped, extent: { width: maxX, depth: maxZ } };
}