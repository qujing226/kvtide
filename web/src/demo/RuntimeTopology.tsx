import { motion, useReducedMotion } from "motion/react";

type RuntimeTopologyProps = {
  active: boolean;
};

const packetPath = [96, 326, 556];

export function RuntimeTopology({ active }: RuntimeTopologyProps) {
  const prefersReducedMotion = useReducedMotion();
  const animatePacket = active && !prefersReducedMotion;

  return (
    <div
      className="runtime-topology"
      data-active={active}
      data-testid="runtime-topology"
      aria-label="Browser to Go server to executor topology"
    >
      <svg viewBox="0 0 650 210" role="img" aria-labelledby="topology-title">
        <title id="topology-title">KVTide request topology</title>
        <path className="topology-link" d="M 146 93 H 276 M 376 93 H 506" />
        <motion.circle
          className="topology-packet"
          r="5"
          cy="93"
          initial={false}
          animate={{ cx: animatePacket ? packetPath : active ? 326 : 96 }}
          transition={
            animatePacket
              ? { duration: 1.8, ease: "linear", repeat: Infinity }
              : { duration: 0.18 }
          }
        />
        <g className="topology-node">
          <rect x="46" y="56" width="100" height="74" rx="8" />
          <text x="96" y="89" textAnchor="middle">Browser</text>
          <text className="topology-node-detail" x="96" y="108" textAnchor="middle">
            Connect stream
          </text>
        </g>
        <g className="topology-node">
          <rect x="276" y="56" width="100" height="74" rx="8" />
          <text x="326" y="89" textAnchor="middle">Go server</text>
          <text className="topology-node-detail" x="326" y="108" textAnchor="middle">
            Scheduler
          </text>
        </g>
        <g className="topology-node">
          <rect x="506" y="56" width="100" height="74" rx="8" />
          <text x="556" y="89" textAnchor="middle">Executor</text>
          <text className="topology-node-detail" x="556" y="108" textAnchor="middle">
            Qwen3
          </text>
        </g>
        <text className="topology-caption" x="46" y="174">
          {active ? "Request in flight" : "Runtime ready"}
        </text>
      </svg>
    </div>
  );
}
