package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"
	"time"
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

// brokerEntry is one endpoint's answer for the current event. The once is what
// makes N rules sharing a binary cost one spawn, without holding the broker's
// lock across the call.
type brokerEntry struct {
	once  sync.Once
	reply pluginReply
	ok    bool
}

// pluginBroker calls each distinct plugin at most once per event. Reset between
// events by Decide: without that, an embedder calling Decide twice would replay
// the first event's answers against the second.
type pluginBroker struct {
	mu      sync.Mutex
	entries map[string]*brokerEntry
	members map[string][]string
}

var broker = &pluginBroker{
	entries: map[string]*brokerEntry{},
	members: map[string][]string{},
}

// reset drops the answers cached for the previous event, keeping membership,
// which is registration-time information rather than per-event state.
func (b *pluginBroker) reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = map[string]*brokerEntry{}
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
	entry := b.entries[key]
	if entry == nil {
		entry = &brokerEntry{}
		b.entries[key] = entry
	}
	asked := membersFor(b.members[key], ev)
	b.mu.Unlock()

	// Outside the lock: a subprocess or an HTTP round trip is far too long to
	// hold one for, and the once already guarantees a single call.
	entry.once.Do(func() {
		payload, err := json.Marshal(pluginRequest{
			Event: eventPayload(ev),
			Rules: asked,
		})
		if err != nil {
			tracef("custom rule %s: encoding request: %v", ruleID, err)
			return
		}
		// Detached from the calling rule's deadline and given the most generous
		// budget among the rules sharing this endpoint. Inheriting whichever
		// rule happened to fire first let a timeout=1 sibling kill a timeout=30.
		callCtx, cancel := context.WithTimeout(
			context.WithoutCancel(ctx), sharedBudget(asked))
		defer cancel()

		raw, err := t.call(callCtx, payload)
		if err != nil {
			tracef("custom rule %s failed: %v", ruleID, err)
			return
		}
		if err := decodeReply(raw, &entry.reply); err != nil {
			tracef("custom rule %s: %v", ruleID, err)
			return
		}
		entry.ok = true
		tracef("called plugin %q once for %d rule(s)", key, len(asked))
	})

	if !entry.ok {
		return nil
	}
	out := entry.reply.for_(ruleID)
	return &out
}

// membersFor narrows an endpoint's rules to those this event actually asks for.
// The wire contract says the request names the roles being asked, so sending
// every registered role would make a plugin re-derive that itself.
func membersFor(ids []string, ev *Event) []string {
	asked := make([]string, 0, len(ids))
	for _, id := range ids {
		for _, r := range registry {
			if r.ID == id && r.handles(ev.HookEventName) && r.appliesTo(ev.ToolName) {
				asked = append(asked, id)
				break
			}
		}
	}
	return asked
}

// sharedBudget is the longest timeout among the rules sharing one endpoint, so
// the call is bounded by the most patient of them rather than the least.
func sharedBudget(ids []string) time.Duration {
	longest := 0
	for _, id := range ids {
		if t := RuleTimeout(id, 0); t > longest {
			longest = t
		}
	}
	if longest <= 0 {
		longest = defaultRuleTimeout
	}
	return time.Duration(longest) * time.Second
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
