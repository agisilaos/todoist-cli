package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/output"

	"io"
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
	Help          bool
	Version       bool
	Quiet         bool
	QuietJSON     bool
	Verbose       bool
	JSON          bool
	Plain         bool
	NDJSON        bool
	NoColor       bool
	NoInput       bool
	TimeoutSec    int
	ConfigPath    string
	Profile       string
	DryRun        bool
	Force         bool
	BaseURL       string
	Fuzzy         bool
	NoFuzzy       bool
	ProgressJSONL string
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

	Client      *api.Client
	Now         func() time.Time
	RequestID   string
	Progress    *progressSink
	lookupCache *lookupCache
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
	mode, err := output.DetectMode(opts.JSON, opts.Plain, opts.NDJSON, isTTYFile(stdout))
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
	if sink, err := newProgressSink(opts.ProgressJSONL, stderr); err == nil {
		ctx.Progress = sink
		defer sink.Close()
	}
	if err := loadConfig(ctx); err != nil {
		fmt.Fprintln(stderr, err)
		return exitError
	}
	if len(rest) == 0 {
		printRootHelp(stdout)
		return exitOK
	}
	if opts.Help {
		rest = append(rest, "--help")
	}

	code := dispatch(ctx, rest)
	return code
}

func parseGlobalFlags(args []string, stderr io.Writer) (GlobalOptions, []string, error) {
	var opts GlobalOptions
	_ = stderr
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		switch {
		case arg == "--help" || arg == "-h":
			opts.Help = true
		case arg == "--version":
			opts.Version = true
		case arg == "--quiet" || arg == "-q":
			opts.Quiet = true
		case arg == "--quiet-json":
			opts.QuietJSON = true
		case arg == "--verbose" || arg == "-v":
			opts.Verbose = true
		case arg == "--json":
			opts.JSON = true
		case arg == "--plain":
			opts.Plain = true
		case arg == "--ndjson":
			opts.NDJSON = true
		case arg == "--no-color":
			opts.NoColor = true
		case arg == "--no-input":
			opts.NoInput = true
		case arg == "--dry-run" || arg == "-n":
			opts.DryRun = true
		case arg == "--force" || arg == "-f":
			opts.Force = true
		case arg == "--fuzzy":
			opts.Fuzzy = true
		case arg == "--no-fuzzy":
			opts.NoFuzzy = true
		case strings.HasPrefix(arg, "--timeout="):
			val := strings.TrimPrefix(arg, "--timeout=")
			timeout, err := strconv.Atoi(val)
			if err != nil {
				return opts, nil, fmt.Errorf("invalid value for --timeout: %s", val)
			}
			opts.TimeoutSec = timeout
		case arg == "--timeout":
			if i+1 >= len(args) {
				return opts, nil, errors.New("flag needs an argument: --timeout")
			}
			i++
			timeout, err := strconv.Atoi(args[i])
			if err != nil {
				return opts, nil, fmt.Errorf("invalid value for --timeout: %s", args[i])
			}
			opts.TimeoutSec = timeout
		case strings.HasPrefix(arg, "--config="):
			opts.ConfigPath = strings.TrimPrefix(arg, "--config=")
		case arg == "--config":
			if i+1 >= len(args) {
				return opts, nil, errors.New("flag needs an argument: --config")
			}
			i++
			opts.ConfigPath = args[i]
		case strings.HasPrefix(arg, "--profile="):
			opts.Profile = strings.TrimPrefix(arg, "--profile=")
		case arg == "--profile":
			if i+1 >= len(args) {
				return opts, nil, errors.New("flag needs an argument: --profile")
			}
			i++
			opts.Profile = args[i]
		case strings.HasPrefix(arg, "--base-url="):
			opts.BaseURL = strings.TrimPrefix(arg, "--base-url=")
		case arg == "--base-url":
			if i+1 >= len(args) {
				return opts, nil, errors.New("flag needs an argument: --base-url")
			}
			i++
			opts.BaseURL = args[i]
		case strings.HasPrefix(arg, "--progress-jsonl="):
			opts.ProgressJSONL = strings.TrimPrefix(arg, "--progress-jsonl=")
		case arg == "--progress-jsonl":
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				opts.ProgressJSONL = args[i]
			} else {
				opts.ProgressJSONL = "-"
			}
		default:
			rest = append(rest, arg)
		}
	}
	if opts.Quiet && opts.Verbose {
		return opts, rest, fmt.Errorf("--quiet and --verbose are mutually exclusive")
	}
	return opts, rest, nil
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

func (e *CodeError) Unwrap() error {
	return e.Err
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
