package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/agisilaos/todoist-cli/internal/api"
	"github.com/agisilaos/todoist-cli/internal/authorization"
	"github.com/agisilaos/todoist-cli/internal/config"
	"github.com/agisilaos/todoist-cli/internal/credentials"
	"github.com/agisilaos/todoist-cli/internal/output"
	"github.com/agisilaos/todoist-cli/internal/skillinstall"
)

var (
	Version = "dev"
	Commit  = "local"
	Date    = "unreleased"
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
	Help                 bool
	Version              bool
	Quiet                bool
	QuietJSON            bool
	Verbose              bool
	Accessible           bool
	JSON                 bool
	Plain                bool
	NDJSON               bool
	IDsOnly              bool
	TaskOutputVersion    int
	TaskOutputVersionSet bool
	NoColor              bool
	NoInput              bool
	TimeoutSec           int
	ConfigPath           string
	Profile              string
	DryRun               bool
	Force                bool
	BaseURL              string
	Fuzzy                bool
	NoFuzzy              bool
	ProgressJSONL        string
}

// Environment supplies invocation inputs. Nil fields use process defaults.
type Environment struct {
	local            localDependencies
	OperationContext context.Context
	Now              func() time.Time
	Stdin            io.Reader
	Getenv           func(string) string
}

type Context struct {
	local            localDependencies
	OperationContext context.Context
	Stdout           io.Writer
	Stderr           io.Writer
	Stdin            io.Reader

	Global                GlobalOptions
	Mode                  output.Mode
	Config                config.Config
	Profile               string
	SelectionSource       string
	UserDefaultProfile    string
	ProjectDefaultProfile string
	ConfigPath            string
	ConfigErr             error
	HelpPath              string
	Fuzzy                 bool
	Accessible            bool

	Credentials    credentials.Store
	CredentialInfo credentials.Info
	CredentialErr  error
	SavingBackend  string

	Token         string
	TokenSource   string
	Authorization *authorization.Report

	Client      *api.Client
	Getenv      func(string) string
	oauth       oauthDependencies
	Now         func() time.Time
	RequestID   string
	Progress    *progressSink
	lookupCache *lookupCache
}

func Execute(args []string, stdout, stderr io.Writer) int {
	return ExecuteWithEnvironment(args, stdout, stderr, Environment{})
}

