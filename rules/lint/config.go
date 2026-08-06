package lint

import (
	"slices"
	"sort"
	"strings"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
)

// config selects which linters run. This rule dispatches to ten tools by file
// extension, so all-or-nothing enabling would demand every one of them from
// every user.
type config struct {
	// A non-empty list is a commitment, which is why a named-but-absent linter
	// is reported rather than skipped.
	Linters []string `toml:"linters" doc:"which linters to run, by command name; empty means every one you have installed"`
}

// installHints is how to get each linter. Only two are not brew formulae, and
// telling someone to "install markdownlint-cli2" without saying npm is the kind
// of unhelpful that makes a person disable the rule.
var installHints = map[string]string{
	"markdownlint-cli2": "npm install -g markdownlint-cli2",
	"ruff":              "brew install ruff",
	"shellcheck":        "brew install shellcheck",
	"yamllint":          "brew install yamllint",
	"selene":            "brew install selene",
	"golangci-lint":     "brew install golangci-lint",
	"swiftlint":         "brew install swiftlint",
	"tofu":              "brew install opentofu",
	"hadolint":          "brew install hadolint",
	"zsh":               "already on macOS; apt install zsh elsewhere",
}

func installHint(cmd string) string {
	if h, ok := installHints[cmd]; ok {
		return h
	}
	return "install " + cmd
}

func defaultConfig() config { return config{} }

func loadConfig() config { return hook.Config(RuleID, defaultConfig()) }

// selected reports whether a linter should run, and whether the user named it
// explicitly. Naming one is what turns a silent skip into a reported failure.
func (c config) selected(cmd string) (run, named bool) {
	if len(c.Linters) == 0 {
		return true, false
	}
	named = slices.Contains(c.Linters, cmd)
	return named, named
}

// docTable lists the legal values for `linters`, built from the dispatch table
// itself so a linter added to one is never missing from the other.
func docTable() hook.DocTable {
	files := map[string][]string{}
	for ext, spec := range lintersByExt {
		files[spec.cmd] = append(files[spec.cmd], ext)
	}

	cmds := make([]string, 0, len(files))
	for cmd := range files {
		cmds = append(cmds, cmd)
	}
	sort.Strings(cmds)

	// Dockerfiles are matched by filename, so they are not in the extension map
	// and would otherwise be missing from a table claiming to be the whole list.
	files[dockerfileLinter.cmd] = []string{"Dockerfile", "*.Dockerfile"}
	cmds = append(cmds, dockerfileLinter.cmd)
	sort.Strings(cmds)

	rows := make([][]string, 0, len(cmds))
	for _, cmd := range cmds {
		exts := files[cmd]
		sort.Strings(exts)
		rows = append(rows, []string{"`" + cmd + "`", "`" + strings.Join(exts, "`, `") + "`", installHint(cmd)})
	}

	return hook.DocTable{
		Title:   "What it can lint",
		Headers: []string{"Name to use in `linters`", "Files", "Install"},
		Rows:    rows,
		Legend:  "names you can put in linters: " + strings.Join(cmds, ", "),
	}
}
