package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/output"
)

type taskAcknowledgement struct {
	ID              string `json:"id,omitempty"`
	Status          string `json:"status"`
	Operation       string `json:"operation"`
	ResultAvailable *bool  `json:"result_available,omitempty"`
}
type taskWriteStep struct {
	Operation  string `json:"operation"`
	Outcome    string `json:"outcome"`
	Dispatched bool   `json:"dispatched"`
	RequestID  string `json:"request_id,omitempty"`
}

func writeTaskAcknowledgement(ctx *Context, id, operation, status string) error {
	ack := taskAcknowledgement{ID: id, Operation: operation, Status: status}
	if status == "accepted" {
		available := false
		ack.ResultAvailable = &available
	}
	if ctx.Mode == output.ModeJSON || ctx.Mode == output.ModeNDJSON {
		return writeStructuredValue(ctx, ack)
	}
	if status == "unchanged" {
		fmt.Fprintf(ctx.Stdout, "No change needed; no write dispatched. ID: %s\n", captureText(id))
		return nil
	}
	fmt.Fprintf(ctx.Stdout, "Task write accepted; resulting task data unavailable.\n")
	if id != "" {
		fmt.Fprintf(ctx.Stdout, "Inspect: todoist task view %s --json --task-output-version 2\n", taskShellReference(id))
	} else {
		fmt.Fprintln(ctx.Stdout, "Inspect task lists or Todoist before retrying creation.")
	}
	return nil
}
func returnedTask(raw []byte, id string) (api.Task, bool) {
	var task api.Task
	if json.Unmarshal(raw, &task) != nil {
		return task, false
	}
	value, ok := task.ResponseFact("id").Text()
	return task, ok && usableTaskID(value) && (id == "" || value == id)
}
func writeReturnedTask(ctx *Context, raw []byte, id, operation string, capture bool) error {
	task, ok := returnedTask(raw, id)
	if !ok {
		return writeTaskAcknowledgement(ctx, id, operation, "accepted")
	}
	if capture {
		return writeCaptureReceipt(ctx, task)
	}
	return writeTaskList(ctx, []api.Task{task}, "", false)
}
func postTaskResource(ctx *Context, path string, body map[string]any) ([]byte, string, error) {
	reqCtx, cancel := requestContext(ctx)
	defer cancel()
	raw, requestID, err := ctx.Client.PostWithOptionalResponse(reqCtx, path, body)
	setRequestID(ctx, requestID)
	return raw, requestID, err
}
func nativeTaskCommand(ctx *Context, kind string, args map[string]any) ([]byte, string, error) {
	reqCtx, cancel := requestContext(ctx)
	defer cancel()
	raw, requestID, err := ctx.Client.TaskCommand(reqCtx, kind, args)
	setRequestID(ctx, requestID)
	return raw, requestID, err
}
func taskPath(id string) string { return "/tasks/" + url.PathEscape(id) }
func writePartialTaskEdit(ctx *Context, id string, steps []taskWriteStep, err error) error {
	result := map[string]any{"id": id, "status": "partial", "operation": "task_update", "steps": steps}
	if ctx.Mode == output.ModeJSON || ctx.Mode == output.ModeNDJSON {
		if writeErr := writeStructuredValue(ctx, result); writeErr != nil {
			return writeErr
		}
	} else {
		fmt.Fprintf(ctx.Stdout, "Task edit partially applied. Inspect id:%s before further edits.\n", captureText(id))
		for _, step := range steps {
			fmt.Fprintf(ctx.Stdout, "%s: %s\n", step.Operation, step.Outcome)
		}
	}
	return &CodeError{Code: exitError, Err: fmt.Errorf("task edit incomplete; inspect id:%s before resubmitting: %w", id, err)}
}

func usableTaskID(id string) bool {
	return id != "" && !strings.ContainsFunc(id, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) })
}
