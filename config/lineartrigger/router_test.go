package lineartrigger

import (
	"testing"

	"github.com/kastheco/kasmos/internal/linear"
	"github.com/stretchr/testify/assert"
)

func TestRouterResolve(t *testing.T) {
	issue := linear.Issue{
		Team:    &linear.Team{ID: "team-1"},
		Project: &linear.Project{ID: "project-1"},
		Labels:  []linear.Label{{ID: "ready"}, {ID: "agent-ready"}},
	}

	t.Run("missing route", func(t *testing.T) {
		router := NewRouter(Config{Routes: []Route{{TeamID: "team-2", Topic: "ops"}}})

		result := router.Resolve(issue)

		assert.Nil(t, result.Match)
		assert.Equal(t, "route_missing", result.Reason)
		assert.False(t, result.Ambiguous)
	})

	t.Run("single route matches team project and labels", func(t *testing.T) {
		router := NewRouter(Config{Routes: []Route{{
			TeamID:        "team-1",
			ProjectID:     "project-1",
			RequireLabels: []string{"ready"},
			Topic:         "eng",
			BranchPrefix:  "linear/",
		}}})

		result := router.Resolve(issue)

		assert.Empty(t, result.Reason)
		assert.False(t, result.Ambiguous)
		assert.Equal(t, &RouteMatch{Topic: "eng", BranchPrefix: "linear/"}, result.Match)
	})

	t.Run("ambiguous when multiple configured routes match", func(t *testing.T) {
		router := NewRouter(Config{Routes: []Route{
			{TeamID: "team-1", ProjectID: "project-1", RequireLabels: []string{"ready"}, Topic: "eng"},
			{TeamID: "team-1", ProjectID: "project-1", RequireLabels: []string{"agent-ready"}, Topic: "ops"},
		}})

		result := router.Resolve(issue)

		assert.Nil(t, result.Match)
		assert.True(t, result.Ambiguous)
		assert.Equal(t, "route_ambiguous", result.Reason)
		assert.Equal(t, []string{"eng", "ops"}, result.Candidates)
	})

	t.Run("project route does not match issue without project", func(t *testing.T) {
		router := NewRouter(Config{Routes: []Route{{TeamID: "team-1", ProjectID: "project-1", Topic: "eng"}}})

		result := router.Resolve(linear.Issue{Team: &linear.Team{ID: "team-1"}})

		assert.Equal(t, "route_missing", result.Reason)
	})
}
