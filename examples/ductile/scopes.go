// ductile models Ductile's API token scopes in Shearline, which SPEC.md step 7 asked for:
// load the tokens and the route table as standing facts, ask each request as a PARC
// Request, and see whether any assumption shaped by Anam leaks. None did; what the
// exercise found instead is written in README.md.
//
// Ductile (github.com/mattjoyce/ductile, internal/auth) authorises with a flat scope
// set per token and a hand-written implication table: plugin:rw implies plugin:ro,
// which implies plugin:catalog:ro, and so on. Each route lists the scopes that reach
// it. Here the scopes are roles, the implications are Inherits facts, and each route
// is a Grant of an action on a capability to the narrowest scope that has it; the
// rule set does the rest. The oracle test holds this model to Ductile's own semantics
// on every scope, every pair of scopes, and every route.
//
// Reading order: this file (the lattice, the route table, Facts), then main.go, then
// scopes_test.go.
package main

import (
	"fmt"
	"sort"

	"github.com/mattjoyce/shearline/policy"
	"github.com/mattjoyce/tarski"
)

// Implies is Ductile's scope implication table, transcribed from normalizeScopes in
// internal/auth/auth.go on 2026-10-23: a scope and one scope it implies. The wildcard
// is a scope like any other here, one that implies the four read-write scopes; in
// Ductile it short-circuits every check, which comes to the same thing over this
// route table.
var Implies = [][2]string{
	{"*", "plugin:rw"}, {"*", "jobs:rw"}, {"*", "events:rw"}, {"*", "system:rw"},
	{"plugin:rw", "plugin:ro"}, {"plugin:rw", "plugin:invoke:ro"},
	{"plugin:ro", "plugin:catalog:ro"},
	{"jobs:rw", "jobs:ro"},
	{"jobs:ro", "jobs:status:ro"}, {"jobs:ro", "jobs:result:ro"}, {"jobs:ro", "jobs:logs:ro"}, {"jobs:ro", "jobs:tree:ro"},
	{"events:rw", "events:ro"},
	{"events:ro", "events:meta:ro"}, {"events:ro", "events:payload:ro"},
	{"system:rw", "system:ro"},
}

// Route is one row of Ductile's route table (internal/api/server.go, the same day):
// the method and path chi matches, the action and capability the router asks the
// policy about, and the scopes Ductile's requireScopes middleware accepts, kept so
// the oracle test can compare. A capability names what the route does, not where it
// lives: five GET routes read job status, so five rows share jobs/status, and the
// read-class and write-class invocations of a plugin command are two actions on one
// capability, which the handler tells apart by the command's manifest.
type Route struct {
	Method, Path       string
	Action, Capability string
	Scopes             []string // Ductile's own list, "*" included
}

