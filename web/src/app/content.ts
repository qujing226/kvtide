export type Language = "en" | "zh";

export const copy = {
  en: {
    lab: "LLM serving control plane",
    playground: "Playground",
    scheduler: "Scheduler Lab",
    benchmark: "Benchmark",
    status: "Interactive model",
    language: "中文",
    topology: "ONE CONTROL PLANE · ONE EXECUTOR",
  },
  zh: {
    lab: "LLM 推理控制平面",
    playground: "请求体验",
    scheduler: "调度实验室",
    benchmark: "基准测试",
    status: "交互模型",
    language: "EN",
    topology: "单控制平面 · 单执行器",
  },
} satisfies Record<Language, Record<string, string>>;
