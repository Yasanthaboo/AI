"use client";

import { FormEvent, useState } from "react";
import type { ServerRates } from "./api";
import { currencies, defaultSettings, saveSettings, units, type Settings } from "./settings";

const rateFields: { key: keyof ServerRates; label: string; unit: string }[] = [
  { key: "plasterUsdPerSquareMeter", label: "Plaster", unit: "USD per m²" },
  { key: "paintUsdPerSquareMeter", label: "Paint", unit: "USD per m²" },
  { key: "tilingUsdPerSquareMeter", label: "Wall tiling", unit: "USD per m²" },
  { key: "flooringUsdPerSquareMeter", label: "Flooring", unit: "USD per m²" },
  { key: "ceilingUsdPerSquareMeter", label: "Ceiling", unit: "USD per m²" },
  { key: "skirtingUsdPerMeter", label: "Skirting", unit: "USD per m" },
];

export default function SettingsView({ settings, onSaved, onClose }: { settings: Settings; onSaved: (settings: Settings) => void; onClose: () => void }) {
  const [draft, setDraft] = useState<Settings>(settings);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  const setRate = (key: keyof ServerRates, value: string) => { setSaved(false); setDraft((current) => ({ ...current, rates: { ...current.rates, [key]: Number(value) } })); };

  const submit = (event: FormEvent) => {
    event.preventDefault();
    const { rates } = draft;
    if (!(rates.plasterUsdPerSquareMeter > 0) || !(rates.paintUsdPerSquareMeter > 0)) { setError("Plaster and paint rates must be greater than zero."); return; }
    if (Object.values(rates).some((value) => !Number.isFinite(value) || value < 0)) { setError("Rates cannot be negative."); return; }
    if (rates.wastePercent > 50) { setError("Waste allowance must be between 0 and 50%."); return; }
    if (!(draft.exchangeRate > 0)) { setError("Enter an exchange rate greater than zero."); return; }
    if (!(draft.defaultWallHeight.value > 0)) { setError("Enter a default wall height greater than zero."); return; }
    setError("");
    saveSettings(draft);
    onSaved(draft);
    setSaved(true);
  };

  return <form className="card step-card" onSubmit={submit} noValidate>
    <header className="step-header"><div><p className="eyebrow">Settings</p><h2>Rates and defaults</h2><p className="muted">Saved in this browser. New estimates start from these rates; existing estimate versions keep the rates they were calculated with.</p></div></header>

    <h3 className="section-title">Rate library</h3>
    <div className="form-grid form-grid--three">
      {rateFields.map((field) => <label key={field.key} className="field"><span>{field.label} rate <small>{field.unit}</small></span>
        <input type="number" min="0" step="0.01" value={draft.rates[field.key]} onChange={(event) => setRate(field.key, event.target.value)} /></label>)}
      <label className="field"><span>Waste allowance <small>% added to cost</small></span><input type="number" min="0" max="50" step="0.5" value={draft.rates.wastePercent} onChange={(event) => setRate("wastePercent", event.target.value)} /></label>
    </div>

    <h3 className="section-title">Currency</h3>
    <div className="form-grid form-grid--three">
      <label className="field"><span>Currency</span><select value={draft.currency} onChange={(event) => { setSaved(false); setDraft({ ...draft, currency: event.target.value, exchangeRate: event.target.value === "USD" ? 1 : draft.exchangeRate }); }}>{currencies.map((currency) => <option key={currency}>{currency}</option>)}</select></label>
      <label className="field"><span>Exchange rate <small>1 USD =</small></span><input type="number" min="0" step="0.0001" disabled={draft.currency === "USD"} value={draft.currency === "USD" ? 1 : draft.exchangeRate} onChange={(event) => { setSaved(false); setDraft({ ...draft, exchangeRate: Number(event.target.value) }); }} /></label>
    </div>

    <h3 className="section-title">Measurements</h3>
    <div className="form-grid form-grid--three">
      <label className="field"><span>Preferred unit <small>for new rooms</small></span><select value={draft.preferredUnit} onChange={(event) => { setSaved(false); setDraft({ ...draft, preferredUnit: event.target.value }); }}>{units.map((unit) => <option key={unit}>{unit}</option>)}</select></label>
      <label className="field"><span>Default wall height <small>used when the drawing has none</small></span><input type="number" min="0" step="0.01" value={draft.defaultWallHeight.value} onChange={(event) => { setSaved(false); setDraft({ ...draft, defaultWallHeight: { ...draft.defaultWallHeight, value: Number(event.target.value) } }); }} /></label>
      <label className="field"><span>Wall height unit</span><select value={draft.defaultWallHeight.unit} onChange={(event) => { setSaved(false); setDraft({ ...draft, defaultWallHeight: { ...draft.defaultWallHeight, unit: event.target.value } }); }}>{units.map((unit) => <option key={unit}>{unit}</option>)}</select></label>
    </div>

    {error && <p className="alert alert--error" role="alert">{error}</p>}
    {saved && <p className="alert alert--success" role="status">Settings saved.</p>}
    <footer className="step-footer">
      <button type="button" className="button button--ghost" onClick={() => { setSaved(false); setDraft(defaultSettings); }}>Restore defaults</button>
      <span className="footer-actions"><button type="button" className="button button--ghost" onClick={onClose}>Close</button><button type="submit" className="button button--primary">Save settings</button></span>
    </footer>
  </form>;
}
