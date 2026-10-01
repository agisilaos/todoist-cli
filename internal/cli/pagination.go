package cli

import (
	"fmt"
	"net/url"
)

func fetchPaginated[T any](ctx *Context, path string, query url.Values, all bool) ([]T, string, error) {
	q := cloneQuery(query)
	items := make([]T, 0)
	var next string
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
		reqCtx, cancel := requestContext(ctx)
		reqID, err := ctx.Client.Get(reqCtx, path, q, response)
		cancel()
		if err != nil {
			return nil, "", err
		}
		setRequestID(ctx, reqID)
		if completedPath {
			page.Results, page.NextCursor = completed.Items, completed.NextCursor
		}
		items = append(items, page.Results...)
		next = page.NextCursor
		if !all || next == "" {
			break
		}
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
