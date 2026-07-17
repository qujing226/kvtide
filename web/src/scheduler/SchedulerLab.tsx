import { useEffect, useRef, useState, type CSSProperties } from "react";

import {
  completeBatch,
  scheduleStep,
  type CompletionResult,
  type ScheduleResult,
  type WorkItem,
  type WorkPhase,
} from "./model";

export const SCHEDULE_TRANSITION_MS = 360;
export const BATCH_EXECUTION_MS = 2_000;

type TransitionDirection = "to-selected" | "to-waiting" | null;

const presets: Array<{
  label: string;
  phase: WorkPhase;
  tokens: number;
}> = [
  {
    label: "Short prefill",
    phase: "prefill",
    tokens: 8,
  },
  {
    label: "Long prefill",
    phase: "prefill",
    tokens: 24,
  },
  {
    label: "Decode step",
    phase: "decode",
    tokens: 1,
  },
];

const initialQueue = (): WorkItem[] => [
  {
    workId: "REQ-A-decode-1",
    requestId: "REQ-A",
    parentWorkId: null,
    requestLabel: "Request A",
    phase: "decode",
    tokens: 1,
    decodeRound: 1,
  },
  {
    workId: "REQ-B-prefill",
    requestId: "REQ-B",
    parentWorkId: null,
    requestLabel: "Request B",
    detail: "Short prompt",
    phase: "prefill",
    tokens: 8,
    decodeRound: 0,
  },
  {
    workId: "REQ-C-prefill",
    requestId: "REQ-C",
    parentWorkId: null,
    requestLabel: "Request C",
    detail: "Long prompt",
    phase: "prefill",
    tokens: 24,
    decodeRound: 0,
  },
  {
    workId: "REQ-D-decode-1",
    requestId: "REQ-D",
    parentWorkId: null,
    requestLabel: "Request D",
    phase: "decode",
    tokens: 1,
    decodeRound: 1,
  },
];

function requestColor(requestId: string) {
  const palette = ["#3266d5", "#2f7c6d", "#52637a", "#8b5d5a", "#7b6a2f"];
  const hash = Array.from(requestId).reduce(
    (total, character) => total + character.charCodeAt(0),
    0,
  );
  return palette[hash % palette.length];
}

function workLabel(item: WorkItem) {
  if (item.phase === "prefill") {
    return `${item.requestLabel} · ${item.detail ?? "Prefill"}`;
  }
  return `${item.requestLabel} · Decode ${item.decodeRound}/2`;
}

function workLineage(item: WorkItem) {
  if (item.phase === "prefill") {
    return `${item.requestId} · PREFILL`;
  }

  const parent =
    item.decodeRound === 1 && item.parentWorkId ? " · FROM P" :
    item.decodeRound === 2 ? " · FROM D1" :
    "";
  return `${item.requestId} · D${item.decodeRound}/2${parent}`;
}

function WorkPill({
  item,
  transitioning = false,
  arriving = false,
}: {
  item: WorkItem;
  transitioning?: boolean;
  arriving?: boolean;
}) {
  return (
    <div
      className={[
        "work-pill",
        `work-${item.phase}`,
        transitioning ? "work-transitioning" : "",
        arriving ? "work-arriving" : "",
      ].filter(Boolean).join(" ")}
      style={
        { "--request-accent": requestColor(item.requestId) } as CSSProperties
      }
    >
      <span>{item.phase === "decode" ? "D" : "P"}</span>
      <div>
        <strong>{workLabel(item)}</strong>
        <small>
          {workLineage(item)} · {item.tokens} tokens
        </small>
      </div>
    </div>
  );
}

