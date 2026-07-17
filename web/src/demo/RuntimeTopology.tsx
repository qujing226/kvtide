import { useRef, useState, type KeyboardEvent, type PointerEvent, type WheelEvent } from "react";

import type { RuntimeInfo } from "./runtimes";

type RuntimeTopologyProps = {
  active: boolean;
  runtimes: RuntimeInfo[];
  selectedExecutor: string | null;
  onSelectExecutor(executorId: string): void;
};

type ViewState = {
  x: number;
  y: number;
  scale: number;
};

const initialView: ViewState = { x: 0, y: 0, scale: 1 };

function clampScale(scale: number) {
  return Math.min(1.8, Math.max(0.65, scale));
}

export function RuntimeTopology({
  active,
  runtimes,
  selectedExecutor,
  onSelectExecutor,
}: RuntimeTopologyProps) {
  const [view, setView] = useState(initialView);
  const drag = useRef<{ pointerId: number; x: number; y: number } | null>(null);
  const runtime = runtimes[0] ?? null;

  const zoom = (delta: number) => {
    setView((current) => ({ ...current, scale: clampScale(current.scale + delta) }));
  };

  const selectWithKeyboard = (event: KeyboardEvent<SVGGElement>) => {
    if (!runtime || (event.key !== "Enter" && event.key !== " ")) return;
    event.preventDefault();
    onSelectExecutor(runtime.executorId);
  };

  const beginPan = (event: PointerEvent<SVGSVGElement>) => {
    if ((event.target as Element).closest('[role="button"]')) return;
    drag.current = { pointerId: event.pointerId, x: event.clientX, y: event.clientY };
    event.currentTarget.setPointerCapture?.(event.pointerId);
  };

  const movePan = (event: PointerEvent<SVGSVGElement>) => {
    if (!drag.current || drag.current.pointerId !== event.pointerId) return;
    const dx = event.clientX - drag.current.x;
    const dy = event.clientY - drag.current.y;
    drag.current = { pointerId: event.pointerId, x: event.clientX, y: event.clientY };
    setView((current) => ({ ...current, x: current.x + dx, y: current.y + dy }));
  };

  const endPan = (event: PointerEvent<SVGSVGElement>) => {
    if (drag.current?.pointerId === event.pointerId) drag.current = null;
  };

  const wheelZoom = (event: WheelEvent<SVGSVGElement>) => {
    event.preventDefault();
    zoom(event.deltaY < 0 ? 0.1 : -0.1);
  };

  return (
    <div
      className="runtime-topology"
      data-active={active}
      data-scale={view.scale.toFixed(2)}
      data-testid="runtime-topology"
    >
      <div className="topology-controls" aria-label="Topology view controls">
        <button type="button" onClick={() => zoom(0.15)} aria-label="Zoom in">+</button>
        <button type="button" onClick={() => zoom(-0.15)} aria-label="Zoom out">−</button>
        <button type="button" onClick={() => setView(initialView)} aria-label="Reset topology view">Reset</button>
      </div>
      <svg
        viewBox="0 0 960 430"
        role="img"
        aria-labelledby="topology-title topology-description"
        onWheel={wheelZoom}
        onPointerDown={beginPan}
        onPointerMove={movePan}
        onPointerUp={endPan}
        onPointerCancel={endPan}
      >
        <title id="topology-title">KVTide runtime topology</title>
        <desc id="topology-description">Drag to pan and use the controls or wheel to zoom.</desc>
        <g transform={`translate(${view.x} ${view.y}) scale(${view.scale})`}>
          <path className="topology-link-base" d="M 338 215 H 622" />
          <path className="topology-link-flow" d="M 338 215 H 622" />
          {active && <circle className="topology-packet" r="6" cy="215"><animate attributeName="cx" values="338;622" dur="1.5s" repeatCount="indefinite" /></circle>}

          <g className="topology-engine-node">
            <rect x="122" y="154" width="216" height="122" rx="14" />
            <text className="topology-node-kicker" x="150" y="186">CONTROL PLANE</text>
            <text className="topology-node-title" x="150" y="222">Engine</text>
            <text className="topology-node-detail" x="150" y="250">schedule · blocks · stream</text>
          </g>

          <g
            className={`topology-executor-node${runtime && selectedExecutor === runtime.executorId ? " is-selected" : ""}${runtime ? "" : " is-offline"}`}
            role={runtime ? "button" : undefined}
            tabIndex={runtime ? 0 : undefined}
            aria-label={runtime ? `Inspect ${runtime.executorId}` : undefined}
            onClick={() => runtime && onSelectExecutor(runtime.executorId)}
            onKeyDown={selectWithKeyboard}
          >
            <rect x="622" y="154" width="216" height="122" rx="14" />
            <text className="topology-node-kicker" x="650" y="186">MODEL RUNTIME</text>
            <text className="topology-node-title" x="650" y="222">Executor</text>
            <text className="topology-node-detail" x="650" y="250">
              {runtime ? `${runtime.modelType} · ${runtime.deviceType}` : "disconnected"}
            </text>
          </g>
        </g>
      </svg>
      <p className="topology-hint">Drag to pan · scroll to zoom · select the executor for runtime data</p>
    </div>
  );
}
