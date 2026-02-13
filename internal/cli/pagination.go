package cli

import "net/url"

func fetchPaginated[T any](ctx *Context, path string, query url.Values, all bool) ([]T, string, error) {
	q := cloneQuery(query)
	var items []T
	var next string
	for {
		var page struct {
			Results    []T    `json:"results"`
			NextCursor string `json:"next_cursor"`
		}
		reqCtx, cancel := requestContext(ctx)
		reqID, err := ctx.Client.Get(reqCtx, path, q, &page)
		cancel()
		if err != nil {
			return nil, "", err
		}
		setRequestID(ctx, reqID)
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
