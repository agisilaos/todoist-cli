package credentials

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agisilaos/todoist-cli/internal/config"
)

const Service = "io.github.agisilaos.todoist-cli.credentials.v1"

type descriptor struct {
	Version   int    `json:"version"`
	Backend   string `json:"backend"`
	Namespace string `json:"namespace,omitempty"`
	Entry     string `json:"entry,omitempty"`
	Revision  string `json:"revision"`
	Disabled  bool   `json:"disabled,omitempty"`
}
type transaction struct {
	Version   int             `json:"version"`
	Profile   string          `json:"profile"`
	Namespace string          `json:"namespace"`
	Before    json.RawMessage `json:"before"`
	After     descriptor      `json:"after"`
	OldEntry  string          `json:"old_entry,omitempty"`
	NewEntry  string          `json:"new_entry,omitempty"`
	Logout    bool            `json:"logout,omitempty"`
}

func digest(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func (s *ProfileStore) namespace() (string, error) {
	path, err := filepath.Abs(filepath.Dir(s.path))
	if err != nil {
		return "", failure(IO)
	}
	canonical, err := canonicalPath(path)
	if err != nil {
		return "", failure(IO)
	}
	return digest(canonical), nil
}
func canonicalPath(path string) (string, error) {
	real, err := filepath.EvalSymlinks(path)
	if err == nil {
		return real, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	parent := filepath.Dir(path)
	if parent == path {
		return "", err
	}
	real, err = canonicalPath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(real, filepath.Base(path)), nil
}
func validHex(s string, n int) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == n && strings.ToLower(s) == s
}
func validEntry(entry, namespace, profile string) bool {
	prefix := namespace + "." + digest(profile) + "."
	return strings.HasPrefix(entry, prefix) && validHex(strings.TrimPrefix(entry, prefix), 16)
}
func (s *ProfileStore) descriptor(c config.Credential, name string) (descriptor, error) {
	if c.Storage == nil {
		return descriptor{Backend: "file"}, nil
	}
	var d descriptor
	if json.Unmarshal(c.Storage, &d) != nil || d.Version == 0 {
		return d, failure(Corrupt)
	}
	if d.Version != 1 {
		return d, failure(Unsupported)
	}
	if !validHex(d.Revision, 16) {
		return d, failure(Corrupt)
	}
	if d.Backend != "file" && d.Backend != "keychain" {
		return d, failure(Unsupported)
	}
	if d.Disabled && (c.Token != "" || d.Entry != "") {
		return d, failure(Corrupt)
	}
	if d.Backend == "file" {
		if d.Entry != "" || d.Namespace != "" {
			return d, failure(Corrupt)
		}
		return d, nil
	}
	if c.Token != "" || !validHex(d.Namespace, 32) || (!d.Disabled && (!validEntry(d.Entry, d.Namespace, name) || !strings.HasSuffix(d.Entry, "."+d.Revision))) {
		return d, failure(Corrupt)
	}
	ns, err := s.namespace()
	if err != nil {
		return d, err
	}
	if ns != d.Namespace {
		return d, failure(Namespace)
	}
	return d, nil
}
func (s *ProfileStore) journalPath() string {
	return filepath.Join(filepath.Dir(s.path), ".credential-transaction.json")
}
func (s *ProfileStore) locked(ctx context.Context) (func(), error) {
	wait, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	unlock, err := s.files.Lock(wait, filepath.Join(filepath.Dir(s.path), ".credentials.lock"))
	if err != nil {
		return nil, Normalize(err)
	}
	return unlock, nil
}
func (s *ProfileStore) pending() (*transaction, error) {
	data, err := s.files.Read(s.journalPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, failure(IO)
	}
	var tx transaction
	if json.Unmarshal(data, &tx) != nil || tx.Version != 1 {
		return nil, failure(Recovery)
	}
	ns, err := s.namespace()
	if err != nil {
		return nil, err
	}
	if tx.Namespace != ns || !validHex(tx.After.Revision, 16) {
		return nil, failure(Recovery)
	}
	for _, id := range []string{tx.NewEntry, tx.OldEntry} {
		if id != "" && !validEntry(id, ns, tx.Profile) {
			return nil, failure(Recovery)
		}
	}
	if tx.NewEntry != "" && tx.NewEntry == tx.OldEntry {
		return nil, failure(Recovery)
	}
	if tx.After.Backend != "file" && tx.After.Backend != "keychain" {
		return nil, failure(Recovery)
	}
	if tx.After.Backend == "keychain" && tx.After.Namespace != ns {
		return nil, failure(Recovery)
	}
	if tx.After.Entry != tx.NewEntry || tx.After.Disabled != tx.Logout {
		return nil, failure(Recovery)
	}
	afterRaw, _ := json.Marshal(tx.After)
	after, err := s.descriptor(config.Credential{Storage: afterRaw}, tx.Profile)
	if err != nil || after != tx.After {
		return nil, failure(Recovery)
	}
	beforeRaw := tx.Before
	if bytes.Equal(bytes.TrimSpace(beforeRaw), []byte("null")) {
		beforeRaw = nil
	}
	before, beforeErr := s.descriptor(config.Credential{Storage: beforeRaw}, tx.Profile)
	if beforeErr != nil {
		var e *Error
		if !errors.As(beforeErr, &e) || e.Kind != Namespace {
			return nil, failure(Recovery)
		}
	}
	expectedOld := ""
	if beforeErr == nil && !before.Disabled {
		expectedOld = before.Entry
	}
	if tx.OldEntry != expectedOld {
		return nil, failure(Recovery)
	}
	return &tx, nil
}
func sameJSON(a, b []byte) bool {
	if len(a) == 0 || bytes.Equal(a, []byte("null")) {
		a = []byte("null")
	}
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		b = []byte("null")
	}
	var x, y bytes.Buffer
	if json.Compact(&x, a) != nil || json.Compact(&y, b) != nil {
		return false
	}
	return bytes.Equal(x.Bytes(), y.Bytes())
}
func committed(tx *transaction, c config.Credential, exists bool) bool {
	var d descriptor
	_ = json.Unmarshal(c.Storage, &d)
	return (d == tx.After && (d.Backend != "keychain" || c.Token == "")) || (tx.Logout && !exists)
}
func (s *ProfileStore) recoveryState(name string, c config.Credential, exists bool) (string, error) {
	tx, err := s.pending()
	if err != nil {
		return "", err
	}
	if tx == nil || tx.Profile != name {
		return "", nil
	}
	if committed(tx, c, exists) {
		return "cleanup", nil
	}
	if sameJSON(c.Storage, tx.Before) {
		return "rollback", nil
	}
	return "", failure(Recovery)
}
func newRevision() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", failure(IO)
	}
	return hex.EncodeToString(b), nil
}
func resolveBackend(selector string) (string, error) {
	switch selector {
	case "native", "keychain":
		return "keychain", nil
	case "file":
		return "file", nil
	default:
		return "", failure(Selection)
	}
}

