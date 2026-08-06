package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/BurntSushi/toml"
)

// projectConfigFile is the per-repo config, discovered by walking up from the
// working directory. One file, like the user config.
const projectConfigFile = ".ollie-hooks.toml"

// floorRules take config from the user file only, always. Credential scanning
// must not be disablable or downgradable by any repo; trusting a repo says "run
// its rules", not "turn off my secret scanning".
var floorRules = map[string]bool{"secret-scan": true, "secret-redact": true}

// isFloorRule reports whether project config is forbidden from touching a rule.
// A floored rule, or any rule that runs an external program, is user-only: a
// trusted repo must never point ollie-hooks at an arbitrary binary (adr/0005).
func isFloorRule(id string) bool {
	if floorRules[id] {
		return true
	}
	for _, r := range registry {
		if r.ID == id {
			return len(r.RequiresBinaries) > 0 || r.Binaries != nil
		}
	}
	return false
}

// ProjectConfigDir returns the directory of the nearest .ollie-hooks.toml
// walking up from the working directory, or "" when none exists. The trust
// command trusts this directory.
func ProjectConfigDir() string {
	_, root := findProjectConfig()
	return root
}

// findProjectConfig returns the nearest .ollie-hooks.toml and its directory,
// walking up from the working directory. Empty when none exists.
func findProjectConfig() (path, root string) {
	dir, err := os.Getwd()
	if err != nil {
		return "", ""
	}
	for {
		p := filepath.Join(dir, projectConfigFile)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ""
		}
		dir = parent
	}
}

type projectRoot struct {
	Rules       map[string]toml.Primitive `toml:"rules"`
	CustomRules map[string]CustomRule     `toml:"custom_rules"`
}

var (
	projectOnce  sync.Once
	projectData  projectRoot
	projectMD    toml.MetaData
	projectOK    bool
	untrustOnce     sync.Once
	malformedPrj    sync.Once
	customRulesOnce sync.Once
)

// loadProjectRoot decodes the trusted project config once per process. It is a
// no-op when no config exists or the repo is untrusted, so an untrusted repo
// contributes nothing to what the gate does.
func loadProjectRoot() (toml.MetaData, projectRoot, bool) {
	projectOnce.Do(func() {
		path, root := findProjectConfig()
		if path == "" {
			return
		}
		if !IsTrusted(root) {
			untrustNotice(path)
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		md, err := toml.Decode(string(data), &projectData)
		if err != nil {
			malformedPrj.Do(func() {
				fmt.Fprintf(os.Stderr, "ollie-hooks: %s does not parse, so its project "+
					"config was ignored: %v\n", path, err)
			})
			return
		}
		projectMD, projectOK = md, true
		if len(projectData.CustomRules) > 0 {
			customRulesOnce.Do(func() {
				fmt.Fprintf(os.Stderr, "ollie-hooks: %s defines custom rules, which project "+
					"config never runs for safety; define them in your user config.\n", path)
			})
		}
	})
	return projectMD, projectData, projectOK
}

// projectRuleSection returns a trusted project's section for a rule, unless the
// rule is floored. found is false when the project does not configure it.
func projectRuleSection(id string) (toml.MetaData, toml.Primitive, bool) {
	if isFloorRule(id) {
		return toml.MetaData{}, toml.Primitive{}, false
	}
	md, root, ok := loadProjectRoot()
	if !ok {
		return toml.MetaData{}, toml.Primitive{}, false
	}
	section, found := root.Rules[id]
	return md, section, found
}

// untrustNotice tells the user once that a project config exists but is dormant
// until trusted. Silence would look like the file was read and did nothing.
func untrustNotice(path string) {
	untrustOnce.Do(func() {
		fmt.Fprintf(os.Stderr, "ollie-hooks: found %s but this repo is not trusted, "+
			"so its config was ignored. Run `ollie-hooks trust` to apply it.\n", path)
	})
}
