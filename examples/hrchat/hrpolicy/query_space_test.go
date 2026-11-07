package hrpolicy

import (
	"slices"
	"strings"
	"testing"

	"github.com/mattjoyce/shearline/examples/hrchat/catalog"
	"github.com/mattjoyce/shearline/examples/hrchat/org"
	"github.com/mattjoyce/shearline/policy"
	"github.com/mattjoyce/tarski"
)

// propertyPrincipals selects 30 representatives in employee order.
// It includes every HR partner and payroll officer.
func propertyPrincipals(t *testing.T, o *org.Org) []*org.Employee {
	t.Helper()
	selected := map[string]bool{o.Employees[0].ID: true}
	roles := map[string]bool{}
	for _, e := range o.Employees {
		key := ""
		switch {
		case strings.HasPrefix(e.Title, "Director"):
			key = "director"
		case e.Title == "Manager":
			key = "manager"
		case !strings.HasPrefix(e.Title, "Chief") && len(e.PartnerOf) == 0 && len(e.Roles) == 0:
			key = "ic/" + e.Dept
		}
		if key != "" && !roles[key] {
			selected[e.ID], roles[key] = true, true
		}
		if len(e.PartnerOf) > 0 || len(e.Roles) > 0 {
			selected[e.ID] = true
		}
	}
	for _, e := range o.Employees {
		if len(selected) < 30 && e.Title == "Manager" {
			selected[e.ID] = true
		}
	}
	var out []*org.Employee
	for _, e := range o.Employees {
		if selected[e.ID] {
			out = append(out, e)
		}
	}
	if len(out) != 30 {
		t.Fatalf("%d principals, want 30", len(out))
	}
	return out
}

func propertyResources(t *testing.T, o *org.Org, p *org.Employee) []string {
	t.Helper()
	out := []string{}
	add := func(id string) {
		if id != "" && len(out) < 10 && !slices.Contains(out, org.Resource(id)) {
			out = append(out, org.Resource(id))
		}
	}
	add(p.ID)
	for _, id := range o.Chain(p.ID) {
		add(id)
		if len(out) == 3 {
			break
		}
	}
	if reports := o.Reports(p.ID); len(reports) > 0 {
		add(reports[0].ID)
		if next := o.Reports(reports[0].ID); len(next) > 0 {
			add(next[0].ID)
		}
	}
	for _, match := range []func(*org.Employee) bool{
		func(e *org.Employee) bool { return e.ManagerID == p.ManagerID },
		func(e *org.Employee) bool { return e.Dept != p.Dept },
		func(e *org.Employee) bool { return len(e.PartnerOf) > 0 },
		func(e *org.Employee) bool { return len(e.Roles) > 0 },
	} {
		for _, e := range o.Employees {
			if e.ID != p.ID && match(e) {
				add(e.ID)
				break
			}
		}
	}
	for _, e := range o.Employees {
		add(e.ID)
	}
	if len(out) != 10 {
		t.Fatalf("%s has %d resources, want 10", p.ID, len(out))
	}
	return out
}

func propertyBags(principal string, resources []string) []map[string]tarski.Const {
	var out []map[string]tarski.Const
	for _, requester := range append([]string{"", principal}, resources...) {
		for _, channel := range []string{"chat", "web"} {
			for _, guardrail := range []string{"clean", "flagged"} {
				for _, mfa := range []bool{false, true} {
					for _, confirmed := range []bool{false, true} {
						c := Context{Channel: channel, Guardrail: guardrail, MFA: mfa, Confirmed: confirmed, Requester: requester}
						out = append(out, c.consts())
					}
				}
			}
		}
	}
	return out
}

func propertySpaces(t *testing.T, o *org.Org) []policy.Space {
	t.Helper()
	var out []policy.Space
	for _, p := range propertyPrincipals(t, o) {
		principal := org.Resource(p.ID)
		resources := propertyResources(t, o, p)
		out = append(out, policy.Space{Principals: []string{principal}, Actions: catalog.ActionNames(),
			Resources: resources, Contexts: propertyBags(principal, resources)})
	}
	return out
}

// requestAt maps the 48-bag loop to Query's resource-independent bags.
func requestAt(s policy.Space, resource, action, variant int) (policy.Request, int) {
	requester, grid := variant/16, variant%16
	bag := grid + requester*16
	if requester == 2 {
		bag = 32 + resource*16 + grid
	}
	return policy.Request{Principal: s.Principals[0], Action: s.Actions[action], Resource: s.Resources[resource], Context: s.Contexts[bag]}, bag
}
