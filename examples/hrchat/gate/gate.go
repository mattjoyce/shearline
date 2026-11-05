// Package gate is the policy enforcement point: the one place a tool call the model
// emits becomes a request the policy decides, and only then a call on the payroll
// store.
//
// The model never names the principal. The session does, from authentication, and the
// gate copies it into every request. The model names a tool and an employee; the gate
// turns those into an action and a resource, refuses an unknown tool or employee before
// the policy is asked, attaches what the app attests (channel, guardrail verdict, MFA,
// confirmation, read velocity, requester) and decides. Every decision, allowed or not,
// goes on the ledger with its hash and what it cost.
package gate

import (
	"context"
	"errors"
	"fmt"

	"github.com/mattjoyce/shearline/examples/hrchat/catalog"
	"github.com/mattjoyce/shearline/examples/hrchat/hrpolicy"
	"github.com/mattjoyce/shearline/examples/hrchat/org"
	"github.com/mattjoyce/tarski"
)

// Session is who is talking, from authentication: never from the model's output.
type Session struct {
	EmployeeID string
	Channel    string // "chat" for the bot, "web" for the HR app
	MFA        bool   // a fresh second factor on this session, as the app judges freshness
}

// Attestation is what the app knows about this turn. Guardrail is the jailbreak
// classifier's verdict; Confirmed is a confirmation the human gave the app, not the
// model; HighReads is the principal's count of high reads this window from the ledger;
// Requester is who raised the change an approval is for.
type Attestation struct {
	Guardrail string
	Confirmed bool
	HighReads int
	Requester string // an employee ID, when known
}

// Call is what the model emitted: a tool and its arguments.
type Call struct {
	Tool string
	Args map[string]string
}

// Outcome is what the gate did with a call.
type Outcome struct {
	Tool, Principal, Action, Resource string
	Allowed                           bool
	Result                            string // the tool's reply, when allowed
	Refusal                           string // one line the bot may relay, when not
	Decision                          hrpolicy.Decision
	Err                               error // a malformed call or a broken policy; always a refusal
}

// Entry is one ledger row: the decision as the audit queries read it. Steps and Bytes
// are what the evaluation cost, and Margin is the larger of the two as a whole percent of
// the policy's bounds, rounded up: the audit alerts on it, and nothing decides by it. A
// call refused before the policy cost nothing; one the bounds stopped carries the cost
// at which they stopped it.
type Entry struct {
	Seq                         int
	Principal, Action, Resource string
	Allowed                     bool
	Hash                        string
	Steps, Bytes, Margin        int
}

// Gate holds the organisation, the policy over it, the store and the ledger.
type Gate struct {
	Org    *org.Org
	Policy hrpolicy.Policy
	Store  *Store
	ledger []Entry
}

// New builds the gate over an organisation: the policy from the org's facts and the
// catalogue's, and a store holding the org's records.
func New(o *org.Org) (*Gate, error) {
	if err := catalog.Check(); err != nil {
		return nil, err
	}
	p, err := hrpolicy.Load(o.Facts(), catalog.Facts())
	if err != nil {
		return nil, err
	}
	return &Gate{Org: o, Policy: p, Store: NewStore(o)}, nil
}

// Call decides one tool call and, when allowed, executes it.
func (g *Gate) Call(ctx context.Context, s Session, c Call, a Attestation) Outcome {
	out := Outcome{Tool: c.Tool, Principal: org.Resource(s.EmployeeID)}
	tool, ok := catalog.Lookup(c.Tool)
	if !ok {
		return g.refuse(out, fmt.Errorf("unknown tool %q", c.Tool))
	}
	out.Action = tool.Action
	subject := c.Args["employee_id"]
	if _, ok := g.Org.Get(subject); !ok {
		return g.refuse(out, fmt.Errorf("unknown employee %q", subject))
	}
	out.Resource = org.Resource(subject)
	req := hrpolicy.Request{
		Principal: out.Principal, Action: out.Action, Resource: out.Resource,
		Context: hrpolicy.Context{
			Channel: s.Channel, Guardrail: a.Guardrail, MFA: s.MFA, Confirmed: a.Confirmed,
			HighReads: a.HighReads, Requester: requester(a.Requester),
		},
	}
	d, err := g.Policy.Decide(ctx, req)
	out.Decision = d
	if err != nil {
		return g.refuse(out, err)
	}
	out.Allowed = d.Allow
	g.record(out) // after Allowed is final: the ledger row is the decision, not a draft of it
	if !d.Allow {
		out.Refusal = hrpolicy.Explain(d)
		return out
	}
	out.Result = g.Store.Exec(tool, subject, c.Args)
	return out
}

func requester(id string) string {
	if id == "" {
		return ""
	}
	return org.Resource(id)
}

func (g *Gate) refuse(out Outcome, err error) Outcome {
	out.Err = err
	out.Refusal = "refused: " + err.Error()
	g.record(out)
	return out
}

func (g *Gate) record(out Outcome) {
	steps, bytes := out.Decision.Steps, out.Decision.Bytes
	var tl *tarski.TooLarge
	if errors.As(out.Err, &tl) {
		steps, bytes = tl.Steps, tl.Bytes
	}
	b := g.Policy.Bounds()
	g.ledger = append(g.ledger, Entry{
		Seq: len(g.ledger) + 1, Principal: out.Principal, Action: out.Action, Resource: out.Resource,
		Allowed: out.Allowed, Hash: out.Decision.Hash,
		Steps: steps, Bytes: bytes, Margin: max(percentOf(steps, b.MaxSteps), percentOf(bytes, b.MaxBytes)),
	})
}

// percentOf is n as a whole percent of bound, rounded up, so a cost just past a
// threshold is past it on the ledger too.
func percentOf(n, bound int) int {
	if bound <= 0 {
		return 0
	}
	return (n*100 + bound - 1) / bound
}

// Ledger is every decision so far, in order.
func (g *Gate) Ledger() []Entry { return append([]Entry(nil), g.ledger...) }
