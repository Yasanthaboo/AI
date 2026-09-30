"use client";

import { useEffect, useState } from "react";
import { request, type Candidate, type Confirmation, type EstimateVersion, type Project } from "./api";
import { loadDrawing, saveDrawing } from "./drawing-store";
import EstimateStep from "./step-estimate";
import ModelStep from "./step-model";
import ProjectStep from "./step-project";
import ReviewStep from "./step-review";
import UploadStep, { type FloorPlan } from "./step-upload";

type StepKey = "project" | "upload" | "review" | "model" | "estimate";
type Session = { projectId: string; analysisId?: string; floorPlan?: FloorPlan };
type Restored = { project: Project; floorPlan?: FloorPlan; analysisId?: string; candidate: Candidate | null; confirmation: Confirmation | null; estimates: EstimateVersion[]; file: File | null };

const sessionKey = "quantity-surveyor-session";
const steps: { key: StepKey; label: string; hint: string }[] = [
  { key: "project", label: "Project", hint: "Details" },
  { key: "upload", label: "Floor plan", hint: "Upload & analyze" },
  { key: "review", label: "Review", hint: "Confirm dimensions" },
  { key: "model", label: "3D model", hint: "Layout preview" },
  { key: "estimate", label: "Estimate", hint: "Costs & export" },
];

function readSession(): Session | null {
  try {
    const saved = JSON.parse(localStorage.getItem(sessionKey) || "null") as Session | null;
    return saved?.projectId ? saved : null;
  } catch {
    return null;
  }
}

async function restore(saved: Session): Promise<Restored> {
  const project = await request<Project>(`/api/projects/${encodeURIComponent(saved.projectId)}`);
  const estimates = (await request<EstimateVersion[] | null>(`/api/projects/${encodeURIComponent(project.id)}/estimates`).catch(() => null)) ?? [];
  if (!saved.analysisId) return { project, floorPlan: saved.floorPlan, candidate: null, confirmation: null, estimates, file: null };
  const id = encodeURIComponent(saved.analysisId);
  const [candidate, confirmation, file] = await Promise.all([
    request<Candidate>(`/api/analyses/${id}`).catch(() => null),
    request<Confirmation>(`/api/analyses/${id}/confirmed-data`).catch(() => null),
    loadDrawing(saved.analysisId).catch(() => null),
  ]);
  return { project, floorPlan: saved.floorPlan, analysisId: saved.analysisId, candidate: candidate?.analysisState === "AnalysisReady" ? candidate : null, confirmation, estimates: estimates.filter((version) => version.sourceAnalysisId === saved.analysisId), file };
}

