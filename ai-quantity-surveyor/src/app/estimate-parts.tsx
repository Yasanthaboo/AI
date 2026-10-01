"use client";

import { useState } from "react";
import { describeMeasurement, formatDate, formatMoney, formatNumber, roomTotal, tradeCost, tradeQuantity, trades, versionRates, type Confirmation, type EstimateVersion, type RoomFinish } from "./api";

const finishFlags = [
  { key: "excludePlaster", label: "Plaster" },
  { key: "excludePaint", label: "Paint" },
  { key: "excludeFlooring", label: "Flooring" },
  { key: "excludeCeiling", label: "Ceiling" },
  { key: "excludeSkirting", label: "Skirting" },
] as const;

export function describeFinish(finish: RoomFinish) {
  const parts = finishFlags.filter((flag) => finish[flag.key]).map((flag) => `no ${flag.label.toLowerCase()}`);
  if (finish.tilePercent) parts.push(`${finish.tilePercent}% wall tiling`);
  return parts.join(", ");
}

export function FinishesEditor({ confirmation, finishes, onChange }: { confirmation: Confirmation; finishes: Record<string, RoomFinish>; onChange: (finishes: Record<string, RoomFinish>) => void }) {
  const update = (roomId: string, patch: RoomFinish) => onChange({ ...finishes, [roomId]: { ...finishes[roomId], ...patch } });
  const customized = Object.values(finishes).filter((finish) => describeFinish(finish)).length;
  return <details className="info-panel no-print">
    <summary><strong>Finishes per room</strong><span className="muted"> · {customized ? `${customized} room${customized === 1 ? "" : "s"} customized` : "all finishes in every room"}</span></summary>
    <div className="table-wrap">
      <table className="data-table data-table--compact finishes-table">
        <thead><tr><th>Room</th>{finishFlags.map((flag) => <th key={flag.key}>{flag.label}</th>)}<th>Wall tiling %</th></tr></thead>
        <tbody>{confirmation.candidate.rooms.map((room) => {
          const finish = finishes[room.id] ?? {};
          return <tr key={room.id}>
            <td>{room.name}</td>
            {finishFlags.map((flag) => <td key={flag.key}><input type="checkbox" aria-label={`${room.name} ${flag.label.toLowerCase()}`} checked={!finish[flag.key]} onChange={(event) => update(room.id, { [flag.key]: !event.target.checked })} /></td>)}
            <td><input type="number" min="0" max="100" step="5" aria-label={`${room.name} wall tiling percent`} value={finish.tilePercent ?? 0} onChange={(event) => update(room.id, { tilePercent: Math.min(100, Math.max(0, Number(event.target.value) || 0)) })} /></td>
          </tr>;
        })}</tbody>
      </table>
    </div>
    <p className="muted small">Tiled wall area is removed from the paint area. Excluded items have zero quantity and cost for that room.</p>
  </details>;
}

function Bars({ title, items, currency }: { title: string; items: { label: string; value: number }[]; currency: string }) {
  const max = Math.max(...items.map((item) => item.value), 0);
  return <figure className="chart">
    <figcaption>{title}</figcaption>
    <ul className="bar-list">{items.map((item) => <li key={item.label} className="bar-row">
      <span className="bar-row__label">{item.label}</span>
      <span className="bar-row__track" aria-hidden><span className="bar-row__fill" style={{ width: `${max > 0 ? (item.value / max) * 100 : 0}%` }} /></span>
      <span className="bar-row__value">{formatMoney(item.value, currency)}</span>
    </li>)}</ul>
  </figure>;
}

export function CostCharts({ version }: { version: EstimateVersion }) {
  const byTrade = trades.map((trade) => ({ label: trade.label, value: tradeCost(version, trade) })).filter((item) => item.value > 0);
  const byRoom = version.result.rooms.map((room) => ({ label: room.roomName, value: roomTotal(room) * version.exchangeRate })).sort((left, right) => right.value - left.value);
  return <div className="chart-grid">
    <Bars title="Cost by trade" items={byTrade} currency={version.targetCurrency} />
    <Bars title="Cost by room" items={byRoom} currency={version.targetCurrency} />
  </div>;
}

export function Assumptions({ version, confirmation }: { version: EstimateVersion; confirmation: Confirmation }) {
  const rates = versionRates(version);
  const items: string[] = [];
  for (const room of confirmation.candidate.rooms) {
    if (room.assumptions?.includes("wallHeight")) items.push(`${room.name}: wall height ${describeMeasurement(room.wallHeight)} is the configured default, not read from the drawing.`);
  }
  const approximate = confirmation.candidate.rooms.filter((room) => !room.bounds).map((room) => room.name);
  if (approximate.length) items.push(`Placed approximately in the 3D model (no boundary detected): ${approximate.join(", ")}. Quantities are unaffected.`);
  items.push(version.targetCurrency === "USD" ? "Costs are in USD; no currency conversion." : `Exchange rate 1 USD = ${version.exchangeRate} ${version.targetCurrency}, entered manually ${formatDate(version.exchangeRateFetchedAt)}.`);
  const priced = trades.filter((trade) => rates[trade.rate] > 0);
  items.push(`Rates (USD): ${priced.map((trade) => `${trade.label.toLowerCase()} ${formatNumber(rates[trade.rate])}/${trade.unit}`).join(", ")}.`);
  const unpriced = trades.filter((trade) => !(rates[trade.rate] > 0));
  if (unpriced.length) items.push(`Not priced (rate 0): ${unpriced.map((trade) => trade.label.toLowerCase()).join(", ")}.`);
  items.push(rates.wastePercent > 0 ? `Waste allowance of ${rates.wastePercent}% is added to every cost; quantities are net.` : "No waste allowance; costs use net quantities.");
  for (const room of version.result.rooms) {
    const finish = version.finishes?.[room.roomId];
    const description = finish && describeFinish(finish);
    if (description) items.push(`${room.roomName}: ${description}.`);
  }
  items.push("Door and window sizes are deducted from wall areas; skirting excludes door widths.");
  if (!version.rates) items.push("This version predates stored rates; rates shown are derived from its costs.");
  return <section className="assumptions" aria-labelledby="assumptions-title">
    <h3 id="assumptions-title" className="section-title">Assumptions</h3>
    <ul>{items.map((item) => <li key={item}>{item}</li>)}</ul>
  </section>;
}

