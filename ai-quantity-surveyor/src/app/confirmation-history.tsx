"use client";

import { useEffect, useState } from "react";
import { describeMeasurement, formatDate, request, type Confirmation, type Room } from "./api";

const same = (left: Room["length"], right: Room["length"]) => left?.value === right?.value && left?.unit === right?.unit;

// Describes what changed from one confirmed revision to the next.
export function describeChanges(previous: Confirmation | undefined, current: Confirmation): string[] {
  if (!previous) return ["Initial confirmation"];
  const before = new Map(previous.candidate.rooms.map((room) => [room.id, room]));
  const after = new Map(current.candidate.rooms.map((room) => [room.id, room]));
  const changes: string[] = [];
  for (const room of current.candidate.rooms) {
    const old = before.get(room.id);
    if (!old) { changes.push(`Added ${room.name}`); continue; }
    if (old.name !== room.name) changes.push(`Renamed ${old.name} to ${room.name}`);
    for (const [field, label] of [["length", "length"], ["width", "width"], ["wallHeight", "wall height"]] as const) {
      if (!same(old[field], room[field])) changes.push(`${room.name} ${label} ${describeMeasurement(old[field])} → ${describeMeasurement(room[field])}`);
    }
  }
  for (const room of previous.candidate.rooms) if (!after.has(room.id)) changes.push(`Removed ${room.name}`);
  const openings = (confirmation: Confirmation) => (confirmation.candidate.doors?.length ?? 0) + (confirmation.candidate.windows?.length ?? 0);
  if (openings(previous) !== openings(current)) changes.push(`Openings ${openings(previous)} → ${openings(current)}`);
  return changes.length ? changes : ["No changes to rooms or openings"];
}

export function useConfirmations(analysisId: string, latestRevision?: number) {
  const [history, setHistory] = useState<Confirmation[] | null>(null);
  useEffect(() => {
    if (!analysisId) return;
    let active = true;
    request<Confirmation[]>(`/api/analyses/${encodeURIComponent(analysisId)}/confirmations`).then((list) => { if (active) setHistory(list); }).catch(() => { if (active) setHistory([]); });
    return () => { active = false; };
  }, [analysisId, latestRevision]);
  return analysisId ? history : [];
}

export default function ConfirmationHistory({ history }: { history: Confirmation[] | null }) {
  if (history === null) return <p className="muted small" role="status">Loading confirmation history…</p>;
  if (history.length === 0) return <p className="empty-note">No confirmed revisions yet.</p>;
  return <ol className="history-list" aria-label="Confirmation history">
    {[...history].reverse().map((confirmation) => {
      const index = history.findIndex((item) => item.revision === confirmation.revision);
      return <li key={confirmation.revision} className="history-item">
        <div className="history-item__head"><strong>Revision {confirmation.revision}</strong><span className="muted small">{formatDate(confirmation.confirmedAt)} · {confirmation.candidate.rooms.length} rooms · {(confirmation.candidate.doors?.length ?? 0) + (confirmation.candidate.windows?.length ?? 0)} openings</span></div>
        <ul>{describeChanges(history[index - 1], confirmation).map((change) => <li key={change}>{change}</li>)}</ul>
      </li>;
    })}
  </ol>;
}
