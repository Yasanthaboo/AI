"use client";

import { useEffect, useState } from "react";
import { API, formatDate, formatMoney, formatNumber, jsonInit, request, type EstimateVersion, type MaterialAllocation, type MaterialBudget, type MaterialComparison, type MaterialOffer, type MaterialPreview, type MaterialRecipe, type MaterialSelection } from "./api";

const labels: Record<string, string> = { paint: "Paint", wallTile: "Wall tiles", floorTile: "Floor tiles", plaster: "Cement + sand plaster", plasterCement: "Cement", plasterSand: "Sand", premixPlaster: "Premixed plaster" };
const initialPaint: MaterialRecipe = { category: "paint", specification: "Interior matt", coats: 2, coverageM2PerLitrePerCoat: 10, packAmount: 5, packUnit: "litre", inputOrigin: "manual" };
const initialTile: MaterialRecipe = { category: "tile", specification: "Floor tiles", packAmount: 1.5, packUnit: "m2", inputOrigin: "manual" };
const emptyMix = { cementSpecification: "", sandSpecification: "", thicknessMm: "", cementParts: "", sandParts: "", dryVolumeFactor: "", cementDensityKgPerM3: "", cementPackAmountKg: "", sandPackAmountM3: "" };
const emptyPremix = { specification: "", thicknessMm: "", consumptionPerM2: "", packAmount: "" };
const positive = (value: string) => Number.isFinite(Number(value)) && Number(value) > 0;

