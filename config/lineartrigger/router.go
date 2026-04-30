package lineartrigger

import "github.com/kastheco/kasmos/internal/linear"

// RouteMatch is the kasmos routing data selected for a Linear issue.
type RouteMatch struct {
	Topic        string
	BranchPrefix string
}

// RouteResult explains how route resolution completed.
type RouteResult struct {
	Match      *RouteMatch
	Reason     string
	Ambiguous  bool
	Candidates []string
}

// Router resolves Linear issues to kasmos task topics.
type Router struct {
	cfg Config
}

// NewRouter returns a route resolver for cfg.
func NewRouter(cfg Config) *Router {
	return &Router{cfg: cfg}
}

// Resolve selects the configured route matching the issue team, project, and labels.
func (r *Router) Resolve(issue linear.Issue) RouteResult {
	var matches []Route
	for _, route := range r.cfg.Routes {
		if routeMatchesIssue(route, issue) {
			matches = append(matches, route)
		}
	}

	switch len(matches) {
	case 0:
		return RouteResult{Reason: "route_missing"}
	case 1:
		return RouteResult{
			Match: &RouteMatch{
				Topic:        matches[0].Topic,
				BranchPrefix: matches[0].BranchPrefix,
			},
		}
	default:
		candidates := make([]string, 0, len(matches))
		for _, route := range matches {
			candidates = append(candidates, route.Topic)
		}
		return RouteResult{
			Reason:     "route_ambiguous",
			Ambiguous:  true,
			Candidates: candidates,
		}
	}
}

func routeMatchesIssue(route Route, issue linear.Issue) bool {
	if issue.Team == nil || route.TeamID != issue.Team.ID {
		return false
	}
	if route.ProjectID != "" {
		if issue.Project == nil || route.ProjectID != issue.Project.ID {
			return false
		}
	}
	if len(route.RequireLabels) == 0 {
		return true
	}

	labels := make(map[string]bool, len(issue.Labels))
	for _, label := range issue.Labels {
		labels[label.ID] = true
	}
	for _, labelID := range route.RequireLabels {
		if !labels[labelID] {
			return false
		}
	}
	return true
}
