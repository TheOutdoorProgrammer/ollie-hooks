// Package rules wires the built-in rules into the registry.
//
// Registration is explicit rather than init()-driven: the enabled set is then
// greppable in one place, and nothing registers as a side effect of an import.
// Order matters — a SingleFailure rule that fires suppresses everything
// registered after it, so the most specific diagnosis goes first.
package rules

import (
	"github.com/TheOutdoorProgrammer/ollie-hooks/rules/bashoutput"
	"github.com/TheOutdoorProgrammer/ollie-hooks/rules/comments"
	"github.com/TheOutdoorProgrammer/ollie-hooks/rules/lint"
	"github.com/TheOutdoorProgrammer/ollie-hooks/rules/mermaid"
	"github.com/TheOutdoorProgrammer/ollie-hooks/rules/nocodegen"
	"github.com/TheOutdoorProgrammer/ollie-hooks/rules/prosewrap"
	"github.com/TheOutdoorProgrammer/ollie-hooks/rules/secretredact"
	"github.com/TheOutdoorProgrammer/ollie-hooks/rules/secretscan"
)

// RegisterAll adds every built-in rule to the registry.
func RegisterAll() {
	nocodegen.Register()
	secretscan.Register()
	secretredact.Register()
	lint.Register()
	comments.Register()
	bashoutput.Register()
	mermaid.Register()
	prosewrap.Register()
}
