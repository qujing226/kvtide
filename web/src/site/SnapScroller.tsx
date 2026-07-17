import { useEffect, useRef, type ReactNode } from "react";

const snapRestoreDelayMs = 500;
const snapAnimationDurationMs = 900;

type SnapScrollerProps = {
  children: ReactNode;
  className: string;
};

function ignoresDampedScroll(target: EventTarget | null) {
  return target instanceof Element && target.closest("[data-snap-ignore]") !== null;
}

function nestedScrollerCanMove(
  target: EventTarget | null,
  container: HTMLElement,
  deltaY: number,
) {
  let element = target instanceof HTMLElement ? target : null;

  while (element && element !== container) {
    const overflowY = window.getComputedStyle(element).overflowY;
    const isScrollable = (overflowY === "auto" || overflowY === "scroll")
      && element.scrollHeight > element.clientHeight;

    if (isScrollable) {
      const atTop = element.scrollTop <= 0;
      const atBottom = element.scrollTop + element.clientHeight >= element.scrollHeight - 1;

      if ((deltaY < 0 && !atTop) || (deltaY > 0 && !atBottom)) return true;
    }

    element = element.parentElement;
  }

  return false;
}

function getSnapOffset(container: HTMLElement, screen: HTMLElement) {
  const scrollPaddingTop = Number.parseFloat(
    window.getComputedStyle(container).scrollPaddingTop,
  ) || 0;
  return Math.max(0, screen.offsetTop - scrollPaddingTop);
}

export function SnapScroller({ children, className }: SnapScrollerProps) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const screens = Array.from(
      container.querySelectorAll<HTMLElement>("[data-snap-screen]"),
    );
    screens[0]?.classList.add("is-snap-active");

    const observer = typeof IntersectionObserver === "undefined"
      ? null
      : new IntersectionObserver(
          (entries) => {
            for (const entry of entries) {
              entry.target.classList.toggle(
                "is-snap-active",
                entry.isIntersecting && entry.intersectionRatio >= 0.55,
              );
            }
          },
          { root: container, threshold: [0.55] },
        );
    screens.forEach((screen) => observer?.observe(screen));

    let snapRestoreTimer: number | null = null;
    let snapAnimationFrame: number | null = null;

    const restoreSnap = () => {
      container.style.removeProperty("scroll-behavior");
      container.style.removeProperty("scroll-snap-type");
    };

    const settleToNearestScreen = () => {
      const targetOffset = screens
        .map((screen) => getSnapOffset(container, screen))
        .reduce<number | null>((nearest, offset) => {
          if (nearest === null) return offset;
          return Math.abs(offset - container.scrollTop)
            < Math.abs(nearest - container.scrollTop)
            ? offset
            : nearest;
        }, null);
      if (targetOffset === null) {
        restoreSnap();
        return;
      }

      const start = container.scrollTop;
      const distance = targetOffset - start;
      if (Math.abs(distance) < 1 || window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
        container.scrollTop = targetOffset;
        restoreSnap();
        return;
      }

      let startedAt: number | null = null;
      const animate = (timestamp: number) => {
        startedAt ??= timestamp;
        const progress = Math.min((timestamp - startedAt) / snapAnimationDurationMs, 1);
        const eased = progress < 0.5
          ? 4 * progress ** 3
          : 1 - (-2 * progress + 2) ** 3 / 2;
        container.scrollTop = start + distance * eased;

        if (progress < 1) {
          snapAnimationFrame = window.requestAnimationFrame(animate);
          return;
        }

        container.scrollTop = targetOffset;
        snapAnimationFrame = null;
        restoreSnap();
      };

      snapAnimationFrame = window.requestAnimationFrame(animate);
    };

    const handleWheel = (event: WheelEvent) => {
      if (
        event.ctrlKey
        || Math.abs(event.deltaY) < 1
        || ignoresDampedScroll(event.target)
        || nestedScrollerCanMove(event.target, container, event.deltaY)
      ) return;

      if (snapAnimationFrame !== null) {
        window.cancelAnimationFrame(snapAnimationFrame);
        snapAnimationFrame = null;
      }
      container.style.scrollSnapType = "none";
      container.style.scrollBehavior = "auto";

      if (snapRestoreTimer !== null) window.clearTimeout(snapRestoreTimer);
      snapRestoreTimer = window.setTimeout(() => {
        snapRestoreTimer = null;
        settleToNearestScreen();
      }, snapRestoreDelayMs);
    };
    container.addEventListener("wheel", handleWheel, { passive: false });

    return () => {
      observer?.disconnect();
      container.removeEventListener("wheel", handleWheel);
      if (snapRestoreTimer !== null) window.clearTimeout(snapRestoreTimer);
      if (snapAnimationFrame !== null) window.cancelAnimationFrame(snapAnimationFrame);
      restoreSnap();
    };
  }, []);

  return (
    <div ref={containerRef} className={className}>
      {children}
    </div>
  );
}
