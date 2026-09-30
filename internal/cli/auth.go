package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"
	"unicode"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/authorization"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

var performOAuthLogin = authOAuthLogin
var performOAuthDeviceLogin = authOAuthDeviceLogin
var generateOAuthRandomFn = generateOAuthRandom
var buildOAuthAuthorizationURLFn = buildOAuthAuthorizationURL
var openOAuthBrowserFn = openOAuthBrowser
var waitForOAuthCodeFn = waitForOAuthCode
var exchangeOAuthTokenFn = exchangeOAuthToken

func authCommand(ctx *Context, args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		printAuthHelp(ctx.Stdout)
		return nil
	}
	if args[0] == "help" {
		if len(args) > 1 && (args[1] == "migrate" || args[1] == "repair") {
			printAuthStorageHelp(ctx.Stdout, args[1])
			return nil
		}
		if len(args) > 1 && args[1] == "login" {
			printAuthLoginHelp(ctx.Stdout)
			return nil
		}
		printAuthHelp(ctx.Stdout)
		return nil
	}
	switch args[0] {
	case "login":
		return authLogin(ctx, args[1:])
	case "migrate", "repair":
		return authStorageCommand(ctx, args[0], args[1:])
	case "status", "logout":
		fs := newFlagSet("auth " + args[0])
		var help bool
		bindHelpFlag(fs, &help)
		if err := parseFlagSetInterspersed(fs, args[1:]); err != nil {
			return &CodeError{Code: exitUsage, Err: err}
		}
		if help {
			printAuthHelp(ctx.Stdout)
			return nil
		}
		if len(fs.Args()) != 0 {
			return &CodeError{Code: exitUsage, Err: fmt.Errorf("auth %s accepts no positional arguments; select a profile with --profile", args[0])}
		}
		if args[0] == "status" {
			return authStatus(ctx)
		}
		return authLogout(ctx)
	default:
		return unknownCommand("auth", args[0])
	}
}

