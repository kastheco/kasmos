package session

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeAppleScript(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no special chars",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "double quote",
			input: `say "hello"`,
			want:  `say \"hello\"`,
		},
		{
			name:  "backslash",
			input: `path\to\file`,
			want:  `path\\to\\file`,
		},
		{
			name:  "backslash before quote",
			input: `\"`,
			want:  `\\\"`,
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeAppleScript(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSendNotificationDisabled(t *testing.T) {
	// When notifications are disabled, SendNotification must be a no-op.
	// We verify by temporarily disabling and ensuring it doesn't panic or exec.
	orig := NotificationsEnabled
	NotificationsEnabled = false
	defer func() { NotificationsEnabled = orig }()

	// Should return without error and without launching any process.
	assert.NotPanics(t, func() {
		SendNotification("test title", "test body")
	})
}

func TestSendNotificationEnabled(t *testing.T) {
	// When enabled, SendNotification should not panic on any platform.
	// Stub the process launcher so the test exercises the enabled path without
	// talking to the real OS notification stack.
	orig := NotificationsEnabled
	origLookPath := notifyLookPath
	origStart := notifyStart
	NotificationsEnabled = true
	defer func() {
		NotificationsEnabled = orig
		notifyLookPath = origLookPath
		notifyStart = origStart
	}()

	started := false
	notifyLookPath = func(file string) (string, error) {
		if runtime.GOOS == "linux" {
			return "/usr/bin/notify-send", nil
		}
		return "", fmt.Errorf("lookup not used on %s", runtime.GOOS)
	}
	notifyStart = func(name string, args ...string) error {
		started = true
		return nil
	}

	assert.NotPanics(t, func() {
		SendNotification("kas", "agent finished")
	})
	assert.True(t, started)
}

func TestLinuxNotifyArgs_UsesKasAppName(t *testing.T) {
	assert.Equal(t, []string{"-a", "kas", "kas", "agent finished"}, linuxNotifyArgs("kas", "agent finished"))
}
