"use client";

import { ChangeEvent, DragEvent, useEffect, useRef, useState } from "react";
import { AnalysisFailure, jsonInit, request, runAnalysis, type Candidate, type FloorPlan, type Project } from "./api";
import type { Settings } from "./settings";
import { PlanGate, usePlans } from "./plans";

type Stage = "idle" | "uploading" | "analyzing" | "preparing" | "error";
export type { FloorPlan };

const engineInfo: Record<string, { label: string; detail: string }> = {
  demo: { label: "Demo", detail: "Sample four-room plan, instant" },
  ollama: { label: "Ollama", detail: "Local vision model" },
  gemini: { label: "Google Gemini", detail: "Cloud vision model, most accurate" },
  nvidia: { label: "NVIDIA Nemotron 3.5 Super VL", detail: "NVIDIA NIM vision model" },
};
const stages: { key: Stage; label: string }[] = [
  { key: "uploading", label: "Upload drawing" },
  { key: "analyzing", label: "AI extracts rooms, dimensions and boundaries" },
  { key: "preparing", label: "Prepare review" },
];

function validate(file: File) {
  if (!["application/pdf", "image/png", "image/jpeg"].includes(file.type)) return "Use a PDF, PNG, JPG, or JPEG drawing.";
  if (file.size > 10 * 1024 * 1024) return "The drawing must be smaller than 10 MB.";
  return "";
}

