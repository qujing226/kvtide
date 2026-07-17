import { AnimatePresence } from "motion/react";
import { useEffect, useState } from "react";
import { Route, Routes, useLocation } from "react-router";

import { NotFoundPage } from "../site/NotFoundPage";
import { RoutePage } from "../site/RoutePage";
import { SiteLayout } from "../site/SiteLayout";

type RoutePlaceholderProps = {
  focusOnMount: boolean;
  title: string;
};

function RoutePlaceholder({ focusOnMount, title }: RoutePlaceholderProps) {
  return <RoutePage title={title} focusOnMount={focusOnMount} />;
}

export function App() {
  const location = useLocation();
  const [initialLocationKey] = useState(location.key);
  const [hasNavigated, setHasNavigated] = useState(false);
  const focusOnMount = hasNavigated || location.key !== initialLocationKey;

  useEffect(() => {
    if (location.key !== initialLocationKey) {
      setHasNavigated(true);
    }
  }, [initialLocationKey, location.key]);

  return (
    <SiteLayout>
      <AnimatePresence initial={false} mode="wait">
        <Routes location={location} key={location.pathname}>
          <Route
            path="/"
            element={<RoutePlaceholder title="KVTide" focusOnMount={focusOnMount} />}
          />
          <Route
            path="/demo"
            element={<RoutePlaceholder title="Demo" focusOnMount={focusOnMount} />}
          />
          <Route
            path="/lab"
            element={<RoutePlaceholder title="Lab" focusOnMount={focusOnMount} />}
          />
          <Route
            path="/blog"
            element={<RoutePlaceholder title="Blog" focusOnMount={focusOnMount} />}
          />
          <Route
            path="/blog/:slug"
            element={<RoutePlaceholder title="Blog" focusOnMount={focusOnMount} />}
          />
          <Route path="*" element={<NotFoundPage focusOnMount={focusOnMount} />} />
        </Routes>
      </AnimatePresence>
    </SiteLayout>
  );
}
