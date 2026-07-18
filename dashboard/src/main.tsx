import "@fontsource-variable/geist/wght.css";
import "@fontsource-variable/geist-mono/wght.css";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router";

import { App } from "./app/App";
import { RuntimeDataProvider } from "./runtime/RuntimeData";
import "./styles.css";

const root = document.getElementById("root");

if (!root) {
  throw new Error("root element not found");
}

createRoot(root).render(
  <StrictMode>
    <BrowserRouter>
      <RuntimeDataProvider>
        <App />
      </RuntimeDataProvider>
    </BrowserRouter>
  </StrictMode>,
);
