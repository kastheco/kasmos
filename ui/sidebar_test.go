package ui

import "testing"

func TestPlanDisplayName(t *testing.T) {
	if got := planDisplayName("2026-02-20-my-feature.md"); got != "my-feature" {
		t.Fatalf("planDisplayName() = %q, want %q", got, "my-feature")
	}
	if got := planDisplayName("plain-plan.md"); got != "plain-plan" {
		t.Fatalf("planDisplayName() = %q, want %q", got, "plain-plan")
	}
}

func TestSidebarSetItems_IncludesPlansSectionBeforeTopics(t *testing.T) {
	s := NewSidebar()
	s.SetPlans([]PlanDisplay{{Filename: "2026-02-20-plan-orchestration.md", Status: "in_progress"}})
	s.SetItems([]string{"alpha"}, map[string]int{"alpha": 1}, 0, map[string]bool{"alpha": false}, map[string]TopicStatus{"alpha": {}})

	if len(s.items) < 5 {
		t.Fatalf("expected at least 5 sidebar items, got %d", len(s.items))
	}

	if s.items[1].Name != "Plans" || !s.items[1].IsSection {
		t.Fatalf("item[1] = %+v, want Plans section", s.items[1])
	}
	if s.items[2].ID != SidebarPlanPrefix+"2026-02-20-plan-orchestration.md" {
		t.Fatalf("item[2].ID = %q, want %q", s.items[2].ID, SidebarPlanPrefix+"2026-02-20-plan-orchestration.md")
	}
	if s.items[3].Name != "Topics" || !s.items[3].IsSection {
		t.Fatalf("item[3] = %+v, want Topics section", s.items[3])
	}
}

func TestGetSelectedPlanFile(t *testing.T) {
	s := NewSidebar()
	s.SetPlans([]PlanDisplay{{Filename: "plan.md", Status: "ready"}})
	s.SetItems(nil, map[string]int{}, 0, map[string]bool{}, map[string]TopicStatus{})

	if s.GetSelectedPlanFile() != "" {
		t.Fatalf("selected plan should be empty when All is selected")
	}

	s.ClickItem(2)
	if got := s.GetSelectedPlanFile(); got != "plan.md" {
		t.Fatalf("GetSelectedPlanFile() = %q, want %q", got, "plan.md")
	}
}
