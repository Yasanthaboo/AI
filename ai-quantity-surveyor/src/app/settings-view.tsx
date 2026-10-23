"use client";

import { FormEvent, useEffect, useState } from "react";
import { Armchair, Boxes, Calculator, ScanLine } from "lucide-react";
import { engineLabels, request, type AnalysisProvider, type RoomDesignCapability, type ServerRates } from "./api";
import { currencies, defaultSettings, designPalettes, designStyles, saveSettings, units, type Settings } from "./settings";

const rateFields: { key: keyof ServerRates; label: string; unit: string }[] = [
  { key: "plasterUsdPerSquareMeter", label: "Plaster", unit: "USD per m²" },
  { key: "paintUsdPerSquareMeter", label: "Paint", unit: "USD per m²" },
  { key: "tilingUsdPerSquareMeter", label: "Wall tiling", unit: "USD per m²" },
  { key: "flooringUsdPerSquareMeter", label: "Flooring", unit: "USD per m²" },
  { key: "ceilingUsdPerSquareMeter", label: "Ceiling", unit: "USD per m²" },
  { key: "skirtingUsdPerMeter", label: "Skirting", unit: "USD per m" },
];

type SettingsSection = "general" | "analysis" | "materials" | "design";
const sections: { id: SettingsSection; label: string; icon: typeof Calculator }[] = [
  { id: "general", label: "General", icon: Calculator },
  { id: "analysis", label: "Analyzer", icon: ScanLine },
  { id: "materials", label: "Material costing", icon: Boxes },
  { id: "design", label: "Design studio", icon: Armchair },
];

