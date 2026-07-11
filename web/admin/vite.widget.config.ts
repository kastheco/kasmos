import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { resolve } from "node:path";

export default defineConfig({
  plugins: [react()],
  publicDir: false,
  define: { "process.env.NODE_ENV": JSON.stringify("production") },
  build: {
    outDir: "widget-dist",
    emptyOutDir: true,
    cssCodeSplit: false,
    minify: true,
    lib: { entry: resolve(__dirname, "src/widget/main.tsx"), formats: ["es"], fileName: () => "monitor.js" },
    rollupOptions: { output: { inlineDynamicImports: true, assetFileNames: (asset) => asset.name?.endsWith(".css") ? "monitor.css" : "[name][extname]" } },
  },
});
