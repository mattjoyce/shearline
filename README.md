# Shearline

A small, deterministic policy engine for Go. Facts in, a decision out, with the proof.

Shearline answers one question: **may this principal take this action on this resource, in this
context?** The policy is written as Datalog rules, the request as facts, and the answer is derived,
deterministic and explainable. An allowed request carries the proof that derived it. A refused
request carries why-not provenance: which rule got how far, and what it did not find.

It is a library, not a service. It owns no state, reads no file and no network, and knows no
vocabulary of its own: a resource name is an opaque string, and the consumer brings every fact.

Status: pre-1.0, API may change. No consumer has imported it yet; its first two will be
[Anam](https://github.com/mattjoyce/anam), whose Datalog evaluator it was cleaved from, and
[Ductile](https://github.com/mattjoyce/ductile).

## Install

```
go get github.com/mattjoyce/shearline
```

Go 1.25 or later. Standard library only.

## Example

```go
import (
    "context"
    "fmt"

    "github.com/mattjoyce/shearline/datalog"
    "github.com/mattjoyce/shearline/policy"
)

p, err := policy.Load(policy.Config{
    Standing: []datalog.Fact{
        policy.Role("ana", "operator"),
        policy.Inherits("operator", "reader"),
        policy.Grant("reader", "read", "timeline/"),
        policy.Deny("ana", "read", "timeline/secret"),
    },
    Sets: []datalog.RuleSet{policy.RBAC()},
})
if err != nil { /* a bad policy never serves */ }

d, err := p.Decide(context.Background(), policy.Request{
    Principal: "ana", Action: "read", Resource: "timeline/bp",
})
fmt.Println(d.Allow)        // true
fmt.Println(d.Proof)        // the derivation: allow <- permit <- holds <- role, inherits, grants
fmt.Println(d.RuleSets)     // ["rbac/1 sha256:…"]

d, _ = p.Decide(context.Background(), policy.Request{
    Principal: "ana", Action: "read", Resource: "timeline/secret",
})
fmt.Println(d.Allow)        // false
fmt.Println(d.Why)          // allow(...) is not derived
                            //   stopped at not deny(P, A, R) ... blocked by deny("ana", "read", "timeline/secret")
```

Every error from `Decide` is a deny. A malformed request wraps `policy.ErrRequest`; anything else is a
broken policy or engine. `Permitted(ctx, principal, action)` lists the grant prefixes a principal holds,
from the same rules that gate each request, so the listing and the gate cannot disagree.

## Two packages

**`datalog`** is the engine: a small Datalog with stratified negation, comparisons and three string
guards (`prefix`, `suffix`, `contains`). `Compile` refuses at load what would fail at request time
(arities, safety, stratification, malformed programs, schema checks). `Evaluate` runs semi-naive to a
fixpoint under a deadline, a byte bound and a step bound, over a `Base` that is immutable and safe
for concurrent use. Answers are sorted, canonically serialised and hashed. Proofs record the first
derivation of every tuple; `WhyNot` explains an absence. Rule sets are named, versioned and hashed,
and a set's text is the one authoritative form.

**`policy`** is the PARC layer (principal, action, resource, context) over it: `Config`, `Load`,
`Decide`, `Permitted`, fact constructors, and the bundled `rbac.dl` with roles, inheritance, prefix
grants and denials, every rule seeded by the request so a decision derives only the requester's
world. Context keys are declared with their kinds or refused.

## Design

- `docs/DESIGN.md` is the contract: the public API, the evaluation invariants, every deviation from
  the original spec, and the six design decisions made with their reasons.
- `SPEC.md` is the original draft spec, kept as history.
- `docs/reviews/` holds the three reviews the module went through before and after it was built: a
  design-stage Luminary panel, a whole-module panel, and an adversarial edge audit with fuzzing and
  oracles. Each finding is tied to the commit that resolved it.

Engineering standards: standard library only, `gofmt` and `goimports`, `golangci-lint`, `gosec`,
table-driven tests with real components, `go test -race -shuffle=on`, coverage above 80%, functions
under about 60 lines and files under about 400.

## Licence

Apache License 2.0. See `LICENSE`.
