import { Activity, Boxes, Gauge, Network } from "lucide-react";
import { lazy, Suspense, type ReactNode } from "react";
import { NavLink, Navigate, Route, Routes } from "react-router";

import { useRuntimeData } from "../runtime/RuntimeData";

const OverviewPage = lazy(() =>
  import("../pages/OverviewPage").then((module) => ({
    default: module.OverviewPage,
  })),
);
const TopologyPage = lazy(() =>
  import("../pages/TopologyPage").then((module) => ({
    default: module.TopologyPage,
  })),
);
const MetricsPage = lazy(() =>
  import("../pages/MetricsPage").then((module) => ({
    default: module.MetricsPage,
  })),
);
const ExecutorsPage = lazy(() =>
  import("../pages/ExecutorsPage").then((module) => ({
    default: module.ExecutorsPage,
  })),
);

type NavigationItem = {
  label: string;
  path: string;
  icon: ReactNode;
};

const navigation: NavigationItem[] = [
  { label: "Overview", path: "/", icon: <Gauge aria-hidden="true" /> },
  { label: "Topology", path: "/topology", icon: <Network aria-hidden="true" /> },
  { label: "Metrics", path: "/metrics", icon: <Activity aria-hidden="true" /> },
  { label: "Executors", path: "/executors", icon: <Boxes aria-hidden="true" /> },
];

function Workspace() {
  const runtime = useRuntimeData();
  const statusLabel =
    runtime.connection === "connected"
      ? "Runtime connected"
      : runtime.connection === "degraded"
        ? "Runtime degraded"
        : "Runtime pending";
  const statusClass =
    runtime.connection === "connected"
      ? ""
      : runtime.connection === "degraded"
        ? " is-error"
        : " is-unknown";

  return (
    <div className="dashboard-shell">
      <aside className="dashboard-sidebar">
        <NavLink className="dashboard-brand" to="/" aria-label="KVTide dashboard">
          <img src="/banner.svg" alt="KVTide" />
        </NavLink>
        <p className="navigation-label">Workspace</p>
        <nav className="dashboard-navigation" aria-label="Dashboard">
          {navigation.map((item) => (
            <NavLink
              className={({ isActive }) =>
                `navigation-item${isActive ? " is-active" : ""}`
              }
              end={item.path === "/"}
              key={item.path}
              to={item.path}
            >
              {item.icon}
              <span>{item.label}</span>
            </NavLink>
          ))}
        </nav>
        <div className="sidebar-runtime">
          <span className={`status-dot${statusClass}`} />
          <div>
            <strong>{statusLabel}</strong>
            <small>{runtime.executors.length} executor(s)</small>
          </div>
        </div>
      </aside>

      <main className="dashboard-main">
        <div className="dashboard-content">
          <Suspense
            fallback={
              <div className="route-loading" role="status">
                Loading workspace
              </div>
            }
          >
            <Routes>
              <Route
                index
                element={
                  <OverviewPage
                    executors={runtime.executors}
                    history={runtime.history}
                  />
                }
              />
              <Route
                path="topology"
                element={<TopologyPage executors={runtime.executors} />}
              />
              <Route
                path="metrics"
                element={<MetricsPage history={runtime.history} />}
              />
              <Route
                path="executors"
                element={<ExecutorsPage executors={runtime.executors} />}
              />
              <Route path="*" element={<Navigate replace to="/" />} />
            </Routes>
          </Suspense>
        </div>
      </main>
    </div>
  );
}

export function App() {
  return <Workspace />;
}
