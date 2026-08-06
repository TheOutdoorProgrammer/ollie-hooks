package comments

import (
	"os"
	"testing"
)

// The rule registers explicitly, so tests that exercise the registry have to
// do what cmd/ does. Registering once per binary keeps IDs unique.
func TestMain(m *testing.M) {
	Register()
	os.Exit(m.Run())
}
