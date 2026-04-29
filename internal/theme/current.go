package theme

import "sync"

var currentPalette = struct {
	sync.RWMutex
	palette Palette
	set     bool
}{}

// Current returns the active palette, or the built-in default before SetCurrent is called.
func Current() Palette {
	currentPalette.RLock()
	defer currentPalette.RUnlock()
	if !currentPalette.set {
		return DefaultPalette()
	}
	return currentPalette.palette
}

// SetCurrent updates the active palette for readers in UI and SDK rendering paths.
func SetCurrent(p Palette) {
	currentPalette.Lock()
	defer currentPalette.Unlock()
	currentPalette.palette = p
	currentPalette.set = true
}
