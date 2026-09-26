package credentials_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/credentials"
)

func TestFileProfileRoundTripAndInspection(t *testing.T) {
	s := credentials.New(filepath.Join(t.TempDir(), "credentials.json"), nil, nil)
	ctx := context.Background()
	if err := s.Save(ctx, "work", config.Credential{Token: "fixture-secret"}, "file"); err != nil {
		t.Fatal(err)
	}
	info, err := s.Inspect(ctx, "work")
	if err != nil || !info.Configured || info.Backend != "file" {
		t.Fatal("profile inspection failed")
	}
	cred, err := s.Load(ctx, "work")
	if err != nil || cred.Token != "fixture-secret" {
		t.Fatal("credential round trip failed")
	}
	names, err := s.List(ctx)
	if err != nil || len(names) != 1 || names[0] != "work" {
		t.Fatal("enumeration failed")
	}
	if err := s.Delete(ctx, "work"); err != nil {
		t.Fatal(err)
	}
	info, err = s.Inspect(ctx, "work")
	if err != nil || info.Configured {
		t.Fatal("logout did not remove profile")
	}
}

type fakeSecrets struct {
	onRead                       func()
	values                       map[string]string
	writeErr, readErr, deleteErr error
}

func (f *fakeSecrets) Probe(context.Context) error { return nil }
func (f *fakeSecrets) Read(_ context.Context, id string) (string, error) {
	if f.onRead != nil {
		action := f.onRead
		f.onRead = nil
		action()
	}
	if f.readErr != nil {
		return "", f.readErr
	}
	v, ok := f.values[id]
	if !ok {
		return "", &credentials.Error{Kind: credentials.Missing}
	}
	return v, nil
}
func (f *fakeSecrets) Write(_ context.Context, id, token string) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.values[id] = token
	return nil
}
func (f *fakeSecrets) Delete(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.values, id)
	return nil
}
func TestMigrationVerifiesNativeTokenAndPreservesMetadata(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "credentials.json")
	f := &fakeSecrets{values: map[string]string{}}
	s := credentials.New(path, f, nil)
	original := config.Credential{Token: "migration-fixture", Authorization: []byte(`{"version":1,"future":"preserved"}`)}
	if err := s.Save(ctx, "work", original, "file"); err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(ctx, "work", "native"); err != nil {
		t.Fatal(err)
	}
	info, err := s.Inspect(ctx, "work")
	if err != nil || info.Backend != "keychain" || !info.Configured {
		t.Fatal("native profile not selected")
	}
	got, err := s.Load(ctx, "work")
	if err != nil || got.Token != original.Token || !equalJSON(got.Authorization, original.Authorization) {
		t.Fatal("migration lost credential or evidence")
	}
	disk, _, err := config.LoadCredentials(path)
	if err != nil || disk.Profiles["work"].Token != "" {
		t.Fatal("migration retained plaintext")
	}
}

func equalJSON(a, b []byte) bool {
	var x, y bytes.Buffer
	_ = json.Compact(&x, a)
	_ = json.Compact(&y, b)
	return bytes.Equal(x.Bytes(), y.Bytes())
}

func TestFailedVerificationPreservesFileCredentialAndCleansStagedSecret(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "credentials.json")
	f := &fakeSecrets{values: map[string]string{}}
	s := credentials.New(path, f, nil)
	if err := s.Save(ctx, "work", config.Credential{Token: "original"}, "file"); err != nil {
		t.Fatal(err)
	}
	f.readErr = &credentials.Error{Kind: credentials.Denied}
	if err := s.Migrate(ctx, "work", "native"); err == nil {
		t.Fatal("migration should fail")
	}
	got, err := s.Load(ctx, "work")
	if err != nil || got.Token != "original" || len(f.values) != 0 {
		t.Fatal("failed migration lost source or leaked staging entry")
	}
}

