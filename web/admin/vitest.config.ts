import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    setupFiles: "./src/test/setup.ts",
    css: true,
    passWithNoTests: true,
    // Exclude the pure-script tsx tests — those run via npm run test:unit, not vitest.
    exclude: [
      "**/node_modules/**",
      "src/api.test.ts",
      "src/hooks/useAutoRefresh.test.ts",
      "src/pages/agentCardModel.test.ts",
      "src/components/TerminalPreview.test.tsx",
    ],
  },
});
