package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/user/kasmos/internal/persist"
	"github.com/user/kasmos/internal/setup"
	"github.com/user/kasmos/internal/task"
	"github.com/user/kasmos/internal/tui"
	"github.com/user/kasmos/internal/worker"
)

// Set at build time: -ldflags "-X main.version=2.0.0"
var version = "2.0.0"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var showVersion bool
	var daemon bool
	var format string
	var spawnAll bool
	var attach bool

	cmd := &cobra.Command{
		Use:   "kasmos",
		Short: "Kasmos agent orchestrator",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				fmt.Fprintf(cmd.OutOrStdout(), "kasmos v%s\n", version)
				return nil
			}

			var source task.Source
			if len(args) > 0 {
				detected, err := task.DetectSourceType(args[0])
				if err != nil {
					return err
				}
				source = detected
			} else {
				source = task.AutoDetect()
			}

			if _, err := source.Load(); err != nil {
				log.Printf("warning: failed to load task source %q (%s): %v", source.Path(), source.Type(), err)
			}

			if !daemon {
				if info, err := os.Stdout.Stat(); err == nil {
					if (info.Mode() & os.ModeCharDevice) == 0 {
						daemon = true
					}
				}
			}

			format = strings.TrimSpace(strings.ToLower(format))
			if format == "" {
				format = "default"
			}
			if format != "default" && format != "json" {
				return fmt.Errorf("invalid --format %q: expected default or json", format)
			}
			if daemon {
				signal.Ignore(syscall.SIGPIPE)
			}

			backend, err := worker.NewSubprocessBackend()
			if err != nil {
				return err
			}

			persister := persist.NewSessionPersister(".")
			sessionID := persist.NewSessionID()

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			model := tui.NewModel(backend, source, version)
			if daemon {
				model.SetDaemonMode(true, format, spawnAll)
			}
			if attach {
				state, err := persister.Load()
				if err != nil {
					if os.IsNotExist(err) {
						return fmt.Errorf("no session found. Start a new session with: kasmos")
					}
					return fmt.Errorf("load session: %w", err)
				}
				if persist.IsPIDAlive(state.PID) {
					return fmt.Errorf("session already active (PID %d)", state.PID)
				}

				for _, snap := range state.Workers {
					w := persist.SnapshotToWorker(snap)
					if w.State == worker.StateRunning || w.State == worker.StateSpawning {
						w.State = worker.StateKilled
						w.ExitedAt = time.Now()
					}
					model.RestoreWorker(w)
				}
				model.ResetWorkerCounter(state.NextWorkerNum)
				sessionID = state.SessionID
				model.SetSessionStartedAt(state.StartedAt)
			}
			model.SetPersister(persister, sessionID)
			opts := []tea.ProgramOption{tea.WithContext(ctx)}
			if daemon {
				opts = append(opts, tea.WithInput(nil))
			} else {
				opts = append(opts, tea.WithAltScreen())
			}
			program := tea.NewProgram(model, opts...)
			model.SetProgram(program)

			finalModel, err := program.Run()
			if err != nil {
				return err
			}
			if final, ok := finalModel.(*tui.Model); ok {
				if err := final.FinalizeSession(); err != nil {
					log.Printf("warning: failed to archive session: %v", err)
				}
				if daemon && final.DaemonExitCode() != 0 {
					return fmt.Errorf("daemon finished with failures")
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&showVersion, "version", false, "print version and exit")
	cmd.Flags().BoolVarP(&daemon, "daemon", "d", false, "run in headless daemon mode")
	cmd.Flags().StringVar(&format, "format", "default", "daemon output format: default or json")
	cmd.Flags().BoolVar(&spawnAll, "spawn-all", false, "spawn workers for all unblocked tasks immediately")
	cmd.Flags().BoolVar(&attach, "attach", false, "restore session from .kasmos/session.json")

	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Validate dependencies and scaffold agent configurations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return setup.Run()
		},
	}
	cmd.AddCommand(setupCmd)

	return cmd
}
