package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mattjoyce/tarski"
)

func querySpace() Space {
	return Space{Principals: []string{"ana", "bo", "zed"}, Actions: []string{"read", "write", "delete"},
		Resources: []string{"timeline/x", "docs/y", "timeline/secret", "other/z"}}
}

func queryJSON(t *testing.T, m Matrix) []byte {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func mustQuery(t *testing.T, p Policy, s Space) Matrix {
	t.Helper()
	m, err := p.Query(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func assertZeroMatrix(t *testing.T, p Policy, m Matrix) {
	t.Helper()
	if m.Allows != nil || m.Steps != 0 || m.Bytes != 0 || !slices.Equal(m.RuleSets, p.ids) {
		t.Fatalf("partial matrix: %d allows, %d steps, %d bytes, sets %v", len(m.Allows), m.Steps, m.Bytes, m.RuleSets)
	}
}

func TestQuery_Agreement(t *testing.T) {
	p := mustLoad(t, fixture(), RBAC())
	s := querySpace()
	m := mustQuery(t, p, s)
	got := map[[3]string]bool{}
	for _, a := range m.Allows {
		got[[3]string{a.Principal, a.Action, a.Resource}] = true
		if a.Proof.Relation != "allow" || !sameTuple(a.Proof.Tuple, fact("allow", a.Principal, a.Action, a.Resource).Tuple) {
			t.Fatal("allow lacks its proof")
		}
	}
	count := 0
	for _, who := range s.Principals {
		for _, action := range s.Actions {
			for _, resource := range s.Resources {
				d, err := p.Decide(context.Background(), Request{Principal: who, Action: action, Resource: resource})
				if err != nil || d.Allow != got[[3]string{who, action, resource}] {
					t.Fatalf("%s/%s/%s: decide=%v query=%v err=%v", who, action, resource, d.Allow, !d.Allow, err)
				}
				if d.Allow {
					count++
				}
			}
		}
	}
	if len(m.Allows) != count || m.Steps == 0 || m.Bytes == 0 {
		t.Fatalf("matrix has %d allows, want %d; cost %d/%d", len(m.Allows), count, m.Steps, m.Bytes)
	}
}

func TestQuery_DenyApplies(t *testing.T) {
	p := mustLoad(t, append(fixture(), Deny("bo", "read", "timeline/secret")), RBAC())
	s := Space{Principals: []string{"bo"}, Actions: []string{"read"}, Resources: []string{"timeline/secret", "timeline/x"}}
	m := mustQuery(t, p, s)
	if len(m.Allows) != 1 || m.Allows[0].Resource != "timeline/x" {
		t.Fatalf("allows=%v", m.Allows)
	}
}

func TestQuery_Bags(t *testing.T) {
	rules := ruleSet(t, "mfa", `permit(P, A, R) :- principal(P), action(A), resource(R), context("mfa", true).`)
	p := mustLoadConfig(t, Config{Sets: []tarski.RuleSet{RBAC(), rules}, Context: map[string]tarski.Kind{"mfa": tarski.KindBool}})
	s := Space{Principals: []string{"bo"}, Actions: []string{"read"}, Resources: []string{"docs/y"},
		Contexts: []map[string]tarski.Const{{"mfa": tarski.Bool(true)}, {"mfa": tarski.Bool(false)}}}
	m := mustQuery(t, p, s)
	if len(m.Allows) != 1 || m.Allows[0].Bag != 0 || s.Contexts[m.Allows[0].Bag]["mfa"].Text() != "true" {
		t.Fatalf("wrong bags: %+v", m.Allows)
	}
	s.Contexts = append(s.Contexts, s.Contexts[0])
	m = mustQuery(t, p, s)
	if len(m.Allows) != 2 || m.Allows[1].Bag != 2 {
		t.Fatal("duplicate bag loses its index")
	}
	first := mustQuery(t, p, Space{Principals: s.Principals, Actions: s.Actions, Resources: s.Resources, Contexts: s.Contexts[:1]})
	if m.Steps != first.Steps || m.Bytes != first.Bytes {
		t.Fatal("cost is not the per-evaluation maximum")
	}
}

func TestQuery_OnlySpace(t *testing.T) {
	stray := ruleSet(t, "stray", `allow("other", "read", "secret/").
allow("bo", "other", "docs/y"). allow("bo", "read", "outside").
allow(7, "read", "docs/y"). allow("bo", true, "docs/y"). allow("bo", "read", 9).`)
	p := mustLoad(t, fixture(), RBAC(), stray)
	s := Space{Principals: []string{"bo", "7"}, Actions: []string{"read", "true"}, Resources: []string{"docs/y", "9"}}
	m := mustQuery(t, p, s)
	if len(m.Allows) != 1 || m.Allows[0].Principal != "bo" || m.Allows[0].Action != "read" || m.Allows[0].Resource != "docs/y" {
		t.Fatalf("outside tuples: %+v", m.Allows)
	}
}

func TestQuery_Validation(t *testing.T) {
	p := mustLoadConfig(t, Config{Standing: fixture(), Sets: rbacOnly(), Context: map[string]tarski.Kind{"mfa": tarski.KindBool}})
	for i, name := range []string{"principal", "action", "resource"} {
		for _, values := range [][]string{nil, {}, {""}} {
			s := querySpace()
			fields := []*[]string{&s.Principals, &s.Actions, &s.Resources}
			*fields[i] = values
			m, err := p.Query(context.Background(), s)
			if !errors.Is(err, ErrRequest) || !errors.Is(err, tarski.ErrInvalid) || !strings.Contains(err.Error(), name) {
				t.Fatalf("%s %v: %v", name, values, err)
			}
			assertZeroMatrix(t, p, m)
		}
	}
	for _, bag := range []map[string]tarski.Const{{"unknown": tarski.Bool(true)}, {"mfa": tarski.String("true")}} {
		s := querySpace()
		s.Contexts = []map[string]tarski.Const{nil, bag}
		m, err := p.Query(context.Background(), s)
		if !errors.Is(err, ErrRequest) || !strings.Contains(err.Error(), "bag 1") {
			t.Fatalf("bad bag: %v", err)
		}
		assertZeroMatrix(t, p, m)
	}
}

func TestQuery_DuplicatesAndCopies(t *testing.T) {
	p := mustLoad(t, fixture(), RBAC())
	s := querySpace()
	before := queryJSON(t, mustQuery(t, p, s))
	s.Principals = append(s.Principals, "bo")
	s.Actions = append(s.Actions, "read")
	s.Resources = append(s.Resources, "docs/y")
	s.Contexts = []map[string]tarski.Const{nil}
	m := mustQuery(t, p, s)
	if !slices.Equal(before, queryJSON(t, m)) {
		t.Fatal("duplicates change matrix")
	}
	m.RuleSets[0] = "changed"
	m.Allows[0].Proof.Tuple[0] = tarski.String("changed")
	if !slices.Equal(before, queryJSON(t, mustQuery(t, p, querySpace()))) {
		t.Fatal("matrix shares policy data")
	}
	if s.Principals[0] != "ana" || len(s.Principals) != 4 {
		t.Fatal("query changes input")
	}
}

func TestQuery_Determinism(t *testing.T) {
	p := mustLoadConfig(t, Config{Standing: fixture(), Sets: rbacOnly(), Context: map[string]tarski.Kind{"x": tarski.KindBool, "y": tarski.KindBool}})
	s := querySpace()
	s.Contexts = []map[string]tarski.Const{{"x": tarski.Bool(true), "y": tarski.Bool(false)}, {"x": tarski.Bool(false)}}
	first := mustQuery(t, p, s)
	slices.Reverse(s.Principals)
	slices.Reverse(s.Actions)
	slices.Reverse(s.Resources)
	s.Contexts[0] = map[string]tarski.Const{}
	s.Contexts[0]["y"], s.Contexts[0]["x"] = tarski.Bool(false), tarski.Bool(true)
	second := mustQuery(t, p, s)
	if !slices.Equal(queryJSON(t, first), queryJSON(t, second)) {
		t.Fatal("shuffled sets change JSON")
	}
	keys := make([]string, len(first.Allows))
	for i, a := range first.Allows {
		keys[i] = fmt.Sprintf("%08d\x00%s\x00%s\x00%s", a.Bag, a.Principal, a.Action, a.Resource)
	}
	if !slices.IsSorted(keys) {
		t.Fatal("allows are not in byte order")
	}
}

func TestQuery_TooLarge(t *testing.T) {
	wide := ruleSet(t, "wide", `wide(P, A, R, Q) :- principal(P), action(A), resource(R), principal(Q), context("wide", true).`)
	p := mustLoadConfig(t, Config{Standing: fixture(), Sets: []tarski.RuleSet{RBAC(), wide}, Context: map[string]tarski.Kind{"wide": tarski.KindBool}})
	s := querySpace()
	for i := range 64 {
		s.Principals = append(s.Principals, fmt.Sprintf("p%02d", i))
		s.Actions = append(s.Actions, fmt.Sprintf("a%02d", i))
		s.Resources = append(s.Resources, fmt.Sprintf("r%02d", i))
	}
	s.Contexts = []map[string]tarski.Const{{"wide": tarski.Bool(false)}, {"wide": tarski.Bool(true)}}
	m, err := p.Query(context.Background(), s)
	var large *tarski.TooLarge
	if !errors.As(err, &large) || !strings.Contains(err.Error(), "split the Space") {
		t.Fatalf("err=%v, want TooLarge", err)
	}
	assertZeroMatrix(t, p, m)
}

func TestQuery_Cancel(t *testing.T) {
	p := mustLoad(t, fixture(), RBAC())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m, err := p.Query(ctx, querySpace())
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	assertZeroMatrix(t, p, m)
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	guard := tarski.Builtin{Args: []tarski.BuiltinArg{{}}, Holds: func(args []tarski.Const) bool {
		if args[0].Text() == "2" {
			cancel()
		}
		return true
	}}
	rules := ruleSet(t, "cancel", `permit(P, A, R) :- principal(P), action(A), resource(R), context("bag", B), guard(B).`)
	p = mustLoadConfig(t, Config{Sets: []tarski.RuleSet{RBAC(), rules}, Context: map[string]tarski.Kind{"bag": tarski.KindNumber}, Builtins: map[string]tarski.Builtin{"guard": guard}})
	s := Space{Principals: []string{"bo"}, Actions: []string{"read"}, Resources: []string{"docs/y"}}
	for i := range 3 {
		s.Contexts = append(s.Contexts, map[string]tarski.Const{"bag": tarski.Int(int64(i + 1))})
	}
	m, err = p.Query(ctx, s)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	assertZeroMatrix(t, p, m)
}

func TestQuery_ContextSnapshot(t *testing.T) {
	s := querySpace()
	s.Contexts = []map[string]tarski.Const{{"bag": tarski.Int(1)}, {"bag": tarski.Int(2)}}
	original := maps.Clone(s.Contexts[1])
	guard := tarski.Builtin{Args: []tarski.BuiltinArg{{}}, Holds: func(_ []tarski.Const) bool {
		s.Contexts[1]["bag"] = tarski.Int(99)
		return true
	}}
	rules := ruleSet(t, "copy", `permit(P, A, R) :- principal(P), action(A), resource(R), context("bag", B), B < 3, guard(B).`)
	p := mustLoadConfig(t, Config{Sets: []tarski.RuleSet{RBAC(), rules}, Context: map[string]tarski.Kind{"bag": tarski.KindNumber}, Builtins: map[string]tarski.Builtin{"guard": guard}})
	m := mustQuery(t, p, s)
	if len(m.Allows) != 72 || reflect.DeepEqual(s.Contexts[1], original) {
		t.Fatal("query does not snapshot the bags before evaluation")
	}
}

func TestGolden_MatrixJSON(t *testing.T) {
	p := mustLoad(t, append(fixture(), Deny("bo", "read", "timeline/secret")), RBAC())
	s := Space{Principals: []string{"bo"}, Actions: []string{"read"}, Resources: []string{"timeline/x", "timeline/secret"}, Contexts: []map[string]tarski.Const{nil, {}}}
	got := append(queryJSON(t, mustQuery(t, p, s)), '\n')
	want, err := os.ReadFile("testdata/matrix.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Fatal("matrix JSON changes; edit the golden on purpose and list the change in CHANGELOG.md")
	}
}
