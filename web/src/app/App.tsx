import { useState } from "react";

import { BenchmarkReport } from "../benchmark/BenchmarkReport";
import { SchedulerLab } from "../scheduler/SchedulerLab";
import { copy, type Language } from "./content";

type Mode = "benchmark" | "scheduler" | "trace";

export function App() {
  const [language, setLanguage] = useState<Language>("en");
  const [mode, setMode] = useState<Mode>("benchmark");
  const text = copy[language];

  return (
    <div className="app-shell" data-language={language}>
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark">M</span>
          <div>
            <strong>MINI LLM SERVE</strong>
            <small>{text.lab}</small>
          </div>
        </div>

        <nav aria-label={language === "zh" ? "产品功能" : "Product modes"}>
          <button
            className={mode === "benchmark" ? "active" : ""}
            type="button"
            onClick={() => setMode("benchmark")}
          >
            <span>01</span>
            {text.benchmark}
          </button>
          <button
            className={mode === "scheduler" ? "active" : ""}
            type="button"
            onClick={() => setMode("scheduler")}
          >
            <span>02</span>
            {text.scheduler}
          </button>
          <button
            className={mode === "trace" ? "active" : ""}
            type="button"
            onClick={() => setMode("trace")}
          >
            <span>03</span>
            <span>
              {text.trace}
              <small>{text.traceState}</small>
            </span>
          </button>
        </nav>

        <div className="topology-note">
          <span className="status-dot" />
          <div>
            <strong>{text.status}</strong>
            <small>{text.topology}</small>
          </div>
        </div>
      </aside>

      <section className="content-shell">
        <header className="topbar">
          <span>MINI LLM SERVE / LAB</span>
          <div>
            <a
              href="https://github.com/qujing226/mini-llm-serve"
              target="_blank"
              rel="noreferrer"
            >
              GITHUB ↗
            </a>
            <button
              className="language-button"
              type="button"
              aria-label={language === "en" ? "中文" : "English"}
              onClick={() =>
                setLanguage((current) => (current === "en" ? "zh" : "en"))
              }
            >
              {text.language}
            </button>
          </div>
        </header>

        {mode === "benchmark" && <BenchmarkReport language={language} />}
        {mode === "scheduler" && (
          <SchedulerLab key={language} language={language} />
        )}
        {mode === "trace" && (
          <main className="page trace-placeholder">
            <div className="trace-grid" />
            <div className="eyebrow">
              {language === "zh" ? "需要请求级事件" : "REQUEST-LEVEL EVENTS REQUIRED"}
            </div>
            <h1>{language === "zh" ? "等待真实请求轨迹" : "Waiting for real traces"}</h1>
            <p>
              {language === "zh"
                ? "当前 streaming API 不包含 prefill、decode、prefix lookup 等逐请求事件。Trace API 就绪后再接入真实数据。"
                : "The current streaming API does not expose per-request prefill, decode, or prefix lookup events. This view waits for a real Trace API."}
            </p>
          </main>
        )}
      </section>
    </div>
  );
}
