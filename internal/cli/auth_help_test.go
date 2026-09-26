package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/credentials"
)

func TestAuthInspectionAndInvalidArgumentsPreserveCredentials(t *testing.T) {
	for _, command := range []string{"logout", "status"} {
		for _, suffix := range [][]string{{"--help"}, {"-h"}, {"--unknown-option"}, {"unexpected-profile"}} {
			t.Run(command+"/"+strings.Join(suffix, "_"), func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, "config.json")
				credentialPath := config.CredentialsPathFromConfig(path)
				store := credentials.New(credentialPath, nil, nil)
				if err := store.Save(context.Background(), "default", config.Credential{Token: "synthetic-help-token"}, "file"); err != nil {
					t.Fatal(err)
				}
				before, err := os.ReadFile(credentialPath)
				if err != nil {
					t.Fatal(err)
				}
				args := append([]string{"auth", command}, suffix...)
				code, out, errOut := executeAuthorization(t, path, args...)
				want := 2
				if suffix[0] == "--help" || suffix[0] == "-h" {
					want = 0
					if !strings.Contains(out, "Usage:") {
						t.Fatal("help did not show usage")
					}
				}
				if code != want {
					t.Fatalf("exit %d, want %d: %s", code, want, errOut)
				}
				after, err := os.ReadFile(credentialPath)
				if err != nil {
					t.Fatal(err)
				}
				if string(before) != string(after) {
					t.Fatal("informational or invalid invocation changed credentials")
				}
			})
		}
	}
}
