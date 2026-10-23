"use client";

import { useEffect, useRef, useState } from "react";
import { Camera, RotateCcw } from "lucide-react";
import * as THREE from "three";
import { OrbitControls } from "three/addons/controls/OrbitControls.js";
import { roofMeshData, type ExteriorInput } from "./exterior";
import { useTheme } from "./theme-provider";

const finishes = { metal: 0x167d8d, tile: 0xc65e40, membrane: 0x435976 };

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
    renderer.outputColorSpace = THREE.SRGBColorSpace;
    renderer.toneMapping = THREE.ACESFilmicToneMapping;
    renderer.toneMappingExposure = 1.15;
    renderer.shadowMap.enabled = true;
    renderer.shadowMap.type = THREE.PCFShadowMap;
    renderer.domElement.setAttribute("aria-label", "Conceptual exterior and roof model");
    container.appendChild(renderer.domElement);
    const scene = new THREE.Scene();
    scene.add(new THREE.HemisphereLight(0xdcefff, 0x82976a, 1.8));
    const data = roofMeshData(options);
    const span = Math.max(options.lengthM, options.widthM, options.wallHeightM + data.rise, 4);
    const light = new THREE.DirectionalLight(0xffedcf, 3);
    light.position.set(span * 0.6, span * 1.5, -span); light.castShadow = true;
    light.shadow.mapSize.set(2048, 2048);
    light.shadow.camera.left = light.shadow.camera.bottom = -span * 1.5;
    light.shadow.camera.right = light.shadow.camera.top = span * 1.5;
    light.shadow.camera.far = span * 6;
    light.shadow.normalBias = 0.04;
    light.shadow.camera.updateProjectionMatrix();
    scene.add(light);
    const wallMaterial = new THREE.MeshStandardMaterial({ color: 0xf2dfbc, roughness: 0.85, side: THREE.DoubleSide });
    const roofMaterial = new THREE.MeshStandardMaterial({ color: finishes[options.finish], roughness: options.finish === "metal" ? 0.4 : 0.9, metalness: options.finish === "metal" ? 0.2 : 0, side: THREE.DoubleSide, polygonOffset: true, polygonOffsetFactor: -2, polygonOffsetUnits: -2 });
    const accentMaterial = new THREE.MeshStandardMaterial({ color: 0x237f83, roughness: 0.8 });
    const trimMaterial = new THREE.MeshStandardMaterial({ color: 0xfff4dc, roughness: 0.8 });
    const baseMaterial = new THREE.MeshStandardMaterial({ color: 0x697681, roughness: 1 });
    const pavingMaterial = new THREE.MeshStandardMaterial({ color: 0xcac1af, roughness: 1 });
    const lawnMaterial = new THREE.MeshStandardMaterial({ color: theme === "dark" ? 0x3f715e : 0x76a65a, roughness: 1 });
    const box = (width: number, height: number, depth: number, x: number, y: number, z: number, material: THREE.Material) => {
      const mesh = new THREE.Mesh(new THREE.BoxGeometry(width, height, depth), material);
      mesh.position.set(x, y, z); mesh.castShadow = true; mesh.receiveShadow = true;
      scene.add(mesh);
      return mesh;
    };
    const { lengthM: length, widthM: width, wallHeightM: height } = options;
    const siteMargin = Math.max(options.overhangM + 0.6, Math.min(span * 0.18, 3));
    box(length + siteMargin * 2, 0.14, width + siteMargin * 2, 0, -0.29, 0, lawnMaterial);
    box(length + 0.7, 0.12, width + 0.7, 0, -0.16, 0, pavingMaterial);
    box(length + 0.12, 0.1, width + 0.12, 0, -0.05, 0, baseMaterial);
    box(length, height, width, 0, height / 2, 0, wallMaterial);
    // Surface accents are decorative only; they do not imply plan-verified openings.
    box(length * 0.28, height * 0.88, 0.035, length * 0.16, height * 0.48, -width / 2 - 0.018, accentMaterial);
    box(0.035, height * 0.88, width * 0.24, length / 2 + 0.018, height * 0.48, width * 0.12, accentMaterial);
    for (const x of [-length / 2, length / 2]) {
      box(0.06, height, width + 0.08, x, height / 2, 0, trimMaterial);
    }
    box(length + 0.1, 0.09, width + 0.1, 0, height - 0.045, 0, trimMaterial);
    const addMesh = (positions: number[], material: THREE.Material) => {
      if (!positions.length) return;
      const geometry = new THREE.BufferGeometry();
      geometry.setAttribute("position", new THREE.Float32BufferAttribute(positions, 3)); geometry.computeVertexNormals();
      const mesh = new THREE.Mesh(geometry, material);
      mesh.castShadow = true; mesh.receiveShadow = true;
      scene.add(mesh);
      return mesh;
    };
    const roof = addMesh(data.roof, roofMaterial); addMesh(data.infill, wallMaterial);
    const edgeMaterial = new THREE.LineBasicMaterial({ color: 0x24495b });
    if (roof) scene.add(new THREE.LineSegments(new THREE.EdgesGeometry(roof.geometry, 25), edgeMaterial));
    const camera = new THREE.PerspectiveCamera(45, 1, 0.1, span * 30);
    const orbit = new OrbitControls(camera, renderer.domElement);
    orbit.enableDamping = true; orbit.minDistance = span * 0.3; orbit.maxDistance = span * 12; orbit.maxPolarAngle = Math.PI / 2 - 0.02;
    const reset = () => {
      const distance = span * 1.8 / Math.min(camera.aspect, 1);
      camera.position.set(distance * 0.8, (height + data.rise) / 2 + distance * 0.5, -distance);
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
      [wallMaterial, roofMaterial, accentMaterial, trimMaterial, baseMaterial, pavingMaterial, lawnMaterial, edgeMaterial].forEach((material) => material.dispose());
      light.shadow.dispose();
      renderer.dispose(); renderer.domElement.remove();
    };
  }, [options, theme, attempt]);

  return <div className="exterior-render">
    {failed && <div className="alert alert--warning" role="status">3D rendering is unavailable. Roof settings and saved options remain available below.<button type="button" className="button button--ghost button--small" onClick={() => { setFailed(false); setAttempt((current) => current + 1); }}>Retry 3D</button></div>}
    <div className="exterior-host" ref={host} role="img" aria-label="Conceptual exterior preview" hidden={failed} />
    {!failed && <><p className="cell-note exterior-orbit-hint">Drag to orbit · Scroll or pinch to zoom</p><div className="exterior-tools"><button type="button" className="icon-button" title="Reset exterior view" aria-label="Reset exterior view" onClick={() => commands.current?.reset()}><RotateCcw size={17} /></button><button type="button" className="icon-button" title="Download exterior screenshot" aria-label="Download exterior screenshot" onClick={() => commands.current?.screenshot()}><Camera size={17} /></button></div></>}
  </div>;
}