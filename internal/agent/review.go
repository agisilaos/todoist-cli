package agent

import (
	"encoding/json"
	"fmt"
)

// Review preserves selection and dispositions alongside ordinary executable actions.
type Review struct {
	Version int          `json:"version"`
	Filter  string       `json:"filter"`
	Tasks   []ReviewTask `json:"tasks"`
}

type ReviewTask struct {
	ID          string                     `json:"id"`
	Content     string                     `json:"content"`
	Disposition string                     `json:"disposition"`
	Snapshot    map[string]json.RawMessage `json:"snapshot"`
	Actions     []int                      `json:"actions"`
}

func ValidateReview(plan Plan) error {
	if plan.Review == nil {
		return nil
	}
	if plan.Review.Version != 1 {
		return fmt.Errorf("unsupported review version")
	}
	seen := map[string]bool{}
	indices := map[int]bool{}
	for _, task := range plan.Review.Tasks {
		var id string
		_ = json.Unmarshal(task.Snapshot["id"], &id)
		if task.ID == "" || seen[task.ID] || id != task.ID || len(task.Snapshot) < 2 {
			return fmt.Errorf("invalid review task snapshot: %s", task.ID)
		}
		seen[task.ID] = true
		switch task.Disposition {
		case "keep", "skip":
			if len(task.Actions) != 0 {
				return fmt.Errorf("%s cannot have actions", task.Disposition)
			}
		case "change", "complete":
			if len(task.Actions) == 0 {
				return fmt.Errorf("%s requires actions", task.Disposition)
			}
		default:
			return fmt.Errorf("invalid review disposition: %s", task.Disposition)
		}
		previous := -1
		for _, idx := range task.Actions {
			if idx < 0 || idx >= len(plan.Actions) || indices[idx] || idx <= previous {
				return fmt.Errorf("invalid review action index")
			}
			a := plan.Actions[idx]
			if a.TaskID != task.ID {
				return fmt.Errorf("review action task mismatch")
			}
			if task.Disposition == "complete" {
				if len(task.Actions) != 1 || a.Type != "task_complete" {
					return fmt.Errorf("invalid completion disposition")
				}
			} else if a.Type != "task_update" && a.Type != "task_move" {
				return fmt.Errorf("unsupported review action: %s", a.Type)
			}
			indices[idx] = true
			previous = idx
		}
	}
	if len(indices) != len(plan.Actions) {
		return fmt.Errorf("review must account for every action")
	}
	return nil
}