func authLogin(ctx *Context, args []string) error {
	fs := newFlagSet("auth login")
	var backend string
	var tokenStdin bool
	var readOnly bool
	var printEnv bool
	var oauth bool
	var oauthDevice bool
	var noBrowser bool
	var clientID string
	var authorizeURL string
	var tokenURL string
	var deviceURL string
	var oauthListen string
	var redirectURI string
	var help bool
	fs.StringVar(&backend, "credential-store", "", "Storage for a new profile: native or file")
	fs.BoolVar(&readOnly, "read-only", false, "Request read-only OAuth access")
	fs.BoolVar(&tokenStdin, "token-stdin", false, "Read token from stdin")
	fs.BoolVar(&printEnv, "print-env", false, "Print export command instead of saving")
	fs.BoolVar(&oauth, "oauth", false, "Authenticate via OAuth PKCE flow")
	fs.BoolVar(&oauthDevice, "oauth-device", false, "Authenticate via OAuth device flow")
	fs.BoolVar(&noBrowser, "no-browser", false, "Do not auto-open browser for OAuth flow")
	fs.StringVar(&clientID, "client-id", "", "OAuth client ID")
	fs.StringVar(&authorizeURL, "oauth-authorize-url", "", "OAuth authorize URL")
	fs.StringVar(&tokenURL, "oauth-token-url", "", "OAuth token URL")
	fs.StringVar(&deviceURL, "oauth-device-url", "", "OAuth device code URL")
	fs.StringVar(&oauthListen, "oauth-listen", "", "OAuth callback listen address (host:port)")
	fs.StringVar(&redirectURI, "oauth-redirect-uri", "", "OAuth redirect URI")
	bindHelpFlag(fs, &help)
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printAuthLoginHelp(ctx.Stdout)
		return nil
	}
	if len(fs.Args()) != 0 {
		return &CodeError{Code: exitUsage, Err: errors.New("auth login accepts no positional arguments; use --token-stdin for a manual token")}
	}
	if readOnly && !oauth && !oauthDevice {
		return &CodeError{Code: exitUsage, Err: errors.New("--read-only requires --oauth or --oauth-device")}
	}
	if oauth && oauthDevice {
		return &CodeError{Code: exitUsage, Err: errors.New("--oauth and --oauth-device are mutually exclusive")}
	}
	var cfg oauthConfig
	if oauth || oauthDevice {
		if tokenStdin {
			flag := "--oauth"
			if oauthDevice {
				flag = "--oauth-device"
			}
			return &CodeError{Code: exitUsage, Err: fmt.Errorf("--token-stdin cannot be used with %s", flag)}
		}
		var err error
		cfg, err = buildOAuthConfig(clientID, authorizeURL, tokenURL, deviceURL, redirectURI, oauthListen, noBrowser || oauthDevice)
		if err != nil {
			return &CodeError{Code: exitUsage, Err: err}
		}
		cfg.ReadOnly = readOnly
		if oauthDevice && cfg.DeviceURL == "" {
			return &CodeError{Code: exitUsage, Err: errors.New("Todoist does not advertise OAuth device authorization. Use --oauth with a public PKCE client or manual auth login. --oauth-device requires an explicitly configured provider device endpoint; live Todoist support is unverified")}
		}
	}
	if !printEnv {
		selected, err := loginBackend(ctx, backend)
		if err != nil {
			return err
		}
		ctx.SavingBackend = selected
	}
	if oauth || oauthDevice {
		previous := ctx.OperationContext
		operation, stop := signal.NotifyContext(operationContext(ctx), os.Interrupt)
		defer stop()
		ctx.OperationContext = operation
		defer func() { ctx.OperationContext = previous }()
		login := performOAuthLogin
		if oauthDevice {
			login = performOAuthDeviceLogin
		}
		token, err := login(ctx, cfg)
		if err != nil {
			return err
		}
		if err := operation.Err(); err != nil {
			return oauthContextError(err)
		}
		if printEnv {
			return writeAuthPrintEnv(ctx, token.AccessToken)
		}
		err = storeProfileCredential(ctx, token.AccessToken, token.Authorization)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return oauthContextError(err)
		}
		return err
	}
	var token string
	if tokenStdin {
		val, err := readAllTrim(ctx.Stdin)
		if err != nil {
			return err
		}
		token = val
	} else {
		if ctx.Global.NoInput {
			return &CodeError{Code: exitUsage, Err: errors.New("token required; use --token-stdin or disable --no-input")}
		}
		if !isTTYReader(ctx.Stdin) {
			return &CodeError{Code: exitUsage, Err: errors.New("stdin is not a TTY; use --token-stdin")}
		}
		fmt.Fprintln(ctx.Stderr, "Copy your API token from Todoist settings. Paste only the token, without quotes or a Bearer prefix. Input is hidden.")
		val, err := readSecret(ctx.Stdin.(*os.File), ctx.Stderr, "Todoist API token: ")
		if err != nil {
			return err
		}
		token = strings.TrimSpace(val)
	}
	if err := validateManualLoginToken(ctx, token); err != nil {
		return err
	}
	if printEnv {
		return writeAuthPrintEnv(ctx, token)
	}
	return storeProfileToken(ctx, token)
}

