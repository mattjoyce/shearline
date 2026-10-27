package policy

import (
	"fmt"
	"sort"

	"github.com/mattjoyce/tarski"
)

// Schema is the PARC base relations. A consumer's standing facts use role, inherits
// and grants; a request supplies principal, action, resource and context. Each call
// returns a fresh value, so no caller can change what later Loads compile against. This
// one declares no context keys, so it refuses any; Load builds its own from Config.Context.
func Schema() tarski.Schema { return schema(nil) }

// schema is Schema with the context keys a policy declares: context/2 is held to them, so
// a rule or a request naming any other key, or a declared key at another kind, is refused.
func schema(declared map[string]tarski.Kind) tarski.Schema {
	return schemaWithRelations(declared, nil)
}

func schemaWithRelations(declared map[string]tarski.Kind, declaredRelations map[string]Relation) tarski.Schema {
	aritities := map[string]int{
		"principal": 1, "action": 1, "resource": 1, "context": 2,
		"role": 2, "inherits": 2, "grants": 3, "denied": 3,
	}
	checks := map[string]func(tarski.Atom) error{
		"principal": nonEmptyString, "action": nonEmptyString, "resource": nonEmptyString,
		"context": contextCheck(declared), "role": nonEmptyString, "inherits": nonEmptyString,
		"grants": nonEmptyString, "denied": nonEmptyString,
	}
	if declaredRelations != nil {
		for name, rel := range declaredRelations {
			aritities[name] = len(rel.Kinds)
			checks[name] = consumerRelationCheck(rel)
		}
	}
	return tarski.Schema{
		Arities: aritities,
		Checks:  checks,
	}
}

func consumerRelationCheck(rel Relation) func(tarski.Atom) error {
	return func(a tarski.Atom) error {
		for i, t := range a.Args {
			if i >= len(rel.Kinds) {
				break
			}
			want := rel.Kinds[i]
			if t.IsVar() {
				continue
			}
			if t.Const.Kind() != want {
				return fmt.Errorf("%s argument %d must be a %s, not a %s", a.Name, i+1, kindName(want), kindName(t.Const.Kind()))
			}
			if want == tarski.KindString && t.Const.Text() == "" {
				return fmt.Errorf("%s argument %d is an empty string; that is a missing value, not a wildcard", a.Name, i+1)
			}
		}
		return nil
	}
}

// nonEmptyString requires a non-empty string constant in every position of an atom
// except the value of context(Key, Value). A number or boolean there is the wrong kind
// of value, and an empty principal, role, action or prefix is a missing value, not a
// wildcard: an empty prefix would match every resource. Variables pass; the check
// sees rule atoms too.
func nonEmptyString(a tarski.Atom) error {
	for i, t := range a.Args {
		if a.Name == "context" && i == 1 {
			continue
		}
		if t.IsVar() {
			continue
		}
		if t.Const.Kind() != tarski.KindString {
			return fmt.Errorf("%s argument %d must be a string, not a number or boolean", a.Name, i+1)
		}
		if t.Const.Text() == "" {
			return fmt.Errorf("%s argument %d is an empty string; that is a missing value, not a wildcard", a.Name, i+1)
		}
	}
	return nil
}

func fact(relation string, args ...string) tarski.Fact {
	t := make(tarski.Tuple, len(args))
	for i, a := range args {
		t[i] = tarski.String(a)
	}
	return tarski.Fact{Relation: relation, Tuple: t}
}

// Role says principal holds role.
func Role(principal, role string) tarski.Fact { return fact("role", principal, role) }

// Inherits says role also holds everything parent grants.
func Inherits(role, parent string) tarski.Fact { return fact("inherits", role, parent) }

// Grant says role may take action on every resource whose name starts with prefix. The
// prefix is a plain string prefix with no path semantics, so a consumer whose names are
// segmented should end it with the separator ("timeline/", not "timeline"), since
// "timeline/health" also matches "timeline/healthy".
func Grant(role, action, prefix string) tarski.Fact { return fact("grants", role, action, prefix) }

// Deny says principal may not take action on resource, whatever it is granted. It is a
// denied/3 fact; rbac.dl derives deny/3 from it. It denies the exact resource
// string, not a subtree; a subtree denial is a consumer rule, for example
// deny(P, A, R) :- principal(P), action(A), resource(R), prefix("timeline/health", R).
func Deny(principal, action, resource string) tarski.Fact {
	return fact("denied", principal, action, resource)
}

// facts builds the facts of one request, context keys sorted so the base, and any
// proof drawn from it, does not depend on map order.
// kindName is a constant kind in words, for a message.
func kindName(k tarski.Kind) string {
	switch k {
	case tarski.KindString:
		return "string"
	case tarski.KindNumber:
		return "number"
	case tarski.KindBool:
		return "boolean"
	}
	return fmt.Sprintf("kind(%d)", k)
}

// contextCheck holds context(Key, Value) to the declared keys. The key must be a
// non-empty string and declared; a constant value must be of the declared kind. A variable
// passes in either position, since only a constant can be wrong.
func contextCheck(declared map[string]tarski.Kind) func(tarski.Atom) error {
	return func(a tarski.Atom) error {
		if len(a.Args) != 2 || a.Args[0].IsVar() {
			return nil
		}
		if err := nonEmptyString(tarski.Atom{Name: a.Name, Args: a.Args[:1]}); err != nil {
			return err
		}
		key := a.Args[0].Const.Text()
		want, ok := declared[key]
		if !ok {
			return fmt.Errorf("context key %q is not declared in Config.Context", key)
		}
		if v := a.Args[1]; !v.IsVar() && v.Const.Kind() != want {
			return fmt.Errorf("context key %q is declared %s, but this value is a %s", key, kindName(want), kindName(v.Const.Kind()))
		}
		return nil
	}
}

func (r Request) facts() []tarski.Fact {
	fs := []tarski.Fact{
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
		fs = append(fs, tarski.Fact{Relation: "context", Tuple: tarski.Tuple{tarski.String(k), r.Context[k]}})
	}
	return fs
}
