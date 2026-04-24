package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

// NewMonitorCmd returns the `kas monitor` cobra command with subcommands.
// The default behaviour (no subcommand) is a live tail of the daemon event
// stream via SSE from the control socket.
func NewMonitorCmd() *cobra.Command {
	var (
		socketPath  string
		repoFilter  string
		planFilter  string
		kindFilter  []string
		sinceFilter string
		jsonOutput  bool
	)

	cmd := &cobra.Command{
		Use:     "monitor",
		Aliases: []string{"mon"},
		Short:   "monitor the kasmos daemon event stream",
		Long: `monitor connects to the running kasmos daemon and streams real-time
orchestration events. by default it outputs colored ANSI text; use --json for
raw JSON suitable for piping to jq.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMonitorTail(cmd, socketPath, repoFilter, planFilter, kindFilter, sinceFilter, jsonOutput)
		},
	}

	cmd.PersistentFlags().StringVar(&socketPath, "socket", daemonSocketPath(), "path to the daemon unix domain socket")
	cmd.Flags().StringVar(&repoFilter, "repo", "", "filter events to a specific repo path")
	cmd.Flags().StringVar(&planFilter, "plan", "", "filter events to a specific plan slug")
	cmd.Flags().StringArrayVar(&kindFilter, "kind", nil, "filter events to one or more event kinds")
	cmd.Flags().StringVar(&sinceFilter, "since", "", "filter events after an RFC3339 timestamp")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output raw JSON event stream (for piping to jq)")

	cmd.AddCommand(newMonitorStatusCmd(&socketPath))

	return cmd
}

// runMonitorTail opens the daemon SSE event stream and writes events to the
// command output until the stream is closed or the user interrupts.
func runMonitorTail(cmd *cobra.Command, socketPath, repoFilter, planFilter string, kindFilter []string, sinceFilter string, jsonOutput bool) error {
	client := daemonHTTPClient(socketPath)
	var since time.Time
	if sinceFilter != "" {
		t, err := time.Parse(time.RFC3339, sinceFilter)
		if err != nil {
			return fmt.Errorf("invalid --since timestamp (RFC3339 required): %w", err)
		}
		since = t
	}

	resp, err := client.Get("http://kas/v1/events")
	if err != nil {
		return fmt.Errorf("daemon not running: %w", err)
	}
	defer resp.Body.Close()

	out := cmd.OutOrStdout()
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}

		// SSE lines start with "data: ".
		const prefix = "data: "
		if len(line) > len(prefix) && line[:len(prefix)] == prefix {
			payload := line[len(prefix):]

			if jsonOutput {
				if ok, err := monitorEventMatches(payload, repoFilter, planFilter, kindFilter, since); err != nil {
					fmt.Fprintln(out, payload)
				} else if ok {
					fmt.Fprintln(out, payload)
				}
				continue
			}

			// Pretty-print for human consumption.
			if err := printMonitorEvent(out, payload, repoFilter, planFilter, kindFilter, since); err != nil {
				// Non-fatal: keep reading even if one event is malformed.
				fmt.Fprintf(out, "event: %s\n", payload)
			}
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("read event stream: %w", err)
	}
	return nil
}

// printMonitorEvent pretty-prints a single SSE JSON payload.
func printMonitorEvent(out io.Writer, payload, repoFilter, planFilter string, kindFilter []string, since time.Time) error {
	var event map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return err
	}

	eventType, _ := event["kind"].(string)
	if !monitorEventMapMatches(event, repoFilter, planFilter, kindFilter, since) {
		return nil
	}

	switch eventType {
	case "heartbeat":
		repos, _ := event["repos"].([]interface{})
		fmt.Fprintf(out, "\033[2m[heartbeat] %d repo(s) active\033[0m\n", len(repos))
	case "connected":
		fmt.Fprintf(out, "\033[32m[connected] monitoring daemon events\033[0m\n")
	default:
		if detail, ok := event["detail"].(string); ok && detail != "" {
			fmt.Fprintf(out, "[%s] %s  detail=%s\n", eventType, humanMessage(event), detail)
		} else {
			fmt.Fprintf(out, "[%s] %s\n", eventType, payload)
		}
	}
	return nil
}

func monitorEventMatches(payload, repoFilter, planFilter string, kindFilter []string, since time.Time) (bool, error) {
	var event map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return false, err
	}
	return monitorEventMapMatches(event, repoFilter, planFilter, kindFilter, since), nil
}

func monitorEventMapMatches(event map[string]interface{}, repoFilter, planFilter string, kindFilter []string, since time.Time) bool {
	eventType, _ := event["kind"].(string)
	repo, _ := event["repo"].(string)
	plan, _ := event["plan_file"].(string)
	if repoFilter != "" && repo != repoFilter {
		return false
	}
	if planFilter != "" && plan != planFilter {
		return false
	}
	if len(kindFilter) > 0 {
		matched := false
		for _, kind := range kindFilter {
			if eventType == kind {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if !since.IsZero() {
		raw, _ := event["timestamp"].(string)
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil || t.Before(since) {
			return false
		}
	}
	return true
}

func humanMessage(event map[string]interface{}) string {
	if msg, ok := event["message"].(string); ok && msg != "" {
		return msg
	}
	if plan, ok := event["plan_file"].(string); ok && plan != "" {
		return plan
	}
	return "event"
}

// newMonitorStatusCmd returns the `kas monitor status` subcommand — a one-shot
// snapshot of daemon state: registered repos, active plans, running agents.
func newMonitorStatusCmd(socketPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "show a snapshot of daemon state (repos, plans, agents)",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := daemonHTTPClient(*socketPath)

			resp, err := client.Get("http://kas/v1/status")
			if err != nil {
				return fmt.Errorf("daemon not running: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("unexpected status %d from daemon", resp.StatusCode)
			}

			var status map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
				return fmt.Errorf("decode status: %w", err)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "daemon status snapshot:")

			repos, _ := status["repos"].([]interface{})
			if len(repos) == 0 {
				fmt.Fprintln(out, "  repos: none")
			} else {
				fmt.Fprintln(out, "  repos:")
				for _, r := range repos {
					fmt.Fprintf(out, "    - %s\n", r)
				}
			}

			if agents, ok := status["agents"].([]interface{}); ok {
				fmt.Fprintln(out, "  agents:")
				for _, a := range agents {
					fmt.Fprintf(out, "    - %v\n", a)
				}
			}

			return nil
		},
	}
}
