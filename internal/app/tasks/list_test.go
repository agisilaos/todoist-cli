package tasks

import (
	"testing"
	"time"
)

func TestPlanListActiveDefault(t *testing.T) {
	plan, err := PlanList(time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC), ListInput{})
	if err != nil {
		t.Fatalf("PlanList: %v", err)
	}
	if plan.Mode != "active" {
		t.Fatalf("expected active mode, got %#v", plan)
	}
}

func TestPlanListFromPreset(t *testing.T) {
	plan, err := PlanList(time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC), ListInput{Preset: "today"})
	if err != nil {
		t.Fatalf("PlanList: %v", err)
	}
	if plan.Mode != "filter" || plan.Filter != "today" {
		t.Fatalf("unexpected plan: %#v", plan)
	}
}

func TestPlanListCompletedNormalizesRange(t *testing.T) {
	now := time.Date(2026, 2, 22, 10, 0, 0, 0, time.UTC)
	plan, err := PlanList(now, ListInput{Completed: true, Since: "30 days ago"})
	if err != nil {
		t.Fatalf("PlanList: %v", err)
	}
	if plan.Mode != "completed" || plan.Since != "2026-01-23" || plan.Until != "2026-02-22" {
		t.Fatalf("unexpected completed plan: %#v", plan)
	}
}

func TestPlanListRejectsInvalidPreset(t *testing.T) {
	_, err := PlanList(time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC), ListInput{Preset: "nope"})
	if err == nil {
		t.Fatalf("expected error")
	}
}
