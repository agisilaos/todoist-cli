package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// SetPlannerCommand changes only the planner field in the user configuration.
// Callers hold the same configuration lock used by default-profile updates.
func SetPlannerCommand(path, command string) error {
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read configuration before setting planner: %w", err)
	}
	fields := map[string]json.RawMessage{}
	if err == nil {
		var known Config
		if json.Unmarshal(data, &known) != nil || json.Unmarshal(data, &fields) != nil || fields == nil {
			return errors.New("invalid configuration; planner command was not changed")
		}
	}
	removeKnownJSONFields(fields, "planner_cmd")
	fields["planner_cmd"], _ = json.Marshal(command)
	updated, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return fmt.Errorf("encode planner configuration: %w", err)
	}
	if err := (SelectionDisk{}).Write(path, updated); err != nil {
		return fmt.Errorf("could not confirm saved planner command; inspect 'todoist planner' before retrying: %w", err)
	}
	return nil
}
