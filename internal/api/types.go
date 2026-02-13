package api

type Paginated[T any] struct {
	Results    []T    `json:"results"`
	NextCursor string `json:"next_cursor"`
}

type Task struct {
	ID          string                 `json:"id"`
	Content     string                 `json:"content"`
	Description string                 `json:"description"`
	ProjectID   string                 `json:"project_id"`
	SectionID   string                 `json:"section_id"`
	ParentID    string                 `json:"parent_id"`
	Labels      []string               `json:"labels"`
	Priority    int                    `json:"priority"`
	Checked     bool                   `json:"checked"`
	Due         map[string]interface{} `json:"due"`
	AddedAt     string                 `json:"added_at"`
	CompletedAt string                 `json:"completed_at"`
	UpdatedAt   string                 `json:"updated_at"`
	NoteCount   int                    `json:"note_count"`
}

type Project struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ParentID      string `json:"parent_id"`
	IsArchived    bool   `json:"is_archived"`
	IsShared      bool   `json:"is_shared"`
	IsFavorite    bool   `json:"is_favorite"`
	IsInbox       bool   `json:"inbox_project"`
	ViewStyle     string `json:"view_style"`
	Description   string `json:"description"`
	WorkspaceID   string `json:"workspace_id"`
	CanAssignTask bool   `json:"can_assign_tasks"`
}

type Section struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ProjectID   string `json:"project_id"`
	IsArchived  bool   `json:"is_archived"`
	IsCollapsed bool   `json:"is_collapsed"`
}

type Label struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	Order      int    `json:"order"`
	IsFavorite bool   `json:"is_favorite"`
}

type Comment struct {
	ID         string                 `json:"id"`
	Content    string                 `json:"content"`
	PostedAt   string                 `json:"posted_at"`
	Attachment map[string]interface{} `json:"file_attachment"`
}

type Collaborator struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Workspace struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	Role                  string `json:"role"`
	Plan                  string `json:"plan"`
	DomainName            string `json:"domain_name"`
	CurrentMemberCount    int    `json:"current_member_count"`
	CurrentActiveProjects int    `json:"current_active_projects"`
}

// Some endpoints return only name arrays (shared labels).
type SharedLabel string
