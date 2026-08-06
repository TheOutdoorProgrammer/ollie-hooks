package hook

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Reset ends a colour run.
const Reset = "\x1b[0m"

// Paint wraps each line separately in a truecolor escape. Per line, not once
// around the block: Claude Code indents continuation lines, and a colour left
// open across that prefix bleeds into the UI's own output.
func Paint(text, hex string) string {
	if NoColor() {
		return text
	}
	seq := TrueColor(hex)
	if seq == "" || text == "" {
		return text
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = seq + line + Reset
		}
	}
	return strings.Join(lines, "\n")
}

// TrueColor turns "rrggbb" into an SGR escape, or "" if it isn't a hex triple.
func TrueColor(hex string) string {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(hex) != 6 {
		return ""
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", v>>16&0xff, v>>8&0xff, v&0xff)
}

// NoColor honours the NO_COLOR standard (https://no-color.org): any value at
// all, even empty, means do not emit escapes. Worth honouring because hook
// output gets piped and scraped, and escapes in a log are noise.
func NoColor() bool {
	_, set := os.LookupEnv("NO_COLOR")
	return set
}

// ansiSeq matches an SGR escape run.
var ansiSeq = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// Strip removes colour escapes, so painted text can be measured or matched.
// Counting runes on a painted string measures the escapes too, which is how a
// width cap starts rejecting output that would have fitted.
func Strip(s string) string {
	return ansiSeq.ReplaceAllString(s, "")
}
