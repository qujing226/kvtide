import { useState } from "react";

import type { Language } from "../app/content";
import {
  scheduleStep,
  type ScheduleResult,
  type WorkItem,
  type WorkPhase,
} from "./model";

interface SchedulerLabProps {
  language: Language;
}

const presets: Array<{
  label: Record<Language, string>;
  phase: WorkPhase;
  tokens: number;
}> = [
  {
    label: { en: "Short prefill", zh: "短提示词预填充" },
    phase: "prefill",
    tokens: 8,
  },
  {
    label: { en: "Long prefill", zh: "长提示词预填充" },
    phase: "prefill",
    tokens: 24,
  },
  {
    label: { en: "Decode step", zh: "单步解码" },
    phase: "decode",
    tokens: 1,
  },
];

const initialQueue = (language: Language): WorkItem[] => [
  {
    id: "decode-01",
    label: language === "zh" ? "请求 A · 第 7 个 token" : "Request A · token 7",
    phase: "decode",
    tokens: 1,
  },
  {
    id: "prefill-01",
    label: language === "zh" ? "请求 B · 短提示词" : "Request B · short",
    phase: "prefill",
    tokens: 8,
  },
  {
    id: "prefill-02",
    label: language === "zh" ? "请求 C · 长提示词" : "Request C · long",
    phase: "prefill",
    tokens: 24,
  },
  {
    id: "decode-02",
    label: language === "zh" ? "请求 D · 第 3 个 token" : "Request D · token 3",
    phase: "decode",
    tokens: 1,
  },
];

function WorkPill({ item, language }: { item: WorkItem; language: Language }) {
  return (
    <div className={`work-pill work-${item.phase}`}>
      <span>{item.phase === "decode" ? "D" : "P"}</span>
      <div>
        <strong>{item.label}</strong>
        <small>
          {language === "zh"
            ? `${item.tokens} 个 token`
            : `${item.tokens} tokens`}
        </small>
      </div>
    </div>
  );
}

