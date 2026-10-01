"use client";

import { useEffect, useState } from "react";
import type { Room } from "./api";

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

export default function DrawingView({ file, rooms, selectedId, onSelect }: { file: File; rooms: Room[]; selectedId?: string; onSelect?: (id: string) => void }) {
  const [drawing, setDrawing] = useState<Drawing | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let disposed = false;
    let url = "";
    renderDrawing(file)
      .then((result) => {
        url = result.url;
        if (disposed) {
          if (url.startsWith("blob:")) URL.revokeObjectURL(url);
          return;
        }
        setFailed(false);
        setDrawing(result);
      })
      .catch(() => { if (!disposed) setFailed(true); });
    return () => {
      disposed = true;
      if (url.startsWith("blob:")) URL.revokeObjectURL(url);
    };
  }, [file]);

  if (failed) return <p className="empty-note">This drawing could not be displayed in the browser.</p>;
  if (!drawing) return <div className="drawing-loading"><span className="spinner" aria-hidden /> Rendering drawing…</div>;
  return <div className={`drawing-view${selectedId ? " drawing-view--selecting" : ""}`} style={{ aspectRatio: `${drawing.width} / ${drawing.height}` }}>
    {/* eslint-disable-next-line @next/next/no-img-element */}
    <img src={drawing.url} alt="Uploaded floor plan" />
    <svg viewBox="0 0 1 1" preserveAspectRatio="none" aria-label="Room boundaries extracted from the drawing">
      {rooms.map((room, index) => room.bounds && <rect key={room.id} data-room={room.id} className={room.id === selectedId ? "drawing-room drawing-room--selected" : "drawing-room"}
        x={room.bounds.x} y={room.bounds.y} width={room.bounds.width} height={room.bounds.height}
        style={{ fill: `${roomPalette[index % roomPalette.length]}${room.id === selectedId ? "66" : "33"}`, stroke: roomPalette[index % roomPalette.length], cursor: onSelect ? "pointer" : undefined }}
        onClick={onSelect ? () => onSelect(room.id) : undefined} />)}
    </svg>
    {rooms.map((room, index) => room.bounds && <span key={room.id} className={room.id === selectedId ? "drawing-label drawing-label--selected" : "drawing-label"} style={{ left: `${(room.bounds.x + room.bounds.width / 2) * 100}%`, top: `${(room.bounds.y + room.bounds.height / 2) * 100}%`, borderColor: roomPalette[index % roomPalette.length] }}>
      {onSelect ? <button type="button" className="drawing-label__button" aria-pressed={room.id === selectedId} onClick={() => onSelect(room.id)}>{room.name}</button> : room.name}
    </span>)}
  </div>;
}
