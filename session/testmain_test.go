package session

import (
	"os"
	"testing"

	"github.com/kastheco/kasmos/log"
)

func TestMain(m *testing.M) {
	log.Initialize(false)
	_ = os.Unsetenv("TMUX")
	_ = os.Unsetenv("TMUX_PANE")
	code := m.Run()
	log.Close()
	os.Exit(code)
}
