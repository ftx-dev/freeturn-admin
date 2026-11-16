import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// В разработке API - на :8080 (go run), фронт - на :5173 с прокси.
export default defineConfig({
  plugins: [react()],
  server: { proxy: { "/api": "http://127.0.0.1:8080" } },
  build: { outDir: "dist", emptyOutDir: true },
});
