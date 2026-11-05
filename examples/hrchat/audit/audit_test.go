package audit_test

import (
	"context"
	"testing"

	"github.com/mattjoyce/shearline/examples/hrchat/audit"
	"github.com/mattjoyce/shearline/examples/hrchat/gate"
)

// TestCostlyPastAQuarter: a decision over 25% of a bound is a finding, and one at 25%
// is not.
func TestCostlyPastAQuarter(t *testing.T) {
	ledger := []gate.Entry{
		{Seq: 1, Principal: "emp/E0001", Action: "read.leave", Resource: "emp/E0002", Margin: 25},
		{Seq: 2, Principal: "emp/E0003", Action: "read.leave", Resource: "emp/E0004", Margin: 26},
	}
	rep, err := audit.Run(context.Background(), ledger)
	if err != nil {
		t.Fatal(err)
	}
	var costly []string
	for _, f := range rep.Findings {
		if f.Relation == "costly" {
			costly = append(costly, f.Tuple[1].Text())
		}
	}
	if len(costly) != 1 || costly[0] != "emp/E0003" {
		t.Fatalf("costly findings %v, want only emp/E0003", costly)
	}
}
