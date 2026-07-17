import { useEffect, useRef } from "react";
import { Link } from "react-router";

import { PageTransition } from "../site/PageTransition";
import { KVFlow } from "./KVFlow";

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
      <div className="home-page">
        <section className="home-intro" aria-labelledby="home-title">
          <div className="home-intro-copy">
            <p className="home-eyebrow">OPEN INFERENCE RUNTIME</p>
            <h1 id="home-title" ref={headingRef} tabIndex={-1}>
              KV-aware LLM serving, built from the runtime up.
            </h1>
            <p className="home-summary">
              KVTide is an open inference system for exploring scheduling,
              execution, and the ownership of paged KV cache across a Go control
              plane and model executors.
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

        <section className="home-vision" aria-labelledby="vision-title">
          <p className="home-eyebrow">VISION</p>
          <div>
            <h2 id="vision-title">
              KV cache should move toward available compute.
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
