package cli

import (
	"context"
	"github.com/agisilaos/todoist-cli/internal/credentials"
)

// Local adapters belong to an invocation, including configuration persistence.
type localDependencies struct {
	credentialStore         func(string) credentials.Store
	persistProfileSelection func(context.Context, string, string) error
}

func (ctx *Context) localDeps() localDependencies {
	d := ctx.local
	if d.credentialStore == nil {
		d.credentialStore = newCredentialStore
	}
	if d.persistProfileSelection == nil {
		d.persistProfileSelection = persistProfileSelection
	}
	return d
}
