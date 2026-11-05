// Package hrpolicy is the HR chatbot's policy: Shearline's bundled rbac/1 rule set plus
// hrchat/1, evaluated by Shearline over the organisation, the catalogue and the
// entitlements, one request at a time.
//
// Shearline does the loading, the bounds and the deciding. This package declares what
// Shearline cannot know: the consumer relations the rules read (reports_to, dept,
// hr_partner and the entitlement lists), the context the gate attests, and the hash the
// ledger keeps for each decision.
package hrpolicy

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/mattjoyce/shearline/policy"
	"github.com/mattjoyce/tarski"
)

//go:embed hrchat.dl
var text string

// Rules is hrchat/1, parsed from the embedded text. A broken asset is an error at Load,
// so a bad policy never serves; TestLoad_RuleSetsAreShearlinesAndOurs holds it.
func Rules() (tarski.RuleSet, error) {
	s, err := tarski.NewRuleSet("hrchat", 1, text)
	if err != nil {
		return tarski.RuleSet{}, fmt.Errorf("hrpolicy: bundled rule set hrchat/1: %w", err)
	}
	return s, nil
}

// ErrRequest marks a request the policy refused as malformed. Any other error is a
// broken policy or engine.
var ErrRequest = policy.ErrRequest

// Relations is every standing relation hrchat/1 reads beyond Shearline's PARC ones. Every
// column is a string, and Shearline refuses an empty one: a missing value, not a wildcard.
func Relations() map[string]policy.Relation {
	arities := map[string]int{
		// The organisation.
		"reports_to": 2, "dept": 2, "hr_partner": 2,
		// The catalogue.
		"verb": 2, "level": 2, "writes": 1,
		// The entitlements.
		"self_may": 1, "manager_may": 1, "hr_may": 1, "role_may": 2,
		"also_needs": 2, "channel": 1, "channel_forbids": 2,
	}
	out := make(map[string]policy.Relation, len(arities))
	for name, n := range arities {
		kinds := make([]tarski.Kind, n)
		for i := range kinds {
			kinds[i] = tarski.KindString
		}
		out[name] = policy.Relation{Kinds: kinds}
	}
	return out
}

// Context is what the gate attests about one request. Channel, Guardrail, MFA and
// Confirmed come from the session and the app, never from the model. HighReads is the
// app's count of this principal's high reads in the current window, read by the
// velocity rule. Requester names who raised the change an approval is for; it is sent
// only when set, and the four-eyes rule requires it positively.
type Context struct {
	Channel   string
	Guardrail string // "clean" or "flagged": the jailbreak guardrail's verdict on this turn
	MFA       bool
	Confirmed bool
	HighReads int
	Requester string
}

// Keys is the declared context: a request may carry these and nothing else.
func Keys() map[string]tarski.Kind {
	return map[string]tarski.Kind{
		"channel": tarski.KindString, "guardrail": tarski.KindString,
		"mfa": tarski.KindBool, "confirmed": tarski.KindBool,
		"high_reads": tarski.KindNumber, "requester": tarski.KindString,
	}
}

// Request is the PARC shape: who, does what, to which record, under what.
type Request struct {
	Principal, Action, Resource string
	Context                     Context
}

// Decision is Shearline's decision with the hash the ledger keeps. Steps and Bytes are
// Shearline's cost, for watching the margin to Policy.Bounds and nothing else.
type Decision struct {
	policy.Decision
	Hash string // sha256 over the rule sets, the answer and its proof or why-not
}

// Policy is the loaded Shearline policy. It is a value: nothing changes after Load, so it
// is safe to share.
type Policy struct {
	p   policy.Policy
	ids []string
}

// Load compiles rbac/1 and hrchat/1 and holds the organisation's, the catalogue's and the
// entitlements' facts. Any error is the operator's to fix before serving; a bad policy
// never serves.
func Load(standing ...[]tarski.Fact) (Policy, error) {
	var facts []tarski.Fact
	for _, s := range standing {
		facts = append(facts, s...)
	}
	facts = append(facts, entitlementFacts()...)
	ours, err := Rules()
	if err != nil {
		return Policy{}, err
	}
	p, err := policy.Load(policy.Config{
		Standing:  facts,
		Sets:      []tarski.RuleSet{policy.RBAC(), ours},
		Context:   Keys(),
		Relations: Relations(),
	})
	if err != nil {
		return Policy{}, fmt.Errorf("hrpolicy: %w", err)
	}
	return Policy{p: p, ids: []string{policy.RBAC().ID(), ours.ID()}}, nil
}

// RuleSets is the loaded sets' IDs, in load order.
func (p Policy) RuleSets() []string { return append([]string(nil), p.ids...) }

// Bounds is what each decision is evaluated under, for the margin.
func (p Policy) Bounds() policy.Bounds { return p.p.Bounds() }

// Decide is Shearline's: total, so nil error with Allow=false is deny by absence and a
// non-nil error is also a deny. Every decision that evaluated carries its hash.
func (p Policy) Decide(ctx context.Context, req Request) (Decision, error) {
	d, err := p.p.Decide(ctx, policy.Request{
		Principal: req.Principal, Action: req.Action, Resource: req.Resource,
		Context: req.Context.consts(),
	})
	out := Decision{Decision: d}
	if err != nil {
		return out, fmt.Errorf("hrpolicy: %w", err)
	}
	if out.Hash, err = evidenceHash(d); err != nil {
		return out, fmt.Errorf("hrpolicy: decide: %w", err)
	}
	return out, nil
}

// consts is the context as Shearline takes it. Requester is sent only when set: the
// four-eyes rule reads it positively.
func (c Context) consts() map[string]tarski.Const {
	m := map[string]tarski.Const{
		"channel": tarski.String(c.Channel), "guardrail": tarski.String(c.Guardrail),
		"mfa": tarski.Bool(c.MFA), "confirmed": tarski.Bool(c.Confirmed),
		"high_reads": tarski.Int(int64(c.HighReads)),
	}
	if c.Requester != "" {
		m["requester"] = tarski.String(c.Requester)
	}
	return m
}

// evidenceHash is what a ledger row commits to: which rules decided, the answer, and the
// proof or why-not that stood behind it. The cost is left out, since it changes with the
// engine and the answer does not.
func evidenceHash(d policy.Decision) (string, error) {
	ev := struct {
		RuleSets []string        `json:"rule_sets"`
		Allow    bool            `json:"allow"`
		Proof    *tarski.Proof   `json:"proof,omitempty"`
		Why      *tarski.Absence `json:"why,omitempty"`
	}{RuleSets: d.RuleSets, Allow: d.Allow}
	if d.Allow {
		ev.Proof = &d.Proof
	} else {
		ev.Why = &d.Why
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