// ExecuteWithEnvironment runs a command with invocation-local inputs, allowing
// callers to pin the clock or provide stdin without changing process globals.
func ExecuteWithEnvironment(args []string, stdout, stderr io.Writer, env Environment) int {
	if env.Now == nil {
		env.Now = time.Now
	}
	if env.Stdin == nil {
		env.Stdin = os.Stdin
	}
	if env.Getenv == nil {
		env.Getenv = os.Getenv
	}
	opts, rest, err := parseGlobalFlags(args, stderr)
	if err != nil {
		if len(rest) > 0 && rest[0] == "skill" {
			writeError(&Context{Stderr: stderr, Global: opts, Mode: skillErrorMode(opts)}, skillUsage(err.Error()))
			return exitUsage
		}
		if opts.IDsOnly {
			writeError(&Context{Stderr: stderr, Global: opts, Mode: output.ModeIDsOnly}, err)
			return exitUsage
		}
		fmt.Fprintln(stderr, err)
		printRootHelp(stderr)
		return exitUsage
	}
	if opts.Version {
		fmt.Fprintf(stdout, "todoist %s (%s) %s\n", Version, Commit, Date)
		return exitOK
	}
	mode, err := output.DetectMode(opts.JSON, opts.Plain, opts.NDJSON, opts.IDsOnly, isTTYFile(stdout))
	if err != nil {
		if len(rest) > 0 && rest[0] == "skill" {
			writeError(&Context{Stderr: stderr, Global: opts, Mode: skillErrorMode(opts)}, skillUsage(err.Error()))
			return exitUsage
		}
		if opts.IDsOnly {
			writeError(&Context{Stderr: stderr, Global: opts, Mode: output.ModeIDsOnly}, err)
			return exitUsage
		}
		fmt.Fprintln(stderr, err)
		return exitUsage
	}

	ctx := &Context{
		local:            env.local,
		OperationContext: env.OperationContext,
		Stdout:           stdout,
		Stderr:           stderr,
		Stdin:            env.Stdin,
		Global:           opts,
		Mode:             mode,
		Now:              env.Now,
		Getenv:           env.Getenv,
	}
	if opts.IDsOnly && !idsOnlyEligible(rest, opts.Help) {
		idsErr := fmt.Errorf("--ids-only is only supported by stable-ID list commands (see 'todoist schema --name ids_only')")
		if len(rest) > 0 && rest[0] == "skill" {
			writeError(ctx, skillUsage(idsErr.Error()))
		} else {
			writeError(ctx, idsErr)
		}
		return exitUsage
	}
	helpArgs := rest
	showHelp := opts.Help || len(rest) == 0
	if len(rest) > 0 && rest[0] == "help" {
		helpArgs, showHelp = rest[1:], true
	} else if len(rest) > 1 && rest[1] == "help" {
		helpArgs = append([]string{rest[0]}, rest[2:]...)
		showHelp = true
	}
	if showHelp {
		err := helpCommand(ctx, helpArgs)
		writeError(ctx, err)
		return toExitCode(err)
	}
	if err := validateTaskOutputSelection(ctx, rest); err != nil {
		if rest[0] == "skill" {
			err = skillUsage(err.Error())
		}
		writeError(ctx, err)
		return exitUsage
	}
	// Skill maintenance is entirely local and independent of Todoist configuration,
	// credential stores, API clients, and progress logs.
	if rest[0] == "skill" {
		return dispatch(ctx, rest)
	}
	sink, err := newProgressSink(opts.ProgressJSONL, stderr)
	if err != nil {
		writeError(ctx, fmt.Errorf("open progress log: %w", err))
		return exitError
	}
	ctx.Progress = sink
	defer sink.Close()
	if err := loadConfig(ctx); err != nil {
		if rest[0] != "doctor" {
			writeError(ctx, err)
			return toExitCode(err)
		}
		ctx.ConfigErr = err
	}

	code := dispatch(ctx, rest)
	return code
}

