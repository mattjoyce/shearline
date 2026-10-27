package gate_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mattjoyce/shearline/examples/hrchat/gate"
	"github.com/mattjoyce/shearline/examples/hrchat/org"
)

// TestScenarios pins the matrix: every row of Scenarios decides as written, and every
// refusal names the reason the row expects.
func TestScenarios(t *testing.T) {
	g, err := gate.New(org.Generate(1))
	if err != nil {
		t.Fatal(err)
	}
	cast := gate.FindCast(g.Org)
	for _, f := range []string{cast.CEO, cast.CFO, cast.CTO, cast.Director, cast.Manager, cast.IC, cast.Peer, cast.Payroll, cast.HRBPEng, cast.HRBPPeople} {
		if f == "" {
			t.Fatalf("cast incomplete: %+v", cast)
		}
	}
	for _, sc := range gate.Scenarios(cast) {
		t.Run(sc.Name, func(t *testing.T) {
			out := g.Call(context.Background(), sc.Session, sc.Call, sc.Attest)
			if out.Allowed != sc.Allow {
				t.Fatalf("allowed=%v, want %v\n%s\n%s", out.Allowed, sc.Allow, out.Refusal, out.Decision.Why.String())
			}
			if !sc.Allow && !strings.Contains(out.Refusal, sc.Reason) {
				t.Fatalf("refusal %q does not name %q\n%s", out.Refusal, sc.Reason, out.Decision.Why.String())
			}
			if out.Err == nil && out.Decision.Hash == "" {
				t.Fatal("a decided call has no evidence hash for the ledger")
			}
			if out.Err == nil && out.Decision.Steps == 0 {
				t.Fatal("a decided call has no cost for the ledger")
			}
		})
	}
}

// TestLedgerRecordsEveryCall holds the gate to recording refusals before the policy as
// well as decisions.
func TestLedgerRecordsEveryCall(t *testing.T) {
	g, err := gate.New(org.Generate(1))
	if err != nil {
		t.Fatal(err)
	}
	cast := gate.FindCast(g.Org)
	calls := gate.Scenarios(cast)
	for _, sc := range calls {
		g.Call(context.Background(), sc.Session, sc.Call, sc.Attest)
	}
	if got := len(g.Ledger()); got != len(calls) {
		t.Fatalf("ledger has %d rows for %d calls", got, len(calls))
	}
	// Each row says what the gate did, and the ledger has allowed rows to audit.
	allowed := 0
	for i, e := range g.Ledger() {
		if e.Allowed != calls[i].Allow {
			t.Fatalf("row %d (%s): ledger says allowed=%v, the gate did allowed=%v", e.Seq, calls[i].Name, e.Allowed, calls[i].Allow)
		}
		if e.Allowed {
			allowed++
		}
	}
	if allowed == 0 {
		t.Fatal("no allowed rows on the ledger")
	}
}

// TestPrincipalComesFromTheSession is the confused-deputy check: the model's arguments
// cannot name who is asking. A call that carries a different employee as the subject
// is decided for the session's principal.
func TestPrincipalComesFromTheSession(t *testing.T) {
	g, err := gate.New(org.Generate(1))
	if err != nil {
		t.Fatal(err)
	}
	cast := gate.FindCast(g.Org)
	out := g.Call(context.Background(), gate.Session{EmployeeID: cast.IC, Channel: "chat", MFA: true},
		gate.Call{Tool: "get_bank_details", Args: map[string]string{"employee_id": cast.Peer, "principal": cast.Peer, "as": cast.Peer}},
		gate.Attestation{Guardrail: "clean"})
	if out.Allowed || out.Principal != org.Resource(cast.IC) {
		t.Fatalf("principal %s allowed=%v; the session's principal must decide", out.Principal, out.Allowed)
	}
}
