import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { LabPage } from "./LabPage";

describe("LabPage", () => {
  it("uses the public Lab identity without legacy stage or localization copy", () => {
    render(<LabPage focusOnMount={false} />);

    expect(screen.getByRole("heading", { name: "Lab", level: 1 })).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Waiting queue" })).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Selected batch" })).toBeInTheDocument();
    expect(screen.queryByText(/stage\s*3/i)).not.toBeInTheDocument();
    expect(document.body.textContent).not.toMatch(/[\u4e00-\u9fff]/);
  });
});