export function SchedulerLab() {
  const [queue, setQueue] = useState(initialQueue);
  const [maxSequences, setMaxSequences] = useState(3);
  const [maxTokens, setMaxTokens] = useState(16);
  const [lastResult, setLastResult] = useState<ScheduleResult | null>(null);
  const [lastCompletion, setLastCompletion] =
    useState<CompletionResult | null>(null);
  const [selectedBatch, setSelectedBatch] = useState<WorkItem[]>([]);
  const [transitioningWorkIds, setTransitioningWorkIds] = useState<string[]>(
    [],
  );
  const [arrivingWorkIds, setArrivingWorkIds] = useState<string[]>([]);
  const [transitionDirection, setTransitionDirection] =
    useState<TransitionDirection>(null);
  const [nextID, setNextID] = useState(5);
  const transitionTimer = useRef<number | null>(null);
  const executionTimer = useRef<number | null>(null);
  const isTransitioning = transitionDirection !== null;

  useEffect(
    () => () => {
      if (transitionTimer.current !== null) {
        window.clearTimeout(transitionTimer.current);
      }
      if (executionTimer.current !== null) {
        window.clearTimeout(executionTimer.current);
      }
    },
    [],
  );

  const addWork = (preset: (typeof presets)[number]) => {
    const requestId = `REQ-${nextID}`;
    setQueue((current) => [
      ...current,
      {
        workId: `${requestId}-${preset.phase}`,
        requestId,
        parentWorkId: null,
        requestLabel: `Request ${nextID}`,
        detail: preset.phase === "prefill" ? preset.label : undefined,
        phase: preset.phase,
        tokens: preset.tokens,
        decodeRound: preset.phase === "decode" ? 1 : 0,
      },
    ]);
    setNextID((current) => current + 1);
  };

  function completeSelectedBatch(batch: WorkItem[]) {
    if (batch.length === 0) {
      return;
    }

    if (executionTimer.current !== null) {
      window.clearTimeout(executionTimer.current);
      executionTimer.current = null;
    }

    const completion = completeBatch(batch);
    setLastCompletion(completion);
    setTransitionDirection("to-waiting");
    setTransitioningWorkIds(batch.map((item) => item.workId));
    setArrivingWorkIds([]);

    transitionTimer.current = window.setTimeout(() => {
      setSelectedBatch([]);
      setQueue((current) => [...current, ...completion.generated]);
      setTransitioningWorkIds([]);
      setArrivingWorkIds(
        completion.generated.map((item) => item.workId),
      );
      setTransitionDirection(null);
      transitionTimer.current = null;
    }, SCHEDULE_TRANSITION_MS);
  }

  function scheduleWaitingQueue() {
    const result = scheduleStep(queue, {
      maxSequences,
      maxTokens,
    });
    const selectedWorkIds = result.selected.map((item) => item.workId);
    const selectedSet = new Set(selectedWorkIds);

    setLastResult(result);
    setArrivingWorkIds([]);

    if (selectedWorkIds.length === 0) {
      return;
    }

    setTransitioningWorkIds(selectedWorkIds);
    setTransitionDirection("to-selected");
    transitionTimer.current = window.setTimeout(() => {
      setQueue((current) =>
        current.filter((item) => !selectedSet.has(item.workId)),
      );
      setSelectedBatch(result.selected);
      setTransitioningWorkIds([]);
      setArrivingWorkIds(selectedWorkIds);
      setTransitionDirection(null);
      transitionTimer.current = null;

      executionTimer.current = window.setTimeout(() => {
        completeSelectedBatch(result.selected);
      }, BATCH_EXECUTION_MS);
    }, SCHEDULE_TRANSITION_MS);
  }

  const runStep = () => {
    if (isTransitioning) {
      return;
    }

    if (selectedBatch.length > 0) {
      completeSelectedBatch(selectedBatch);
      return;
    }

    scheduleWaitingQueue();
  };

  const reset = () => {
    if (transitionTimer.current !== null) {
      window.clearTimeout(transitionTimer.current);
      transitionTimer.current = null;
    }
    if (executionTimer.current !== null) {
      window.clearTimeout(executionTimer.current);
      executionTimer.current = null;
    }
    setQueue(initialQueue());
    setLastResult(null);
    setLastCompletion(null);
    setSelectedBatch([]);
    setTransitioningWorkIds([]);
    setArrivingWorkIds([]);
    setTransitionDirection(null);
    setNextID(5);
  };

  return (
    <div className="page scheduler-page">
      <section className="scheduler-heading">
        <div className="eyebrow">
          TOKEN-AWARE SCHEDULING · STEP MODE
        </div>
      </section>

      <section className="scheduler-layout">
        <aside className="control-panel">
          <div className="section-kicker">BUDGET</div>
          <label>
            <span>Max sequences</span>
            <output>{maxSequences}</output>
            <input
              aria-label="Max sequences"
              type="range"
              min="1"
              max="8"
              value={maxSequences}
              disabled={isTransitioning}
              onChange={(event) => setMaxSequences(Number(event.target.value))}
            />
          </label>
          <label>
            <span>Max tokens</span>
            <output>{maxTokens}</output>
            <input
              aria-label="Max tokens"
              type="range"
              min="4"
              max="48"
              step="4"
              value={maxTokens}
              disabled={isTransitioning}
              onChange={(event) => setMaxTokens(Number(event.target.value))}
            />
          </label>

          <div className="section-kicker preset-title">
            ADD WORK
          </div>
          <div className="preset-buttons">
            {presets.map((preset) => (
              <button
                key={preset.label}
                type="button"
                aria-label={`Add ${preset.label}`}
                disabled={isTransitioning}
                onClick={() => addWork(preset)}
              >
                <span>{preset.phase === "decode" ? "D" : "P"}</span>
                <strong>{preset.label}</strong>
                <small>{preset.tokens} tokens</small>
              </button>
            ))}
          </div>

          <button
            className="run-button"
            type="button"
            onClick={runStep}
            disabled={
              isTransitioning ||
              (queue.length === 0 && selectedBatch.length === 0)
            }
          >
            {isTransitioning ? "Moving work" : "Run next step"}
          </button>
          <button className="reset-button" type="button" onClick={reset}>
            Reset experiment
          </button>
        </aside>

        <div
          className={[
            "queue-workspace",
            isTransitioning ? "queue-transitioning" : "",
            transitionDirection === "to-selected" ? "queue-to-selected" : "",
            transitionDirection === "to-waiting" ? "queue-to-waiting" : "",
          ].filter(Boolean).join(" ")}
        >
          <section
            className="queue-column"
            aria-label="Waiting queue"
          >
            <div className="queue-header">
              <div>
                <span>01</span>
                <h2>Waiting queue</h2>
              </div>
              <strong>{queue.length}</strong>
            </div>
            <div className="work-stack">
              {queue.length > 0 ? (
                queue.map((item) => (
                  <WorkPill
                    item={item}
                    key={item.workId}
                    transitioning={
                      transitionDirection === "to-selected" &&
                      transitioningWorkIds.includes(item.workId)
                    }
                    arriving={arrivingWorkIds.includes(item.workId)}
                  />
                ))
              ) : (
                <p className="empty-state">
                  The queue is empty.
                </p>
              )}
            </div>
          </section>

          <section
            className="queue-column selected-column"
            aria-label="Selected batch"
          >
            <div className="queue-header">
              <div>
                <span>02</span>
                <h2>Selected batch</h2>
              </div>
              <strong>{selectedBatch.length}</strong>
            </div>
            <div className="work-stack">
              {selectedBatch.length > 0 ? (
                selectedBatch.map((item) => (
                  <WorkPill
                    item={item}
                    key={item.workId}
                    transitioning={
                      transitionDirection === "to-waiting" &&
                      transitioningWorkIds.includes(item.workId)
                    }
                    arriving={arrivingWorkIds.includes(item.workId)}
                  />
                ))
              ) : (
                <p className="empty-state">
                  Tune the budget, then run one scheduling step.
                </p>
              )}
            </div>
          </section>
        </div>
      </section>

      <section className="decision-ledger">
        <div>
          <span>DECISION LEDGER</span>
          <strong>
            {lastResult
              ? `${lastResult.selected.length} selected · ${lastResult.skipped.length} deferred`
              : "No scheduling decision yet"}
          </strong>
        </div>
        <p>
          {lastResult && lastResult.skipped.length > 0
            ? lastResult.skipped
                .map(
                  ({ item, reason }) =>
                    `${workLabel(item)}: ${reason.replace("-", " ")}`,
                )
                .join(" · ")
            : lastCompletion && lastCompletion.finishedRequestIds.length > 0
              ? `${lastCompletion.finishedRequestIds.join(", ")} completed two Decode rounds.`
              : "Work that cannot fit the current budget remains queued."}
        </p>
      </section>
    </div>
  );
}
