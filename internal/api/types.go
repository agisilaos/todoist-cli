package api

import "encoding/json"
import "strings"

type Paginated[T any] struct {
	Results    []T    `json:"results"`
	NextCursor string `json:"next_cursor"`
}

type Task struct {
	ID          string   `json:"id"`
	Content     string   `json:"content"`
	Description string   `json:"description"`
	ProjectID   string   `json:"project_id"`
	SectionID   string   `json:"section_id"`
	ParentID    string   `json:"parent_id"`
	Labels      []string `json:"labels"`
	Priority    int      `json:"priority"`
	Checked     bool     `json:"checked"`
	Due         *Due     `json:"due"`
	AddedAt     string   `json:"added_at"`
	CompletedAt string   `json:"completed_at"`
	UpdatedAt   string   `json:"updated_at"`
	NoteCount   int      `json:"note_count"`
}

type Due struct {
	Date     string `json:"date,omitempty"`
	Datetime string `json:"datetime,omitempty"`
	String   string `json:"string,omitempty"`
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
	ID         string          `json:"id"`
	Content    string          `json:"content"`
	PostedAt   string          `json:"posted_at"`
	Attachment *FileAttachment `json:"file_attachment"`
}

type FileAttachment struct {
	FileName string `json:"file_name,omitempty"`
	FileType string `json:"file_type,omitempty"`
	FileURL  string `json:"file_url,omitempty"`
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

type Filter struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Query      string `json:"query"`
	Color      string `json:"color"`
	IsFavorite bool   `json:"is_favorite"`
}

type ActivityEvent struct {
	ID              string         `json:"id"`
	EventType       string         `json:"event_type"`
	EventDate       string         `json:"event_date"`
	ObjectType      string         `json:"object_type"`
	ObjectID        string         `json:"object_id"`
	ParentProjectID string         `json:"parent_project_id"`
	InitiatorID     string         `json:"initiator_id"`
	ExtraData       map[string]any `json:"extra_data"`
}

func (e *ActivityEvent) UnmarshalJSON(data []byte) error {
	type rawActivityEvent struct {
		ID              json.RawMessage `json:"id"`
		EventType       string          `json:"event_type"`
		EventDate       string          `json:"event_date"`
		ObjectType      string          `json:"object_type"`
		ObjectID        json.RawMessage `json:"object_id"`
		ParentProjectID json.RawMessage `json:"parent_project_id"`
		InitiatorID     json.RawMessage `json:"initiator_id"`
		ExtraData       map[string]any  `json:"extra_data"`
	}
	var raw rawActivityEvent
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	e.ID = rawJSONScalarToString(raw.ID)
	e.EventType = raw.EventType
	e.EventDate = raw.EventDate
	e.ObjectType = raw.ObjectType
	e.ObjectID = rawJSONScalarToString(raw.ObjectID)
	e.ParentProjectID = rawJSONScalarToString(raw.ParentProjectID)
	e.InitiatorID = rawJSONScalarToString(raw.InitiatorID)
	e.ExtraData = raw.ExtraData
	return nil
}

func rawJSONScalarToString(raw json.RawMessage) string {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return ""
	}
	if strings.HasPrefix(text, `"`) {
		var value string
		if err := json.Unmarshal(raw, &value); err == nil {
			return value
		}
	}
	return text
}

// Some endpoints return only name arrays (shared labels).
type SharedLabel string
