package skillinstall

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

type fileOps struct {
	writeFile func(string, []byte, os.FileMode) error
	rename    func(string, string) error
	remove    func(string) error
	removeAll func(string) error
}

type manager struct{ fs fileOps }

func newManager() manager {
	return manager{fs: fileOps{writeFile: os.WriteFile, rename: os.Rename, remove: os.Remove, removeAll: os.RemoveAll}}
}

func Install(location Location, bundle Bundle) (Result, error) {
	return newManager().install(location, bundle, false, UpdateOptions{})
}

func Update(location Location, bundle Bundle, options UpdateOptions) (Result, error) {
	return newManager().install(location, bundle, true, options)
}

func Uninstall(location Location, options UninstallOptions) (Result, error) {
	return newManager().uninstall(location, options)
}

func initialResult(operation string, location Location) Result {
	return Result{Operation: operation, Status: "unchanged", Target: location.Target, Scope: location.Scope, Path: location.Path, Files: []string{}, Retained: []string{}}
}

func (m manager) install(location Location, bundle Bundle, update bool, options UpdateOptions) (result Result, returnedErr error) {
	operation := "install"
	if update {
		operation = "update"
	}
	result = initialResult(operation, location)
	location, err := ValidateLocation(location)
	if err != nil {
		return result, err
	}
	result.Path = location.Path
	hashes, err := bundleHashes(bundle)
	if err != nil {
		return result, err
	}
	result.Version, result.PackageDigest = bundle.Version, hashesDigest(hashes)
	release, err := m.lock(location)
	if err != nil {
		return result, err
	}
	defer func() {
		returnedErr = release(returnedErr, result.Status != "unchanged")
		var typed *Error
		if result.BackupPath != "" && errors.As(returnedErr, &typed) {
			typed.BackupPath = result.BackupPath
		}
	}()
	inspection, err := inspect(location)
	if err != nil {
		return result, err
	}
	if update && inspection.manifest == nil {
		return result, &Error{Code: "SKILL_NOT_INSTALLED", Message: "No managed skill is installed here. Run skill install for this target and path first.", Path: location.Path}
	}
	if inspection.manifest != nil {
		if len(inspection.modified)+len(inspection.missing) != 0 && (!update || !options.Backup) {
			return result, modifiedError(location, inspection, update)
		}
		if inspection.manifest.PackageDigest == result.PackageDigest && inspection.manifest.Version == bundle.Version && len(inspection.modified)+len(inspection.missing) == 0 {
			return result, nil
		}
		if !update {
			return result, &Error{Code: "SKILL_CONFLICT", Message: "A different skill package is already installed. Use skill update deliberately; install never replaces an installed package.", Path: location.Path}
		}
	}
	// Inspect every destination before staging, including files newly added by an
	// update. Unowned files are never adopted, even if their bytes happen to match.
	for _, name := range sortedKeys(bundle.Files) {
		if _, owned := inspection.snapshot[name]; owned {
			continue
		}
		full, err := safeFile(location.Path, name)
		if err != nil {
			return result, err
		}
		before, err := readSnapshot(full)
		if err != nil {
			return result, err
		}
		if before.exists {
			return result, &Error{Code: "SKILL_CONFLICT", Message: "An unowned file conflicts with this package. Move it or choose another destination; backup does not replace unowned content.", Path: full, Files: []string{name}}
		}
		inspection.snapshot[name] = before
	}
	if options.Backup && len(inspection.modified) > 0 {
		backup, err := m.backup(location, inspection)
		result.BackupPath = backup
		if err != nil {
			return result, err
		}
	}
	manifestData, err := json.MarshalIndent(manifest{FormatVersion: 1, Target: location.Target, Scope: location.Scope, Version: bundle.Version, PackageDigest: result.PackageDigest, Files: hashes}, "", "  ")
	if err != nil {
		return result, ioError(location.Path, "Encode the skill ownership manifest and retry.", err)
	}
	desired := map[string]snapshot{ManifestName: {data: append(manifestData, '\n'), mode: 0o600, exists: true}}
	for name, data := range bundle.Files {
		desired[name] = snapshot{data: data, mode: 0o644, exists: true}
	}
	for name := range inspection.snapshot {
		if _, exists := desired[name]; !exists {
			desired[name] = snapshot{}
		}
	}
	if err := m.change(location, inspection.snapshot, desired); err != nil {
		if typed, ok := err.(*Error); ok {
			if typed.Committed {
				result.Status = "updated"
				if !update {
					result.Status = "installed"
				}
			}
		}
		return result, err
	}
	result.Status = "installed"
	if update {
		result.Status = "updated"
	}
	result.Files = sortedKeys(hashes)
	return result, nil
}

