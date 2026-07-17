import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import { App } from "./App";

function renderApp(route: string) {
  return render(
    <MemoryRouter initialEntries={[route]}>
      <App />
    </MemoryRouter>,
  );
}

describe("App", () => {
  it("renders the public navigation on the demo route", () => {
    renderApp("/demo");

    const banner = screen.getByRole("banner");
    const navigation = within(banner).getByRole("navigation");

    expect(navigation).toBeVisible();
    expect(within(banner).getByRole("link", { name: "KVTide" })).toHaveAttribute(
      "href",
      "/",
    );
    expect(within(navigation).getByRole("link", { name: "Demo" })).toHaveAttribute(
      "href",
      "/demo",
    );
    expect(within(navigation).getByRole("link", { name: "Lab" })).toHaveAttribute(
      "href",
      "/lab",
    );
    expect(within(navigation).getByRole("link", { name: "Blog" })).toHaveAttribute(
      "href",
      "/blog",
    );
    expect(within(navigation).getByRole("link", { name: "Docs" })).toHaveAttribute(
      "href",
      "https://github.com/qujing226/kvtide#readme",
    );
    expect(within(navigation).getByRole("link", { name: "GitHub" })).toHaveAttribute(
      "href",
      "https://github.com/qujing226/kvtide",
    );
  });

  it("keeps the footer mounted across routes", async () => {
    const user = userEvent.setup();
    renderApp("/demo");

    const footer = screen.getByRole("contentinfo");
    await user.click(
      within(screen.getByRole("banner")).getByRole("link", { name: "Lab" }),
    );

    expect(await screen.findByRole("heading", { name: "Lab" })).toBeInTheDocument();
    expect(screen.getByRole("contentinfo")).toBe(footer);
  });

  it("renders the not found page and footer for an unknown route", () => {
    renderApp("/missing");

    expect(
      screen.getByRole("heading", { name: "Page not found" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("contentinfo")).toBeInTheDocument();
  });
});