func parseGlobalFlags(args []string, stderr io.Writer) (GlobalOptions, []string, error) {
	var opts GlobalOptions
	var parseErr error
	// Finish scanning so trailing output flags still control error formatting.
	recordError := func(err error) {
		if parseErr == nil {
			parseErr = err
		}
	}
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
		case strings.HasPrefix(arg, "--help=") || strings.HasPrefix(arg, "-h="):
			_, value, _ := strings.Cut(arg, "=")
			help, err := strconv.ParseBool(value)
			if err != nil {
				recordError(fmt.Errorf("invalid value for --help: %s", value))
				continue
			}
			opts.Help = help
		case arg == "--version":
			opts.Version = true
		case arg == "--quiet" || arg == "-q":
			opts.Quiet = true
		case arg == "--quiet-json":
			opts.QuietJSON = true
		case arg == "--verbose" || arg == "-v":
			opts.Verbose = true
		case arg == "--accessible":
			opts.Accessible = true
		case arg == "--json":
			opts.JSON = true
		case arg == "--plain":
			opts.Plain = true
		case arg == "--ndjson":
			opts.NDJSON = true
		case arg == "--ids-only":
			opts.IDsOnly = true
		case strings.HasPrefix(arg, "--task-output-version="):
			value := strings.TrimPrefix(arg, "--task-output-version=")
			opts.TaskOutputVersionSet = true
			version, err := parseTaskOutputVersion(value)
			if err != nil {
				recordError(err)
			}
			opts.TaskOutputVersion = version
		case arg == "--task-output-version":
			opts.TaskOutputVersionSet = true
			if i+1 >= len(args) {
				recordError(errors.New("flag needs an argument: --task-output-version"))
				continue
			}
			i++
			version, err := parseTaskOutputVersion(args[i])
			if err != nil {
				recordError(err)
			}
			opts.TaskOutputVersion = version
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
				recordError(fmt.Errorf("invalid value for --timeout: %s", val))
				continue
			}
			opts.TimeoutSec = timeout
		case arg == "--timeout":
			if i+1 >= len(args) {
				recordError(errors.New("flag needs an argument: --timeout"))
				continue
			}
			i++
			timeout, err := strconv.Atoi(args[i])
			if err != nil {
				recordError(fmt.Errorf("invalid value for --timeout: %s", args[i]))
				continue
			}
			opts.TimeoutSec = timeout
		case strings.HasPrefix(arg, "--config="):
			opts.ConfigPath = strings.TrimPrefix(arg, "--config=")
		case arg == "--config":
			if i+1 >= len(args) {
				recordError(errors.New("flag needs an argument: --config"))
				continue
			}
			i++
			opts.ConfigPath = args[i]
		case strings.HasPrefix(arg, "--profile="):
			opts.Profile = strings.TrimPrefix(arg, "--profile=")
		case arg == "--profile":
			if i+1 >= len(args) {
				recordError(errors.New("flag needs an argument: --profile"))
				continue
			}
			i++
			opts.Profile = args[i]
		case strings.HasPrefix(arg, "--base-url="):
			opts.BaseURL = strings.TrimPrefix(arg, "--base-url=")
		case arg == "--base-url":
			if i+1 >= len(args) {
				recordError(errors.New("flag needs an argument: --base-url"))
				continue
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
			name, hasValue := splitFlagName(arg)
			if !hasValue && commandFlagTakesValue(name) && i+1 < len(args) {
				i++
				rest = append(rest, args[i])
			}
		}
	}
	if parseErr != nil {
		return opts, rest, parseErr
	}
	if opts.Quiet && opts.Verbose {
		return opts, rest, fmt.Errorf("--quiet and --verbose are mutually exclusive")
	}
	return opts, rest, nil
}

