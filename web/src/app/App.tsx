import { AnimatePresence } from "motion/react";
import { useEffect, useState } from "react";
import { Route, Routes, useLocation } from "react-router";

import { BlogArticlePage } from "../blog/BlogArticlePage";
import { BlogIndexPage } from "../blog/BlogIndexPage";
import { DemoPage } from "../demo/DemoPage";
import { HomePage } from "../home/HomePage";
import { LabPage } from "../scheduler/LabPage";
import { NotFoundPage } from "../site/NotFoundPage";
import { SiteLayout } from "../site/SiteLayout";

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
            element={<LabPage focusOnMount={focusOnMount} />}
          />
          <Route
            path="/blog"
            element={<BlogIndexPage focusOnMount={focusOnMount} />}
          />
          <Route
            path="/blog/:slug"
            element={<BlogArticlePage focusOnMount={focusOnMount} />}
          />
          <Route path="*" element={<NotFoundPage focusOnMount={focusOnMount} />} />
        </Routes>
      </AnimatePresence>
    </SiteLayout>
  );
}