export default function SettingsView({ settings, engines, onSaved, onClose }: { settings: Settings; engines: string[]; onSaved: (settings: Settings) => void; onClose: () => void }) {
  const [draft, setDraft] = useState<Settings>(settings);
  const [section, setSection] = useState<SettingsSection>("general");
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  const [providers, setProviders] = useState<AnalysisProvider[]>([]);
  const [imageCapabilities, setImageCapabilities] = useState<RoomDesignCapability | null>(null);
  useEffect(() => {
    let active = true;
    request<AnalysisProvider[]>("/api/analysis-providers").then((result) => { if (active) setProviders(result); }).catch(() => {});
    request<RoomDesignCapability>("/api/room-design-capabilities").then((result) => { if (active) setImageCapabilities(result); }).catch(() => {});
    return () => { active = false; };
  }, []);
  const setRate = (key: keyof ServerRates, value: string) => { setSaved(false); setDraft((current) => ({ ...current, rates: { ...current.rates, [key]: Number(value) } })); };
  const setMaterialDefault = (key: keyof Settings["materialDefaults"], value: string) => { setSaved(false); setDraft((current) => ({ ...current, materialDefaults: { ...current.materialDefaults, [key]: Number(value) } })); };
  const availableEngines = Array.from(new Set(["demo", ...engines, ...providers.map((entry) => entry.id), draft.analysis.preferredEngine]));

  const submit = (event: FormEvent) => {
    event.preventDefault();
    const { rates } = draft;
    if (!(rates.plasterUsdPerSquareMeter > 0) || !(rates.paintUsdPerSquareMeter > 0)) { setError("Plaster and paint rates must be greater than zero."); return; }
    if (Object.values(rates).some((value) => !Number.isFinite(value) || value < 0)) { setError("Rates cannot be negative."); return; }
    if (rates.wastePercent > 50) { setError("Waste allowance must be between 0 and 50%."); return; }
    if (!(draft.exchangeRate > 0)) { setError("Enter an exchange rate greater than zero."); return; }
    if (!(draft.defaultWallHeight.value > 0)) { setError("Enter a default wall height greater than zero."); return; }
    const materialValues = Object.values(draft.materialDefaults);
    if (materialValues.some((value) => !Number.isFinite(value)) || draft.materialDefaults.wastePercent < 0 || draft.materialDefaults.wastePercent > 50 || !Number.isInteger(draft.materialDefaults.paintCoats) || draft.materialDefaults.paintCoats < 1 || draft.materialDefaults.paintCoats > 10 || draft.materialDefaults.paintCoverageM2PerLitrePerCoat <= 0 || draft.materialDefaults.paintPackLitres <= 0 || draft.materialDefaults.tilePackM2 <= 0) { setError("Material defaults must be positive, with waste at most 50% and paint coats from 1 to 10."); return; }
    setError("");
    saveSettings(draft);
    onSaved(draft);
    setSaved(true);
  };

  return <form className="card step-card" onSubmit={submit} noValidate>
    <header className="step-header"><div><p className="eyebrow">Preferences</p><h2>Settings</h2><p className="muted">Saved in this browser. Existing project, material, and estimate snapshots are unchanged.</p></div></header>
    <nav className="settings-sections" aria-label="Settings sections">{sections.map(({ id, label, icon: Icon }) => <button type="button" aria-current={section === id ? "page" : undefined} className={`settings-section${section === id ? " settings-section--active" : ""}`} key={id} onClick={() => setSection(id)}><Icon size={17} aria-hidden="true" /><span>{label}</span></button>)}</nav>

    {section === "general" && <section className="settings-panel" aria-label="General settings">
      <div><h3 className="section-title">Rate library</h3><p className="muted small">New estimates use these defaults. Existing versions keep their saved rates.</p></div>
      <div className="form-grid form-grid--three">
        {rateFields.map((field) => <label key={field.key} className="field"><span>{field.label} rate <small>{field.unit}</small></span><input type="number" min="0" step="0.01" value={draft.rates[field.key]} onChange={(event) => setRate(field.key, event.target.value)} /></label>)}
        <label className="field"><span>Waste allowance <small>% added to cost</small></span><input type="number" min="0" max="50" step="0.5" value={draft.rates.wastePercent} onChange={(event) => setRate("wastePercent", event.target.value)} /></label>
      </div>
      <h3 className="section-title">Currency</h3><div className="form-grid form-grid--three">
        <label className="field"><span>Currency</span><select value={draft.currency} onChange={(event) => { setSaved(false); setDraft({ ...draft, currency: event.target.value, exchangeRate: event.target.value === "USD" ? 1 : draft.exchangeRate }); }}>{currencies.map((currency) => <option key={currency}>{currency}</option>)}</select></label>
        <label className="field"><span>Exchange rate <small>1 USD =</small></span><input type="number" min="0" step="0.0001" disabled={draft.currency === "USD"} value={draft.currency === "USD" ? 1 : draft.exchangeRate} onChange={(event) => { setSaved(false); setDraft({ ...draft, exchangeRate: Number(event.target.value) }); }} /></label>
      </div>
      <h3 className="section-title">Measurements</h3><div className="form-grid form-grid--three">
        <label className="field"><span>Preferred unit <small>for new rooms</small></span><select value={draft.preferredUnit} onChange={(event) => { setSaved(false); setDraft({ ...draft, preferredUnit: event.target.value }); }}>{units.map((unit) => <option key={unit}>{unit}</option>)}</select></label>
        <label className="field"><span>Default wall height <small>used when the drawing has none</small></span><input type="number" min="0" step="0.01" value={draft.defaultWallHeight.value} onChange={(event) => { setSaved(false); setDraft({ ...draft, defaultWallHeight: { ...draft.defaultWallHeight, value: Number(event.target.value) } }); }} /></label>
        <label className="field"><span>Wall height unit</span><select value={draft.defaultWallHeight.unit} onChange={(event) => { setSaved(false); setDraft({ ...draft, defaultWallHeight: { ...draft.defaultWallHeight, unit: event.target.value } }); }}>{units.map((unit) => <option key={unit}>{unit}</option>)}</select></label>
      </div>
    </section>}

    {section === "analysis" && <section className="settings-panel" aria-label="Analyzer settings">
      <h3 className="section-title">Default analyzer</h3><p className="muted small">Choose the starting engine on new drawings. The extraction remains subject to manual confirmation.</p>
      <label className="field"><span>Analysis engine</span><select value={draft.analysis.preferredEngine} onChange={(event) => { setSaved(false); setDraft((current) => ({ ...current, analysis: { preferredEngine: event.target.value } })); }}>
        {availableEngines.map((engine) => <option key={engine} value={engine} disabled={engine !== "demo" && !engines.includes(engine)}>{engineLabels[engine] ?? engine}{providers.find((entry) => entry.id === engine)?.model ? ` - ${providers.find((entry) => entry.id === engine)!.model}` : ""}{engine !== "demo" && !engines.includes(engine) ? " (unavailable)" : ""}</option>)}
      </select></label>
      <p className="muted small">Available engines are supplied by the API. NVIDIA Nemotron 3.5 Super VL requires an NVIDIA API key, positive daily call limit, and shared quota scope on the API service.</p>
      {providers.length > 0 && <div className="provider-status-list">{providers.map((provider) => <div key={provider.id}><div><strong>{provider.label}</strong><span className="muted small">{provider.model || "No model configured"}</span></div><span className={`badge ${provider.enabled ? "badge--success" : "badge--warning"}`}>{provider.enabled ? "Enabled" : "Unavailable"}</span>{!provider.enabled && <p className="muted small">{provider.reason}</p>}</div>)}</div>}
    </section>}

    {section === "materials" && <section className="settings-panel" aria-label="Material costing settings">
      <h3 className="section-title">Material-only defaults</h3><p className="muted small">These are editable manual assumptions for new material budgets, not supplier specifications. Each budget stores its own reviewed values.</p>
      <div className="form-grid form-grid--three">
        <label className="field"><span>Material waste <small>%</small></span><input type="number" min="0" max="50" step="0.5" value={draft.materialDefaults.wastePercent} onChange={(event) => setMaterialDefault("wastePercent", event.target.value)} /></label>
        <label className="field"><span>Paint coats <small>count</small></span><input type="number" min="1" max="10" step="1" value={draft.materialDefaults.paintCoats} onChange={(event) => setMaterialDefault("paintCoats", event.target.value)} /></label>
        <label className="field"><span>Paint coverage <small>m²/litre/coat</small></span><input type="number" min="0.01" step="0.1" value={draft.materialDefaults.paintCoverageM2PerLitrePerCoat} onChange={(event) => setMaterialDefault("paintCoverageM2PerLitrePerCoat", event.target.value)} /></label>
        <label className="field"><span>Paint pack <small>litres</small></span><input type="number" min="0.01" step="0.1" value={draft.materialDefaults.paintPackLitres} onChange={(event) => setMaterialDefault("paintPackLitres", event.target.value)} /></label>
        <label className="field"><span>Tile pack coverage <small>m²</small></span><input type="number" min="0.01" step="0.1" value={draft.materialDefaults.tilePackM2} onChange={(event) => setMaterialDefault("tilePackM2", event.target.value)} /></label>
      </div>
    </section>}

    {section === "design" && <section className="settings-panel" aria-label="Design studio settings">
      {(imageCapabilities?.providers?.length ?? 0) > 0 && <><h3 className="section-title">Configured image providers</h3><div className="provider-status-list">{imageCapabilities!.providers!.map((provider) => <div key={provider.id}><div><strong>{provider.label}</strong><span className="muted small">{provider.model}</span></div><span className="badge">{provider.id === imageCapabilities!.defaultProvider ? "Server default" : "Configured"}</span></div>)}</div></>}
      <h3 className="section-title">Room concept defaults</h3><p className="muted small">Applied as starting choices in the Design Studio. Each generated concept remains attached to its confirmed room revision.</p>
      <div className="form-grid form-grid--three">
        <label className="field"><span>Interior style</span><select value={draft.designDefaults.style} onChange={(event) => { setSaved(false); setDraft((current) => ({ ...current, designDefaults: { ...current.designDefaults, style: event.target.value } })); }}>{designStyles.map((style) => <option key={style}>{style}</option>)}</select></label>
        <label className="field"><span>Color palette</span><select value={draft.designDefaults.palette} onChange={(event) => { setSaved(false); setDraft((current) => ({ ...current, designDefaults: { ...current.designDefaults, palette: event.target.value } })); }}>{designPalettes.map((palette) => <option key={palette}>{palette}</option>)}</select></label>
      </div>
      <p className="muted small">Only confirmed room dimensions and openings are sent for generation. Floor-plan images are not sent.</p>
    </section>}

    {error && <p className="alert alert--error" role="alert">{error}</p>}
    {saved && <p className="alert alert--success" role="status">Settings saved.</p>}
    <footer className="step-footer">
      <button type="button" className="button button--ghost" onClick={() => { setSaved(false); setDraft(defaultSettings); }}>Restore defaults</button>
      <span className="footer-actions"><button type="button" className="button button--ghost" onClick={onClose}>Close</button><button type="submit" className="button button--primary">Save settings</button></span>
    </footer>
  </form>;
}
