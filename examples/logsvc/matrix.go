// Command logsvc is a worked example of embedding Shearline in a service.
//
// The service is a log store with eight endpoints. Callers present a token. Tokens hold
// roles. Roles may call endpoints. The question at every request is: may this token call
// this endpoint? Shearline answers it, and shows its working.
//
// Read the files in this order:
//
//	rbac.yaml      the permission matrix, as a product owner would write it
//	tokens.yaml    which token holds which roles
//	matrix.go      (this file) the YAML shapes, and the one function that turns them into facts
//	load.go        reading the files, failing closed, and swapping a new policy in at run time
//	main.go        a command that prints the matrix or decides one request
//	logsvc_test.go the matrix pinned as a table-driven test
//
// Run it from this directory:
//
//	go run .                         # the matrix: every token, every endpoint it may call
//	go run . tok-0b44 /stop          # one decision, with its proof
//	go run . tok-9c1e /stop          # one refusal, with the reason
//	go test ./...
//
// Shearline itself reads no file and knows no YAML. The YAML dependency belongs to this
// example, which is why the example is its own Go module: the engine stays standard
// library only.
//
// When you have run it, three extensions show whether the shape holds:
//
//   - A new role is a new entry in rbac.yaml. Nothing in Go changes.
//   - A denial for one token on one endpoint is a policy.Deny fact appended in Facts; the
//     bundled rules subtract it after the grants are summed, and a refusal names it.
//   - Per-method permissions make each grant a {method, endpoint} pair, with the method
//     passed as Grant's second argument in place of the single action. Request.Action then
//     carries the method of the call.
package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mattjoyce/shearline/policy"
	"github.com/mattjoyce/tarski"
)

// Matrix is the shape of rbac.yaml.
//
// Action is the single verb this service uses. Every endpoint is "called", so one action
// serves. A service that distinguishes GET from DELETE would put the method here instead,
// and carry it per grant.
//
// Roles maps a role name to what it inherits and what it grants. The map is unordered.
// That is deliberate: a policy is a set of facts, and Shearline sorts them at Load, so
// the order in the file cannot change a decision or a proof.
type Matrix struct {
	Action string          `yaml:"action"`
	Roles  map[string]Role `yaml:"roles"`
}

// Role is one entry under roles in rbac.yaml.
type Role struct {
	// Inherits lists the roles whose grants this role also holds. In Datalog terms it is
	// the fact inherits(child, parent): the child's permission set includes the parent's.
	// The relation is transitive. Admin inherits RW, RW inherits RO, so Admin holds RO
	// without saying so. It need not be a chain: Admin also inherits Operator, which RW
	// does not, so the roles form a lattice.
	Inherits []string `yaml:"inherits"`

	// Grants lists the endpoints this role may call directly. Each becomes the fact
	// grants(role, action, endpoint).
	//
	// Shearline matches the endpoint as a string prefix of the requested resource, so a
	// grant of /stop also covers /stopall and /stop/x. Two rules follow, and this example
	// enforces both. First, ask Shearline about the route your router matched, never the
	// raw request path; see decide in main.go. Second, keep no endpoint a prefix of another;
	// Facts refuses a matrix that breaks this.
	Grants []string `yaml:"grants"`
}

// Tokens is the shape of tokens.yaml: token to the roles it holds.
type Tokens struct {
	Tokens map[string][]string `yaml:"tokens"`
}

// Facts translates the two files into Shearline facts.
//
// This is where the service meets the engine. Everything else in the example is reading
// files and printing results. Three fact constructors from the policy package are used:
//
//	policy.Inherits(child, parent)        inherits/2   the child role includes the parent
//	policy.Grant(role, action, endpoint)  grants/3     the role may do action on endpoint
//	policy.Role(token, role)              role/2       the token holds the role
//
// Each returns a plain tarski.Fact: a relation name and a tuple of values. The
// constructors exist so that the relation name, the number of columns and the order of
// the columns cannot be mistyped.
//
// The bundled rule set, policy.RBAC(), derives the answer from these relations. Its source
// is policy/rbac.dl in the Shearline repository: nineteen lines, worth reading once. It
// also reads one relation this example never writes, denied/3, from
// policy.Deny(token, action, endpoint): one exact resource blocked whatever the roles say.
//
// Facts also checks what Shearline cannot. A token holding the role "Opertor" is, to the
// engine, a valid role with no grants: the token would be refused everything and nothing
// would say why. An endpoint that is a prefix of another would make one grant cover two
// routes. Both are mistakes in this service's vocabulary, so this service's code catches
// them.
func Facts(m Matrix, t Tokens) ([]tarski.Fact, error) {
	if m.Action == "" {
		return nil, fmt.Errorf("rbac: action is empty")
	}
	eps := Endpoints(m)
	for i, a := range eps {
		if a == "" {
			continue // an empty endpoint is Load's to refuse, with the engine's own message
		}
		for _, b := range eps[i+1:] {
			if strings.HasPrefix(b, a) {
				return nil, fmt.Errorf("rbac: endpoint %q is a prefix of %q; a grant of one would cover both", a, b)
			}
		}
	}
	var fs []tarski.Fact
	for name, r := range m.Roles {
		for _, parent := range r.Inherits {
			if _, ok := m.Roles[parent]; !ok {
				return nil, fmt.Errorf("rbac: role %q inherits unknown role %q", name, parent)
			}
			fs = append(fs, policy.Inherits(name, parent))
		}
		for _, ep := range r.Grants {
			fs = append(fs, policy.Grant(name, m.Action, ep))
		}
	}
	for tok, roles := range t.Tokens {
		for _, role := range roles {
			if _, ok := m.Roles[role]; !ok {
				return nil, fmt.Errorf("tokens: token %q holds unknown role %q", tok, role)
			}
			fs = append(fs, policy.Role(tok, role))
		}
	}
	return fs, nil
}

// Endpoints is every endpoint the matrix names, sorted and without duplicates. This is
// the service's route table: a request path that is not in it is unknown, and an unknown
// path is answered before Shearline is asked, not by it.
func Endpoints(m Matrix) []string {
	seen := map[string]bool{}
	for _, r := range m.Roles {
		for _, ep := range r.Grants {
			seen[ep] = true
		}
	}
	eps := make([]string, 0, len(seen))
	for ep := range seen {
		eps = append(eps, ep)
	}
	sort.Strings(eps)
	return eps
}
