package skillinstall

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

type manifest struct {
	FormatVersion int               `json:"format_version"`
	Target        string            `json:"target"`
	Scope         string            `json:"scope"`
	Version       string            `json:"version"`
	PackageDigest string            `json:"package_digest"`
	Files         map[string]string `json:"files"`
}

type snapshot struct {
	data   []byte
	mode   os.FileMode
	exists bool
}

type inspected struct {
	manifest *manifest
	snapshot map[string]snapshot
	modified []string
	missing  []string
}

func Inspect(location Location) (State, error) {
	location, err := ValidateLocation(location)
	if err != nil {
		return State{}, err
	}
	state := State{Target: location.Target, Scope: location.Scope, Path: location.Path, Modified: []string{}, Missing: []string{}}
	inspection, err := inspect(location)
	if err != nil {
		return state, err
	}
	if inspection.manifest == nil {
		if _, err := os.Lstat(filepath.Join(location.Path, "SKILL.md")); err == nil {
			return state, &Error{Code: "SKILL_CONFLICT", Message: "An unowned skill already exists here. Choose another destination or move it before installing.", Path: location.Path}
		} else if !errors.Is(err, os.ErrNotExist) {
			return state, ioError(location.Path, "Inspect the unowned destination and retry.", err)
		}
		return state, nil
	}
	state.Installed = true
	state.PackageDigest = inspection.manifest.PackageDigest
	state.Version = inspection.manifest.Version
	state.Modified = inspection.modified
	state.Missing = inspection.missing
	return state, nil
}

func inspect(location Location) (inspected, error) {
	result := inspected{snapshot: map[string]snapshot{}, modified: []string{}, missing: []string{}}
	manifestPath, err := safeFile(location.Path, ManifestName)
	if err != nil {
		return result, err
	}
	before, err := readSnapshot(manifestPath)
	if err != nil {
		return result, err
	}
	result.snapshot[ManifestName] = before
	if !before.exists {
		return result, nil
	}
	m, err := decodeManifest(before.data, location)
	if err != nil {
		return result, err
	}
	result.manifest = &m
	for _, name := range sortedKeys(m.Files) {
		full, err := safeFile(location.Path, name)
		if err != nil {
			return result, err
		}
		before, err := readSnapshot(full)
		if err != nil {
			return result, err
		}
		result.snapshot[name] = before
		if !before.exists {
			result.missing = append(result.missing, name)
		} else if fileDigest(before.data) != m.Files[name] {
			result.modified = append(result.modified, name)
		}
	}
	return result, nil
}

func readSnapshot(name string) (snapshot, error) {
	info, err := os.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return snapshot{}, nil
	}
	if err != nil {
		return snapshot{}, ioError(name, "Read the managed file's metadata and retry.", err)
	}
	if !info.Mode().IsRegular() {
		return snapshot{}, pathError(name, "Managed files must be regular files, not directories, symlinks, or devices.")
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return snapshot{}, ioError(name, "Read the managed file and retry.", err)
	}
	return snapshot{data: data, mode: info.Mode().Perm(), exists: true}, nil
}

func decodeManifest(data []byte, location Location) (manifest, error) {
	invalid := func() (manifest, error) {
		return manifest{}, &Error{Code: "SKILL_MANIFEST_INVALID", Message: "The skill ownership manifest is invalid or unsupported. Preserve this directory and restore a valid manifest before updating or uninstalling.", Path: filepath.Join(location.Path, ManifestName)}
	}
	if len(data) > 1024*1024 || rejectDuplicateKeys(data) != nil {
		return invalid()
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return invalid()
	}
	allowed := map[string]bool{"format_version": true, "target": true, "scope": true, "version": true, "package_digest": true, "files": true}
	for field := range fields {
		if !allowed[field] {
			return invalid()
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var m manifest
	if err := decoder.Decode(&m); err != nil {
		return invalid()
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return invalid()
	}
	if m.FormatVersion != 1 || m.Target != location.Target || m.Scope != location.Scope || m.Version == "" || m.Files["SKILL.md"] == "" {
		return invalid()
	}
	if err := validateFileNames(sortedKeys(m.Files)); err != nil {
		return invalid()
	}
	for _, digest := range m.Files {
		decoded, err := hex.DecodeString(digest)
		if err != nil || len(decoded) != 32 || hex.EncodeToString(decoded) != digest {
			return invalid()
		}
	}
	if m.PackageDigest != hashesDigest(m.Files) {
		return invalid()
	}
	return m, nil
}

// The standard JSON decoder accepts duplicate keys. Ownership paths must never
// depend on which duplicate a decoder happens to keep.
func rejectDuplicateKeys(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	var value func() error
	value = func() error {
		token, err := d.Token()
		if err != nil {
			return err
		}
		if delimiter, ok := token.(json.Delim); ok {
			switch delimiter {
			case '{':
				keys := map[string]bool{}
				for d.More() {
					token, err := d.Token()
					if err != nil {
						return err
					}
					key, ok := token.(string)
					if !ok || keys[key] {
						return errors.New("duplicate manifest key")
					}
					keys[key] = true
					if err := value(); err != nil {
						return err
					}
				}
			case '[':
				for d.More() {
					if err := value(); err != nil {
						return err
					}
				}
			default:
				return errors.New("invalid manifest JSON")
			}
			_, err = d.Token()
			return err
		}
		return nil
	}
	if err := value(); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("trailing manifest JSON")
	}
	return nil
}
