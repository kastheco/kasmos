package symbols

import (
	"path/filepath"
	"sort"
	"sync"
)

// Store keeps an in-memory symbol index keyed by absolute file path.
type Store struct {
	mu    sync.RWMutex
	files map[string][]Symbol
}

// NewStore creates an empty symbol store.
func NewStore() *Store {
	return &Store{files: make(map[string][]Symbol)}
}

// Lookup returns a copy of the symbols currently indexed for path.
func (s *Store) Lookup(path string) []Symbol {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	symbols := s.files[filepath.Clean(path)]
	if len(symbols) == 0 {
		return nil
	}

	copyOfSymbols := make([]Symbol, len(symbols))
	copy(copyOfSymbols, symbols)
	return copyOfSymbols
}

// LookupPresent reports whether path is present in the store. It returns a
// copy of the stored slice (possibly empty) and true when the key exists, or
// nil and false when the store has never indexed this path. Callers use this
// to distinguish a cached-empty entry (file legitimately has no symbols) from
// a true cache miss.
func (s *Store) LookupPresent(path string) ([]Symbol, bool) {
	if s == nil {
		return nil, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	symbols, ok := s.files[filepath.Clean(path)]
	if !ok {
		return nil, false
	}
	if len(symbols) == 0 {
		return nil, true
	}
	copyOfSymbols := make([]Symbol, len(symbols))
	copy(copyOfSymbols, symbols)
	return copyOfSymbols, true
}

// LookupAt returns the smallest symbol enclosing line within path.
func (s *Store) LookupAt(path string, line int) *Symbol {
	if s == nil || line < 1 {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	symbols := s.files[filepath.Clean(path)]
	var best *Symbol
	for i := range symbols {
		symbol := symbols[i]
		if !symbolContainsLine(symbol, line) {
			continue
		}
		if best == nil || symbolSpan(symbol) < symbolSpan(*best) {
			candidate := symbol
			best = &candidate
		}
	}
	return best
}

// Update replaces the symbol slice for path.
func (s *Store) Update(path string, symbols []Symbol) {
	if s == nil {
		return
	}

	stored := append([]Symbol(nil), symbols...)
	sort.Slice(stored, func(i, j int) bool {
		if stored[i].Line == stored[j].Line {
			return symbolSpan(stored[i]) < symbolSpan(stored[j])
		}
		return stored[i].Line < stored[j].Line
	})

	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[filepath.Clean(path)] = stored
}

// Remove drops all indexed symbols for path.
func (s *Store) Remove(path string) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.files, filepath.Clean(path))
}

func symbolContainsLine(symbol Symbol, line int) bool {
	if symbol.End == 0 {
		return symbol.Line == line
	}
	return symbol.Line <= line && line <= symbol.End
}

func symbolSpan(symbol Symbol) int {
	if symbol.End == 0 || symbol.End < symbol.Line {
		return 0
	}
	return symbol.End - symbol.Line
}
