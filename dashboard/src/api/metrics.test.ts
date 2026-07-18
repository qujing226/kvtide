import { describe, expect, it, vi } from "vitest";

import { createMetricsClient } from "./metrics";

describe("createMetricsClient", () => {
  it("scrapes a timestamped Prometheus frame", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response("llm_active_requests 3", {
        status: 200,
        headers: { "Content-Type": "text/plain" },
      }),
    );

    const result = await createMetricsClient("/metrics", fetcher, () => 42).scrape();

    expect(fetcher).toHaveBeenCalledWith("/metrics", {
      headers: { Accept: "text/plain" },
    });
    expect(result).toEqual({
      timestamp: 42,
      samples: [{ name: "llm_active_requests", labels: {}, value: 3 }],
    });
  });

  it("rejects unsuccessful scrapes", async () => {
    const fetcher = vi.fn().mockResolvedValue(new Response("no", { status: 503 }));

    await expect(
      createMetricsClient("/metrics", fetcher).scrape(),
    ).rejects.toThrow("metrics scrape failed: 503");
  });
});
