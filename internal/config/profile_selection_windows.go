//go:build windows

package config

// Windows cannot sync an ordinary directory handle with os.File.Sync. The
// replacement file is flushed before rename; replacement failures stay explicit.
func syncSelectionDirectory(string) error { return nil }
