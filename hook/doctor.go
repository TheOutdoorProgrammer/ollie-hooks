package hook

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"

	"github.com/BurntSushi/toml"
)

// RuleStatus is one rule's effective state after config resolves. Severity is
// meaningful only for Check rules; the others leave it empty.
type RuleStatus struct {
	ID       string
	Enabled  bool
	Severity Severity
	IsCheck  bool
}

// BinaryGap is an enabled rule whose required tool is not on PATH, so the rule
// reports instead of running.
type BinaryGap struct {
	Rule    string
	Bin     string
	Install string
}

// Diagnosis is everything doctor found about a setup. It is data, not output,
// so the same result can be rendered or asserted in a test.
type Diagnosis struct {
	ConfigPath   string
	ConfigExists bool
	ConfigParses bool
	ConfigError  string
	// StrayKeys are config keys no rule or framework section reads: a typo like
	// max_commet_lines that is silently ignored today.
	StrayKeys []string
	// UnknownRules are [rules.<id>] sections whose id is not registered.
	UnknownRules []string

	Wiring          WiringDrift
	MissingBinaries []BinaryGap
	Rules           []RuleStatus

	TrustedProjects []string
	ProjectDir      string
	ProjectTrusted  bool
}

// OK reports whether doctor found no problems, which is the CI-usable signal.
// An untrusted project config is informational, not a failure.
func (d Diagnosis) OK() bool {
	if d.ConfigExists && !d.ConfigParses {
		return false
	}
	return len(d.StrayKeys) == 0 &&
		len(d.UnknownRules) == 0 &&
		d.Wiring.Clean() &&
		len(d.MissingBinaries) == 0
}

// Diagnose inspects the live setup: user config, wiring, binaries, effective
// rule state, and project trust. It reads files but changes nothing.
func Diagnose() Diagnosis {
	d := Diagnosis{Wiring: CheckWiring()}
	d.ConfigPath, d.ConfigExists, d.ConfigParses, d.ConfigError, d.StrayKeys, d.UnknownRules = diagnoseConfig()

	for _, r := range registry {
		enabled := RuleEnabled(r.ID, r.EnabledByDefault)
		st := RuleStatus{ID: r.ID, Enabled: enabled, IsCheck: r.Check != nil}
		if st.IsCheck {
			st.Severity = r.severity()
		}
		d.Rules = append(d.Rules, st)
		if ruleActive(r, enabled, st.Severity) {
			for _, b := range MissingBinaries(r.needs()) {
				d.MissingBinaries = append(d.MissingBinaries, BinaryGap{Rule: r.ID, Bin: b.Bin, Install: b.Install})
			}
		}
	}

	d.TrustedProjects = TrustedProjects()
	d.ProjectDir = ProjectConfigDir()
	if d.ProjectDir != "" {
		d.ProjectTrusted = IsTrusted(d.ProjectDir)
	}
	return d
}

// ruleActive reports whether a rule would run, so doctor only demands binaries
// for checks that are actually live. A Check silenced to "off" is as inert as a
// disabled rule and must not nag about its tools.
func ruleActive(r Rule, enabled bool, sev Severity) bool {
	if !enabled {
		return false
	}
	if r.Check != nil && sev == SeverityOff {
		return false
	}
	return true
}

// diagnoseConfig strict-parses the user config: it decodes every known section
// so any key left over is one nothing reads. Unknown [rules.<id>] sections are
// reported by id and consumed, so their inner keys do not read as stray.
func diagnoseConfig() (path string, exists, parses bool, errStr string, stray, unknown []string) {
	path = configPath(configFile)
	if path == "" {
		return "", false, true, "", nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return path, false, true, "", nil, nil
	}
	var root struct {
		Rules       map[string]toml.Primitive `toml:"rules"`
		Output      toml.Primitive            `toml:"output"`
		CustomRules map[string]CustomRule     `toml:"custom_rules"`
	}
	md, err := toml.Decode(string(data), &root)
	if err != nil {
		return path, true, false, err.Error(), nil, nil
	}

	known := map[string]any{}
	for _, r := range registry {
		known[r.ID] = r.Config
	}
	for id, section := range root.Rules {
		cfg, isKnown := known[id]
		if !isKnown {
			unknown = append(unknown, id)
			var junk map[string]any
			_ = md.PrimitiveDecode(section, &junk)
			continue
		}
		var universal struct {
			Enabled  *bool   `toml:"enabled"`
			Severity *string `toml:"severity"`
			Timeout  *int    `toml:"timeout"`
		}
		_ = md.PrimitiveDecode(section, &universal)
		if cfg != nil {
			_ = md.PrimitiveDecode(section, reflect.New(reflect.TypeOf(cfg)).Interface())
		}
	}
	var out struct {
		WrapWidth int `toml:"wrap_width"`
	}
	_ = md.PrimitiveDecode(root.Output, &out)

	for _, k := range md.Undecoded() {
		stray = append(stray, k.String())
	}
	sort.Strings(stray)
	sort.Strings(unknown)
	return path, true, true, "", stray, unknown
}