export function SchedulerLab({ language }: SchedulerLabProps) {
  const isZh = language === "zh";
  const [queue, setQueue] = useState(() => initialQueue(language));
  const [maxSequences, setMaxSequences] = useState(3);
  const [maxTokens, setMaxTokens] = useState(16);
  const [lastResult, setLastResult] = useState<ScheduleResult | null>(null);
  const [nextID, setNextID] = useState(5);

  const addWork = (preset: (typeof presets)[number]) => {
    setQueue((current) => [
      ...current,
      {
        id: `${preset.phase}-${nextID}`,
        label: `${preset.label[language]} · #${nextID}`,
        phase: preset.phase,
        tokens: preset.tokens,
      },
    ]);
    setNextID((current) => current + 1);
  };

  const runStep = () => {
    const result = scheduleStep(queue, { maxSequences, maxTokens });
    const selectedIDs = new Set(result.selected.map((item) => item.id));
    setLastResult(result);
    setQueue((current) => current.filter((item) => !selectedIDs.has(item.id)));
  };

  const reset = () => {
    setQueue(initialQueue(language));
    setLastResult(null);
    setNextID(5);
  };

  return (
    <main className="page scheduler-page">
      <section className="scheduler-heading">
        <div>
          <div className="eyebrow">
            {isZh ? "TOKEN-AWARE 调度 · 单步模式" : "TOKEN-AWARE SCHEDULING · STEP MODE"}
          </div>
          <h1>{isZh ? "组合下一批任务" : "Compose the next batch"}</h1>
          <p>
            {isZh
              ? "Decode 优先保证流式进度，剩余 token budget 再吸收合适的 prefill 工作。"
              : "Decode protects streaming progress; remaining token budget admits prefill work that still fits."}
          </p>
        </div>
        <div className="budget-dial">
          <span>{isZh ? "剩余预算" : "REMAINING"}</span>
          <strong>{lastResult?.remainingTokens ?? maxTokens}</strong>
          <small>tokens</small>
        </div>
      </section>

      <section className="scheduler-layout">
        <aside className="control-panel">
          <div className="section-kicker">{isZh ? "调度预算" : "BUDGET"}</div>
          <label>
            <span>{isZh ? "最大序列数" : "Max sequences"}</span>
            <output>{maxSequences}</output>
            <input
              aria-label="Max sequences"
              type="range"
              min="1"
              max="8"
              value={maxSequences}
              onChange={(event) => setMaxSequences(Number(event.target.value))}
            />
          </label>
          <label>
            <span>{isZh ? "最大 token 数" : "Max tokens"}</span>
            <output>{maxTokens}</output>
            <input
              aria-label="Max tokens"
              type="range"
              min="4"
              max="48"
              step="4"
              value={maxTokens}
              onChange={(event) => setMaxTokens(Number(event.target.value))}
            />
          </label>

          <div className="section-kicker preset-title">
            {isZh ? "添加任务" : "ADD WORK"}
          </div>
          <div className="preset-buttons">
            {presets.map((preset) => (
              <button
                key={preset.label.en}
                type="button"
                aria-label={
                  isZh
                    ? `添加${preset.label.zh}`
                    : `Add ${preset.label.en}`
                }
                onClick={() => addWork(preset)}
              >
                <span>{preset.phase === "decode" ? "D" : "P"}</span>
                <strong>{preset.label[language]}</strong>
                <small>
                  {isZh
                    ? `${preset.tokens} 个 token`
                    : `${preset.tokens} tokens`}
                </small>
              </button>
            ))}
          </div>

          <button
            className="run-button"
            type="button"
            onClick={runStep}
            disabled={queue.length === 0}
          >
            {isZh ? "调度下一批" : "Schedule next batch"}
          </button>
          <button className="reset-button" type="button" onClick={reset}>
            {isZh ? "重置实验" : "Reset experiment"}
          </button>
        </aside>

        <div className="queue-workspace">
          <section className="queue-column">
            <header>
              <div>
                <span>01</span>
                <h2>{isZh ? "等待队列" : "Waiting queue"}</h2>
              </div>
              <strong>{queue.length}</strong>
            </header>
            <div className="work-stack">
              {queue.length > 0 ? (
                queue.map((item) => (
                  <WorkPill item={item} key={item.id} language={language} />
                ))
              ) : (
                <p className="empty-state">
                  {isZh ? "队列已清空。" : "The queue is empty."}
                </p>
              )}
            </div>
          </section>

          <section className="queue-column selected-column">
            <header>
              <div>
                <span>02</span>
                <h2>{isZh ? "选中批次" : "Selected batch"}</h2>
              </div>
              <strong>{lastResult?.selected.length ?? 0}</strong>
            </header>
            <div className="work-stack">
              {lastResult && lastResult.selected.length > 0 ? (
                lastResult.selected.map((item) => (
                  <WorkPill item={item} key={item.id} language={language} />
                ))
              ) : (
                <p className="empty-state">
                  {isZh
                    ? "调整预算，然后执行一个调度 step。"
                    : "Tune the budget, then run one scheduling step."}
                </p>
              )}
            </div>
          </section>
        </div>
      </section>

      <section className="decision-ledger">
        <div>
          <span>{isZh ? "调度决策" : "DECISION LEDGER"}</span>
          <strong>
            {lastResult
              ? isZh
                ? `选中 ${lastResult.selected.length} 项 · 延后 ${lastResult.skipped.length} 项`
                : `${lastResult.selected.length} selected · ${lastResult.skipped.length} deferred`
              : isZh
                ? "尚未执行调度"
                : "No scheduling decision yet"}
          </strong>
        </div>
        <p>
          {lastResult && lastResult.skipped.length > 0
            ? lastResult.skipped
                .map(
                  ({ item, reason }) =>
                    isZh
                      ? `${item.label}：${
                          reason === "token-budget"
                            ? "超出 token 预算"
                            : "超出序列预算"
                        }`
                      : `${item.label}: ${reason.replace("-", " ")}`,
                )
                .join(" · ")
            : isZh
              ? "无法放入当前 budget 的工作会保留在队列中。"
              : "Work that cannot fit the current budget remains queued."}
        </p>
      </section>
    </main>
  );
}
