package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// splitCommand tokenises a configured command line. There is no shell in the
// way, so this handles the two things people write anyway and would otherwise
// lose silently: quoted arguments containing spaces, and a leading ~.
func splitCommand(s string) ([]string, error) {
	var (
		fields  []string
		current strings.Builder
		quote   rune
		started bool
	)
	flush := func() {
		if started {
			fields = append(fields, expandHome(current.String()))
			current.Reset()
			started = false
		}
	}

	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
				continue
			}
			current.WriteRune(r)
		case r == '\'' || r == '"':
			quote = r
			started = true // "" is a real, empty argument
		case r == ' ' || r == '\t':
			flush()
		default:
			current.WriteRune(r)
			started = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unbalanced %c in command %q", quote, s)
	}
	flush()
	return fields, nil
}

// expandHome resolves a leading ~ against the user's home directory. Only a
// leading one, and only when it starts a path: a shell does the same, and
// anything cleverer would surprise someone who meant a literal tilde.
func expandHome(s string) string {
	if s != "~" && !strings.HasPrefix(s, "~/") && !strings.HasPrefix(s, `~\`) {
		return s
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return s
	}
	if s == "~" {
		return home
	}
	return filepath.Join(home, s[2:])
}
