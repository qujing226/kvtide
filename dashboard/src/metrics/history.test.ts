import { describe, expect, it } from "vitest";

import { appendFrame } from "./history";
import type { MetricsFrame } from "./prometheus";

const frame = (timestamp: number): MetricsFrame => ({ timestamp, samples: [] });

describe("appendFrame", () => {
  it("keeps only the configured time window", () => {
    const history = [frame(0), frame(100), frame(200)];

    expect(appendFrame(history, frame(1_000), 500, 10)).toEqual([
      frame(1_000),
    ]);
  });

  it("caps the number of browser samples", () => {
    const history = [frame(100), frame(200), frame(300)];

    expect(appendFrame(history, frame(400), 10_000, 3)).toEqual([
      frame(200),
      frame(300),
      frame(400),
    ]);
  });
});