func modifiedError(location Location, inspection inspected, update bool) *Error {
	files := append(append([]string{}, inspection.modified...), inspection.missing...)
	sort.Strings(files)
	message := "Owned skill files were changed or removed. Restore them before installing; user changes were preserved."
	if update {
		message = "Owned skill files were changed or removed. Restore them, or use update --backup to preserve changed originals before replacement."
	}
	return &Error{Code: "SKILL_MODIFIED", Message: message, Path: location.Path, Files: files}
}

func (m manager) uninstall(location Location, options UninstallOptions) (result Result, returnedErr error) {
	result = initialResult("uninstall", location)
	location, err := ValidateLocation(location)
	if err != nil {
		return result, err
	}
	result.Path = location.Path
	// An absent target with no parent directory has no state to serialize. Avoid
	// creating a target merely to uninstall an already absent package.
	if _, err := os.Lstat(filepath.Dir(location.Path)); errors.Is(err, os.ErrNotExist) {
		return result, nil
	} else if err != nil {
		return result, ioError(location.Path, "Inspect the skill destination and retry.", err)
	}
	release, err := m.lock(location)
	if err != nil {
		return result, err
	}
	defer func() { returnedErr = release(returnedErr, result.Status != "unchanged") }()
	inspection, err := inspect(location)
	if err != nil {
		return result, err
	}
	if inspection.manifest == nil {
		result.Retained, err = retainedFiles(location.Path)
		return result, err
	}
	result.Version, result.PackageDigest = inspection.manifest.Version, inspection.manifest.PackageDigest
	if len(inspection.modified)+len(inspection.missing) != 0 && !options.KeepModified {
		err := modifiedError(location, inspection, false)
		err.Message = "Owned skill files were changed or removed. Restore them, or use uninstall --keep-modified to remove management while retaining changed files."
		return result, err
	}
	desired := map[string]snapshot{ManifestName: {}}
	modified := map[string]bool{}
	for _, name := range inspection.modified {
		modified[name] = true
	}
	for _, name := range sortedKeys(inspection.manifest.Files) {
		if modified[name] {
			continue
		}
		desired[name] = snapshot{}
		if inspection.snapshot[name].exists {
			result.Files = append(result.Files, name)
		}
	}
	if err := m.change(location, inspection.snapshot, desired); err != nil {
		if typed, ok := err.(*Error); ok && typed.Committed {
			result.Status = "uninstalled"
		}
		return result, err
	}
	result.Status = "uninstalled"
	if err := m.removeEmptyDirectories(location.Path, sortedKeys(inspection.manifest.Files)); err != nil {
		typed := ioError(location.Path, "Owned files were uninstalled, but an empty directory could not be removed. Inspect the remaining directory before cleanup.", err)
		typed.Committed = true
		return result, typed
	}
	result.Retained, err = retainedFiles(location.Path)
	if typed, ok := err.(*Error); ok {
		typed.Committed = true
	}
	return result, err
}

