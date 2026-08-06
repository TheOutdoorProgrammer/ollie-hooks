package hook

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX shell")
	}
}

// A child killed by the deadline must not be reported as a successful run.
// It surfaces as an ordinary *exec.ExitError carrying partial output, so the
// obvious error-classifying switch accepts a truncated scan as a clean one.
func TestExecRejectsADeadlineKill(t *testing.T) {
	skipOnWindows(t)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	res, ok := Exec(ctx, "", "sh", "-c", "echo PARTIAL; sleep 5")
	if ok {
		t.Fatalf("ok = true for a killed child; got output %q", res.Stdout)
	}
	if res.Stdout != "" {
		t.Errorf("partial output leaked to the caller: %q", res.Stdout)
	}
}

func TestExecRejectsAnAlreadyCancelledContext(t *testing.T) {
	skipOnWindows(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, ok := Exec(ctx, "", "sh", "-c", "echo hi"); ok {
		t.Error("ok = true for a cancelled context")
	}
}

// A non-zero exit is a result, not a failure: scanners and linters report
// findings that way, and treating it as an error would silence them.
func TestExecKeepsANonZeroExit(t *testing.T) {
	skipOnWindows(t)

	res, ok := Exec(context.Background(), "", "sh", "-c", "echo found; exit 3")
	if !ok {
		t.Fatal("ok = false for a completed run that exited non-zero")
	}
	if res.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", res.ExitCode)
	}
	if res.Stdout != "found\n" {
		t.Errorf("Stdout = %q, want %q", res.Stdout, "found\n")
	}
}

func TestExecReportsAMissingBinary(t *testing.T) {
	if _, ok := Exec(context.Background(), "", "ollie-hooks-no-such-binary-xyz"); ok {
		t.Error("ok = true for a binary that is not on PATH")
	}
}

func TestExecPassesStdin(t *testing.T) {
	skipOnWindows(t)

	res, ok := Exec(context.Background(), "hello\n", "cat")
	if !ok {
		t.Fatal("ok = false")
	}
	if res.Stdout != "hello\n" {
		t.Errorf("Stdout = %q, want %q", res.Stdout, "hello\n")
	}
}

func TestExecCapturesStderr(t *testing.T) {
	skipOnWindows(t)

	res, ok := Exec(context.Background(), "", "sh", "-c", "echo oops >&2")
	if !ok {
		t.Fatal("ok = false")
	}
	if res.Stderr != "oops\n" {
		t.Errorf("Stderr = %q, want %q", res.Stderr, "oops\n")
	}
}
