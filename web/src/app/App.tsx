import { useState } from "react";

import { BenchmarkReport } from "../benchmark/BenchmarkReport";
import { Playground } from "../playground/Playground";
import { SchedulerLab } from "../scheduler/SchedulerLab";
import { copy, type Language } from "./content";

type Mode = "playground" | "benchmark" | "scheduler";

export function App() {
  const [language, setLanguage] = useState<Language>("en");
  const [mode, setMode] = useState<Mode>("playground");
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const text = copy[language];

  return (
    <div
      className={`app-shell ${sidebarCollapsed ? "sidebar-collapsed" : ""}`}
      data-language={language}
    >
      <aside className="sidebar">
        <div className="brand">
          <img
            className="brand-mark"
            src="/favicon.svg"
            alt=""
            aria-hidden="true"
          />
          <div className="brand-copy">
            <strong>MINI LLM SERVE</strong>
            <small>{text.lab}</small>
          </div>
        </div>
        <button
          className="drawer-toggle"
          type="button"
          aria-label={
            sidebarCollapsed ? "Expand navigation" : "Collapse navigation"
          }
          onClick={() => setSidebarCollapsed((current) => !current)}
        >
          {sidebarCollapsed ? "→" : "←"}
        </button>

        <nav aria-label={language === "zh" ? "产品功能" : "Product modes"}>
          <button
            className={mode === "playground" ? "active" : ""}
            type="button"
            onClick={() => setMode("playground")}
          >
            <span>01</span>
            <span className="nav-label">{text.playground}</span>
          </button>
          <button
            className={mode === "scheduler" ? "active" : ""}
            type="button"
            onClick={() => setMode("scheduler")}
          >
            <span>02</span>
            <span className="nav-label">{text.scheduler}</span>
          </button>
          <button
            className={mode === "benchmark" ? "active" : ""}
            type="button"
            onClick={() => setMode("benchmark")}
          >
            <span>03</span>
            <span className="nav-label">{text.benchmark}</span>
          </button>
        </nav>

        <div className="topology-note">
          <span className="status-dot" />
          <div className="topology-copy">
            <strong>{text.status}</strong>
            <small>{text.topology}</small>
          </div>
        </div>
      </aside>

      <section className="content-shell">
        <header className="topbar">
          <span>MINI LLM SERVE / CONTROL PLANE</span>
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

        {mode === "playground" && <Playground language={language} />}
        {mode === "benchmark" && <BenchmarkReport language={language} />}
        {mode === "scheduler" && (
          <SchedulerLab key={language} language={language} />
        )}
      </section>
    </div>
  );
}
