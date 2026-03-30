package symbols

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreLookupReturnsCopy(t *testing.T) {
	store := NewStore()
	path := filepath.Join(t.TempDir(), "sample.go")
	store.Update(path, []Symbol{{Name: "Outer", Kind: "type", Line: 10, End: 40}})

	lookup := store.Lookup(path)
	require.Len(t, lookup, 1)
	lookup[0].Name = "Changed"

	again := store.Lookup(path)
	require.Len(t, again, 1)
	assert.Equal(t, "Outer", again[0].Name)
}

func TestStoreLookupAtFindsSmallestEnclosingSymbol(t *testing.T) {
	store := NewStore()
	path := filepath.Join(t.TempDir(), "sample.go")
	store.Update(path, []Symbol{
		{Name: "Outer", Kind: "type", Line: 10, End: 40},
		{Name: "Inner", Kind: "method", Line: 20, End: 24, Parent: "Outer"},
		{Name: "Exact", Kind: "function", Line: 30},
	})

	inner := store.LookupAt(path, 21)
	require.NotNil(t, inner)
	assert.Equal(t, &Symbol{Name: "Inner", Kind: "method", Line: 20, End: 24, Parent: "Outer"}, inner)

	exact := store.LookupAt(path, 30)
	require.NotNil(t, exact)
	assert.Equal(t, &Symbol{Name: "Exact", Kind: "function", Line: 30}, exact)

	assert.Nil(t, store.LookupAt(path, 50))
	assert.Nil(t, store.LookupAt(path, 0))
}

func TestStoreRemoveClearsSymbols(t *testing.T) {
	store := NewStore()
	path := filepath.Join(t.TempDir(), "sample.go")
	store.Update(path, []Symbol{{Name: "Outer", Kind: "type", Line: 10, End: 40}})

	store.Remove(path)

	assert.Nil(t, store.Lookup(path))
	assert.Nil(t, store.LookupAt(path, 10))
}
