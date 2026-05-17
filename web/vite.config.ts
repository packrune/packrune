/* SPDX-License-Identifier: AGPL-3.0-only WITH Commons-Clause */
/* Copyright (C) 2026 Packrune Contributors */

import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { fileURLToPath } from "node:url";

// The dev server proxies API and Docker registry traffic to the Go backend
// so the frontend can be developed without HTTPS or CORS headaches.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    port: 5173,
    proxy: {
      "/api": "http://localhost:8080",
      "/v2": "http://localhost:8080",
      "/healthz": "http://localhost:8080",
      "/readyz": "http://localhost:8080",
      "/version": "http://localhost:8080",
    },
  },
  build: {
    // Output directly into the Go package that embeds us so a single
    // `make build` produces a self-contained binary with the latest UI.
    outDir: "../internal/web/dist",
    emptyOutDir: true,
    sourcemap: true,
  },
});