func TestReplacementAndLogoutRemainCommittedWhenCleanupFails(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "credentials.json")
	f := &fakeSecrets{values: map[string]string{}}
	s := credentials.New(path, f, nil)
	if err := s.Save(ctx, "work", config.Credential{Token: "original"}, "native"); err != nil {
		t.Fatal(err)
	}
	f.deleteErr = &credentials.Error{Kind: credentials.Interaction}
	err := s.Save(ctx, "work", config.Credential{Token: "replacement"}, "")
	var failure *credentials.Error
	if !errors.As(err, &failure) || failure.Kind != credentials.Cleanup || failure.Committed == nil || !*failure.Committed {
		t.Fatal("partial replacement not explicit")
	}
	got, err := s.Load(ctx, "work")
	if err != nil || got.Token != "replacement" {
		t.Fatal("replacement rolled back after commit")
	}
	f.deleteErr = nil
	if err := s.Repair(ctx, "work"); err != nil {
		t.Fatal(err)
	}
	if len(f.values) != 1 {
		t.Fatal("repair did not remove obsolete entry")
	}
	f.deleteErr = &credentials.Error{Kind: credentials.Interaction}
	if err := s.Delete(ctx, "work"); !errors.As(err, &failure) || failure.Kind != credentials.Cleanup {
		t.Fatal("logout should report pending cleanup")
	}
	info, err := s.Inspect(ctx, "work")
	if err != nil || info.Configured {
		t.Fatal("logout failed to disable profile")
	}
	got, err = s.Load(ctx, "work")
	if err != nil || got.Token != "" {
		t.Fatal("disabled profile still authenticates")
	}
	f.deleteErr = nil
	if err := s.Delete(ctx, "work"); err != nil {
		t.Fatal(err)
	}
	if len(f.values) != 0 {
		t.Fatal("repeated logout did not clean up")
	}
}

type failingDisk struct {
	credentials.Disk
	path  string
	after bool
	fail  bool
}

