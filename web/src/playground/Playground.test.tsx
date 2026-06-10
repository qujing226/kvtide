import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { GenerationClient } from "./generation";
import { DEFAULT_PROMPT, Playground } from "./Playground";

describe("Playground", () => {
  it("starts with a cacheable two-block prompt", () => {
    render(
      <Playground
        language="en"
        client={{ generateStream: vi.fn() } as GenerationClient}
      />,
    );

    expect(DEFAULT_PROMPT.trim().split(/\s+/)).toHaveLength(32);
    expect(screen.getByLabelText(/prompt/i)).toHaveValue(DEFAULT_PROMPT);
  });

  it("does not submit a cleared prompt", async () => {
    const user = userEvent.setup();
    const generateStream = vi.fn();
    render(
      <Playground
        language="en"
        client={{ generateStream } as GenerationClient}
      />,
    );

    const submit = screen.getByRole("button", { name: /generate response/i });
    await user.clear(screen.getByLabelText(/prompt/i));
    expect(submit).toBeDisabled();

    await user.type(screen.getByLabelText(/prompt/i), "   ");
    expect(submit).toBeDisabled();
    expect(generateStream).not.toHaveBeenCalled();
  });

  it("streams a response and reports request measurements", async () => {
    const user = userEvent.setup();
    let releaseStream: () => void = () => {};
    const waiting = new Promise<void>((resolve) => {
      releaseStream = resolve;
    });
    const client: GenerationClient = {
      generateStream: () => ({
        async *[Symbol.asyncIterator]() {
          await waiting;
          yield { deltaText: "# Continuous batching\n\n", done: false };
          yield {
            deltaText: "- Preserves **throughput**.",
            done: true,
            outputTokens: 4,
          };
        },
      }),
    };
    render(<Playground language="en" client={client} />);

    await user.clear(screen.getByLabelText(/prompt/i));
    await user.type(
      screen.getByLabelText(/prompt/i),
      "Explain continuous batching.",
    );
    await user.click(
      screen.getByRole("button", { name: /generate response/i }),
    );

    expect(
      screen.getByRole("button", { name: /generating/i }),
    ).toBeDisabled();

    releaseStream();

    expect(
      await screen.findByRole("heading", { name: "Continuous batching" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("list")).toBeInTheDocument();
    expect(screen.getByText("throughput").tagName).toBe("STRONG");
    await waitFor(() => {
      expect(screen.getAllByText("Completed").length).toBeGreaterThan(0);
    });
    expect(screen.getByText("4 tokens")).toBeInTheDocument();
    expect(screen.getByText(/\d+ ms/)).toBeInTheDocument();
  });
});