export default function UploadStep(props: {
  project: Project;
  engines: string[];
  settings: Settings;
  file: File | null;
  floorPlan: FloorPlan | null;
  onFile: (file: File) => void;
  onProjectReplaced: (project: Project) => void;
  onFloorPlan: (floorPlan: FloorPlan) => void;
  onAnalyzed: (analysisId: string, candidate: Candidate) => void;
  onBack: () => void;
  onViewPlans: () => void;
}) {
  const { can } = usePlans();
  const { project, engines, settings, file, floorPlan } = props;
  const [engineChoice, setEngine] = useState<string | null>(null);
  const preferredEngine = engineChoice ?? settings.analysis.preferredEngine;
  const engine = !can("aiAnalysis") ? "demo" : preferredEngine === "demo" || engines.includes(preferredEngine) ? preferredEngine : engines.includes("gemini") ? "gemini" : "demo";
  const [stage, setStage] = useState<Stage>("idle");
  const [error, setError] = useState("");
  const [dragging, setDragging] = useState(false);
  const [startedAt, setStartedAt] = useState(0);
  const [now, setNow] = useState(0);
  const cancelled = useRef(false);
  const replacedFile = useRef(false);
  const [pageInfo, setPageInfo] = useState<{ file: File; count: number; error?: string } | null>(null);
  const [pageChoice, setPageChoice] = useState<{ file: File; number: number } | null>(null);
  const [failedAnalysisId, setFailedAnalysisId] = useState<string | null>(null);
  const isPDF = file?.type === "application/pdf";
  const pageCount = pageInfo?.file === file ? pageInfo.count : 0;
  const pageError = pageInfo?.file === file ? pageInfo.error : undefined;
  const selectedPage = pageChoice?.file === file ? pageChoice.number : undefined;

  useEffect(() => { cancelled.current = false; return () => { cancelled.current = true; }; }, []);
  useEffect(() => {
    if (!file || file.type !== "application/pdf") return;
    let disposed = false;
    const loadPages = async () => {
      const pdfjs = await import("pdfjs-dist");
      pdfjs.GlobalWorkerOptions.workerSrc = new URL("pdfjs-dist/build/pdf.worker.min.mjs", import.meta.url).toString();
      const task = pdfjs.getDocument({ data: new Uint8Array(await file.arrayBuffer()) });
      try {
        const document = await task.promise;
        if (!disposed) setPageInfo({ file, count: document.numPages });
      } finally {
        await task.destroy();
      }
    };
    void loadPages().catch(() => { if (!disposed) setPageInfo({ file, count: 0, error: "PDF pages could not be read. Choose an unencrypted readable drawing." }); });
    return () => { disposed = true; };
  }, [file]);
  useEffect(() => {
    if (stage !== "analyzing") return;
    const timer = setInterval(() => setNow(Date.now()), 500);
    return () => clearInterval(timer);
  }, [stage]);

  const choose = (selected: File | undefined) => {
    if (!selected) return;
    const problem = validate(selected);
    setError(problem);
    if (!problem) { replacedFile.current = true; props.onFile(selected); }
  };

  const analyze = async () => {
    if (!file) { setError("Choose a drawing first."); return; }
    if (isPDF && !selectedPage) { setError("Select one PDF page before analysis."); return; }
    setError("");
    setFailedAnalysisId(null);
    try {
      let target = project;
      let plan = !replacedFile.current && floorPlan && floorPlan.fileName === file.name && floorPlan.fileSize === file.size ? floorPlan : null;
      if (!plan) {
        if (floorPlan) {
          target = await request<Project>("/api/projects", jsonInit("POST", { name: project.name, client: project.client, location: project.location, description: project.description }));
          props.onProjectReplaced(target);
        }
        setStage("uploading");
        const form = new FormData();
        form.append("file", file);
        const uploaded = await request<FloorPlan>(`/api/projects/${target.id}/floor-plan`, { method: "POST", body: form });
        plan = { id: uploaded.id, fileName: uploaded.fileName, fileSize: uploaded.fileSize };
        props.onFloorPlan(plan);
        replacedFile.current = false;
      }
      setStage("analyzing");
      setStartedAt(Date.now());
      setNow(Date.now());
      const finished = await runAnalysis(plan.id, engine, () => cancelled.current, isPDF ? selectedPage : undefined);
      if (!finished) return;
      if (isPDF && (finished.candidate.source?.pageNumber !== selectedPage || finished.candidate.source?.floorPlanId !== plan.id)) {
        throw new AnalysisFailure("Analysis source does not match the selected PDF page. Update the API and retry.", finished.id);
      }
      setStage("preparing");
      props.onAnalyzed(finished.id, finished.candidate);
    } catch (failure) {
      if (cancelled.current) return;
      setStage("error");
      if (failure instanceof AnalysisFailure) setFailedAnalysisId(failure.analysisId);
      setError(failure instanceof Error ? failure.message : "The analysis failed.");
    }
  };

  const busy = stage === "uploading" || stage === "analyzing" || stage === "preparing";
  const activeIndex = stages.findIndex((item) => item.key === stage);
  const elapsed = startedAt ? Math.max(0, Math.round((now - startedAt) / 1000)) : 0;

  return <section className="card step-card">
    <header className="step-header"><div><p className="eyebrow">Step 2</p><h2>Upload the floor plan</h2><p className="muted">One residential drawing: PDF, PNG, JPG, or JPEG up to 10 MB.</p></div></header>

    <label className={`dropzone${dragging ? " dropzone--active" : ""}${busy ? " dropzone--disabled" : ""}`}
      onDragOver={(event: DragEvent) => { event.preventDefault(); if (!busy) setDragging(true); }}
      onDragLeave={() => setDragging(false)}
      onDrop={(event: DragEvent) => { event.preventDefault(); setDragging(false); if (!busy) choose(event.dataTransfer.files[0]); }}>
      <input type="file" accept=".pdf,.png,.jpg,.jpeg" disabled={busy} onChange={(event: ChangeEvent<HTMLInputElement>) => choose(event.target.files?.[0])} />
      <span className="dropzone__icon" aria-hidden>⇪</span>
      {file ? <><strong>{file.name}</strong><span className="muted">{(file.size / 1024 / 1024).toFixed(2)} MB · click or drop to replace</span></>
        : <><strong>Drop your drawing here</strong><span className="muted">or click to browse</span></>}
    </label>

    {isPDF && <div className="pdf-page-picker">
      <label htmlFor="analysis-page">Drawing page</label>
      <select id="analysis-page" value={selectedPage ?? ""} disabled={busy || !pageCount} onChange={(event) => { if (file) setPageChoice(event.target.value ? { file, number: Number(event.target.value) } : null); }}>
        <option value="">{pageCount ? "Select a page" : "Reading PDF pages..."}</option>
        {Array.from({ length: pageCount }, (_, index) => <option key={index + 1} value={index + 1}>Page {index + 1}</option>)}
      </select>
      {selectedPage && <span className="muted">Page {selectedPage} of {pageCount}; other pages unprocessed</span>}
      {pageError && <p className="alert alert--error" role="alert">{pageError}</p>}
    </div>}

    <fieldset className="engine-options" disabled={busy}>
      <legend>Analysis engine</legend>
      {Object.entries(engineInfo).map(([key, info]) => {
        const entitled = key === "demo" || can("aiAnalysis");
        const available = entitled && (key === "demo" || engines.includes(key));
        return <label key={key} className={`engine-option${engine === key ? " engine-option--selected" : ""}${available ? "" : " engine-option--disabled"}`}>
          <input type="radio" name="engine" value={key} checked={engine === key} disabled={!available} onChange={() => setEngine(key)} />
          <span><strong>{info.label}</strong><small>{!entitled ? "Pro or Studio required" : available ? info.detail : "Not configured on the server"}</small></span>
        </label>;
      })}
    </fieldset>
    {!can("aiAnalysis") && <PlanGate feature="aiAnalysis" onViewPlans={props.onViewPlans} />}

    {(busy || stage === "error") && <div className="progress-panel" role="status" aria-live="polite">
      <div className={`progress-bar${busy ? " progress-bar--active" : ""}`}><span /></div>
      <ol className="progress-stages">
        {stages.map((item, index) => {
          const state = stage === "error" ? (index < Math.max(activeIndex, 1) ? "done" : "pending") : index < activeIndex ? "done" : index === activeIndex ? "active" : "pending";
          return <li key={item.key} className={`progress-stage progress-stage--${state}`}>
            <span className="progress-stage__marker" aria-hidden>{state === "done" ? "✓" : state === "active" ? <span className="spinner" /> : index + 1}</span>
            <span>{item.label}{item.key === "analyzing" && stage === "analyzing" && <em> · {elapsed}s{engine === "gemini" && elapsed > 20 ? " — large drawings can take a few minutes" : ""}</em>}</span>
          </li>;
        })}
      </ol>
    </div>}

    {error && <p className="alert alert--error" role="alert">{error}</p>}
    {failedAnalysisId && <p className="cell-note">Analysis {failedAnalysisId}</p>}
    <footer className="step-footer">
      <button type="button" className="button button--ghost" onClick={props.onBack} disabled={busy}>Back</button>
      <button type="button" className="button button--primary" onClick={analyze} disabled={busy || !file || !!(isPDF && (!pageCount || !selectedPage))}>{busy && <span className="spinner" aria-hidden />}{busy ? "Analyzing…" : stage === "error" ? "Retry analysis" : "Analyze drawing"}</button>
    </footer>
  </section>;
}
