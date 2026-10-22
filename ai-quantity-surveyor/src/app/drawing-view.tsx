"use client";

import { useEffect, useState } from "react";
import { fetchSourceDrawing, type AnalysisSource, type Bounds, type Room } from "./api";

type Drawing = { url: string; width: number; height: number };

export const roomPalette = ["#16838f", "#c75c50", "#638a61", "#b7791f", "#6b5fb5", "#2f6db0"];

async function renderDrawing(file: File): Promise<Drawing> {
  if (file.type === "application/pdf") {
    const pdfjs = await import("pdfjs-dist");
    pdfjs.GlobalWorkerOptions.workerSrc = new URL("pdfjs-dist/build/pdf.worker.min.mjs", import.meta.url).toString();
    const task = pdfjs.getDocument({ data: new Uint8Array(await file.arrayBuffer()) });
    try {
      const document = await task.promise;
      const page = await document.getPage(1);
      const base = page.getViewport({ scale: 1 });
      const viewport = page.getViewport({ scale: Math.min(2, 1800 / Math.max(base.width, base.height)) });
      const canvas = window.document.createElement("canvas");
      canvas.width = Math.ceil(viewport.width);
      canvas.height = Math.ceil(viewport.height);
      const context = canvas.getContext("2d");
      if (!context) throw new Error("Canvas unavailable");
      await page.render({ canvas, canvasContext: context, viewport }).promise;
      return { url: canvas.toDataURL("image/png"), width: canvas.width, height: canvas.height };
    } finally {
      await task.destroy();
    }
  }
  const url = URL.createObjectURL(file);
  const image = new Image();
  image.src = url;
  await image.decode();
  return { url, width: image.naturalWidth, height: image.naturalHeight };
}

export default function DrawingView({ file, rooms, selectedId, onSelect, analysisId, source, highlight }: { file?: File | null; rooms: Room[]; selectedId?: string; onSelect?: (id: string) => void; analysisId?: string; source?: AnalysisSource; highlight?: Bounds | null }) {
  const [drawing, setDrawing] = useState<Drawing | null>(null);
  const [failed, setFailed] = useState(false);
  const [loadedKey, setLoadedKey] = useState("");
  const [zoomState, setZoomState] = useState({ key: "", value: 1 });
  const sourceKey = source ? `${analysisId}:${source.imageDigest}` : file ? `${file.name}:${file.size}:${file.lastModified}` : "missing";
  const zoom = zoomState.key === sourceKey ? zoomState.value : 1;
  const validHighlight = highlight && [highlight.x, highlight.y, highlight.width, highlight.height].every(Number.isFinite) && highlight.x >= 0 && highlight.y >= 0 && highlight.width > 0 && highlight.height > 0 && highlight.x + highlight.width <= 1 && highlight.y + highlight.height <= 1 ? highlight : null;

  useEffect(() => {
    let disposed = false;
    let url = "";
    const abort = new AbortController();
    const load = async () => {
      if (source) {
        if (!analysisId) throw new Error("Source analysis unavailable");
        const canonical = await fetchSourceDrawing(analysisId, source, abort.signal);
        const rendered = await renderDrawing(canonical);
        if (rendered.width !== source.pixelWidth || rendered.height !== source.pixelHeight) {
          URL.revokeObjectURL(rendered.url);
          throw new Error("Source dimensions do not match analysis");
        }
        return rendered;
      }
      if (!file) throw new Error("Drawing unavailable");
      return renderDrawing(file);
    };
    load()
      .then((result) => {
        url = result.url;
        if (disposed) {
          if (url.startsWith("blob:")) URL.revokeObjectURL(url);
          return;
        }
        setFailed(false);
        setDrawing(result);
        setLoadedKey(sourceKey);
      })
      .catch(() => { if (!disposed) { setFailed(true); setLoadedKey(sourceKey); } });
    return () => {
      disposed = true;
      abort.abort();
      if (url.startsWith("blob:")) URL.revokeObjectURL(url);
    };
  }, [file, analysisId, source, sourceKey]);

  if (loadedKey === sourceKey && failed) return <p className="empty-note" role="status">{source ? "Canonical drawing unavailable; dimensions remain available for review." : "This drawing could not be displayed in the browser."}</p>;
  if (loadedKey !== sourceKey || !drawing) return <div className="drawing-loading"><span className="spinner" aria-hidden /> Rendering drawing…</div>;
  return <div className="drawing-preview">
    <div className="drawing-tools">
      <span className="cell-note">{source ? `Source page ${source.pageNumber}` : "Source provenance unavailable (legacy)"}</span>
      <button type="button" className="icon-button" title="Zoom out" aria-label="Zoom out" disabled={zoom <= 1} onClick={() => setZoomState({ key: sourceKey, value: Math.max(1, zoom - 0.25) })}>-</button>
      <output aria-label="Drawing zoom">{Math.round(zoom * 100)}%</output>
      <button type="button" className="icon-button" title="Zoom in" aria-label="Zoom in" disabled={zoom >= 3} onClick={() => setZoomState({ key: sourceKey, value: Math.min(3, zoom + 0.25) })}>+</button>
      <button type="button" className="button button--ghost button--small" onClick={() => setZoomState({ key: sourceKey, value: 1 })}>Reset view</button>
    </div>
    <div className="drawing-scroll"><div className={`drawing-view${selectedId ? " drawing-view--selecting" : ""}`} style={{ aspectRatio: `${drawing.width} / ${drawing.height}`, width: `${zoom * 100}%` }}>
    {/* eslint-disable-next-line @next/next/no-img-element */}
    <img src={drawing.url} alt="Uploaded floor plan" />
    <svg viewBox="0 0 1 1" preserveAspectRatio="none" aria-label="Room boundaries extracted from the drawing">
      {validHighlight && <rect className="drawing-evidence" x={validHighlight.x} y={validHighlight.y} width={validHighlight.width} height={validHighlight.height} />}
      {rooms.map((room, index) => room.bounds && <rect key={room.id} data-room={room.id} className={room.id === selectedId ? "drawing-room drawing-room--selected" : "drawing-room"}
        x={room.bounds.x} y={room.bounds.y} width={room.bounds.width} height={room.bounds.height}
        style={{ fill: `${roomPalette[index % roomPalette.length]}${room.id === selectedId ? "66" : "33"}`, stroke: roomPalette[index % roomPalette.length], cursor: onSelect ? "pointer" : undefined }}
        onClick={onSelect ? () => onSelect(room.id) : undefined} />)}
    </svg>
    {rooms.map((room, index) => room.bounds && <span key={room.id} className={room.id === selectedId ? "drawing-label drawing-label--selected" : "drawing-label"} style={{ left: `${(room.bounds.x + room.bounds.width / 2) * 100}%`, top: `${(room.bounds.y + room.bounds.height / 2) * 100}%`, borderColor: roomPalette[index % roomPalette.length] }}>
      {onSelect ? <button type="button" className="drawing-label__button" aria-pressed={room.id === selectedId} onClick={() => onSelect(room.id)}>{room.name}</button> : room.name}
    </span>)}
    </div></div>
  </div>;
}
