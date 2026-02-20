package worker

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

type mockTmuxCLI struct {
	SplitWindowFn   func(ctx context.Context, opts SplitOpts) (string, error)
	KillPaneFn      func(ctx context.Context, paneID string) error
	SelectPaneFn    func(ctx context.Context, paneID string) error
	JoinPaneFn      func(ctx context.Context, opts JoinOpts) error
	NewWindowFn     func(ctx context.Context, opts NewWindowOpts) (string, error)
	ListPanesFn     func(ctx context.Context, target string) ([]PaneInfo, error)
	CapturePaneFn   func(ctx context.Context, paneID string) (string, error)
	DisplayMsgFn    func(ctx context.Context, format string) (string, error)
	VersionFn       func(ctx context.Context) (string, error)
	SetEnvFn        func(ctx context.Context, key, value string) error
	ShowEnvFn       func(ctx context.Context) (map[string]string, error)
	UnsetEnvFn      func(ctx context.Context, key string) error
	ShowOptionFn    func(ctx context.Context, key string) (string, error)
	SetOptionFn     func(ctx context.Context, key, value string) error
	SetPaneTitleFn  func(ctx context.Context, paneID, title string) error
	SetPaneOptionFn func(ctx context.Context, paneID, option, value string) error
}

var _ TmuxCLI = (*mockTmuxCLI)(nil)

func (m *mockTmuxCLI) SplitWindow(ctx context.Context, opts SplitOpts) (string, error) {
	if m.SplitWindowFn != nil {
		return m.SplitWindowFn(ctx, opts)
	}
	return "", nil
}

func (m *mockTmuxCLI) KillPane(ctx context.Context, paneID string) error {
	if m.KillPaneFn != nil {
		return m.KillPaneFn(ctx, paneID)
	}
	return nil
}

func (m *mockTmuxCLI) SelectPane(ctx context.Context, paneID string) error {
	if m.SelectPaneFn != nil {
		return m.SelectPaneFn(ctx, paneID)
	}
	return nil
}

func (m *mockTmuxCLI) JoinPane(ctx context.Context, opts JoinOpts) error {
	if m.JoinPaneFn != nil {
		return m.JoinPaneFn(ctx, opts)
	}
	return nil
}

func (m *mockTmuxCLI) NewWindow(ctx context.Context, opts NewWindowOpts) (string, error) {
	if m.NewWindowFn != nil {
		return m.NewWindowFn(ctx, opts)
	}
	return "", nil
}

func (m *mockTmuxCLI) ListPanes(ctx context.Context, target string) ([]PaneInfo, error) {
	if m.ListPanesFn != nil {
		return m.ListPanesFn(ctx, target)
	}
	return nil, nil
}

func (m *mockTmuxCLI) CapturePane(ctx context.Context, paneID string) (string, error) {
	if m.CapturePaneFn != nil {
		return m.CapturePaneFn(ctx, paneID)
	}
	return "", nil
}

func (m *mockTmuxCLI) DisplayMessage(ctx context.Context, format string) (string, error) {
	if m.DisplayMsgFn != nil {
		return m.DisplayMsgFn(ctx, format)
	}
	return "", nil
}

func (m *mockTmuxCLI) Version(ctx context.Context) (string, error) {
	if m.VersionFn != nil {
		return m.VersionFn(ctx)
	}
	return "", nil
}

func (m *mockTmuxCLI) SetEnvironment(ctx context.Context, key, value string) error {
	if m.SetEnvFn != nil {
		return m.SetEnvFn(ctx, key, value)
	}
	return nil
}

func (m *mockTmuxCLI) ShowEnvironment(ctx context.Context) (map[string]string, error) {
	if m.ShowEnvFn != nil {
		return m.ShowEnvFn(ctx)
	}
	return map[string]string{}, nil
}

func (m *mockTmuxCLI) UnsetEnvironment(ctx context.Context, key string) error {
	if m.UnsetEnvFn != nil {
		return m.UnsetEnvFn(ctx, key)
	}
	return nil
}

func (m *mockTmuxCLI) SetOption(ctx context.Context, key, value string) error {
	if m.SetOptionFn != nil {
		return m.SetOptionFn(ctx, key, value)
	}
	return nil
}

func (m *mockTmuxCLI) ShowOption(ctx context.Context, key string) (string, error) {
	if m.ShowOptionFn != nil {
		return m.ShowOptionFn(ctx, key)
	}
	return "", nil
}

func (m *mockTmuxCLI) SetPaneTitle(ctx context.Context, paneID, title string) error {
	if m.SetPaneTitleFn != nil {
		return m.SetPaneTitleFn(ctx, paneID, title)
	}
	return nil
}

func (m *mockTmuxCLI) SetPaneOption(ctx context.Context, paneID, option, value string) error {
	if m.SetPaneOptionFn != nil {
		return m.SetPaneOptionFn(ctx, paneID, option, value)
	}
	return nil
}

