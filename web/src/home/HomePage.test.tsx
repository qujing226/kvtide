import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { HomePage } from "./HomePage";

const { reducedMotion } = vi.hoisted(() => ({
  reducedMotion: vi.fn(() => true),
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
    reducedMotion.mockReturnValue(true);
  });

  it("introduces KVTide with the primary project actions", () => {
    renderHome();

    expect(
      screen.getByRole("heading", { level: 1, name: /KV-aware/i }),
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
        name: "KV cache should move toward available compute.",
      }),
    ).toBeInTheDocument();
  });

  it("renders KV flow as reduced-motion decorative identity without architecture labels", () => {
    const { container } = renderHome();
    const flow = container.querySelector("svg.home-kv-flow");

    expect(reducedMotion).toHaveBeenCalled();
    expect(flow).toHaveAttribute("aria-hidden", "true");
    expect(flow).toHaveAttribute("focusable", "false");
    expect(flow).toHaveAttribute("data-motion", "reduced");
    expect(flow).not.toHaveTextContent(/server|executor|scheduler/i);
  });
});