// SelectBackend applies the common selection policy. The caller supplies any
// user-config default only for a new profile; existing profiles remain sticky.
func SelectBackend(info Info, selector string) (string, error) {
	if selector == "" {
		if info.Configured {
			return info.Backend, nil
		}
		return "keychain", nil
	}
	selected, err := resolveBackend(selector)
	if err != nil {
		return "", err
	}
	if info.Configured && selected != info.Backend {
		return "", failure(Selection)
	}
	return selected, nil
}
func (s *ProfileStore) Save(ctx context.Context, name string, c config.Credential, selector string) error {
	if strings.TrimSpace(c.Token) == "" {
		return failure(Missing)
	}
	unlock, err := s.locked(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	all, err := s.read()
	if err != nil {
		return err
	}
	old, exists := all.Profiles[name]
	d, err := s.descriptor(old, name)
	if err != nil {
		var e *Error
		// New login in a moved directory creates its own entry without touching the
		// original namespace. Unsupported or malformed descriptors stay protected.
		if !errors.As(err, &e) || e.Kind != Namespace {
			return err
		}
		exists = false
		d = descriptor{Backend: "keychain"}
	}
	if d.Disabled || (old.Storage == nil && old.Token == "") {
		exists = false
	}
	backend, err := SelectBackend(Info{Configured: exists, Backend: d.Backend}, selector)
	if err != nil {
		return err
	}
	return s.change(ctx, all, name, c, backend, false)
}
func (s *ProfileStore) Migrate(ctx context.Context, name, selector string) error {
	backend, err := resolveBackend(selector)
	if err != nil {
		return err
	}
	unlock, err := s.locked(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	all, err := s.read()
	if err != nil {
		return err
	}
	c, exists := all.Profiles[name]
	if !exists {
		return failure(Missing)
	}
	d, err := s.descriptor(c, name)
	if err != nil {
		return err
	}
	if d.Disabled {
		return failure(Missing)
	}
	if tx, err := s.pending(); err != nil {
		return err
	} else if tx != nil {
		return failure(Recovery)
	}
	if d.Backend == backend {
		return nil
	}
	if d.Backend == "keychain" {
		if s.native == nil {
			return failure(Unavailable)
		}
		c.Token, err = s.native.Read(ctx, d.Entry)
		if err != nil {
			return Normalize(err)
		}
	}
	if c.Token == "" {
		return failure(Missing)
	}
	return s.change(ctx, all, name, c, backend, false)
}
func (s *ProfileStore) Delete(ctx context.Context, name string) error {
	unlock, err := s.locked(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	tx, err := s.pending()
	if err != nil {
		return err
	}
	if tx != nil {
		if tx.Profile != name || !tx.Logout {
			return failure(Recovery)
		}
		if err := s.repair(ctx, name); err != nil {
			return err
		}
	}
	all, err := s.read()
	if err != nil {
		return err
	}
	c, ok := all.Profiles[name]
	if !ok {
		return nil
	}
	d, err := s.descriptor(c, name)
	if err != nil {
		return err
	}
	return s.change(ctx, all, name, config.Credential{}, d.Backend, true)
}
func (s *ProfileStore) change(ctx context.Context, all config.Credentials, name string, c config.Credential, backend string, logout bool) error {
	if tx, err := s.pending(); err != nil {
		return err
	} else if tx != nil {
		return failure(Recovery)
	}
	if err := s.files.RemoveStaging(s.path); err != nil {
		return Normalize(err)
	}
	ns, err := s.namespace()
	if err != nil {
		return err
	}
	rev, err := newRevision()
	if err != nil {
		return err
	}
	before := all.Profiles[name]
	old, oldErr := s.descriptor(before, name)
	after := descriptor{Version: 1, Backend: backend, Revision: rev, Disabled: logout}
	if backend == "keychain" {
		after.Namespace = ns
		if !logout {
			after.Entry = ns + "." + digest(name) + "." + rev
		}
	}
	tx := transaction{Version: 1, Profile: name, Namespace: ns, Before: before.Storage, After: after, Logout: logout, NewEntry: after.Entry}
	if oldErr == nil && !old.Disabled {
		tx.OldEntry = old.Entry
	}
	if after.Entry != "" {
		if s.native == nil {
			return failure(Unavailable)
		}
		if err := s.native.Probe(ctx); err != nil {
			return Normalize(err)
		}
	}
	journal, _ := json.Marshal(tx)
	if s.files.Write(s.journalPath(), journal) != nil {
		return failure(Recovery)
	}
	if after.Entry != "" {
		if err := s.native.Write(ctx, after.Entry, c.Token); err != nil {
			return s.abort(ctx, name, err)
		}
		token, err := s.native.Read(ctx, after.Entry)
		if err != nil {
			return s.abort(ctx, name, err)
		}
		if token != c.Token {
			return s.abort(ctx, name, failure(Corrupt))
		}
		c.Token = ""
	}
	c.Storage, _ = json.Marshal(after)
	all.Profiles[name] = c
	if err := s.save(all); err != nil {
		// Write may have returned an error after rename. Preserve recovery evidence;
		// never guess whether a new credential became visible or durable.
		return failure(Recovery)
	}
	return s.repair(ctx, name)
}
func (s *ProfileStore) abort(ctx context.Context, name string, cause error) error {
	if err := s.repair(ctx, name); err != nil {
		return failure(Recovery)
	}
	return Normalize(cause)
}
func (s *ProfileStore) Repair(ctx context.Context, name string) error {
	unlock, err := s.locked(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return s.repair(ctx, name)
}
func (s *ProfileStore) repair(ctx context.Context, name string) error {
	tx, err := s.pending()
	if err != nil {
		return err
	}
	all, err := s.read()
	if err != nil {
		return err
	}
	if tx == nil {
		if _, err := s.descriptor(all.Profiles[name], name); err != nil {
			return err
		}
		return Normalize(s.files.RemoveStaging(s.path))
	}
	if tx.Profile != name {
		return failure(Recovery)
	}
	c, exists := all.Profiles[name]
	done := committed(tx, c, exists)
	if !done && !sameJSON(c.Storage, tx.Before) {
		return failure(Recovery)
	}
	if err := s.files.RemoveStaging(s.path); err != nil {
		return Normalize(err)
	}
	// Recovery may follow a rename whose directory sync failed. Confirm the
	// selected file state durably before deleting either side's native entries.
	if err := s.save(all); err != nil {
		return failure(Recovery)
	}
	remove := tx.NewEntry
	if done {
		remove = tx.OldEntry
	}
	cleanupErr := func() error {
		if done {
			yes := true
			return &Error{Kind: Cleanup, Committed: &yes}
		}
		return failure(Recovery)
	}
	if remove != "" {
		if s.native == nil {
			return cleanupErr()
		}
		if err := s.native.Delete(ctx, remove); err != nil {
			var e *Error
			if !errors.As(err, &e) || e.Kind != Missing {
				return cleanupErr()
			}
		}
	}
	if done && tx.Logout && exists {
		delete(all.Profiles, name)
		if s.save(all) != nil {
			return cleanupErr()
		}
	}
	if s.files.Remove(s.journalPath()) != nil {
		return cleanupErr()
	}
	return nil
}
