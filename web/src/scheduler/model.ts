export type WorkPhase = "prefill" | "decode";

export interface WorkItem {
  id: string;
  label: string;
  phase: WorkPhase;
  tokens: number;
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
