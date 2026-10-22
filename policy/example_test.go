package policy_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/mattjoyce/shearline/datalog"
	"github.com/mattjoyce/shearline/policy"
)

// A policy is built once from standing facts and the bundled RBAC rules, then answers
// requests. Allowed carries the proof; denied carries why, here a deny/3 fact.
func Example() {
	p, err := policy.Load(policy.Config{
		Standing: []datalog.Fact{
			policy.Role("ana", "operator"),
			policy.Inherits("operator", "reader"),
			policy.Grant("reader", "read", "timeline/"),
			policy.Deny("ana", "read", "timeline/secret"),
		},
		Sets: []datalog.RuleSet{policy.RBAC()},
	})
	if err != nil {
		panic(err) // a bad policy never serves
	}

	d, _ := p.Decide(context.Background(), policy.Request{Principal: "ana", Action: "read", Resource: "timeline/bp"})
	fmt.Println(d.Allow)

	d, _ = p.Decide(context.Background(), policy.Request{Principal: "ana", Action: "read", Resource: "timeline/secret"})
	fmt.Println(d.Allow)
	fmt.Println(strings.SplitN(d.Why.String(), "\n", 2)[0])
	// Output:
	// true
	// false
	// allow("ana", "read", "timeline/secret") is not derived
}
