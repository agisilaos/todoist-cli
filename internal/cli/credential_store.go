package cli

import (
	"errors"
	"fmt"

	"github.com/agisilaos/todoist-cli/internal/authorization"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/credentials"
	"github.com/agisilaos/todoist-cli/internal/output"
)

var newCredentialStore = func(path string) credentials.Store {
	return credentials.New(config.CredentialsPathFromConfig(path), credentials.NewNative(), nil)
}

func profileStore(ctx *Context) credentials.Store {
	if ctx.Credentials == nil {
		ctx.Credentials = newCredentialStore(ctx.ConfigPath)
	}
	return ctx.Credentials
}
func inspectProfile(ctx *Context) {
	info, err := profileStore(ctx).Inspect(operationContext(ctx), ctx.Profile)
	ctx.CredentialInfo = info
	ctx.CredentialErr = err
	if info.Configured {
		ctx.TokenSource = "credentials"
		report := authorization.Resolve(info.Authorization, "credentials", true)
		ctx.Authorization = &report
	}
}
func configuredCredential(ctx *Context) bool { return ctx.Token != "" || ctx.CredentialInfo.Configured }
func resolveStoredToken(ctx *Context) error {
	if ctx.Token != "" {
		return nil
	}
	if ctx.CredentialErr != nil {
		return ctx.CredentialErr
	}
	if !ctx.CredentialInfo.Configured {
		return nil
	}
	req, cancel := requestContext(ctx)
	defer cancel()
	c, err := profileStore(ctx).Load(req, ctx.Profile)
	if err != nil {
		return err
	}
	ctx.Token = c.Token
	report := authorization.Resolve(c.Authorization, "credentials", c.Token != "")
	ctx.Authorization = &report
	return nil
}
func loginBackend(ctx *Context, explicit string) (string, error) {
	if explicit != "" && explicit != "native" && explicit != "file" {
		return "", &CodeError{Code: exitUsage, Err: fmt.Errorf("--credential-store must be native or file")}
	}
	info, err := profileStore(ctx).Inspect(operationContext(ctx), ctx.Profile)
	if err != nil {
		var e *credentials.Error
		if !errors.As(err, &e) || e.Kind != credentials.Namespace {
			return "", err
		}
	}
	requested := explicit
	if requested == "" && !info.Configured {
		requested = ctx.Config.CredentialStore
		if requested != "" && requested != "native" && requested != "file" {
			return "", &CodeError{Code: exitUsage, Err: fmt.Errorf("credential_store must be native or file")}
		}
	}
	selected, err := credentials.SelectBackend(info, requested)
	if err != nil {
		return "", err
	}
	return selected, nil
}
func authStorageCommand(ctx *Context, operation string, args []string) error {
	fs := newFlagSet("auth migrate")
	var target string
	var help bool
	bindHelpFlag(fs, &help)
	if operation == "migrate" {
		fs.StringVar(&target, "credential-store", "", "Destination: native or file")
	}
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printAuthStorageHelp(ctx.Stdout, operation)
		return nil
	}
	if len(fs.Args()) != 0 {
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("auth %s accepts no positional arguments", operation)}
	}
	if operation == "migrate" && target != "native" && target != "file" {
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("auth migrate requires --credential-store=native|file")}
	}
	req, cancel := requestContext(ctx)
	defer cancel()
	var err error
	if operation == "migrate" {
		err = profileStore(ctx).Migrate(req, ctx.Profile, target)
	} else {
		err = profileStore(ctx).Repair(req, ctx.Profile)
	}
	if err != nil {
		return err
	}
	info, err := profileStore(ctx).Inspect(req, ctx.Profile)
	if err != nil {
		return err
	}
	payload := map[string]any{"profile": ctx.Profile, "operation": operation, "completed": true, "backend": info.Backend}
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, payload)
	}
	if ctx.Mode == output.ModeNDJSON {
		return output.WriteNDJSON(ctx.Stdout, []any{payload})
	}
	fmt.Fprintf(ctx.Stdout, "%s completed for profile %q; backend: %s\n", operation, ctx.Profile, info.Backend)
	return nil
}
func storageErrorDetails(err error) map[string]any {
	var e *credentials.Error
	if !errors.As(err, &e) {
		return nil
	}
	details := map[string]any{}
	if e.Committed != nil {
		details["committed"] = *e.Committed
	}
	return details
}
func credentialBackend(ctx *Context) string {
	if ctx.TokenSource == "env" {
		return "environment"
	}
	return ctx.CredentialInfo.Backend
}
