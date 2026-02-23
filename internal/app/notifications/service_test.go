package notifications

import (
	"testing"

	"github.com/agisilaos/todoist-cli/internal/api"
)

func TestListFiltersAndPagination(t *testing.T) {
	items := []api.Notification{
		{ID: "n1", Type: "item_assigned", IsUnread: true, CreatedAt: "2026-02-23T10:00:00Z"},
		{ID: "n2", Type: "item_completed", IsUnread: false, CreatedAt: "2026-02-22T10:00:00Z"},
		{ID: "n3", Type: "item_assigned", IsUnread: true, CreatedAt: "2026-02-21T10:00:00Z"},
	}
	out, err := List(items, ListInput{Type: "item_assigned", Unread: true, Limit: 1, Offset: 0})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if out.Total != 2 || len(out.Items) != 1 || !out.HasMore || out.Items[0].ID != "n1" {
		t.Fatalf("unexpected result: %#v", out)
	}
}

func TestListRejectsUnreadReadConflict(t *testing.T) {
	if _, err := List(nil, ListInput{Unread: true, Read: true}); err == nil {
		t.Fatalf("expected error")
	}
}
