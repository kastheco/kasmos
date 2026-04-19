package auditlog_test

import (
	"testing"

	"github.com/kastheco/kasmos/config/auditlog"
	"github.com/stretchr/testify/assert"
)

func TestEventKind_String(t *testing.T) {
	assert.Equal(t, "agent_spawned", auditlog.EventAgentSpawned.String())
	assert.Equal(t, "plan_transition", auditlog.EventPlanTransition.String())
}

func TestNopLogger_DoesNotPanic(t *testing.T) {
	l := auditlog.NopLogger()
	assert.NotPanics(t, func() {
		l.Emit(auditlog.Event{Kind: auditlog.EventAgentSpawned})
	})
}

func TestWithExecutionMode(t *testing.T) {
	e := auditlog.Event{}
	auditlog.WithExecutionMode("sdk")(&e)
	assert.Equal(t, `{"execution_mode":"sdk"}`, e.Detail)

	auditlog.WithExecutionMode("tmux")(&e)
	assert.Equal(t, `{"execution_mode":"tmux"}`, e.Detail)
}
