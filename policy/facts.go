package policy

import (
	"fmt"
	"sort"

	"github.com/mattjoyce/shearline/datalog"
)

// Schema is the PARC base relations. A consumer's standing facts use role, inherits
// and grants; a request supplies principal, action, resource and context.
var Schema = datalog.Schema{
	Arities: map[string]int{
		"principal": 1, "action": 1, "resource": 1, "context": 2,
		"role": 2, "inherits": 2, "grants": 3, "denied": 3,
	},
	Checks: map[string]func(datalog.Atom) error{
		"principal": nonEmptyString, "action": nonEmptyString, "resource": nonEmptyString,
		"context": nonEmptyString, "role": nonEmptyString, "inherits": nonEmptyString,
		"grants": nonEmptyString, "denied": nonEmptyString,
	},
}

// nonEmptyString requires a non-empty string constant in every position of an atom
// except the value of context(Key, Value). A number or boolean there is the wrong kind
// of value, and an empty principal, role, action or prefix is a missing value, not a
// wildcard: an empty prefix would match every resource. Variables pass; the check
// sees rule atoms too.
func nonEmptyString(a datalog.Atom) error {
	for i, t := range a.Args {
		if a.Name == "context" && i == 1 {
			continue
		}
		if t.IsVar() {
			continue
		}
		if t.Const.Kind() != datalog.KindString {
			return fmt.Errorf("%s argument %d must be a string, not a number or boolean", a.Name, i+1)
		}
		if t.Const.Text() == "" {
			return fmt.Errorf("%s argument %d is an empty string; that is a missing value, not a wildcard", a.Name, i+1)
		}
	}
	return nil
}

func fact(relation string, args ...string) datalog.Fact {
	t := make(datalog.Tuple, len(args))
	for i, a := range args {
		t[i] = datalog.String(a)
	}
	return datalog.Fact{Relation: relation, Tuple: t}
}

// Role says principal holds role.
func Role(principal, role string) datalog.Fact { return fact("role", principal, role) }

// Inherits says role also holds everything parent grants.
func Inherits(role, parent string) datalog.Fact { return fact("inherits", role, parent) }

// Grant says role may take action on every resource whose name starts with prefix. The
// prefix is a plain string prefix with no path semantics, so a consumer whose names are
// segmented should end it with the separator ("timeline/", not "timeline"), since
// "timeline/health" also matches "timeline/healthy".
func Grant(role, action, prefix string) datalog.Fact { return fact("grants", role, action, prefix) }

// Deny says principal may not take action on resource, whatever it is granted. It is a
// denied/3 fact; rbac.dl derives deny/3 from it. It denies the exact resource
// string, not a subtree; a subtree denial is a consumer rule, for example
// deny(P, A, R) :- principal(P), action(A), resource(R), prefix("timeline/health", R).
func Deny(principal, action, resource string) datalog.Fact {
	return fact("denied", principal, action, resource)
}

// facts builds the facts of one request, context keys sorted so the base, and any
// proof drawn from it, does not depend on map order.
func (r Request) facts() []datalog.Fact {
	fs := []datalog.Fact{
		fact("principal", r.Principal),
		fact("action", r.Action),
		fact("resource", r.Resource),
	}
	keys := make([]string, 0, len(r.Context))
	for k := range r.Context {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fs = append(fs, datalog.Fact{Relation: "context", Tuple: datalog.Tuple{datalog.String(k), r.Context[k]}})
	}
	return fs
}
