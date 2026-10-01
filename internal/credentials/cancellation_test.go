package credentials_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/credentials"
)

type cancellationDisk struct {
	credentials.Disk
	phase  string
	path   string
	cancel context.CancelFunc
}

func (d *cancellationDisk) Lock(ctx context.Context, path string) (func(), error) {
	unlock, err := d.Disk.Lock(ctx, path)
	if err == nil && d.phase == "lock" {
		d.cancel()
	}
	return unlock, err
}

func (d *cancellationDisk) Read(path string) ([]byte, error) {
	data, err := d.Disk.Read(path)
	if err == nil && d.phase == "read" && path == d.path {
		d.cancel()
	}
	return data, err
}

func (d *cancellationDisk) Write(path string, data []byte) error {
	err := d.Disk.Write(path, data)
	if err == nil && d.phase == "journal" && filepath.Base(path) == ".credential-transaction.json" {
		d.cancel()
	}
	return err
}

func TestSaveCancellationBeforeProfilePublicationPreservesCredentials(t *testing.T) {
	for _, phase := range []string{"lock", "read", "journal", "native", "native-cleanup-failure"} {
		t.Run(phase, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "credentials.json")
			disk := &cancellationDisk{path: path}
			native := &fakeSecrets{values: map[string]string{}}
			store := credentials.New(path, native, disk)
			backend := "file"
			if phase == "native" || phase == "native-cleanup-failure" {
				backend = "native"
			}
			for _, name := range []string{"selected", "unrelated"} {
				if err := store.Save(context.Background(), name, config.Credential{Token: "synthetic-existing-" + name}, backend); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			disk.phase, disk.cancel = phase, cancel
			if backend == "native" {
				native.onRead = cancel
			}
			if phase == "native-cleanup-failure" {
				native.deleteErr = &credentials.Error{Kind: credentials.Denied}
			}
			err = store.Save(ctx, "selected", config.Credential{Token: "synthetic-candidate"}, backend)
			if phase == "native-cleanup-failure" {
				var recovery *credentials.Error
				if !errors.As(err, &recovery) || recovery.Kind != credentials.Recovery {
					t.Fatal("cancelled staging cleanup must retain explicit recovery")
				}
			} else if !errors.Is(err, context.Canceled) {
				t.Fatal("cancelled preparation did not report cancellation")
			}
			after, readErr := os.ReadFile(path)
			if readErr != nil || !bytes.Equal(before, after) {
				t.Fatal("cancelled preparation changed stored credentials")
			}
			for _, name := range []string{"selected", "unrelated"} {
				credential, err := store.Load(context.Background(), name)
				if err != nil || credential.Token != "synthetic-existing-"+name {
					t.Fatal("cancelled preparation replaced an active credential")
				}
			}
			if phase == "native-cleanup-failure" {
				info, err := store.Inspect(context.Background(), "selected")
				if err != nil || info.Recovery != "rollback" {
					t.Fatal("pending rollback cleanup was not retained")
				}
				native.deleteErr = nil
				if err := store.Repair(context.Background(), "selected"); err != nil {
					t.Fatal("cancelled staging cleanup could not be retried")
				}
			}
			if backend == "native" && len(native.values) != 2 {
				t.Fatal("unselected candidate native entry was not cleaned up")
			}
		})
	}
}
