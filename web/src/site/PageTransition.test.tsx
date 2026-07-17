import { render, screen } from "@testing-library/react";
import type { ComponentProps } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { PageTransition } from "./PageTransition";

const { motionDiv, reducedMotion } = vi.hoisted(() => ({
  motionDiv: vi.fn(),
  reducedMotion: vi.fn(),
}));

vi.mock("motion/react", () => ({
  motion: {
    div: motionDiv,
  },
  useReducedMotion: reducedMotion,
}));

type MotionDivProps = ComponentProps<"div"> & {
  animate?: unknown;
  exit?: unknown;
  initial?: unknown;
  transition?: unknown;
};

describe("PageTransition", () => {
  beforeEach(() => {
    motionDiv.mockReset();
    motionDiv.mockImplementation(
      ({ animate, children, exit, initial, transition, ...props }: MotionDivProps) => (
        <div {...props}>{children}</div>
      ),
    );
    reducedMotion.mockReset();
    reducedMotion.mockReturnValue(false);
  });

  it("provides route animation props in full-motion mode", () => {
    render(<PageTransition>Route content</PageTransition>);

    expect(screen.getByText("Route content")).toBeInTheDocument();
    expect(motionDiv).toHaveBeenCalledOnce();
    expect(motionDiv.mock.calls[0]?.[0]).toEqual(
      expect.objectContaining({
        animate: expect.any(Object),
        exit: expect.any(Object),
        initial: expect.any(Object),
        transition: expect.any(Object),
      }),
    );
  });

  it("omits every route animation prop in reduced-motion mode", () => {
    reducedMotion.mockReturnValue(true);

    render(<PageTransition>Route content</PageTransition>);

    expect(screen.getByText("Route content")).toBeInTheDocument();
    expect(motionDiv).toHaveBeenCalledOnce();

    const props = motionDiv.mock.calls[0]?.[0];
    expect(props).not.toHaveProperty("initial");
    expect(props).not.toHaveProperty("animate");
    expect(props).not.toHaveProperty("exit");
    expect(props).not.toHaveProperty("transition");
  });
});
