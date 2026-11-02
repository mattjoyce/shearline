package hrpolicy

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/mattjoyce/shearline/examples/hrchat/catalog"
	"github.com/mattjoyce/shearline/examples/hrchat/org"
	"github.com/mattjoyce/shearline/policy"
	"github.com/mattjoyce/tarski"
)

// loadShearline loads the policy as Load does, with each entitlement fact keep refuses
// left out: a mutant, as a second author's slip would make one.
func loadShearline(t *testing.T, o *org.Org, keep func(tarski.Fact) bool) policy.Policy {
	t.Helper()
	ours, err := Rules()
	if err != nil {
		t.Fatal(err)
	}
	var ent []tarski.Fact
	for _, f := range entitlementFacts() {
		if keep(f) {
			ent = append(ent, f)
		}
	}
	p, err := policy.Load(policy.Config{
		Standing:  slices.Concat(o.Facts(), catalog.Facts(), ent),
		Sets:      []tarski.RuleSet{policy.RBAC(), ours},
		Context:   Keys(),
		Relations: Relations(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestCompare_MissingFourEyesShowsOnlySelfApprovals: compare the policy with a mutant
// that lost approve.salary's four-eyes attestation. Over every HR partner, a few members
// of each department they partner, three actions and three requesters, the only diffs
// are pay approvals where the approver raised the change or nobody is recorded. Each diff
// says why: the policy's why-not, reduced by Explain, names four_eyes.
func TestCompare_MissingFourEyesShowsOnlySelfApprovals(t *testing.T) {
	o := org.Generate(1)
	full := loadShearline(t, o, func(tarski.Fact) bool { return true })
	mutant := loadShearline(t, o, func(f tarski.Fact) bool { return f.Relation != "also_needs" })

	members := map[string][]string{} // dept -> the first three employees in it
	for _, e := range o.Employees {
		if len(members[e.Dept]) < 3 {
			members[e.Dept] = append(members[e.Dept], e.ID)
		}
	}
	var reqs []policy.Request
	for _, hr := range o.Employees {
		for _, d := range hr.PartnerOf {
			for _, s := range members[d] {
				if s == hr.ID {
					continue
				}
				for _, a := range []string{"approve.salary", "read.salary", "approve.leave"} {
					for _, requester := range []string{"", org.Resource(hr.ID), org.Resource(s)} {
						c := Context{Channel: "web", Guardrail: "clean", MFA: true, Confirmed: true}
						c.Requester = requester // consts sends it only when set; a resource name, as the gate sends it
						reqs = append(reqs, policy.Request{
							Principal: org.Resource(hr.ID), Action: a, Resource: org.Resource(s), Context: c.consts(),
						})
					}
				}
			}
		}
	}
	if len(reqs) == 0 {
		t.Fatal("no HR partner requests: the org has no partners")
	}

	diffs, err := policy.Compare(context.Background(), full, mutant, slices.Values(reqs))
	if err != nil {
		t.Fatal(err)
	}
	// Two diffs (no requester, self as requester) for every pair where the policy approves
	// with a proper requester. A pair denied for another reason (the subject is above the
	// partner in their own reporting line) is denied by both, so it is no diff.
	approvable := 0
	for _, r := range reqs {
		if q, ok := r.Context["requester"]; ok && r.Action == "approve.salary" && q.Text() == r.Resource {
			d, err := full.Decide(context.Background(), r)
			if err != nil {
				t.Fatal(err)
			}
			if d.Allow {
				approvable++
			}
		}
	}
	if approvable == 0 {
		t.Fatal("no approvable pay change: the test proves nothing")
	}
	if want := approvable * 2; len(diffs) != want {
		t.Fatalf("%d diffs over %d requests, want %d", len(diffs), len(reqs), want)
	}
	for _, d := range diffs {
		req, recorded := d.Request.Context["requester"]
		bad := !recorded || req.Text() == d.Request.Principal
		if d.Request.Action != "approve.salary" || !bad {
			t.Fatalf("unexpected diff: %+v", d.Request)
		}
		if d.ErrA != nil || d.ErrB != nil || d.A.Allow || !d.B.Allow {
			t.Fatalf("diff %+v: want the policy to deny and the mutant to allow", d.Request)
		}
		if why := Explain(Decision{Decision: d.A}); !strings.Contains(why, `"four_eyes")`) {
			t.Fatalf("diff %+v: the policy's why-not is %q, want four_eyes", d.Request, why)
		}
	}
}