// WriteDoctor renders a diagnosis in scannable sections and reports whether it
// is clean, so the CLI can exit non-zero when there is something to fix.
func WriteDoctor(w io.Writer) bool {
	d := Diagnose()
	p := func(format string, args ...any) { _, _ = fmt.Fprintf(w, format+"\n", args...) }

	doctorConfigSection(p, d)
	doctorWiringSection(p, d)
	doctorBinariesSection(p, d)
	doctorRulesSection(p, d)
	doctorTrustSection(p, d)

	p("")
	if d.OK() {
		p("%s", Paint("All clear.", DraculaGreen))
	} else {
		p("%s", Paint("Problems found (see the marked lines above).", DraculaRed))
	}
	return d.OK()
}

// docSection prints a coloured section header with a blank line above it.
func docSection(p func(string, ...any), title string) {
	p("")
	p("%s", Paint("── "+title+" ──", DraculaPurple))
}

func doctorConfigSection(p func(string, ...any), d Diagnosis) {
	docSection(p, "Config")
	if !d.ConfigExists {
		p("%s", Paint("✓", DraculaGreen)+" no config file at "+d.ConfigPath+" (all rules on defaults)")
		return
	}
	if !d.ConfigParses {
		p("%s", Paint("✗ "+d.ConfigPath+" does not parse", DraculaRed))
		p("  %s", d.ConfigError)
		return
	}
	p("%s", Paint("✓", DraculaGreen)+" "+d.ConfigPath+" parses")
	for _, id := range d.UnknownRules {
		p("  %s", Paint("✗ [rules."+id+"] is not a registered rule", DraculaRed))
	}
	for _, k := range d.StrayKeys {
		p("  %s", Paint("✗ "+k+" is set but nothing reads it", DraculaRed))
	}
	if len(d.UnknownRules) == 0 && len(d.StrayKeys) == 0 {
		p("  %s", Paint("✓ every key is read by a rule", DraculaGreen))
	}
}

func doctorWiringSection(p func(string, ...any), d Diagnosis) {
	docSection(p, "Wiring")
	if !d.Wiring.SettingsFound {
		p("  %s", Paint("✗ settings.json not found at "+d.Wiring.SettingsPath, DraculaRed))
	}
	if d.Wiring.Clean() {
		p("  %s", Paint("✓ every event with rules is wired", DraculaGreen))
		return
	}
	p("  %s", Paint("✗ events with rules that are NOT wired:", DraculaRed))
	for _, e := range d.Wiring.Missing {
		p("    %s", Paint(string(e), DraculaOrange))
	}
	p("  Fix: run `ollie-hooks wiring` and append the entries to settings.json.")
}

func doctorBinariesSection(p func(string, ...any), d Diagnosis) {
	docSection(p, "Binaries")
	if len(d.MissingBinaries) == 0 {
		p("  %s", Paint("✓ every enabled rule has the tools it needs", DraculaGreen))
		return
	}
	for _, g := range d.MissingBinaries {
		p("  %s", Paint("✗ "+g.Rule+" needs "+g.Bin, DraculaRed)+" (install: "+g.Install+")")
	}
}

func doctorRulesSection(p func(string, ...any), d Diagnosis) {
	docSection(p, "Rules")
	for _, r := range d.Rules {
		state, colour := "disabled", DraculaComment
		if r.Enabled {
			state, colour = "enabled", DraculaGreen
		}
		sev := ""
		if r.IsCheck && r.Enabled {
			sev = "  severity=" + string(r.Severity)
		}
		p("  %-22s %s%s", r.ID, Paint(state, colour), sev)
	}
}

func doctorTrustSection(p func(string, ...any), d Diagnosis) {
	docSection(p, "Projects")
	if len(d.TrustedProjects) == 0 {
		p("  no trusted project configs")
	} else {
		p("  trusted:")
		for _, t := range d.TrustedProjects {
			p("    %s", t)
		}
	}
	if d.ProjectDir == "" {
		p("  this dir has no .ollie-hooks.toml")
		return
	}
	if d.ProjectTrusted {
		p("  %s", Paint("✓ this project's .ollie-hooks.toml is trusted and applied", DraculaGreen))
		return
	}
	p("  %s", Paint("! "+d.ProjectDir+" has a .ollie-hooks.toml but is untrusted", DraculaOrange))
	p("  Run `ollie-hooks trust` here to apply it.")
}
