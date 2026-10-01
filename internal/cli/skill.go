package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/agisilaos/todoist-cli/internal/agentskill"
	"github.com/agisilaos/todoist-cli/internal/output"
	"github.com/agisilaos/todoist-cli/internal/skillinstall"
)

func skillCommand(ctx *Context, args []string) error {
	if len(args) == 0 {
		printSkillHelp(ctx.Stdout)
		return nil
	}
	if ctx.Global.Force || ctx.Global.DryRun {
		return skillUsage("skill commands do not support --force or --dry-run; inspect with 'todoist skill list', update edits with --backup, or uninstall with --keep-modified")
	}
	switch args[0] {
	case "install":
		return skillInstall(ctx, args[1:])
	case "list":
		return skillList(ctx, args[1:])
	case "update":
		return skillUpdate(ctx, args[1:])
	case "uninstall":
		return skillUninstall(ctx, args[1:])
	default:
		return skillUsage(fmt.Sprintf("unknown skill subcommand: %s; run 'todoist skill --help'", args[0]))
	}
}

func skillUsage(message string) error {
	return &skillinstall.Error{Code: "SKILL_USAGE", Message: message}
}

func skillErrorMode(opts GlobalOptions) output.Mode {
	if opts.JSON || opts.IDsOnly {
		return output.ModeJSON
	}
	if opts.NDJSON {
		return output.ModeNDJSON
	}
	return output.ModePlain
}

func skillBundle() skillinstall.Bundle {
	return skillinstall.Bundle{Version: Version, Files: agentskill.Files()}
}

func skillInstall(ctx *Context, args []string) error {
	fs := newFlagSet("skill install")
	var scope, path string
	fs.StringVar(&scope, "scope", "", "Installation scope: local or global")
	fs.StringVar(&path, "path", "", "Absolute skill directory")
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return skillUsage(err.Error() + "; run 'todoist skill install --help'")
	}
	loc, err := skillWriteLocation(fs.Args(), scope, path)
	if err != nil {
		return err
	}
	result, err := skillinstall.Install(loc, skillBundle())
	if err != nil {
		return err
	}
	return writeSkillResult(ctx, result)
}

func skillUpdate(ctx *Context, args []string) error {
	fs := newFlagSet("skill update")
	var scope, path string
	var backup bool
	fs.StringVar(&scope, "scope", "", "Installation scope: local or global")
	fs.StringVar(&path, "path", "", "Absolute skill directory")
	fs.BoolVar(&backup, "backup", false, "Back up modified files before replacement")
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return skillUsage(err.Error() + "; run 'todoist skill update --help'")
	}
	loc, err := skillWriteLocation(fs.Args(), scope, path)
	if err != nil {
		return err
	}
	result, err := skillinstall.Update(loc, skillBundle(), skillinstall.UpdateOptions{Backup: backup})
	if err != nil {
		return err
	}
	return writeSkillResult(ctx, result)
}

func skillUninstall(ctx *Context, args []string) error {
	fs := newFlagSet("skill uninstall")
	var scope, path string
	var keepModified bool
	fs.StringVar(&scope, "scope", "", "Installation scope: local or global")
	fs.StringVar(&path, "path", "", "Absolute skill directory")
	fs.BoolVar(&keepModified, "keep-modified", false, "Leave modified managed files behind")
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return skillUsage(err.Error() + "; run 'todoist skill uninstall --help'")
	}
	loc, err := skillWriteLocation(fs.Args(), scope, path)
	if err != nil {
		return err
	}
	result, err := skillinstall.Uninstall(loc, skillinstall.UninstallOptions{KeepModified: keepModified})
	if err != nil {
		return err
	}
	return writeSkillResult(ctx, result)
}

func skillWriteLocation(args []string, scope, path string) (skillinstall.Location, error) {
	if len(args) != 1 || scope == "" || path == "" {
		return skillinstall.Location{}, skillUsage("select one target and supply --scope local|global and --path with its full absolute skill directory; run 'todoist skill list' for conventional locations")
	}
	return skillinstall.ValidateLocation(skillinstall.Location{Target: args[0], Scope: scope, Path: path})
}

type skillListItem struct {
	skillinstall.State
	Status          string `json:"status"`
	AvailableDigest string `json:"available_digest"`
	Code            string `json:"code,omitempty"`
	Error           string `json:"error,omitempty"`
}

