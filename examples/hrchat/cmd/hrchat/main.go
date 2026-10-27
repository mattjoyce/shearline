// Command hrchat drives the experiment: generate the organisation, list the tools,
// decide one call with its proof or why-not, print a principal's matrix, or run the
// scenario table and audit the ledger it leaves.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mattjoyce/shearline/examples/hrchat/audit"
	"github.com/mattjoyce/shearline/examples/hrchat/catalog"
	"github.com/mattjoyce/shearline/examples/hrchat/gate"
	"github.com/mattjoyce/shearline/examples/hrchat/hrpolicy"
	"github.com/mattjoyce/shearline/examples/hrchat/org"
)

const usage = `hrchat: an HR chatbot's tool gate, decided by Datalog.

  hrchat org                      the organisation: size, shape, the cast
  hrchat who <id>                 one employee: chain, reports, roles
  hrchat tools                    the MCP tool manifest, with action and level
  hrchat rules                    the rule sets and their hashes
  hrchat decide --as E0042 --tool get_bank_details --subject E0042 [--mfa] [--confirmed]
                [--channel chat|web] [--guardrail clean|flagged] [--high-reads N] [--requester E0007]
  hrchat matrix --as E0042        every tool against self, a report, a skip report, a peer, the manager
  hrchat scenarios                run the scenario table, then audit the ledger

Flags common to all: --seed N (default 1).
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	seed := fs.Uint64("seed", 1, "organisation seed")
	as := fs.String("as", "", "the principal's employee ID")
	tool := fs.String("tool", "", "the tool to call")
	subject := fs.String("subject", "", "the employee the call is about")
	value := fs.String("value", "", "a value for a write")
	channel := fs.String("channel", "chat", "chat or web")
	guardrail := fs.String("guardrail", "clean", "the guardrail verdict: clean or flagged")
	mfa := fs.Bool("mfa", false, "a fresh second factor on the session")
	confirmed := fs.Bool("confirmed", false, "the human confirmed the change in the app")
	highReads := fs.Int("high-reads", 0, "high reads by this principal this window")
	requester := fs.String("requester", "", "who raised the change an approval is for")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	o := org.Generate(*seed)
	g, err := gate.New(o)
	if err != nil {
		fail(err)
	}
	ctx := context.Background()
	switch cmd {
	case "org":
		showOrg(o)
	case "who":
		if fs.NArg() != 1 {
			fail(fmt.Errorf("who <id>"))
		}
		showWho(o, fs.Arg(0))
	case "tools":
		b, err := catalog.Manifest()
		if err != nil {
			fail(err)
		}
		fmt.Println(string(b))
	case "rules":
		for _, id := range g.Policy.RuleSets() {
			fmt.Println(id)
		}
	case "decide":
		s := gate.Session{EmployeeID: *as, Channel: *channel, MFA: *mfa}
		a := gate.Attestation{Guardrail: *guardrail, Confirmed: *confirmed, HighReads: *highReads, Requester: *requester}
		c := gate.Call{Tool: *tool, Args: map[string]string{"employee_id": *subject, "value": *value, "request_id": *value}}
		showOutcome(g.Call(ctx, s, c, a), true)
	case "matrix":
		showMatrix(ctx, g, *as, *channel)
	case "scenarios":
		runScenarios(ctx, g)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "hrchat:", err)
	os.Exit(1)
}

func showOrg(o *org.Org) {
	counts := map[string]int{}
	for _, e := range o.Employees {
		counts[e.Dept]++
	}
	fmt.Printf("seed %d: %d employees\n", o.Seed, len(o.Employees))
	for _, d := range o.Depts() {
		fmt.Printf("  %-12s %4d\n", d, counts[d])
	}
	c := gate.FindCast(o)
	fmt.Printf("cast: CEO %s  CFO %s  CTO %s  director %s  manager %s  IC %s  peer %s  payroll %s  HRBP(eng) %s  HRBP(people) %s\n",
		c.CEO, c.CFO, c.CTO, c.Director, c.Manager, c.IC, c.Peer, c.Payroll, c.HRBPEng, c.HRBPPeople)
}

func showWho(o *org.Org, id string) {
	e, ok := o.Get(id)
	if !ok {
		fail(fmt.Errorf("no employee %s", id))
	}
	fmt.Printf("%s %s, %s, %s, %s\n", e.ID, e.Name, e.Title, e.Dept, e.Location)
	fmt.Printf("  chain: %s\n", strings.Join(o.Chain(id), " > "))
	fmt.Printf("  direct reports: %d\n", len(o.Reports(id)))
	if len(e.Roles) > 0 {
		fmt.Printf("  roles: %s\n", strings.Join(e.Roles, ", "))
	}
	if len(e.PartnerOf) > 0 {
		fmt.Printf("  HR partner for: %s\n", strings.Join(e.PartnerOf, ", "))
	}
}

func showOutcome(out gate.Outcome, full bool) {
	verdict := "DENY "
	if out.Allowed {
		verdict = "ALLOW"
	}
	fmt.Printf("%s %s %s %s on %s\n", verdict, out.Tool, out.Principal, out.Action, out.Resource)
	if out.Allowed {
		fmt.Printf("  result: %s\n", out.Result)
	} else {
		fmt.Printf("  %s\n", out.Refusal)
	}
	if !full {
		return
	}
	if out.Err != nil {
		return
	}
	fmt.Printf("  cost: %d steps, %d bytes; evidence hash %s\n", out.Decision.Steps, out.Decision.Bytes, out.Decision.Hash)
	if out.Allowed {
		fmt.Println(indent(out.Decision.Proof.String()))
	} else {
		fmt.Println(indent(out.Decision.Why.String()))
	}
}

func indent(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = "    " + l
	}
	return strings.Join(lines, "\n")
}

// showMatrix is every tool against five subjects: the principal, a direct report, a
// report's report, a peer and the manager. Attested as a clean chat turn with MFA and
// confirmation, so what remains is the relationship and the channel.
func showMatrix(ctx context.Context, g *gate.Gate, id, channel string) {
	e, ok := g.Org.Get(id)
	if !ok {
		fail(fmt.Errorf("no employee %s", id))
	}
	cols := []struct{ label, id string }{{"self", id}}
	if r := g.Org.Reports(id); len(r) > 0 {
		cols = append(cols, struct{ label, id string }{"report", r[0].ID})
		if rr := g.Org.Reports(r[0].ID); len(rr) > 0 {
			cols = append(cols, struct{ label, id string }{"skip", rr[0].ID})
		}
	}
	if e.ManagerID != "" {
		for _, p := range g.Org.Reports(e.ManagerID) {
			if p.ID != id {
				cols = append(cols, struct{ label, id string }{"peer", p.ID})
				break
			}
		}
		cols = append(cols, struct{ label, id string }{"manager", e.ManagerID})
	}
	fmt.Printf("%-30s", id+" via "+channel)
	for _, c := range cols {
		fmt.Printf(" %-8s", c.label)
	}
	fmt.Println()
	s := gate.Session{EmployeeID: id, Channel: channel, MFA: true}
	a := gate.Attestation{Guardrail: "clean", Confirmed: true, Requester: "E0000"}
	for _, t := range catalog.Tools {
		fmt.Printf("%-30s", t.Name)
		for _, c := range cols {
			out := g.Call(ctx, s, gate.Call{Tool: t.Name, Args: map[string]string{"employee_id": c.id, "value": "x", "request_id": "x"}}, a)
			mark := "."
			if out.Allowed {
				mark = "ALLOW"
			}
			fmt.Printf(" %-8s", mark)
		}
		fmt.Println()
	}
}

func runScenarios(ctx context.Context, g *gate.Gate) {
	cast := gate.FindCast(g.Org)
	pass := 0
	scenarios := gate.Scenarios(cast)
	for _, sc := range scenarios {
		out := g.Call(ctx, sc.Session, sc.Call, sc.Attest)
		ok := out.Allowed == sc.Allow && (sc.Allow || strings.Contains(out.Refusal, sc.Reason))
		mark := "ok  "
		if !ok {
			mark = "FAIL"
		} else {
			pass++
		}
		fmt.Printf("%s %-68s ", mark, sc.Name)
		if out.Allowed {
			fmt.Printf("ALLOW  %s\n", out.Result)
		} else {
			fmt.Printf("DENY   %s\n", out.Refusal)
		}
	}
	fmt.Printf("\n%d of %d scenarios as expected\n\n", pass, len(scenarios))
	rep, err := audit.Run(ctx, g.Ledger())
	if err != nil {
		fail(err)
	}
	worst := gate.Entry{}
	for _, e := range g.Ledger() {
		if e.Margin > worst.Margin {
			worst = e
		}
	}
	b := g.Policy.Bounds()
	fmt.Printf("largest decision: %d steps, %d bytes, %d%% of a bound (%d steps, %d bytes); alert past 25%%\n",
		worst.Steps, worst.Bytes, worst.Margin, b.MaxSteps, b.MaxBytes)
	fmt.Printf("audit over %d ledger rows (%d steps):\n", len(g.Ledger()), rep.Steps)
	if len(rep.Findings) == 0 {
		fmt.Println("  nothing to report")
	}
	for _, f := range rep.Findings {
		fmt.Printf("  %s%s\n", f.Relation, hrpolicy.TupleText(f.Tuple))
	}
}
