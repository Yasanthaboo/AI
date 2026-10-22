"use client";

import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { ArrowLeft, Check, CreditCard, LockKeyhole, RefreshCw, X } from "lucide-react";
import { jsonInit, request } from "./api";

export type PlanFeature = "aiAnalysis" | "model3d" | "materials" | "exports" | "designStudio";
export type Plan = { id: string; name: string; priceLabel: string; description: string; features: PlanFeature[] };
export type PlanAccount = { plan: Plan; assignmentSource: string; demoSwitchingEnabled: boolean };

export const planFeatureLabels: Record<PlanFeature, string> = {
  aiAnalysis: "AI drawing analysis",
  model3d: "3D model and conceptual exterior",
  materials: "Material budgets and price comparisons",
  exports: "Excel, CSV and print exports",
  designStudio: "Room interior generation",
};

type PlanState = {
  account: PlanAccount | null;
  loading: boolean;
  error: string;
  refresh: () => Promise<void>;
  switchPlan: (planId: string) => Promise<void>;
  can: (feature: PlanFeature) => boolean;
};

const PlanContext = createContext<PlanState>({ account: null, loading: true, error: "", refresh: async () => {}, switchPlan: async () => {}, can: () => false });

export function usePlans() { return useContext(PlanContext); }

export function PlanProvider({ children }: { children: ReactNode }) {
  const [account, setAccount] = useState<PlanAccount | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const refresh = async () => {
    setLoading(true);
    try {
      const result = await request<PlanAccount>("/api/account/plan");
      setAccount(result);
      setError("");
    } catch {
      setAccount(null);
      setError("Account plan could not be loaded. Paid features are temporarily unavailable.");
    } finally { setLoading(false); }
  };

  useEffect(() => {
    let active = true;
    const load = () => {
      request<PlanAccount>("/api/account/plan").then((result) => {
        if (active) { setAccount(result); setError(""); }
      }).catch(() => {
        if (active) { setAccount(null); setError("Account plan could not be loaded. Paid features are temporarily unavailable."); }
      }).finally(() => { if (active) setLoading(false); });
    };
    load();
    window.addEventListener("focus", load);
    return () => { active = false; window.removeEventListener("focus", load); };
  }, []);

  const switchPlan = async (planId: string) => {
    const result = await request<PlanAccount>("/api/account/plan", jsonInit("PUT", { planId }));
    setAccount(result);
    setError("");
  };

  return <PlanContext.Provider value={{ account, loading, error, refresh, switchPlan, can: (feature) => account?.plan.features.includes(feature) ?? false }}>{children}</PlanContext.Provider>;
}

export function PlanGate({ feature, onViewPlans }: { feature: PlanFeature; onViewPlans: () => void }) {
  const { loading, error, refresh } = usePlans();
  const tier = feature === "designStudio" ? "Studio" : "Pro";
  return <section className="plan-gate no-print" aria-label={`${planFeatureLabels[feature]} access`}>
    <LockKeyhole size={22} aria-hidden="true" />
    <div><h3>{planFeatureLabels[feature]}</h3><p className="muted">{loading ? "Loading account access..." : error || `Included with ${tier}${tier === "Pro" ? " and Studio" : ""}.`}</p></div>
    {error ? <button type="button" className="button button--ghost button--small" onClick={() => void refresh()}><RefreshCw size={15} aria-hidden="true" />Retry</button> : <button type="button" className="button button--ghost button--small" onClick={onViewPlans}><CreditCard size={15} aria-hidden="true" />View plans</button>}
  </section>;
}

export function PlansView({ onClose }: { onClose: () => void }) {
  const { account, loading, error: accountError, refresh, switchPlan } = usePlans();
  const [catalog, setCatalog] = useState<Plan[]>([]);
  const [error, setError] = useState("");
  const [pending, setPending] = useState("");
  const [notice, setNotice] = useState("");
  useEffect(() => {
    let active = true;
    request<Plan[]>("/api/plans").then((result) => { if (active) setCatalog(result); }).catch(() => { if (active) setError("Plans could not be loaded. Reopen this page to retry."); });
    return () => { active = false; };
  }, []);

  const change = async (plan: Plan) => {
    setPending(plan.id); setError(""); setNotice("");
    try {
      await switchPlan(plan.id);
      setNotice(`Demo plan changed to ${plan.name}. No payment was taken.`);
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "Plan could not be changed.");
      await refresh();
    } finally { setPending(""); }
  };

  return <section className="step-card plans-view">
    <header className="step-header"><div><p className="eyebrow">Account</p><h2>Plans</h2><p className="muted">Current plan: <strong>{loading ? "Loading..." : account?.plan.name ?? "Unavailable"}</strong></p></div><button type="button" className="button button--ghost button--small" onClick={onClose}><ArrowLeft size={16} aria-hidden="true" />Back</button></header>
    <p className={`alert ${account?.demoSwitchingEnabled ? "alert--warning" : "alert--info"}`}>{account?.demoSwitchingEnabled ? "Demo mode: plan changes are for testing only. No payment is taken, and demo upgrades do not grant paid access outside demo mode." : "Paid pricing is coming soon. Payments and self-service upgrades are not enabled."}</p>
    {(error || accountError) && <p className="alert alert--error" role="alert">{error || accountError}</p>}
    {notice && <p className="alert alert--success" role="status">{notice}</p>}
    <div className="plan-catalog">{catalog.map((plan) => {
      const current = account?.plan.id === plan.id;
      return <article className={`plan-option${current ? " plan-option--current" : ""}`} key={plan.id}>
        <header><h3>{plan.name}</h3>{current && <span className="badge badge--success">Current plan</span>}</header>
        <p className="plan-option__price">{plan.priceLabel}</p><p className="muted">{plan.description}</p>
        <ul className="plan-feature-list"><li><Check size={16} aria-hidden="true" />Projects, demo analysis and basic estimates</li>{(Object.entries(planFeatureLabels) as [PlanFeature, string][]).map(([feature, label]) => <li key={feature} className={plan.features.includes(feature) ? "" : "plan-feature--excluded"}>{plan.features.includes(feature) ? <Check size={16} aria-hidden="true" /> : <X size={16} aria-hidden="true" />}<span>{label}<span className="sr-only">{plan.features.includes(feature) ? " included" : " not included"}</span></span></li>)}</ul>
        <button type="button" className={`button ${current ? "button--ghost" : "button--primary"}`} disabled={current || !account?.demoSwitchingEnabled || !!pending || loading} onClick={() => void change(plan)}>{pending === plan.id ? "Changing plan..." : current ? "Current plan" : account?.demoSwitchingEnabled ? `Switch to ${plan.name} (demo)` : "Not available yet"}</button>
      </article>;
    })}</div>
    <p className="muted small">Provider availability and daily usage limits still apply. Switching plans does not reset usage or alter existing projects, estimates, or saved designs.</p>
  </section>;
}