func (m manager) lock(location Location) (func(error, bool) error, error) {
	agentDir := filepath.Dir(filepath.Dir(location.Path))
	if err := safeDirectories(agentDir, filepath.Dir(location.Path)); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(location.Path), 0o755); err != nil {
		return nil, ioError(agentDir, "Create the selected agent directory and retry.", err)
	}
	name := filepath.Join(agentDir, ".todoist-cli.lock")
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return nil, &Error{Code: "SKILL_BUSY", Message: "Another skill operation holds the installation lock. Wait for it to finish; after an interruption, verify no operation is running before removing the lock.", Path: name}
	}
	if err != nil {
		return nil, ioError(name, "Create the installation lock and retry.", err)
	}
	if _, err := fmt.Fprintf(file, "pid=%d\n", os.Getpid()); err != nil {
		_ = file.Close()
		_ = m.fs.remove(name)
		return nil, ioError(name, "Write the installation lock and retry.", err)
	}
	if err := file.Close(); err != nil {
		_ = m.fs.remove(name)
		return nil, ioError(name, "Close the installation lock and retry.", err)
	}
	return func(original error, committed bool) error {
		if err := m.fs.remove(name); err != nil {
			typed := &Error{Code: "SKILL_RECOVERY_REQUIRED", Message: "The installation lock could not be removed. Preserve the installation and any staging or backup copies; verify no operation is running before removing the lock and retrying.", Path: name, LockPath: name, RecoveryRequired: true, RecoveryPath: name, Committed: committed, err: err}
			var prior *Error
			if errors.As(original, &prior) {
				typed.Committed = typed.Committed || prior.Committed
				typed.BackupPath = prior.BackupPath
				if prior.RecoveryPath != "" {
					typed.RecoveryPath = prior.RecoveryPath
				}
			}
			return typed
		}
		return original
	}, nil
}

func (m manager) backup(location Location, inspection inspected) (string, error) {
	name, err := os.MkdirTemp(filepath.Dir(filepath.Dir(location.Path)), "todoist-cli-backup-")
	if err != nil {
		return "", ioError(location.Path, "Create a new sibling backup directory and retry.", err)
	}
	names := append([]string{ManifestName}, inspection.modified...)
	for _, relative := range names {
		before := inspection.snapshot[relative]
		full := filepath.Join(name, filepath.FromSlash(relative))
		err := os.MkdirAll(filepath.Dir(full), 0o700)
		if err == nil {
			err = m.fs.writeFile(full, before.data, before.mode)
		}
		if err == nil {
			continue
		}
		if cleanupErr := m.fs.removeAll(name); cleanupErr != nil {
			return name, &Error{Code: "SKILL_RECOVERY_REQUIRED", Message: "The backup was incomplete and could not be removed. The installed skill is unchanged; preserve and inspect the backup directory before retrying.", Path: name, RecoveryRequired: true, RecoveryPath: name, err: err}
		}
		return "", ioError(full, "The backup could not be completed; the installed skill was preserved. Check directory access and retry.", err)
	}
	return name, nil
}

