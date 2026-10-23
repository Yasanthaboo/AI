"use client";

import { useEffect, useRef, useState } from "react";
import { Camera, RotateCcw } from "lucide-react";
import * as THREE from "three";
import { OrbitControls } from "three/addons/controls/OrbitControls.js";
import { roofMeshData, type ExteriorInput } from "./exterior";
import { useTheme } from "./theme-provider";

const finishes = { metal: 0x407483, tile: 0xb36c52, membrane: 0x68757a };

export default function ExteriorScene({ options }: { options: ExteriorInput }) {
  const { theme } = useTheme();
  const host = useRef<HTMLDivElement>(null);
  const commands = useRef<{ reset: () => void; screenshot: () => void } | null>(null);
  const [failed, setFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    const container = host.current;
    if (!container) return;
    let renderer: THREE.WebGLRenderer;
    try { renderer = new THREE.WebGLRenderer({ antialias: true, preserveDrawingBuffer: true }); }
    catch { const frame = requestAnimationFrame(() => setFailed(true)); return () => cancelAnimationFrame(frame); }
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    renderer.setClearColor(new THREE.Color(getComputedStyle(container).backgroundColor));
    renderer.domElement.setAttribute("aria-label", "Conceptual exterior and roof model");
    container.appendChild(renderer.domElement);
    const scene = new THREE.Scene();
    scene.add(new THREE.HemisphereLight(0xffffff, 0x607c88, 2.5));
    const light = new THREE.DirectionalLight(0xffffff, 3);
    light.position.set(8, 15, 10); scene.add(light);
    const wallMaterial = new THREE.MeshStandardMaterial({ color: 0xd4dfe2, roughness: 0.85, side: THREE.DoubleSide });
    const roofMaterial = new THREE.MeshStandardMaterial({ color: finishes[options.finish], roughness: options.finish === "metal" ? 0.45 : 0.9, metalness: options.finish === "metal" ? 0.25 : 0, side: THREE.DoubleSide, polygonOffset: true, polygonOffsetFactor: -2, polygonOffsetUnits: -2 });
    const body = new THREE.Mesh(new THREE.BoxGeometry(options.lengthM, options.wallHeightM, options.widthM), wallMaterial);
    body.position.y = options.wallHeightM / 2; scene.add(body);
    const data = roofMeshData(options);
    const addMesh = (positions: number[], material: THREE.Material) => {
      if (!positions.length) return;
      const geometry = new THREE.BufferGeometry();
      geometry.setAttribute("position", new THREE.Float32BufferAttribute(positions, 3)); geometry.computeVertexNormals();
      scene.add(new THREE.Mesh(geometry, material));
    };
    addMesh(data.roof, roofMaterial); addMesh(data.infill, wallMaterial);
    const span = Math.max(options.lengthM, options.widthM, options.wallHeightM + data.rise, 4);
    const grid = new THREE.GridHelper(span * 2, 20, theme === "dark" ? 0x345164 : 0xc4d1d8, theme === "dark" ? 0x223e51 : 0xe0e7eb);
    grid.position.y = -0.02; scene.add(grid);
    const camera = new THREE.PerspectiveCamera(45, 1, 0.1, span * 30);
    const orbit = new OrbitControls(camera, renderer.domElement);
    orbit.enableDamping = true; orbit.minDistance = span * 0.3; orbit.maxDistance = span * 12; orbit.maxPolarAngle = Math.PI / 2 - 0.02;
    const reset = () => {
      const distance = span * 1.8 / Math.min(camera.aspect, 1);
      camera.position.set(distance * 0.7, (options.wallHeightM + data.rise) / 2 + distance * 0.75, -distance);
      orbit.target.set(0, (options.wallHeightM + data.rise) / 2, 0); orbit.update();
    };
    const resize = () => {
      const width = Math.max(container.clientWidth, 1), height = Math.max(container.clientHeight, 1);
      renderer.setSize(width, height); camera.aspect = width / height; camera.updateProjectionMatrix(); reset();
    };
    const observer = new ResizeObserver(resize); observer.observe(container); resize();
    commands.current = { reset, screenshot: () => { renderer.render(scene, camera); const link = document.createElement("a"); link.href = renderer.domElement.toDataURL("image/png"); link.download = `conceptual-${options.roof}-roof.png`; link.click(); } };
    let frame = 0;
    const render = () => { frame = requestAnimationFrame(render); orbit.update(); renderer.render(scene, camera); }; render();
    return () => {
      cancelAnimationFrame(frame); observer.disconnect(); orbit.dispose(); commands.current = null;
      scene.traverse((object) => { if (object instanceof THREE.Mesh || object instanceof THREE.LineSegments) { object.geometry.dispose(); } });
      wallMaterial.dispose(); roofMaterial.dispose();
      if (Array.isArray(grid.material)) grid.material.forEach((material) => material.dispose()); else grid.material.dispose();
      renderer.dispose(); renderer.domElement.remove();
    };
  }, [options, theme, attempt]);

  return <div className="exterior-render">
    {failed && <div className="alert alert--warning" role="status">3D rendering is unavailable. Roof settings and saved options remain available below.<button type="button" className="button button--ghost button--small" onClick={() => { setFailed(false); setAttempt((current) => current + 1); }}>Retry 3D</button></div>}
    <div className="exterior-host" ref={host} role="img" aria-label="Conceptual exterior preview" hidden={failed} />
    {!failed && <div className="exterior-tools"><button type="button" className="icon-button" title="Reset exterior view" aria-label="Reset exterior view" onClick={() => commands.current?.reset()}><RotateCcw size={17} /></button><button type="button" className="icon-button" title="Download exterior screenshot" aria-label="Download exterior screenshot" onClick={() => commands.current?.screenshot()}><Camera size={17} /></button></div>}
  </div>;
}