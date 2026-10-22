"use client";

import { useEffect, useEffectEvent, useState } from "react";
import { CreditCard, LockKeyhole, LogOut, Moon, Settings as SettingsIcon, Sun } from "lucide-react";
import { useAuth } from "./auth-provider";
import { useTheme } from "./theme-provider";
import { fetchDrawing, request, type Candidate, type Confirmation, type EstimateVersion, type FloorPlan, type Project, type ProjectSummary } from "./api";
import Dashboard from "./dashboard";
import { loadDrawing, saveDrawing } from "./drawing-store";
import Overview from "./overview";
import SettingsView from "./settings-view";
import { defaultSettings, loadSettings, type Settings } from "./settings";
import EstimateStep from "./step-estimate";
import ModelStep from "./step-model";
import ProjectStep from "./step-project";
import ReviewStep from "./step-review";
import UploadStep from "./step-upload";
import RoomDesignStudio from "./room-design-studio";
import ExteriorView from "./exterior-view";
import { PlanGate, PlansView, usePlans } from "./plans";

type View = "dashboard" | "project" | "design-studio" | "exterior" | "settings" | "plans";
type StepKey = "overview" | "project" | "upload" | "review" | "model" | "estimate";
type Session = { projectId: string };
type Workspace = { project: Project; floorPlan: FloorPlan | null; analysisId: string; candidate: Candidate | null; confirmation: Confirmation | null; estimates: EstimateVersion[] };

const sessionKey = "quantity-surveyor-session";
const steps: { key: Exclude<StepKey, "overview">; label: string; hint: string }[] = [
  { key: "project", label: "Project", hint: "Details" },
  { key: "upload", label: "Floor plan", hint: "Upload & analyze" },
  { key: "review", label: "Review", hint: "Confirm dimensions" },
  { key: "model", label: "3D model", hint: "Layout preview" },
  { key: "estimate", label: "Estimate", hint: "Costs & export" },
];

function readSession(): Session | null {
  try {
    const saved = JSON.parse(localStorage.getItem(sessionKey) || "null") as Session | null;
    return typeof saved?.projectId === "string" && saved.projectId ? saved : null;
  } catch {
    return null;
  }
}

async function loadWorkspace(projectId: string): Promise<Workspace> {
  const summary = await request<ProjectSummary>(`/api/projects/${encodeURIComponent(projectId)}/summary`);
  const workspace: Workspace = { project: summary.project, floorPlan: summary.floorPlan ?? null, analysisId: summary.analysisId ?? "", candidate: null, confirmation: null, estimates: [] };
  if (!summary.analysisId) return workspace;
  const id = encodeURIComponent(summary.analysisId);
  const [candidate, confirmation, estimates] = await Promise.all([
    request<Candidate>(`/api/analyses/${id}`).catch(() => null),
    request<Confirmation>(`/api/analyses/${id}/confirmed-data`).catch(() => null),
    request<EstimateVersion[] | null>(`/api/projects/${encodeURIComponent(projectId)}/estimates`).catch(() => null),
  ]);
  workspace.candidate = candidate?.analysisState === "AnalysisReady" ? candidate : null;
  workspace.confirmation = confirmation;
  workspace.estimates = (estimates ?? []).filter((version) => version.sourceAnalysisId === summary.analysisId);
  return workspace;
}

async function loadFile(analysisId: string, floorPlan: FloorPlan | null) {
  const cached = analysisId ? await loadDrawing(analysisId).catch(() => null) : null;
  if (cached || !floorPlan) return cached;
  return fetchDrawing(floorPlan).catch(() => null);
}

