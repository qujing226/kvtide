import type { MetricsFrame } from "./prometheus";

export const DEFAULT_HISTORY_WINDOW_MS = 15 * 60 * 1000;
export const DEFAULT_MAX_HISTORY_POINTS = 450;

export function appendFrame(
  history: MetricsFrame[],
  next: MetricsFrame,
  windowMs = DEFAULT_HISTORY_WINDOW_MS,
  maxPoints = DEFAULT_MAX_HISTORY_POINTS,
): MetricsFrame[] {
  const oldestTimestamp = next.timestamp - windowMs;
  const withinWindow = [...history, next].filter(
    (frame) => frame.timestamp >= oldestTimestamp,
  );

  return withinWindow.slice(-maxPoints);
}
