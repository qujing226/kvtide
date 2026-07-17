import { useEffect, useRef } from "react";

import { PageTransition } from "../site/PageTransition";
import { SnapScroller } from "../site/SnapScroller";
import { SchedulerLab } from "./SchedulerLab";
import "./lab.css";

type LabPageProps = {
  focusOnMount: boolean;
};

export function LabPage({ focusOnMount }: LabPageProps) {
  const headingRef = useRef<HTMLHeadingElement>(null);

  useEffect(() => {
    document.title = "Lab | KVTide";

    if (focusOnMount) {
      headingRef.current?.focus();
    }
  }, [focusOnMount]);

  return (
    <PageTransition>
      <SnapScroller className="lab-scroll-container">
        <section className="lab-screen" data-snap-screen aria-labelledby="scheduler-lab-title">
          <div className="scheduler-heading" data-reveal="1">
            <h1 id="scheduler-lab-title" ref={headingRef} tabIndex={-1}>
              schedule · step mode
            </h1>
          </div>
          <SchedulerLab />
        </section>
      </SnapScroller>
    </PageTransition>
  );
}