func (d *failingDisk) Write(path string, data []byte) error {
	if d.fail && path == d.path {
		d.fail = false
		if d.after {
			if err := d.Disk.Write(path, data); err != nil {
				return err
			}
		}
		return errors.New("untrusted persistence error contains synthetic-secret")
	}
	return d.Disk.Write(path, data)
}
func TestInterruptedMigrationRecoversOnEitherSideOfCommit(t *testing.T) {
	for _, after := range []bool{false, true} {
		t.Run(fmt.Sprint(after), func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "credentials.json")
			disk := &failingDisk{path: path, after: after}
			f := &fakeSecrets{values: map[string]string{}}
			s := credentials.New(path, f, disk)
			if err := s.Save(ctx, "work", config.Credential{Token: "original-fixture"}, "file"); err != nil {
				t.Fatal(err)
			}
			disk.fail = true
			err := s.Migrate(ctx, "work", "native")
			if err == nil || strings.Contains(err.Error(), "synthetic-secret") {
				t.Fatal("commit failure not safely reported")
			}
			// Reconstruct the store to model a process restart; recovery uses durable state.
			restarted := credentials.New(path, f, nil)
			if err := restarted.Repair(ctx, "work"); err != nil {
				t.Fatal(err)
			}
			got, err := restarted.Load(ctx, "work")
			if err != nil || got.Token != "original-fixture" {
				t.Fatal("restart lost credential")
			}
			info, _ := restarted.Inspect(ctx, "work")
			expected := "file"
			entries := 0
			if after {
				expected = "keychain"
				entries = 1
			}
			if info.Backend != expected || len(f.values) != entries {
				t.Fatal("recovery selected the wrong commit side")
			}
		})
	}
}
func TestFailedRollbackRemainsRepairable(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "credentials.json")
	f := &fakeSecrets{values: map[string]string{}}
	s := credentials.New(path, f, nil)
	if err := s.Save(ctx, "work", config.Credential{Token: "original"}, "file"); err != nil {
		t.Fatal(err)
	}
	f.readErr = &credentials.Error{Kind: credentials.Denied}
	f.deleteErr = &credentials.Error{Kind: credentials.Locked}
	if err := s.Migrate(ctx, "work", "native"); err == nil {
		t.Fatal("rollback failure not reported")
	}
	got, err := s.Load(ctx, "work")
	if err != nil || got.Token != "original" {
		t.Fatal("rollback damaged source")
	}
	f.readErr = nil
	f.deleteErr = nil
	if err := s.Repair(ctx, "work"); err != nil {
		t.Fatal(err)
	}
	if len(f.values) != 0 {
		t.Fatal("rollback repair retained staged entry")
	}
}
func TestCopiedDirectoryCannotUseOrDeleteOriginalNativeEntries(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "credentials.json")
	f := &fakeSecrets{values: map[string]string{}}
	s := credentials.New(path, f, nil)
	if err := s.Save(ctx, "work", config.Credential{Token: "original"}, "native"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(t.TempDir(), "credentials.json")
	if err := os.WriteFile(copyPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	copyStore := credentials.New(copyPath, f, nil)
	if _, err := copyStore.Load(ctx, "work"); err == nil {
		t.Fatal("copied directory read original entry")
	}
	if err := copyStore.Delete(ctx, "work"); err == nil {
		t.Fatal("copied directory removed original entry")
	}
	if err := copyStore.Save(ctx, "work", config.Credential{Token: "copy-token"}, "native"); err != nil {
		t.Fatal(err)
	}
	if len(f.values) != 2 {
		t.Fatal("copied login modified original entries")
	}
	got, err := s.Load(ctx, "work")
	if err != nil || got.Token != "original" {
		t.Fatal("original credential changed")
	}
}
func TestSelectionNeverFallsBackAndExplicitReverseMigration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "credentials.json")
	unavailable := credentials.New(path, nil, nil)
	if err := unavailable.Save(ctx, "work", config.Credential{Token: "new-token"}, ""); err == nil {
		t.Fatal("native default fell back")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("unavailable native login wrote plaintext")
	}
	f := &fakeSecrets{values: map[string]string{}}
	s := credentials.New(path, f, nil)
	if err := s.Save(ctx, "work", config.Credential{Token: "original"}, "native"); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(ctx, "work", config.Credential{Token: "replacement"}, "file"); err == nil {
		t.Fatal("login changed backend")
	}
	if err := s.Migrate(ctx, "work", "file"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load(ctx, "work")
	if err != nil || got.Token != "original" || len(f.values) != 0 {
		t.Fatal("reverse migration failed")
	}
}
func TestStorePreservesInactiveProfilesAndRejectsCorruptActiveStorage(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "credentials.json")
	initial := []byte(`{"future":true,"profiles":{"work":{"token":"old","authorization":null},"other":{"token":"other","future":42,"storage":{"version":99}}}}`)
	if err := os.WriteFile(path, initial, 0600); err != nil {
		t.Fatal(err)
	}
	s := credentials.New(path, nil, nil)
	if err := s.Save(ctx, "work", config.Credential{Token: "replacement"}, "file"); err != nil {
		t.Fatal(err)
	}
	all, _, err := config.LoadCredentials(path)
	if err != nil {
		t.Fatal(err)
	}
	if all.Profiles["other"].Token != "other" {
		t.Fatal("inactive credential changed")
	}
	if _, err := s.Load(ctx, "other"); err == nil {
		t.Fatal("unsupported selected storage was used")
	}
	data, _ := os.ReadFile(path)
	var decoded map[string]any
	if json.Unmarshal(data, &decoded) != nil || decoded["future"] != true {
		t.Fatal("unknown fields lost")
	}
	if err := os.WriteFile(path, []byte(`{"profiles":`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(ctx, "work", config.Credential{Token: "replacement"}, "file"); err == nil {
		t.Fatal("corrupt file overwritten")
	}
}
func TestConcurrentWriterHasBoundedBusyFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	disk := credentials.Disk{}
	unlock, err := disk.Lock(context.Background(), filepath.Join(filepath.Dir(path), ".credentials.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	err = credentials.New(path, nil, nil).Save(ctx, "work", config.Credential{Token: "fixture"}, "file")
	var failure *credentials.Error
	if !errors.As(err, &failure) || failure.Kind != credentials.Busy {
		t.Fatal("writer did not report busy")
	}
}

func TestRetryLogoutAfterFailedPublicationActuallyDisablesProfile(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "credentials.json")
	disk := &failingDisk{path: path}
	f := &fakeSecrets{values: map[string]string{}}
	s := credentials.New(path, f, disk)
	if err := s.Save(ctx, "work", config.Credential{Token: "original"}, "native"); err != nil {
		t.Fatal(err)
	}
	disk.fail = true
	if err := s.Delete(ctx, "work"); err == nil {
		t.Fatal("logout publication should fail")
	}
	if err := s.Delete(ctx, "work"); err != nil {
		t.Fatal(err)
	}
	info, err := s.Inspect(ctx, "work")
	if err != nil || info.Configured || len(f.values) != 0 {
		t.Fatal("retry reported logout without removing credential")
	}
}

func TestRepairRequiresDurableProfileBeforeDeletingOldSecret(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "credentials.json")
	disk := &failingDisk{path: path, after: true}
	f := &fakeSecrets{values: map[string]string{}}
	s := credentials.New(path, f, disk)
	if err := s.Save(ctx, "work", config.Credential{Token: "original"}, "native"); err != nil {
		t.Fatal(err)
	}
	disk.fail = true
	if err := s.Save(ctx, "work", config.Credential{Token: "replacement"}, ""); err == nil {
		t.Fatal("uncertain publication should fail")
	}
	disk.fail = true
	disk.after = false
	if err := s.Repair(ctx, "work"); err == nil {
		t.Fatal("repair did not require durable profile publication")
	}
	if len(f.values) != 2 {
		t.Fatal("old secret deleted before durability confirmation")
	}
	if err := s.Repair(ctx, "work"); err != nil {
		t.Fatal(err)
	}
	if len(f.values) != 1 {
		t.Fatal("durable repair did not finish cleanup")
	}
}

