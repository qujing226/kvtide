import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { HomePage } from "./HomePage";

const { reducedMotion } = vi.hoisted(() => ({
  reducedMotion: vi.fn(),
}));

vi.mock("motion/react", async (importOriginal) => {
  const actual = await importOriginal<typeof import("motion/react")>();

  return {
    ...actual,
    useReducedMotion: reducedMotion,
  };
});

function renderHome() {
  return render(
    <MemoryRouter>
      <HomePage focusOnMount={false} />
    </MemoryRouter>,
  );
}

describe("HomePage", () => {
  beforeEach(() => {
    reducedMotion.mockReset();
    reducedMotion.mockReturnValue(false);
  });

  it("introduces KVTide with the primary project actions", () => {
    renderHome();

    expect(
      screen.getByRole("heading", { level: 1, name: "KVTide" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "KVTide is a Kubernetes-native LLM serving runtime built from the ground up for cache-aware scheduling and proactive peer-to-peer KV mobility.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Explore Demo" })).toHaveAttribute(
      "href",
      "/demo",
    );
    expect(screen.getByRole("link", { name: "View on GitHub" })).toHaveAttribute(
      "href",
      "https://github.com/qujing226/kvtide",
    );
  });

  it("keeps the home page focused on the approved intro and vision", () => {
    renderHome();

    expect(screen.queryByText(/capabilities/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/latest designs/i)).not.toBeInTheDocument();
    expect(
      screen.getByRole("heading", {
        name: "KV cache should move toward available compute automatically.",
      }),
    ).toBeInTheDocument();
  });

  it("renders KV flow as full-motion decorative identity without architecture labels", () => {
    const { container } = renderHome();
    const flow = container.querySelector("svg.home-kv-flow");

    expect(reducedMotion).toHaveBeenCalled();
    expect(flow).toHaveAttribute("aria-hidden", "true");
    expect(flow).toHaveAttribute("focusable", "false");
    expect(flow).toHaveAttribute("data-motion", "full");
    expect(flow).not.toHaveTextContent(/server|executor|scheduler/i);
    expect(flow?.querySelector(".kv-flow-path-moving")).toBeInTheDocument();
    expect(flow?.querySelectorAll(".kv-flow-moving-page")).toHaveLength(5);
    expect(flow?.querySelectorAll("animateTransform")).toHaveLength(5);
  });

  it("shows the three-stage peer KV transfer vision", () => {
    const { container } = renderHome();
    const transfer = container.querySelector("svg.home-kv-transfer");

    expect(transfer).toHaveAttribute("aria-hidden", "true");
    expect(transfer?.querySelectorAll(".kv-transfer-node")).toHaveLength(3);
    expect(transfer?.querySelectorAll(".kv-transfer-pulse")).toHaveLength(3);
    expect(transfer?.querySelector(".kv-transfer-request-node")).toBeInTheDocument();
    expect(transfer?.querySelectorAll(".kv-transfer-matrix-node")).toHaveLength(2);
    expect(transfer?.querySelectorAll("animateMotion")).toHaveLength(3);
  });

  it("owns the two-screen scroll container without mutating the root element", () => {
    const { container } = renderHome();
    const scrollContainer = container.querySelector(".home-scroll-container");

    expect(scrollContainer).toBeInTheDocument();
    expect(scrollContainer?.querySelectorAll(":scope > .home-screen")).toHaveLength(2);
    expect(scrollContainer?.querySelectorAll("[data-snap-screen]")).toHaveLength(2);
    expect(scrollContainer?.querySelectorAll("[data-reveal]")).toHaveLength(4);
    expect(document.documentElement).not.toHaveClass("home-scroll-snap");
  });

  it("presents Vision with the same display-title treatment as KVTide", () => {
    renderHome();

    expect(screen.getByText("VISION")).toHaveClass("home-vision-title");
    expect(screen.getByText("VISION")).not.toHaveClass("home-eyebrow");
  });

  it("renders KV flow in its static settled frame with reduced motion", () => {
    reducedMotion.mockReturnValue(true);

    const { container } = renderHome();
    const flow = container.querySelector("svg.home-kv-flow");

    expect(reducedMotion).toHaveBeenCalled();
    expect(flow).toHaveAttribute("data-motion", "reduced");
  });
});
