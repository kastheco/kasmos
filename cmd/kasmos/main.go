package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/user/kasmos/internal/setup"
	"github.com/user/kasmos/internal/task"
	"github.com/user/kasmos/internal/tui"
	"github.com/user/kasmos/internal/worker"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var showVersion bool

	cmd := &cobra.Command{
		Use:   "kasmos",
		Short: "Kasmos agent orchestrator",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				fmt.Fprintln(cmd.OutOrStdout(), "kasmos v0.1.0")
				return nil
			}

			var source task.Source = &task.AdHocSource{}
			if len(args) > 0 {
				detected, err := task.DetectSourceType(args[0])
				if err != nil {
					return err
				}
				source = detected
			}

			if _, err := source.Load(); err != nil {
				log.Printf("warning: failed to load task source %q (%s): %v", source.Path(), source.Type(), err)
			}

			backend, err := worker.NewSubprocessBackend()
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			model := tui.NewModel(backend, source)
			program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(ctx))
			model.SetProgram(program)

			_, err = program.Run()
			return err
		},
	}

	cmd.Flags().BoolVar(&showVersion, "version", false, "print version and exit")

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
