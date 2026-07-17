import { motion, useReducedMotion } from "motion/react";

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
  { delay: 0, index: 4, travel: 22 },
  { delay: 1.8, index: 10, travel: 18 },
  { delay: 3.6, index: 18, travel: 24 },
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
        className="kv-flow-path kv-flow-path-accent"
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
            <motion.g
              key={index}
              initial={false}
              animate={
                prefersReducedMotion
                  ? { opacity: 0.72, x: travel }
                  : {
                      opacity: [0.25, 0.95, 0.25],
                      x: [0, travel, 0],
                    }
              }
              transition={
                prefersReducedMotion
                  ? { duration: 0 }
                  : {
                      delay,
                      duration: 7.2,
                      ease: "easeInOut",
                      repeat: Infinity,
                    }
              }
            >
              <rect x={x} y={y} width="13" height="13" rx="2" />
            </motion.g>
          );
        })}
      </g>
    </svg>
  );
}
