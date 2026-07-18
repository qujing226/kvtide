import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { MetricsPage } from "./MetricsPage";

describe("MetricsPage", () => {
  it("groups the complete metric catalog instead of stacking every chart", async () => {
    const user = userEvent.setup();
    render(<MetricsPage history={[]} />);

    expect(screen.getByRole("tab", { name: "Serving" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByText("Inference requests")).toBeVisible();
    expect(screen.getByText("Request duration")).toBeVisible();
    expect(screen.getByText("TTFT")).toBeVisible();
    expect(screen.getByText("TBT")).toBeVisible();

    await user.click(screen.getByRole("tab", { name: "Cache" }));

    expect(screen.getByText("KV blocks")).toBeVisible();
    expect(screen.getByText("Prefix cache hit rate")).toBeVisible();
    expect(screen.queryByText("Request duration")).not.toBeInTheDocument();
  });
});
