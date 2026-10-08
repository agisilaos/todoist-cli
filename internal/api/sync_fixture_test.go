package api

import (
	"encoding/json"
	"testing"
)

func successfulSyncResponse(t *testing.T, commands string) []byte {
	t.Helper()
	var entries []struct{ UUID string }
	if err := json.Unmarshal([]byte(commands), &entries); err != nil || len(entries) == 0 {
		t.Fatalf("invalid fixture commands: %v", err)
	}
	statuses := map[string]string{}
	for _, entry := range entries {
		statuses[entry.UUID] = "ok"
	}
	data, err := json.Marshal(map[string]any{"sync_status": statuses})
	if err != nil {
		t.Fatal(err)
	}
	return data
}
