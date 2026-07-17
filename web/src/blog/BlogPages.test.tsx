import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { describe, expect, it } from "vitest";

import { BlogArticlePage } from "./BlogArticlePage";
import { BlogIndexPage } from "./BlogIndexPage";
import type { BlogEntry } from "./entries";

const fixture: BlogEntry = {
  slug: "paged-kv-ownership",
  title: "Paged KV ownership",
  summary: "How KVTide separates logical ownership from physical cache slots.",
  publishedAt: "2026-07-17",
  content: "## Ownership boundary\n\nA request keeps a **logical block table**.",
};

describe("Blog pages", () => {
  it("keeps the public index empty until authored entries are registered", () => {
    render(<BlogIndexPage focusOnMount={false} />);

    expect(screen.queryByRole("heading", { name: "Blog" })).not.toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Blog" })).toBeInTheDocument();
    expect(screen.getByText("No entries yet.")).toBeInTheDocument();
    expect(screen.queryByRole("article")).not.toBeInTheDocument();
  });

  it("renders a registered Markdown entry", () => {
    render(
      <MemoryRouter initialEntries={[`/blog/${fixture.slug}`]}>
        <Routes>
          <Route
            path="/blog/:slug"
            element={
              <BlogArticlePage focusOnMount={false} entries={[fixture]} />
            }
          />
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.getByRole("heading", { name: fixture.title, level: 1 })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Ownership boundary" })).toBeInTheDocument();
    expect(screen.getByText("logical block table").tagName).toBe("STRONG");
  });

  it("uses the site not-found page for an unknown slug", () => {
    render(
      <MemoryRouter initialEntries={["/blog/missing"]}>
        <Routes>
          <Route
            path="/blog/:slug"
            element={<BlogArticlePage focusOnMount={false} entries={[fixture]} />}
          />
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.getByRole("heading", { name: "Page not found" })).toBeInTheDocument();
  });
});
