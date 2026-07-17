import { useReducedMotion } from "motion/react";

type TransferPulseProps = {
  path: string;
  keyPoints: string;
  keyTimes: string;
};

function TransferPulse({ path, keyPoints, keyTimes }: TransferPulseProps) {
  return (
    <circle className="kv-transfer-pulse" cx="0" cy="0" r="5">
      <animateMotion
        path={path}
        dur="9s"
        repeatCount="indefinite"
        calcMode="linear"
        keyPoints={keyPoints}
        keyTimes={keyTimes}
      />
      <animate
        attributeName="opacity"
        values="0; 1; 1; 0; 0"
        keyTimes={keyTimes}
        dur="9s"
        repeatCount="indefinite"
      />
    </circle>
  );
}

function Matrix({ x }: { x: number }) {
  const cells = [
    [0, 0],
    [9, 0],
    [18, 0],
    [0, 9],
    [9, 9],
    [18, 9],
  ] as const;

  return (
    <g className="kv-transfer-matrix">
      {cells.map(([dx, dy], index) => (
        <rect
          className={index === 1 || index === 5 ? "is-active" : undefined}
          key={`${dx}-${dy}`}
          x={x + dx}
          y={184 + dy}
          width="6"
          height="6"
          rx="1"
        />
      ))}
    </g>
  );
}

export function KVTransfer() {
  const prefersReducedMotion = useReducedMotion();

  return (
    <svg
      className="home-kv-transfer"
      viewBox="0 0 320 240"
      aria-hidden="true"
      focusable="false"
      data-motion={prefersReducedMotion ? "reduced" : "full"}
    >
      <circle className="kv-transfer-field" cx="160" cy="126" r="109" />

      <g className="kv-transfer-links">
        <path d="M147 70 L87 172" />
        <path d="M97 195 H222" />
        <path d="M173 70 L233 172" />
      </g>

      <g className="kv-transfer-node kv-transfer-request-node">
        <rect x="138" y="26" width="44" height="44" rx="7" />
        <circle cx="150" cy="39" r="3" />
        <path d="M157 39 H172 M148 49 H172 M148 59 H166" />
      </g>

      <g className="kv-transfer-node kv-transfer-matrix-node">
        <rect x="53" y="173" width="44" height="44" rx="7" />
        <Matrix x={65} />
      </g>

      <g className="kv-transfer-node kv-transfer-matrix-node">
        <rect x="223" y="173" width="44" height="44" rx="7" />
        <Matrix x={235} />
      </g>

      {!prefersReducedMotion && (
        <g className="kv-transfer-pulses">
          <TransferPulse
            path="M147 70 L87 172"
            keyPoints="0; 0; 1; 1; 1"
            keyTimes="0; 0.04; 0.24; 0.28; 1"
          />
          <TransferPulse
            path="M97 195 H222"
            keyPoints="0; 0; 0; 1; 1"
            keyTimes="0; 0.33; 0.37; 0.57; 1"
          />
          <TransferPulse
            path="M173 70 L233 172"
            keyPoints="0; 0; 0; 1; 1"
            keyTimes="0; 0.66; 0.7; 0.9; 1"
          />
        </g>
      )}
    </svg>
  );
}
