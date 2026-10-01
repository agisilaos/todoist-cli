package skillinstall

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func targetDirectory(target string) string {
	for _, candidate := range Targets() {
		if target == candidate.Name {
			return candidate.Directory
		}
	}
	return ""
}

// ValidateLocation checks the explicit destination and canonicalizes ancestors
// above the target's agent directory. Symlinks inside that directory are refused.
// Scope labels describe placement intent; they do not establish loader behavior.
func ValidateLocation(location Location) (Location, error) {
	directory := targetDirectory(location.Target)
	if directory == "" {
		return Location{}, pathError(location.Path, "Choose a supported target: codex or claude-code.")
	}
	if location.Scope != "local" && location.Scope != "global" {
		return Location{}, pathError(location.Path, "Choose an explicit scope: local or global.")
	}
	if !filepath.IsAbs(location.Path) || strings.ContainsRune(location.Path, 0) {
		return Location{}, pathError(location.Path, "Use a full absolute skill destination path.")
	}
	for _, component := range strings.Split(filepath.ToSlash(location.Path), "/") {
		if component == ".." {
			return Location{}, pathError(location.Path, "The skill destination must not contain parent traversal.")
		}
	}
	clean := filepath.Clean(location.Path)
	agentDir := filepath.Dir(filepath.Dir(clean))
	if filepath.Base(clean) != "todoist-cli" || filepath.Base(filepath.Dir(clean)) != "skills" || filepath.Base(agentDir) != directory {
		return Location{}, pathError(location.Path, fmt.Sprintf("The %s destination must end in %s/skills/todoist-cli.", location.Target, directory))
	}
	anchor, err := canonicalAncestor(filepath.Dir(agentDir))
	if err != nil {
		return Location{}, ioError(location.Path, "Resolve the destination's existing ancestors and retry.", err)
	}
	location.Path = filepath.Join(anchor, directory, "skills", "todoist-cli")
	if err := safeDirectories(filepath.Join(anchor, directory), location.Path); err != nil {
		return Location{}, err
	}
	return location, nil
}

func canonicalAncestor(name string) (string, error) {
	current := name
	var tail []string
	for {
		_, err := os.Lstat(current)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			info, err := os.Stat(resolved)
			if err != nil {
				return "", err
			}
			if !info.IsDir() {
				return "", fmt.Errorf("ancestor is not a directory")
			}
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return resolved, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		next := filepath.Dir(current)
		if next == current {
			return "", err
		}
		tail = append(tail, filepath.Base(current))
		current = next
	}
}

func safeDirectories(first, last string) error {
	current := first
	for {
		info, err := os.Lstat(current)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return ioError(current, "Inspect destination directories and retry.", err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return pathError(current, "Agent and managed directories must be real directories, not symlinks or files.")
		}
		if current == last {
			return nil
		}
		relative, err := filepath.Rel(current, last)
		if err != nil || strings.HasPrefix(relative, "..") {
			return pathError(last, "The managed path must remain inside the selected agent directory.")
		}
		current = filepath.Join(current, strings.Split(relative, string(filepath.Separator))[0])
	}
}

func validateFileNames(names []string) error {
	seen := map[string]bool{}
	for _, name := range names {
		if name == "" || path.IsAbs(name) || path.Clean(name) != name || strings.ContainsAny(name, "\\\x00\r\n<>:\"|?*") || name == "." || strings.HasPrefix(name, "../") || strings.EqualFold(strings.Split(name, "/")[0], ManifestName) {
			return invalidBundle("The package contains an unsafe or reserved relative file path.")
		}
		for _, component := range strings.Split(name, "/") {
			if strings.HasSuffix(component, ".") || strings.HasSuffix(component, " ") {
				return invalidBundle("Package file paths must not end in dots or spaces.")
			}
			base := strings.ToUpper(strings.Split(component, ".")[0])
			if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
				return invalidBundle("Package file paths must not use reserved device names.")
			}
		}
		fold := strings.ToLower(name)
		if seen[fold] {
			return invalidBundle("Package file paths must be distinct on case-insensitive filesystems.")
		}
		seen[fold] = true
	}
	for name := range seen {
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if seen[parent] {
				return invalidBundle("A package file path cannot also be another file's directory.")
			}
		}
	}
	return nil
}

func safeFile(root, relative string) (string, error) {
	name := filepath.Join(root, filepath.FromSlash(relative))
	if err := safeDirectories(filepath.Dir(filepath.Dir(root)), filepath.Dir(name)); err != nil {
		return "", err
	}
	info, err := os.Lstat(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return name, nil
		}
		return "", ioError(name, "Inspect the managed file and retry.", err)
	}
	if !info.Mode().IsRegular() {
		return "", pathError(name, "Managed files must be regular files, not directories, symlinks, or devices.")
	}
	return name, nil
}

func pathError(name, message string) *Error {
	return &Error{Code: "SKILL_PATH_INVALID", Message: message, Path: name}
}

func ioError(name, message string, err error) *Error {
	return &Error{Code: "SKILL_IO", Message: message, Path: name, err: err}
}
