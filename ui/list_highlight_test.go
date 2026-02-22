package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/kastheco/klique/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestInstance(title, planFile string) *session.Instance {
	return &session.Instance{
		Title:    title,
		PlanFile: planFile,
		Status:   session.Running,
	}
}

func TestListHighlightFilter_MatchedFirst(t *testing.T) {
	s := spinner.New()
	l := NewList(&s, false)

	a := makeTestInstance("alpha", "plan-a.md")
	b := makeTestInstance("bravo", "plan-b.md")
	c := makeTestInstance("charlie", "plan-a.md")

	l.AddInstance(a)()
	l.AddInstance(b)()
	l.AddInstance(c)()

	l.SetHighlightFilter("plan", "plan-a.md")

	items := l.GetFilteredInstances()
	require.Len(t, items, 3)
	// Matched instances (plan-a.md) should come first
	assert.Equal(t, "plan-a.md", items[0].PlanFile)
	assert.Equal(t, "plan-a.md", items[1].PlanFile)
	// Unmatched last
	assert.Equal(t, "plan-b.md", items[2].PlanFile)
}

func TestListHighlightFilter_EmptyShowsAll(t *testing.T) {
	s := spinner.New()
	l := NewList(&s, false)

	a := makeTestInstance("alpha", "plan-a.md")
	b := makeTestInstance("bravo", "plan-b.md")

	l.AddInstance(a)()
	l.AddInstance(b)()

	l.SetHighlightFilter("", "")

	items := l.GetFilteredInstances()
	assert.Len(t, items, 2)
}

func TestListHighlightFilter_TopicMatch(t *testing.T) {
	s := spinner.New()
	l := NewList(&s, false)

	a := makeTestInstance("alpha", "plan-a.md")
	a.Topic = "auth"
	b := makeTestInstance("bravo", "plan-b.md")
	b.Topic = "deploy"
	c := makeTestInstance("charlie", "plan-c.md")
	c.Topic = "auth"

	l.AddInstance(a)()
	l.AddInstance(b)()
	l.AddInstance(c)()

	l.SetHighlightFilter("topic", "auth")

	items := l.GetFilteredInstances()
	require.Len(t, items, 3)
	assert.Equal(t, "auth", items[0].Topic)
	assert.Equal(t, "auth", items[1].Topic)
	assert.Equal(t, "deploy", items[2].Topic)
}

func TestListIsHighlighted(t *testing.T) {
	s := spinner.New()
	l := NewList(&s, false)

	a := makeTestInstance("alpha", "plan-a.md")
	b := makeTestInstance("bravo", "plan-b.md")

	l.AddInstance(a)()
	l.AddInstance(b)()

	l.SetHighlightFilter("plan", "plan-a.md")

	assert.True(t, l.IsHighlighted(a))
	assert.False(t, l.IsHighlighted(b))
}

func TestListIsHighlighted_NoFilter(t *testing.T) {
	s := spinner.New()
	l := NewList(&s, false)

	a := makeTestInstance("alpha", "plan-a.md")
	l.AddInstance(a)()

	l.SetHighlightFilter("", "")
	// No filter active — everything is "highlighted" (normal rendering)
	assert.True(t, l.IsHighlighted(a))
}
