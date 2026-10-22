package policy

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/mattjoyce/shearline/datalog"
)

func TestPermitted(t *testing.T) {
	tests := []struct {
		name              string
		principal, action string
		want              []string
	}{
		{"deduped across inherited roles, sorted", "ana", "read", []string{"docs/", "timeline/"}},
		{"direct role", "bo", "read", []string{"docs/", "timeline/"}},
		{"inherited action", "ana", "write", []string{"timeline/"}},
		{"action not granted", "bo", "delete", []string{}},
		{"unknown principal", "zed", "read", []string{}},
	}
	p := mustLoad(t, fixture(), RBAC())
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := p.Permitted(context.Background(), tc.principal, tc.action)
			if err != nil {
				t.Fatalf("Permitted: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPermitted_IgnoresDeny(t *testing.T) {
	p := mustLoad(t, append(fixture(), Deny("bo", "read", "timeline/secret")), RBAC())
	got, err := p.Permitted(context.Background(), "bo", "read")
	if err != nil || !reflect.DeepEqual(got, []string{"docs/", "timeline/"}) {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestPermitted_OnlyTheCallersTuples(t *testing.T) {
	// A rule set may derive permit tuples for any principal; the listing keeps the caller's.
	stray := ruleSet(t, "stray", `permit("other", "read", "secret/"). permit("bo", "write", "elsewhere/").`)
	p := mustLoad(t, fixture(), RBAC(), stray)
	got, err := p.Permitted(context.Background(), "bo", "read")
	if err != nil || !reflect.DeepEqual(got, []string{"docs/", "timeline/"}) {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestPermitted_ThirdColumnContract(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"number", `permit(P, A, 7) :- principal(P), action(A).`},
		{"empty string", `permit(P, A, "") :- principal(P), action(A).`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := mustLoad(t, fixture(), RBAC(), ruleSet(t, "bad", tc.text))
			got, err := p.Permitted(context.Background(), "bo", "read")
			if !errors.Is(err, datalog.ErrInvalid) || got != nil {
				t.Fatalf("got %v, %v; want nil and ErrInvalid", got, err)
			}
		})
	}
}
