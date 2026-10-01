"use client";

import { FormEvent, useState } from "react";
import { API, formatMoney, formatNumber, jsonInit, request, roomTotal, tradeCost, tradeQuantity, trades, versionRates, type Confirmation, type EstimateVersion, type Project, type RoomFinish, type ServerRates } from "./api";
import { Assumptions, CompareVersions, CostCharts, FinishesEditor, describeFinish } from "./estimate-parts";
import { currencies, type Settings } from "./settings";

type Form = { rates: ServerRates; currency: string; exchangeRate: number };

const rateFields: { key: keyof ServerRates; label: string; unit: string }[] = [
  { key: "plasterUsdPerSquareMeter", label: "Plaster rate", unit: "USD per m²" },
  { key: "paintUsdPerSquareMeter", label: "Paint rate", unit: "USD per m²" },
  { key: "tilingUsdPerSquareMeter", label: "Wall tiling rate", unit: "USD per m²" },
  { key: "flooringUsdPerSquareMeter", label: "Flooring rate", unit: "USD per m²" },
  { key: "ceilingUsdPerSquareMeter", label: "Ceiling rate", unit: "USD per m²" },
  { key: "skirtingUsdPerMeter", label: "Skirting rate", unit: "USD per m" },
  { key: "wastePercent", label: "Waste allowance", unit: "% added to cost" },
];

function slug(value: string) {
  return value.trim().toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "") || "project";
}

function download(blob: Blob, name: string) {
  const link = document.createElement("a");
  link.href = URL.createObjectURL(blob);
  link.download = name;
  link.click();
  URL.revokeObjectURL(link.href);
}

