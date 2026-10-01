"use client";

import { FormEvent, useState } from "react";
import { jsonInit, request, type Project, type ProjectDetails } from "./api";

export default function ProjectStep({ project, onCreated, onContinue }: { project: Project | null; onCreated: (project: Project) => void; onContinue: () => void }) {
  const [details, setDetails] = useState<ProjectDetails>({ name: project?.name ?? "", client: project?.client ?? "", location: project?.location ?? "", description: project?.description ?? "" });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const update = (field: keyof ProjectDetails) => (event: { target: { value: string } }) => setDetails((current) => ({ ...current, [field]: event.target.value }));

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!details.name.trim()) { setError("Enter a project name."); return; }
    setBusy(true);
    setError("");
    try {
      onCreated(await request<Project>("/api/projects", jsonInit("POST", details)));
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "The project could not be created.");
    } finally {
      setBusy(false);
    }
  };

  if (project) {
    return <section className="card step-card">
      <header className="step-header"><div><p className="eyebrow">Step 1</p><h2>Project details</h2></div><span className="badge badge--success">Created</span></header>
      <dl className="summary-grid">
        <div><dt>Project</dt><dd>{project.name}</dd></div>
        <div><dt>Client</dt><dd>{project.client || "—"}</dd></div>
        <div><dt>Site location</dt><dd>{project.location || "—"}</dd></div>
        <div><dt>Created</dt><dd>{new Date(project.createdAt).toLocaleString()}</dd></div>
        {project.description && <div className="summary-grid__wide"><dt>Notes</dt><dd>{project.description}</dd></div>}
      </dl>
      <footer className="step-footer"><span className="muted">To change these details, create a new project from Projects.</span><button type="button" className="button button--primary" onClick={onContinue}>Continue to floor plan</button></footer>
    </section>;
  }

  return <form className="card step-card" onSubmit={submit} noValidate>
    <header className="step-header"><div><p className="eyebrow">Step 1</p><h2>Project details</h2><p className="muted">Tell us about the residential project before uploading its floor plan.</p></div></header>
    <div className="form-grid">
      <label className="field"><span>Project name <span className="required">*</span></span><input value={details.name} onChange={update("name")} placeholder="e.g. Smith residence" maxLength={120} required autoFocus /></label>
      <label className="field"><span>Client</span><input value={details.client} onChange={update("client")} placeholder="Client or company" maxLength={120} /></label>
      <label className="field field--wide"><span>Site location</span><input value={details.location} onChange={update("location")} placeholder="Street, city" maxLength={200} /></label>
      <label className="field field--wide"><span>Notes</span><textarea value={details.description} onChange={update("description")} placeholder="Scope notes, finish specification, deadlines…" rows={3} maxLength={2000} /></label>
    </div>
    {error && <p className="alert alert--error" role="alert">{error}</p>}
    <footer className="step-footer"><span className="muted">Fields marked * are required.</span><button type="submit" className="button button--primary" disabled={busy}>{busy && <span className="spinner" aria-hidden />}{busy ? "Creating project…" : "Create project"}</button></footer>
  </form>;
}
