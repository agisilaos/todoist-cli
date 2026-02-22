package agent

import "testing"

func TestBuildStatus(t *testing.T) {
	svc := Service{}
	status := svc.BuildStatus("planner", "config", "/tmp/last_plan.json", true)
	if status.PlannerCmd != "planner" || status.PlannerSource != "config" || !status.LastPlanExists {
		t.Fatalf("unexpected status: %#v", status)
	}
}
