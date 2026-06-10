export type WorkPhase = "prefill" | "decode";
export type DecodeRound = 0 | 1 | 2;

export interface WorkItem {
  workId: string;
  requestId: string;
  parentWorkId: string | null;
  requestLabel: string;
  detail?: string;
  phase: WorkPhase;
  tokens: number;
  decodeRound: DecodeRound;
}

export interface SchedulerBudget {
  maxSequences: number;
  maxTokens: number;
}

export interface SkippedWork {
  item: WorkItem;
  reason: "sequence-budget" | "token-budget";
}

export interface ScheduleResult {
  selected: WorkItem[];
  skipped: SkippedWork[];
  remainingTokens: number;
}

export interface CompletionResult {
  generated: WorkItem[];
  finishedRequestIds: string[];
}

export function completeBatch(batch: WorkItem[]): CompletionResult {
  const generated: WorkItem[] = [];
  const finishedRequestIds: string[] = [];

  for (const item of batch) {
    if (item.phase === "prefill") {
      generated.push(createDecodeSuccessor(item, 1));
      continue;
    }

    if (item.decodeRound === 1) {
      generated.push(createDecodeSuccessor(item, 2));
      continue;
    }

    finishedRequestIds.push(item.requestId);
  }

  return { generated, finishedRequestIds };
}

export function scheduleStep(
  queue: WorkItem[],
  budget: SchedulerBudget,
): ScheduleResult {
  const decode = queue.filter((item) => item.phase === "decode");
  const prefill = queue.filter((item) => item.phase === "prefill");
  const ordered: WorkItem[] = [];

  while (decode.length > 0 || prefill.length > 0) {
    const nextDecode = decode.shift();
    if (nextDecode) {
      ordered.push(nextDecode);
    }

    const nextPrefill = prefill.shift();
    if (nextPrefill) {
      ordered.push(nextPrefill);
    }
  }

  const selected: WorkItem[] = [];
  const skipped: SkippedWork[] = [];
  let remainingTokens = budget.maxTokens;

  for (const item of ordered) {
    if (selected.length >= budget.maxSequences) {
      skipped.push({ item, reason: "sequence-budget" });
      continue;
    }

    if (item.tokens > remainingTokens) {
      skipped.push({ item, reason: "token-budget" });
      continue;
    }

    selected.push(item);
    remainingTokens -= item.tokens;
  }

  return { selected, skipped, remainingTokens };
}

function createDecodeSuccessor(
  item: WorkItem,
  decodeRound: 1 | 2,
): WorkItem {
  return {
    workId: `${item.requestId}-decode-${decodeRound}`,
    requestId: item.requestId,
    parentWorkId: item.workId,
    requestLabel: item.requestLabel,
    phase: "decode",
    tokens: 1,
    decodeRound,
  };
}
