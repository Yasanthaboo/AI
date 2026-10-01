"use client";

import { useMemo, useState } from "react";
import { jsonInit, request, type Bounds, type Candidate, type Confirmation, type Opening } from "./api";
import AnalysisDetails from "./analysis-details";
import { describeDefaultHeight, wallHeightIn, type Settings } from "./settings";

type EditableRoom = { id: string; name: string; length: string; width: string; wallHeight: string; unit: string; bounds?: Bounds | null; flag: string | null; defaultHeight?: boolean };
type EditableOpening = { id: string; type: "Door" | "Window"; roomId: string; width: string; height: string; unit: string };

const units = [["m", "m"], ["cm", "cm"], ["ft", "ft"], ["in", "in"]];
const text = (value?: number) => (value && value > 0 ? String(value) : "");
const positive = (value: string) => Number.isFinite(Number(value)) && Number(value) > 0;

function toOpenings(candidate: Candidate, fallbackRoom: string): EditableOpening[] {
  const map = (type: EditableOpening["type"]) => (opening: Opening): EditableOpening => ({
    id: opening.id, type, roomId: opening.roomId || fallbackRoom,
    width: text(opening.width?.value), height: text(opening.height?.value), unit: opening.width?.unit || "m",
  });
  return [...(candidate.doors || []).map(map("Door")), ...(candidate.windows || []).map(map("Window"))];
}

