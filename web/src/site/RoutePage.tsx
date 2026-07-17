import { useEffect, useRef, type ReactNode } from "react";

import { PageTransition } from "./PageTransition";

type RoutePageProps = {
  children?: ReactNode;
  focusOnMount: boolean;
  title: string;
};

function formatDocumentTitle(title: string) {
  return title === "KVTide" ? title : `${title} | KVTide`;
}

export function RoutePage({ children, focusOnMount, title }: RoutePageProps) {
  const headingRef = useRef<HTMLHeadingElement>(null);

  useEffect(() => {
    document.title = formatDocumentTitle(title);

    if (focusOnMount) {
      headingRef.current?.focus();
    }
  }, [focusOnMount, title]);

  return (
    <PageTransition>
      <section className="route-page">
        <h1 ref={headingRef} tabIndex={-1}>
          {title}
        </h1>
        {children}
      </section>
    </PageTransition>
  );
}
