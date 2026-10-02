package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/credentials"
)

// Identity supports human advice without changing existing machine error text.
var (
	errMissingTaskViewRef  = errors.New("task view requires id or text reference")
	errMissingToken        = errors.New("missing auth token; run 'todoist auth login' or set TODOIST_TOKEN")
	errInvalidManualToken  = errors.New("Invalid API token format. Paste only the API token from Todoist settings, without spaces, quotes, or a Bearer prefix. Nothing was saved; run `todoist auth login` to retry.")
	errRejectedManualToken = errors.New("API token was not accepted by Todoist. Copy your API token from Todoist settings and run `todoist auth login` again. Nothing was saved; existing credentials are unchanged.")
)

const credentialSelectionHint = "Keep the same --config, --profile and --base-url selections. TODOIST_TOKEN overrides stored credentials; login does not replace that environment value."

func credentialCommand(ctx *Context, profile string, args ...string) string {
	words := []string{"todoist"}
	if ctx.ConfigPath != "" {
		path, err := filepath.Abs(ctx.ConfigPath)
		if err != nil {
			path = ctx.ConfigPath
		}
		words = append(words, "--config", path)
	}
	if profile != "" {
		words = append(words, "--profile", profile)
	}
	// Local recovery needs no API endpoint. Remote next commands retain explicit
	// endpoint selection rather than relying on unrelated defaults.
	if ctx.Global.BaseURL != "" && len(args) > 0 && args[0] == "today" {
		words = append(words, "--base-url", ctx.Global.BaseURL)
	}
	return strings.Join(escapeArgs(append(words, args...)), " ")
}

func writeRecoveryHints(ctx *Context, err error) {
	if !humanCommandHints(ctx.Global) {
		return
	}
	var storage *credentials.Error
	var remote *api.APIError
	var usage *CodeError
	var target *profileStorageError
	switch {
	case errors.As(err, &target) && errors.As(err, &storage) && (storage.Kind == credentials.Cleanup || storage.Kind == credentials.Recovery):
		fmt.Fprintf(ctx.Stderr, "Retry the named profile: %s\n", credentialCommand(ctx, "", "profile", "remove", target.Profile))
		fmt.Fprintf(ctx.Stderr, "Repair that profile: %s\n", credentialCommand(ctx, target.Profile, "auth", "repair"))
	case errors.Is(err, errMissingToken), errors.Is(err, errInvalidManualToken), errors.Is(err, errRejectedManualToken):
		if errors.Is(err, errMissingToken) {
			fmt.Fprintln(ctx.Stderr, "The authenticated operation did not start.")
		}
		fmt.Fprintln(ctx.Stderr, credentialSelectionHint)
		fmt.Fprintln(ctx.Stderr, "For noninteractive login, supply a token file: todoist auth login --no-input --token-stdin < token.txt")
		fmt.Fprintln(ctx.Stderr, "Protect token.txt as a secret. For a new profile requiring portable plaintext storage, explicitly add --credential-store=file.")
	case errors.As(err, &storage) && (storage.Kind == credentials.Unavailable || storage.Kind == credentials.Missing):
		fmt.Fprintln(ctx.Stderr, credentialSelectionHint)
		fmt.Fprintln(ctx.Stderr, "Inspect the selected profile without retrieving its secret: todoist auth status --no-input")
		if storage.Kind == credentials.Unavailable {
			fmt.Fprintln(ctx.Stderr, "Use a build with native storage support for an existing native profile. File login does not migrate it; migration requires access to the original credential.")
			fmt.Fprintln(ctx.Stderr, "Only for a new or file-backed profile: todoist auth login --credential-store=file --no-input --token-stdin < token.txt (plaintext storage; protect token.txt).")
		} else {
			fmt.Fprintln(ctx.Stderr, "Supply a replacement token: todoist auth login --no-input --token-stdin < token.txt (protect this secret file).")
		}
	case errors.As(err, &remote) && remote.Status == 401:
		fmt.Fprintln(ctx.Stderr, "The request was rejected as unauthenticated. Earlier actions may have succeeded; inspect their results before resubmitting mutations.")
		fmt.Fprintln(ctx.Stderr, credentialSelectionHint)
		fmt.Fprintln(ctx.Stderr, "Inspect credential selection: todoist auth status --no-input (offline; does not verify token validity).")
		fmt.Fprintln(ctx.Stderr, "Replace an active TODOIST_TOKEN deliberately, or unset it to use a stored profile. For stored login: todoist auth login --no-input --token-stdin < token.txt (protect this secret file).")
	case errors.As(err, &usage) && usage.Code == exitUsage && ctx.HelpPath != "":
		if errors.Is(err, errMissingTaskViewRef) {
			fmt.Fprintln(ctx.Stderr, "Example: todoist task view id:<id> (replace <id> with a task ID)")
		}
		fmt.Fprintf(ctx.Stderr, "See: todoist %s --help\n", ctx.HelpPath)
	}
}

func writeReviewRecovery(ctx *Context, report reviewReport) {
	if !humanCommandHints(ctx.Global) {
		return
	}
	uncertain := false
	for _, task := range report.Tasks {
		for _, action := range task.Actions {
			if !action.RemoteOutcomeUncertain {
				continue
			}
			if !uncertain {
				fmt.Fprintln(ctx.Stderr, "A Todoist mutation may have happened. Do not reapply this plan or force resubmission. Keep the saved review plan and replay evidence.")
				fmt.Fprintln(ctx.Stderr, "Use the same --config, --profile, credential source (including TODOIST_TOKEN), and --base-url for inspection.")
				uncertain = true
			}
			// Quote the reference as data, never as an executable shell fragment. IDs
			// containing shell metacharacters must be copied as a literal argument.
			fmt.Fprintf(ctx.Stderr, "Inspect with todoist task view --full --no-input and this literal reference: %q\n", "id:"+task.ID)
			break
		}
	}
	if uncertain {
		fmt.Fprintln(ctx.Stderr, "Compare current fields with the saved plan and reported applied actions. A missing task or an advanced recurring due date is not proof of completion; verify the exact task in Todoist's completed history/activity.")
		fmt.Fprintln(ctx.Stderr, "Only after manual reconciliation, start a fresh review for remaining changes. There is no automatic reconciliation command; preserve the old plan and replay evidence. --no-input cannot start an interactive review.")
	}
}
