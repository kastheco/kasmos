package fstools

import (
	"os"
	"path/filepath"
)

// EnrichMatches returns a copy of matches annotated with symbol metadata when
// the symbol store can resolve a symbol at the match line.
func EnrichMatches(matches []GrepMatch, store SymbolLookup) []GrepMatch {
	return enrichMatchesWithRoot(matches, store, "")
}

func enrichMatchesWithRoot(matches []GrepMatch, store SymbolLookup, searchRoot string) []GrepMatch {
	if len(matches) == 0 {
		return []GrepMatch{}
	}

	enriched := append([]GrepMatch(nil), matches...)
	if store == nil {
		return enriched
	}

	// Compute the base directory once for all matches rather than calling
	// os.Stat per match (up to MaxGrepMatches=200 syscalls per call otherwise).
	base := resolveSearchBase(searchRoot)

	for i := range enriched {
		lookupPath := resolveMatchLookupPath(enriched[i].File, base)
		symbol := store.LookupAt(lookupPath, enriched[i].Line)
		if symbol == nil {
			continue
		}
		enriched[i].SymbolKind = symbol.Kind
		enriched[i].SymbolName = symbol.Name
		enriched[i].SymbolParent = symbol.Parent
	}

	return enriched
}

// resolveSearchBase returns the directory component of searchRoot, using a
// single os.Stat call to determine whether the root is itself a file. The
// result is reused across all matches in a single enrichment pass.
func resolveSearchBase(searchRoot string) string {
	if searchRoot == "" {
		return ""
	}
	base := filepath.Clean(searchRoot)
	if info, err := os.Stat(base); err == nil && !info.IsDir() {
		return filepath.Dir(base)
	}
	return base
}

func resolveMatchLookupPath(matchFile, base string) string {
	cleanFile := filepath.Clean(matchFile)
	if filepath.IsAbs(cleanFile) || base == "" {
		return cleanFile
	}
	return filepath.Clean(filepath.Join(base, cleanFile))
}
