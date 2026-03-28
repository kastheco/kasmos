package session

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var notifyLookPath = exec.LookPath
var notifyStart = func(name string, args ...string) error {
	return exec.Command(name, args...).Start()
}

// NotificationsEnabled controls whether desktop notifications are sent.
// Set from config at startup.
var NotificationsEnabled = defaultNotificationsEnabled()

func defaultNotificationsEnabled() bool {
	return !strings.HasSuffix(os.Args[0], ".test")
}

// SendNotification fires a desktop notification. The underlying command is
// started but not awaited — callers do not block on OS notification delivery.
func SendNotification(title, body string) {
	if !NotificationsEnabled {
		return
	}
	switch runtime.GOOS {
	case "darwin":
		sendDarwin(title, body)
	case "linux":
		sendLinux(title, body)
	}
}

// sendDarwin delivers a notification via osascript on macOS.
func sendDarwin(title, body string) {
	script := `display notification "` + escapeAppleScript(body) +
		`" with title "` + escapeAppleScript(title) + `"`
	_ = notifyStart("osascript", "-e", script)
}

// sendLinux delivers a notification via notify-send on Linux.
// The call is a no-op when notify-send is not installed.
func sendLinux(title, body string) {
	path, err := notifyLookPath("notify-send")
	if err != nil {
		return
	}
	_ = notifyStart(path, linuxNotifyArgs(title, body)...)
}

func linuxNotifyArgs(title, body string) []string {
	return []string{"-a", "kas", title, body}
}

// escapeAppleScript escapes backslashes and double-quotes for use inside
// an AppleScript string literal.
func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
