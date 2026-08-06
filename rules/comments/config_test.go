package comments

import (
	"testing"

	"github.com/TheOutdoorProgrammer/ollie-hooks/hook/hooktest"
)

func writeConfig(t *testing.T, body string) {
	t.Helper()
	hooktest.Config(t, RuleID, body)
}

func TestLoadCommentConfigDefaults(t *testing.T) {
	hooktest.NoConfig(t)
	if cfg := loadConfig(); cfg != defaultConfig() {
		t.Errorf("no config file should yield defaults: %+v", cfg)
	}
}

func TestLoadCommentConfigOverride(t *testing.T) {
	writeConfig(t, "max_comment_lines = 5\nagent_memo = false\n")
	cfg := loadConfig()
	if cfg.MaxCommentLines != 5 || cfg.AgentMemo {
		t.Errorf("override not applied: %+v", cfg)
	}
	if cfg.MaxLineLength != 80 {
		t.Errorf("keys the file omits must keep defaults: %+v", cfg)
	}
}

func TestLoadCommentConfigMalformed(t *testing.T) {
	writeConfig(t, "this is not = valid = toml ][")
	if cfg := loadConfig(); cfg != defaultConfig() {
		t.Errorf("malformed config should fall back to defaults: %+v", cfg)
	}
}
