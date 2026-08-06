package hook

import (
	"fmt"
	"reflect"
	"strings"
)

// KeyDoc is one configurable key: its TOML name, what it does, and the default
// the rule actually runs with. Table is non-empty when the key is a nested
// table, which TOML writes as [[rules.<id>.<key>]].
type KeyDoc struct {
	Key     string
	Doc     string
	Default any
	Table   []KeyDoc
}

// Registered returns the rules in registration order. Exported because the docs
// commands need it, and because a framework whose registry cannot be read back
// forces every embedder to keep a parallel list.
func Registered() []Rule {
	out := make([]Rule, len(registry))
	copy(out, registry)
	return out
}

// ConfigKeys describes the rule's own configurable keys, excluding the
// universal ones every rule shares (see UniversalKeys).
func (r Rule) ConfigKeys() []KeyDoc {
	if r.Config == nil {
		return nil
	}
	return keysOf(reflect.ValueOf(r.Config))
}

// UniversalKeys are the keys the framework itself reads for every rule, with
// this rule's defaults filled in. They live here rather than in each rule's
// config struct because the framework, not the rule, implements them.
func (r Rule) UniversalKeys() []KeyDoc {
	keys := []KeyDoc{{
		Key:     "enabled",
		Doc:     "turn the rule on",
		Default: r.EnabledByDefault,
	}}
	if r.Check != nil {
		sev := r.DefaultSeverity
		if sev == "" {
			sev = SeverityBlock
		}
		keys = append(keys, KeyDoc{
			Key:     "severity",
			Doc:     `how findings arrive: "block", "advisory" or "off"`,
			Default: string(sev),
		})
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = defaultRuleTimeout
	}
	return append(keys, KeyDoc{
		Key:     "timeout",
		Doc:     "seconds this rule may take before it is abandoned",
		Default: timeout,
	})
}

// keysOf reflects a config value into its documented keys. It descends into
// nested structs and slices of structs, which is what makes a table like
// no-codegen's [[paths]] describable without special-casing it.
func keysOf(v reflect.Value) []KeyDoc {
	v = reflect.Indirect(v)
	if v.Kind() != reflect.Struct {
		return nil
	}

	var out []KeyDoc
	t := v.Type()
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		kd := KeyDoc{
			Key:     tomlName(f),
			Doc:     f.Tag.Get("doc"),
			Default: v.Field(i).Interface(),
		}
		if elem, ok := structElem(f.Type); ok {
			// A zero element still carries the tags, which is all the nested
			// keys need — the defaults of a table nobody configured are moot.
			kd.Table = keysOf(reflect.New(elem).Elem())
		}
		out = append(out, kd)
	}
	return out
}

// structElem reports the struct type behind a field that is a struct, or a
// slice of structs, so both document their inner keys the same way.
func structElem(t reflect.Type) (reflect.Type, bool) {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	if t.Kind() == reflect.Struct {
		return t, true
	}
	return nil, false
}

// tomlName is the field's TOML key, falling back to the lowercased Go name so
// an untagged field is still described rather than silently skipped.
func tomlName(f reflect.StructField) string {
	tag := f.Tag.Get("toml")
	if name, _, _ := strings.Cut(tag, ","); name != "" {
		return name
	}
	return strings.ToLower(f.Name)
}

// checkConfigTags rejects a Config whose fields are not fully documented. This
// runs at Register, so an undocumented key fails in the rule package's own
// tests rather than shipping a config nobody can discover.
func checkConfigTags(id string, cfg any) error {
	if cfg == nil {
		return nil
	}
	v := reflect.Indirect(reflect.ValueOf(cfg))
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("Config must be a struct, got %s", v.Kind())
	}
	return checkStructTags(id, v.Type(), "")
}

func checkStructTags(id string, t reflect.Type, prefix string) error {
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name := prefix + tomlName(f)
		if f.Tag.Get("toml") == "" {
			return fmt.Errorf("field %s needs a toml tag", name)
		}
		if f.Tag.Get("doc") == "" {
			return fmt.Errorf("field %s needs a doc tag, or it cannot be documented", name)
		}
		if elem, ok := structElem(f.Type); ok {
			if err := checkStructTags(id, elem, name+"."); err != nil {
				return err
			}
		}
	}
	return nil
}
