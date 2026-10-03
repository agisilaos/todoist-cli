package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
)

func fetchPaginated[T any](ctx *Context, path string, query url.Values, all bool, strict ...bool) ([]T, string, error) {
	q := cloneQuery(query)
	items := make([]T, 0)
	var next string
	seen := map[string]bool{}
	if initial := q.Get("cursor"); initial != "" {
		seen[initial] = true
	}
	validate := len(strict) > 0 && strict[0]
	for {
		var page struct {
			Results    []T    `json:"results"`
			NextCursor string `json:"next_cursor"`
		}
		var completed struct {
			Items      []T    `json:"items"`
			NextCursor string `json:"next_cursor"`
		}
		completedPath := path == "/tasks/completed/by_completion_date" || path == "/tasks/completed/by_due_date"
		var response any = &page
		if completedPath {
			response = &completed
		}
		var raw map[string]json.RawMessage
		if validate {
			response = &raw
		}
		reqCtx, cancel := requestContext(ctx)
		reqID, err := ctx.Client.Get(reqCtx, path, q, response)
		cancel()
		if err != nil {
			return nil, "", err
		}
		setRequestID(ctx, reqID)
		if validate {
			field := "results"
			if completedPath {
				field = "items"
			}
			collection, ok := raw[field]
			if !ok || string(collection) == "null" || json.Unmarshal(collection, &page.Results) != nil || page.Results == nil {
				return nil, "", errors.New("task page missing a valid collection")
			}
			cursor, ok := raw["next_cursor"]
			if !ok {
				return nil, "", errors.New("task page missing cursor evidence")
			}
			if string(cursor) != "null" {
				if json.Unmarshal(cursor, &page.NextCursor) != nil || page.NextCursor == "" {
					return nil, "", errors.New("task page invalid cursor evidence")
				}
			}
		}
		if completedPath && !validate {
			page.Results, page.NextCursor = completed.Items, completed.NextCursor
		}
		items = append(items, page.Results...)
		next = page.NextCursor
		if !all || next == "" {
			break
		}
		if seen[next] {
			return nil, "", errors.New("pagination cursor cycle")
		}
		seen[next] = true
		q.Set("cursor", next)
	}
	return items, next, nil
}

func cloneQuery(in url.Values) url.Values {
	if in == nil {
		return url.Values{}
	}
	out := make(url.Values, len(in))
	for k, vs := range in {
		cp := make([]string, len(vs))
		copy(cp, vs)
		out[k] = cp
	}
	return out
}

func writeCursorNotice(ctx *Context, cursor string) error {
	if cursor == "" {
		return nil
	}
	_, err := fmt.Fprintf(ctx.Stderr, "More available. Use --cursor %q\n", cursor)
	return err
}
