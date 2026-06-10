import { describe, expect, it } from "vitest";

import { completeBatch, scheduleStep, type WorkItem } from "./model";

const work = (
  workId: string,
  phase: WorkItem["phase"],
  tokens: number,
  decodeRound: WorkItem["decodeRound"] = phase === "decode" ? 1 : 0,
): WorkItem => ({
  workId,
  requestId: workId.split("-").slice(0, 2).join("-"),
  parentWorkId: null,
  requestLabel: workId,
  phase,
  tokens,
  decodeRound,
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

    expect(result.selected.map((item) => item.workId)).toEqual([
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

    expect(result.selected.map((item) => item.workId)).toEqual(["decode-1"]);
    expect(result.skipped).toEqual([
      {
        item: expect.objectContaining({ workId: "prefill-large" }),
        reason: "token-budget",
      },
    ]);
  });

  it("turns completed prefill work into the first decode round", () => {
    const prefill = work("req-a-prefill", "prefill", 16, 0);

    const result = completeBatch([prefill]);

    expect(result.generated).toEqual([
      expect.objectContaining({
        requestId: prefill.requestId,
        parentWorkId: prefill.workId,
        phase: "decode",
        decodeRound: 1,
        tokens: 1,
      }),
    ]);
    expect(result.finishedRequestIds).toEqual([]);
  });

  it("runs decode for two rounds and then finishes the request", () => {
    const decodeOne = work("req-a-decode-1", "decode", 1, 1);

    const afterFirstRound = completeBatch([decodeOne]);
    expect(afterFirstRound.generated).toEqual([
      expect.objectContaining({
        requestId: decodeOne.requestId,
        parentWorkId: decodeOne.workId,
        decodeRound: 2,
      }),
    ]);
    expect(afterFirstRound.finishedRequestIds).toEqual([]);

    const afterSecondRound = completeBatch(afterFirstRound.generated);
    expect(afterSecondRound.generated).toEqual([]);
    expect(afterSecondRound.finishedRequestIds).toEqual([
      decodeOne.requestId,
    ]);
  });
});
