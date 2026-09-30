"use client";

import { useEffect, useRef, useState } from "react";
import * as THREE from "three";
import { OrbitControls } from "three/addons/controls/OrbitControls.js";
import { CSS2DObject, CSS2DRenderer } from "three/addons/renderers/CSS2DRenderer.js";
import type { ModelRoom } from "./preview";

export default function RoomScene({ rooms }: { rooms: ModelRoom[] }) {
  const host = useRef<HTMLDivElement>(null);
  const controls = useRef<OrbitControls | null>(null);
  const [failed, setFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const container = host.current;
    if (!container || rooms.length === 0) return;
    let renderer: THREE.WebGLRenderer;
    try {
      renderer = new THREE.WebGLRenderer({ antialias: true });
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
    const colors = [0x16838f, 0xc75c50, 0x638a61, 0xb7791f];
    const walls = new Set<string>();
    rooms.forEach((room, index) => {
      const color = room.approximate ? 0x9aa7b3 : colors[index % colors.length];
      const floor = new THREE.Mesh(new THREE.PlaneGeometry(room.length, room.width), new THREE.MeshStandardMaterial({ color, transparent: true, opacity: room.approximate ? 0.16 : 0.3, side: THREE.DoubleSide }));
      floor.rotation.x = -Math.PI / 2;
      floor.position.set(room.x + room.length / 2, 0, room.z + room.width / 2);
      scene.add(floor);
      const segments = [
        [room.x, room.z, room.x + room.length, room.z],
        [room.x, room.z + room.width, room.x + room.length, room.z + room.width],
        [room.x, room.z, room.x, room.z + room.width],
        [room.x + room.length, room.z, room.x + room.length, room.z + room.width],
      ];
      for (const [startX, startZ, endX, endZ] of segments) {
        const key = [startX, startZ, endX, endZ].map((value) => value.toFixed(3)).join(":");
        if (walls.has(key)) continue;
        walls.add(key);
        const alongX = startZ === endZ;
        const wall = new THREE.Mesh(new THREE.BoxGeometry(alongX ? endX - startX : 0.06, room.height, alongX ? 0.06 : endZ - startZ), new THREE.MeshStandardMaterial({ color: room.approximate ? 0x8795a1 : 0x28727a, transparent: true, opacity: room.approximate ? 0.45 : 0.82 }));
        wall.position.set((startX + endX) / 2, room.height / 2, (startZ + endZ) / 2);
        scene.add(wall);
      }
      const label = document.createElement("span");
      label.className = room.approximate ? "scene-room-label scene-room-label--approximate" : "scene-room-label";
      label.textContent = room.approximate ? `${room.name} (approx.)` : room.name;
      const marker = new CSS2DObject(label);
      marker.position.set(room.x + room.length / 2, room.height + 0.3, room.z + room.width / 2);
      scene.add(marker);
    });

    const distance = Math.max((maxX - minX) * 1.6, (maxZ - minZ) * 2, maxHeight * 3, 8);
    const orbit = new OrbitControls(camera, renderer.domElement);
    controls.current = orbit;
    orbit.target.set(centerX, maxHeight / 2, centerZ);
    camera.position.set(centerX + distance * 0.35, distance * 0.65, centerZ + distance);
    orbit.enableDamping = true;
    orbit.minDistance = 2;
    orbit.maxDistance = Math.max(distance * 5, 30);
    orbit.update();
    orbit.saveState();
    const resize = () => {
      const width = Math.max(container.clientWidth, 1);
      const height = Math.max(container.clientHeight, 1);
      camera.aspect = width / height;
      camera.position.set(centerX + distance * 0.35 / Math.min(camera.aspect, 1), distance * 0.65, centerZ + distance / Math.min(camera.aspect, 1));
      camera.updateProjectionMatrix();
      renderer.setSize(width, height);
      labels.setSize(width, height);
    };
    const observer = new ResizeObserver(resize);
    observer.observe(container);
    resize();
    orbit.saveState();
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
      scene.traverse((object) => {
        if (object instanceof THREE.Mesh || object instanceof THREE.LineSegments) {
          object.geometry.dispose();
          if (object.material instanceof THREE.Material) object.material.dispose();
        }
      });
      renderer.dispose();
      renderer.domElement.remove();
      labels.domElement.remove();
    };
  }, [rooms, attempt]);

  return <div className="scene-wrap">
    {failed ? <div className="scene-unavailable">3D rendering is unavailable in this browser. Room details remain listed. <button onClick={() => { setFailed(false); setAttempt((current) => current + 1); }}>Retry</button></div> : null}
    <div ref={host} className="scene-host" role="img" aria-label="3D model of the floor plan" />
    {!failed && <button className="scene-reset" type="button" onClick={() => controls.current?.reset()} title="Reset view" aria-label="Reset view">Reset view</button>}
  </div>;
}