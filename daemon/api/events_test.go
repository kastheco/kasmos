package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventBroadcaster_Subscribe(t *testing.T) {
	b := NewEventBroadcaster()
	defer b.Close()

	sub := b.Subscribe()
	b.Emit(Event{Kind: "signal_processed", Message: "test"})

	select {
	case ev := <-sub:
		assert.Equal(t, "signal_processed", ev.Kind)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestEventBroadcaster_MultipleSubscribers(t *testing.T) {
	b := NewEventBroadcaster()
	defer b.Close()

	sub1 := b.Subscribe()
	sub2 := b.Subscribe()
	b.Emit(Event{Kind: "agent_spawned", Message: "coder for my-plan"})

	ev1 := <-sub1
	ev2 := <-sub2
	assert.Equal(t, ev1.Kind, ev2.Kind)
}

func TestEventBroadcaster_Unsubscribe(t *testing.T) {
	b := NewEventBroadcaster()

	ch := b.Subscribe()
	b.Unsubscribe(ch)

	// After unsubscribe the channel must be closed (readable with zero value, ok==false).
	select {
	case _, ok := <-ch:
		assert.False(t, ok, "channel should be closed after Unsubscribe")
	case <-time.After(time.Second):
		t.Fatal("timeout: channel was not closed by Unsubscribe")
	}

	// The broadcaster's subscriber list must now be empty.
	b.Emit(Event{Kind: "test"}) // must not panic
}

func TestEventBroadcaster_Unsubscribe_AfterClose(t *testing.T) {
	b := NewEventBroadcaster()
	ch := b.Subscribe()

	b.Close()
	// Calling Unsubscribe after Close must be a no-op (not panic).
	b.Unsubscribe(ch)
}

func TestHandler_EventsSSE(t *testing.T) {
	broadcaster := NewEventBroadcaster()
	state := &DaemonState{Running: true}
	h := NewHandlerWithBroadcaster(state, broadcaster)

	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/v1/events", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
}

func TestEventJSON_DetailRoundTrip(t *testing.T) {
	ev := Event{
		Kind:      "wave_failed",
		Message:   "wave 1 needs a decision",
		Timestamp: time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC),
		Detail:    `{"outcome":"wave_decision","retry_generation":1}`,
	}

	encoded, err := json.Marshal(ev)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(encoded, &raw))
	require.Contains(t, raw, "detail")

	var detail map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw["detail"].(string)), &detail))
	assert.Equal(t, "wave_decision", detail["outcome"])
	assert.Equal(t, float64(1), detail["retry_generation"])
}

func TestEventJSON_OmitsEmptyDetail(t *testing.T) {
	ev := Event{
		Kind:      "wave_completed",
		Message:   "wave 1 complete",
		Timestamp: time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC),
	}

	encoded, err := json.Marshal(ev)
	require.NoError(t, err)

	assert.JSONEq(t, `{"kind":"wave_completed","message":"wave 1 complete","timestamp":"2026-04-24T12:00:00Z"}`, string(encoded))
	assert.NotContains(t, string(encoded), "detail")
}
