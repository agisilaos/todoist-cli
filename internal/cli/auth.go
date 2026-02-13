package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

var performOAuthLogin = authOAuthLogin
var generateOAuthRandomFn = generateOAuthRandom
var buildOAuthAuthorizationURLFn = buildOAuthAuthorizationURL
var openOAuthBrowserFn = openOAuthBrowser
var waitForOAuthCodeFn = waitForOAuthCode
var exchangeOAuthTokenFn = exchangeOAuthToken

func authCommand(ctx *Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printAuthHelp(ctx.Stdout)
		return nil
	}
	switch args[0] {
	case "login":
		return authLogin(ctx, args[1:])
	case "status":
		return authStatus(ctx)
	case "logout":
		return authLogout(ctx)
	default:
		return &CodeError{Code: exitUsage, Err: fmt.Errorf("unknown auth subcommand: %s", args[0])}
	}
}

func authLogin(ctx *Context, args []string) error {
	fs := flag.NewFlagSet("auth login", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var tokenStdin bool
	var printEnv bool
	var oauth bool
	var noBrowser bool
	var clientID string
	var authorizeURL string
	var tokenURL string
	var oauthListen string
	var redirectURI string
	var help bool
	fs.BoolVar(&tokenStdin, "token-stdin", false, "Read token from stdin")
	fs.BoolVar(&printEnv, "print-env", false, "Print export command instead of saving")
	fs.BoolVar(&oauth, "oauth", false, "Authenticate via OAuth PKCE flow")
	fs.BoolVar(&noBrowser, "no-browser", false, "Do not auto-open browser for OAuth flow")
	fs.StringVar(&clientID, "client-id", "", "OAuth client ID")
	fs.StringVar(&authorizeURL, "oauth-authorize-url", "", "OAuth authorize URL")
	fs.StringVar(&tokenURL, "oauth-token-url", "", "OAuth token URL")
	fs.StringVar(&oauthListen, "oauth-listen", "", "OAuth callback listen address (host:port)")
	fs.StringVar(&redirectURI, "oauth-redirect-uri", "", "OAuth redirect URI")
	fs.BoolVar(&help, "help", false, "Show help")
	fs.BoolVar(&help, "h", false, "Show help")
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return &CodeError{Code: exitUsage, Err: err}
	}
	if help {
		printAuthHelp(ctx.Stdout)
		return nil
	}
	if oauth {
		if tokenStdin {
			return &CodeError{Code: exitUsage, Err: errors.New("--token-stdin cannot be used with --oauth")}
		}
		cfg, err := buildOAuthConfig(clientID, authorizeURL, tokenURL, redirectURI, oauthListen, noBrowser)
		if err != nil {
			return &CodeError{Code: exitUsage, Err: err}
		}
		token, err := performOAuthLogin(ctx, cfg)
		if err != nil {
			return err
		}
		if printEnv {
			fmt.Fprintf(ctx.Stdout, "export TODOIST_TOKEN=%s\n", token)
			return nil
		}
		return storeProfileToken(ctx, token)
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
		fmt.Fprint(ctx.Stderr, "Todoist API token: ")
		val, err := readLine(ctx.Stdin)
		if err != nil {
			return err
		}
		token = strings.TrimSpace(val)
	}
	if token == "" {
		return &CodeError{Code: exitUsage, Err: errors.New("token is empty")}
	}
	if printEnv {
		fmt.Fprintf(ctx.Stdout, "export TODOIST_TOKEN=%s\n", token)
		return nil
	}
	return storeProfileToken(ctx, token)
}

func authOAuthLogin(ctx *Context, cfg oauthConfig) (string, error) {
	verifier, err := generateOAuthRandomFn(32)
	if err != nil {
		return "", err
	}
	state, err := generateOAuthRandomFn(16)
	if err != nil {
		return "", err
	}
	authURL, err := buildOAuthAuthorizationURLFn(cfg, oauthCodeChallenge(verifier), state)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(ctx.Stderr, "OAuth authorization URL:\n%s\n", authURL)
	if !cfg.NoBrowser {
		if err := openOAuthBrowserFn(authURL); err != nil {
			fmt.Fprintf(ctx.Stderr, "warning: could not open browser automatically: %v\n", err)
			fmt.Fprintln(ctx.Stderr, "Open the OAuth authorization URL manually to continue.")
		}
	}
	reqCtx, cancel := requestContext(ctx)
	defer cancel()
	code, err := waitForOAuthCodeFn(reqCtx, cfg, state, 3*time.Minute)
	if err != nil {
		return "", err
	}
	token, err := exchangeOAuthTokenFn(reqCtx, cfg, code, verifier)
	if err != nil {
		return "", err
	}
	return token, nil
}

func storeProfileToken(ctx *Context, token string) error {
	credsPath := config.CredentialsPathFromConfig(ctx.ConfigPath)
	creds, _, err := config.LoadCredentials(credsPath)
	if err != nil {
		return err
	}
	if creds.Profiles == nil {
		creds.Profiles = map[string]config.Credential{}
	}
	creds.Profiles[ctx.Profile] = config.Credential{Token: token}
	if err := config.SaveCredentials(credsPath, creds); err != nil {
		return err
	}
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, map[string]any{
			"profile": ctx.Profile,
			"stored":  true,
		}, output.Meta{})
	}
	fmt.Fprintf(ctx.Stdout, "stored token for profile %q\n", ctx.Profile)
	return nil
}

func authStatus(ctx *Context) error {
	source := ctx.TokenSource
	configured := ctx.Token != ""
	if source == "" && configured {
		source = "unknown"
	}
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, map[string]any{
			"profile":    ctx.Profile,
			"configured": configured,
			"source":     source,
		}, output.Meta{})
	}
	if configured {
		fmt.Fprintf(ctx.Stdout, "profile %q token source: %s\n", ctx.Profile, source)
		return nil
	}
	fmt.Fprintf(ctx.Stdout, "profile %q has no token configured\n", ctx.Profile)
	return nil
}

func authLogout(ctx *Context) error {
	credsPath := config.CredentialsPathFromConfig(ctx.ConfigPath)
	creds, _, err := config.LoadCredentials(credsPath)
	if err != nil {
		return err
	}
	if creds.Profiles != nil {
		delete(creds.Profiles, ctx.Profile)
	}
	if err := config.SaveCredentials(credsPath, creds); err != nil {
		return err
	}
	if ctx.Mode == output.ModeJSON {
		return output.WriteJSON(ctx.Stdout, map[string]any{
			"profile": ctx.Profile,
			"removed": true,
		}, output.Meta{})
	}
	fmt.Fprintf(ctx.Stdout, "removed token for profile %q\n", ctx.Profile)
	return nil
}
