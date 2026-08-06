package hook

import (
	"bytes"
	"context"

	"fmt"
	"net/http"
	"os/exec"
	"strings"
)

// PluginResponse is what an out-of-process rule returns. It mirrors a built-in
// rule's verbs, and the plugin's DECLARED verb decides which field is read, so
// one enabled to advise cannot quietly start denying calls.
type PluginResponse struct {
	Findings []Finding `json:"findings,omitempty"`
	Advice   string    `json:"advice,omitempty"`
	Decision *Decision `json:"decision,omitempty"`
	Mutation *Mutation `json:"mutation,omitempty"`
	Display  string    `json:"display,omitempty"`
}

// transport carries one request to a plugin and back.
type transport interface {
	call(ctx context.Context, payload []byte) ([]byte, error)
}

// execTransport runs the plugin as a short-lived child: request on stdin,
// response on stdout, exit. Deliberately not a supervised daemon — every hook
// invocation is its own process, so reusing a child would mean managing one
// that outlives us. Warm state belongs behind a server_url instead.
type execTransport struct{ command string }

func (t execTransport) call(ctx context.Context, payload []byte) ([]byte, error) {
	fields := strings.Fields(t.command)
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty startup_cmd")
	}
	cmd := exec.CommandContext(ctx, fields[0], fields[1:]...)
	cmd.Stdin = bytes.NewReader(payload)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s: %w: %s", fields[0], err, strings.TrimSpace(errBuf.String()))
	}
	return out.Bytes(), nil
}

// httpTransport posts to an already-running plugin. This is the path for a
// plugin that needs warm state — a loaded index, a live language server.
type httpTransport struct{ url string }

func (t httpTransport) call(ctx context.Context, payload []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("%s returned %s", t.url, resp.Status)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// transportFor builds the configured transport, or an error naming what is
// wrong. Exactly one of server_url and startup_cmd must be set: guessing which
// the user meant is how a plugin silently talks to the wrong thing.
func transportFor(c CustomRule) (transport, error) {
	switch {
	case c.ServerURL != "" && c.StartupCmd != "":
		return nil, fmt.Errorf("set either server_url or startup_cmd, not both")
	case c.ServerURL != "":
		return httpTransport{url: c.ServerURL}, nil
	case c.StartupCmd != "":
		return execTransport{command: c.StartupCmd}, nil
	}
	return nil, fmt.Errorf("needs a server_url or a startup_cmd")
}
