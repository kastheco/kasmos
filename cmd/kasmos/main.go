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

	"github.com/user/kasmos/internal/config"
	"github.com/user/kasmos/internal/persist"
	"github.com/user/kasmos/internal/setup"
	"github.com/user/kasmos/internal/task"
	"github.com/user/kasmos/internal/tui"
	"github.com/user/kasmos/internal/worker"
)

// Set at build time: -ldflags "-X main.version=2.0.0"
var version = "2.0.7"

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
	var tmuxMode bool

	cmd := &cobra.Command{
		Use:   "kasmos",
		Short: "Kasmos agent orchestrator",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				fmt.Fprintf(cmd.OutOrStdout(), "kasmos v%s\n", version)
				return nil
			}

			if !daemon {
				if info, err := os.Stdout.Stat(); err == nil {
					if (info.Mode() & os.ModeCharDevice) == 0 {
						daemon = true
					}
				}
			}

			if tmuxMode && daemon {
				return fmt.Errorf("--tmux and -d (daemon mode) are mutually exclusive.\n" +
					"Tmux mode requires the interactive dashboard and cannot run headless.\n" +
					"Use --tmux for interactive agent sessions, or -d for headless batch processing.")
			}

			cfg, err := config.Load(".")
			if err != nil {
				log.Printf("warning: failed to load config: %v", err)
				cfg = config.DefaultConfig()
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

			persister := persist.NewSessionPersister(".")
			sessionID := persist.NewSessionID()
			var attachState *persist.SessionState

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

				attachState = state
				sessionID = state.SessionID

				if state.BackendMode == "tmux" && !tmuxMode {
					if os.Getenv("TMUX") != "" {
						tmuxMode = true
					} else {
						log.Printf("notice: session used tmux mode but not in tmux session, restoring as subprocess mode")
					}
				}
			}

			// Config-based tmux activation (FR-002, FR-004)
			// Priority: --tmux flag > attach inference > cfg.TmuxMode > default (subprocess)
			if !tmuxMode && cfg.TmuxMode {
				if os.Getenv("TMUX") != "" {
					tmuxMode = true
					log.Printf("info: tmux mode activated from config")
				} else {
					log.Printf("notice: tmux_mode configured but not in tmux session, using subprocess mode")
				}
			}

			var backend worker.WorkerBackend
			var tmuxBackend *worker.TmuxBackend
			if tmuxMode {
				if os.Getenv("TMUX") == "" {
					return fmt.Errorf("--tmux requires running inside a tmux session.\n" +
						"Start one with: tmux new-session -s kasmos\n" +
						"Then run: kasmos --tmux")
				}

				cli, err := worker.NewTmuxExec()
				if err != nil {
					return fmt.Errorf("tmux mode: %w", err)
				}

				tmuxBackend, err = worker.NewTmuxBackend(cli)
				if err != nil {
					return err
				}

				if err := tmuxBackend.Init(sessionID); err != nil {
					return fmt.Errorf("tmux init: %w", err)
				}

				backend = tmuxBackend
			} else {
				backend, err = worker.NewSubprocessBackend()
				if err != nil {
					return err
				}
			}

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			showLauncher := len(args) == 0 && !attach && !daemon

			model := tui.NewModel(backend, source, version, cfg, showLauncher)
			if tmuxMode && tmuxBackend != nil {
				model.SetTmuxMode(tmuxBackend)
			}
			if daemon {
				model.SetDaemonMode(true, format, spawnAll)
			}

			if attachState != nil {
				survivingWorkerIDs := make(map[string]bool)
				if tmuxMode && tmuxBackend != nil {
					reconnected, err := tmuxBackend.Reconnect(sessionID)
					if err != nil {
						log.Printf("warning: tmux reconnect failed: %v", err)
					} else {
						for _, rw := range reconnected {
							if !rw.Dead {
								survivingWorkerIDs[rw.WorkerID] = true
							}
						}
					}
				}

				for _, snap := range attachState.Workers {
					w := persist.SnapshotToWorker(snap)
					if (w.State == worker.StateRunning || w.State == worker.StateSpawning) && !survivingWorkerIDs[w.ID] {
						w.State = worker.StateKilled
						w.ExitedAt = time.Now()
					}
					model.RestoreWorker(w)
				}

				if tmuxMode && tmuxBackend != nil {
					for workerID := range survivingWorkerIDs {
						w := model.FindWorker(workerID)
						if w == nil {
							continue
						}

						handle := tmuxBackend.Handle(workerID, w.SpawnedAt)
						if handle != nil {
							w.Handle = handle
							w.State = worker.StateRunning
						}
					}
				}

				model.ResetWorkerCounter(attachState.NextWorkerNum)
				model.SetSessionStartedAt(attachState.StartedAt)
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
	cmd.Flags().BoolVar(&tmuxMode, "tmux", false, "run workers as interactive tmux panes")

	var forceSetup bool
	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Validate dependencies and scaffold agent configurations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return setup.Run(forceSetup)
		},
	}
	setupCmd.Flags().BoolVar(&forceSetup, "force", false, "overwrite existing agent definitions and configurations")
	cmd.AddCommand(setupCmd)

	return cmd
}