// Routes is the table. The resource names end in no separator because none is a
// prefix of another; a service with jobs/status and jobs/status/history would end
// its grants with "/".
var Routes = []Route{
	{"GET", "/plugin/{plugin}", "read", "plugin/catalog", []string{"plugin:catalog:ro", "plugin:rw", "*"}},
	{"POST", "/plugin/{plugin}/{command}", "read", "plugin/invoke", []string{"plugin:invoke:ro", "plugin:rw", "*"}},
	{"POST", "/plugin/{plugin}/{command}", "write", "plugin/invoke", []string{"plugin:rw", "*"}},
	{"POST", "/pipeline/{pipeline}", "write", "pipeline/trigger", []string{"plugin:rw", "*"}},
	{"GET", "/topology", "read", "system/topology", []string{"system:ro", "system:rw", "*"}},
	{"GET", "/config/view", "read", "system/config", []string{"system:ro", "system:rw", "*"}},
	{"GET", "/system/doctor", "read", "system/doctor", []string{"system:ro", "system:rw", "*"}},
	{"GET", "/system/selfcheck", "read", "system/selfcheck", []string{"system:ro", "system:rw", "*"}},
	{"POST", "/system/reload", "write", "system/reload", []string{"system:rw", "*"}},
	{"GET", "/job/{jobID}", "read", "jobs/status", []string{"jobs:status:ro", "jobs:rw", "*"}},
	{"GET", "/jobs", "read", "jobs/status", []string{"jobs:status:ro", "jobs:rw", "*"}},
	{"GET", "/scheduler/jobs", "read", "jobs/status", []string{"jobs:status:ro", "jobs:rw", "*"}},
	{"GET", "/stopwatch/{plugin}", "read", "jobs/status", []string{"jobs:status:ro", "jobs:rw", "*"}},
	{"GET", "/analytics/summary", "read", "jobs/status", []string{"jobs:status:ro", "*"}},
	{"GET", "/analytics/queue", "read", "jobs/status", []string{"jobs:status:ro", "*"}},
	{"GET", "/job/{jobID}/tree", "read", "jobs/tree", []string{"jobs:tree:ro", "jobs:rw", "*"}},
	{"GET", "/job-logs", "read", "jobs/logs", []string{"jobs:logs:ro", "jobs:rw", "*"}},
	// Not a route: the result, log and tree fields of a job response, which Ductile
	// shapes per token (its D1 rule). A field class is a capability like any other,
	// decided once per response, which is what policy.Permitted is not for.
	{"", "job response: result fields", "read", "jobs/result", []string{"jobs:result:ro", "jobs:rw", "*"}},
	{"GET", "/events", "read", "events/stream", []string{"events:ro", "events:rw", "*"}},
}

// Grants is the route table as grants: for each row, the narrowest scope in its list
// is granted the action on the capability. The wider scopes reach it through Implies.
// "The narrowest" is the first one Ductile lists, by its own convention.
func Grants() []tarski.Fact {
	seen := map[[3]string]bool{}
	var out []tarski.Fact
	for _, r := range Routes {
		k := [3]string{r.Scopes[0], r.Action, r.Capability}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, policy.Grant(r.Scopes[0], r.Action, r.Capability))
	}
	return out
}

// Facts is the standing facts for a set of tokens: the lattice, the grants, and a
// Role fact per token per scope. A token with a scope the lattice and the route table
// never name is refused, since in Ductile it would be a scope that grants nothing and
// nobody would notice.
func Facts(tokens map[string][]string) ([]tarski.Fact, error) {
	known := map[string]bool{}
	for _, im := range Implies {
		known[im[0]], known[im[1]] = true, true
	}
	for _, r := range Routes {
		for _, s := range r.Scopes {
			known[s] = true
		}
	}
	var out []tarski.Fact
	for _, im := range Implies {
		out = append(out, policy.Inherits(im[0], im[1]))
	}
	out = append(out, Grants()...)
	names := make([]string, 0, len(tokens))
	for tok := range tokens {
		names = append(names, tok)
	}
	sort.Strings(names)
	for _, tok := range names {
		for _, s := range tokens[tok] {
			if !known[s] {
				return nil, fmt.Errorf("token %s has scope %q, which no route and no implication names", tok, s)
			}
			out = append(out, policy.Role(tok, s))
		}
	}
	return out, nil
}

// Load is a Policy over the tokens. Ductile reloads its tokens file at run time; this
// is the function it would call, and the Policy it returns is a value to swap in.
func Load(tokens map[string][]string) (policy.Policy, error) {
	fs, err := Facts(tokens)
	if err != nil {
		return policy.Policy{}, err
	}
	return policy.Load(policy.Config{Standing: fs, Sets: []tarski.RuleSet{policy.RBAC()}})
}

// Tokens is the three tokens Ductile's CONFIG_REFERENCE.md shows, plus two narrow
// ones an operator who wants least privilege would mint.
var Tokens = map[string][]string{
	"admin":    {"*"},
	"readonly": {"plugin:ro", "jobs:ro", "events:ro"},
	"operator": {"plugin:rw", "jobs:rw", "events:ro"},
	"status":   {"jobs:status:ro"},
	"invoker":  {"plugin:invoke:ro"},
}
