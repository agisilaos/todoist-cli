package tasks

import (
	"fmt"
	"strings"
	"time"
)

type ListInput struct {
	Filter      string
	Preset      string
	Completed   bool
	CompletedBy string
	Since       string
	Until       string
}

type ListPlan struct {
	Mode        string
	Filter      string
	CompletedBy string
	Since       string
	Until       string
}

func PlanList(now time.Time, in ListInput) (ListPlan, error) {
	filter := strings.TrimSpace(in.Filter)
	preset := strings.TrimSpace(in.Preset)
	completedBy := strings.TrimSpace(in.CompletedBy)
	if completedBy == "" {
		completedBy = "completion"
	}
	if in.Completed {
		since, until, err := NormalizeCompletedDateRange(now, in.Since, in.Until)
		if err != nil {
			return ListPlan{}, err
		}
		return ListPlan{
			Mode:        "completed",
			Filter:      filter,
			CompletedBy: completedBy,
			Since:       since,
			Until:       until,
		}, nil
	}
	if filter == "" && preset != "" {
		switch preset {
		case "today":
			filter = "today"
		case "overdue":
			filter = "overdue"
		case "next7":
			filter = "next 7 days"
		default:
			return ListPlan{}, fmt.Errorf("invalid preset: %s", preset)
		}
	}
	if filter != "" {
		return ListPlan{Mode: "filter", Filter: filter, CompletedBy: completedBy}, nil
	}
	return ListPlan{Mode: "active", CompletedBy: completedBy}, nil
}
