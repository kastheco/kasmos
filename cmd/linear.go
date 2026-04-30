package cmd

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/config/linearlink"
	"github.com/kastheco/kasmos/config/lineartrigger"
	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
	"github.com/spf13/cobra"
)

var newLinearClient = func(cfg linear.Config) linearDiscoveryClient {
	return linear.NewClientFromConfig(cfg)
}

type linearDiscoveryClient interface {
	Viewer(ctx context.Context) (*linear.User, error)
	Labels(ctx context.Context, p linear.PageOptions) ([]linear.Label, linear.PageInfo, error)
	Teams(ctx context.Context, p linear.PageOptions) ([]linear.Team, linear.PageInfo, error)
	WorkflowStates(ctx context.Context, p linear.PageOptions) ([]linear.WorkflowState, linear.PageInfo, error)
	Projects(ctx context.Context, p linear.PageOptions) ([]linear.Project, linear.PageInfo, error)
}

type linearPollClient interface {
	lineartrigger.LinearClient
	linearlink.IssueFetcher
}

// NewLinearCmd builds the top-level `kas linear` command group.
func NewLinearCmd() *cobra.Command {
	linearCmd := &cobra.Command{Use: "linear", Short: "linear integration helpers"}

	linearPollOnceCmd := &cobra.Command{
		Use:   "poll-once",
		Short: "run a single linear trigger poll cycle for the current repo",
		Args:  cobra.NoArgs,
		RunE:  runLinearPollOnce,
	}

	linearDiscoverCmd := &cobra.Command{
		Use:       "discover [labels|users|workflow-states|teams|projects]",
		Short:     "list linear identifiers needed for [linear.triggers] config",
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"labels", "users", "workflow-states", "teams", "projects"},
		RunE:      runLinearDiscover,
	}

	linearCmd.AddCommand(linearPollOnceCmd)
	linearCmd.AddCommand(linearDiscoverCmd)
	return linearCmd
}

func runLinearPollOnce(cmd *cobra.Command, _ []string) error {
	_, project, err := resolveRepoInfo()
	if err != nil {
		return err
	}
	tomlCfg, err := config.LoadTOMLConfig()
	if err != nil {
		return err
	}
	if tomlCfg == nil || !tomlCfg.LinearTriggers.Enabled {
		fmt.Fprintln(cmd.OutOrStdout(), "linear triggers: disabled")
		return nil
	}
	linearCfg, err := linear.ConfigFromEnv()
	if err != nil {
		return err
	}
	client, ok := newLinearClient(linearCfg).(linearPollClient)
	if !ok {
		return fmt.Errorf("linear client does not support trigger polling")
	}

	store, err := taskstore.OpenAuthoritativeStore(project)
	if err != nil {
		return err
	}
	defer store.Close()
	gateway, err := taskstore.OpenAuthoritativeSignalGateway(project)
	if err != nil {
		return err
	}
	defer gateway.Close()
	logger, closeLogger := openTaskAuditLogger()
	defer closeLogger()

	poller := lineartrigger.NewPoller(lineartrigger.PollerDeps{
		Project: project,
		Config:  tomlCfg.LinearTriggers,
		Store:   store,
		Linker:  linearlink.New(store, client, logger, project),
		Linear:  client,
		Gateway: gateway,
		Audit:   logger,
	})
	stats := poller.PollOnce(cmd.Context())
	if stats.Err != nil {
		var rateLimit *linear.RateLimitError
		if errors.As(stats.Err, &rateLimit) {
			fmt.Fprintf(cmd.OutOrStdout(), "linear triggers: rate limited (%s)\n", stats.Err)
			return nil
		}
		return stats.Err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "linear triggers: %d received, %d dispatched, %d rejected, %d ignored, %d errors\n",
		stats.Received, stats.Dispatched, stats.Rejected, stats.Ignored, stats.Failed)
	return nil
}

func runLinearDiscover(cmd *cobra.Command, args []string) error {
	cfg, err := linear.ConfigFromEnv()
	if err != nil {
		return err
	}
	client := newLinearClient(cfg)
	rows, err := discoverLinearRows(cmd.Context(), client, args[0])
	if err != nil {
		return err
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Name == rows[j].Name {
			return rows[i].ID < rows[j].ID
		}
		return rows[i].Name < rows[j].Name
	})
	for _, row := range rows {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", row.ID, row.Name)
	}
	return nil
}

type linearDiscoverRow struct {
	ID   string
	Name string
}

func discoverLinearRows(ctx context.Context, client linearDiscoveryClient, kind string) ([]linearDiscoverRow, error) {
	switch kind {
	case "labels":
		return collectLinearRows(ctx, func(p linear.PageOptions) ([]linearDiscoverRow, linear.PageInfo, error) {
			labels, page, err := client.Labels(ctx, p)
			rows := make([]linearDiscoverRow, 0, len(labels))
			for _, label := range labels {
				rows = append(rows, linearDiscoverRow{ID: label.ID, Name: label.Name})
			}
			return rows, page, err
		})
	case "teams":
		return collectLinearRows(ctx, func(p linear.PageOptions) ([]linearDiscoverRow, linear.PageInfo, error) {
			teams, page, err := client.Teams(ctx, p)
			rows := make([]linearDiscoverRow, 0, len(teams))
			for _, team := range teams {
				name := team.Name
				if strings.TrimSpace(name) == "" {
					name = team.Key
				}
				rows = append(rows, linearDiscoverRow{ID: team.ID, Name: name})
			}
			return rows, page, err
		})
	case "workflow-states":
		return collectLinearRows(ctx, func(p linear.PageOptions) ([]linearDiscoverRow, linear.PageInfo, error) {
			states, page, err := client.WorkflowStates(ctx, p)
			rows := make([]linearDiscoverRow, 0, len(states))
			for _, state := range states {
				rows = append(rows, linearDiscoverRow{ID: state.ID, Name: state.Name})
			}
			return rows, page, err
		})
	case "projects":
		return collectLinearRows(ctx, func(p linear.PageOptions) ([]linearDiscoverRow, linear.PageInfo, error) {
			projects, page, err := client.Projects(ctx, p)
			rows := make([]linearDiscoverRow, 0, len(projects))
			for _, project := range projects {
				rows = append(rows, linearDiscoverRow{ID: project.ID, Name: project.Name})
			}
			return rows, page, err
		})
	case "users":
		viewer, err := client.Viewer(ctx)
		if err != nil {
			return nil, err
		}
		name := viewer.Name
		if strings.TrimSpace(name) == "" {
			name = viewer.Email
		}
		return []linearDiscoverRow{{ID: viewer.ID, Name: name}}, nil
	default:
		return nil, fmt.Errorf("unsupported linear discovery kind %q", kind)
	}
}

func collectLinearRows(ctx context.Context, next func(linear.PageOptions) ([]linearDiscoverRow, linear.PageInfo, error)) ([]linearDiscoverRow, error) {
	var rows []linearDiscoverRow
	page := linear.PageOptions{First: 50}
	for {
		batch, info, err := next(page)
		if err != nil {
			return nil, err
		}
		rows = append(rows, batch...)
		if !info.HasNextPage || info.EndCursor == "" {
			return rows, nil
		}
		page.After = info.EndCursor
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
}
