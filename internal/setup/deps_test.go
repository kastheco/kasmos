package setup

import "testing"

func TestCheckDependencies(t *testing.T) {
	t.Parallel()

	results := CheckDependencies()
	if len(results) != 2 {
		t.Fatalf("expected 2 dependency results, got %d", len(results))
	}

	seen := map[string]DepResult{}
	for _, r := range results {
		if r.Name == "" {
			t.Fatal("dependency name must not be empty")
		}
		if r.InstallHint == "" {
			t.Fatalf("install hint must not be empty for %q", r.Name)
		}
		seen[r.Name] = r
	}

	gitResult, ok := seen["git"]
	if !ok {
		t.Fatal("expected git dependency in results")
	}
	if !gitResult.Found {
		t.Skip("git not available in this test environment")
	}
}
