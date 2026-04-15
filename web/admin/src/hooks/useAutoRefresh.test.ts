// Regression coverage for the selection-switch path in useAutoRefresh.
//
// Bug history: when the input deps changed, the dep-change effect reset
// loading/error/refs but left the previously fetched `data` untouched. The
// admin instances page reads `capture.data` directly, so clicking a different
// row briefly rendered the previous instance's pane under the new title until
// the fresh poll resolved. This test pins down that the dep-change reset
// clears `data` so consumers can never reuse old capture content for a new
// selection.
//
// Runs as a pure script via tsx — no DOM, no React renderer. The hook's
// dep-change reset logic is exposed via `resetAutoRefreshStateForDepsChange`
// precisely so it can be unit-tested without a stateful renderer.

import { resetAutoRefreshStateForDepsChange } from "./useAutoRefresh.ts";

function assertEqual<T>(actual: T, expected: T, msg: string): void {
  if (actual !== expected) {
    throw new Error(
      `${msg}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`,
    );
  }
}

// Selection-switch regression: after a deps change (e.g. the admin instances
// page user clicking a different row), the hook's data slot must be cleared
// so the next render shows null instead of the previous selection's pane.
{
  let data: string | null = "stale pane content from instance A";
  let loading = false;
  let error: string | null = "previous fetch error";
  const generationRef = { current: 5 };
  const inFlightRef = { current: true };
  const hasDataRef = { current: true };

  resetAutoRefreshStateForDepsChange<string>(
    (v) => {
      data = v;
    },
    (v) => {
      loading = v;
    },
    (v) => {
      error = v;
    },
    generationRef,
    inFlightRef,
    hasDataRef,
  );

  assertEqual(
    data,
    null,
    "data cleared on dep change so old capture cannot be reused",
  );
  assertEqual(loading, true, "loading flips to true on dep change");
  assertEqual(error, null, "stale error is cleared on dep change");
  assertEqual(
    generationRef.current,
    6,
    "generationRef bumped so any in-flight previous fetch is discarded",
  );
  assertEqual(
    inFlightRef.current,
    false,
    "inFlightRef cleared so the new fetch can proceed immediately",
  );
  assertEqual(
    hasDataRef.current,
    false,
    "hasDataRef cleared so refresh treats the next fetch as a fresh load",
  );
}

// Reset behaves the same when nothing was previously cached: idempotent on
// initial-mount-style state, no spurious mutations.
{
  let data: string | null = null;
  let loading = true;
  let error: string | null = null;
  const generationRef = { current: 0 };
  const inFlightRef = { current: false };
  const hasDataRef = { current: false };

  resetAutoRefreshStateForDepsChange<string>(
    (v) => {
      data = v;
    },
    (v) => {
      loading = v;
    },
    (v) => {
      error = v;
    },
    generationRef,
    inFlightRef,
    hasDataRef,
  );

  assertEqual(data, null, "data stays null on initial-mount-style reset");
  assertEqual(loading, true, "loading stays true on initial-mount-style reset");
  assertEqual(error, null, "error stays null on initial-mount-style reset");
  assertEqual(
    generationRef.current,
    1,
    "generationRef still bumps on initial-mount-style reset",
  );
}

console.log("useAutoRefresh.test.ts ok");
