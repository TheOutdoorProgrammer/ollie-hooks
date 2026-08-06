package hook

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

// waitDelay is the grace period between killing a child and giving up on the
// output pipes a surviving grandchild may still hold open.
const waitDelay = time.Second

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
	// Killing the child does not kill grandchildren it spawned, and they inherit
	// the output pipe Run waits on — so without this, `sh -c 'lint & '` outlives
	// its deadline by however long the grandchild runs.
	cmd.WaitDelay = waitDelay
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf

	err := cmd.Run()

	// Checked first: a deadline-killed child reports an ordinary *exec.ExitError
	// ("signal: killed") carrying whatever it wrote before dying, so classifying
	// the error first would accept partial output as a complete result.
	if ctx.Err() != nil {
		return RunResult{}, false
	}

	res := RunResult{Stdout: out.String(), Stderr: errBuf.String()}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		res.ExitCode = exitErr.ExitCode()
	default:
		// Never started, or died in a way that leaves no exit status.
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
