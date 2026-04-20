package overlay

import (
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
)

var readClipboardText = clipboard.ReadAll

func isPasteShortcut(msg tea.KeyPressMsg) bool {
	if !msg.Mod.Contains(tea.ModCtrl) || msg.Mod.Contains(tea.ModAlt) {
		return false
	}

	code := msg.Code
	if key := msg.Key(); key.BaseCode != 0 {
		code = key.BaseCode
	}

	return unicode.ToLower(code) == 'v'
}
