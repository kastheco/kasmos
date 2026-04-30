package linearreceipt

import (
	"context"
	"testing"

	"github.com/kastheco/kasmos/config/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifyPRCreatedSkipsEmptyPRURL(t *testing.T) {
	client := &mockLinearClient{}
	hook, _ := newLinkedHook(t, enabledConfig(), client, nil)

	hook.NotifyPRCreated(context.Background(), testTask, "")

	assert.Equal(t, 0, client.createCount())
}

func TestNotifyPRCreatedSkipsUnlinkedTask(t *testing.T) {
	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(testProject, taskstore.TaskEntry{Filename: testTask, Status: taskstore.StatusReady}))
	client := &mockLinearClient{}
	hook := NewHook(enabledConfig(), store, client, nil, testProject)

	hook.NotifyPRCreated(context.Background(), testTask, "https://github.com/kastheco/kasmos/pull/123")

	assert.Equal(t, 0, client.createCount())
}

func TestNotifyPRCreatedPostsReceipt(t *testing.T) {
	client := &mockLinearClient{}
	hook, _ := newLinkedHook(t, enabledConfig(), client, nil)

	hook.NotifyPRCreated(context.Background(), testTask, "https://github.com/kastheco/kasmos/pull/123")

	require.Equal(t, 1, client.createCount())
	assert.Contains(t, client.firstCreate().body, "kasmos pr receipt")
	assert.Contains(t, client.firstCreate().body, "pr: https://github.com/kastheco/kasmos/pull/123")
}

func TestNotifyPRCreatedSkipsDisabledConfig(t *testing.T) {
	client := &mockLinearClient{}
	hook, _ := newLinkedHook(t, Config{}, client, nil)

	hook.NotifyPRCreated(context.Background(), testTask, "https://github.com/kastheco/kasmos/pull/123")

	assert.Equal(t, 0, client.createCount())
}

func TestNotifyPRCreatedNilReceiverIsSafe(t *testing.T) {
	require.NotPanics(t, func() {
		var hook *Hook
		hook.NotifyPRCreated(context.Background(), testTask, "https://github.com/kastheco/kasmos/pull/123")
	})
}

func TestNotifyPlanMergedGatingAndReceipt(t *testing.T) {
	client := &mockLinearClient{}
	hook, _ := newLinkedHook(t, enabledConfig(), client, nil)

	hook.NotifyPlanMerged(context.Background(), testTask)

	require.Equal(t, 1, client.createCount())
	assert.Contains(t, client.firstCreate().body, "kasmos merge receipt")
}

func TestNotifyPlanMergedSkipsDisabledAndUnlinked(t *testing.T) {
	client := &mockLinearClient{}
	cfg := enabledConfig()
	cfg.MergeReceipts = false
	hook, _ := newLinkedHook(t, cfg, client, nil)
	hook.NotifyPlanMerged(context.Background(), testTask)
	assert.Equal(t, 0, client.createCount())

	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(testProject, taskstore.TaskEntry{Filename: testTask, Status: taskstore.StatusDone}))
	unlinked := NewHook(enabledConfig(), store, client, nil, testProject)
	unlinked.NotifyPlanMerged(context.Background(), testTask)
	assert.Equal(t, 0, client.createCount())
}

func TestNotifyPlanCancelledGatingAndReceipt(t *testing.T) {
	client := &mockLinearClient{}
	hook, _ := newLinkedHook(t, enabledConfig(), client, nil)

	hook.NotifyPlanCancelled(context.Background(), testTask, "superseded")

	require.Equal(t, 1, client.createCount())
	assert.Contains(t, client.firstCreate().body, "kasmos cancellation receipt")
	assert.Contains(t, client.firstCreate().body, "reason: superseded")
}

func TestNotifyPlanCancelledSkipsDisabledAndUnlinked(t *testing.T) {
	client := &mockLinearClient{}
	cfg := enabledConfig()
	cfg.CancelReceipt = false
	hook, _ := newLinkedHook(t, cfg, client, nil)
	hook.NotifyPlanCancelled(context.Background(), testTask, "superseded")
	assert.Equal(t, 0, client.createCount())

	store := taskstore.NewTestStore(t)
	require.NoError(t, store.Create(testProject, taskstore.TaskEntry{Filename: testTask, Status: taskstore.StatusReady}))
	unlinked := NewHook(enabledConfig(), store, client, nil, testProject)
	unlinked.NotifyPlanCancelled(context.Background(), testTask, "superseded")
	assert.Equal(t, 0, client.createCount())
}
