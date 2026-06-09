import type { Language } from "../app/content";
import { benchmarkScenarios } from "./data";

interface BenchmarkReportProps {
  language: Language;
}

function ComparisonBars({
  metric,
  values,
  format,
  labels,
}: {
  metric: string;
  values: number[];
  format: (value: number) => string;
  labels: string[];
}) {
  const max = Math.max(...values);

  return (
    <section className="chart-card">
      <div className="section-kicker">{metric}</div>
      <div className="bar-list">
        {benchmarkScenarios.map((scenario, index) => {
          const value = values[index] ?? 0;
          return (
            <div className="bar-row" key={scenario.id}>
              <span>{labels[index] ?? scenario.shortLabel}</span>
              <div className="bar-track">
                <div
                  className="bar-fill"
                  style={{
                    background: scenario.color,
                    width: `${Math.max(6, (value / max) * 100)}%`,
                  }}
                />
              </div>
              <strong>{format(value)}</strong>
            </div>
          );
        })}
      </div>
    </section>
  );
}

export function BenchmarkReport({ language }: BenchmarkReportProps) {
  const isZh = language === "zh";

  return (
    <main className="page benchmark-page">
      <section className="hero-panel">
        <div>
          <div className="eyebrow">
            {isZh ? "推理控制平面 · 性能对照" : "SERVING CONTROL PLANE · PERFORMANCE"}
          </div>
          <h1>{isZh ? "服务行为剖面" : "Serving behavior"}</h1>
          <p>
            {isZh
              ? "从首 token 延迟、吞吐、批处理形状和 KV block churn 观察系统在不同负载下的行为。"
              : "Read the system through first-token latency, throughput, batch shape, and KV block churn across representative workloads."}
          </p>
        </div>
        <div className="hero-stamp">
          <span>PYTHON</span>
          <strong>MOCK</strong>
          <span>EXECUTOR</span>
        </div>
      </section>

      <section
        className="metric-strip"
        aria-label={isZh ? "性能摘要" : "performance highlights"}
      >
        <article>
          <span>{isZh ? "首 TOKEN 延迟降幅" : "TTFT REDUCTION"}</span>
          <strong>48.1%</strong>
          <small>cache miss → cache hit</small>
        </article>
        <article>
          <span>{isZh ? "节省的 PREFILL TOKEN" : "PREFILL TOKENS SAVED"}</span>
          <strong>80K</strong>
          <small>1,000 warmed requests</small>
        </article>
        <article>
          <span>{isZh ? "压力场景 EVICTION" : "PRESSURE EVICTIONS"}</span>
          <strong>6,164</strong>
          <small>KV cache churn signal</small>
        </article>
      </section>

      <section className="chart-grid">
        <ComparisonBars
          metric={isZh ? "首 TOKEN 延迟" : "TIME TO FIRST TOKEN"}
          values={benchmarkScenarios.map((item) => item.ttft)}
          format={(value) => `${value.toFixed(3)}s`}
          labels={isZh ? ["未命中", "命中", "混合", "压力"] : ["MISS", "HIT", "MIXED", "PRESSURE"]}
        />
        <ComparisonBars
          metric={isZh ? "吞吐" : "THROUGHPUT"}
          values={benchmarkScenarios.map((item) => item.throughput)}
          format={(value) => `${value.toFixed(2)}/s`}
          labels={isZh ? ["未命中", "命中", "混合", "压力"] : ["MISS", "HIT", "MIXED", "PRESSURE"]}
        />
        <ComparisonBars
          metric={isZh ? "平均批次大小" : "AVERAGE BATCH"}
          values={benchmarkScenarios.map((item) => item.batch)}
          format={(value) => value.toFixed(2)}
          labels={isZh ? ["未命中", "命中", "混合", "压力"] : ["MISS", "HIT", "MIXED", "PRESSURE"]}
        />
        <ComparisonBars
          metric={isZh ? "KV BLOCK 淘汰次数" : "KV BLOCK EVICTIONS"}
          values={benchmarkScenarios.map((item) => item.evictions)}
          format={(value) => value.toLocaleString()}
          labels={isZh ? ["未命中", "命中", "混合", "压力"] : ["MISS", "HIT", "MIXED", "PRESSURE"]}
        />
      </section>

      <section className="evidence-grid">
        <article className="finding finding-green">
          <span>{isZh ? "01 / 前缀缓存" : "01 / PREFIX CACHE"}</span>
          <h2>{isZh ? "省掉的是 prefill 工作" : "The saved work is prefill"}</h2>
          <p>
            {isZh
              ? "Cache hit 将平均 TTFT 从 1.6175s 降到 0.8390s，同时吞吐提升到 4.90 req/s。"
              : "Cache hits move average TTFT from 1.6175s to 0.8390s while throughput rises to 4.90 req/s."}
          </p>
        </article>
        <article className="finding finding-red">
          <span>{isZh ? "02 / BLOCK 压力" : "02 / BLOCK PRESSURE"}</span>
          <h2>{isZh ? "内存压力会破坏批处理形状" : "Memory pressure reshapes the batch"}</h2>
          <p>
            {isZh
              ? "平均 batch size 降到 3.38，eviction 上升到 6,164；这比单看吞吐更能说明 KV churn。"
              : "Average batch size falls to 3.38 while evictions reach 6,164, exposing KV churn beyond throughput alone."}
          </p>
        </article>
      </section>

      <p className="method-note">
        {isZh
          ? "说明：这些数字来自 Python mock executor，衡量的是 serving control-plane 行为，不代表 GPU 推理性能。"
          : "Method note: results use the Python mock executor and describe serving control-plane behavior, not GPU inference performance."}
      </p>
    </main>
  );
}
