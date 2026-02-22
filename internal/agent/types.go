package agent

type PlannerRequest struct {
	Instruction string         `json:"instruction"`
	Profile     string         `json:"profile"`
	Context     PlannerContext `json:"context"`
	Now         string         `json:"now"`
}

type PlannerContext struct {
	Projects       []any `json:"projects"`
	Sections       []any `json:"sections"`
	Labels         []any `json:"labels"`
	CompletedTasks []any `json:"completed_tasks,omitempty"`
}

type Plan struct {
	Version      int         `json:"version"`
	Instruction  string      `json:"instruction"`
	CreatedAt    string      `json:"created_at"`
	ConfirmToken string      `json:"confirm_token"`
	Summary      PlanSummary `json:"summary"`
	Actions      []Action    `json:"actions"`
	AppliedAt    string      `json:"applied_at,omitempty"`
}

type PlanSummary struct {
	Tasks    int `json:"tasks"`
	Projects int `json:"projects"`
	Sections int `json:"sections"`
	Labels   int `json:"labels"`
	Comments int `json:"comments"`
}

type Action struct {
	Type         string   `json:"type"`
	TaskID       string   `json:"task_id,omitempty"`
	ProjectID    string   `json:"project_id,omitempty"`
	SectionID    string   `json:"section_id,omitempty"`
	LabelID      string   `json:"label_id,omitempty"`
	CommentID    string   `json:"comment_id,omitempty"`
	Idempotent   bool     `json:"idempotent,omitempty"`
	Content      string   `json:"content,omitempty"`
	Description  string   `json:"description,omitempty"`
	Name         string   `json:"name,omitempty"`
	Labels       []string `json:"labels,omitempty"`
	Project      string   `json:"project,omitempty"`
	Section      string   `json:"section,omitempty"`
	Parent       string   `json:"parent,omitempty"`
	Priority     int      `json:"priority,omitempty"`
	Due          string   `json:"due,omitempty"`
	DueDate      string   `json:"due_date,omitempty"`
	DueDatetime  string   `json:"due_datetime,omitempty"`
	DueLang      string   `json:"due_lang,omitempty"`
	Duration     int      `json:"duration,omitempty"`
	DurationUnit string   `json:"duration_unit,omitempty"`
	Deadline     string   `json:"deadline_date,omitempty"`
	Assignee     string   `json:"assignee_id,omitempty"`
	Color        string   `json:"color,omitempty"`
	Order        int      `json:"order,omitempty"`
	Favorite     *bool    `json:"is_favorite,omitempty"`
}
