import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { LabPage } from "./LabPage";

describe("LabPage", () => {
  it("renders the scheduler as the only screen in an extensible lab container", () => {
    const { container } = render(<LabPage focusOnMount={false} />);

    expect(
      screen.getByRole("heading", { name: "schedule · step mode", level: 1 }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Lab" })).not.toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Waiting queue" })).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Selected batch" })).toBeInTheDocument();
    expect(container.querySelector(".lab-scroll-container")).toBeInTheDocument();
    expect(container.querySelectorAll(".lab-screen")).toHaveLength(1);
    expect(container.querySelector(".lab-screen[data-snap-screen]")).toBeInTheDocument();
    expect(container.querySelectorAll(".lab-screen [data-reveal]")).toHaveLength(2);
    expect(screen.queryByText("DECISION LEDGER")).not.toBeInTheDocument();
    expect(screen.queryByText(/stage\s*3/i)).not.toBeInTheDocument();
    expect(document.body.textContent).not.toMatch(/[\u4e00-\u9fff]/);
  });
});
