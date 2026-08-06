package hook

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
)

// RunResult is a helper binary's output. ExitCode is a result, not an error:
// scanners and linters routinely signal findings with a non-zero exit.
type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Exec runs bin with args under the rule's context. ok is false only when the
// binary is missing or could not start: a rule leaning on a tool the user
// lacks should fall silent, not report every absent linter.
func Exec(ctx context.Context, stdin, bin string, args ...string) (RunResult, bool) {
	if _, err := exec.LookPath(bin); err != nil {
		return RunResult{}, false
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf

	err := cmd.Run()
	res := RunResult{Stdout: out.String(), Stderr: errBuf.String()}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		res.ExitCode = exitErr.ExitCode()
	default:
		// Killed by the deadline, or never started — no trustworthy result.
		return RunResult{}, false
	}
	return res, true
}

// Installed reports whether a helper binary is on PATH, so a rule can skip work
// rather than shelling out to find out.
func Installed(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

// MissingBinary is the standard finding when an enabled rule lacks a program.
// Enabling a rule is an explicit request for it to run, so this fails loudly:
// a check you believe is running but isn't is worse than no check.
func MissingBinary(rule, bin, install string) Finding {
	return Finding{Rule: rule, Message: `
		` + bin + ` is not installed, but the ` + rule + ` rule is enabled and
		needs it — so this check did NOT run. Install it (` + install + `) or
		disable the rule in ollie-hooks config.toml.`}
}
