import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import type { MetricsClient } from "../api/metrics";
import type { RuntimeInventoryClient } from "../api/runtime";
import { RuntimeDataProvider } from "../runtime/RuntimeData";
import { App } from "./App";

const metrics: MetricsClient = {
  scrape: async () => ({ timestamp: 42, samples: [] }),
};
const inventory: RuntimeInventoryClient = { list: async () => [] };

function renderApp() {
  return render(
    <MemoryRouter initialEntries={["/"]}>
      <RuntimeDataProvider metrics={metrics} inventory={inventory}>
        <App />
      </RuntimeDataProvider>
    </MemoryRouter>,
  );
}

describe("App", () => {
  it("presents the dashboard workspace without inference controls", async () => {
    renderApp();

    expect(screen.getByRole("link", { name: "Overview" })).toBeVisible();
    expect(screen.getByRole("link", { name: "Topology" })).toBeVisible();
    expect(screen.getByRole("link", { name: "Metrics" })).toBeVisible();
    expect(screen.getByRole("link", { name: "Executors" })).toBeVisible();
    expect(
      await screen.findByRole("region", { name: "Runtime summary" }),
    ).toBeVisible();
    expect(document.querySelector(".dashboard-topbar")).not.toBeInTheDocument();
    expect(screen.queryByText("Waiting for first sample")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /send/i })).not.toBeInTheDocument();
  });

  it("switches workspace pages without duplicating a page title", async () => {
    const user = userEvent.setup();
    renderApp();

    await user.click(screen.getByRole("link", { name: "Metrics" }));

    expect(await screen.findByTestId("metrics-page")).toBeVisible();
    expect(screen.getByRole("tab", { name: "Serving" })).toBeVisible();
    expect(screen.queryByRole("heading", { name: "Metrics" })).not.toBeInTheDocument();
  });
});
