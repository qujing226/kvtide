import "@xyflow/react/dist/style.css";

import {
  Background,
  Controls,
  ReactFlow,
  type Edge,
  type Node,
  type NodeProps,
} from "@xyflow/react";
import { useState } from "react";

import type { RuntimeInfo } from "../api/runtime";

type TopologyPageProps = { executors: RuntimeInfo[] };
type EngineNode = Node<Record<string, never>, "engine">;
type ExecutorNode = Node<{ runtime: RuntimeInfo }, "executor">;
type RuntimeNode = EngineNode | ExecutorNode;

function EngineNodeView() {
  return <div className="topology-node engine-node">Engine</div>;
}

function ExecutorNodeView({ data }: NodeProps<ExecutorNode>) {
  const runtime = data.runtime;
  return (
    <div className="topology-node executor-node">
      <i className="node-status" />
      <strong>{runtime.executorId}</strong>
      <small>{runtime.modelType.toUpperCase()} · {runtime.deviceType.toUpperCase()} · {runtime.numKvBlocks} BLOCKS</small>
    </div>
  );
}

const nodeTypes = { engine: EngineNodeView, executor: ExecutorNodeView };

function buildTopology(executors: RuntimeInfo[]): { nodes: RuntimeNode[]; edges: Edge[] } {
  const nodes: RuntimeNode[] = [
    { id: "engine", type: "engine", position: { x: 80, y: 220 }, data: {} },
    ...executors.map((runtime, index) => ({
      id: runtime.executorId,
      type: "executor" as const,
      position: { x: 560, y: 100 + index * 190 },
      data: { runtime },
    })),
  ];
  const edges = executors.map((runtime) => ({
    id: `engine-${runtime.executorId}`,
    source: "engine",
    target: runtime.executorId,
    animated: true,
    style: { stroke: "#3266d5", strokeWidth: 1.8 },
  }));
  return { nodes, edges };
}

function formatBytes(bytes: bigint): string {
  return `${(Number(bytes) / 1_000_000_000).toFixed(2)} GB`;
}

export function TopologyPage({ executors }: TopologyPageProps) {
  const [selected, setSelected] = useState<RuntimeInfo | null>(null);
  const topology = buildTopology(executors);

  return (
    <section className="dashboard-page topology-layout" data-testid="topology-page">
      <article className="topology-panel panel">
        <ReactFlow
          edges={topology.edges}
          fitView
          maxZoom={1.7}
          minZoom={0.45}
          nodes={topology.nodes}
          nodeTypes={nodeTypes}
          nodesDraggable={false}
          onNodeClick={(_, node) => {
            if (node.type === "executor") {
              setSelected((node.data as { runtime: RuntimeInfo }).runtime);
            }
          }}
          proOptions={{ hideAttribution: true }}
        >
          <Background color="#dce4ee" gap={22} size={1} />
          <Controls showInteractive={false} />
        </ReactFlow>
      </article>
      {selected ? (
        <aside className="runtime-drawer panel">
          <span className="drawer-label">Executor</span>
          <strong>{selected.executorId}</strong>
          <dl>
            <div><dt>Model</dt><dd>{selected.modelId}</dd></div>
            <div><dt>Runtime epoch</dt><dd>{selected.runtimeEpoch}</dd></div>
            <div><dt>Device</dt><dd>{selected.deviceType} · {selected.dtype}</dd></div>
            <div><dt>KV cache</dt><dd>{formatBytes(selected.kvCacheBytes)}</dd></div>
            <div><dt>Blocks</dt><dd>{selected.numKvBlocks} × {selected.blockSize}</dd></div>
          </dl>
        </aside>
      ) : null}
    </section>
  );
}
