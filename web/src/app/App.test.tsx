import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { App } from "./App";

describe("App", () => {
  it("navigates between benchmark report and scheduler lab", async () => {
    const user = userEvent.setup();
    render(<App />);

    expect(
      screen.getByRole("heading", { name: /serving behavior/i }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/stage 3/i)).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /scheduler lab/i }));

    expect(
      screen.getByRole("heading", { name: /compose the next batch/i }),
    ).toBeInTheDocument();
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
