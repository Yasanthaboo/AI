"use client";

import { useMemo, useState } from "react";
import { describeMeasurement, formatNumber, jsonInit, request, type ArchitecturalFeature, type Bounds, type Candidate, type Confirmation, type Measurement, type Opening } from "./api";
import AnalysisDetails from "./analysis-details";
import DrawingView from "./drawing-view";
import { describeDefaultHeight, wallHeightIn, type Settings } from "./settings";

type EditableRoom = { id: string; name: string; length: string; width: string; wallHeight: string; unit: string; bounds?: Bounds | null; flag: string | null; defaultHeight?: boolean };
type EditableOpening = { id: string; type: "Door" | "Window"; roomId: string; width: string; height: string; unit: string };

const units = [["m", "m"], ["cm", "cm"], ["ft", "ft"], ["in", "in"]];
const text = (value?: number) => (value && value > 0 ? String(value) : "");
const positive = (value: string) => Number.isFinite(Number(value)) && Number(value) > 0;
const metreFactors: Record<string, number> = { m: 1, cm: 0.01, ft: 0.3048, in: 0.0254 };
const measurementText = (measurement: Measurement | null | undefined, unit: string) => measurement && metreFactors[measurement.unit] && metreFactors[unit] ? text(measurement.value * metreFactors[measurement.unit] / metreFactors[unit]) : "";

function toOpenings(candidate: Candidate): EditableOpening[] {
  const map = (type: EditableOpening["type"]) => (opening: Opening): EditableOpening => ({
    id: opening.id, type, roomId: opening.roomId || "",
    width: measurementText(opening.width, opening.width?.unit || "m"), height: measurementText(opening.height, opening.width?.unit || "m"), unit: opening.width?.unit || "m",
  });
  return [...(candidate.doors || []).map(map("Door")), ...(candidate.windows || []).map(map("Window"))];
}

