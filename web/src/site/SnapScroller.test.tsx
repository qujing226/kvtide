import { render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SnapScroller } from "./SnapScroller";

type ObserverCallback = IntersectionObserverCallback;

const observers: ObserverCallback[] = [];

class IntersectionObserverStub {
  constructor(callback: ObserverCallback) {
    observers.push(callback);
  }

  observe() {}
  disconnect() {}
  unobserve() {}
  takeRecords() { return []; }
  readonly root = null;
  readonly rootMargin = "0px";
  readonly thresholds = [0.55];
}

function renderScroller() {
  return render(
    <SnapScroller className="test-scroller">
      <section data-snap-screen>
        <h1 data-reveal>First</h1>
      </section>
      <section data-snap-screen>
        <h2 data-reveal>Second</h2>
      </section>
    </SnapScroller>,
  );
}

describe("SnapScroller", () => {
  beforeEach(() => {
    observers.length = 0;
    vi.stubGlobal("IntersectionObserver", IntersectionObserverStub);
    vi.stubGlobal("matchMedia", vi.fn().mockReturnValue({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    }));
  });

  it("marks the visible screen active for staggered reveals", () => {
    const { container } = renderScroller();
    const screens = container.querySelectorAll<HTMLElement>("[data-snap-screen]");

    expect(screens[0]).toHaveClass("is-snap-active");
    expect(screens[1]).not.toHaveClass("is-snap-active");

    observers[0]?.(
      [
        { target: screens[0]!, isIntersecting: false, intersectionRatio: 0.2 },
        { target: screens[1]!, isIntersecting: true, intersectionRatio: 0.8 },
      ] as unknown as IntersectionObserverEntry[],
      {} as IntersectionObserver,
    );

    expect(screens[0]).not.toHaveClass("is-snap-active");
    expect(screens[1]).toHaveClass("is-snap-active");
  });

  it("keeps native wheel speed and slowly settles after scrolling stops", () => {
    vi.useFakeTimers();
    try {
      const { container } = renderScroller();
      const scroller = container.querySelector<HTMLElement>(".test-scroller")!;
      const screens = scroller.querySelectorAll<HTMLElement>("[data-snap-screen]");
      Object.defineProperty(screens[0], "offsetTop", { value: 65 });
      Object.defineProperty(screens[1], "offsetTop", { value: 1065 });
      scroller.style.scrollPaddingTop = "65px";
      scroller.scrollTop = 400;

      const animationFrames: FrameRequestCallback[] = [];
      vi.stubGlobal("requestAnimationFrame", vi.fn((callback: FrameRequestCallback) => {
        animationFrames.push(callback);
        return animationFrames.length;
      }));
      vi.stubGlobal("cancelAnimationFrame", vi.fn());
      const event = new WheelEvent("wheel", { deltaY: -80, cancelable: true });

      scroller.dispatchEvent(event);

      expect(event.defaultPrevented).toBe(false);
      expect(scroller.style.scrollSnapType).toBe("none");
      expect(animationFrames).toHaveLength(0);

      vi.advanceTimersByTime(499);
      expect(animationFrames).toHaveLength(0);

      vi.advanceTimersByTime(1);
      expect(animationFrames).toHaveLength(1);

      animationFrames.shift()?.(0);
      animationFrames.shift()?.(900);

      expect(scroller.scrollTop).toBe(0);
      expect(scroller.style.scrollSnapType).toBe("");
    } finally {
      vi.useRealTimers();
    }
  });
});
