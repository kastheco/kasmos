// ESM loader hooks that stub out CSS module imports.
// Registered by css-mock-loader.mjs via node:module register().

export function load(url, context, nextLoad) {
  if (url.endsWith(".css")) {
    return {
      format: "module",
      shortCircuit: true,
      source: "export default new Proxy({}, { get: (_, k) => String(k) });",
    };
  }
  return nextLoad(url, context);
}
