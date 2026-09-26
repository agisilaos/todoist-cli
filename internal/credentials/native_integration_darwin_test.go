//go:build darwin && cgo && credentialintegration

package credentials

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestIsolatedKeychain(t *testing.T) {
	if os.Getenv("TODOIST_TEST_KEYCHAIN") != "1" {
		t.Skip("set TODOIST_TEST_KEYCHAIN=1 to opt into a disposable keychain")
	}
	path := filepath.Join(t.TempDir(), "todoist-integration.keychain")
	password, err := newRevision()
	if err != nil {
		t.Fatal("generate test password failed")
	}
	if err := isolatedKeychain(path, password, 0); err != nil {
		t.Fatalf("isolated keychain creation failed: %v", err)
	}
	defer func() {
		if err := isolatedKeychain(path, password, 3); err != nil {
			t.Errorf("isolated cleanup failed: %v", err)
		}
	}()
	k := &keychain{path: path}
	ctx := context.Background()
	if err := k.Write(ctx, "fixture-entry", "synthetic-native-token"); err != nil {
		t.Fatal(err)
	}
	got, err := k.Read(ctx, "fixture-entry")
	if err != nil || got != "synthetic-native-token" {
		t.Fatal("native read failed")
	}
	// Replacements use fresh generations, never mutate the active entry in place.
	if err := k.Write(ctx, "replacement-entry", "synthetic-replacement"); err != nil {
		t.Fatal(err)
	}
	if err := k.Delete(ctx, "fixture-entry"); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Read(ctx, "fixture-entry"); err == nil {
		t.Fatal("deleted native entry remains readable")
	}
	if err := isolatedKeychain(path, password, 1); err != nil {
		t.Fatal(err)
	}
	_, err = k.Read(ctx, "replacement-entry")
	var classified *Error
	if !errors.As(err, &classified) || classified.Kind != Locked {
		t.Fatal("locked keychain did not fail without prompting")
	}
	if err := isolatedKeychain(path, password, 2); err != nil {
		t.Fatal(err)
	}
	if err := k.Delete(ctx, "replacement-entry"); err != nil {
		t.Fatal(err)
	}
}
