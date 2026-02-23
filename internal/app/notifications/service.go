package notifications

import (
	"errors"
	"sort"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
)

type ListInput struct {
	Type   string
	Unread bool
	Read   bool
	Limit  int
	Offset int
}

type ListResult struct {
	Items   []api.Notification
	Total   int
	HasMore bool
	Offset  int
	Limit   int
}

func List(items []api.Notification, in ListInput) (ListResult, error) {
	if in.Unread && in.Read {
		return ListResult{}, errors.New("cannot combine --unread and --read")
	}
	filtered := make([]api.Notification, 0, len(items))
	typeSet := make(map[string]struct{})
	if strings.TrimSpace(in.Type) != "" {
		for _, piece := range strings.Split(in.Type, ",") {
			value := strings.ToLower(strings.TrimSpace(piece))
			if value != "" {
				typeSet[value] = struct{}{}
			}
		}
	}
	for _, item := range items {
		if len(typeSet) > 0 {
			if _, ok := typeSet[strings.ToLower(strings.TrimSpace(item.Type))]; !ok {
				continue
			}
		}
		if in.Unread && !item.IsUnread {
			continue
		}
		if in.Read && item.IsUnread {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt > filtered[j].CreatedAt
	})
	total := len(filtered)
	limit := in.Limit
	if limit <= 0 {
		limit = 10
	}
	offset := in.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return ListResult{Items: []api.Notification{}, Total: total, Offset: offset, Limit: limit, HasMore: false}, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return ListResult{
		Items:   filtered[offset:end],
		Total:   total,
		HasMore: end < total,
		Offset:  offset,
		Limit:   limit,
	}, nil
}
