"use client";

import dynamic from "next/dynamic";
import { useMemo } from "react";
import { describeMeasurement, type Confirmation } from "./api";
import DrawingView, { roomPalette } from "./drawing-view";
import { buildModel } from "./preview";

const RoomScene = dynamic(() => import("./room-scene"), { ssr: false, loading: () => <div className="drawing-loading"><span className="spinner" aria-hidden /> Building 3D model…</div> });

export default function ModelStep({ confirmation, file, onBack, onContinue }: { confirmation: Confirmation; file: File | null; onBack: () => void; onContinue: () => void }) {
  const rooms = confirmation.candidate.rooms;
  const model = useMemo(() => buildModel(rooms, confirmation.candidate.drawingAspect), [rooms, confirmation.candidate.drawingAspect]);
  const traced = rooms.filter((room) => room.bounds).length;
  const openings = [...(confirmation.candidate.doors || []), ...(confirmation.candidate.windows || [])];

  return <section className="card step-card">
    <header className="step-header">
      <div><p className="eyebrow">Step 4</p><h2>3D model</h2><p className="muted">Rooms are placed where the AI detected them on the drawing and scaled from your confirmed dimensions.</p></div>
      <div className="stat-chips"><span className="badge badge--info">AI-estimated layout</span><span className="badge">Revision {confirmation.revision}</span></div>
    </header>

    {traced === 0 ? <p className="alert alert--info">The AI did not return room boundaries for this drawing, so rooms are arranged approximately. Try the Gemini engine for a closer match.</p>
      : model.approximate.length > 0 && <p className="alert alert--info">Placed approximately (no boundary detected): {model.approximate.join(", ")}.</p>}
    {model.skipped.length > 0 && <p className="alert alert--warning">Not shown: {model.skipped.join(", ")} — missing dimensions.</p>}

    <div className="model-layout">
      <figure className="model-panel">
        <figcaption>Floor plan with detected rooms</figcaption>
        {file ? <DrawingView file={file} rooms={rooms} /> : <p className="empty-note">The original drawing is not stored in this browser. The model below still uses the saved room boundaries.</p>}
      </figure>
      <figure className="model-panel">
        <figcaption>3D model <span className="muted">· drag to orbit, scroll to zoom</span></figcaption>
        {model.rooms.length ? <RoomScene rooms={model.rooms} /> : <p className="empty-note">No rooms with dimensions to model.</p>}
      </figure>
    </div>

    <div className="table-wrap">
      <table className="data-table data-table--compact">
        <thead><tr><th>Room</th><th>Length</th><th>Width</th><th>Wall height</th><th>Placement</th></tr></thead>
        <tbody>{rooms.map((room, index) => <tr key={room.id}>
          <td><span className="swatch" style={{ background: roomPalette[index % roomPalette.length] }} aria-hidden />{room.name}</td>
          <td>{describeMeasurement(room.length)}</td><td>{describeMeasurement(room.width)}</td><td>{describeMeasurement(room.wallHeight)}</td>
          <td>{room.bounds ? <span className="badge badge--success">From drawing</span> : <span className="badge">Approximate</span>}</td>
        </tr>)}</tbody>
      </table>
    </div>
    {openings.length > 0 && <p className="muted small">{openings.length} door/window openings are included in the estimate deductions; their positions are not drawn in the model.</p>}
    <p className="muted small">The model is a proof-of-concept visual aid. Estimates always use the confirmed dimensions, never the model geometry.</p>

    <footer className="step-footer">
      <button type="button" className="button button--ghost" onClick={onBack}>Back to review</button>
      <button type="button" className="button button--primary" onClick={onContinue}>Continue to estimate</button>
    </footer>
  </section>;
}
