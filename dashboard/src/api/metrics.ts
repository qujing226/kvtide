import { parsePrometheusText, type MetricsFrame } from "../metrics/prometheus";

export type MetricsClient = {
  scrape(): Promise<MetricsFrame>;
};

export function createMetricsClient(
  url = "/api/metrics",
  fetcher: typeof fetch = fetch,
  now: () => number = Date.now,
): MetricsClient {
  return {
    async scrape() {
      const response = await fetcher(url, {
        headers: { Accept: "text/plain" },
      });
      if (!response.ok) {
        throw new Error(`metrics scrape failed: ${response.status}`);
      }

      return {
        timestamp: now(),
        samples: parsePrometheusText(await response.text()),
      };
    },
  };
}

export const metricsClient = createMetricsClient();
