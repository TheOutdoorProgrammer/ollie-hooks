#!/bin/sh
# Install ollie-hooks on Linux, or Windows via Git Bash / WSL. macOS has a tap.
# VERSION pins a release; INSTALL_DIR chooses where it lands.
set -eu

REPO=TheOutdoorProgrammer/ollie-hooks
VERSION=${VERSION:-latest}
INSTALL_DIR=${INSTALL_DIR:-$HOME/.local/bin}

die() { printf 'install: %s\n' "$1" >&2; exit 1; }

for cmd in curl tar uname; do
	command -v "$cmd" >/dev/null 2>&1 || die "$cmd is required"
done

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
	linux) ;;
	darwin) printf 'install: on macOS prefer: brew install %s/tap/ollie-hooks\n' "TheOutdoorProgrammer" >&2 ;;
	mingw*|msys*|cygwin*) os=windows ;;
	*) die "unsupported OS: $os" ;;
esac

case "$(uname -m)" in
	x86_64|amd64) arch=amd64 ;;
	aarch64|arm64) arch=arm64 ;;
	*) die "unsupported architecture: $(uname -m)" ;;
esac

if [ "$VERSION" = latest ]; then
	url="https://github.com/$REPO/releases/latest/download"
else
	url="https://github.com/$REPO/releases/download/$VERSION"
fi

ext=tar.gz
[ "$os" = windows ] && ext=zip
asset="ollie-hooks_${os}_${arch}.${ext}"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

printf 'install: fetching %s\n' "$asset"
curl -fsSL "$url/$asset" -o "$tmp/$asset" || die "no release asset $asset — check $url"

# Verify against the published checksums when they are available. A silently
# corrupted download would install a binary that gates every tool call.
if curl -fsSL "$url/checksums.txt" -o "$tmp/checksums.txt" 2>/dev/null; then
	if command -v sha256sum >/dev/null 2>&1; then
		( cd "$tmp" && grep " $asset\$" checksums.txt | sha256sum -c - ) || die "checksum mismatch"
	elif command -v shasum >/dev/null 2>&1; then
		( cd "$tmp" && grep " $asset\$" checksums.txt | shasum -a 256 -c - ) || die "checksum mismatch"
	else
		printf 'install: no sha256 tool, skipping verification\n' >&2
	fi
else
	printf 'install: no checksums published, skipping verification\n' >&2
fi

if [ "$ext" = zip ]; then
	command -v unzip >/dev/null 2>&1 || die "unzip is required"
	unzip -q "$tmp/$asset" -d "$tmp"
else
	tar -xzf "$tmp/$asset" -C "$tmp"
fi

mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/ollie-hooks" "$INSTALL_DIR/ollie-hooks" 2>/dev/null \
	|| { cp "$tmp/ollie-hooks" "$INSTALL_DIR/ollie-hooks" && chmod 0755 "$INSTALL_DIR/ollie-hooks"; }

printf 'install: ollie-hooks -> %s\n' "$INSTALL_DIR/ollie-hooks"

case ":$PATH:" in
	*":$INSTALL_DIR:"*) ;;
	*) printf 'install: %s is not on PATH — add it before wiring the hook\n' "$INSTALL_DIR" >&2 ;;
esac

printf '\nNothing runs until you enable it:\n\n  ollie-hooks wiring\n\n'
