import { useEffect, useRef } from "react";
import { Link } from "react-router";

import { PageTransition } from "../site/PageTransition";
import { SnapScroller } from "../site/SnapScroller";
import { KVFlow } from "./KVFlow";
import { KVTransfer } from "./KVTransfer";

const githubUrl = "https://github.com/qujing226/kvtide";

type HomePageProps = {
  focusOnMount: boolean;
};

export function HomePage({ focusOnMount }: HomePageProps) {
  const headingRef = useRef<HTMLHeadingElement>(null);

  useEffect(() => {
    document.title = "KVTide";

    if (focusOnMount) {
      headingRef.current?.focus();
    }
  }, [focusOnMount]);

  return (
    <PageTransition>
      <SnapScroller className="home-page home-scroll-container">
        <section className="home-intro home-screen" data-snap-screen aria-labelledby="home-title">
          <div className="home-intro-copy" data-reveal="1">
            <p className="home-eyebrow">OPEN INFERENCE RUNTIME</p>
            <h1 id="home-title" ref={headingRef} tabIndex={-1}>
              KVTide
            </h1>
            <p className="home-summary">
              KVTide is a Kubernetes-native LLM serving runtime built from the
              ground up for cache-aware scheduling and proactive peer-to-peer KV
              mobility.
            </p>
            <div className="home-actions">
              <Link className="home-action home-action-primary" to="/demo">
                Explore Demo
              </Link>
              <a className="home-action home-action-secondary" href={githubUrl}>
                View on GitHub
              </a>
            </div>
          </div>

          <div className="home-visual" data-reveal="2">
            <KVFlow />
          </div>
        </section>

        <section className="home-vision home-screen" data-snap-screen aria-labelledby="vision-title">
          <div className="home-vision-mark" data-reveal="1">
            <p className="home-vision-title">VISION</p>
            <KVTransfer />
          </div>
          <div className="home-vision-copy" data-reveal="2">
            <h2 id="vision-title">
              KV cache should move toward available compute automatically.
            </h2>
            <p>
              KVTide is working toward a runtime where compatible executors can
              exchange cache ownership instead of forcing every request to
              recompute its context.
            </p>
          </div>
        </section>
      </SnapScroller>
    </PageTransition>
  );
}