func TestParsePaneList(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []PaneInfo
	}{
		{
			name:  "two panes one dead",
			input: "%0 12345 0 0\n%1 12346 1 137",
			want: []PaneInfo{
				{ID: "%0", PID: 12345, Dead: false, DeadStatus: 0},
				{ID: "%1", PID: 12346, Dead: true, DeadStatus: 137},
			},
		},
		{
			name:  "empty output",
			input: "",
			want:  nil,
		},
		{
			name:  "whitespace only",
			input: "  \n  \n",
			want:  nil,
		},
		{
			name:  "malformed line skipped",
			input: "%0 12345 0 0\nbadline\n%2 12347 0 0",
			want: []PaneInfo{
				{ID: "%0", PID: 12345, Dead: false, DeadStatus: 0},
				{ID: "%2", PID: 12347, Dead: false, DeadStatus: 0},
			},
		},
		{
			name:  "invalid numeric values fall back",
			input: "%3 nope 1 bad",
			want: []PaneInfo{
				{ID: "%3", PID: 0, Dead: true, DeadStatus: -1},
			},
		},
		{
			name:  "missing dead status",
			input: "%4 4545 0",
			want: []PaneInfo{
				{ID: "%4", PID: 4545, Dead: false, DeadStatus: -1},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parsePaneList(tc.input)
			if err != nil {
				t.Fatalf("parsePaneList returned error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parsePaneList mismatch: got=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestParseEnvironment(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{
			name:  "basic environment",
			input: "KASMOS_PANE_w-001=%42\nKASMOS_PANE_w-002=%43",
			want: map[string]string{
				"KASMOS_PANE_w-001": "%42",
				"KASMOS_PANE_w-002": "%43",
			},
		},
		{
			name:  "skips unset and malformed lines",
			input: "-REMOVED\nPATH=/usr/bin\nMALFORMED\nKEY=value=with_equals",
			want: map[string]string{
				"PATH": "/usr/bin",
				"KEY":  "value=with_equals",
			},
		},
		{
			name:  "empty input",
			input: "",
			want:  map[string]string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseEnvironment(tc.input)
			if err != nil {
				t.Fatalf("parseEnvironment returned error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseEnvironment mismatch: got=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestTmuxErrorFormat(t *testing.T) {
	tests := []struct {
		name string
		err  *TmuxError
		want string
	}{
		{
			name: "with stderr",
			err:  &TmuxError{Command: []string{"kill-pane", "-t", "%5"}, Stderr: "can't find pane: %5", Err: errors.New("exit status 1")},
			want: "tmux kill-pane -t %5: exit status 1 (stderr: can't find pane: %5)",
		},
		{
			name: "without stderr",
			err:  &TmuxError{Command: []string{"list-panes"}, Stderr: "", Err: errors.New("exit status 1")},
			want: "tmux list-panes: exit status 1",
		},
		{
			name: "empty command",
			err:  &TmuxError{Command: nil, Stderr: "", Err: errors.New("signal: killed")},
			want: "tmux: signal: killed",
		},
		{
			name: "nil error",
			err:  (*TmuxError)(nil),
			want: "tmux command failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.err.Error()
			if got != tc.want {
				t.Fatalf("TmuxError.Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTmuxErrorUnwrap(t *testing.T) {
	inner := errors.New("exit status 1")
	err := &TmuxError{Command: []string{"split-window"}, Err: inner}
	if !errors.Is(err, inner) {
		t.Fatal("TmuxError.Unwrap() did not expose inner error")
	}

	var nilErr *TmuxError
	if nilErr.Unwrap() != nil {
		t.Fatal("nil TmuxError.Unwrap() should return nil")
	}
}

func TestTmuxErrorHelpers(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		notFound    bool
		noSpace     bool
		sessionGone bool
	}{
		{
			name:        "nil error",
			err:         nil,
			notFound:    false,
			noSpace:     false,
			sessionGone: false,
		},
		{
			name:        "non tmux error",
			err:         errors.New("plain failure"),
			notFound:    false,
			noSpace:     false,
			sessionGone: false,
		},
		{
			name:        "not found detection",
			err:         &TmuxError{Stderr: "can't find pane: %99", Err: errors.New("exit status 1")},
			notFound:    true,
			noSpace:     false,
			sessionGone: false,
		},
		{
			name:        "wrapped not found detection",
			err:         fmt.Errorf("wrapped: %w", &TmuxError{Stderr: "window not found", Err: errors.New("exit status 1")}),
			notFound:    true,
			noSpace:     false,
			sessionGone: false,
		},
		{
			name:        "no space detection",
			err:         &TmuxError{Stderr: "No Space For New Pane", Err: errors.New("exit status 1")},
			notFound:    false,
			noSpace:     true,
			sessionGone: false,
		},
		{
			name:        "no server running detection",
			err:         &TmuxError{Stderr: "no server running on /tmp/tmux-1000/default", Err: errors.New("exit status 1")},
			notFound:    false,
			noSpace:     false,
			sessionGone: true,
		},
		{
			name:        "session not found detection",
			err:         &TmuxError{Stderr: "session not found", Err: errors.New("exit status 1")},
			notFound:    true,
			noSpace:     false,
			sessionGone: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsNotFound(tc.err); got != tc.notFound {
				t.Fatalf("IsNotFound() mismatch: got=%t want=%t", got, tc.notFound)
			}
			if got := IsNoSpace(tc.err); got != tc.noSpace {
				t.Fatalf("IsNoSpace() mismatch: got=%t want=%t", got, tc.noSpace)
			}
			if got := IsSessionGone(tc.err); got != tc.sessionGone {
				t.Fatalf("IsSessionGone() mismatch: got=%t want=%t", got, tc.sessionGone)
			}
		})
	}
}
