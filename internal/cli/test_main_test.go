package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/credentials"
)

// CLI tests never open the developer's native store or default credential file.
// Tests needing native behavior inject a fake; real integration is separately opt-in.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "todoist-cli-tests-")
	if err != nil {
		os.Exit(1)
	}
	os.Setenv("XDG_CONFIG_HOME", dir)
	os.Setenv("TODOIST_CONFIG", filepath.Join(dir, "config.json"))
	os.Setenv("TODOIST_TOKEN", "")
	os.Setenv("TODOIST_PROFILE", "")
	newCredentialStore = func(path string) credentials.Store {
		return credentials.New(config.CredentialsPathFromConfig(path), &cliSecrets{inaccessible: true}, nil)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
