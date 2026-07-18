import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";

import { metricsClient, type MetricsClient } from "../api/metrics";
import {
  runtimeInventoryClient,
  type RuntimeInfo,
  type RuntimeInventoryClient,
} from "../api/runtime";
import { appendFrame } from "../metrics/history";
import type { MetricsFrame } from "../metrics/prometheus";

export type RuntimeConnection = "connecting" | "connected" | "degraded";

export type RuntimeData = {
  connection: RuntimeConnection;
  executors: RuntimeInfo[];
  history: MetricsFrame[];
  lastUpdated: number | null;
  metricsError: string | null;
  inventoryError: string | null;
};

const RuntimeDataContext = createContext<RuntimeData | null>(null);

type RuntimeDataProviderProps = {
  children: ReactNode;
  metrics?: MetricsClient;
  inventory?: RuntimeInventoryClient;
};

const visibleMetricsIntervalMs = 2_000;
const hiddenMetricsIntervalMs = 10_000;
const inventoryIntervalMs = 10_000;

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export function RuntimeDataProvider({
  children,
  metrics = metricsClient,
  inventory = runtimeInventoryClient,
}: RuntimeDataProviderProps) {
  const [history, setHistory] = useState<MetricsFrame[]>([]);
  const [executors, setExecutors] = useState<RuntimeInfo[]>([]);
  const [inventoryLoaded, setInventoryLoaded] = useState(false);
  const [metricsError, setMetricsError] = useState<string | null>(null);
  const [inventoryError, setInventoryError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    let timer: number | undefined;

    const scrape = async () => {
      try {
        const frame = await metrics.scrape();
        if (!cancelled) {
          setHistory((current) => appendFrame(current, frame));
          setMetricsError(null);
        }
      } catch (error) {
        if (!cancelled) {
          setMetricsError(errorMessage(error));
        }
      } finally {
        if (!cancelled) {
          timer = window.setTimeout(
            scrape,
            document.hidden ? hiddenMetricsIntervalMs : visibleMetricsIntervalMs,
          );
        }
      }
    };

    void scrape();
    return () => {
      cancelled = true;
      if (timer !== undefined) {
        window.clearTimeout(timer);
      }
    };
  }, [metrics]);

  useEffect(() => {
    let cancelled = false;
    let timer: number | undefined;

    const refresh = async () => {
      try {
        const nextExecutors = await inventory.list();
        if (!cancelled) {
          setExecutors(nextExecutors);
          setInventoryLoaded(true);
          setInventoryError(null);
        }
      } catch (error) {
        if (!cancelled) {
          setInventoryLoaded(true);
          setInventoryError(errorMessage(error));
        }
      } finally {
        if (!cancelled) {
          timer = window.setTimeout(refresh, inventoryIntervalMs);
        }
      }
    };

    void refresh();
    return () => {
      cancelled = true;
      if (timer !== undefined) {
        window.clearTimeout(timer);
      }
    };
  }, [inventory]);

  const connection: RuntimeConnection =
    metricsError || inventoryError
      ? "degraded"
      : history.length > 0 && inventoryLoaded
        ? "connected"
        : "connecting";

  return (
    <RuntimeDataContext
      value={{
        connection,
        executors,
        history,
        lastUpdated: history.at(-1)?.timestamp ?? null,
        metricsError,
        inventoryError,
      }}
    >
      {children}
    </RuntimeDataContext>
  );
}

export function useRuntimeData(): RuntimeData {
  const runtime = useContext(RuntimeDataContext);
  if (!runtime) {
    throw new Error("useRuntimeData must be used within RuntimeDataProvider");
  }
  return runtime;
}
