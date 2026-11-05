// Package builtins holds guards a policy may declare in policy.Config.Builtins, for the
// values that facts cannot compare: an address in a network, one time before another, a
// time within a duration of another. Each is a tarski.Builtin; the consumer chooses the
// name a rule uses:
//
//	policy.Config{
//		Context:  map[string]tarski.Kind{"ip": tarski.KindString},
//		Builtins: map[string]tarski.Builtin{"in_net": builtins.InCIDR},
//		...
//	}
//
//	allow(P, A, R) :- permit(P, A, Pat), resource(R), prefix(Pat, R),
//		context("ip", IP), in_net(IP, "10.0.0.0/8").
//
// Every argument is a string. A value of the wrong kind or shape is false, never an
// error, since it came from a request. So under not a malformed value holds: write a
// guard positively, in the rule that allows, never as not in a rule that denies. A
// constant of the wrong shape written in a rule is refused at Load, with the line.
package builtins

import (
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/mattjoyce/tarski"
)

// InCIDR is in_net(Addr, Network): Addr is an IP address inside Network, a CIDR prefix such
// as "10.0.0.0/8" or "2001:db8::/32". An IPv4 address written in IPv6 form
// ("::ffff:10.1.2.3") is read as the IPv4 address. A network with bits set past its
// length ("10.1.0.0/8") is refused at Load, since it is most often a typo.
var InCIDR = tarski.Builtin{
	Args: []tarski.BuiltinArg{
		{Name: "Addr", Doc: "an IP address, such as 10.1.2.3"},
		{Name: "Network", Doc: "a CIDR prefix, such as 10.0.0.0/8"},
	},
	Doc: "true when Addr is inside Network; a value of the wrong kind or shape is false",
	Check: func(consts []*tarski.Const) error {
		if c := consts[0]; c != nil {
			if _, ok := addr(*c); !ok {
				return fmt.Errorf("argument Addr: %q is not an IP address", c.Text())
			}
		}
		if c := consts[1]; c != nil {
			if _, ok := network(*c); !ok {
				return fmt.Errorf("argument Network: %q is not a CIDR prefix with no bits set past its length", c.Text())
			}
		}
		return nil
	},
	Holds: func(args []tarski.Const) bool {
		a, okA := addr(args[0])
		n, okN := network(args[1])
		return okA && okN && n.Contains(a)
	},
}

// Before is before(A, B): time A is strictly before time B. Both are RFC 3339 text, such
// as "2026-11-04T09:00:00+11:00"; times in different zones compare as instants.
var Before = tarski.Builtin{
	Args: []tarski.BuiltinArg{
		{Name: "A", Doc: "an RFC 3339 time"},
		{Name: "B", Doc: "an RFC 3339 time"},
	},
	Doc:   "true when A is before B; a value of the wrong kind or shape is false",
	Check: checkTimes("A", "B"),
	Holds: func(args []tarski.Const) bool {
		a, okA := instant(args[0])
		b, okB := instant(args[1])
		return okA && okB && a.Before(b)
	},
}

// Within is within(A, B, Max): time B is at or after time A, and at most Max after it. A
// and B are RFC 3339 text; Max is Go duration text, such as "15m" or "72h", and not
// negative. A token issued at A is fresh at B when within(Issued, Now, "15m") holds.
var Within = tarski.Builtin{
	Args: []tarski.BuiltinArg{
		{Name: "A", Doc: "an RFC 3339 time, the start"},
		{Name: "B", Doc: "an RFC 3339 time, the end"},
		{Name: "Max", Doc: "a Go duration, not negative, such as 15m"},
	},
	Doc: "true when B is at or after A and B minus A is at most Max; a value of the wrong kind or shape is false",
	Check: func(consts []*tarski.Const) error {
		if err := checkTimes("A", "B")(consts); err != nil {
			return err
		}
		if c := consts[2]; c != nil {
			if _, ok := duration(*c); !ok {
				return fmt.Errorf("argument Max: %q is not a Go duration of zero or more, such as 15m", c.Text())
			}
		}
		return nil
	},
	Holds: func(args []tarski.Const) bool {
		a, okA := instant(args[0])
		b, okB := instant(args[1])
		limit, okD := duration(args[2])
		if !okA || !okB || !okD || b.Before(a) {
			return false
		}
		// a + limit, not b - a: Sub saturates at the largest Duration, so a span longer
		// than that would compare equal to a Max of exactly that.
		return !b.After(a.Add(limit))
	},
}

// checkTimes refuses a constant that is not RFC 3339 text in the first len(names) positions.
func checkTimes(names ...string) func([]*tarski.Const) error {
	return func(consts []*tarski.Const) error {
		var errs []error
		for i, name := range names {
			if c := consts[i]; c != nil {
				if _, ok := instant(*c); !ok {
					errs = append(errs, fmt.Errorf("argument %s: %q is not an RFC 3339 time", name, c.Text()))
				}
			}
		}
		return errors.Join(errs...)
	}
}

func addr(c tarski.Const) (netip.Addr, bool) {
	if c.Kind() != tarski.KindString {
		return netip.Addr{}, false
	}
	a, err := netip.ParseAddr(c.Text())
	if err != nil {
		return netip.Addr{}, false
	}
	return a.Unmap(), true
}

func network(c tarski.Const) (netip.Prefix, bool) {
	if c.Kind() != tarski.KindString {
		return netip.Prefix{}, false
	}
	p, err := netip.ParsePrefix(c.Text())
	if err != nil || p != p.Masked() {
		return netip.Prefix{}, false
	}
	return p, true
}

func instant(c tarski.Const) (time.Time, bool) {
	if c.Kind() != tarski.KindString {
		return time.Time{}, false
	}
	text := c.Text()
	t, err := time.Parse(time.RFC3339, text)
	return t, err == nil && offsetInRange(text)
}

// offsetInRange holds RFC 3339's offset to hours 00 to 23 and minutes 00 to 59. time.Parse
// accepts any two digits there ("+24:00", "+00:75"), which would move the instant by a day
// or more. The text has parsed, so it ends in Z or z, or in a sign and HH:MM.
func offsetInRange(text string) bool {
	if last := text[len(text)-1]; last == 'Z' || last == 'z' {
		return true
	}
	off := text[len(text)-5:]
	return off[:2] <= "23" && off[3:] <= "59"
}

func duration(c tarski.Const) (time.Duration, bool) {
	if c.Kind() != tarski.KindString {
		return 0, false
	}
	d, err := time.ParseDuration(c.Text())
	return d, err == nil && d >= 0
}
