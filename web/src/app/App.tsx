import { AnimatePresence } from "motion/react";
import { useEffect, useState } from "react";
import { Route, Routes, useLocation } from "react-router";

import { HomePage } from "../home/HomePage";
import { DemoPage } from "../demo/DemoPage";
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
          <Route path="/" element={<HomePage focusOnMount={focusOnMount} />} />
          <Route
            path="/demo"
            element={<DemoPage focusOnMount={focusOnMount} />}
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
