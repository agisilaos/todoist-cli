package agent

type Status struct {
	PlannerCmd     string
	PlannerSource  string
	LastPlanPath   string
	LastPlanExists bool
}

type Service struct{}

func (Service) BuildStatus(plannerCmd, plannerSource, lastPlanPath string, hasPlan bool) Status {
	return Status{
		PlannerCmd:     plannerCmd,
		PlannerSource:  plannerSource,
		LastPlanPath:   lastPlanPath,
		LastPlanExists: hasPlan,
	}
}
