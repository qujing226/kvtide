import { describe, expect, it } from "vitest";

import { scheduleStep, type WorkItem } from "./model";

const work = (
  id: string,
  phase: WorkItem["phase"],
  tokens: number,
): WorkItem => ({
  id,
  label: id,
  phase,
  tokens,
});

describe("scheduleStep", () => {
  it("respects sequence and token budgets", () => {
    const result = scheduleStep(
      [
        work("decode-1", "decode", 1),
        work("prefill-1", "prefill", 12),
        work("decode-2", "decode", 1),
      ],
      { maxSequences: 2, maxTokens: 12 },
    );

    expect(result.selected.map((item) => item.id)).toEqual([
      "decode-1",
      "decode-2",
    ]);
    expect(result.remainingTokens).toBe(10);
  });

  it("mixes decode and prefill work when both fit", () => {
    const result = scheduleStep(
      [
        work("prefill-1", "prefill", 8),
        work("decode-1", "decode", 1),
        work("prefill-2", "prefill", 6),
      ],
      { maxSequences: 2, maxTokens: 10 },
    );

    expect(result.selected.map((item) => item.phase)).toEqual([
      "decode",
      "prefill",
    ]);
    expect(result.remainingTokens).toBe(1);
  });

  it("reports work skipped by the token budget", () => {
    const result = scheduleStep(
      [work("prefill-large", "prefill", 24), work("decode-1", "decode", 1)],
      { maxSequences: 3, maxTokens: 8 },
    );

    expect(result.selected.map((item) => item.id)).toEqual(["decode-1"]);
    expect(result.skipped).toEqual([
      {
        item: expect.objectContaining({ id: "prefill-large" }),
        reason: "token-budget",
      },
    ]);
  });
});
