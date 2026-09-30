"use client";

import { FormEvent, useState } from "react";
import { API, formatMoney, formatNumber, jsonInit, request, type EstimateVersion, type Project, type Rates } from "./api";

const currencies = ["USD", "EUR", "GBP", "AUD", "CAD", "INR", "LKR", "AED"];

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

export default function EstimateStep({ project, analysisId, estimates, onEstimate, onBack }: { project: Project; analysisId: string; estimates: EstimateVersion[]; onEstimate: (version: EstimateVersion) => void; onBack: () => void }) {
  const latest = estimates[0];
  const [rates, setRates] = useState<Rates>({ plaster: latest ? latest.result.totalPlasterCostUsd / Math.max(latest.result.totalPlasterQuantity, 1e-9) : 10, paint: latest ? latest.result.totalPaintCostUsd / Math.max(latest.result.totalPaintQuantity, 1e-9) : 5, currency: latest?.targetCurrency ?? "USD", exchangeRate: latest?.exchangeRate ?? 1 });
  const [selectedId, setSelectedId] = useState(latest?.id ?? "");
  const [busy, setBusy] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [error, setError] = useState("");
  const selected = estimates.find((version) => version.id === selectedId) ?? latest;

  const calculate = async (event: FormEvent) => {
    event.preventDefault();
    if (!(rates.plaster > 0) || !(rates.paint > 0)) { setError("Enter plaster and paint rates greater than zero."); return; }
    if (!(rates.exchangeRate > 0)) { setError("Enter an exchange rate greater than zero."); return; }
    setBusy(true);
    setError("");
    try {
      const created = await request<EstimateVersion>(`/api/projects/${project.id}/estimates`, jsonInit("POST", {
        analysisId, plasterUsdPerSquareMeter: rates.plaster, paintUsdPerSquareMeter: rates.paint,
        targetCurrency: rates.currency, exchangeRate: rates.currency === "USD" ? 1 : rates.exchangeRate, exchangeRateFetchedAt: new Date().toISOString(),
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
    const header = ["Room", "Floor area (m2)", "Gross wall area (m2)", "Door deduction (m2)", "Window deduction (m2)", "Net plaster area (m2)", "Paint area (m2)", `Plaster cost (${selected.targetCurrency})`, `Paint cost (${selected.targetCurrency})`];
    const rows = selected.result.rooms.map((room) => [room.roomName, room.floorArea, room.grossWallArea, room.doorDeduction, room.windowDeduction, room.netPlasterArea, room.paintArea, room.plasterCostUsd * rate, room.paintCostUsd * rate].map((value) => typeof value === "number" ? value.toFixed(2) : value));
    rows.push(["Total", "", "", "", "", selected.result.totalPlasterQuantity.toFixed(2), selected.result.totalPaintQuantity.toFixed(2), selected.convertedPlasterCost.toFixed(2), selected.convertedPaintCost.toFixed(2)]);
    const csv = [header, ...rows].map((row) => row.map(csvCell).join(",")).join("\r\n");
    download(new Blob([csv], { type: "text/csv;charset=utf-8" }), `${slug(project.name)}-estimate-v${selected.version}.csv`);
  };

  return <section className="card step-card">
    <header className="step-header">
      <div><p className="eyebrow">Step 5</p><h2>Estimate</h2><p className="muted">Wall plaster and paint quantities from confirmed dimensions. Each calculation is saved as a new version.</p></div>
      {selected && <div className="export-actions no-print">
        <button type="button" className="button button--secondary" onClick={exportExcel} disabled={exporting}>{exporting && <span className="spinner" aria-hidden />}Export Excel</button>
        <button type="button" className="button button--ghost" onClick={exportCsv}>Export CSV</button>
        <button type="button" className="button button--ghost" onClick={() => window.print()}>Print / PDF</button>
      </div>}
    </header>

    <form className="rates-form no-print" onSubmit={calculate}>
      <label className="field"><span>Plaster rate <small>USD per m²</small></span><input type="number" min="0" step="0.01" value={Number.isFinite(rates.plaster) ? Number(rates.plaster.toFixed(4)) : ""} onChange={(event) => setRates({ ...rates, plaster: Number(event.target.value) })} /></label>
      <label className="field"><span>Paint rate <small>USD per m²</small></span><input type="number" min="0" step="0.01" value={Number.isFinite(rates.paint) ? Number(rates.paint.toFixed(4)) : ""} onChange={(event) => setRates({ ...rates, paint: Number(event.target.value) })} /></label>
      <label className="field"><span>Currency</span><select value={rates.currency} onChange={(event) => setRates({ ...rates, currency: event.target.value, exchangeRate: event.target.value === "USD" ? 1 : rates.exchangeRate })}>{currencies.map((currency) => <option key={currency}>{currency}</option>)}</select></label>
      <label className="field"><span>Exchange rate <small>1 USD =</small></span><input type="number" min="0" step="0.0001" value={rates.currency === "USD" ? 1 : rates.exchangeRate} disabled={rates.currency === "USD"} onChange={(event) => setRates({ ...rates, exchangeRate: Number(event.target.value) })} /></label>
      <button type="submit" className="button button--primary" disabled={busy}>{busy && <span className="spinner" aria-hidden />}{busy ? "Calculating…" : selected ? "Recalculate" : "Calculate estimate"}</button>
    </form>
    {error && <p className="alert alert--error" role="alert">{error}</p>}

    {!selected ? <div className="empty-state"><strong>No estimate yet</strong><span className="muted">Set your rates and calculate to see quantities and costs.</span></div> : <>
      <div className="print-only print-header"><h1>{project.name} — estimate v{selected.version}</h1><p>{[project.client, project.location].filter(Boolean).join(" · ")}</p></div>
      <div className="metric-cards">
        <div className="metric-card"><span>Plaster area</span><strong>{formatNumber(selected.result.totalPlasterQuantity)} m²</strong><small>{formatMoney(selected.convertedPlasterCost, selected.targetCurrency)}</small></div>
        <div className="metric-card"><span>Paint area</span><strong>{formatNumber(selected.result.totalPaintQuantity)} m²</strong><small>{formatMoney(selected.convertedPaintCost, selected.targetCurrency)}</small></div>
        <div className="metric-card metric-card--total"><span>Grand total</span><strong>{formatMoney(selected.convertedGrandTotal, selected.targetCurrency)}</strong><small>{selected.targetCurrency === "USD" ? "USD" : `${formatMoney(selected.result.grandTotalUsd, "USD")} at ${selected.exchangeRate}`}</small></div>
      </div>

      <div className="table-wrap">
        <table className="data-table data-table--numeric">
          <thead><tr><th>Room</th><th>Floor area</th><th>Gross wall</th><th>Doors</th><th>Windows</th><th>Net plaster</th><th>Paint</th><th>Cost</th></tr></thead>
          <tbody>{selected.result.rooms.map((room) => <tr key={room.roomId}>
            <td>{room.roomName}</td><td>{formatNumber(room.floorArea)}</td><td>{formatNumber(room.grossWallArea)}</td>
            <td>{room.doorDeduction > 0 ? `−${formatNumber(room.doorDeduction)}` : "—"}</td><td>{room.windowDeduction > 0 ? `−${formatNumber(room.windowDeduction)}` : "—"}</td>
            <td>{formatNumber(room.netPlasterArea)}</td><td>{formatNumber(room.paintArea)}</td>
            <td>{formatMoney((room.plasterCostUsd + room.paintCostUsd) * selected.exchangeRate, selected.targetCurrency)}</td>
          </tr>)}</tbody>
          <tfoot><tr><th>Total</th><td /><td /><td /><td /><td>{formatNumber(selected.result.totalPlasterQuantity)}</td><td>{formatNumber(selected.result.totalPaintQuantity)}</td><td>{formatMoney(selected.convertedGrandTotal, selected.targetCurrency)}</td></tr></tfoot>
        </table>
      </div>
      <p className="muted small">Areas in m². Calculated {new Date(selected.calculatedAt).toLocaleString()} · version {selected.version}.</p>

      {estimates.length > 1 && <div className="version-list no-print"><h3 className="section-title">Versions</h3>
        <ul>{estimates.map((version) => <li key={version.id}><button type="button" className={`version-item${version.id === selected.id ? " version-item--active" : ""}`} onClick={() => setSelectedId(version.id)}>
          <strong>v{version.version}</strong><span>{formatMoney(version.convertedGrandTotal, version.targetCurrency)}</span><small>{new Date(version.calculatedAt).toLocaleString()}</small>
        </button></li>)}</ul>
      </div>}
    </>}

    <footer className="step-footer no-print"><button type="button" className="button button--ghost" onClick={onBack}>Back to model</button><span className="muted small">Export includes Summary, Rooms and Openings worksheets.</span></footer>
  </section>;
}
