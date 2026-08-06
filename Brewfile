# Helper tools the built-in rules shell out to.
#
#   brew bundle --file=Brewfile
#
# You only need the tools for rules you actually enable — every rule ships
# disabled. But once a rule IS enabled, its tools are required: an enabled rule
# whose tool is missing fails CLOSED and says so, rather than passing quietly.
# A gate that silently does not run is worse than no gate, because you think
# you have one.
#
# The lint rule is the exception, and it is configurable rather than
# all-or-nothing: list the linters you want under [rules.lint] and only those
# are required.
#
# Nothing is version-pinned on purpose: these are developer tools where the
# current release is the one you want, and a pin here would rot silently.

# --- secret-scan -----------------------------------------------------------
# Scans prompts for credentials before they ever reach the API. MIT.
brew "betterleaks"

# --- lint ------------------------------------------------------------------
# Dispatched by file extension. Install the ones you list in your config.
brew "shellcheck"       # sh, bash
brew "yamllint"         # yaml, yml
brew "ruff"             # python
brew "golangci-lint"    # go
brew "hadolint"         # Dockerfile
brew "opentofu"         # tf, tofu  (provides `tofu`)
brew "selene"           # lua
brew "swiftlint"        # swift (macOS only)

# markdownlint-cli2 is npm-only:
#   npm install -g markdownlint-cli2
#
# mermaid-ascii has no formula and is Go-only. The mermaid-stream rule needs it:
#   go install github.com/AlexanderGrooff/mermaid-ascii@latest