export default function MaterialBudgetPanel({ estimate, projectId }: { estimate: EstimateVersion; projectId: string }) {
  const [budgets, setBudgets] = useState<MaterialBudget[]>([]);
  const [budgetId, setBudgetId] = useState("");
  const [offers, setOffers] = useState<MaterialOffer[]>([]);
  const [plasterMaterialsEnabled, setPlasterMaterialsEnabled] = useState(false);
  const [paint, setPaint] = useState(initialPaint);
  const [tile, setTile] = useState(initialTile);
  const [wallTileSpecification, setWallTileSpecification] = useState("Wall tiles");
  const [plasterMix, setPlasterMix] = useState(emptyMix);
  const [premix, setPremix] = useState(emptyPremix);
  const [waste, setWaste] = useState(10);
  const [allocations, setAllocations] = useState<Record<string, boolean>>({});
  const [reviewed, setReviewed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [preview, setPreview] = useState<MaterialPreview | null>(null);
  const [choices, setChoices] = useState<MaterialSelection[][]>([]);
  const [saved, setSaved] = useState<MaterialComparison | null>(null);
  const [comparisonId, setComparisonId] = useState("");
  const [locationChoice, setLocationChoice] = useState("");
  const budget = budgets.find((entry) => entry.id === budgetId) ?? budgets[0];
  const sourceReady = !!estimate.sourceConfirmationRevision && !!estimate.sourceFloorPlanId && !!estimate.sourcePageNumber && !!estimate.sourceImageDigest;
  const pricedRequirements = budget?.requirements.filter((requirement) => requirement.requiredAmount > 0) ?? [];
  const fixturesCoverRequirements = pricedRequirements.length > 0 && pricedRequirements.every((requirement) => offers.some((offer) => offer.recipe.category === requirement.recipe.category && offer.recipe.specification === requirement.recipe.specification && (requirement.trade !== "paint" || offer.recipe.coats === requirement.recipe.coats)));
  const location = { country: locationChoice === "LK/Colombo" ? "LK" : "", city: locationChoice === "LK/Colombo" ? "Colombo" : "" };
  const selectableTrades: MaterialAllocation["trade"][] = ["paint", "wallTile", "floorTile"];
  if (plasterMaterialsEnabled) selectableTrades.push("plaster", "premixPlaster");

  useEffect(() => {
    let active = true;
    Promise.all([request<MaterialBudget[]>(`/api/projects/${encodeURIComponent(projectId)}/material-budgets`), request<MaterialOffer[]>("/api/material-offers?country=LK&city=Colombo"), request<{ plasterMaterials: boolean }>("/api/material-capabilities")])
      .then(([versions, catalog, capabilities]) => { if (active) { const matching = versions.filter((version) => version.estimateId === estimate.id); setBudgets(matching); if (matching.length) setBudgetId(matching[0].id); setOffers(catalog); setPlasterMaterialsEnabled(capabilities.plasterMaterials); } })
      .catch((failure) => { if (active) setError(failure instanceof Error ? failure.message : "Material data unavailable."); });
    return () => { active = false; };
  }, [estimate.id, projectId]);

  const create = async () => {
    const entries: MaterialAllocation[] = [];
    const mixRecipe: MaterialRecipe = { category: "plasterMix", specification: "Traditional site mix", cementSpecification: plasterMix.cementSpecification.trim(), sandSpecification: plasterMix.sandSpecification.trim(), thicknessMm: Number(plasterMix.thicknessMm), cementParts: Number(plasterMix.cementParts), sandParts: Number(plasterMix.sandParts), dryVolumeFactor: Number(plasterMix.dryVolumeFactor), cementDensityKgPerM3: Number(plasterMix.cementDensityKgPerM3), cementPackAmountKg: Number(plasterMix.cementPackAmountKg), sandPackAmountM3: Number(plasterMix.sandPackAmountM3), inputOrigin: "manual", packAmount: Number(plasterMix.cementPackAmountKg), packUnit: "kg" };
    const premixRecipe: MaterialRecipe = { category: "premixedPlaster", specification: premix.specification.trim(), thicknessMm: Number(premix.thicknessMm), consumptionPerM2: Number(premix.consumptionPerM2), packAmount: Number(premix.packAmount), packUnit: "kg", inputOrigin: "manual" };
    for (const room of estimate.result.rooms) {
      for (const trade of selectableTrades) {
        if (allocations[`${room.roomId}:${trade}`]) entries.push({ roomId: room.roomId, trade, recipe: trade === "paint" ? paint : trade === "wallTile" ? { ...tile, specification: wallTileSpecification } : trade === "floorTile" ? tile : trade === "plaster" ? mixRecipe : premixRecipe });
      }
    }
    if (!entries.length || !reviewed || (entries.some((entry) => entry.trade === "plaster") && (!mixRecipe.cementSpecification || !mixRecipe.sandSpecification || ![plasterMix.thicknessMm, plasterMix.cementParts, plasterMix.sandParts, plasterMix.dryVolumeFactor, plasterMix.cementDensityKgPerM3, plasterMix.cementPackAmountKg, plasterMix.sandPackAmountM3].every(positive))) || (entries.some((entry) => entry.trade === "premixPlaster") && (!premixRecipe.specification || ![premix.thicknessMm, premix.consumptionPerM2, premix.packAmount].every(positive)))) { setError("Complete every reviewed plaster recipe and select its plaster system per room."); return; }
    setBusy(true); setError("");
    try {
      const created = await request<MaterialBudget>(`/api/projects/${encodeURIComponent(projectId)}/material-budgets`, jsonInit("POST", { estimateId: estimate.id, materialWastePercent: waste, allocations: entries }));
      setBudgets((current) => [created, ...current]); setBudgetId(created.id); setPreview(null); setSaved(null);
    } catch (failure) { setError(failure instanceof Error ? failure.message : "Budget could not be saved."); }
    finally { setBusy(false); }
  };

  const compare = async () => {
    if (!budget) return;
    const alternatives: MaterialSelection[][] = [];
    const matching = pricedRequirements.map((requirement) => ({ requirement, offers: offers.filter((offer) => offer.recipe.category === requirement.recipe.category && offer.recipe.specification === requirement.recipe.specification && (requirement.trade !== "paint" || offer.recipe.coats === requirement.recipe.coats)) }));
    if (!matching.length || matching.some((entry) => !entry.offers.length)) { setError("Comparable fixture coverage is incomplete for these specifications."); return; }
    for (let index = 0; index < Math.min(20, Math.max(...matching.map((entry) => entry.offers.length))); index++) alternatives.push(matching.map((entry) => ({ requirementId: entry.requirement.id, observationId: entry.offers[index % entry.offers.length].id })));
    setBusy(true); setError("");
    try {
      const result = await request<MaterialPreview>(`/api/material-budgets/${budget.id}/comparisons/preview`, jsonInit("POST", { location, alternatives }));
      setChoices(alternatives); setPreview(result);
    } catch (failure) { setError(failure instanceof Error ? failure.message : "Preview unavailable."); }
    finally { setBusy(false); }
  };

  const save = async (index: number) => {
    if (!budget) return;
    setBusy(true); setError("");
    try {
      const result = await request<MaterialComparison>(`/api/material-budgets/${budget.id}/comparisons`, jsonInit("POST", { location, selections: choices[index] }));
      setSaved(result); setComparisonId(result.id);
    } catch (failure) { setError(failure instanceof Error ? failure.message : "Selection expired; preview again."); setPreview(null); }
    finally { setBusy(false); }
  };

  const retrieve = async () => {
    setError("");
    try { const result = await request<MaterialComparison>(`/api/material-comparisons/${encodeURIComponent(comparisonId)}`); if (result.budgetId !== budget?.id) throw new Error("Comparison belongs to another budget."); setSaved(result); }
    catch (failure) { setError(failure instanceof Error ? failure.message : "Saved comparison unavailable."); }
  };

  return <section className="material-workspace no-print">
    <h3 className="section-title">Material-only budget</h3>
    <p className="muted small">Separate from finish-rate costs; plaster materials use confirmed net plaster area. Labour and the QS grand total are unchanged.</p>
    {!sourceReady && <p className="alert alert--warning">Exact confirmed drawing page and image digest unavailable. Recalculate a new estimate from a confirmed drawing before material budgeting.</p>}
    <fieldset disabled={busy || !sourceReady} className="material-inputs">
      <legend>New budget consumption inputs</legend>
      <div className="rates-grid">
        <label className="field"><span>Paint specification</span><input value={paint.specification} onChange={(event) => { setPaint({ ...paint, specification: event.target.value }); setReviewed(false); }} /></label>
        <label className="field"><span>Paint coats</span><input aria-label="Material paint coats" type="number" min="1" step="1" value={paint.coats} onChange={(event) => { setPaint({ ...paint, coats: Number(event.target.value) }); setReviewed(false); }} /></label>
        <label className="field"><span>Coverage m²/litre/coat</span><input aria-label="Material paint coverage" type="number" min="0" step="0.01" value={paint.coverageM2PerLitrePerCoat} onChange={(event) => { setPaint({ ...paint, coverageM2PerLitrePerCoat: Number(event.target.value) }); setReviewed(false); }} /></label>
        <label className="field"><span>Paint pack litres</span><input aria-label="Material paint pack" type="number" min="0" step="0.01" value={paint.packAmount} onChange={(event) => { setPaint({ ...paint, packAmount: Number(event.target.value) }); setReviewed(false); }} /></label>
        <label className="field"><span>Floor tile specification</span><input value={tile.specification} onChange={(event) => { setTile({ ...tile, specification: event.target.value }); setReviewed(false); }} /></label>
        <label className="field"><span>Wall tile specification</span><input value={wallTileSpecification} onChange={(event) => { setWallTileSpecification(event.target.value); setReviewed(false); }} /></label>
        <label className="field"><span>Tile pack coverage m²</span><input aria-label="Material tile pack" type="number" min="0" step="0.01" value={tile.packAmount} onChange={(event) => { setTile({ ...tile, packAmount: Number(event.target.value) }); setReviewed(false); }} /></label>
        <label className="field"><span>Material waste %</span><input type="number" min="0" max="50" value={waste} onChange={(event) => { setWaste(Number(event.target.value)); setReviewed(false); }} /></label>
      </div>
      <div className="table-wrap"><table className="data-table"><thead><tr><th>Room</th><th>Paint</th><th>Wall tiles</th><th>Floor tiles</th>{plasterMaterialsEnabled && <><th>Cement + sand plaster</th><th>Premix plaster</th></>}</tr></thead><tbody>{estimate.result.rooms.map((room) => <tr key={room.roomId}><td>{room.roomName}<div className="cell-note">Net plaster {formatNumber(room.netPlasterArea ?? 0)} m²</div></td>{selectableTrades.map((trade) => { const other = trade === "plaster" ? "premixPlaster" : trade === "premixPlaster" ? "plaster" : null; return <td key={trade}><input type="checkbox" aria-label={`${room.roomName} material ${trade}`} checked={!!allocations[`${room.roomId}:${trade}`]} onChange={(event) => { setAllocations({ ...allocations, [`${room.roomId}:${trade}`]: event.target.checked, ...(event.target.checked && other ? { [`${room.roomId}:${other}`]: false } : {}) }); setReviewed(false); }} /></td>; })}</tr>)}</tbody></table></div>
      {(Object.keys(allocations).some((key) => key.endsWith(":plaster") && allocations[key]) || Object.keys(allocations).some((key) => key.endsWith(":premixPlaster") && allocations[key])) && <div className="plaster-recipes">
        {Object.keys(allocations).some((key) => key.endsWith(":plaster") && allocations[key]) && <fieldset className="material-recipe"><legend>Traditional cement + sand mix</legend><div className="rates-grid">
          <label className="field"><span>Cement specification</span><input value={plasterMix.cementSpecification} onChange={(event) => { setPlasterMix({ ...plasterMix, cementSpecification: event.target.value }); setReviewed(false); }} /></label>
          <label className="field"><span>Sand specification</span><input value={plasterMix.sandSpecification} onChange={(event) => { setPlasterMix({ ...plasterMix, sandSpecification: event.target.value }); setReviewed(false); }} /></label>
          {([["thicknessMm", "Layer thickness mm"], ["cementParts", "Cement parts by volume"], ["sandParts", "Sand parts by volume"], ["dryVolumeFactor", "Dry-volume factor"], ["cementDensityKgPerM3", "Cement bulk density kg/m³"]] as const).map(([key, label]) => <label className="field" key={key}><span>{label}</span><input aria-label={label} type="number" min="0" step="any" value={plasterMix[key]} onChange={(event) => { setPlasterMix({ ...plasterMix, [key]: event.target.value }); setReviewed(false); }} /></label>)}
          <label className="field"><span>Cement bag weight kg</span><input aria-label="Cement bag weight kg" type="number" min="0" step="any" value={plasterMix.cementPackAmountKg} onChange={(event) => { setPlasterMix({ ...plasterMix, cementPackAmountKg: event.target.value }); setReviewed(false); }} /></label>
          <label className="field"><span>Sand pack volume m³</span><input aria-label="Sand pack volume m³" type="number" min="0" step="any" value={plasterMix.sandPackAmountM3} onChange={(event) => { setPlasterMix({ ...plasterMix, sandPackAmountM3: event.target.value }); setReviewed(false); }} /></label>
        </div><p className="cell-note">Enter the reviewed local mix, material density, layer thickness and pack sizes. No defaults are assumed.</p></fieldset>}
        {Object.keys(allocations).some((key) => key.endsWith(":premixPlaster") && allocations[key]) && <fieldset className="material-recipe"><legend>Premixed plaster</legend><div className="rates-grid"><label className="field"><span>Product specification</span><input value={premix.specification} onChange={(event) => { setPremix({ ...premix, specification: event.target.value }); setReviewed(false); }} /></label><label className="field"><span>Layer thickness mm</span><input aria-label="Premix layer thickness mm" type="number" min="0" step="any" value={premix.thicknessMm} onChange={(event) => { setPremix({ ...premix, thicknessMm: event.target.value }); setReviewed(false); }} /></label><label className="field"><span>Product consumption kg/m²</span><input aria-label="Premix consumption kg per m2" type="number" min="0" step="any" value={premix.consumptionPerM2} onChange={(event) => { setPremix({ ...premix, consumptionPerM2: event.target.value }); setReviewed(false); }} /></label><label className="field"><span>Bag weight kg</span><input aria-label="Premix bag weight kg" type="number" min="0" step="any" value={premix.packAmount} onChange={(event) => { setPremix({ ...premix, packAmount: event.target.value }); setReviewed(false); }} /></label></div><p className="cell-note">Use the selected product data sheet; thickness and consumption are required. No defaults are assumed.</p></fieldset>}
      </div>}
      <label className="material-review-check"><input type="checkbox" checked={reviewed} onChange={(event) => setReviewed(event.target.checked)} /> Consumption inputs reviewed; defaults are manual assumptions, not supplier specifications.</label>
      <button type="button" className="button button--secondary" disabled={!reviewed} onClick={create}>Save material budget</button>
    </fieldset>
    {budgets.length > 0 && <label className="field field--inline"><span>Material budget version</span><select disabled={busy} value={budget?.id ?? ""} onChange={(event) => {
      const entry = budgets.find((version) => version.id === event.target.value);
      setBudgetId(event.target.value); setPreview(null); setSaved(null); setComparisonId("");
      if (entry) {
        setWaste(entry.materialWastePercent); setReviewed(false);
        setAllocations(Object.fromEntries(entry.allocations.map((allocation) => [`${allocation.roomId}:${allocation.trade}`, true])));
        setPaint(entry.allocations.find((allocation) => allocation.trade === "paint")?.recipe ?? initialPaint);
        setTile(entry.allocations.find((allocation) => allocation.trade === "floorTile")?.recipe ?? initialTile);
        setWallTileSpecification(entry.allocations.find((allocation) => allocation.trade === "wallTile")?.recipe.specification ?? "Wall tiles");
        const mix = entry.allocations.find((allocation) => allocation.trade === "plaster")?.recipe;
        setPlasterMix({ cementSpecification: mix?.cementSpecification ?? "", sandSpecification: mix?.sandSpecification ?? "", thicknessMm: mix?.thicknessMm === undefined ? "" : String(mix.thicknessMm), cementParts: mix?.cementParts === undefined ? "" : String(mix.cementParts), sandParts: mix?.sandParts === undefined ? "" : String(mix.sandParts), dryVolumeFactor: mix?.dryVolumeFactor === undefined ? "" : String(mix.dryVolumeFactor), cementDensityKgPerM3: mix?.cementDensityKgPerM3 === undefined ? "" : String(mix.cementDensityKgPerM3), cementPackAmountKg: mix?.cementPackAmountKg === undefined ? "" : String(mix.cementPackAmountKg), sandPackAmountM3: mix?.sandPackAmountM3 === undefined ? "" : String(mix.sandPackAmountM3) });
        const premixRecipe = entry.allocations.find((allocation) => allocation.trade === "premixPlaster")?.recipe;
        setPremix({ specification: premixRecipe?.specification ?? "", thicknessMm: premixRecipe?.thicknessMm === undefined ? "" : String(premixRecipe.thicknessMm), consumptionPerM2: premixRecipe?.consumptionPerM2 === undefined ? "" : String(premixRecipe.consumptionPerM2), packAmount: premixRecipe?.packAmount === undefined ? "" : String(premixRecipe.packAmount) });
      }
    }}>{budgets.map((entry) => <option key={entry.id} value={entry.id}>Version {entry.version} · revision {entry.sourceConfirmationRevision}</option>)}</select></label>}
    {budget && <>
      <details><summary>Frozen budget inputs · {budget.materialWastePercent}% material waste</summary><p className="small">Estimate {budget.estimateId} · confirmation revision {budget.sourceConfirmationRevision} · page {budget.sourcePageNumber} · image {budget.sourceImageDigest} · {formatDate(budget.createdAt)}</p>{budget.requirements.map((entry) => <p key={entry.id} className="small">{labels[entry.trade]} · {entry.recipe.specification || "Excluded"} · source {formatNumber(entry.sourceAreaM2)} m² · pack {entry.recipe.packAmount} {entry.recipe.packUnit}{entry.trade === "paint" && entry.requiredAmount > 0 ? ` · ${entry.recipe.coats} coats · coverage ${entry.recipe.coverageM2PerLitrePerCoat} m²/litre/coat` : ""} · {entry.recipe.inputOrigin || "unknown origin"} · {entry.formulaVersion}</p>)}</details>
      <div className="table-wrap"><table className="data-table data-table--numeric"><thead><tr><th>Material</th><th>Product specification</th><th>Source area</th><th>Required</th><th>Packs</th><th>Purchased</th><th>Surplus</th></tr></thead><tbody>{budget.requirements.map((entry) => <tr key={entry.id}><td>{labels[entry.trade] ?? entry.trade}</td><td>{entry.recipe.specification || "Not specified"}</td><td>{formatNumber(entry.sourceAreaM2)} m²</td><td>{formatNumber(entry.requiredAmount)} {entry.unit}</td><td>{entry.packCount}</td><td>{formatNumber(entry.purchasedAmount)}</td><td>{formatNumber(entry.surplus)}</td></tr>)}</tbody></table></div>
      <h3 className="section-title">Colombo supplier-price fixtures</h3>
      <p className="alert alert--info">Fictional sample prices in LKR. Not live supplier quotations, stock or procurement.</p>
      {fixturesCoverRequirements ? <>
        <label className="field field--inline"><span>Comparison location</span><select aria-label="Supplier comparison location" value={locationChoice} onChange={(event) => { setLocationChoice(event.target.value); setPreview(null); setSaved(null); }}><option value="">Choose a comparison location</option><option value="LK/Colombo">Colombo, Sri Lanka</option></select></label>
        <button type="button" className="button button--secondary" disabled={busy || !locationChoice} onClick={compare}>Preview price alternatives</button>
      </> : <p className="alert alert--warning">No comparable fixture prices cover every positive material requirement. Price comparison is unavailable; no offers are inferred.</p>}
      {fixturesCoverRequirements && preview?.alternatives.map((basket, index) => <div className="material-alternative" key={index}>
        <h4>Option {index + 1} · {formatMoney(basket.materialSubtotal, "LKR")} material subtotal</h4>
        {basket.items.map((item) => <p key={item.requirementId} className="small">{item.offer.supplier} · {item.requirement.packCount} packs · {formatMoney(item.offer.price, item.offer.currency)}/pack · {formatDate(item.offer.observedAt)} · {item.offer.availability} · tax {item.offer.taxIncluded === null ? "unknown" : item.offer.taxIncluded ? "included" : "excluded"} · delivery {item.offer.delivery === null ? "unknown" : formatMoney(item.offer.delivery, item.offer.currency)}</p>)}
        <p className="small">Delivered total: {basket.deliveredTotal === null ? "Incomplete — unknown charges or stale observations" : formatMoney(basket.deliveredTotal, "LKR")}</p>
        {basket.warnings.map((warning, warningIndex) => <p key={warningIndex} className="cell-note">{warning}</p>)}
        <button type="button" className="button button--ghost button--small" disabled={busy} onClick={() => save(index)}>Save option {index + 1}</button>
      </div>)}
      <div className="material-saved-controls"><label className="field"><span>Saved comparison ID</span><input value={comparisonId} onChange={(event) => setComparisonId(event.target.value)} /></label><button type="button" className="button button--ghost button--small" disabled={!comparisonId} onClick={retrieve}>Open saved comparison</button></div>
      {saved && <div className="material-saved" role="status"><strong>Saved material comparison v{saved.version}</strong><p>{formatMoney(saved.basket.materialSubtotal, saved.currency)} material subtotal · {saved.id}</p><a className="button button--secondary button--small" href={`${API}/api/material-comparisons/${encodeURIComponent(saved.id)}/export`}>Export material comparison</a></div>}
    </>}
    {error && <p role="alert" className="alert alert--error">{error}</p>}
  </section>;
}