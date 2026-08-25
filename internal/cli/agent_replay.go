package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

var errReplayPathUnavailable = errors.New("replay journal path unavailable")

type replayStore interface {
	Contains(key string) bool
	RecordApplied(key string, at time.Time) error
}

type replayJournal struct {
	Applied map[string]string `json:"applied"`
}

type fileReplayStore struct {
	path    string
	journal replayJournal
	persist func(string, replayJournal) error
}

func replayJournalPath(ctx *Context) string {
	if ctx == nil || ctx.ConfigPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(ctx.ConfigPath), "agent_replay.json")
}

func loadReplayStore(ctx *Context) (*fileReplayStore, error) {
	path := replayJournalPath(ctx)
	if path == "" {
		return nil, errReplayPathUnavailable
	}
	journal, err := readReplayJournal(path)
	if err != nil {
		return nil, err
	}
	return &fileReplayStore{
		path:    path,
		journal: journal,
		persist: persistReplayJournal,
	}, nil
}

func readReplayJournal(path string) (replayJournal, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyReplayJournal(), nil
		}
		return replayJournal{}, err
	}
	var journal replayJournal
	if err := json.Unmarshal(data, &journal); err != nil {
		return replayJournal{}, err
	}
	if journal.Applied == nil {
		journal.Applied = map[string]string{}
	}
	return journal, nil
}

func emptyReplayJournal() replayJournal {
	return replayJournal{Applied: map[string]string{}}
}

func (s *fileReplayStore) Contains(key string) bool {
	_, ok := s.journal.Applied[key]
	return ok
}

func (s *fileReplayStore) RecordApplied(key string, at time.Time) error {
	if s.Contains(key) {
		return nil
	}
	candidate := replayJournal{Applied: make(map[string]string, len(s.journal.Applied)+1)}
	for existingKey, appliedAt := range s.journal.Applied {
		candidate.Applied[existingKey] = appliedAt
	}
	candidate.Applied[key] = at.UTC().Format(time.RFC3339)
	if err := s.persist(s.path, candidate); err != nil {
		return err
	}
	s.journal = candidate
	return nil
}

func (s *fileReplayStore) Len() int {
	return len(s.journal.Applied)
}

func persistReplayJournal(path string, journal replayJournal) error {
	if path == "" {
		return errReplayPathUnavailable
	}
	if journal.Applied == nil {
		journal.Applied = map[string]string{}
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".agent_replay-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
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
