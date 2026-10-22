"use client";

import dynamic from "next/dynamic";
import { useEffect, useState } from "react";
import { ArrowLeft, Save } from "lucide-react";
import { jsonInit, request, type Confirmation, type Project } from "./api";
import { exteriorAssumptions, suggestExterior, validExterior, type ExteriorInput, type ExteriorOption, type RoofForm } from "./exterior";
import { PlanGate, usePlans } from "./plans";

const ExteriorScene = dynamic(() => import("./exterior-scene"), { ssr: false, loading: () => <div className="drawing-loading" role="status">Loading exterior preview...</div> });
const roofForms: { id: RoofForm; label: string }[] = [{ id: "flat", label: "Flat" }, { id: "gable", label: "Gable" }, { id: "hip", label: "Hip" }, { id: "shed", label: "Shed" }];
const draftFromOption = (option: ExteriorOption): ExteriorInput => ({ analysisId: option.analysisId, confirmationRevision: option.confirmationRevision, roof: option.roof, pitchDegrees: option.pitchDegrees, overhangM: option.overhangM, lengthM: option.lengthM, widthM: option.widthM, wallHeightM: option.wallHeightM, finish: option.finish, assumptionsReviewed: option.assumptionsReviewed });

function ExteriorEditor({ project, confirmation }: { project: Project; confirmation: Confirmation }) {
  const [draft, setDraft] = useState<ExteriorInput>(() => suggestExterior(confirmation));
  const [saved, setSaved] = useState<ExteriorOption[]>([]);
  const [selected, setSelected] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [retry, setRetry] = useState(0);
  const path = `/api/projects/${encodeURIComponent(project.id)}/exterior-options`;
  useEffect(() => {
    let active = true;
    request<ExteriorOption[]>(`${path}?analysisId=${encodeURIComponent(confirmation.analysisId)}&revision=${confirmation.revision}`).then((options) => {
      if (!active) return;
      setSaved(options); setError("");
      if (options[0]) { setDraft(draftFromOption(options[0])); setSelected(options[0].id); }
    }).catch(() => { if (active) setError("Saved exterior options could not be loaded."); }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [path, confirmation.analysisId, confirmation.revision, retry]);

  const update = (changes: Partial<ExteriorInput>) => {
    setDraft((current) => ({ ...current, ...changes, assumptionsReviewed: changes.assumptionsReviewed ?? false }));
    setSelected(""); setNotice("");
  };
  const save = async () => {
    if (!validExterior(draft) || !draft.assumptionsReviewed) { setError("Review supported envelope dimensions and the exterior assumptions before saving."); return; }
    setBusy(true); setError(""); setNotice("");
    try {
      const option = await request<ExteriorOption>(path, jsonInit("POST", draft));
      setSaved((current) => [option, ...current].slice(0, 100)); setSelected(option.id); setNotice("Exterior option saved. Estimates are unchanged.");
    } catch (failure) { setError(failure instanceof Error ? failure.message : "Exterior option could not be saved."); }
    finally { setBusy(false); }
  };

  return <>
    <p className="alert alert--warning">{exteriorAssumptions}</p>
    {loading ? <p role="status">Loading saved exterior options...</p> : <div className="exterior-layout">
      <aside className="exterior-controls">
        <fieldset className="roof-options"><legend>Roof form</legend>{roofForms.map((roof) => <label key={roof.id} className={draft.roof === roof.id ? "roof-option roof-option--active" : "roof-option"}><input type="radio" name="roof-form" value={roof.id} checked={draft.roof === roof.id} onChange={() => update({ roof: roof.id, pitchDegrees: roof.id === "flat" ? 0 : draft.pitchDegrees || 25 })} />{roof.label}</label>)}</fieldset>
        <label className="field"><span>Roof pitch <small>degrees{draft.roof === "flat" ? " - schematic flat surface" : ""}</small></span><input aria-label="Roof pitch" type="range" min={draft.roof === "flat" ? 0 : 5} max="60" step="1" value={draft.pitchDegrees} disabled={draft.roof === "flat"} onChange={(event) => update({ pitchDegrees: Number(event.target.value) })} /><output>{draft.pitchDegrees} degrees</output></label>
        <label className="field"><span>Roof overhang <small>m</small></span><input aria-label="Roof overhang" type="range" min="0" max="2" step="0.1" value={draft.overhangM} onChange={(event) => update({ overhangM: Number(event.target.value) })} /><output>{draft.overhangM.toFixed(1)} m</output></label>
        <h3 className="section-title">Envelope assumptions</h3>
        {([["lengthM", "Envelope length"], ["widthM", "Envelope width"], ["wallHeightM", "Exterior wall height"]] as const).map(([key, label]) => <label className="field" key={key}><span>{label} <small>m</small></span><input type="number" min="1" max={key === "wallHeightM" ? 12 : 100} step="0.01" value={draft[key]} onChange={(event) => update({ [key]: Number(event.target.value) })} /></label>)}
        <label className="field"><span>Roof appearance</span><select value={draft.finish} onChange={(event) => update({ finish: event.target.value as ExteriorInput["finish"] })}><option value="metal">Metal appearance</option><option value="tile">Tile appearance</option><option value="membrane">Membrane appearance</option></select></label>
        <label className="checkbox"><input type="checkbox" checked={draft.assumptionsReviewed} onChange={(event) => update({ assumptionsReviewed: event.target.checked })} />I reviewed the exterior assumptions</label>
        <button type="button" className="button button--primary" disabled={busy || !draft.assumptionsReviewed || !validExterior(draft)} onClick={() => void save()}><Save size={16} aria-hidden="true" />{busy ? "Saving..." : "Save exterior option"}</button>
        <label className="field"><span>Saved exterior option</span><select value={selected} onChange={(event) => { const option = saved.find((entry) => entry.id === event.target.value); if (option) { setDraft(draftFromOption(option)); setSelected(option.id); setNotice(""); } }}><option value="">Unsaved option</option>{saved.map((option, index) => <option value={option.id} key={option.id}>{option.roof} - revision {option.confirmationRevision} - option {saved.length - index}</option>)}</select></label>
      </aside>
      <div className="exterior-preview">
        <div className="design-results__heading"><h3>{roofForms.find((roof) => roof.id === draft.roof)?.label} roof</h3><span className="badge">Assumed single storey</span></div>
        {validExterior(draft) ? <ExteriorScene options={draft} /> : <p className="alert alert--warning">Enter supported envelope dimensions before rendering. Missing dimensions are not inferred as verified facts.</p>}
        <dl className="summary-grid"><div><dt>Envelope</dt><dd>{draft.lengthM} x {draft.widthM} m</dd></div><div><dt>Wall height</dt><dd>{draft.wallHeightM} m</dd></div><div><dt>Roof pitch</dt><dd>{draft.pitchDegrees} degrees</dd></div><div><dt>Overhang</dt><dd>{draft.overhangM} m</dd></div></dl>
        <p className="muted small exterior-provenance">Analysis {confirmation.analysisId} - confirmed revision {confirmation.revision}. Initial envelope suggestions follow the conceptual room arrangement; review them against the actual building. No elevations or extra levels are verified.</p>
      </div>
    </div>}
    {error && <div className="alert alert--error" role="alert">{error}<button type="button" className="button button--ghost button--small" onClick={() => { setLoading(true); setRetry((current) => current + 1); }}>Reload saved options</button></div>}
    {notice && <p className="alert alert--success" role="status">{notice}</p>}
  </>;
}

export default function ExteriorView({ project, confirmation, onBack, onViewPlans }: { project: Project; confirmation: Confirmation; onBack: () => void; onViewPlans: () => void }) {
  const { can } = usePlans();
  const allowed = can("model3d");
  const [history, setHistory] = useState<Confirmation[]>([confirmation]);
  const [revision, setRevision] = useState(confirmation.revision);
  useEffect(() => {
    if (!allowed) return;
    let active = true;
    request<Confirmation[]>(`/api/analyses/${encodeURIComponent(confirmation.analysisId)}/confirmations`).then((entries) => {
      if (active) setHistory((entries.some((entry) => entry.revision === confirmation.revision) ? entries : [...entries, confirmation]).sort((first, second) => second.revision - first.revision));
    }).catch(() => {});
    return () => { active = false; };
  }, [confirmation, allowed]);
  const active = history.find((entry) => entry.revision === revision) ?? confirmation;
  return <section className="step-card exterior-view">
    <header className="step-header"><div><p className="eyebrow">{project.name}</p><h2>Exterior and roof</h2><p className="muted">Conceptual option - confirmed revision {active.revision}</p></div><button type="button" className="button button--ghost button--small" onClick={onBack}><ArrowLeft size={16} aria-hidden="true" />Back to project</button></header>
    {!allowed ? <PlanGate feature="model3d" onViewPlans={onViewPlans} /> : <>
      <label className="field field--inline"><span>Exterior source revision</span><select value={active.revision} onChange={(event) => setRevision(Number(event.target.value))}>{history.map((entry) => <option key={entry.revision} value={entry.revision}>Revision {entry.revision}</option>)}</select></label>
      <ExteriorEditor key={`${active.analysisId}:${active.revision}`} project={project} confirmation={active} />
    </>}
  </section>;
}