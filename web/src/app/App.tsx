import { AnimatePresence } from "motion/react";
import { Route, Routes, useLocation } from "react-router";

import { NotFoundPage } from "../site/NotFoundPage";
import { PageTransition } from "../site/PageTransition";
import { SiteLayout } from "../site/SiteLayout";

function RoutePlaceholder({ title }: { title: string }) {
  return (
    <PageTransition>
      <h1>{title}</h1>
    </PageTransition>
  );
}

export function App() {
  const location = useLocation();

  return (
    <SiteLayout>
      <AnimatePresence initial={false} mode="wait">
        <Routes location={location} key={location.pathname}>
          <Route path="/" element={<RoutePlaceholder title="KVTide" />} />
          <Route path="/demo" element={<RoutePlaceholder title="Demo" />} />
          <Route path="/lab" element={<RoutePlaceholder title="Lab" />} />
          <Route path="/blog" element={<RoutePlaceholder title="Blog" />} />
          <Route path="/blog/:slug" element={<RoutePlaceholder title="Blog" />} />
          <Route path="*" element={<NotFoundPage />} />
        </Routes>
      </AnimatePresence>
    </SiteLayout>
  );
}
