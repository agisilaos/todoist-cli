package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

const (
	exitOK       = 0
	exitError    = 1
	exitUsage    = 2
	exitAuth     = 3
	exitNotFound = 4
	exitConflict = 5
)

type GlobalOptions struct {
	Help       bool
	Version    bool
	Quiet      bool
	Verbose    bool
	JSON       bool
	Plain      bool
	NoColor    bool
	NoInput    bool
	TimeoutSec int
	ConfigPath string
	Profile    string
	DryRun     bool
	Force      bool
	BaseURL    string
	Fuzzy      bool
	NoFuzzy    bool
}

type Context struct {
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader

	Global     GlobalOptions
	Mode       output.Mode
	Config     config.Config
	Profile    string
	ConfigPath string
	Fuzzy      bool

	Token       string
	TokenSource string

	Client    *api.Client
	Now       func() time.Time
	RequestID string
}

func Execute(args []string, stdout, stderr io.Writer) int {
	opts, rest, err := parseGlobalFlags(args, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		printRootHelp(stderr)
		return exitUsage
	}
	if opts.Version {
		fmt.Fprintf(stdout, "todoist %s (%s) %s\n", Version, Commit, Date)
		return exitOK
	}
	mode, err := output.DetectMode(opts.JSON, opts.Plain, isTTYFile(stdout))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitUsage
	}

	ctx := &Context{
		Stdout: stdout,
		Stderr: stderr,
		Stdin:  os.Stdin,
		Global: opts,
		Mode:   mode,
		Now:    time.Now,
	}
	if err := loadConfig(ctx); err != nil {
		fmt.Fprintln(stderr, err)
		return exitError
	}
	if opts.Help || len(rest) == 0 {
		printRootHelp(stdout)
		return exitOK
	}

	code := dispatch(ctx, rest)
	return code
}

func parseGlobalFlags(args []string, stderr io.Writer) (GlobalOptions, []string, error) {
	fs := flag.NewFlagSet("todoist", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var opts GlobalOptions
	fs.BoolVar(&opts.Help, "help", false, "Show help")
	fs.BoolVar(&opts.Help, "h", false, "Show help")
	fs.BoolVar(&opts.Version, "version", false, "Show version")
	fs.BoolVar(&opts.Quiet, "quiet", false, "Suppress non-essential output")
	fs.BoolVar(&opts.Quiet, "q", false, "Suppress non-essential output")
	fs.BoolVar(&opts.Verbose, "verbose", false, "Enable verbose output")
	fs.BoolVar(&opts.Verbose, "v", false, "Enable verbose output")
	fs.BoolVar(&opts.JSON, "json", false, "JSON output")
	fs.BoolVar(&opts.Plain, "plain", false, "Plain output")
	fs.BoolVar(&opts.NoColor, "no-color", false, "Disable color")
	fs.BoolVar(&opts.NoInput, "no-input", false, "Disable prompts")
	fs.IntVar(&opts.TimeoutSec, "timeout", 0, "Timeout in seconds (default 10)")
	fs.StringVar(&opts.ConfigPath, "config", "", "Config file path")
	fs.StringVar(&opts.Profile, "profile", "", "Profile name")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "Preview changes without applying")
	fs.BoolVar(&opts.DryRun, "n", false, "Preview changes without applying")
	fs.BoolVar(&opts.Force, "force", false, "Skip confirmation prompts")
	fs.BoolVar(&opts.Force, "f", false, "Skip confirmation prompts")
	fs.BoolVar(&opts.Fuzzy, "fuzzy", false, "Enable fuzzy name resolution")
	fs.BoolVar(&opts.NoFuzzy, "no-fuzzy", false, "Disable fuzzy name resolution")
	fs.StringVar(&opts.BaseURL, "base-url", "", "Override API base URL")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return opts, fs.Args(), nil
		}
		return opts, fs.Args(), err
	}
	if opts.Quiet && opts.Verbose {
		return opts, fs.Args(), fmt.Errorf("--quiet and --verbose are mutually exclusive")
	}
	return opts, fs.Args(), nil
}

