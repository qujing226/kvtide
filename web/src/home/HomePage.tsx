import { useEffect, useRef } from "react";
import { Link } from "react-router";

import { PageTransition } from "../site/PageTransition";
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
      <div className="home-page home-scroll-container">
        <section className="home-intro home-screen" aria-labelledby="home-title">
          <div className="home-intro-copy">
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

          <div className="home-visual">
            <KVFlow />
          </div>
        </section>

        <section className="home-vision home-screen" aria-labelledby="vision-title">
          <div className="home-vision-mark">
            <p className="home-vision-title">VISION</p>
            <KVTransfer />
          </div>
          <div className="home-vision-copy">
            <h2 id="vision-title">
              KV cache should automatically move toward available compute.
            </h2>
            <p>
              KVTide is working toward a runtime where compatible executors can
              exchange cache ownership instead of forcing every request to
              recompute its context.
            </p>
          </div>
        </section>
      </div>
    </PageTransition>
  );
}