function Delta({ before, after, format }: { before: number; after: number; format: (value: number) => string }) {
  const change = after - before;
  if (Math.abs(change) < 0.005) return <span className="muted">—</span>;
  const percent = before !== 0 ? ` (${change > 0 ? "+" : ""}${((change / before) * 100).toFixed(1)}%)` : "";
  return <span className={change > 0 ? "delta delta--up" : "delta delta--down"}>{change > 0 ? "+" : "−"}{format(Math.abs(change))}{percent}</span>;
}

export function CompareVersions({ versions }: { versions: EstimateVersion[] }) {
  const [leftId, setLeftId] = useState(versions[1]?.id ?? "");
  const [rightId, setRightId] = useState(versions[0]?.id ?? "");
  const left = versions.find((version) => version.id === leftId);
  const right = versions.find((version) => version.id === rightId);
  if (versions.length < 2) return null;
  const sameCurrency = left && right && left.targetCurrency === right.targetCurrency;
  const currency = sameCurrency ? left.targetCurrency : "USD";
  const money = (version: EstimateVersion, usd: number) => (sameCurrency ? usd * version.exchangeRate : usd);
  const format = (value: number) => formatMoney(value, currency);
  const picker = (label: string, value: string, set: (id: string) => void) => <label className="field field--inline"><span>{label}</span>
    <select aria-label={`Compare ${label.toLowerCase()}`} value={value} onChange={(event) => set(event.target.value)}>{versions.map((version) => <option key={version.id} value={version.id}>v{version.version} · {formatMoney(version.convertedGrandTotal, version.targetCurrency)}</option>)}</select></label>;

  return <details className="info-panel compare no-print">
    <summary><strong>Compare versions</strong></summary>
    <div className="toolbar">{picker("Base", leftId, setLeftId)}{picker("Against", rightId, setRightId)}</div>
    {left && right && <div className="table-wrap">
      <table className="data-table data-table--compact compare-table">
        <thead><tr><th>Item</th><th>v{left.version}</th><th>v{right.version}</th><th>Change</th></tr></thead>
        <tbody>
          {trades.map((trade) => {
            const before = versionRates(left)[trade.rate];
            const after = versionRates(right)[trade.rate];
            const costBefore = money(left, left.result[trade.cost] ?? 0);
            const costAfter = money(right, right.result[trade.cost] ?? 0);
            if (!before && !after && !costBefore && !costAfter) return null;
            return [
              <tr key={`${trade.key}-q`}><td>{trade.label} quantity</td><td>{formatNumber(tradeQuantity(left, trade))} {trade.unit}</td><td>{formatNumber(tradeQuantity(right, trade))} {trade.unit}</td><td><Delta before={tradeQuantity(left, trade)} after={tradeQuantity(right, trade)} format={(value) => `${formatNumber(value)} ${trade.unit}`} /></td></tr>,
              <tr key={`${trade.key}-r`}><td>{trade.label} rate (USD)</td><td>{formatNumber(before)}</td><td>{formatNumber(after)}</td><td><Delta before={before} after={after} format={(value) => formatNumber(value)} /></td></tr>,
              <tr key={`${trade.key}-c`}><td>{trade.label} cost</td><td>{format(costBefore)}</td><td>{format(costAfter)}</td><td><Delta before={costBefore} after={costAfter} format={format} /></td></tr>,
            ];
          })}
          <tr><td>Waste allowance</td><td>{versionRates(left).wastePercent}%</td><td>{versionRates(right).wastePercent}%</td><td><Delta before={versionRates(left).wastePercent} after={versionRates(right).wastePercent} format={(value) => `${value} pts`} /></td></tr>
          {right.result.rooms.map((room) => {
            const before = left.result.rooms.find((item) => item.roomId === room.roomId);
            const costBefore = before ? money(left, roomTotal(before)) : 0;
            const costAfter = money(right, roomTotal(room));
            return <tr key={`room-${room.roomId}`}><td>{room.roomName}</td><td>{before ? format(costBefore) : "—"}</td><td>{format(costAfter)}</td><td><Delta before={costBefore} after={costAfter} format={format} /></td></tr>;
          })}
        </tbody>
        <tfoot><tr><th>Grand total</th><td>{format(money(left, left.result.grandTotalUsd))}</td><td>{format(money(right, right.result.grandTotalUsd))}</td><td><Delta before={money(left, left.result.grandTotalUsd)} after={money(right, right.result.grandTotalUsd)} format={format} /></td></tr></tfoot>
      </table>
    </div>}
    {!sameCurrency && <p className="muted small">The versions use different currencies, so they are compared in USD.</p>}
  </details>;
}
