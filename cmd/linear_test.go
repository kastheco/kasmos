package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/kastheco/kasmos/internal/linear"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinearDiscoverLabelsSortedByName(t *testing.T) {
	client := &linearDiscoverFake{
		labels: []linear.Label{
			{ID: "label-z", Name: "zeta"},
			{ID: "label-a", Name: "alpha"},
			{ID: "label-b", Name: "alpha"},
		},
	}
	rows, err := discoverLinearRows(context.Background(), client, "labels")
	require.NoError(t, err)

	cmd := NewLinearCmd()
	discover, _, err := cmd.Find([]string{"discover"})
	require.NoError(t, err)
	var out bytes.Buffer
	discover.SetOut(&out)

	original := newLinearClient
	newLinearClient = func(linear.Config) linearDiscoveryClient {
		return client
	}
	t.Cleanup(func() { newLinearClient = original })
	t.Setenv("KASMOS_LINEAR_API_KEY", "test-key")

	err = runLinearDiscover(discover, []string{"labels"})
	require.NoError(t, err)
	assert.Equal(t, []linearDiscoverRow{
		{ID: "label-a", Name: "alpha"},
		{ID: "label-b", Name: "alpha"},
		{ID: "label-z", Name: "zeta"},
	}, sortedLinearRows(rows))
	assert.Equal(t, "label-a\talpha\nlabel-b\talpha\nlabel-z\tzeta\n", out.String())
}

func TestLinearDiscoverUsersListsWorkspaceUsers(t *testing.T) {
	client := &linearDiscoverFake{
		users: []linear.User{
			{ID: "user-z", Name: "zara", Email: "zara@example.com"},
			{ID: "user-a", Email: "ann@example.com"},
		},
	}
	rows, err := discoverLinearRows(context.Background(), client, "users")
	require.NoError(t, err)

	assert.Equal(t, []linearDiscoverRow{
		{ID: "user-a", Name: "ann@example.com"},
		{ID: "user-z", Name: "zara"},
	}, sortedLinearRows(rows))
}

func TestRootCommandRegistersLinearGroup(t *testing.T) {
	cmd, _, err := NewRootCmd().Find([]string{"linear", "discover"})
	require.NoError(t, err)
	assert.Equal(t, "discover [labels|users|workflow-states|teams|projects]", cmd.Use)
}

func TestLinearWebhookStatusTextAndJSON(t *testing.T) {
	summary := statusLinearWebhooks{
		Enabled:      true,
		LastDelivery: "5m",
		Accepted:     3,
		Duplicate:    1,
		Rejected:     1,
	}

	cmd := NewLinearCmd()
	webhookStatus, _, err := cmd.Find([]string{"webhook", "status"})
	require.NoError(t, err)
	var out bytes.Buffer
	webhookStatus.SetOut(&out)

	assert.Equal(t, "linear webhooks: enabled (last delivery 5m ago, 3 accepted, 1 duplicate, 0 ignored, 1 rejected, 0 errors)\n", renderStatusLinearWebhooks(summary))
	require.NoError(t, writeLinearWebhookStatusJSON(webhookStatus, summary))

	var parsed statusLinearWebhooks
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(out.Bytes()), &parsed))
	assert.True(t, parsed.Enabled)
	assert.Equal(t, 3, parsed.Accepted)
	assert.Equal(t, 1, parsed.Duplicate)
}

func TestLinearWebhookStatusDisabledLine(t *testing.T) {
	assert.Equal(t, "linear webhooks: disabled\n", renderStatusLinearWebhooks(statusLinearWebhooks{}))
}

func TestLinearWebhookDeliveriesFormattingAndLimit(t *testing.T) {
	store := taskstore.NewTestSQLiteStore(t)
	project := "linear-webhook-cli-project"
	receivedAt := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	for i, delivery := range []taskstore.LinearWebhookDelivery{
		{DeliveryID: "delivery-1", Status: "accepted", Reason: "", LinearIssueID: "issue-1"},
		{DeliveryID: "delivery-2", Status: "rejected", Reason: "invalid_signature", LinearIssueID: "issue-2"},
	} {
		delivery.ReceivedAt = receivedAt.Add(time.Duration(i) * time.Minute)
		recorded, err := store.RecordLinearWebhookDelivery(project, delivery)
		require.NoError(t, err)
		require.True(t, recorded)
	}

	cmd := NewLinearCmd()
	deliveries, _, err := cmd.Find([]string{"webhook", "deliveries"})
	require.NoError(t, err)
	var out bytes.Buffer
	deliveries.SetOut(&out)

	require.NoError(t, writeLinearWebhookDeliveries(deliveries, store, project, 500))
	assert.Equal(t, "delivery-2\trejected\tinvalid_signature\t2026-05-01T10:01:00Z\tissue-2\ndelivery-1\taccepted\t\t2026-05-01T10:00:00Z\tissue-1\n", out.String())
}

func sortedLinearRows(rows []linearDiscoverRow) []linearDiscoverRow {
	out := append([]linearDiscoverRow(nil), rows...)
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Name < out[i].Name || (out[j].Name == out[i].Name && out[j].ID < out[i].ID) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

type linearDiscoverFake struct {
	labels []linear.Label
	users  []linear.User
}

func (f *linearDiscoverFake) Viewer(context.Context) (*linear.User, error) {
	return &linear.User{ID: "user-1", Name: "viewer"}, nil
}

func (f *linearDiscoverFake) Users(context.Context, linear.PageOptions) ([]linear.User, linear.PageInfo, error) {
	if f.users == nil {
		return []linear.User{{ID: "user-1", Name: "viewer"}}, linear.PageInfo{}, nil
	}
	return append([]linear.User(nil), f.users...), linear.PageInfo{}, nil
}

func (f *linearDiscoverFake) Labels(context.Context, linear.PageOptions) ([]linear.Label, linear.PageInfo, error) {
	return append([]linear.Label(nil), f.labels...), linear.PageInfo{}, nil
}

func (f *linearDiscoverFake) Teams(context.Context, linear.PageOptions) ([]linear.Team, linear.PageInfo, error) {
	return []linear.Team{{ID: "team-1", Name: "engineering"}}, linear.PageInfo{}, nil
}

func (f *linearDiscoverFake) WorkflowStates(context.Context, linear.PageOptions) ([]linear.WorkflowState, linear.PageInfo, error) {
	return []linear.WorkflowState{{ID: "state-1", Name: "started"}}, linear.PageInfo{}, nil
}

func (f *linearDiscoverFake) Projects(context.Context, linear.PageOptions) ([]linear.Project, linear.PageInfo, error) {
	return []linear.Project{{ID: "project-1", Name: "phase 1"}}, linear.PageInfo{}, nil
}
