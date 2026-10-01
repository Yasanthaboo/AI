"use client";

import { useEffect, useRef, useState } from "react";
import * as THREE from "three";
import { OrbitControls } from "three/addons/controls/OrbitControls.js";
import { CSS2DObject, CSS2DRenderer } from "three/addons/renderers/CSS2DRenderer.js";
import { buildWalls, wallLength, type ModelRoom, type Wall } from "./preview";

type Hover = { x: number; y: number; room: ModelRoom };
type Controls = { reset: () => void; top: () => void; screenshot: () => string };

const colors = [0x16838f, 0xc75c50, 0x638a61, 0xb7791f, 0x6b5fb5, 0x2f6db0];
const thickness = 0.06;

// Splits a wall into solid pieces around its openings; returns [start, end, bottom, top] spans along the wall.
function wallPieces(wall: Wall): [number, number, number, number][] {
  const length = wallLength(wall);
  const pieces: [number, number, number, number][] = [];
  let cursor = 0;
  for (const opening of [...wall.openings].sort((left, right) => left.center - right.center)) {
    const start = opening.center - opening.width / 2;
    const end = opening.center + opening.width / 2;
    if (start > cursor) pieces.push([cursor, start, 0, wall.height]);
    if (opening.sill > 0) pieces.push([start, end, 0, opening.sill]);
    if (opening.sill + opening.height < wall.height) pieces.push([start, end, opening.sill + opening.height, wall.height]);
    cursor = end;
  }
  if (cursor < length) pieces.push([cursor, length, 0, wall.height]);
  return pieces;
}

