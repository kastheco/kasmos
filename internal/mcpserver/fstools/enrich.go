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

	for i := range enriched {
		lookupPath := resolveMatchLookupPath(enriched[i].File, searchRoot)
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

func resolveMatchLookupPath(matchFile, searchRoot string) string {
	cleanFile := filepath.Clean(matchFile)
	if filepath.IsAbs(cleanFile) || searchRoot == "" {
		return cleanFile
	}

	base := filepath.Clean(searchRoot)
	if info, err := os.Stat(base); err == nil && !info.IsDir() {
		base = filepath.Dir(base)
	}

	return filepath.Clean(filepath.Join(base, cleanFile))
}