export default function Home() {
  const { enabled, user, signOut } = useAuth();
  const { theme, toggleTheme } = useTheme();
  const { account, can, error: planError, refresh: refreshPlan } = usePlans();
  const [view, setView] = useState<View>("dashboard");
  const [returnView, setReturnView] = useState<View>("dashboard");
  const [step, setStep] = useState<StepKey>("project");
  const [settings, setSettings] = useState<Settings>(() => (typeof window === "undefined" ? defaultSettings : loadSettings()));
  const [project, setProject] = useState<Project | null>(null);
  const [engines, setEngines] = useState<string[]>(["demo"]);
  const [file, setFile] = useState<File | null>(null);
  const [floorPlan, setFloorPlan] = useState<FloorPlan | null>(null);
  const [analysisId, setAnalysisId] = useState("");
  const [candidate, setCandidate] = useState<Candidate | null>(null);
  const [confirmation, setConfirmation] = useState<Confirmation | null>(null);
  const [estimates, setEstimates] = useState<EstimateVersion[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const apply = (workspace: Workspace) => {
    setProject(workspace.project);
    setFloorPlan(workspace.floorPlan);
    setAnalysisId(workspace.analysisId);
    setCandidate(workspace.candidate);
    setConfirmation(workspace.confirmation);
    setEstimates(workspace.estimates);
    setFile(null);
    void loadFile(workspace.analysisId, workspace.floorPlan).then(setFile);
  };

  const resumeStep = (workspace: Workspace): StepKey => workspace.estimates.length ? "estimate" : workspace.confirmation ? (can("model3d") ? "model" : "estimate") : workspace.candidate ? "review" : "upload";
  const restoreWorkspace = useEffectEvent((workspace: Workspace) => {
    apply(workspace);
    setView("project");
    setStep(resumeStep(workspace));
  });

  useEffect(() => {
    request<string[]>("/api/analysis-engines").then(setEngines).catch(() => {});
    const saved = readSession();
    (saved ? loadWorkspace(saved.projectId) : Promise.resolve(null))
      .then((workspace) => {
        if (!workspace) return;
        restoreWorkspace(workspace);
      })
      .catch(() => localStorage.removeItem(sessionKey))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    if (view === "project" && project) localStorage.setItem(sessionKey, JSON.stringify({ projectId: project.id } satisfies Session));
    if (view === "dashboard") localStorage.removeItem(sessionKey);
  }, [view, project]);

  const clear = () => { setProject(null); setFile(null); setFloorPlan(null); setAnalysisId(""); setCandidate(null); setConfirmation(null); setEstimates([]); };
  const resetAnalysis = () => { setAnalysisId(""); setCandidate(null); setConfirmation(null); setEstimates([]); };

  const openProject = async (summary: ProjectSummary) => {
    setLoading(true);
    setError("");
    try {
      apply(await loadWorkspace(summary.project.id));
      setView("project");
      setStep("overview");
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "The project could not be opened.");
    } finally {
      setLoading(false);
    }
  };

  const newProject = () => { clear(); setView("project"); setStep("project"); };
  const showSettings = () => { if (view !== "settings") setReturnView(view); setView("settings"); };
  const showPlans = () => { if (view !== "plans") setReturnView(view); setView("plans"); };
  const leaveAccount = async () => {
    setError("");
    try {
      await signOut();
      clear();
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "Could not sign out.");
    }
  };
  const analyzed = (id: string, result: Candidate) => {
    setAnalysisId(id); setCandidate(result); setConfirmation(null); setEstimates([]);
    if (file) void saveDrawing(id, file).catch(() => {});
    setStep("review");
  };

  const reachable: Record<StepKey, boolean> = { overview: !!project, project: true, upload: !!project, review: !!candidate, model: !!confirmation && can("model3d"), estimate: !!confirmation };
  const complete: Record<StepKey, boolean> = { overview: false, project: !!project, upload: !!candidate, review: !!confirmation, model: !!confirmation && step === "estimate", estimate: estimates.length > 0 };
  const currentIndex = steps.findIndex((item) => item.key === step);
  const nextKey = project ? resumeStep({ project, floorPlan, analysisId, candidate, confirmation, estimates }) : "project";
  const next = steps.find((item) => item.key === nextKey) ?? steps[0];

  return <div className="app">
    <div className="app-top no-print">
      <header className="app-header">
        <div className="brand"><span className="brand__mark">QS</span><div><strong>AI Quantity Surveyor</strong><small>Interior finishes estimating</small></div></div>
        {(view === "project" || view === "design-studio" || view === "exterior") && project && <div className="header-project"><span className="muted">Project</span><strong>{project.name}</strong>{project.client && <small>{project.client}</small>}</div>}
        <nav className="header-nav" aria-label="Main">
          <button type="button" className={`nav-link${view === "dashboard" ? " nav-link--active" : ""}`} aria-current={view === "dashboard" ? "page" : undefined} onClick={() => { setError(""); setView("dashboard"); }}>Projects</button>
          {project && confirmation && <button type="button" className={`nav-link${view === "design-studio" ? " nav-link--active" : ""}`} aria-current={view === "design-studio" ? "page" : undefined} onClick={() => { setError(""); setView("design-studio"); }}>{!can("designStudio") && <LockKeyhole size={14} aria-hidden="true" />}Design Studio</button>}
          {project && confirmation && <button type="button" className={`nav-link${view === "exterior" ? " nav-link--active" : ""}`} aria-current={view === "exterior" ? "page" : undefined} onClick={() => { setError(""); setView("exterior"); }}>{!can("model3d") && <LockKeyhole size={14} aria-hidden="true" />}Exterior</button>}
          <button type="button" className={`nav-link nav-link--plan${view === "plans" ? " nav-link--active" : ""}`} aria-current={view === "plans" ? "page" : undefined} onClick={showPlans}><CreditCard size={15} aria-hidden="true" />Plans <span className="header-plan-label">{account?.plan.name ?? "..."}</span></button>
        </nav>
        <div className="header-actions">
          <button type="button" className={`icon-button header-icon-button${view === "settings" ? " header-icon-button--active" : ""}`} aria-label="Settings" title="Settings" aria-current={view === "settings" ? "page" : undefined} onClick={showSettings}><SettingsIcon size={18} aria-hidden="true" /></button>
          <button type="button" className="icon-button header-icon-button" aria-label={theme === "dark" ? "Switch to light theme" : "Switch to dark theme"} title={theme === "dark" ? "Switch to light theme" : "Switch to dark theme"} onClick={toggleTheme}>{theme === "dark" ? <Sun size={18} aria-hidden="true" /> : <Moon size={18} aria-hidden="true" />}</button>
          {enabled && <div className="account-controls"><span className="account-email" title={user?.email ?? "Signed in"}>{user?.email ?? "Signed in"}</span><button type="button" className="icon-button header-icon-button" aria-label="Sign out" title="Sign out" onClick={() => void leaveAccount()}><LogOut size={18} aria-hidden="true" /></button></div>}
        </div>
      </header>

      {view === "project" && <nav className="stepper" aria-label="Workflow">
        <button type="button" className={`stepper__overview${step === "overview" ? " stepper__overview--current" : ""}`} disabled={!project} aria-current={step === "overview" ? "page" : undefined} onClick={() => setStep("overview")}>Overview</button>
        <ol>{steps.map((item, index) => {
          const state = item.key === step ? "current" : complete[item.key] ? "complete" : reachable[item.key] ? "available" : "locked";
          return <li key={item.key} className={`stepper__item stepper__item--${state}`}>
            <button type="button" disabled={!reachable[item.key]} aria-current={item.key === step ? "step" : undefined} onClick={() => setStep(item.key)}>
              <span className="stepper__index" aria-hidden>{item.key === "model" && !can("model3d") ? <LockKeyhole size={13} /> : state === "complete" ? "✓" : index + 1}</span>
              <span className="stepper__text"><strong>{item.label}</strong><small>{item.hint}</small></span>
            </button>
          </li>;
        })}</ol>
        <div className="stepper__progress" aria-hidden><span style={{ width: `${(Math.max(currentIndex, 0) / (steps.length - 1)) * 100}%` }} /></div>
      </nav>}
    </div>

    <main className="app-main">
      {error && <p className="alert alert--error" role="alert">{error}</p>}
      {planError && view !== "plans" && <div className="alert alert--warning plan-status" role="alert"><span>{planError}</span><button type="button" className="button button--ghost button--small" onClick={() => void refreshPlan()}>Retry plan lookup</button></div>}
      {loading ? <div className="card step-card loading-card" role="status"><span className="spinner" aria-hidden /> Loading…</div> : <>
        {view === "dashboard" && <Dashboard onOpen={openProject} onNew={newProject} />}
        {view === "plans" && <PlansView onClose={() => setView(returnView === "plans" ? "dashboard" : returnView)} />}
        {view === "settings" && <SettingsView settings={settings} engines={engines} onSaved={setSettings} onClose={() => setView(returnView === "settings" ? "dashboard" : returnView)} />}
        {view === "design-studio" && project && confirmation && <RoomDesignStudio key={`${project.id}:${confirmation.revision}`} project={project} confirmation={confirmation} settings={settings} onViewPlans={showPlans} onBack={() => setView("project")} />}
        {view === "exterior" && project && confirmation && <ExteriorView key={project.id} project={project} confirmation={confirmation} onViewPlans={showPlans} onBack={() => setView("project")} />}
        {view === "project" && <>
          {step === "overview" && project && <Overview project={project} floorPlan={floorPlan} file={file} analysisId={analysisId} candidate={candidate} confirmation={confirmation} estimates={estimates} engines={can("aiAnalysis") ? engines : ["demo"]}
            onRerun={analyzed} onContinue={() => setStep(next.key)} continueLabel={`Continue: ${next.label}`} />}
          {step === "project" && <ProjectStep key={project?.id ?? "new"} project={project} onCreated={(created) => { setProject(created); setStep("upload"); }} onContinue={() => setStep("upload")} />}
          {step === "upload" && project && <UploadStep project={project} engines={engines} settings={settings} file={file} floorPlan={floorPlan}
            onViewPlans={showPlans}
            onFile={(selected) => { setFile(selected); resetAnalysis(); }}
            onProjectReplaced={(replacement) => { setProject(replacement); setFloorPlan(null); }}
            onFloorPlan={setFloorPlan}
            onAnalyzed={analyzed}
            onBack={() => setStep("project")} />}
          {step === "review" && candidate && <ReviewStep key={analysisId} analysisId={analysisId} candidate={confirmation?.candidate ?? candidate} analysis={candidate} settings={settings} engines={can("aiAnalysis") ? engines : ["demo"]} floorPlanId={floorPlan?.id} file={file} lastConfirmed={confirmation}
            onRerun={analyzed} onBack={() => setStep("upload")} onConfirmed={(confirmed) => { setConfirmation(confirmed); setStep(can("model3d") ? "model" : "estimate"); }} />}
          {step === "model" && confirmation && (can("model3d") ? <ModelStep confirmation={confirmation} file={file} onBack={() => setStep("review")} onContinue={() => setStep("estimate")} /> : <section className="card step-card"><PlanGate feature="model3d" onViewPlans={showPlans} /><button type="button" className="button button--primary" onClick={() => setStep("estimate")}>Continue to estimate</button></section>)}
          {step === "estimate" && project && confirmation && <EstimateStep key={analysisId} project={project} analysisId={analysisId} confirmation={confirmation} estimates={estimates} settings={settings} onViewPlans={showPlans} onEstimate={(created) => setEstimates((current) => [created, ...current])} onBack={() => setStep(can("model3d") ? "model" : "review")} />}
        </>}
      </>}
    </main>
  </div>;
}
