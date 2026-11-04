package builtins

import (
	"strings"
	"testing"

	"github.com/mattjoyce/tarski"
)

func s(v string) tarski.Const { return tarski.String(v) }

func num(t *testing.T, v string) tarski.Const {
	t.Helper()
	c, err := tarski.Number(v)
	if err != nil {
		t.Fatalf("Number(%q): %v", v, err)
	}
	return c
}

// Every case runs Holds and never panics: a wrong kind or shape is false.
func TestHolds(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		b    tarski.Builtin
		args []tarski.Const
		want bool
	}{
		{"v4 inside", InCIDR, []tarski.Const{s("10.1.2.3"), s("10.0.0.0/8")}, true},
		{"v4 outside", InCIDR, []tarski.Const{s("192.168.0.1"), s("10.0.0.0/8")}, false},
		{"v4 edge", InCIDR, []tarski.Const{s("10.255.255.255"), s("10.0.0.0/8")}, true},
		{"v4 mapped", InCIDR, []tarski.Const{s("::ffff:10.1.2.3"), s("10.0.0.0/8")}, true},
		{"v6 inside", InCIDR, []tarski.Const{s("2001:db8::1"), s("2001:db8::/32")}, true},
		{"v4 against v6", InCIDR, []tarski.Const{s("10.1.2.3"), s("2001:db8::/32")}, false},
		{"host bits set", InCIDR, []tarski.Const{s("10.1.2.3"), s("10.1.0.0/8")}, false},
		{"not an address", InCIDR, []tarski.Const{s("ten"), s("10.0.0.0/8")}, false},
		{"not a network", InCIDR, []tarski.Const{s("10.1.2.3"), s("10.0.0.0")}, false},
		{"empty", InCIDR, []tarski.Const{s(""), s("")}, false},
		{"number", InCIDR, []tarski.Const{num(t, "10"), s("10.0.0.0/8")}, false},
		{"bool", InCIDR, []tarski.Const{s("10.1.2.3"), tarski.Bool(true)}, false},

		{"before", Before, []tarski.Const{s("2026-11-04T09:00:00Z"), s("2026-11-04T09:00:01Z")}, true},
		{"after", Before, []tarski.Const{s("2026-11-04T09:00:01Z"), s("2026-11-04T09:00:00Z")}, false},
		{"equal is not before", Before, []tarski.Const{s("2026-11-04T09:00:00Z"), s("2026-11-04T09:00:00Z")}, false},
		{"zones are instants", Before, []tarski.Const{s("2026-11-04T19:00:00+11:00"), s("2026-11-04T08:30:00Z")}, true},
		{"fractions", Before, []tarski.Const{s("2026-11-04T09:00:00.1Z"), s("2026-11-04T09:00:00.2Z")}, true},
		{"date only", Before, []tarski.Const{s("2026-11-04"), s("2026-11-05T00:00:00Z")}, false},
		{"number time", Before, []tarski.Const{num(t, "1"), s("2026-11-05T00:00:00Z")}, false},

		{"inside", Within, []tarski.Const{s("2026-11-04T09:00:00Z"), s("2026-11-04T09:10:00Z"), s("15m")}, true},
		{"at the limit", Within, []tarski.Const{s("2026-11-04T09:00:00Z"), s("2026-11-04T09:15:00Z"), s("15m")}, true},
		{"past the limit", Within, []tarski.Const{s("2026-11-04T09:00:00Z"), s("2026-11-04T09:15:01Z"), s("15m")}, false},
		{"same instant", Within, []tarski.Const{s("2026-11-04T09:00:00Z"), s("2026-11-04T09:00:00Z"), s("0s")}, true},
		{"end before start", Within, []tarski.Const{s("2026-11-04T09:10:00Z"), s("2026-11-04T09:00:00Z"), s("15m")}, false},
		{"negative max", Within, []tarski.Const{s("2026-11-04T09:00:00Z"), s("2026-11-04T09:00:00Z"), s("-1m")}, false},
		{"bad max", Within, []tarski.Const{s("2026-11-04T09:00:00Z"), s("2026-11-04T09:00:00Z"), s("soon")}, false},
		{"number max", Within, []tarski.Const{s("2026-11-04T09:00:00Z"), s("2026-11-04T09:00:00Z"), num(t, "900")}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := c.b.Holds(c.args); got != c.want {
				t.Errorf("Holds(%v) = %v, want %v", c.args, got, c.want)
			}
		})
	}
}

// Check refuses a constant of the wrong shape and passes a variable (nil) and a good one.
func TestCheck(t *testing.T) {
	t.Parallel()
	p := func(c tarski.Const) *tarski.Const { return &c }
	cases := []struct {
		name   string
		b      tarski.Builtin
		consts []*tarski.Const
		want   string // "" is no error
	}{
		{"vars", InCIDR, []*tarski.Const{nil, nil}, ""},
		{"good network", InCIDR, []*tarski.Const{nil, p(s("10.0.0.0/8"))}, ""},
		{"bad network", InCIDR, []*tarski.Const{nil, p(s("not-a-cidr"))}, `Network: "not-a-cidr"`},
		{"host bits", InCIDR, []*tarski.Const{nil, p(s("10.1.0.0/8"))}, `Network: "10.1.0.0/8"`},
		{"bad addr", InCIDR, []*tarski.Const{p(s("ten")), nil}, `Addr: "ten"`},
		{"number network", InCIDR, []*tarski.Const{nil, p(num(t, "8"))}, `Network: "8"`},
		{"good times", Before, []*tarski.Const{p(s("2026-11-04T09:00:00Z")), nil}, ""},
		{"bad B", Before, []*tarski.Const{nil, p(s("tomorrow"))}, `B: "tomorrow"`},
		{"good max", Within, []*tarski.Const{nil, nil, p(s("72h"))}, ""},
		{"negative max", Within, []*tarski.Const{nil, nil, p(s("-5m"))}, `Max: "-5m"`},
		{"bad A in within", Within, []*tarski.Const{p(s("x")), nil, nil}, `A: "x"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := c.b.Check(c.consts)
			switch {
			case c.want == "" && err != nil:
				t.Errorf("Check = %v, want nil", err)
			case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
				t.Errorf("Check = %v, want an error containing %q", err, c.want)
			}
		})
	}
}

// Each builtin documents its arguments, so a refusal can show its signature.
func TestDocumented(t *testing.T) {
	t.Parallel()
	for name, b := range map[string]tarski.Builtin{"InCIDR": InCIDR, "Before": Before, "Within": Within} {
		if b.Doc == "" || b.Holds == nil || b.Check == nil || len(b.Args) == 0 {
			t.Errorf("%s: missing Doc, Holds, Check or Args", name)
		}
		for _, a := range b.Args {
			if a.Name == "" || a.Doc == "" {
				t.Errorf("%s: an argument has no name or doc", name)
			}
		}
	}
}