// Validate the candidate, not an environment override or the previously saved
// profile. Never include provider responses: they may echo the candidate secret.
func validateManualLoginToken(ctx *Context, token string) error {
	if token == "" || strings.ContainsAny(token, "\"'") || strings.HasPrefix(token, "TODOIST_TOKEN=") || strings.ContainsFunc(token, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return &CodeError{Code: exitUsage, Err: errInvalidManualToken}
	}
	timeout := time.Duration(ctx.Config.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	client := api.NewClient(ctx.Config.BaseURL, token, timeout, authorization.Resolve(nil, "env", true))
	req, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var page api.Paginated[api.Project]
	_, err := client.Get(req, "/projects", url.Values{"limit": {"1"}}, &page)
	if err == nil {
		return nil
	}
	var apiErr *api.APIError
	if errors.As(err, &apiErr) && (apiErr.Status == 401 || apiErr.Status == 403) {
		return &CodeError{Code: exitAuth, Err: errRejectedManualToken}
	}
	return &CodeError{Code: exitError, Err: errors.New("Could not verify the API token. Check your connection and API endpoint, then run `todoist auth login` again. Nothing was saved; existing credentials are unchanged.")}
}

func authOAuthLogin(ctx *Context, cfg oauthConfig) (oauthToken, error) {
	if err := operationContext(ctx).Err(); err != nil {
		return oauthToken{}, oauthContextError(err)
	}
	verifier, err := generateOAuthRandomFn(32)
	if err != nil {
		return oauthToken{}, err
	}
	state, err := generateOAuthRandomFn(16)
	if err != nil {
		return oauthToken{}, err
	}
	cfg, err = prepareOAuthCallback(cfg, state)
	if err != nil {
		return oauthToken{}, err
	}
	defer cfg.callback.close()
	authURL, err := buildOAuthAuthorizationURLFn(cfg, oauthCodeChallenge(verifier), state)
	if err != nil {
		return oauthToken{}, err
	}
	fmt.Fprintf(ctx.Stderr, "OAuth authorization URL:\n%s\n", authURL)
	if !cfg.NoBrowser {
		if err := openOAuthBrowserFn(authURL); err != nil {
			fmt.Fprintln(ctx.Stderr, "warning: could not open browser automatically.")
			fmt.Fprintln(ctx.Stderr, "Open the OAuth authorization URL manually to continue.")
		}
	}
	code, err := waitForOAuthCodeFn(operationContext(ctx), cfg, state, 3*time.Minute)
	if err != nil {
		return oauthToken{}, err
	}
	reqCtx, cancel := requestContext(ctx)
	defer cancel()
	token, err := exchangeOAuthTokenFn(reqCtx, cfg, code, verifier)
	if err != nil {
		return oauthToken{}, err
	}
	return token, nil
}

func authOAuthDeviceLogin(ctx *Context, cfg oauthConfig) (oauthToken, error) {
	reqCtx, cancel := requestContext(ctx)
	defer cancel()
	deviceCode, userCode, verifyURL, verifyURLComplete, intervalSec, expiresInSec, err := startOAuthDeviceFlow(reqCtx, cfg)
	cancel()
	if err != nil {
		return oauthToken{}, err
	}
	fmt.Fprintln(ctx.Stderr, "OAuth device flow started.")
	if verifyURLComplete != "" {
		fmt.Fprintf(ctx.Stderr, "Open this URL and approve access:\n%s\n", verifyURLComplete)
	} else {
		fmt.Fprintf(ctx.Stderr, "Open: %s\n", verifyURL)
		fmt.Fprintf(ctx.Stderr, "Code: %s\n", userCode)
	}
	fmt.Fprintln(ctx.Stderr, "Waiting for approval...")
	cfg.RequestTimeout = time.Duration(ctx.Config.TimeoutSeconds) * time.Second
	token, err := pollOAuthDeviceToken(operationContext(ctx), cfg, deviceCode, intervalSec, expiresInSec)
	if err != nil {
		return oauthToken{}, err
	}
	return token, nil
}

func storeProfileToken(ctx *Context, token string) error {
	return storeProfileCredential(ctx, token, authorization.ManualMetadata())
}

func storeProfileCredential(ctx *Context, token string, metadata authorization.Metadata) error {
	raw, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	report := authorization.Resolve(raw, "credentials", true)
	req, cancel := requestContext(ctx)
	defer cancel()
	if err := req.Err(); err != nil {
		return err
	}
	if err := profileStore(ctx).Save(req, ctx.Profile, config.Credential{Token: token, Authorization: raw}, ctx.SavingBackend); err != nil {
		return err
	}
	info, err := profileStore(ctx).Inspect(req, ctx.Profile)
	if err != nil {
		return err
	}

	payload := map[string]any{
		"profile":                  ctx.Profile,
		"stored":                   true,
		"backend":                  info.Backend,
		"authorization":            report,
		"environment_token_active": os.Getenv("TODOIST_TOKEN") != "",
	}
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, payload, output.Meta{})
	}
	if ctx.Mode == output.ModeNDJSON {
		return output.WriteNDJSON(ctx.Stdout, []any{payload})
	}
	storage := "the credential file"
	if info.Backend == "keychain" {
		storage = "macOS Keychain"
	}
	fmt.Fprintf(ctx.Stdout, "Connected to Todoist. Token saved in %s for profile %q.\n", storage, ctx.Profile)
	fmt.Fprintf(ctx.Stdout, "Run `%s` to see your tasks.\n", credentialCommand(ctx, ctx.Profile, "today"))
	if os.Getenv("TODOIST_TOKEN") != "" {
		fmt.Fprintln(ctx.Stderr, "TODOIST_TOKEN still overrides the stored profile.")
	}
	return nil
}

