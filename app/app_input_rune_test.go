package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFirstRuneIsPrintable checks the rune-decoding helper used by the SDK
// focus-mode key handler.  The important edge cases are:
//   - empty string → false (nothing to decode)
//   - invalid UTF-8 → false (DecodeRuneInString returns RuneError with size 1)
//   - a legitimate U+FFFD replacement character → true (encoded as 3 bytes,
//     DecodeRuneInString returns RuneError with size 3 and unicode.IsPrint is
//     true for U+FFFD).  Misclassifying this as non-printable would swallow
//     keystrokes that legitimately produced the replacement character.
//   - multi-byte printable runes (é, 中, emoji) → true
//   - control characters (tab, carriage return) → false
func TestFirstRuneIsPrintable(t *testing.T) {
	cases := []struct {
		name string
		s    string
		want bool
	}{
		{"empty", "", false},
		{"ascii letter", "a", true},
		{"ascii space", " ", true},
		{"ascii control tab", "\t", false},
		{"ascii control CR", "\r", false},
		{"latin-1 e acute", "é", true},
		{"cjk", "中", true},
		{"emoji", "🙂", true},
		{"replacement character U+FFFD (valid encoding)", "\uFFFD", true},
		{"invalid utf8 bytes", "\xff\xfe", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, firstRuneIsPrintable(tc.s))
		})
	}
}