export default function ReviewStep({ analysisId, candidate, analysis, settings, engines, floorPlanId, file, lastConfirmed, onRerun, onConfirmed, onBack }: {
  analysisId: string;
  candidate: Candidate;
  analysis: Candidate;
  settings: Settings;
  engines: string[];
  floorPlanId?: string;
  file?: File | null;
  lastConfirmed?: Confirmation | null;
  onRerun: (analysisId: string, candidate: Candidate) => void;
  onConfirmed: (confirmation: Confirmation) => void;
  onBack: () => void;
}) {
  const defaultLabel = `Default (${describeDefaultHeight(settings)})`;
  const [rooms, setRooms] = useState<EditableRoom[]>(() => (candidate.rooms || []).map((room, index) => {
    const unit = room.length?.unit || room.width?.unit || settings.preferredUnit;
    const wallHeight = measurementText(room.wallHeight, unit);
    return {
      id: room.id || `room-${index + 1}`, name: room.name, length: measurementText(room.length, unit), width: measurementText(room.width, unit),
      wallHeight: wallHeight || wallHeightIn(settings, unit), defaultHeight: !wallHeight || !!room.assumptions?.includes("wallHeight"), unit, bounds: room.bounds,
      flag: room.uncertainty || ((room.confidence ?? 1) < 1 ? "Low confidence" : null),
    };
  }));
  const [openings, setOpenings] = useState<EditableOpening[]>(() => toOpenings(candidate));
  const [features, setFeatures] = useState<ArchitecturalFeature[]>(() => candidate.features || []);
  const [selectedFeatureId, setSelectedFeatureId] = useState("");
  const [selectedId, setSelectedId] = useState(candidate.rooms?.[0]?.id || "");
  const [selectedOpeningId, setSelectedOpeningId] = useState("");
  const selectedRoom = rooms.find((room) => room.id === selectedId);
  const originalRoom = analysis.rooms?.find((room) => room.id === selectedId);
  const confirmedRoom = lastConfirmed?.candidate.rooms.find((room) => room.id === selectedId);
  const selectedOpening = openings.find((opening) => opening.id === selectedOpeningId);
  const originalOpening = [...(analysis.doors || []), ...(analysis.windows || [])].find((opening) => opening.id === selectedOpeningId);
  const confirmedOpening = [...(lastConfirmed?.candidate.doors || []), ...(lastConfirmed?.candidate.windows || [])].find((opening) => opening.id === selectedOpeningId);
  const selectedFeature = features.find((feature) => feature.id === selectedFeatureId);
  const selectedRegion = selectedFeature ? selectedFeature.evidence?.region : selectedOpening ? originalOpening?.evidence?.width?.region || originalOpening?.evidence?.height?.region : originalRoom?.evidence?.length?.region || originalRoom?.evidence?.width?.region;
  const selectRoom = (id: string) => { setSelectedId(id); setSelectedOpeningId(""); setSelectedFeatureId(""); };
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
    for (const feature of features) {
      if (feature.disposition === "pending") list.push(`${feature.label || feature.type}: acknowledge or remove the feature.`);
      if (feature.disposition !== "removed" && !feature.label.trim()) list.push("Every retained feature needs a label.");
    }
    return list;
  }, [rooms, openings, features]);

  const editRoom = (id: string, patch: Partial<EditableRoom>) => setRooms((current) => current.map((room) => room.id === id ? { ...room, ...patch } : room));
  const editOpening = (id: string, patch: Partial<EditableOpening>) => setOpenings((current) => current.map((opening) => opening.id === id ? { ...opening, ...patch } : opening));
  const nextId = (prefix: string, ids: string[]) => { let index = ids.length + 1; while (ids.includes(`${prefix}-${index}`)) index += 1; return `${prefix}-${index}`; };
  const addRoom = () => setRooms((current) => [...current, { id: nextId("room", current.map((room) => room.id)), name: "New room", length: "", width: "", wallHeight: wallHeightIn(settings, current[0]?.unit || settings.preferredUnit), defaultHeight: true, unit: current[0]?.unit || settings.preferredUnit, flag: null }]);
  const addOpening = (type: EditableOpening["type"]) => setOpenings((current) => [...current, { id: nextId(type.toLowerCase(), current.map((opening) => opening.id)), type, roomId: "", width: "", height: "", unit: "m" }]);

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
      features,
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

    <div className="review-preview-layout">
      <div className="review-drawing">
        <h3 className="section-title">Dimension preview</h3>
        <DrawingView file={file} analysisId={analysisId} source={analysis.source} rooms={(candidate.rooms || []).map((room) => ({ ...room, name: rooms.find((editable) => editable.id === room.id)?.name || room.name }))} selectedId={selectedId} onSelect={selectRoom} highlight={selectedRegion} />
      </div>
      <div className="dimension-comparison">
        <h3 className="section-title">{selectedFeature ? selectedFeature.label || selectedFeature.type : selectedOpening ? `${selectedOpening.type} ${selectedOpening.id}` : selectedRoom?.name || "Room dimensions"}</h3>
        <span className="badge badge--warning">Unconfirmed review</span>
        {lastConfirmed && <p className="cell-note">Last confirmed revision {lastConfirmed.revision}</p>}
        {selectedFeature ? <p className="cell-note">{selectedFeature.evidence?.observedText || "No source text available"} · {selectedFeature.evidence?.region ? "Source region highlighted" : "Source region unavailable"} · informational only</p> : selectedOpening ? <dl>{(["width", "height"] as const).map((field) => <div key={field}>
          <dt>{field === "width" ? "Width" : "Height"}</dt>
          <dd className="cell-note">Original: {describeMeasurement(originalOpening?.[field])}</dd>
          <dd>Pending: {selectedOpening[field] || "Missing"} {selectedOpening.unit}{positive(selectedOpening[field]) && metreFactors[selectedOpening.unit] ? ` (${formatNumber(Number(selectedOpening[field]) * metreFactors[selectedOpening.unit], 4)} m)` : ""}</dd>
          {confirmedOpening && <dd className="cell-note">Confirmed: {describeMeasurement(confirmedOpening[field])}</dd>}
        </div>)}</dl> : selectedRoom ? <dl>
          {(["length", "width", "wallHeight"] as const).map((field) => <div key={field}>
            <dt>{field === "wallHeight" ? "Wall height" : field === "length" ? "Length" : "Width"}</dt>
            <dd><span className="cell-note">Original: {describeMeasurement(originalRoom?.[field])}</span></dd>
            <dd>Pending: {selectedRoom[field] || "Missing"} {selectedRoom.unit}{positive(selectedRoom[field]) && metreFactors[selectedRoom.unit] ? ` (${formatNumber(Number(selectedRoom[field]) * metreFactors[selectedRoom.unit], 4)} m)` : ""}</dd>
            {confirmedRoom && <dd className="cell-note">Confirmed: {describeMeasurement(confirmedRoom[field])}</dd>}
            {field === "wallHeight" && selectedRoom.defaultHeight && <dd className="cell-note">{defaultLabel}</dd>}
          </div>)}
        </dl> : <p className="empty-note">Select a room to compare dimensions.</p>}
        <p className="cell-note">{selectedOpening ? selectedRegion ? "Opening evidence region highlighted; wall placement remains indicative." : "Source region not available for this opening." : selectedRoom?.bounds ? "Room boundary is AI-estimated; dimension labels remain subject to review." : "Source region not available for this room."}</p>
      </div>
    </div>

    <div className="table-wrap">
      <table className="data-table">
        <thead><tr><th>Room</th><th>Length</th><th>Width</th><th>Wall height</th><th>Unit</th><th><span className="sr-only">Actions</span></th></tr></thead>
        <tbody>
          {rooms.map((room) => <tr key={room.id} className={[room.flag ? "row--flagged" : "", selectedId === room.id ? "row--selected" : ""].join(" ")} onFocus={() => selectRoom(room.id)}>
            <td><input aria-label="Room name" value={room.name} onChange={(event) => editRoom(room.id, { name: event.target.value })} />
              {room.flag && <div className="row-flag"><span>⚠ {room.flag}</span><button type="button" className="link-button" onClick={() => editRoom(room.id, { flag: null })}>Looks right</button></div>}</td>
            {(["length", "width", "wallHeight"] as const).map((field) => <td key={field}><input aria-label={`${room.name} ${field === "wallHeight" ? "wall height" : field}`} type="number" min="0" step="0.01" inputMode="decimal" value={room[field]} className={positive(room[field]) ? undefined : "input--invalid"} onChange={(event) => editRoom(room.id, { [field]: event.target.value, ...(field === "wallHeight" ? { defaultHeight: false } : {}) })} />
              {field === "wallHeight" && room.defaultHeight && <div className="cell-note">{defaultLabel}</div>}</td>)}
            <td><select aria-label={`${room.name} unit`} value={room.unit} onChange={(event) => { const unit = event.target.value; const convert = (value: string) => positive(value) && metreFactors[room.unit] ? String(Number(value) * metreFactors[room.unit] / metreFactors[unit]) : value; editRoom(room.id, { unit, length: convert(room.length), width: convert(room.width), wallHeight: room.defaultHeight ? wallHeightIn(settings, unit) : convert(room.wallHeight) }); }}>{units.map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></td>
            <td><button type="button" className="icon-button" aria-label={`Remove ${room.name}`} title="Remove room" onClick={() => { setRooms((current) => current.filter((item) => item.id !== room.id)); setOpenings((current) => current.filter((item) => item.roomId !== room.id)); setFeatures((current) => current.map((feature) => feature.roomId === room.id ? { ...feature, roomId: undefined } : feature)); }}>✕</button></td>
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
            {openings.map((opening) => <tr key={opening.id} onFocus={() => { setSelectedId(opening.roomId); setSelectedOpeningId(opening.id); }}>
              <td><span className={`badge ${opening.type === "Door" ? "badge--door" : "badge--window"}`}>{opening.type}</span></td>
              <td><select aria-label={`${opening.id} room`} value={opening.roomId} onChange={(event) => editOpening(opening.id, { roomId: event.target.value })}>{!rooms.some((room) => room.id === opening.roomId) && <option value="">Choose room</option>}{rooms.map((room) => <option key={room.id} value={room.id}>{room.name}</option>)}</select></td>
              <td><input aria-label={`${opening.type} width`} type="number" min="0" step="0.01" value={opening.width} onChange={(event) => editOpening(opening.id, { width: event.target.value })} /></td>
              <td><input aria-label={`${opening.type} height`} type="number" min="0" step="0.01" value={opening.height} onChange={(event) => editOpening(opening.id, { height: event.target.value })} /></td>
              <td><select aria-label={`${opening.id} unit`} value={opening.unit} onChange={(event) => { const unit = event.target.value; const convert = (value: string) => positive(value) && metreFactors[opening.unit] ? String(Number(value) * metreFactors[opening.unit] / metreFactors[unit]) : value; editOpening(opening.id, { unit, width: convert(opening.width), height: convert(opening.height) }); }}>{units.map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></td>
              <td><button type="button" className="icon-button" aria-label={`Remove ${opening.id}`} title="Remove opening" onClick={() => setOpenings((current) => current.filter((item) => item.id !== opening.id))}>✕</button></td>
            </tr>)}
          </tbody>
        </table>
      </div>}
    <div className="table-actions"><button type="button" className="button button--ghost button--small" onClick={() => addOpening("Door")}>+ Door</button><button type="button" className="button button--ghost button--small" onClick={() => addOpening("Window")}>+ Window</button></div>

    <h3 className="section-title">Stairs and balconies</h3>
    <p className="cell-note">Informational only. Excluded from structural and external-work quantities.</p>
    {features.length > 0 && <div className="table-wrap"><table className="data-table"><thead><tr><th>Type / origin</th><th>Label</th><th>Room</th><th>Review</th><th>Source</th></tr></thead><tbody>{features.map((feature) => <tr key={feature.id}>
      <td>{feature.type} · {feature.origin || "unknown"}{feature.uncertainty && <p className="cell-note">{feature.uncertainty}</p>}</td>
      <td><input aria-label={`${feature.id} label`} value={feature.label} onChange={(event) => setFeatures((current) => current.map((entry) => entry.id === feature.id ? { ...entry, label: event.target.value } : entry))} /><p className="cell-note">Original: {analysis.features?.find((entry) => entry.id === feature.id)?.label || "Manual addition"}</p></td>
      <td><select aria-label={`${feature.id} room`} value={feature.roomId || ""} onChange={(event) => setFeatures((current) => current.map((entry) => entry.id === feature.id ? { ...entry, roomId: event.target.value || undefined } : entry))}><option value="">Unassociated</option>{rooms.map((room) => <option key={room.id} value={room.id}>{room.name}</option>)}</select></td>
      <td><select aria-label={`${feature.id} disposition`} value={feature.disposition} onChange={(event) => setFeatures((current) => current.map((entry) => entry.id === feature.id ? { ...entry, disposition: event.target.value as ArchitecturalFeature["disposition"] } : entry))}><option value="pending">Pending review</option><option value="acknowledged">Acknowledged</option><option value="removed">Removed</option></select></td>
      <td><button type="button" className="button button--ghost button--small" onClick={() => { setSelectedFeatureId(feature.id); setSelectedOpeningId(""); }}>Show source</button></td>
    </tr>)}</tbody></table></div>}
    <div className="table-actions">{(["stair", "balcony"] as const).map((type) => <button type="button" key={type} className="button button--ghost button--small" onClick={() => setFeatures((current) => [...current, { id: nextId(type, [...rooms.map((room) => room.id), ...openings.map((opening) => opening.id), ...current.map((feature) => feature.id)]), type, label: type === "stair" ? "Manual stair" : "Manual balcony", origin: "manual", disposition: "pending" }])}>+ {type === "stair" ? "Stair" : "Balcony"}</button>)}</div>
    {!!lastConfirmed?.candidate.corrections?.length && <details><summary>Saved correction audit · revision {lastConfirmed.revision}</summary><div className="table-wrap"><table className="data-table"><thead><tr><th>Entity / field</th><th>Original</th><th>Confirmed</th><th>Origin</th></tr></thead><tbody>{lastConfirmed.candidate.corrections.map((correction, index) => <tr key={index}><td>{correction.entityId} · {correction.field}</td><td className="material-audit-value">{JSON.stringify(correction.originalValue)}</td><td className="material-audit-value">{JSON.stringify(correction.confirmedValue)}</td><td>{correction.origin}</td></tr>)}</tbody></table></div></details>}

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