func writeAuthPrintEnv(ctx *Context, token string) error {
	exportLine := fmt.Sprintf("export TODOIST_TOKEN=%s", shellEscape(token))
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, map[string]any{
			"profile": ctx.Profile,
			"env_var": "TODOIST_TOKEN",
			"export":  exportLine,
		}, output.Meta{})
	}
	if ctx.Mode == output.ModeNDJSON {
		return output.WriteNDJSON(ctx.Stdout, []any{
			map[string]any{
				"profile": ctx.Profile,
				"env_var": "TODOIST_TOKEN",
				"export":  exportLine,
			},
		})
	}
	fmt.Fprintln(ctx.Stdout, exportLine)
	return nil
}

func authStatus(ctx *Context) error {
	if ctx.CredentialErr != nil {
		return ctx.CredentialErr
	}
	report := currentAuthorization(ctx)
	source := ctx.TokenSource
	configured := configuredCredential(ctx)
	if source == "" && configured {
		source = "unknown"
	}
	if ctx.Mode == output.ModeJSON || ctx.Mode == output.ModeNDJSON {
		payload := map[string]any{"profile": ctx.Profile, "configured": configured, "source": source, "authorization": report, "backend": credentialBackend(ctx), "accessibility": "unchecked", "recovery": ctx.CredentialInfo.Recovery}
		var err error
		if ctx.Mode == output.ModeNDJSON {
			err = output.WriteNDJSON(ctx.Stdout, []any{payload})
		} else {
			err = output.WriteJSON(ctx.Stdout, payload, output.Meta{})
		}
		if err != nil {
			return err
		}
	} else if configured {
		fmt.Fprintf(ctx.Stdout, "profile %q token source: %s; backend: %s; accessibility unchecked; %s\n", ctx.Profile, source, credentialBackend(ctx), report.Summary())
	} else {
		fmt.Fprintf(ctx.Stdout, "profile %q has no token configured\n", ctx.Profile)
	}
	if ctx.CredentialInfo.Recovery != "" && ctx.Mode != output.ModeJSON && ctx.Mode != output.ModeNDJSON {
		fmt.Fprintf(ctx.Stdout, "credential recovery: %s; run todoist auth repair\n", ctx.CredentialInfo.Recovery)
	}
	return report.CheckCredential()
}

func authLogout(ctx *Context) error {
	req, cancel := requestContext(ctx)
	defer cancel()
	if err := profileStore(ctx).Delete(req, ctx.Profile); err != nil {
		return err
	}
	payload := map[string]any{
		"profile":                  ctx.Profile,
		"removed":                  true,
		"environment_token_active": os.Getenv("TODOIST_TOKEN") != "",
	}
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, payload, output.Meta{})
	}
	if ctx.Mode == output.ModeNDJSON {
		return output.WriteNDJSON(ctx.Stdout, []any{payload})
	}
	fmt.Fprintf(ctx.Stdout, "removed token for profile %q\n", ctx.Profile)
	if os.Getenv("TODOIST_TOKEN") != "" {
		fmt.Fprintln(ctx.Stderr, "TODOIST_TOKEN remains active; logout only removes stored credentials.")
	}
	return nil
}

func currentAuthorization(ctx *Context) authorization.Report {
	if ctx.Authorization != nil {
		return *ctx.Authorization
	}
	return authorization.Resolve(nil, ctx.TokenSource, ctx.Token != "")
}
