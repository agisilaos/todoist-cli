// Package credentials owns profile persistence independently of CLI workflows.
package credentials

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sort"

	"github.com/agisilaos/todoist-cli/internal/config"
)

// Store keeps authorization evidence paired with the selected token.
// Inspect and List never retrieve native secrets.
type Store interface {
	Load(context.Context, string) (config.Credential, error)
	Inspect(context.Context, string) (Info, error)
	List(context.Context) ([]string, error)
	Save(context.Context, string, config.Credential, string) error
	Delete(context.Context, string) error
	Migrate(context.Context, string, string) error
	Repair(context.Context, string) error
	Probe(context.Context, string) error
}

type Info struct {
	Recovery      string          `json:"recovery,omitempty"`
	Configured    bool            `json:"configured"`
	Backend       string          `json:"backend"`
	Accessibility string          `json:"accessibility"`
	Authorization json.RawMessage `json:"-"`
}

// Secrets is the native boundary. Errors must be classified before rendering.
// Write creates a fresh entry; it must not replace an existing entry.
type Secrets interface {
	Read(context.Context, string) (string, error)
	Write(context.Context, string, string) error
	Delete(context.Context, string) error
	Probe(context.Context) error
}

type ProfileStore struct {
	path   string
	files  Persistence
	native Secrets
}

func New(path string, native Secrets, files Persistence) *ProfileStore {
	if files == nil {
		files = Disk{}
	}
	return &ProfileStore{path: path, files: files, native: native}
}

func (s *ProfileStore) read() (config.Credentials, error) {
	data, err := s.files.Read(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return config.Credentials{Profiles: map[string]config.Credential{}}, nil
	}
	if err != nil {
		return config.Credentials{}, failure(IO)
	}
	var all config.Credentials
	if json.Unmarshal(data, &all) != nil {
		return all, failure(Corrupt)
	}
	if all.Profiles == nil {
		all.Profiles = map[string]config.Credential{}
	}
	return all, nil
}
func (s *ProfileStore) save(all config.Credentials) error {
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return failure(Corrupt)
	}
	if s.files.Write(s.path, data) != nil {
		return failure(IO)
	}
	return nil
}
func (s *ProfileStore) Inspect(ctx context.Context, name string) (Info, error) {
	all, err := s.read()
	if err != nil {
		return Info{}, err
	}
	c, exists := all.Profiles[name]
	recovery, err := s.recoveryState(name, c, exists)
	if err != nil {
		return Info{}, err
	}
	d, err := s.descriptor(c, name)
	if err != nil {
		return Info{}, err
	}
	return Info{Recovery: recovery, Configured: !d.Disabled && (c.Token != "" || d.Backend == "keychain"), Backend: d.Backend, Accessibility: "unchecked", Authorization: c.Authorization}, nil
}
func (s *ProfileStore) Load(ctx context.Context, name string) (config.Credential, error) {
	return s.load(ctx, name, true)
}
func (s *ProfileStore) load(ctx context.Context, name string, retry bool) (config.Credential, error) {
	all, err := s.read()
	if err != nil {
		return config.Credential{}, err
	}
	c, exists := all.Profiles[name]
	d, err := s.descriptor(c, name)
	if err != nil {
		return config.Credential{}, err
	}
	if d.Disabled {
		return config.Credential{}, nil
	}
	if _, err := s.recoveryState(name, c, exists); err != nil {
		return config.Credential{}, err
	}
	if d.Backend == "keychain" {
		if s.native == nil {
			return config.Credential{}, failure(Unavailable)
		}
		c.Token, err = s.native.Read(ctx, d.Entry)
		if err != nil {
			var e *Error
			if retry && errors.As(err, &e) && e.Kind == Missing {
				latest, readErr := s.read()
				if readErr != nil {
					return config.Credential{}, readErr
				}
				if !sameJSON(latest.Profiles[name].Storage, c.Storage) {
					return s.load(ctx, name, false)
				}
			}
			return config.Credential{}, Normalize(err)
		}
		if c.Token == "" {
			return config.Credential{}, failure(Missing)
		}
	}
	return c, nil
}
func (s *ProfileStore) List(ctx context.Context) ([]string, error) {
	all, err := s.read()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(all.Profiles))
	for name := range all.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}
func (s *ProfileStore) Probe(ctx context.Context, name string) error {
	info, err := s.Inspect(ctx, name)
	if err != nil {
		return err
	}
	if info.Recovery != "" {
		if info.Recovery == "cleanup" {
			yes := true
			return &Error{Kind: Cleanup, Committed: &yes}
		}
		return failure(Recovery)
	}
	if info.Backend == "keychain" {
		if s.native == nil {
			return failure(Unavailable)
		}
		return Normalize(s.native.Probe(ctx))
	}
	return nil
}
