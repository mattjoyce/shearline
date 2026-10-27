package hrpolicy

import (
	"fmt"
	"strings"

	"github.com/mattjoyce/tarski"
)

// Explain is one line a chatbot may relay for a refusal: the farthest point the policy
// reached. Among the rules that could have derived a tuple, the ones that got farthest
// through their body are the ones worth naming: a self path that reached cleared(A)
// and found MFA missing explains more than a manager path that found no report. A
// denial a deny rule blocked names the blocking tuple. The full why-not is
// Decision.Why; this is its headline.
func Explain(d Decision) string {
	if d.Allow {
		return "allowed"
	}
	leaves := leaves(d.Why, 0)
	if len(leaves) == 0 {
		return "refused: no rule reaches allow"
	}
	return "refused: " + strings.Join(dedupe(leaves), "; ")
}

// leaves walks an absence to what stopped it, in the order a reader wants: a deny rule
// that blocked the request; else a path whose relationship held but whose attestation
// did not, descended to the attestation; else the literal each path stopped at, which
// together say there was no path. Shearline's own grants rule is skipped when a rule of
// this policy explains more, since no grants/3 facts are loaded.
func leaves(a tarski.Absence, depth int) []string {
	var blockedBy, attest, rest []tarski.RuleFailure
	for _, rf := range a.Rules {
		switch {
		case rf.Blocked != nil:
			blockedBy = append(blockedBy, rf)
		case rf.Failed.Atom != nil && rf.Failed.Atom.Name == "cleared":
			attest = append(attest, rf)
		default:
			rest = append(rest, rf)
		}
	}
	var out []string
	switch {
	case len(blockedBy) > 0:
		for _, rf := range blockedBy {
			out = append(out, blocked(*rf.Blocked))
		}
	case len(attest) > 0:
		for _, rf := range attest {
			out = append(out, descend(rf, depth)...)
		}
	default:
		ours := false
		for _, rf := range rest {
			ours = ours || (rf.Rule != nil && rf.Rule.Set != "rbac/1")
		}
		var stops []string
		for _, rf := range rest {
			if ours && rf.Rule != nil && rf.Rule.Set == "rbac/1" {
				continue
			}
			stops = append(stops, descend(rf, depth)...)
		}
		if depth == 0 && len(stops) > 1 {
			return []string{"no path: " + strings.Join(dedupe(stops), " / ")}
		}
		out = append(out, stops...)
	}
	return out
}

// descend explains one rule failure: through Because when the failed literal is derived,
// else the literal itself.
func descend(rf tarski.RuleFailure, depth int) []string {
	if len(rf.Because) > 0 && depth < 8 {
		var deeper []string
		for _, b := range rf.Because {
			deeper = append(deeper, leaves(b, depth+1)...)
		}
		if len(deeper) > 0 {
			return deeper
		}
	}
	if rf.Reason != "" {
		return []string{rf.Reason}
	}
	return []string{"stopped at " + literal(rf.Failed, rf.Bound)}
}

func dedupe(s []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// blocked names the tuple that held against a negation and, for a derived one, the
// base facts its proof rests on: unmet("read.bank") rests on needs("read.bank", "mfa").
func blocked(p tarski.Proof) string {
	s := "blocked by " + p.Relation + TupleText(p.Tuple)
	var facts []string
	for _, b := range p.Body {
		if !requestRelations[b.Relation] {
			facts = append(facts, b.Relation+TupleText(b.Tuple))
		}
	}
	if len(facts) > 0 {
		s += " [" + strings.Join(facts, ", ") + "]"
	}
	if p.Rule != nil {
		s += " by " + strings.TrimSpace(p.Rule.Text)
	}
	return s
}

func literal(l tarski.Literal, bound map[string]tarski.Const) string {
	switch {
	case l.Compare != nil:
		return term(l.Compare.Left, bound) + " " + l.Compare.Op + " " + term(l.Compare.Right, bound)
	case l.Atom != nil:
		args := make([]string, len(l.Atom.Args))
		for i, t := range l.Atom.Args {
			args[i] = term(t, bound)
		}
		s := l.Atom.Name + "(" + strings.Join(args, ", ") + ")"
		if l.Negated {
			return "not " + s
		}
		return s
	}
	return "aggregate"
}

func term(t tarski.Term, bound map[string]tarski.Const) string {
	if t.IsVar() {
		if c, ok := bound[t.Var]; ok {
			return constText(c)
		}
		return t.Var
	}
	return constText(t.Const)
}

func constText(c tarski.Const) string {
	if c.Kind() == tarski.KindString {
		return fmt.Sprintf("%q", c.Text())
	}
	return c.Text()
}

// TupleText renders a tuple the way a fact prints: strings quoted, the rest bare.
func TupleText(t tarski.Tuple) string {
	parts := make([]string, len(t))
	for i, c := range t {
		parts[i] = constText(c)
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// requestRelations are the facts a request supplies; a headline names the standing facts
// behind a block, not the request that met them.
var requestRelations = map[string]bool{"principal": true, "action": true, "resource": true, "context": true}
