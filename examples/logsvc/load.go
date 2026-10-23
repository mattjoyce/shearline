package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"sync/atomic"

	"github.com/mattjoyce/shearline/datalog"
	"github.com/mattjoyce/shearline/policy"
	"gopkg.in/yaml.v3"
)

// Snapshot is everything the service knows at one instant: the Policy, and the two lists
// the Policy does not offer because they are the service's idea, not the engine's. A
// Policy answers questions about a principal and a resource; it does not enumerate
// either. The service knows its own route table and its own tokens, so it keeps them
// beside the Policy they were loaded with.
type Snapshot struct {
	Policy    policy.Policy
	Endpoints []string // the route table, sorted
	Tokens    []string // every token in tokens.yaml, sorted
}

// LoadSnapshot reads both files and returns a ready Snapshot, or the first error.
//
// Nothing is served until every layer accepts the input. There are three:
//
//  1. The YAML decoder refuses a key it does not know, so `grant:` for `grants:` is an
//     error at the line it occurs, not a role that silently grants nothing.
//  2. Facts refuses a name that one file uses and the other does not define, and an
//     endpoint that is a prefix of another.
//  3. policy.Load refuses an empty string in any fact, a fact for a relation the schema
//     does not declare, a fact with the wrong number of columns, and a rule set whose
//     hash does not match its text.
//
// The Policy is a value, not a pointer. It does not change after Load, and it is safe to
// share between goroutines without a lock.
func LoadSnapshot(matrixPath, tokensPath string) (Snapshot, error) {
	var m Matrix
	var t Tokens
	if err := readYAML(matrixPath, &m); err != nil {
		return Snapshot{}, err
	}
	if err := readYAML(tokensPath, &t); err != nil {
		return Snapshot{}, err
	}
	fs, err := Facts(m, t)
	if err != nil {
		return Snapshot{}, err
	}
	p, err := policy.Load(policy.Config{
		Standing: fs,
		Sets:     []datalog.RuleSet{policy.RBAC()},
	})
	if err != nil {
		return Snapshot{}, err
	}
	toks := make([]string, 0, len(t.Tokens))
	for tok := range t.Tokens {
		toks = append(toks, tok)
	}
	sort.Strings(toks)
	return Snapshot{Policy: p, Endpoints: Endpoints(m), Tokens: toks}, nil
}

// readYAML decodes one file into the given shape, strictly.
func readYAML(path string, into any) error {
	b, err := os.ReadFile(path) // #nosec G304 -- the path names this program's own configuration
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(into); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// Guard holds the current Snapshot and lets it be replaced while requests are in flight.
//
// A Policy is immutable, so the service never edits the one it has. When /mint succeeds
// and tokens.yaml gains a line, the service builds a whole new Snapshot and swaps it in.
// The swap is one atomic pointer write. A request that began under the old Snapshot
// finishes under the old Snapshot, and the next request sees the new one. No lock, no
// half-updated state, and if the new files are bad the old Snapshot stays in force.
//
// Guard is off the guided path: the command loads once and never calls Reload. A real
// service would, after /mint rewrites tokens.yaml. TestGuardKeepsOldPolicyOnBadReload is
// where Reload runs in this example.
type Guard struct {
	cur atomic.Pointer[Snapshot]
}

// NewGuard loads the files once and fails if they do not load. A service with no policy
// has nothing safe to serve.
func NewGuard(matrixPath, tokensPath string) (*Guard, error) {
	g := &Guard{}
	if err := g.Reload(matrixPath, tokensPath); err != nil {
		return nil, err
	}
	return g, nil
}

// Reload reads the files again and, only if they load, replaces the current Snapshot.
func (g *Guard) Reload(matrixPath, tokensPath string) error {
	s, err := LoadSnapshot(matrixPath, tokensPath)
	if err != nil {
		return err
	}
	g.cur.Store(&s)
	return nil
}

// Current returns the Snapshot in force at this instant.
func (g *Guard) Current() Snapshot { return *g.cur.Load() }
