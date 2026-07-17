import { render, screen, waitFor, within } from "@testing-library/react";
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
    expect(within(navigation).getByRole("link", { name: "Docs" })).not.toHaveAttribute(
      "target",
    );
    expect(within(navigation).getByRole("link", { name: "GitHub" })).toHaveAttribute(
      "href",
      "https://github.com/qujing226/kvtide",
    );
    expect(
      within(navigation).getByRole("link", { name: "GitHub" }),
    ).not.toHaveAttribute("target");
  });

  it.each([
    ["/demo", "Demo"],
    ["/lab", "Lab"],
    ["/blog", "Blog"],
    ["/blog/paged-kv-cache", "Blog"],
  ])("marks %s as the current navigation route", (route, linkName) => {
    renderApp(route);

    const navigation = within(screen.getByRole("banner")).getByRole("navigation");
    expect(within(navigation).getByRole("link", { name: linkName })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });

  it.each([
    "/demo/invalid",
    "/lab/invalid",
    "/blog/paged-kv-cache/invalid",
  ])("does not mark %s as a current navigation route", (route) => {
    renderApp(route);

    const navigation = within(screen.getByRole("banner")).getByRole("navigation");
    expect(
      within(navigation).queryByRole("link", { current: "page" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Page not found" }),
    ).toBeInTheDocument();
  });

  it("keeps the shell mounted and manages route focus and title", async () => {
    const user = userEvent.setup();
    renderApp("/demo");

    const banner = screen.getByRole("banner");
    const main = screen.getByRole("main");
    const footer = screen.getByRole("contentinfo");
    const demoHeading = screen.getByRole("heading", { name: "Demo" });

    expect(document.title).toBe("Demo | KVTide");
    expect(demoHeading).not.toHaveFocus();

    await user.click(
      within(banner).getByRole("link", { name: "Lab" }),
    );

    expect(screen.getByRole("heading", { name: "Demo" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Lab" })).not.toBeInTheDocument();

    const labHeading = await screen.findByRole("heading", { name: "Lab" });

    expect(screen.queryByRole("heading", { name: "Demo" })).not.toBeInTheDocument();
    await waitFor(() => expect(labHeading).toHaveFocus());
    expect(document.title).toBe("Lab | KVTide");
    expect(screen.getByRole("banner")).toBe(banner);
    expect(screen.getByRole("main")).toBe(main);
    expect(screen.getByRole("contentinfo")).toBe(footer);

    await user.click(within(banner).getByRole("link", { name: "Blog" }));

    const blogHeading = await screen.findByRole("heading", { name: "Blog" });
    await waitFor(() => expect(blogHeading).toHaveFocus());
    expect(document.title).toBe("Blog | KVTide");
    expect(within(banner).getByRole("link", { name: "Blog" })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });

  it("integrates the Home route with its title, content, and navigation focus", async () => {
    const user = userEvent.setup();
    renderApp("/demo");

    await user.click(
      within(screen.getByRole("banner")).getByRole("link", { name: "KVTide" }),
    );

    const homeHeading = await screen.findByRole("heading", {
      level: 1,
      name: /KV-aware LLM serving/i,
    });

    await waitFor(() => expect(homeHeading).toHaveFocus());
    expect(document.title).toBe("KVTide");
    expect(
      screen.getByText(/KVTide is an open inference system for exploring scheduling/i),
    ).toBeInTheDocument();
  });

  it("renders the not found page and footer for an unknown route", () => {
    renderApp("/missing");

    expect(
      screen.getByRole("heading", { name: "Page not found" }),
    ).toBeInTheDocument();
    expect(document.title).toBe("Page not found | KVTide");
    expect(screen.getByRole("contentinfo")).toBeInTheDocument();
  });
});