export default function ReviewStep({ analysisId, candidate, analysis, settings, engines, floorPlanId, onRerun, onConfirmed, onBack }: {
  analysisId: string;
  candidate: Candidate;
  analysis: Candidate;
  settings: Settings;
  engines: string[];
  floorPlanId?: string;
  onRerun: (analysisId: string, candidate: Candidate) => void;
  onConfirmed: (confirmation: Confirmation) => void;
  onBack: () => void;
}) {
  const defaultLabel = `Default (${describeDefaultHeight(settings)})`;
  const [rooms, setRooms] = useState<EditableRoom[]>(() => (candidate.rooms || []).map((room, index) => {
    const unit = room.length?.unit || room.width?.unit || settings.preferredUnit;
    const wallHeight = text(room.wallHeight?.value);
    return {
      id: room.id || `room-${index + 1}`, name: room.name, length: text(room.length?.value), width: text(room.width?.value),
      wallHeight: wallHeight || wallHeightIn(settings, unit), defaultHeight: !wallHeight || !!room.assumptions?.includes("wallHeight"), unit, bounds: room.bounds,
      flag: room.uncertainty || ((room.confidence ?? 1) < 1 ? "Low confidence" : null),
    };
  }));
  const [openings, setOpenings] = useState<EditableOpening[]>(() => toOpenings(candidate, candidate.rooms?.[0]?.id || "room-1"));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const problems = useMemo(() => {
    const list: string[] = [];
    if (!rooms.length) list.push("Add at least one room.");
    for (const room of rooms) {
      if (!room.name.trim()) list.push("Every room needs a name.");
      const missing = [["length", room.length], ["width", room.width], ["wall height", room.wallHeight]].filter(([, value]) => !positive(value)).map(([label]) => label);
      if (missing.length) list.push(`${room.name || "Unnamed room"}: enter ${missing.join(", ")}.`);
      if (room.flag) list.push(`${room.name}: check the flagged values.`);
    }
    for (const opening of openings) {
      if (!rooms.some((room) => room.id === opening.roomId)) list.push(`${opening.type} ${opening.id}: choose a room.`);
      if (!positive(opening.width) || !positive(opening.height)) list.push(`${opening.type} ${opening.id}: enter width and height.`);
    }
    return list;
  }, [rooms, openings]);

  const editRoom = (id: string, patch: Partial<EditableRoom>) => setRooms((current) => current.map((room) => room.id === id ? { ...room, ...patch } : room));
  const editOpening = (id: string, patch: Partial<EditableOpening>) => setOpenings((current) => current.map((opening) => opening.id === id ? { ...opening, ...patch } : opening));
  const nextId = (prefix: string, ids: string[]) => { let index = ids.length + 1; while (ids.includes(`${prefix}-${index}`)) index += 1; return `${prefix}-${index}`; };
  const addRoom = () => setRooms((current) => [...current, { id: nextId("room", current.map((room) => room.id)), name: "New room", length: "", width: "", wallHeight: wallHeightIn(settings, current[0]?.unit || settings.preferredUnit), defaultHeight: true, unit: current[0]?.unit || settings.preferredUnit, flag: null }]);
  const addOpening = (type: EditableOpening["type"]) => setOpenings((current) => [...current, { id: nextId(type.toLowerCase(), current.map((opening) => opening.id)), type, roomId: rooms[0]?.id || "", width: type === "Door" ? "0.9" : "1.2", height: type === "Door" ? "2.1" : "1.2", unit: "m" }]);

  const confirm = async () => {
    if (problems.length) { setError(problems[0]); return; }
    setBusy(true);
    setError("");
    const measure = (value: string, unit: string) => ({ value: Number(value), unit });
    const toApi = (opening: EditableOpening) => ({ id: opening.id, roomId: opening.roomId, width: measure(opening.width, opening.unit), height: measure(opening.height, opening.unit), confidence: 1 });
    const payload = {
      ...candidate,
      rooms: rooms.map((room) => ({ id: room.id, name: room.name.trim(), length: measure(room.length, room.unit), width: measure(room.width, room.unit), wallHeight: measure(room.wallHeight, room.unit), bounds: room.bounds ?? undefined, confidence: 1, assumptions: room.defaultHeight ? ["wallHeight"] : undefined })),
      doors: openings.filter((opening) => opening.type === "Door").map(toApi),
      windows: openings.filter((opening) => opening.type === "Window").map(toApi),
    };
    try {
      const confirmed = await request<Candidate & { confirmationRevision: number }>(`/api/analyses/${analysisId}/confirmed-data`, jsonInit("PUT", payload));
      onConfirmed({ analysisId, revision: confirmed.confirmationRevision, candidate: { ...confirmed, rooms: confirmed.rooms || [] } });
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "The data could not be confirmed.");
    } finally {
      setBusy(false);
    }
  };

  const flagged = rooms.filter((room) => room.flag).length;
  return <section className="card step-card">
    <header className="step-header">
      <div><p className="eyebrow">Step 3</p><h2>Review the extraction</h2><p className="muted">Check what the AI read from the drawing. Quantities are calculated only from values you confirm here.</p></div>
      <div className="stat-chips"><span className="badge">{rooms.length} rooms</span><span className="badge">{openings.length} openings</span>{flagged > 0 && <span className="badge badge--warning">{flagged} to check</span>}</div>
    </header>
    <AnalysisDetails candidate={analysis} engines={engines} floorPlanId={floorPlanId} onRerun={onRerun} compact />

    <div className="table-wrap">
      <table className="data-table">
        <thead><tr><th>Room</th><th>Length</th><th>Width</th><th>Wall height</th><th>Unit</th><th><span className="sr-only">Actions</span></th></tr></thead>
        <tbody>
          {rooms.map((room) => <tr key={room.id} className={room.flag ? "row--flagged" : undefined}>
            <td><input aria-label="Room name" value={room.name} onChange={(event) => editRoom(room.id, { name: event.target.value })} />
              {room.flag && <div className="row-flag"><span>⚠ {room.flag}</span><button type="button" className="link-button" onClick={() => editRoom(room.id, { flag: null })}>Looks right</button></div>}</td>
            {(["length", "width", "wallHeight"] as const).map((field) => <td key={field}><input aria-label={`${room.name} ${field === "wallHeight" ? "wall height" : field}`} type="number" min="0" step="0.01" inputMode="decimal" value={room[field]} className={positive(room[field]) ? undefined : "input--invalid"} onChange={(event) => editRoom(room.id, { [field]: event.target.value, ...(field === "wallHeight" ? { defaultHeight: false } : {}) })} />
              {field === "wallHeight" && room.defaultHeight && <div className="cell-note">{defaultLabel}</div>}</td>)}
            <td><select aria-label={`${room.name} unit`} value={room.unit} onChange={(event) => editRoom(room.id, { unit: event.target.value, ...(room.defaultHeight ? { wallHeight: wallHeightIn(settings, event.target.value) } : {}) })}>{units.map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></td>
            <td><button type="button" className="icon-button" aria-label={`Remove ${room.name}`} title="Remove room" onClick={() => { setRooms((current) => current.filter((item) => item.id !== room.id)); setOpenings((current) => current.filter((item) => item.roomId !== room.id)); }}>✕</button></td>
          </tr>)}
        </tbody>
      </table>
    </div>
    <div className="table-actions"><button type="button" className="button button--ghost button--small" onClick={addRoom}>+ Add room</button></div>

    <h3 className="section-title">Doors and windows</h3>
    {openings.length === 0 ? <p className="empty-note">No openings were detected. Add any doors or windows that affect wall areas.</p> :
      <div className="table-wrap">
        <table className="data-table">
          <thead><tr><th>Type</th><th>Room</th><th>Width</th><th>Height</th><th>Unit</th><th><span className="sr-only">Actions</span></th></tr></thead>
          <tbody>
            {openings.map((opening) => <tr key={opening.id}>
              <td><span className={`badge ${opening.type === "Door" ? "badge--door" : "badge--window"}`}>{opening.type}</span></td>
              <td><select aria-label={`${opening.id} room`} value={opening.roomId} onChange={(event) => editOpening(opening.id, { roomId: event.target.value })}>{!rooms.some((room) => room.id === opening.roomId) && <option value="">Choose room</option>}{rooms.map((room) => <option key={room.id} value={room.id}>{room.name}</option>)}</select></td>
              <td><input aria-label={`${opening.type} width`} type="number" min="0" step="0.01" value={opening.width} onChange={(event) => editOpening(opening.id, { width: event.target.value })} /></td>
              <td><input aria-label={`${opening.type} height`} type="number" min="0" step="0.01" value={opening.height} onChange={(event) => editOpening(opening.id, { height: event.target.value })} /></td>
              <td><select aria-label={`${opening.id} unit`} value={opening.unit} onChange={(event) => editOpening(opening.id, { unit: event.target.value })}>{units.map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></td>
              <td><button type="button" className="icon-button" aria-label={`Remove ${opening.id}`} title="Remove opening" onClick={() => setOpenings((current) => current.filter((item) => item.id !== opening.id))}>✕</button></td>
            </tr>)}
          </tbody>
        </table>
      </div>}
    <div className="table-actions"><button type="button" className="button button--ghost button--small" onClick={() => addOpening("Door")}>+ Door</button><button type="button" className="button button--ghost button--small" onClick={() => addOpening("Window")}>+ Window</button></div>

    {error && <p className="alert alert--error" role="alert">{error}</p>}
    <footer className="step-footer">
      <button type="button" className="button button--ghost" onClick={onBack} disabled={busy}>Back</button>
      <div className="step-footer__end">
        {problems.length > 0 && <span className="muted">{problems.length === 1 ? "1 item needs" : `${problems.length} items need`} attention</span>}
        <button type="button" className="button button--primary" onClick={confirm} disabled={busy || problems.length > 0}>{busy && <span className="spinner" aria-hidden />}{busy ? "Confirming…" : "Confirm and build model"}</button>
      </div>
    </footer>
  </section>;
}
