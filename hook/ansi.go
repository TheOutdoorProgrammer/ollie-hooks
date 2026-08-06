package hook

import (
	"fmt"
	"strconv"
	"strings"
)

// Reset ends a colour run.
const Reset = "\x1b[0m"

// Paint wraps each line separately in a truecolor escape. Per line, not once
// around the block: Claude Code indents continuation lines, and a colour left
// open across that prefix bleeds into the UI's own output.
func Paint(text, hex string) string {
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
