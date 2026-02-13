package cli

import (
	"encoding/json"
	"io"
	"os"
	"time"
)

type progressSink struct {
	out    io.Writer
	closer io.Closer
}

func newProgressSink(path string, defaultOut io.Writer) (*progressSink, error) {
	if path == "" {
		return nil, nil
	}
	if path == "-" {
		return &progressSink{out: defaultOut}, nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &progressSink{out: f, closer: f}, nil
}

func (s *progressSink) Close() error {
	if s == nil || s.closer == nil {
		return nil
	}
	return s.closer.Close()
}

func emitProgress(ctx *Context, eventType string, fields map[string]any) {
	if ctx == nil || ctx.Progress == nil || ctx.Progress.out == nil {
		return
	}
	payload := map[string]any{
		"type":      eventType,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	for k, v := range fields {
		payload[k] = v
	}
	enc := json.NewEncoder(ctx.Progress.out)
	_ = enc.Encode(payload)
}
