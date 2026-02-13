package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type replayJournal struct {
	Applied map[string]string `json:"applied"`
}

func loadReplayJournal(ctx *Context) (replayJournal, string, error) {
	if ctx == nil || ctx.ConfigPath == "" {
		return replayJournal{Applied: map[string]string{}}, "", nil
	}
	path := filepath.Join(filepath.Dir(ctx.ConfigPath), "agent_replay.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return replayJournal{Applied: map[string]string{}}, path, nil
		}
		return replayJournal{}, path, err
	}
	var j replayJournal
	if err := json.Unmarshal(data, &j); err != nil {
		return replayJournal{}, path, err
	}
	if j.Applied == nil {
		j.Applied = map[string]string{}
	}
	return j, path, nil
}

func saveReplayJournal(path string, j replayJournal) error {
	if path == "" {
		return nil
	}
	if j.Applied == nil {
		j.Applied = map[string]string{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func makeReplayKey(confirmToken string, actionIndex int, action Action) string {
	payload := struct {
		Confirm string `json:"confirm"`
		Index   int    `json:"index"`
		Action  Action `json:"action"`
	}{
		Confirm: confirmToken,
		Index:   actionIndex,
		Action:  action,
	}
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func markReplayApplied(j *replayJournal, key string, at time.Time) {
	if j == nil {
		return
	}
	if j.Applied == nil {
		j.Applied = map[string]string{}
	}
	j.Applied[key] = at.UTC().Format(time.RFC3339)
}
