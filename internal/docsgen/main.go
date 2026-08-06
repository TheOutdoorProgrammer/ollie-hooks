// Command docsgen writes the generated documentation: the full rule reference,
// and the shipped-rules table spliced into README.md between its markers.
// CI runs it and diffs, so a rule added without docs fails the build.
package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook"
	"github.com/TheOutdoorProgrammer/ollie-hooks/rules"
)

const (
	referencePath = "docs/rules.md"
	examplePath   = "config.example.toml"
	readmePath    = "README.md"
	readmeMarker  = "ollie-hooks:rules"
)

func main() {
	rules.RegisterAll()

	var reference bytes.Buffer
	hook.WriteRuleDocs(&reference)
	write(referencePath, reference.Bytes())

	// Committed as well as printable, so it can be read on the repo page by
	// someone deciding whether to install this at all.
	var example bytes.Buffer
	hook.WriteConfigExample(&example)
	write(examplePath, example.Bytes())

	var summary bytes.Buffer
	hook.WriteRuleSummary(&summary)
	splice(readmePath, readmeMarker, summary.String())
}

func write(path string, data []byte) {
	if err := os.WriteFile(path, data, 0o644); err != nil {
		fail("writing %s: %v", path, err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", path)
}

func splice(path, marker, body string) {
	doc, err := os.ReadFile(path)
	if err != nil {
		fail("reading %s: %v", path, err)
	}
	updated, err := hook.ReplaceSection(string(doc), marker, body)
	if err != nil {
		fail("%s: %v", path, err)
	}
	write(path, []byte(updated))
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "docsgen: "+format+"\n", args...)
	os.Exit(1)
}