func loadConfig(ctx *Context) error {
	configPath := ctx.Global.ConfigPath
	if configPath == "" {
		configPath = ctx.getenv("TODOIST_CONFIG")
	}
	if configPath == "" {
		path, err := config.DefaultUserConfigPathWithEnv(ctx.getenv)
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
		return fmt.Errorf("load config %s: %w", configPath, err)
	}
	projectCfg, _, err := config.LoadConfig(projectConfigPath)
	if err != nil {
		return fmt.Errorf("load config %s: %w", projectConfigPath, err)
	}
	cfg := config.MergeConfig(userCfg, projectCfg)
	applyEnvString(ctx, "TODOIST_BASE_URL", &cfg.BaseURL)
	if ctx.Global.BaseURL != "" {
		cfg.BaseURL = ctx.Global.BaseURL
	}
	applyEnvInt(ctx, "TODOIST_TIMEOUT", &cfg.TimeoutSeconds, false)
	if ctx.Global.TimeoutSec > 0 {
		cfg.TimeoutSeconds = ctx.Global.TimeoutSec
	}
	applyEnvInt(ctx, "TODOIST_TABLE_WIDTH", &cfg.TableWidth, true)
	if cfg.TimeoutSeconds == 0 {
		cfg.TimeoutSeconds = 10
	}
	ctx.Config = cfg

	ctx.UserDefaultProfile = userCfg.DefaultProfile
	ctx.ProjectDefaultProfile = projectCfg.DefaultProfile
	ctx.Profile, ctx.SelectionSource = resolveProfileSelection(ctx, ctx.Global.Profile, projectCfg.DefaultProfile, userCfg.DefaultProfile)

	// Fuzzy resolution flag/env
	fuzzy := ctx.Global.Fuzzy
	if parsePositiveEnvFlag(ctx, "TODOIST_FUZZY") {
		fuzzy = true
	}
	if ctx.Global.NoFuzzy {
		fuzzy = false
	}
	ctx.Fuzzy = fuzzy
	accessible := ctx.Global.Accessible
	if parsePositiveEnvFlag(ctx, "TODOIST_ACCESSIBLE") {
		accessible = true
	}
	ctx.Accessible = accessible

	token := ctx.getenv("TODOIST_TOKEN")
	if token != "" {
		ctx.Token = token
		ctx.TokenSource = "env"
		report := authorization.Resolve(nil, "env", true)
		ctx.Authorization = &report
	} else {
		inspectProfile(ctx)
	}
	if ctx.Token != "" {
		ctx.Client = api.NewClient(cfg.BaseURL, ctx.Token, time.Duration(cfg.TimeoutSeconds)*time.Second, currentAuthorization(ctx))
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
	if ctx.CredentialErr != nil && ctx.TokenSource != "env" {
		return ctx.CredentialErr
	}
	if err := currentAuthorization(ctx).CheckCredential(); err != nil {
		return err
	}
	if err := resolveStoredToken(ctx); err != nil {
		return err
	}
	if err := currentAuthorization(ctx).CheckCredential(); err != nil {
		return err
	}
	if ctx.Token == "" {
		return &CodeError{Code: exitAuth, Err: errMissingToken}
	}
	if ctx.Client == nil {
		ctx.Client = api.NewClient(ctx.Config.BaseURL, ctx.Token, time.Duration(ctx.Config.TimeoutSeconds)*time.Second, currentAuthorization(ctx))
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
	var skillErr *skillinstall.Error
	if errors.As(err, &skillErr) {
		switch skillErr.Code {
		case "SKILL_USAGE", "SKILL_PATH_INVALID":
			return exitUsage
		case "SKILL_CONFLICT", "SKILL_MODIFIED", "SKILL_MANIFEST_INVALID", "SKILL_BUSY":
			return exitConflict
		case "SKILL_NOT_INSTALLED":
			return exitNotFound
		default:
			return exitError
		}
	}
	var storageErr *credentials.Error
	if errors.As(err, &storageErr) {
		if storageErr.Kind == credentials.Selection {
			return exitUsage
		}
		return exitAuth
	}
	var authorizationErr *authorization.Error
	if errors.As(err, &authorizationErr) {
		return exitAuth
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

func operationContext(ctx *Context) context.Context {
	if ctx.OperationContext != nil {
		return ctx.OperationContext
	}
	return context.Background()
}

func requestContext(ctx *Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(operationContext(ctx), time.Duration(ctx.Config.TimeoutSeconds)*time.Second)
}

func applyEnvString(ctx *Context, key string, target *string) {
	if target == nil {
		return
	}
	if env := ctx.getenv(key); env != "" {
		*target = env
	}
}

func applyEnvInt(ctx *Context, key string, target *int, positiveOnly bool) {
	if target == nil {
		return
	}
	env := strings.TrimSpace(ctx.getenv(key))
	if env == "" {
		return
	}
	v, err := strconv.Atoi(env)
	if err != nil {
		return
	}
	if positiveOnly && v <= 0 {
		return
	}
	*target = v
}

func parsePositiveEnvFlag(ctx *Context, key string) bool {
	v, err := strconv.Atoi(strings.TrimSpace(ctx.getenv(key)))
	return err == nil && v > 0
}

func resolveProfile(ctx *Context, flagValue, defaultProfile string) string {
	name, _ := resolveProfileSelection(ctx, flagValue, "", defaultProfile)
	return name
}

func resolveProfileSelection(ctx *Context, flagValue, projectDefault, userDefault string) (string, string) {
	if flagValue != "" {
		return flagValue, "flag"
	}
	if env := ctx.getenv("TODOIST_PROFILE"); env != "" {
		return env, "environment"
	}
	if projectDefault != "" {
		return projectDefault, "project"
	}
	if userDefault != "" {
		return userDefault, "user"
	}
	return "default", "fallback"
}

func (ctx *Context) getenv(key string) string {
	if ctx != nil && ctx.Getenv != nil {
		return ctx.Getenv(key)
	}
	return os.Getenv(key)
}

func isOperationCancellation(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
