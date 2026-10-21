"use client";

import { useEffect, useState } from "react";
import Image from "next/image";
import { API, authenticatedFetch, describeMeasurement, jsonInit, request, type Confirmation, type Project, type RoomDesignCapability, type RoomDesignConcept } from "./api";
import type { Settings } from "./settings";

const emptyRooms: NonNullable<Confirmation["candidate"]["rooms"]> = [];
const emptyConcepts: RoomDesignConcept[] = [];

export default function RoomDesignStudio({ project, confirmation, settings, onBack }: { project: Project; confirmation: Confirmation; settings: Settings; onBack: () => void }) {
  const [confirmations, setConfirmations] = useState<Confirmation[]>([confirmation]);
  const [selectedRevision, setSelectedRevision] = useState(confirmation.revision);
  const activeConfirmation = confirmations.find((entry) => entry.revision === selectedRevision) ?? confirmation;
  const rooms = activeConfirmation.candidate.rooms ?? emptyRooms;
  const [selectedRoomId, setSelectedRoomId] = useState(confirmation.candidate.rooms?.[0]?.id ?? "");
  const [capabilities, setCapabilities] = useState<RoomDesignCapability | null>(null);
  const [designCollection, setDesignCollection] = useState<{ key: string; concepts: RoomDesignConcept[] } | null>(null);
  const [imageCollection, setImageCollection] = useState<{ key: string; urls: Record<string, string> } | null>(null);
  const [style, setStyle] = useState(settings.designDefaults.style);
  const [palette, setPalette] = useState(settings.designDefaults.palette);
  const [preferences, setPreferences] = useState("");
  const [busy, setBusy] = useState(false);
  const [actingId, setActingId] = useState("");
  const [error, setError] = useState("");
  const room = rooms.find((entry) => entry.id === selectedRoomId) ?? rooms[0];
  const roomId = room?.id ?? "";
  const revision = activeConfirmation.revision;
  const doors = activeConfirmation.candidate.doors?.filter((opening) => opening.roomId === roomId) ?? [];
  const windows = activeConfirmation.candidate.windows?.filter((opening) => opening.roomId === roomId) ?? [];
  const collectionKey = `${project.id}|${roomId}|${revision}`;
  const concepts = designCollection?.key === collectionKey ? designCollection.concepts : emptyConcepts;
  const imageKey = concepts.map((concept) => concept.id).join("|");
  const images = imageCollection?.key === imageKey ? imageCollection.urls : {};

  useEffect(() => {
    let active = true;
    request<RoomDesignCapability>("/api/room-design-capabilities")
      .then((value) => { if (active) setCapabilities(value); })
      .catch(() => { if (active) setCapabilities({ enabled: false, dailyImageLimit: 0, styles: [], palettes: [] }); });
    return () => { active = false; };
  }, []);

  useEffect(() => {
    let active = true;
    request<Confirmation[]>(`/api/analyses/${encodeURIComponent(confirmation.analysisId)}/confirmations`)
      .then((history) => {
        if (!active) return;
        const revisions = history.some((entry) => entry.revision === confirmation.revision) ? history : [...history, confirmation];
        revisions.sort((first, second) => second.revision - first.revision);
        setConfirmations(revisions);
      })
      .catch(() => setConfirmations([confirmation]));
    return () => { active = false; };
  }, [confirmation]);

  useEffect(() => {
    if (!roomId) return;
    let active = true;
    request<RoomDesignConcept[]>(`/api/projects/${encodeURIComponent(project.id)}/rooms/${encodeURIComponent(roomId)}/designs?revision=${revision}`)
      .then((value) => { if (active) setDesignCollection({ key: collectionKey, concepts: value }); })
      .catch((failure) => { if (active) setError(failure instanceof Error ? failure.message : "Saved room concepts could not be loaded."); });
    return () => { active = false; };
  }, [collectionKey, project.id, revision, roomId]);

  useEffect(() => {
    let active = true;
    const objectURLs: string[] = [];
    if (!imageKey) return;
    Promise.all(imageKey.split("|").map(async (conceptID) => {
      try {
        const response = await authenticatedFetch(`${API}/api/room-designs/${encodeURIComponent(conceptID)}/image`);
        if (!response.ok) return null;
        const objectURL = URL.createObjectURL(await response.blob());
        objectURLs.push(objectURL);
        return [conceptID, objectURL] as const;
      } catch {
        return null;
      }
    })).then((entries) => {
      if (active) setImageCollection({ key: imageKey, urls: Object.fromEntries(entries.filter((entry): entry is readonly [string, string] => entry !== null)) });
    });
    return () => {
      active = false;
      for (const objectURL of objectURLs) URL.revokeObjectURL(objectURL);
    };
  }, [imageKey]);

  const generate = async () => {
    if (!room) return;
    setBusy(true);
    setError("");
    try {
      const generated = await request<RoomDesignConcept[]>(`/api/projects/${encodeURIComponent(project.id)}/rooms/${encodeURIComponent(room.id)}/designs`, jsonInit("POST", { roomId: room.id, confirmationRevision: revision, style, palette, preferences }));
      setDesignCollection((current) => ({ key: collectionKey, concepts: [...generated, ...(current?.key === collectionKey ? current.concepts : [])] }));
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "Room concepts could not be generated.");
    } finally {
      setBusy(false);
    }
  };

  const select = async (concept: RoomDesignConcept) => {
    setActingId(concept.id);
    setError("");
    try {
      await request(`/api/room-designs/${encodeURIComponent(concept.id)}/selection`, jsonInit("POST", {}));
      setDesignCollection((current) => current?.key === collectionKey ? { ...current, concepts: current.concepts.map((entry) => ({ ...entry, selected: entry.id === concept.id })) } : current);
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "Concept selection could not be saved.");
    } finally {
      setActingId("");
    }
  };

  const remove = async (concept: RoomDesignConcept) => {
    setActingId(concept.id);
    setError("");
    try {
      const response = await authenticatedFetch(`${API}/api/room-designs/${encodeURIComponent(concept.id)}`, { method: "DELETE" });
      if (!response.ok) throw new Error("Concept could not be removed.");
      setDesignCollection((current) => current?.key === collectionKey ? { ...current, concepts: current.concepts.filter((entry) => entry.id !== concept.id) } : current);
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "Concept could not be removed.");
    } finally {
      setActingId("");
    }
  };

  return <section className="card step-card room-design-studio">
    <header className="step-header">
      <div className="design-heading"><button type="button" className="button button--ghost button--small" onClick={onBack}>Back to project</button><div><p className="eyebrow">Design studio · {project.name}</p><h2>Room interiors</h2><p className="muted">Confirmed revision {revision} · concepts are illustrative, not construction-ready.</p></div></div>
      <span className="badge badge--success">Confirmed source</span>
    </header>
    <div className="design-layout">
      <aside className="design-controls">
        {confirmations.length > 1 && <label className="field"><span>Confirmed revision</span><select aria-label="Design confirmation revision" value={selectedRevision} onChange={(event) => setSelectedRevision(Number(event.target.value))}>{confirmations.map((entry) => <option key={entry.revision} value={entry.revision}>Revision {entry.revision}</option>)}</select></label>}
        <label className="field"><span>Room</span><select aria-label="Design room" value={roomId} onChange={(event) => setSelectedRoomId(event.target.value)}>{rooms.map((entry) => <option key={entry.id} value={entry.id}>{entry.name}</option>)}</select></label>
        {room && <dl className="design-room-facts">
          <div><dt>Length</dt><dd>{describeMeasurement(room.length)}</dd></div>
          <div><dt>Width</dt><dd>{describeMeasurement(room.width)}</dd></div>
          <div><dt>Wall height</dt><dd>{describeMeasurement(room.wallHeight)}</dd></div>
          <div><dt>Openings</dt><dd>{doors.length} doors · {windows.length} windows</dd></div>
        </dl>}
        <label className="field"><span>Interior style</span><select value={style} onChange={(event) => setStyle(event.target.value)} disabled={!capabilities?.enabled}>{(capabilities?.styles ?? []).map((entry) => <option key={entry}>{entry}</option>)}</select></label>
        <label className="field"><span>Color palette</span><select value={palette} onChange={(event) => setPalette(event.target.value)} disabled={!capabilities?.enabled}>{(capabilities?.palettes ?? []).map((entry) => <option key={entry}>{entry}</option>)}</select></label>
        <label className="field"><span>Preferences <small>Optional · 300 characters max</small></span><textarea rows={3} maxLength={300} value={preferences} onChange={(event) => setPreferences(event.target.value)} disabled={!capabilities?.enabled} placeholder="For example, reading nook and hidden storage" /></label>
        {!capabilities?.enabled && <p className="alert alert--warning">Image generation is not configured. Set an API-side OpenAI or NVIDIA NIM image provider to enable this studio.</p>}
        <button type="button" className="button button--primary" disabled={!room || !capabilities?.enabled || busy} onClick={() => void generate()}>{busy ? <><span className="spinner" aria-hidden="true" /> Generating…</> : "Generate two concepts"}</button>
        {capabilities?.enabled && <p className="small muted">Daily limit: {capabilities.dailyImageLimit} generated images per account. Confirmed room data only; no plan image is sent.</p>}
      </aside>
      <div className="design-results" aria-live="polite">
        <div className="design-results__heading"><h3>{room?.name ?? "Select a room"}</h3><span className="muted small">Revision {revision}</span></div>
        {concepts.length === 0 ? <div className="empty-state"><strong>No concepts saved for this revision</strong><span className="muted">Choose a style and palette, then generate two options.</span></div>
          : <div className="design-concepts">{concepts.map((concept, index) => <article className={`design-concept${concept.selected ? " design-concept--selected" : ""}`} key={concept.id}>
            {images[concept.id] ? <Image src={images[concept.id]} alt={`${concept.style} ${concept.palette} concept for ${concept.roomName}`} width={1024} height={768} sizes="(max-width: 960px) 100vw, 40vw" unoptimized /> : <div className="design-image-placeholder" role="img" aria-label={`Loading concept ${index + 1}`}><span className="spinner" aria-hidden="true" /></div>}
            <div className="design-concept__body"><div className="design-concept__title"><div><h4>Option {index + 1}</h4><p className="muted small">{concept.style} · {concept.palette}</p></div>{concept.selected && <span className="badge badge--success">Selected</span>}</div>
              <p className="muted small">{new Date(concept.createdAt).toLocaleString()} · {concept.model} · confirmation {concept.confirmationRevision}</p>
              <div className="design-concept__actions"><button type="button" className="button button--secondary button--small" disabled={actingId === concept.id || concept.selected} onClick={() => void select(concept)}>{concept.selected ? "Selected" : "Select design"}</button><button type="button" className="button button--ghost button--small" disabled={actingId === concept.id} onClick={() => void remove(concept)}>Remove</button></div>
            </div>
          </article>)}</div>}
      </div>
    </div>
    {error && <p className="alert alert--error" role="alert">{error}</p>}
  </section>;
}