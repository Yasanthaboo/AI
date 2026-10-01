"use client";

import { engineLabels, formatDate, formatMoney, type Candidate, type Confirmation, type EstimateVersion, type FloorPlan, type Project } from "./api";
import AnalysisDetails from "./analysis-details";
import ConfirmationHistory, { useConfirmations } from "./confirmation-history";
import DrawingView from "./drawing-view";

type Event = { at: string; label: string; detail?: string };

export default function Overview(props: {
  project: Project;
  floorPlan: FloorPlan | null;
  file: File | null;
  analysisId: string;
  candidate: Candidate | null;
  confirmation: Confirmation | null;
  estimates: EstimateVersion[];
  engines: string[];
  onRerun: (analysisId: string, candidate: Candidate) => void;
  onContinue: () => void;
  continueLabel: string;
}) {
  const { project, floorPlan, file, analysisId, candidate, confirmation, estimates } = props;
  const history = useConfirmations(analysisId, confirmation?.revision);
  const source = confirmation?.candidate ?? candidate;
  const rooms = source?.rooms ?? [];
  const openings = (source?.doors?.length ?? 0) + (source?.windows?.length ?? 0);
  const latest = estimates[0];

  const events: Event[] = [{ at: project.createdAt, label: "Project created" }];
  if (floorPlan?.uploadedAt) events.push({ at: floorPlan.uploadedAt, label: "Drawing uploaded", detail: floorPlan.fileName });
  if (candidate?.startedAt) events.push({ at: candidate.startedAt, label: "Analysis started", detail: engineLabels[candidate.engine ?? ""] ?? candidate.engine });
  if (candidate?.completedAt) events.push({ at: candidate.completedAt, label: "Analysis completed", detail: `${candidate.rooms?.length ?? 0} rooms found` });
  for (const item of history ?? []) if (item.confirmedAt && new Date(item.confirmedAt).getFullYear() > 2000) events.push({ at: item.confirmedAt, label: `Dimensions confirmed (revision ${item.revision})`, detail: `${item.candidate.rooms.length} rooms` });
  for (const version of estimates) events.push({ at: version.calculatedAt, label: `Estimate v${version.version} calculated`, detail: formatMoney(version.convertedGrandTotal, version.targetCurrency) });
  events.sort((left, right) => new Date(right.at).getTime() - new Date(left.at).getTime());

  return <section className="card step-card">
    <header className="step-header">
      <div><p className="eyebrow">Overview</p><h2>{project.name}</h2><p className="muted">{[project.client, project.location].filter(Boolean).join(" · ") || "No client or location recorded"}</p></div>
      <button type="button" className="button button--primary" onClick={props.onContinue}>{props.continueLabel}</button>
    </header>

    <div className="metric-cards">
      <div className="metric-card"><span>Rooms</span><strong>{rooms.length || "—"}</strong><small>{confirmation ? `Confirmed revision ${confirmation.revision}` : candidate ? "Awaiting confirmation" : "Not analyzed"}</small></div>
      <div className="metric-card"><span>Openings</span><strong>{source ? openings : "—"}</strong><small>Doors and windows</small></div>
      <div className="metric-card metric-card--total"><span>Latest estimate</span><strong>{latest ? formatMoney(latest.convertedGrandTotal, latest.targetCurrency) : "—"}</strong><small>{latest ? `v${latest.version} of ${estimates.length}` : "No estimate yet"}</small></div>
    </div>

    <div className="overview-grid">
      <div>
        <h3 className="section-title">Details</h3>
        <dl className="summary-grid">
          <div><dt>Client</dt><dd>{project.client || "—"}</dd></div>
          <div><dt>Site location</dt><dd>{project.location || "—"}</dd></div>
          <div><dt>Created</dt><dd>{formatDate(project.createdAt)}</dd></div>
          <div><dt>Drawing</dt><dd>{floorPlan?.fileName ?? "Not uploaded"}</dd></div>
          {project.description && <div className="summary-grid__wide"><dt>Notes</dt><dd>{project.description}</dd></div>}
        </dl>
        {candidate && <AnalysisDetails candidate={candidate} engines={props.engines} floorPlanId={floorPlan?.id} onRerun={props.onRerun} compact />}
      </div>
      <figure className="model-panel overview-drawing">
        <figcaption>Drawing</figcaption>
        {file ? <DrawingView file={file} rooms={rooms} /> : <p className="empty-note">{floorPlan ? "Loading drawing…" : "No drawing uploaded yet."}</p>}
      </figure>
    </div>

    <div className="overview-grid">
      <div>
        <h3 className="section-title">Activity</h3>
        <ol className="timeline">{events.map((event, index) => <li key={`${event.label}-${index}`}><span className="timeline__dot" aria-hidden /><div><strong>{event.label}</strong>{event.detail && <span className="muted"> · {event.detail}</span>}<small className="muted block">{formatDate(event.at)}</small></div></li>)}</ol>
      </div>
      <div>
        <h3 className="section-title">Confirmation history</h3>
        {analysisId ? <ConfirmationHistory history={history} /> : <p className="empty-note">Analyze and confirm the drawing to start a history.</p>}
      </div>
    </div>
  </section>;
}
