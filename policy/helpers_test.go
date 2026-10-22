package policy

import (
	"testing"

	"github.com/mattjoyce/shearline/datalog"
)

// fixture: ana is an admin, admin inherits editor, editor inherits reader.
func fixture() []datalog.Fact {
	return []datalog.Fact{
		Role("ana", "admin"),
		Role("bo", "reader"),
		Inherits("admin", "editor"),
		Inherits("editor", "reader"),
		Grant("reader", "read", "timeline/"),
		Grant("reader", "read", "docs/"),
		Grant("editor", "read", "docs/"), // same prefix by a second route
		Grant("editor", "write", "timeline/"),
		Grant("admin", "delete", "timeline/"),
	}
}

func mustLoad(t *testing.T, standing []datalog.Fact, sets ...datalog.RuleSet) Policy {
	t.Helper()
	return mustLoadConfig(t, Config{Standing: standing, Sets: sets})
}

func mustLoadConfig(t *testing.T, cfg Config) Policy {
	t.Helper()
	p, err := Load(cfg)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return p
}

func rbacOnly() []datalog.RuleSet { return []datalog.RuleSet{RBAC()} }

func ruleSet(t *testing.T, name, text string) datalog.RuleSet {
	t.Helper()
	s, err := datalog.NewRuleSet(name, 1, text)
	if err != nil {
		t.Fatalf("NewRuleSet: %v", err)
	}
	return s
}

// reaches reports whether the chain of absences ends at a literal over relation. A base
// relation is not an Absence of its own: it is the Failed literal of the rule that read
// it, so both are looked at.
func reaches(a datalog.Absence, relation string) bool {
	if a.Relation == relation {
		return true
	}
	for _, r := range a.Rules {
		if r.Failed.Atom != nil && r.Failed.Atom.Name == relation {
			return true
		}
		for _, b := range r.Because {
			if reaches(b, relation) {
				return true
			}
		}
	}
	return false
}

// failures lists every RuleFailure under a, depth first.
func failures(a datalog.Absence) []datalog.RuleFailure {
	var out []datalog.RuleFailure
	for _, r := range a.Rules {
		out = append(out, r)
		for _, b := range r.Because {
			out = append(out, failures(b)...)
		}
	}
	return out
}

func proofLeaves(p datalog.Proof, into map[string]bool) {
	if p.Rule == nil {
		into[p.Relation] = true
		return
	}
	for _, b := range p.Body {
		proofLeaves(b, into)
	}
}
