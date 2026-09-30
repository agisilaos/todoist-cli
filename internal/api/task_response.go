package api

import "encoding/json"

// UnmarshalJSON retains response facts used by capture receipts while leaving
// the established JSON/NDJSON serialization unchanged.
func (t *Task) UnmarshalJSON(data []byte) error {
	type taskValue Task
	var value taskValue
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	var fields struct {
		Due         json.RawMessage `json:"due"`
		Description json.RawMessage `json:"description"`
		SectionID   json.RawMessage `json:"section_id"`
		ParentID    json.RawMessage `json:"parent_id"`
		Checked     json.RawMessage `json:"checked"`
		CompletedAt json.RawMessage `json:"completed_at"`
		NoteCount   json.RawMessage `json:"note_count"`
	}
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*t = Task(value)
	t.DueReturned = len(fields.Due) > 0
	valueReturned := func(value json.RawMessage) bool {
		return len(value) > 0 && string(value) != "null"
	}
	t.Returned = TaskReturnedFields{
		Description: valueReturned(fields.Description),
		SectionID:   len(fields.SectionID) > 0,
		ParentID:    len(fields.ParentID) > 0,
		Checked:     valueReturned(fields.Checked),
		CompletedAt: len(fields.CompletedAt) > 0,
		NoteCount:   valueReturned(fields.NoteCount),
	}
	return nil
}

func (d *Due) UnmarshalJSON(data []byte) error {
	type dueValue Due
	var value dueValue
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	var facts struct {
		Timezone    *string `json:"timezone"`
		IsRecurring *bool   `json:"is_recurring"`
	}
	// These previously ignored fields are advisory. A malformed optional fact
	// stays unknown rather than turning an accepted creation into a failure.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if err := json.Unmarshal(fields["timezone"], &facts.Timezone); err != nil {
		facts.Timezone = nil
	}
	if err := json.Unmarshal(fields["is_recurring"], &facts.IsRecurring); err != nil {
		facts.IsRecurring = nil
	}
	*d = Due(value)
	d.Timezone = facts.Timezone
	d.IsRecurring = facts.IsRecurring
	return nil
}
