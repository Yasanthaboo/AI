"use client";

import { useEffect, useRef, useState } from "react";
import { engineLabels, formatDate, runAnalysis, type Candidate } from "./api";

function duration(candidate: Candidate) {
  if (!candidate.startedAt || !candidate.completedAt) return "—";
  const seconds = Math.max(0, (new Date(candidate.completedAt).getTime() - new Date(candidate.startedAt).getTime()) / 1000);
  return seconds < 60 ? `${seconds.toFixed(1)} s` : `${Math.floor(seconds / 60)} min ${Math.round(seconds % 60)} s`;
}

export function missingValues(candidate: Candidate) {
  const rooms = candidate.rooms || [];
  return { length: rooms.filter((room) => !room.length).length, width: rooms.filter((room) => !room.width).length, wallHeight: rooms.filter((room) => !room.wallHeight).length };
}

export default function AnalysisDetails({ candidate, engines, floorPlanId, onRerun, compact }: {
  candidate: Candidate;
  engines: string[];
  floorPlanId?: string;
  onRerun?: (analysisId: string, candidate: Candidate) => void;
  compact?: boolean;
}) {
  const others = engines.filter((engine) => engine !== candidate.engine);
  const [engine, setEngine] = useState(others[0] ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const cancelled = useRef(false);
  useEffect(() => { cancelled.current = false; return () => { cancelled.current = true; }; }, []);

  const rooms = candidate.rooms || [];
  const missing = missingValues(candidate);
  const missingTotal = missing.length + missing.width + missing.wallHeight;
  const flagged = rooms.filter((room) => (room.confidence ?? 1) < 1 || room.uncertainty).length;

  const rerun = async () => {
    if (!floorPlanId || !onRerun || !engine) return;
    if (!window.confirm(`Re-run the analysis with ${engineLabels[engine] ?? engine}? Unconfirmed edits, the current confirmation and estimates will be replaced in this workspace (earlier records stay on the server).`)) return;
    setBusy(true);
    setError("");
    try {
      const finished = await runAnalysis(floorPlanId, engine, () => cancelled.current);
      if (finished) onRerun(finished.id, finished.candidate);
    } catch (failure) {
      if (!cancelled.current) setError(failure instanceof Error ? failure.message : "The analysis failed.");
    } finally {
      if (!cancelled.current) setBusy(false);
    }
  };

  return <details className="info-panel" open={!compact}>
    <summary><strong>Analysis details</strong><span className="muted"> · {engineLabels[candidate.engine ?? ""] ?? (candidate.engine || "Default engine")}{candidate.model ? ` (${candidate.model})` : ""}</span></summary>
    <dl className="summary-grid summary-grid--four">
      <div><dt>Engine</dt><dd>{engineLabels[candidate.engine ?? ""] ?? (candidate.engine || "Default")}</dd></div>
      <div><dt>Model</dt><dd>{candidate.model || "—"}</dd></div>
      <div><dt>Completed</dt><dd>{formatDate(candidate.completedAt)}</dd></div>
      <div><dt>Duration</dt><dd>{duration(candidate)}</dd></div>
      <div><dt>Rooms / openings</dt><dd>{rooms.length} / {(candidate.doors?.length ?? 0) + (candidate.windows?.length ?? 0)}</dd></div>
      <div><dt>Low confidence</dt><dd>{flagged ? `${flagged} room${flagged === 1 ? "" : "s"}` : "None"}</dd></div>
      <div className="summary-grid__wide"><dt>Missing values</dt><dd>{missingTotal === 0 ? "None — every room has length, width and wall height" : [["length", missing.length], ["width", missing.width], ["wall height", missing.wallHeight]].filter(([, count]) => Number(count) > 0).map(([label, count]) => `${count} ${label}`).join(", ")}</dd></div>
    </dl>
    {rooms.length > 0 && <div className="confidence-list" aria-label="Confidence per room">
      {rooms.map((room) => {
        const confidence = Math.round((room.confidence ?? 1) * 100);
        return <div key={room.id} className="confidence-item">
          <span>{room.name}</span>
          <span className="meter" aria-hidden><span className={confidence < 80 ? "meter__fill meter__fill--low" : "meter__fill"} style={{ width: `${confidence}%` }} /></span>
          <span className="muted">{confidence}%</span>
        </div>;
      })}
    </div>}
    {onRerun && floorPlanId && others.length > 0 && <div className="rerun-row">
      <label className="field field--inline"><span>Re-run with</span>
        <select value={engine} onChange={(event) => setEngine(event.target.value)} disabled={busy} aria-label="Re-run engine">{others.map((key) => <option key={key} value={key}>{engineLabels[key] ?? key}</option>)}</select></label>
      <button type="button" className="button button--secondary button--small" onClick={rerun} disabled={busy}>{busy && <span className="spinner" aria-hidden />}{busy ? "Analyzing…" : "Re-run analysis"}</button>
    </div>}
    {error && <p className="alert alert--error" role="alert">{error}</p>}
  </details>;
}
