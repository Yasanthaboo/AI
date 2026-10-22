import type { Confirmation } from "./api";
import { buildModel, toMeters } from "./preview";

export type RoofForm = "flat" | "gable" | "hip" | "shed";
export type ExteriorInput = { analysisId: string; confirmationRevision: number; roof: RoofForm; pitchDegrees: number; overhangM: number; lengthM: number; widthM: number; wallHeightM: number; finish: "metal" | "tile" | "membrane"; assumptionsReviewed: boolean };
export type ExteriorOption = ExteriorInput & { id: string; projectId: string; assumptions: string; createdAt: string };
export const exteriorAssumptions = "Conceptual rectangular single-storey envelope. Dimensions, wall height and roof geometry are visualization assumptions, not verified elevations. Openings, additional levels, drainage, structure and code compliance are not modeled. Roof finish is appearance only. Estimates are unchanged.";

export function suggestExterior(confirmation: Confirmation): ExteriorInput {
  const rooms = confirmation.candidate.rooms ?? [];
  const model = buildModel(rooms, confirmation.candidate.drawingAspect);
  const heights = rooms.map((room) => toMeters(room.wallHeight)).filter((value): value is number => value !== null);
  const minimumX = model.rooms.length ? Math.min(...model.rooms.map((room) => room.x)) : 0;
  const minimumZ = model.rooms.length ? Math.min(...model.rooms.map((room) => room.z)) : 0;
  const maximumX = model.rooms.length ? Math.max(...model.rooms.map((room) => room.x + room.length)) : 0;
  const maximumZ = model.rooms.length ? Math.max(...model.rooms.map((room) => room.z + room.width)) : 0;
  return { analysisId: confirmation.analysisId, confirmationRevision: confirmation.revision, roof: "gable", pitchDegrees: 25, overhangM: 0.4, lengthM: Number((maximumX - minimumX).toFixed(2)), widthM: Number((maximumZ - minimumZ).toFixed(2)), wallHeightM: heights.length ? Number(Math.max(...heights).toFixed(2)) : 0, finish: "metal", assumptionsReviewed: false };
}

export function validExterior(input: ExteriorInput) {
  return [input.lengthM, input.widthM, input.wallHeightM, input.overhangM, input.pitchDegrees].every(Number.isFinite)
    && input.lengthM >= 1 && input.lengthM <= 100 && input.widthM >= 1 && input.widthM <= 100 && input.wallHeightM >= 1 && input.wallHeightM <= 12
    && input.overhangM >= 0 && input.overhangM <= 2 && (input.roof === "flat" ? input.pitchDegrees === 0 : input.pitchDegrees >= 5 && input.pitchDegrees <= 60);
}

type Point = [number, number, number];
export function roofMeshData(input: ExteriorInput) {
  if (!validExterior(input)) throw new Error("Exterior dimensions are outside supported bounds.");
  const roof: number[] = [];
  const infill: number[] = [];
  const halfLength = input.lengthM / 2 + input.overhangM;
  const halfWidth = input.widthM / 2 + input.overhangM;
  const eave = input.wallHeightM;
  const slope = Math.tan(input.pitchDegrees * Math.PI / 180);
  const triangle = (target: number[], first: Point, second: Point, third: Point) => target.push(...first, ...second, ...third);
  const quad = (target: number[], first: Point, second: Point, third: Point, fourth: Point) => { triangle(target, first, second, third); triangle(target, first, third, fourth); };
  const frontLeft: Point = [-halfLength, eave, -halfWidth];
  const frontRight: Point = [halfLength, eave, -halfWidth];
  const backRight: Point = [halfLength, eave, halfWidth];
  const backLeft: Point = [-halfLength, eave, halfWidth];
  let rise = 0;
  if (input.roof === "flat") {
    quad(roof, frontLeft, frontRight, backRight, backLeft);
  } else if (input.roof === "gable") {
    rise = halfWidth * slope;
    const ridgeLeft: Point = [-halfLength, eave + rise, 0];
    const ridgeRight: Point = [halfLength, eave + rise, 0];
    quad(roof, frontLeft, frontRight, ridgeRight, ridgeLeft);
    quad(roof, ridgeLeft, ridgeRight, backRight, backLeft);
    triangle(infill, frontLeft, ridgeLeft, backLeft);
    triangle(infill, frontRight, backRight, ridgeRight);
  } else if (input.roof === "shed") {
    rise = halfWidth * 2 * slope;
    const highLeft: Point = [-halfLength, eave + rise, halfWidth];
    const highRight: Point = [halfLength, eave + rise, halfWidth];
    quad(roof, frontLeft, frontRight, highRight, highLeft);
    const wallHalfLength = input.lengthM / 2;
    const wallHalfWidth = input.widthM / 2;
    const frontHeight = eave + input.overhangM * slope;
    const backHeight = eave + (input.widthM + input.overhangM) * slope;
    quad(infill, [-wallHalfLength, eave, -wallHalfWidth], [-wallHalfLength, frontHeight, -wallHalfWidth], [-wallHalfLength, backHeight, wallHalfWidth], [-wallHalfLength, eave, wallHalfWidth]);
    quad(infill, [wallHalfLength, eave, -wallHalfWidth], [wallHalfLength, eave, wallHalfWidth], [wallHalfLength, backHeight, wallHalfWidth], [wallHalfLength, frontHeight, -wallHalfWidth]);
    quad(infill, [-wallHalfLength, eave, wallHalfWidth], [-wallHalfLength, backHeight, wallHalfWidth], [wallHalfLength, backHeight, wallHalfWidth], [wallHalfLength, eave, wallHalfWidth]);
    quad(infill, [-wallHalfLength, eave, -wallHalfWidth], [wallHalfLength, eave, -wallHalfWidth], [wallHalfLength, frontHeight, -wallHalfWidth], [-wallHalfLength, frontHeight, -wallHalfWidth]);
  } else {
    const longHalf = Math.max(halfLength, halfWidth);
    const shortHalf = Math.min(halfLength, halfWidth);
    rise = shortHalf * slope;
    const orient = (point: Point): Point => halfLength >= halfWidth ? point : [point[2], point[1], point[0]];
    const cornerFrontLeft = orient([-longHalf, eave, -shortHalf]);
    const cornerFrontRight = orient([longHalf, eave, -shortHalf]);
    const cornerBackRight = orient([longHalf, eave, shortHalf]);
    const cornerBackLeft = orient([-longHalf, eave, shortHalf]);
    const ridgeLeft = orient([-(longHalf - shortHalf), eave + rise, 0]);
    const ridgeRight = orient([longHalf - shortHalf, eave + rise, 0]);
    quad(roof, cornerFrontLeft, cornerFrontRight, ridgeRight, ridgeLeft);
    quad(roof, ridgeLeft, ridgeRight, cornerBackRight, cornerBackLeft);
    triangle(roof, cornerFrontLeft, ridgeLeft, cornerBackLeft);
    triangle(roof, cornerFrontRight, cornerBackRight, ridgeRight);
  }
  return { roof, infill, rise };
}