package credentials_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/credentials"
)

func TestCredentialAliasesDoNotRetainPlaintext(t *testing.T) {
	inputs := map[string]string{
		"canonical":         `{"profiles":{"work":{"token":"alias-fixture"}}}`,
		"profiles_alias":    `{"Profiles":{"work":{"token":"alias-fixture"}}}`,
		"token_alias":       `{"profiles":{"work":{"Token":"alias-fixture"}}}`,
		"unicode_aliases":   `{"profileſ":{"work":{"toKen":"alias-fixture"}}}`,
		"escaped_aliases":   `{"\u0050rofiles":{"work":{"To\u006ben":"alias-fixture"}}}`,
		"duplicate_aliases": `{"Profiles":{"work":{"token":"stale-fixture"}},"profiles":{"work":{"Token":"stale-fixture","token":"alias-fixture"}}}`,
	}
	for name, input := range inputs {
		for _, operation := range []string{"migrate", "replace", "logout"} {
			t.Run(name+"/"+operation, func(t *testing.T) {
				ctx := context.Background()
				path := filepath.Join(t.TempDir(), "credentials.json")
				if err := os.WriteFile(path, []byte(input), 0600); err != nil {
					t.Fatal(err)
				}
				native := &fakeSecrets{values: map[string]string{}}
				s := credentials.New(path, native, nil)
				before, err := s.Load(ctx, "work")
				if err != nil || before.Token != "alias-fixture" {
					t.Fatal("legacy credential was not readable")
				}
				switch operation {
				case "migrate":
					err = s.Migrate(ctx, "work", "native")
				case "replace":
					err = s.Save(ctx, "work", config.Credential{Token: "replacement-fixture"}, "file")
				case "logout":
					err = s.Delete(ctx, "work")
				}
				if err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if bytes.Contains(data, []byte("alias-fixture")) || bytes.Contains(data, []byte("stale-fixture")) {
					t.Fatal("credential file retained an obsolete plaintext token")
				}
				// A new store instance must observe the committed state without repair.
				s = credentials.New(path, native, nil)
				info, err := s.Inspect(ctx, "work")
				if err != nil || info.Recovery != "" {
					t.Fatal("credential change left recovery pending")
				}
				got, err := s.Load(ctx, "work")
				if err != nil {
					t.Fatal(err)
				}
				switch operation {
				case "migrate":
					if !info.Configured || info.Backend != "keychain" || got.Token != before.Token || len(native.values) != 1 {
						t.Fatal("native migration did not preserve the usable credential")
					}
					if err := s.Migrate(ctx, "work", "file"); err != nil {
						t.Fatal(err)
					}
					got, err = s.Load(ctx, "work")
					if err != nil || got.Token != before.Token || len(native.values) != 0 {
						t.Fatal("explicit reverse migration failed")
					}
				case "replace":
					if !info.Configured || info.Backend != "file" || got.Token != "replacement-fixture" {
						t.Fatal("replacement credential was not selected")
					}
				case "logout":
					names, err := s.List(ctx)
					if err != nil || len(names) != 0 || info.Configured || got.Token != "" {
						t.Fatal("logout resurrected the removed credential profile")
					}
				}
			})
		}
	}
}
