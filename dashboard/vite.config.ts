import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [react()],
  server: {
    host: "0.0.0.0",
    port: 5174,
    proxy: {
      "/kvtide.v1.AdminService": {
        target: "http://127.0.0.1:8801",
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
