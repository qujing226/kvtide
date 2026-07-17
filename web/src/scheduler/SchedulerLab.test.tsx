import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  BATCH_EXECUTION_MS,
  SchedulerLab,
  SCHEDULE_TRANSITION_MS,
} from "./SchedulerLab";

describe("SchedulerLab", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("keeps the selected batch empty until scheduling finishes", async () => {
    render(<SchedulerLab />);

    const selectedBatch = screen.getByRole("region", { name: "Selected batch" });
    expect(within(selectedBatch).getByText("0")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Run next step" }));

    expect(within(selectedBatch).getByText("0")).toBeInTheDocument();
    expect(screen.getByText("Request A · Decode 1/2").closest(".work-pill")).toHaveClass(
      "work-transitioning",
    );

    act(() => {
      vi.advanceTimersByTime(SCHEDULE_TRANSITION_MS);
    });

    expect(within(selectedBatch).getByText("3")).toBeInTheDocument();
    expect(
      within(selectedBatch).getByText("Request A · Decode 1/2"),
    ).toBeInTheDocument();
  });

  it("automatically returns successor work to the waiting queue after two seconds", () => {
    render(<SchedulerLab />);
    const schedule = screen.getByRole("button", { name: "Run next step" });

    fireEvent.click(schedule);
    act(() => {
      vi.advanceTimersByTime(SCHEDULE_TRANSITION_MS);
    });

    const selectedBatch = screen.getByRole("region", { name: "Selected batch" });
    expect(within(selectedBatch).getByText("3")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Run next step" }),
    ).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(BATCH_EXECUTION_MS - 1);
    });
    expect(within(selectedBatch).getByText("3")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(
      within(selectedBatch)
        .getByText("Request A · Decode 1/2")
        .closest(".work-pill"),
    ).toHaveClass("work-transitioning");

    act(() => {
      vi.advanceTimersByTime(SCHEDULE_TRANSITION_MS);
    });

    const waitingQueue = screen.getByRole("region", { name: "Waiting queue" });
    expect(within(selectedBatch).getByText("0")).toBeInTheDocument();
    expect(
      within(waitingQueue).getByText("Request A · Decode 2/2"),
    ).toBeInTheDocument();
    expect(
      within(waitingQueue).getByText("Request B · Decode 1/2"),
    ).toBeInTheDocument();
    expect(
      within(waitingQueue).getByText(/REQ-A · D2\/2 · FROM D1/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Run next step" }),
    ).toBeInTheDocument();
  });

  it("lets the user finish the selected batch before the timer expires", () => {
    render(<SchedulerLab />);

    fireEvent.click(screen.getByRole("button", { name: "Run next step" }));
    act(() => {
      vi.advanceTimersByTime(SCHEDULE_TRANSITION_MS);
    });

    fireEvent.click(screen.getByRole("button", { name: "Run next step" }));

    const selectedBatch = screen.getByRole("region", { name: "Selected batch" });
    expect(
      within(selectedBatch)
        .getByText("Request B · Short prompt")
        .closest(".work-pill"),
    ).toHaveClass("work-transitioning");

    act(() => {
      vi.advanceTimersByTime(SCHEDULE_TRANSITION_MS);
    });

    const waitingQueue = screen.getByRole("region", { name: "Waiting queue" });
    expect(within(selectedBatch).getByText("0")).toBeInTheDocument();
    expect(
      within(waitingQueue).getByText("Request B · Decode 1/2"),
    ).toBeInTheDocument();
    expect(
      within(waitingQueue).queryByText("Request B · Decode 2/2"),
    ).not.toBeInTheDocument();
  });
});
