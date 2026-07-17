import { useReducedMotion } from "motion/react";

const pages = [
  [58, 82],
  [104, 54],
  [150, 102],
  [196, 68],
  [242, 122],
  [288, 88],
  [334, 142],
  [380, 110],
  [426, 164],
  [472, 130],
  [518, 186],
  [564, 150],
  [86, 204],
  [132, 170],
  [178, 224],
  [224, 190],
  [270, 244],
  [316, 210],
  [362, 264],
  [408, 230],
  [454, 282],
  [500, 248],
  [546, 302],
  [592, 268],
] as const;

const activePages = [
  { delay: 5.4, index: 1, travel: 20 },
  { delay: 0, index: 4, travel: 22 },
  { delay: 1.8, index: 10, travel: 18 },
  { delay: 3.6, index: 18, travel: 24 },
  { delay: 7.2, index: 22, travel: 21 },
] as const;

export function KVFlow() {
  const prefersReducedMotion = useReducedMotion();
  const motionMode = prefersReducedMotion ? "reduced" : "full";

  return (
    <svg
      className="home-kv-flow"
      viewBox="0 0 650 360"
      aria-hidden="true"
      focusable="false"
      data-motion={motionMode}
    >
      <path
        className="kv-flow-path kv-flow-path-muted"
        d="M26 255 C154 112 254 300 370 172 C458 76 526 124 626 54"
      />
      <path
        className="kv-flow-path kv-flow-path-accent kv-flow-path-moving"
        d="M26 255 C154 112 254 300 370 172 C458 76 526 124 626 54"
      />

      <g className="kv-flow-pages">
        {pages.map(([x, y]) => (
          <rect key={`${x}-${y}`} x={x} y={y} width="13" height="13" rx="2" />
        ))}
      </g>

      <g className="kv-flow-active-pages">
        {activePages.map(({ delay, index, travel }) => {
          const [x, y] = pages[index];

          return (
            <rect
              className="kv-flow-moving-page"
              key={index}
              x={x}
              y={y}
              width="13"
              height="13"
              rx="2"
              opacity={prefersReducedMotion ? 0.72 : 0.25}
              transform={prefersReducedMotion ? `translate(${travel} 0)` : undefined}
            >
              {!prefersReducedMotion && (
                <>
                  <animateTransform
                    attributeName="transform"
                    type="translate"
                    values={`0 0; ${travel} -8; 0 0`}
                    begin={`${delay}s`}
                    dur="7.2s"
                    repeatCount="indefinite"
                  />
                  <animate
                    attributeName="opacity"
                    values="0.25; 0.95; 0.25"
                    begin={`${delay}s`}
                    dur="7.2s"
                    repeatCount="indefinite"
                  />
                </>
              )}
            </rect>
          );
        })}
      </g>
    </svg>
  );
}
