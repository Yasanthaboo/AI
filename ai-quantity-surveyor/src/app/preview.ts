import type { Measurement, Opening, Room } from "./api";

export type ModelOpening = { id: string; kind: "door" | "window"; width: number; height: number; sill: number };

export type ModelRoom = {
  id: string;
  name: string;
  x: number;
  z: number;
  length: number;
  width: number;
  height: number;
  approximate: boolean;
  openings: ModelOpening[];
  detail: string;
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

// Opening positions are not extracted; windows get an indicative 0.9 m sill.
const windowSill = 0.9;

function roomOpenings(roomId: string, openings: { doors?: Opening[]; windows?: Opening[] }): ModelOpening[] {
  const convert = (kind: ModelOpening["kind"]) => (opening: Opening): ModelOpening[] => {
    const width = toMeters(opening.width);
    const height = toMeters(opening.height);
    return opening.roomId === roomId && width && height ? [{ id: opening.id, kind, width, height, sill: kind === "window" ? windowSill : 0 }] : [];
  };
  return [...(openings.doors ?? []).flatMap(convert("door")), ...(openings.windows ?? []).flatMap(convert("window"))];
}

const describe = (measurement: Measurement | null) => (measurement ? `${measurement.value} ${measurement.unit}` : "?");

function roomDetail(room: Room) {
  const length = toMeters(room.length);
  const width = toMeters(room.width);
  return `${describe(room.length)} × ${describe(room.width)}${length && width ? ` · ${(length * width).toFixed(1)} m²` : ""}`;
}

export type PlacedOpening = ModelOpening & { center: number };
export type Wall = { x1: number; z1: number; x2: number; z2: number; height: number; approximate: boolean; openings: PlacedOpening[] };

// Walls are deduplicated; each room's openings go on its walls longest-first and are evenly spaced (indicative only).
export function buildWalls(rooms: ModelRoom[]): Wall[] {
  const walls = new Map<string, Wall & { pending: ModelOpening[] }>();
  for (const room of rooms) {
    const segments: [number, number, number, number][] = [
      [room.x, room.z, room.x + room.length, room.z],
      [room.x, room.z + room.width, room.x + room.length, room.z + room.width],
      [room.x, room.z, room.x, room.z + room.width],
      [room.x + room.length, room.z, room.x + room.length, room.z + room.width],
    ];
    const keys = segments.map((segment) => {
      const key = segment.map((value) => value.toFixed(3)).join(":");
      const existing = walls.get(key);
      if (existing) {
        existing.height = Math.max(existing.height, room.height);
        existing.approximate &&= room.approximate;
      } else {
        const [x1, z1, x2, z2] = segment;
        walls.set(key, { x1, z1, x2, z2, height: room.height, approximate: room.approximate, openings: [], pending: [] });
      }
      return key;
    });
    const byLength = [...keys].sort((left, right) => wallLength(walls.get(right)!) - wallLength(walls.get(left)!));
    room.openings.forEach((opening, index) => walls.get(byLength[index % byLength.length])!.pending.push(opening));
  }
  return [...walls.values()].map(({ pending, ...wall }) => {
    const length = wallLength(wall);
    const slot = length / (pending.length + 1);
    wall.openings = pending.map((opening, index) => ({
      ...opening,
      center: slot * (index + 1),
      width: Math.min(opening.width, slot * 0.9),
      height: Math.max(0, Math.min(opening.height, wall.height - opening.sill - 0.05)),
    })).filter((opening) => opening.width > 0.05 && opening.height > 0.05);
    return wall;
  });
}

export const wallLength = (wall: { x1: number; z1: number; x2: number; z2: number }) => Math.hypot(wall.x2 - wall.x1, wall.z2 - wall.z1);

// Footprints follow the AI-extracted drawing bounds; one scale is fitted from confirmed room areas.
export function buildModel(rooms: Room[], drawingAspect?: number, openings: { doors?: Opening[]; windows?: Opening[] } = {}): Model {
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
      openings: roomOpenings(room.id, openings),
      detail: roomDetail(room),
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
    result.push({ id: room.id, name: room.name, x: cursorX, z: cursorZ, length, width, height: toMeters(room.wallHeight) ?? fallbackHeight, approximate: true, openings: roomOpenings(room.id, openings), detail: roomDetail(room) });
    approximate.push(room.name);
    cursorX += length;
    rowDepth = Math.max(rowDepth, width);
    maxX = Math.max(maxX, cursorX);
    maxZ = Math.max(maxZ, cursorZ + width);
  }
  return { rooms: result, approximate, skipped, extent: { width: maxX, depth: maxZ } };
}