func skillList(ctx *Context, args []string) error {
	fs := newFlagSet("skill list")
	var scope, path string
	fs.StringVar(&scope, "scope", "", "Restrict to local or global installations")
	fs.StringVar(&path, "path", "", "Inspect one absolute skill directory; target and scope required")
	if err := parseFlagSetInterspersed(fs, args); err != nil {
		return skillUsage(err.Error() + "; run 'todoist skill list --help'")
	}
	if len(fs.Args()) > 1 {
		return skillUsage("skill list accepts at most one target")
	}
	target := ""
	if len(fs.Args()) == 1 {
		target = fs.Args()[0]
	}
	if scope != "" && scope != "local" && scope != "global" {
		return skillUsage("--scope must be local or global")
	}
	var locations []skillinstall.Location
	if path != "" {
		loc, err := skillWriteLocation(fs.Args(), scope, path)
		if err != nil {
			return err
		}
		locations = append(locations, loc)
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return &skillinstall.Error{Code: "SKILL_IO", Message: "cannot determine current directory; supply an explicit --path"}
		}
		home, err := os.UserHomeDir()
		if err != nil && scope != "local" {
			return &skillinstall.Error{Code: "SKILL_IO", Message: "cannot determine user home; supply an explicit --path"}
		}
		found := target == ""
		for _, supported := range skillinstall.Targets() {
			if target != "" && target != supported.Name {
				continue
			}
			found = true
			for _, placement := range []struct{ scope, root string }{{"local", cwd}, {"global", home}} {
				if scope == "" || scope == placement.scope {
					locations = append(locations, skillinstall.Location{Target: supported.Name, Scope: placement.scope, Path: filepath.Join(placement.root, supported.Directory, "skills", agentskill.Name)})
				}
			}
		}
		if !found {
			return skillUsage("unsupported target; use codex or claude-code")
		}
	}
	digest, err := skillinstall.Digest(skillBundle())
	if err != nil {
		return err
	}
	items := make([]skillListItem, 0, len(locations))
	for _, loc := range locations {
		state, inspectErr := skillinstall.Inspect(loc)
		item := skillListItem{State: state, Status: "absent", AvailableDigest: digest}
		if inspectErr != nil {
			item.State = skillinstall.State{Target: loc.Target, Scope: loc.Scope, Path: loc.Path}
			item.Status, item.Error = "error", inspectErr.Error()
			var typed *skillinstall.Error
			if errors.As(inspectErr, &typed) {
				item.Code = typed.Code
				if typed.Code == "SKILL_CONFLICT" || typed.Code == "SKILL_MANIFEST_INVALID" {
					item.Status = "conflict"
				}
			}
		} else if state.Installed {
			item.Status = "installed"
			if state.PackageDigest != digest {
				item.Status = "outdated"
			}
			if len(state.Modified) > 0 || len(state.Missing) > 0 {
				item.Status = "modified"
			}
		}
		if item.Modified == nil {
			item.Modified = []string{}
		}
		if item.Missing == nil {
			item.Missing = []string{}
		}
		items = append(items, item)
	}
	switch ctx.Mode {
	case output.ModeJSON:
		return skillInventoryOutputError(output.WriteJSONArray(ctx.Stdout, items))
	case output.ModeNDJSON:
		return skillInventoryOutputError(output.WriteNDJSONSlice(ctx.Stdout, items))
	default:
		for _, item := range items {
			if _, err := fmt.Fprintf(ctx.Stdout, "%s\t%s\t%s\t%s\n", item.Target, item.Scope, item.Status, strconv.Quote(item.Path)); err != nil {
				return skillInventoryOutputError(err)
			}
			if item.Error != "" {
				fmt.Fprintf(ctx.Stderr, "%s: %s\n", item.Code, item.Error)
			}
		}
		return nil
	}
}

func writeSkillResult(ctx *Context, result skillinstall.Result) error {
	if result.Files == nil {
		result.Files = []string{}
	}
	if result.Retained == nil {
		result.Retained = []string{}
	}
	var err error
	switch ctx.Mode {
	case output.ModeJSON:
		err = output.WriteJSON(ctx.Stdout, result, output.Meta{})
	case output.ModeNDJSON:
		err = json.NewEncoder(ctx.Stdout).Encode(result)
	default:
		if ctx.Global.Quiet {
			return nil
		}
		_, err = fmt.Fprintf(ctx.Stdout, "%s\t%s\t%s\t%s\t%s\n", result.Operation, result.Status, result.Target, result.Scope, strconv.Quote(result.Path))
		if err == nil && result.BackupPath != "" {
			_, err = fmt.Fprintf(ctx.Stdout, "Backup: %s\n", strconv.Quote(result.BackupPath))
		}
		for _, retained := range result.Retained {
			if err != nil {
				break
			}
			_, err = fmt.Fprintf(ctx.Stdout, "Retained: %s\n", strconv.Quote(retained))
		}
	}
	if err != nil {
		return &skillinstall.Error{Code: "SKILL_IO", Message: "The skill operation finished, but its result could not be written. Inspect this installation before retrying.", Path: result.Path, BackupPath: result.BackupPath, Committed: result.Status != "unchanged"}
	}
	return nil
}

func skillInventoryOutputError(err error) error {
	if err == nil {
		return nil
	}
	return &skillinstall.Error{Code: "SKILL_IO", Message: "The skill inventory could not be written. Check the output destination and retry; no skill files changed."}
}

func printSkillHelp(out interface{ Write([]byte) (int, error) }) {
	fmt.Fprint(out, `Usage:
  todoist skill list [codex|claude-code] [--scope local|global] [--path <absolute-directory>]
  todoist skill install <codex|claude-code> --scope <local|global> --path <absolute-directory>
  todoist skill update <codex|claude-code> --scope <local|global> --path <absolute-directory> [--backup]
  todoist skill uninstall <codex|claude-code> --scope <local|global> --path <absolute-directory> [--keep-modified]

Targets:
  codex        <project-or-home>/.agents/skills/todoist-cli
  claude-code  <project-or-home>/.claude/skills/todoist-cli

Notes:
  Install one selected target; supply its full absolute path and scope for every write.
  List shows current-directory and user-home locations; explicit --path also requires a target and scope.
  Uses the running binary's bundled skill. No automatic or network updates.
  Modified files block update and uninstall by default. --backup preserves originals before update;
  --keep-modified uninstalls unchanged owned files, leaves edits, and removes management ownership.
  Unrelated files, global agent instructions, and shell profiles are preserved. No prompts or credentials.
  --force and --dry-run are unsupported; use list for inspection.
  Use --json or --ndjson for stable output; lifecycle errors are JSON on stderr in either mode.
  See 'todoist --help' for global flags and 'todoist skill install --help' for loading checks.

Examples:
  todoist skill list --json
  todoist skill install codex --scope local --path /work/project/.agents/skills/todoist-cli --no-input
`)
}