export default function Home() {
  const [step, setStep] = useState<StepKey>("project");
  const [project, setProject] = useState<Project | null>(null);
  const [engines, setEngines] = useState<string[]>(["demo"]);
  const [file, setFile] = useState<File | null>(null);
  const [floorPlan, setFloorPlan] = useState<FloorPlan | null>(null);
  const [analysisId, setAnalysisId] = useState("");
  const [candidate, setCandidate] = useState<Candidate | null>(null);
  const [confirmation, setConfirmation] = useState<Confirmation | null>(null);
  const [estimates, setEstimates] = useState<EstimateVersion[]>([]);
  const [restoring, setRestoring] = useState(true);

  useEffect(() => {
    request<string[]>("/api/analysis-engines").then(setEngines).catch(() => {});
    const saved = readSession();
    (saved ? restore(saved) : Promise.resolve(null))
      .then((restored) => {
        if (!restored) return;
        setProject(restored.project);
        setFloorPlan(restored.floorPlan ?? null);
        setAnalysisId(restored.analysisId ?? "");
        setCandidate(restored.candidate);
        setConfirmation(restored.confirmation);
        setEstimates(restored.estimates);
        setFile(restored.file);
        setStep(restored.estimates.length ? "estimate" : restored.confirmation ? "model" : restored.candidate ? "review" : "upload");
      })
      .catch(() => localStorage.removeItem(sessionKey))
      .finally(() => setRestoring(false));
  }, []);

  useEffect(() => {
    if (project) localStorage.setItem(sessionKey, JSON.stringify({ projectId: project.id, analysisId: analysisId || undefined, floorPlan: floorPlan ?? undefined } satisfies Session));
  }, [project, analysisId, floorPlan]);

  const reachable: Record<StepKey, boolean> = { project: true, upload: !!project, review: !!candidate, model: !!confirmation, estimate: !!confirmation };
  const complete: Record<StepKey, boolean> = { project: !!project, upload: !!candidate, review: !!confirmation, model: !!confirmation && step === "estimate", estimate: estimates.length > 0 };
  const currentIndex = steps.findIndex((item) => item.key === step);

  const startOver = () => {
    if (project && !window.confirm("Start a new project? The current project stays saved on the server.")) return;
    localStorage.removeItem(sessionKey);
    setProject(null); setFile(null); setFloorPlan(null); setAnalysisId(""); setCandidate(null); setConfirmation(null); setEstimates([]); setStep("project");
  };

  const resetAnalysis = () => { setAnalysisId(""); setCandidate(null); setConfirmation(null); setEstimates([]); };

  return <div className="app">
    <div className="app-top no-print">
    <header className="app-header">
      <div className="brand"><span className="brand__mark">QS</span><div><strong>AI Quantity Surveyor</strong><small>Plaster &amp; paint estimating</small></div></div>
      {project && <div className="header-project"><span className="muted">Project</span><strong>{project.name}</strong>{project.client && <small>{project.client}</small>}</div>}
      <button type="button" className="button button--ghost button--small" onClick={startOver}>New project</button>
    </header>

    <nav className="stepper" aria-label="Workflow">
      <ol>{steps.map((item, index) => {
        const state = item.key === step ? "current" : complete[item.key] ? "complete" : reachable[item.key] ? "available" : "locked";
        return <li key={item.key} className={`stepper__item stepper__item--${state}`}>
          <button type="button" disabled={!reachable[item.key]} aria-current={item.key === step ? "step" : undefined} onClick={() => setStep(item.key)}>
            <span className="stepper__index" aria-hidden>{state === "complete" ? "✓" : index + 1}</span>
            <span className="stepper__text"><strong>{item.label}</strong><small>{item.hint}</small></span>
          </button>
        </li>;
      })}</ol>
      <div className="stepper__progress" aria-hidden><span style={{ width: `${(currentIndex / (steps.length - 1)) * 100}%` }} /></div>
    </nav>
    </div>

    <main className="app-main">
      {restoring ? <div className="card step-card loading-card" role="status"><span className="spinner" aria-hidden /> Restoring your last project…</div> : <>
        {step === "project" && <ProjectStep key={project?.id ?? "new"} project={project} onCreated={(created) => { setProject(created); setStep("upload"); }} onContinue={() => setStep("upload")} />}
        {step === "upload" && project && <UploadStep project={project} engines={engines} file={file} floorPlan={floorPlan}
          onFile={(selected) => { setFile(selected); resetAnalysis(); }}
          onProjectReplaced={(replacement) => { setProject(replacement); setFloorPlan(null); }}
          onFloorPlan={setFloorPlan}
          onAnalyzed={(id, result) => { setAnalysisId(id); setCandidate(result); setConfirmation(null); setEstimates([]); if (file) void saveDrawing(id, file).catch(() => {}); setStep("review"); }}
          onBack={() => setStep("project")} />}
        {step === "review" && candidate && <ReviewStep key={analysisId} analysisId={analysisId} candidate={confirmation?.candidate ?? candidate} onBack={() => setStep("upload")} onConfirmed={(confirmed) => { setConfirmation(confirmed); setStep("model"); }} />}
        {step === "model" && confirmation && <ModelStep confirmation={confirmation} file={file} onBack={() => setStep("review")} onContinue={() => setStep("estimate")} />}
        {step === "estimate" && project && confirmation && <EstimateStep project={project} analysisId={analysisId} estimates={estimates} onEstimate={(created) => setEstimates((current) => [created, ...current])} onBack={() => setStep("model")} />}
      </>}
    </main>
  </div>;
}
