export type Language = "en" | "zh";

export const copy = {
  en: {
    lab: "LLM serving control plane",
    benchmark: "Performance",
    scheduler: "Scheduler Lab",
    trace: "Request Trace",
    traceState: "API required",
    status: "Interactive model",
    language: "中文",
    topology: "ONE CONTROL PLANE · ONE EXECUTOR",
  },
  zh: {
    lab: "LLM 推理控制平面",
    benchmark: "性能剖面",
    scheduler: "调度实验室",
    trace: "请求追踪",
    traceState: "需要 Trace API",
    status: "交互模型",
    language: "EN",
    topology: "单控制平面 · 单执行器",
  },
} satisfies Record<Language, Record<string, string>>;
