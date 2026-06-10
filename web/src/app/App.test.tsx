import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { App } from "./App";

describe("App", () => {
  it("orders playground, scheduler, and benchmark navigation", async () => {
    const user = userEvent.setup();
    render(<App />);

    const navigation = screen.getByRole("navigation");
    const buttons = Array.from(navigation.querySelectorAll("button"));
    expect(buttons.map((button) => button.textContent)).toEqual([
      "01Playground",
      "02Scheduler Lab",
      "03Benchmark",
    ]);

    await user.click(screen.getByRole("button", { name: /benchmark/i }));

    expect(screen.getByText(/TTFT REDUCTION/i)).toBeInTheDocument();
  });

  it("collapses and expands the navigation drawer", async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(
      screen.getByRole("button", { name: /collapse navigation/i }),
    );
    expect(document.querySelector(".app-shell")).toHaveClass("sidebar-collapsed");

    await user.click(
      screen.getByRole("button", { name: /expand navigation/i }),
    );
    expect(document.querySelector(".app-shell")).not.toHaveClass(
      "sidebar-collapsed",
    );
  });

  it("localizes scheduler controls in Chinese", async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole("button", { name: "中文" }));
    await user.click(screen.getByRole("button", { name: /调度实验室/i }));

    expect(
      screen.getByRole("button", { name: /添加短提示词预填充/i }),
    ).toBeInTheDocument();
    expect(screen.getByText("尚未执行调度")).toBeInTheDocument();
  });
});
