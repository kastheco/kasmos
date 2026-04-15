// Registers a hook that stubs CSS module imports so tsx-based tests can import
// React components using CSS Modules without a bundler.
// Usage: node --import ./src/css-mock-loader.mjs --import tsx/esm <script>

import { register } from "node:module";

register(new URL("./css-mock-hooks.mjs", import.meta.url));
