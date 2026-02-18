package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/user/kasmos/internal/tui"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				fmt.Fprintln(cmd.OutOrStdout(), "kasmos v0.1.0")
				return nil
			}

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			program := tea.NewProgram(tui.NewModel(), tea.WithAltScreen(), tea.WithContext(ctx))
			_, err := program.Run()
			return err
		},
	}

	cmd.Flags().BoolVar(&showVersion, "version", false, "print version and exit")

	return cmd
}
