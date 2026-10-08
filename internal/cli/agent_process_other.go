//go:build !unix

package cli

import "os/exec"

// Retain CommandContext's direct-process cancellation on other platforms.
// WaitDelay still bounds inherited output pipes; the planner requires /bin/sh.
func configurePlannerCancellation(cmd *exec.Cmd) {}
