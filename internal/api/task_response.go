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
		Due json.RawMessage `json:"due"`
	}
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*t = Task(value)
	t.DueReturned = len(fields.Due) > 0
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