func loadConfig(ctx *Context) error {
	configPath := ctx.Global.ConfigPath
	if configPath == "" {
		configPath = os.Getenv("TODOIST_CONFIG")
	}
	if configPath == "" {
		path, err := config.DefaultUserConfigPath()
		if err != nil {
			return err
		}
		configPath = path
	}
	ctx.ConfigPath = configPath
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	projectConfigPath := config.DefaultProjectConfigPath(cwd)

	userCfg, _, err := config.LoadConfig(configPath)
	if err != nil {
		return err
	}
	projectCfg, _, err := config.LoadConfig(projectConfigPath)
	if err != nil {
		return err
	}
	cfg := config.MergeConfig(userCfg, projectCfg)
	if env := os.Getenv("TODOIST_BASE_URL"); env != "" {
		cfg.BaseURL = env
	}
	if ctx.Global.BaseURL != "" {
		cfg.BaseURL = ctx.Global.BaseURL
	}
	if env := os.Getenv("TODOIST_TIMEOUT"); env != "" {
		if v, err := strconv.Atoi(env); err == nil {
			cfg.TimeoutSeconds = v
		}
	}
	if ctx.Global.TimeoutSec > 0 {
		cfg.TimeoutSeconds = ctx.Global.TimeoutSec
	}
	if env := os.Getenv("TODOIST_TABLE_WIDTH"); env != "" {
		if v, err := strconv.Atoi(env); err == nil && v > 0 {
			cfg.TableWidth = v
		}
	}
	if cfg.TimeoutSeconds == 0 {
		cfg.TimeoutSeconds = 10
	}
	ctx.Config = cfg

	profile := ctx.Global.Profile
	if profile == "" {
		profile = os.Getenv("TODOIST_PROFILE")
	}
	if profile == "" {
		profile = cfg.DefaultProfile
	}
	if profile == "" {
		profile = "default"
	}
	ctx.Profile = profile

	// Fuzzy resolution flag/env
	fuzzy := ctx.Global.Fuzzy
	if env := os.Getenv("TODOIST_FUZZY"); env != "" {
		if v, err := strconv.Atoi(env); err == nil && v > 0 {
			fuzzy = true
		}
	}
	if ctx.Global.NoFuzzy {
		fuzzy = false
	}
	ctx.Fuzzy = fuzzy

	token := os.Getenv("TODOIST_TOKEN")
	if token != "" {
		ctx.Token = token
		ctx.TokenSource = "env"
	} else {
		credsPath := config.CredentialsPathFromConfig(configPath)
		creds, _, err := config.LoadCredentials(credsPath)
		if err != nil {
			return err
		}
		if cred, ok := creds.Profiles[profile]; ok && cred.Token != "" {
			ctx.Token = cred.Token
			ctx.TokenSource = "credentials"
		}
	}
	if ctx.Token != "" {
		ctx.Client = api.NewClient(cfg.BaseURL, ctx.Token, time.Duration(cfg.TimeoutSeconds)*time.Second)
	}
	return nil
}

func isTTYFile(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return output.IsTTY(f)
}

func ensureClient(ctx *Context) error {
	if ctx.Token == "" {
		return &CodeError{Code: exitAuth, Err: fmt.Errorf("missing auth token; run 'todoist auth login' or set TODOIST_TOKEN")}
	}
	if ctx.Client == nil {
		ctx.Client = api.NewClient(ctx.Config.BaseURL, ctx.Token, time.Duration(ctx.Config.TimeoutSeconds)*time.Second)
	}
	return nil
}

type CodeError struct {
	Code int
	Err  error
}

func (e *CodeError) Error() string {
	return e.Err.Error()
}

func toExitCode(err error) int {
	if err == nil {
		return exitOK
	}
	var codeErr *CodeError
	if errors.As(err, &codeErr) {
		return codeErr.Code
	}
	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Status {
		case 401, 403:
			return exitAuth
		case 404:
			return exitNotFound
		case 409:
			return exitConflict
		default:
			return exitError
		}
	}
	return exitError
}

func requestContext(ctx *Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Duration(ctx.Config.TimeoutSeconds)*time.Second)
}

func parseIDOrName(input string) string {
	return stripIDPrefix(strings.TrimSpace(input))
}