func TestLoadRetriesWhenConcurrentReplacementRemovesOldEntry(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "credentials.json")
	f := &fakeSecrets{values: map[string]string{}}
	s := credentials.New(path, f, nil)
	if err := s.Save(ctx, "work", config.Credential{Token: "original"}, "native"); err != nil {
		t.Fatal(err)
	}
	f.onRead = func() {
		if err := s.Save(ctx, "work", config.Credential{Token: "replacement", Authorization: []byte(`{"version":99}`)}, ""); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Load(ctx, "work")
	if err != nil || got.Token != "replacement" || !equalJSON(got.Authorization, []byte(`{"version":99}`)) {
		t.Fatal("concurrent replacement lost token/evidence pairing")
	}
}

func TestCorruptRecoveryJournalCannotDeleteNativeEntries(t *testing.T) {
	for _, corruption := range []string{"old-entry", "after-version", "before-version", "current-reference"} {
		t.Run(corruption, func(t *testing.T) {
			ctx := context.Background()
			dir := t.TempDir()
			path := filepath.Join(dir, "credentials.json")
			disk := &failingDisk{path: path, after: true}
			f := &fakeSecrets{values: map[string]string{}}
			s := credentials.New(path, f, disk)
			if err := s.Save(ctx, "work", config.Credential{Token: "original"}, "native"); err != nil {
				t.Fatal(err)
			}
			disk.fail = true
			if err := s.Save(ctx, "work", config.Credential{Token: "replacement"}, ""); err == nil {
				t.Fatal("expected uncertain write")
			}
			journalPath := filepath.Join(dir, ".credential-transaction.json")
			raw, _ := os.ReadFile(journalPath)
			var journal map[string]any
			if json.Unmarshal(raw, &journal) != nil {
				t.Fatal("journal missing")
			}
			switch corruption {
			case "old-entry":
				journal["old_entry"] = journal["new_entry"]
			case "after-version":
				journal["after"].(map[string]any)["version"] = 99
			case "before-version":
				journal["before"].(map[string]any)["version"] = 99
			case "current-reference":
				raw, _ := os.ReadFile(path)
				var file map[string]any
				json.Unmarshal(raw, &file)
				file["profiles"].(map[string]any)["work"].(map[string]any)["storage"].(map[string]any)["entry"] = journal["old_entry"]
				raw, _ = json.Marshal(file)
				os.WriteFile(path, raw, 0600)
			}
			raw, _ = json.Marshal(journal)
			if err := os.WriteFile(journalPath, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if err := s.Repair(ctx, "work"); err == nil {
				t.Fatal("corrupt journal accepted")
			}
			if len(f.values) != 2 {
				t.Fatal("corrupt recovery deleted native entry")
			}
			if _, err := s.Inspect(ctx, "work"); err == nil {
				t.Fatal("inspection hid recovery corruption")
			}
		})
	}
}

func TestMigrationAndRepairRemoveAbandonedOwnedPlaintextStaging(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	f := &fakeSecrets{values: map[string]string{}}
	s := credentials.New(path, f, nil)
	if err := s.Save(ctx, "work", config.Credential{Token: "interrupted-plaintext"}, "file"); err != nil {
		t.Fatal(err)
	}
	// A killed process does not execute Disk.Write's deferred temporary removal.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(dir, ".credentials-123456789")
	if err := os.WriteFile(orphan, raw, 0600); err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(dir, ".credentials-user-notes")
	if err := os.WriteFile(unrelated, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(ctx, "work", "native"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatal("native migration retained abandoned plaintext staging")
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Fatal("cleanup touched unrelated file")
	}
	// Repair also clears abandoned staging when a crash happened before the
	// transaction journal itself was published.
	orphan = filepath.Join(dir, ".todoist-credentials-stage-abandoned")
	if err := os.WriteFile(orphan, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Repair(ctx, "work"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatal("repair retained orphan staging")
	}
}

func TestCorruptStatePreservesStagingForDeliberateRecovery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	os.WriteFile(path, []byte(`{"profiles":`), 0600)
	stage := filepath.Join(dir, ".todoist-credentials-stage-recovery")
	os.WriteFile(stage, []byte(`{"profiles":{"default":{"token":"recoverable-fixture"}}}`), 0600)
	s := credentials.New(path, nil, nil)
	if err := s.Repair(context.Background(), "default"); err == nil {
		t.Fatal("corrupt state repair should fail")
	}
	if _, err := os.Stat(stage); err != nil {
		t.Fatal("corrupt-state repair destroyed recovery material")
	}
}
