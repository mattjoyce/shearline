// Package audit runs tarski queries over the gate's ledger. The policy decides one
// request and cannot count; the audit counts across requests and cannot decide. The
// split is Shearline's: aggregates are for queries, so the detection of a scrape or a
// probe is a query over what the policy already decided, with the same engine and the
// same proofs.
package audit

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/mattjoyce/shearline/examples/hrchat/catalog"
	"github.com/mattjoyce/shearline/examples/hrchat/gate"
	"github.com/mattjoyce/tarski"
)

//go:embed audit.dl
var text string

// Finding is one derived tuple of a detection relation.
type Finding struct {
	Relation string
	Tuple    tarski.Tuple
}

// Report is what the queries found, and what they cost.
type Report struct {
	Findings     []Finding
	Steps, Bytes int
	Hash         map[string]string // the answer hash per detection relation
}

var detections = []string{"probing", "fanout", "costly"}

// Run evaluates the audit queries over the ledger.
func Run(ctx context.Context, ledger []gate.Entry) (Report, error) {
	schema := tarski.Schema{Arities: map[string]int{"decision": 5, "cost": 2, "level": 2}}
	prog, err := tarski.Parse(text)
	if err != nil {
		return Report{}, fmt.Errorf("audit: %w", err)
	}
	plan, err := tarski.Compile(schema, prog)
	if err != nil {
		return Report{}, fmt.Errorf("audit: %w", err)
	}
	facts := make([]tarski.Fact, 0, len(ledger))
	for _, e := range ledger {
		if e.Action == "" || e.Resource == "" {
			continue // refused before the policy: no action or resource to count
		}
		facts = append(facts, tarski.Fact{Relation: "decision", Tuple: tarski.Tuple{
			tarski.Int(int64(e.Seq)), tarski.String(e.Principal), tarski.String(e.Action), tarski.String(e.Resource), tarski.Bool(e.Allowed),
		}}, tarski.Fact{Relation: "cost", Tuple: tarski.Tuple{tarski.Int(int64(e.Seq)), tarski.Int(int64(e.Margin))}})
	}
	for _, f := range catalog.Facts() {
		if f.Relation == "level" {
			facts = append(facts, f)
		}
	}
	base, err := tarski.NewBase(schema, facts...)
	if err != nil {
		return Report{}, fmt.Errorf("audit: %w", err)
	}
	res, err := tarski.Evaluate(ctx, plan, base, tarski.Options{MaxSteps: 1 << 24, MaxBytes: 1 << 24})
	if err != nil {
		return Report{}, fmt.Errorf("audit: %w", err)
	}
	r := Report{Hash: map[string]string{}}
	r.Steps, r.Bytes = res.Cost()
	for _, rel := range detections {
		ans, err := res.Relation(rel)
		if err != nil {
			return Report{}, fmt.Errorf("audit: %w", err)
		}
		r.Hash[rel] = ans.Hash()
		for _, t := range ans.Tuples() {
			r.Findings = append(r.Findings, Finding{rel, t})
		}
	}
	return r, nil
}
