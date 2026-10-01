"use client";

import { useEffect, useMemo, useState } from "react";
import { engineLabels, formatMoney, jsonInit, request, type Project, type ProjectSummary } from "./api";

const stageTone: Record<ProjectSummary["stage"], string> = { Draft: "", Uploaded: "badge--info", Analyzed: "badge--info", Confirmed: "badge--warning", Estimated: "badge--success" };

export default function Dashboard({ onOpen, onNew }: { onOpen: (summary: ProjectSummary) => void; onNew: () => void }) {
  const [summaries, setSummaries] = useState<ProjectSummary[] | null>(null);
  const [query, setQuery] = useState("");
  const [showArchived, setShowArchived] = useState(false);
  const [busyId, setBusyId] = useState("");
  const [error, setError] = useState("");

  const load = () => request<ProjectSummary[]>("/api/projects").then((list) => { setSummaries(list); setError(""); }).catch((failure: Error) => { setSummaries([]); setError(failure.message); });
  useEffect(() => { void load(); }, []);

  const visible = useMemo(() => {
    const term = query.trim().toLowerCase();
    return (summaries ?? []).filter((summary) => (showArchived || !summary.project.archived)
      && (!term || [summary.project.name, summary.project.client, summary.project.location].some((value) => value?.toLowerCase().includes(term))));
  }, [summaries, query, showArchived]);
  const archivedCount = (summaries ?? []).filter((summary) => summary.project.archived).length;

  const act = async (id: string, action: () => Promise<unknown>) => {
    setBusyId(id);
    try {
      await action();
      await load();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "The action failed.");
    } finally {
      setBusyId("");
    }
  };

  return <section className="card step-card">
    <header className="step-header">
      <div><p className="eyebrow">Workspace</p><h2>Projects</h2><p className="muted">Every project, its stage, and its latest estimate.</p></div>
      <button type="button" className="button button--primary" onClick={onNew}>+ New project</button>
    </header>

    <div className="toolbar">
      <input type="search" className="toolbar__search" aria-label="Search projects" placeholder="Search by name, client, or location" value={query} onChange={(event) => setQuery(event.target.value)} />
      <label className="checkbox"><input type="checkbox" checked={showArchived} onChange={(event) => setShowArchived(event.target.checked)} /> Show archived{archivedCount ? ` (${archivedCount})` : ""}</label>
    </div>
    {error && <p className="alert alert--error" role="alert">{error}</p>}

    {summaries === null ? <div className="loading-card" role="status"><span className="spinner" aria-hidden /> Loading projects…</div>
      : visible.length === 0 ? <div className="empty-state"><strong>{summaries.length ? "No matching projects" : "No projects yet"}</strong><span className="muted">{summaries.length ? "Try another search or show archived projects." : "Create a project to upload and estimate a floor plan."}</span></div>
      : <div className="table-wrap">
        <table className="data-table project-table">
          <thead><tr><th>Project</th><th>Stage</th><th>Latest total</th><th>Updated</th><th><span className="sr-only">Actions</span></th></tr></thead>
          <tbody>{visible.map((summary) => {
            const project: Project = summary.project;
            return <tr key={project.id} className={project.archived ? "row--archived" : undefined}>
              <td><button type="button" className="link-button project-link" onClick={() => onOpen(summary)}>{project.name}</button>
                <small className="muted block">{[project.client, project.location].filter(Boolean).join(" · ") || "—"}</small></td>
              <td><span className={`badge ${stageTone[summary.stage]}`}>{summary.stage}</span>{project.archived && <span className="badge">Archived</span>}
                {summary.engine && <small className="muted block">{engineLabels[summary.engine] ?? summary.engine} · {summary.roomCount} rooms</small>}</td>
              <td>{summary.latestTotal !== undefined && summary.latestCurrency ? <strong>{formatMoney(summary.latestTotal, summary.latestCurrency)}</strong> : <span className="muted">—</span>}
                {summary.estimateCount > 1 && <small className="muted block">{summary.estimateCount} versions</small>}</td>
              <td>{new Date(summary.updatedAt).toLocaleDateString()}</td>
              <td><div className="row-actions">
                <button type="button" className="button button--secondary button--small" onClick={() => onOpen(summary)}>Open</button>
                <button type="button" className="button button--ghost button--small" disabled={busyId === project.id} aria-label={`Duplicate ${project.name}`} onClick={() => act(project.id, () => request<Project>(`/api/projects/${project.id}/duplicate`, { method: "POST" }))}>Duplicate</button>
                <button type="button" className="button button--ghost button--small" disabled={busyId === project.id} aria-label={`${project.archived ? "Unarchive" : "Archive"} ${project.name}`} onClick={() => act(project.id, () => request<Project>(`/api/projects/${project.id}`, jsonInit("PATCH", { archived: !project.archived })))}>{project.archived ? "Unarchive" : "Archive"}</button>
              </div></td>
            </tr>;
          })}</tbody>
        </table>
      </div>}
  </section>;
}
