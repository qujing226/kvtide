import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [react()],
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes("react-markdown") || id.includes("remark-") || id.includes("unified")) {
            return "markdown";
          }
          if (id.includes("@connectrpc") || id.includes("@bufbuild")) {
            return "connect";
          }
          if (id.includes("motion") || id.includes("framer-motion")) {
            return "motion";
          }
          if (id.includes("node_modules")) {
            return "vendor";
          }
        },
      },
    },
  },
  server: {
    host: "0.0.0.0",
    port: 5173,
    proxy: {
      "/kvtide.v1.InferenceService": {
        target: "http://127.0.0.1:8800",
      },
      "/api/metrics": {
        target: "http://127.0.0.1:8801",
        rewrite: () => "/metrics",
      },
    },
  },
  test: {
    environment: "jsdom",
    setupFiles: "./src/test/setup.ts",
    css: true,
  },
});
