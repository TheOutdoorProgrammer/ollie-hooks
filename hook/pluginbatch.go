package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"
)

// pluginRequest is what a plugin reads on stdin: the event exactly as Claude
// Code sent it, plus which of this plugin's rules are being asked. Rules is how
// one binary serves several roles in a single call rather than one spawn each.
type pluginRequest struct {
	Event json.RawMessage `json:"event"`
	Rules []string        `json:"rules"`
}

// pluginReply is what a plugin writes. A single-rule plugin can answer with the
// bare response; one serving several roles keys them by rule id.
type pluginReply struct {
	PluginResponse
	Rules map[string]PluginResponse `json:"rules,omitempty"`
}

// for returns the response meant for one rule, preferring an explicit entry.
func (r pluginReply) for_(id string) PluginResponse {
	if got, ok := r.Rules[id]; ok {
		return got
	}
	return r.PluginResponse
}

// pluginBroker calls each distinct plugin at most once per hook invocation.
// Every invocation is a fresh process handling one event, so memoising here
// collapses N rules sharing a binary into one spawn — with no daemon at all.
type pluginBroker struct {
	mu      sync.Mutex
	byKey   map[string]pluginReply
	failed  map[string]bool
	members map[string][]string
}

var broker = &pluginBroker{
	byKey:   map[string]pluginReply{},
	failed:  map[string]bool{},
	members: map[string][]string{},
}

// join records that a rule is served by a transport, so the one call made for
// that transport asks for every rule at once.
func (b *pluginBroker) join(key, ruleID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.members[key] = append(b.members[key], ruleID)
}

// response returns this rule's slice of the plugin's answer, calling the plugin
// only if it has not already answered for this event.
func (b *pluginBroker) response(ctx context.Context, key string, t transport, ev *Event, ruleID string) *PluginResponse {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.failed[key] {
		return nil
	}
	reply, done := b.byKey[key]
	if !done {
		payload, err := json.Marshal(pluginRequest{
			Event: eventPayload(ev),
			Rules: b.members[key],
		})
		if err != nil {
			b.failed[key] = true
			tracef("custom rule %s: encoding request: %v", ruleID, err)
			return nil
		}
		raw, err := t.call(ctx, payload)
		if err != nil {
			b.failed[key] = true
			tracef("custom rule %s failed: %v", ruleID, err)
			return nil
		}
		if err := decodeReply(raw, &reply); err != nil {
			b.failed[key] = true
			tracef("custom rule %s: %v", ruleID, err)
			return nil
		}
		b.byKey[key] = reply
		tracef("called plugin %q once for %d rule(s)", key, len(b.members[key]))
	}
	out := reply.for_(ruleID)
	return &out
}

func decodeReply(raw []byte, into *pluginReply) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil // nothing to say is a valid answer
	}
	return json.Unmarshal(raw, into)
}

// eventPayload is the event as received, falling back to re-encoding for a
// hand-built Event (tests, and any caller that did not use DecodeEvent).
func eventPayload(ev *Event) json.RawMessage {
	if len(ev.Raw) > 0 {
		return ev.Raw
	}
	raw, err := json.Marshal(ev)
	if err != nil {
		return json.RawMessage("{}")
	}
	return raw
}