export default function RoomScene({ rooms, selectedId, onSelect }: { rooms: ModelRoom[]; selectedId?: string; onSelect?: (id: string) => void }) {
  const host = useRef<HTMLDivElement>(null);
  const controls = useRef<Controls | null>(null);
  const floors = useRef(new Map<string, THREE.MeshStandardMaterial>());
  const select = useRef(onSelect);
  const [failed, setFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [hover, setHover] = useState<Hover | null>(null);
  const [topView, setTopView] = useState(false);

  useEffect(() => { select.current = onSelect; }, [onSelect]);

  useEffect(() => {
    const container = host.current;
    if (!container || rooms.length === 0) return;
    let renderer: THREE.WebGLRenderer;
    try {
      renderer = new THREE.WebGLRenderer({ antialias: true, preserveDrawingBuffer: true });
    } catch {
      const frame = requestAnimationFrame(() => setFailed(true));
      return () => cancelAnimationFrame(frame);
    }
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    renderer.setClearColor(0xf4f7fb);
    renderer.outputColorSpace = THREE.SRGBColorSpace;
    renderer.domElement.setAttribute("aria-label", "3D floor plan model");
    container.appendChild(renderer.domElement);

    const scene = new THREE.Scene();
    const camera = new THREE.PerspectiveCamera(50, 1, 0.1, 10000);
    const labels = new CSS2DRenderer();
    labels.domElement.className = "scene-labels";
    container.appendChild(labels.domElement);
    scene.add(new THREE.HemisphereLight(0xffffff, 0x879ba5, 2.2));
    const light = new THREE.DirectionalLight(0xffffff, 2);
    light.position.set(6, 12, 8);
    scene.add(light);

    const minX = Math.min(...rooms.map((room) => room.x));
    const maxX = Math.max(...rooms.map((room) => room.x + room.length));
    const minZ = Math.min(...rooms.map((room) => room.z));
    const maxZ = Math.max(...rooms.map((room) => room.z + room.width));
    const centerX = (minX + maxX) / 2;
    const centerZ = (minZ + maxZ) / 2;
    const maxHeight = Math.max(...rooms.map((room) => room.height));
    const floorMeshes: THREE.Mesh[] = [];
    floors.current.clear();
    rooms.forEach((room, index) => {
      const material = new THREE.MeshStandardMaterial({ color: room.approximate ? 0x9aa7b3 : colors[index % colors.length], transparent: true, opacity: room.approximate ? 0.16 : 0.3, side: THREE.DoubleSide });
      const floor = new THREE.Mesh(new THREE.PlaneGeometry(room.length, room.width), material);
      floor.rotation.x = -Math.PI / 2;
      floor.position.set(room.x + room.length / 2, 0, room.z + room.width / 2);
      floor.userData.roomId = room.id;
      scene.add(floor);
      floorMeshes.push(floor);
      floors.current.set(room.id, material);
      const label = document.createElement("span");
      label.className = room.approximate ? "scene-room-label scene-room-label--approximate" : "scene-room-label";
      label.textContent = room.approximate ? `${room.name} (approx.)` : room.name;
      const marker = new CSS2DObject(label);
      marker.position.set(room.x + room.length / 2, room.height + 0.3, room.z + room.width / 2);
      scene.add(marker);
    });

    const pane = new THREE.MeshStandardMaterial({ color: 0x8ec9f0, transparent: true, opacity: 0.4 });
    for (const wall of buildWalls(rooms)) {
      const material = new THREE.MeshStandardMaterial({ color: wall.approximate ? 0x8795a1 : 0x28727a, transparent: true, opacity: wall.approximate ? 0.45 : 0.82 });
      const alongX = wall.z1 === wall.z2;
      const place = (start: number, end: number, bottom: number, top: number, meshMaterial: THREE.Material, depth = thickness) => {
        const size = end - start;
        const mesh = new THREE.Mesh(new THREE.BoxGeometry(alongX ? size : depth, top - bottom, alongX ? depth : size), meshMaterial);
        const middle = (start + end) / 2;
        mesh.position.set(alongX ? wall.x1 + middle : wall.x1, (bottom + top) / 2, alongX ? wall.z1 : wall.z1 + middle);
        scene.add(mesh);
      };
      for (const [start, end, bottom, top] of wallPieces(wall)) if (end - start > 0.001 && top - bottom > 0.001) place(start, end, bottom, top, material);
      for (const opening of wall.openings) {
        if (opening.kind === "window") place(opening.center - opening.width / 2, opening.center + opening.width / 2, opening.sill, opening.sill + opening.height, pane, thickness / 3);
      }
    }

    const distance = Math.max((maxX - minX) * 1.6, (maxZ - minZ) * 2, maxHeight * 3, 8);
    const orbit = new OrbitControls(camera, renderer.domElement);
    orbit.target.set(centerX, maxHeight / 2, centerZ);
    orbit.enableDamping = true;
    orbit.minDistance = 2;
    orbit.maxDistance = Math.max(distance * 5, 30);
    const perspective = () => {
      const stretch = 1 / Math.min(camera.aspect, 1);
      camera.position.set(centerX + distance * 0.35 * stretch, distance * 0.65, centerZ + distance * stretch);
      orbit.target.set(centerX, maxHeight / 2, centerZ);
      orbit.update();
    };
    const resize = () => {
      const width = Math.max(container.clientWidth, 1);
      const height = Math.max(container.clientHeight, 1);
      camera.aspect = width / height;
      camera.updateProjectionMatrix();
      renderer.setSize(width, height);
      labels.setSize(width, height);
    };
    const observer = new ResizeObserver(resize);
    observer.observe(container);
    resize();
    perspective();
    controls.current = {
      reset: perspective,
      top: () => {
        const span = Math.max(maxX - minX, (maxZ - minZ) * camera.aspect) / Math.min(camera.aspect, 1);
        camera.position.set(centerX, span * 1.25 + maxHeight, centerZ + 0.001);
        orbit.target.set(centerX, 0, centerZ);
        orbit.update();
      },
      screenshot: () => {
        renderer.render(scene, camera);
        return renderer.domElement.toDataURL("image/png");
      },
    };

    const raycaster = new THREE.Raycaster();
    const pointer = new THREE.Vector2();
    const pick = (event: PointerEvent) => {
      const bounds = renderer.domElement.getBoundingClientRect();
      pointer.set(((event.clientX - bounds.left) / bounds.width) * 2 - 1, -((event.clientY - bounds.top) / bounds.height) * 2 + 1);
      raycaster.setFromCamera(pointer, camera);
      const id = raycaster.intersectObjects(floorMeshes)[0]?.object.userData.roomId as string | undefined;
      return { room: rooms.find((room) => room.id === id), x: event.clientX - bounds.left, y: event.clientY - bounds.top };
    };
    let down: { x: number; y: number } | null = null;
    const onMove = (event: PointerEvent) => {
      if (event.buttons) return;
      const hit = pick(event);
      setHover(hit.room ? { x: hit.x, y: hit.y, room: hit.room } : null);
    };
    const onDown = (event: PointerEvent) => { down = { x: event.clientX, y: event.clientY }; };
    const onUp = (event: PointerEvent) => {
      if (down && Math.hypot(event.clientX - down.x, event.clientY - down.y) < 5) {
        const hit = pick(event);
        if (hit.room) select.current?.(hit.room.id);
      }
      down = null;
    };
    const onLeave = () => setHover(null);
    renderer.domElement.addEventListener("pointermove", onMove);
    renderer.domElement.addEventListener("pointerdown", onDown);
    renderer.domElement.addEventListener("pointerup", onUp);
    renderer.domElement.addEventListener("pointerleave", onLeave);

    let frame = 0;
    const render = () => {
      frame = requestAnimationFrame(render);
      orbit.update();
      renderer.render(scene, camera);
      labels.render(scene, camera);
    };
    render();
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
      orbit.dispose();
      controls.current = null;
      renderer.domElement.removeEventListener("pointermove", onMove);
      renderer.domElement.removeEventListener("pointerdown", onDown);
      renderer.domElement.removeEventListener("pointerup", onUp);
      renderer.domElement.removeEventListener("pointerleave", onLeave);
      scene.traverse((object) => {
        if (object instanceof THREE.Mesh) {
          object.geometry.dispose();
          if (object.material instanceof THREE.Material) object.material.dispose();
        }
      });
      renderer.dispose();
      renderer.domElement.remove();
      labels.domElement.remove();
    };
  }, [rooms, attempt]);

  useEffect(() => {
    rooms.forEach((room) => {
      const material = floors.current.get(room.id);
      if (!material) return;
      const base = room.approximate ? 0.16 : 0.3;
      material.opacity = !selectedId ? base : room.id === selectedId ? 0.75 : base * 0.5;
      material.emissive.setHex(room.id === selectedId ? 0x333333 : 0x000000);
    });
  }, [rooms, selectedId, attempt]);

  const screenshot = () => {
    const url = controls.current?.screenshot();
    if (!url) return;
    const link = document.createElement("a");
    link.href = url;
    link.download = "3d-model.png";
    link.click();
  };

  return <div className="scene-wrap">
    {failed ? <div className="scene-unavailable">3D rendering is unavailable in this browser. Room details remain listed. <button onClick={() => { setFailed(false); setAttempt((current) => current + 1); }}>Retry</button></div> : null}
    <div ref={host} className="scene-host" role="img" aria-label="3D model of the floor plan" />
    {hover && <div className="scene-tooltip" role="tooltip" style={{ left: hover.x, top: hover.y }}><strong>{hover.room.name}</strong><span>{hover.room.detail}</span>{hover.room.openings.length > 0 && <span>{hover.room.openings.length} opening{hover.room.openings.length === 1 ? "" : "s"}</span>}</div>}
    {!failed && <div className="scene-tools">
      <button type="button" aria-pressed={topView} onClick={() => { if (topView) controls.current?.reset(); else controls.current?.top(); setTopView(!topView); }}>{topView ? "3D view" : "Top view"}</button>
      <button type="button" onClick={() => { controls.current?.reset(); setTopView(false); }}>Reset view</button>
      <button type="button" onClick={screenshot}>Screenshot</button>
    </div>}
  </div>;
}