// change stages both replacements and rollback copies outside the loadable
// skills directory. Per-file replacement and manifest publication are not a
// crash/power-loss transaction. A failed rollback retains recovery copies.
func (m manager) change(location Location, before, desired map[string]snapshot) error {
	stage, err := os.MkdirTemp(filepath.Dir(filepath.Dir(location.Path)), ".todoist-cli-stage-")
	if err != nil {
		return ioError(location.Path, "Create a staging directory and retry.", err)
	}
	cleanup := func(original error, committed bool) error {
		if err := m.fs.removeAll(stage); err != nil {
			return &Error{Code: "SKILL_RECOVERY_REQUIRED", Message: "The operation's staging directory could not be removed. Inspect the installation and preserve the staging directory before cleanup.", Path: location.Path, Committed: committed, RecoveryRequired: true, RecoveryPath: stage, err: original}
		}
		return original
	}
	for _, name := range sortedKeys(desired) {
		for directory, value := range map[string]snapshot{"new": desired[name], "old": before[name]} {
			if !value.exists {
				continue
			}
			full := filepath.Join(stage, directory, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
				return cleanup(ioError(full, "Staging failed before changing owned files. Check directory access and retry.", err), false)
			}
			if err := m.fs.writeFile(full, value.data, value.mode); err != nil {
				return cleanup(ioError(full, "Staging failed before changing owned files. Check directory access and retry.", err), false)
			}
		}
	}
	names := sortedKeys(desired)
	// Publish/remove the ownership manifest last, so it never claims a completed
	// operation while managed files are still being replaced.
	names = append(without(names, ManifestName), ManifestName)
	var changed []string
	for _, name := range names {
		full, err := safeFile(location.Path, name)
		if err == nil {
			var current snapshot
			current, err = readSnapshot(full)
			if err == nil && (current.exists != before[name].exists || !bytes.Equal(current.data, before[name].data)) {
				err = &Error{Code: "SKILL_CONFLICT", Message: "A managed destination changed during the operation. Preserve the user's changes and retry after other edits finish.", Path: full, Files: []string{name}}
			}
		}
		if err == nil && desired[name].exists {
			err = os.MkdirAll(filepath.Dir(full), 0o755)
			if err == nil {
				err = m.fs.rename(filepath.Join(stage, "new", filepath.FromSlash(name)), full)
			}
		} else if err == nil && before[name].exists {
			err = m.fs.remove(full)
		}
		if err != nil {
			for i := len(changed) - 1; i >= 0; i-- {
				relative := changed[i]
				target, rollbackErr := safeFile(location.Path, relative)
				if rollbackErr == nil {
					var current snapshot
					current, rollbackErr = readSnapshot(target)
					if rollbackErr == nil && (current.exists != desired[relative].exists || !bytes.Equal(current.data, desired[relative].data)) {
						rollbackErr = errors.New("a published file changed before rollback")
					}
				}
				if rollbackErr != nil {
					return &Error{Code: "SKILL_RECOVERY_REQUIRED", Message: "The operation failed and a published destination changed before rollback. Preserve the user's changes and staging copies; inspect both before retrying.", Path: location.Path, RecoveryRequired: true, RecoveryPath: stage, err: rollbackErr}
				}
				if before[relative].exists {
					rollbackErr = m.fs.rename(filepath.Join(stage, "old", filepath.FromSlash(relative)), target)
				} else {
					rollbackErr = m.fs.remove(target)
				}
				if rollbackErr != nil {
					return &Error{Code: "SKILL_RECOVERY_REQUIRED", Message: "The operation failed and rollback could not restore every owned file. Preserve the installation and staging copies; inspect both before retrying.", Path: location.Path, RecoveryRequired: true, RecoveryPath: stage, err: rollbackErr}
				}
			}
			_ = m.removeEmptyDirectories(location.Path, names)
			var typed *Error
			if !errors.As(err, &typed) {
				err = ioError(full, "The skill operation failed; owned files were restored. Check file access and retry.", err)
			}
			return cleanup(err, false)
		}
		if desired[name].exists || before[name].exists {
			changed = append(changed, name)
		}
	}
	return cleanup(nil, true)
}

func without(values []string, excluded string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != excluded {
			out = append(out, value)
		}
	}
	return out
}

func (m manager) removeEmptyDirectories(root string, files []string) error {
	directories := map[string]bool{root: true}
	for _, name := range files {
		for current := filepath.Dir(filepath.Join(root, filepath.FromSlash(name))); current != root; current = filepath.Dir(current) {
			directories[current] = true
		}
	}
	names := sortedKeys(directories)
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })
	for _, name := range names {
		entries, err := os.ReadDir(name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			if err := m.fs.remove(name); err != nil {
				return err
			}
		}
	}
	return nil
}

func retainedFiles(root string) ([]string, error) {
	files := []string{}
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) && name == root {
			return nil
		}
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			relative, err := filepath.Rel(root, name)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(relative))
		}
		return nil
	})
	if err != nil {
		return files, ioError(root, "Inspect retained files in the skill directory.", err)
	}
	sort.Strings(files)
	return files, nil
}
