// This suite reads the widget sources off disk, so it needs node builtins even
// though tsconfig.app.json is the browser project and declares no node types.
// The imports carried `@ts-expect-error` for that reason; they no longer suppress
// anything (tsc resolves `node:fs` to `any` here rather than erroring) and TS2578
// failed the build. Nothing outside __tests__ may import these.
import { readFileSync, readdirSync, statSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { expect, it } from "vitest";

it("authority-boundary invariant", () => {
  const root = join(dirname(fileURLToPath(import.meta.url)), "..");
  const files: string[] = [];
  const walk = (dir: string) => readdirSync(dir).forEach((name: string) => { const path = join(dir, name); if (statSync(path).isDirectory() && name !== "__tests__") walk(path); else if (/\.tsx?$/.test(path)) files.push(path); });
  walk(root);
  const source = files.map((path) => readFileSync(path, "utf8")).join("\n");
  expect(source).not.toMatch(/fetch\s*\(|XMLHttpRequest|WebSocket|open_monitor/);
  expect([...source.matchAll(/["']([a-z_]+monitor)["']/g)].map((match) => match[1]).filter((name) => name !== "kasmos monitor")).toEqual(["refresh_monitor"]);
});