function csvCell(value: string | number) {
  const text = String(value);
  return /[",\n]/.test(text) || /^[=+\-@]/.test(text) ? `"${text.replace(/^([=+\-@])/, "'$1").replace(/"/g, '""')}"` : text;
}

const round = (value: number) => Number(value.toFixed(4));

export default function EstimateStep({ project, analysisId, confirmation, estimates, settings, onEstimate, onBack }: {
  project: Project;
  analysisId: string;
  confirmation: Confirmation;
  estimates: EstimateVersion[];
  settings: Settings;
  onEstimate: (version: EstimateVersion) => void;
  onBack: () => void;
}) {
  const latest = estimates[0];
  const [form, setForm] = useState<Form>(() => latest
    ? { rates: Object.fromEntries(Object.entries(versionRates(latest)).map(([key, value]) => [key, round(value)])) as ServerRates, currency: latest.targetCurrency, exchangeRate: latest.exchangeRate }
    : { rates: settings.rates, currency: settings.currency, exchangeRate: settings.exchangeRate });
  const [finishes, setFinishes] = useState<Record<string, RoomFinish>>(latest?.finishes ?? {});
  const [selectedId, setSelectedId] = useState(latest?.id ?? "");
  const [busy, setBusy] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [error, setError] = useState("");
  const selected = estimates.find((version) => version.id === selectedId) ?? latest;

  const calculate = async (event: FormEvent) => {
    event.preventDefault();
    const { rates } = form;
    if (!(rates.plasterUsdPerSquareMeter > 0) || !(rates.paintUsdPerSquareMeter > 0)) { setError("Enter plaster and paint rates greater than zero."); return; }
    if (Object.values(rates).some((value) => !Number.isFinite(value) || value < 0)) { setError("Rates cannot be negative."); return; }
    if (rates.wastePercent > 50) { setError("Waste allowance must be between 0 and 50%."); return; }
    if (!(form.exchangeRate > 0)) { setError("Enter an exchange rate greater than zero."); return; }
    setBusy(true);
    setError("");
    try {
      const custom = Object.fromEntries(Object.entries(finishes).filter(([, finish]) => describeFinish(finish)));
      const created = await request<EstimateVersion>(`/api/projects/${project.id}/estimates`, jsonInit("POST", {
        analysisId, ...rates, finishes: custom,
        targetCurrency: form.currency, exchangeRate: form.currency === "USD" ? 1 : form.exchangeRate, exchangeRateFetchedAt: new Date().toISOString(),
      }));
      onEstimate(created);
      setSelectedId(created.id);
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "The estimate could not be calculated.");
    } finally {
      setBusy(false);
    }
  };

  const exportExcel = async () => {
    if (!selected) return;
    setExporting(true);
    setError("");
    try {
      const response = await fetch(`${API}/api/estimates/${selected.id}/export`);
      if (!response.ok) throw new Error("Excel export failed.");
      download(await response.blob(), `${slug(project.name)}-estimate-v${selected.version}.xlsx`);
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "Excel export failed.");
    } finally {
      setExporting(false);
    }
  };

  const exportCsv = () => {
    if (!selected) return;
    const rate = selected.exchangeRate;
    const money = `(${selected.targetCurrency})`;
    const header = ["Room", "Floor area (m2)", "Gross wall area (m2)", "Door deduction (m2)", "Window deduction (m2)", "Net plaster area (m2)", "Paint area (m2)", "Wall tile area (m2)", "Flooring area (m2)", "Ceiling area (m2)", "Skirting length (m)", `Plaster cost ${money}`, `Paint cost ${money}`, `Tiling cost ${money}`, `Flooring cost ${money}`, `Ceiling cost ${money}`, `Skirting cost ${money}`, `Room total ${money}`];
    const rows = selected.result.rooms.map((room) => [room.roomName, room.floorArea, room.grossWallArea, room.doorDeduction, room.windowDeduction, room.netPlasterArea, room.paintArea, room.tileArea ?? 0, room.flooringArea ?? 0, room.ceilingArea ?? 0, room.skirtingLength ?? 0,
      room.plasterCostUsd * rate, room.paintCostUsd * rate, (room.tilingCostUsd ?? 0) * rate, (room.flooringCostUsd ?? 0) * rate, (room.ceilingCostUsd ?? 0) * rate, (room.skirtingCostUsd ?? 0) * rate, roomTotal(room) * rate]
      .map((value) => typeof value === "number" ? value.toFixed(2) : value));
    rows.push(["Total", "", "", "", "", ...trades.map((trade) => tradeQuantity(selected, trade).toFixed(2)), ...trades.map((trade) => tradeCost(selected, trade).toFixed(2)), selected.convertedGrandTotal.toFixed(2)]);
    const csv = [header, ...rows].map((row) => row.map(csvCell).join(",")).join("\r\n");
    download(new Blob([csv], { type: "text/csv;charset=utf-8" }), `${slug(project.name)}-estimate-v${selected.version}.csv`);
  };

  const pricedTrades = selected ? trades.filter((trade) => tradeQuantity(selected, trade) > 0 || tradeCost(selected, trade) > 0) : [];

  return <section className="card step-card">
    <header className="step-header">
      <div><p className="eyebrow">Step 5</p><h2>Estimate</h2><p className="muted">Interior finishes quantities from confirmed dimensions. Each calculation is saved as a new version.</p></div>
      {selected && <div className="export-actions no-print">
        <button type="button" className="button button--secondary" onClick={exportExcel} disabled={exporting}>{exporting && <span className="spinner" aria-hidden />}Export Excel</button>
        <button type="button" className="button button--ghost" onClick={exportCsv}>Export CSV</button>
        <button type="button" className="button button--ghost" onClick={() => window.print()}>Print / PDF</button>
      </div>}
    </header>

    <form className="rates-form no-print" onSubmit={calculate}>
      <div className="rates-grid">
        {rateFields.map((field) => <label key={field.key} className="field"><span>{field.label} <small>{field.unit}</small></span>
          <input type="number" min="0" max={field.key === "wastePercent" ? 50 : undefined} step="0.01" value={Number.isFinite(form.rates[field.key]) ? form.rates[field.key] : ""} onChange={(event) => setForm({ ...form, rates: { ...form.rates, [field.key]: Number(event.target.value) } })} /></label>)}
        <label className="field"><span>Currency</span><select value={form.currency} onChange={(event) => setForm({ ...form, currency: event.target.value, exchangeRate: event.target.value === "USD" ? 1 : form.exchangeRate })}>{currencies.map((currency) => <option key={currency}>{currency}</option>)}</select></label>
        <label className="field"><span>Exchange rate <small>1 USD =</small></span><input type="number" min="0" step="0.0001" value={form.currency === "USD" ? 1 : form.exchangeRate} disabled={form.currency === "USD"} onChange={(event) => setForm({ ...form, exchangeRate: Number(event.target.value) })} /></label>
      </div>
      <FinishesEditor confirmation={confirmation} finishes={finishes} onChange={setFinishes} />
      <div className="rates-actions"><span className="muted small">Defaults come from Settings. Unpriced trades (rate 0) are still measured.</span>
        <button type="submit" className="button button--primary" disabled={busy}>{busy && <span className="spinner" aria-hidden />}{busy ? "Calculating…" : selected ? "Recalculate" : "Calculate estimate"}</button></div>
    </form>
    {error && <p className="alert alert--error" role="alert">{error}</p>}

    {!selected ? <div className="empty-state"><strong>No estimate yet</strong><span className="muted">Set your rates and calculate to see quantities and costs.</span></div> : <>
      <div className="print-only print-header"><h1>{project.name} — estimate v{selected.version}</h1><p>{[project.client, project.location].filter(Boolean).join(" · ")}</p></div>
      <div className="metric-cards">
        {pricedTrades.slice(0, 3).map((trade) => <div key={trade.key} className="metric-card"><span>{trade.label}</span><strong>{formatNumber(tradeQuantity(selected, trade))} {trade.unit}</strong><small>{formatMoney(tradeCost(selected, trade), selected.targetCurrency)}</small></div>)}
        <div className="metric-card metric-card--total"><span>Grand total</span><strong>{formatMoney(selected.convertedGrandTotal, selected.targetCurrency)}</strong><small>{selected.targetCurrency === "USD" ? "USD" : `${formatMoney(selected.result.grandTotalUsd, "USD")} at ${selected.exchangeRate}`}</small></div>
      </div>

      <div className="table-wrap">
        <table className="data-table trade-table">
          <thead><tr><th>Trade</th><th>Quantity</th><th>Rate (USD)</th><th>Cost</th></tr></thead>
          <tbody>{trades.map((trade) => {
            const rate = versionRates(selected)[trade.rate];
            return <tr key={trade.key} className={rate > 0 ? undefined : "row--muted"}><td>{trade.label}</td><td>{formatNumber(tradeQuantity(selected, trade))} {trade.unit}</td><td>{rate > 0 ? `${formatNumber(rate)} / ${trade.unit}` : "Not priced"}</td><td>{formatMoney(tradeCost(selected, trade), selected.targetCurrency)}</td></tr>;
          })}</tbody>
          <tfoot><tr><th>Total{versionRates(selected).wastePercent > 0 ? ` (incl. ${versionRates(selected).wastePercent}% waste)` : ""}</th><td /><td /><td>{formatMoney(selected.convertedGrandTotal, selected.targetCurrency)}</td></tr></tfoot>
        </table>
      </div>

      <CostCharts version={selected} />

      <div className="table-wrap">
        <table className="data-table data-table--numeric">
          <thead><tr><th>Room</th><th>Floor area</th><th>Net wall</th><th>Openings</th><th>Plaster</th><th>Paint</th><th>Tiles</th><th>Skirting (m)</th><th>Cost</th></tr></thead>
          <tbody>{selected.result.rooms.map((room) => {
            const deductions = room.doorDeduction + room.windowDeduction;
            return <tr key={room.roomId}>
              <td>{room.roomName}</td><td>{formatNumber(room.floorArea)}</td><td>{formatNumber(room.netWallArea ?? room.grossWallArea - deductions)}</td>
              <td>{deductions > 0 ? `−${formatNumber(deductions)}` : "—"}</td>
              <td>{formatNumber(room.netPlasterArea)}</td><td>{formatNumber(room.paintArea)}</td><td>{room.tileArea ? formatNumber(room.tileArea) : "—"}</td><td>{formatNumber(room.skirtingLength ?? 0)}</td>
              <td>{formatMoney(roomTotal(room) * selected.exchangeRate, selected.targetCurrency)}</td>
            </tr>;
          })}</tbody>
          <tfoot><tr><th>Total</th><td>{formatNumber(selected.result.rooms.reduce((sum, room) => sum + room.floorArea, 0))}</td><td /><td /><td>{formatNumber(selected.result.totalPlasterQuantity)}</td><td>{formatNumber(selected.result.totalPaintQuantity)}</td><td>{formatNumber(selected.result.totalTilingQuantity ?? 0)}</td><td>{formatNumber(selected.result.totalSkirtingQuantity ?? 0)}</td><td>{formatMoney(selected.convertedGrandTotal, selected.targetCurrency)}</td></tr></tfoot>
        </table>
      </div>
      <p className="muted small">Areas in m². Calculated {new Date(selected.calculatedAt).toLocaleString()} · version {selected.version}.</p>

      <Assumptions version={selected} confirmation={confirmation} />

      {estimates.length > 1 && <div className="version-list no-print"><h3 className="section-title">Versions</h3>
        <ul>{estimates.map((version) => <li key={version.id}><button type="button" className={`version-item${version.id === selected.id ? " version-item--active" : ""}`} onClick={() => setSelectedId(version.id)}>
          <strong>v{version.version}</strong><span>{formatMoney(version.convertedGrandTotal, version.targetCurrency)}</span><small>{new Date(version.calculatedAt).toLocaleString()}</small>
        </button></li>)}</ul>
      </div>}
      <CompareVersions key={estimates.length} versions={estimates} />
    </>}

    <footer className="step-footer no-print"><button type="button" className="button button--ghost" onClick={onBack}>Back to model</button><span className="muted small">Excel export includes Summary, Rooms, Openings and Assumptions worksheets.</span></footer>
  </section>;
}
