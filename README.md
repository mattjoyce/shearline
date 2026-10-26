# Shearline

[![ci](https://img.shields.io/github/actions/workflow/status/mattjoyce/shearline/ci.yml?branch=master&label=ci)](https://github.com/mattjoyce/shearline/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/mattjoyce/shearline.svg)](https://pkg.go.dev/github.com/mattjoyce/shearline)
![Go version](https://img.shields.io/github/go-mod/go-version/mattjoyce/shearline)
[![Licence](https://img.shields.io/github/license/mattjoyce/shearline)](LICENSE)

A small, deterministic policy engine for Go. Facts in, a decision out, with the proof.

## What it is

Shearline answers one question: **may this principal take this action on this resource, in this
context?** You write the policy as rules, hand the engine the facts, and it derives the answer. An
allowed request comes back with the proof that derived it. A refused request comes back with why:
which rule got how far, and what it did not find.

Two words a stranger needs. *Datalog* is a tiny rule language: facts like `role("ana", "reader").`
and rules like `holds(P, R) :- role(P, R).` that derive new facts from old ones until nothing new
appears; it always terminates. *PARC* is the vocabulary the bundled rules use: principal, action,
resource, context.

Shearline is a library, not a service. It owns no state, reads no file and no network, and knows
no vocabulary of its own: a resource name is an opaque string, and you bring every fact.

Status: pre-1.0, the API may change. The rule engine is not part of this module: it is
[tarski](https://github.com/mattjoyce/tarski), a Datalog engine of its own, cut out of Shearline
with its history and required here at v0.2.2. Shearline is the PARC layer over it. No consumer has
imported Shearline yet; the first two will be [Anam](https://github.com/mattjoyce/anam), whose
Datalog evaluator became tarski, and [Ductile](https://github.com/mattjoyce/ductile), whose token
scopes `examples/ductile` already models, with a parity test against Ductile's own semantics.
Because tarski is one module, a consumer that imports both resolves to a single engine version,
and the `Fact` and `RuleSet` types it builds are the ones Shearline accepts. A policy rule set
cannot use aggregates (`count`, `sum`, `min`, `max`); `Load` refuses it, since a decision is per
request and aggregates are for queries. Moving to tarski changed two things a v0.1 user can see:
engine errors start `tarski:` instead of `datalog:`, and numbers at or above 1e6 or below 1e-4 print in
full, which changes the hash of an answer that carries one.

## Try it in a minute

Go 1.25 or later. One dependency, our own engine: [tarski](https://github.com/mattjoyce/tarski).

```
git clone https://github.com/mattjoyce/shearline && cd shearline
go test ./...
go test ./policy -run Example -v
```

The last command runs a compiled twin of the program below (`policy/example_test.go`) and checks
its output, so this page cannot drift from the API. To use it in your own module:

```
go get github.com/mattjoyce/shearline
```

```go
package main

import (
    "context"
    "fmt"

    "github.com/mattjoyce/shearline/policy"
    "github.com/mattjoyce/tarski"
)

func main() {
    p, err := policy.Load(policy.Config{
        Standing: []tarski.Fact{
            policy.Role("ana", "operator"),
            policy.Inherits("operator", "reader"),
            policy.Grant("reader", "read", "timeline/"),
            policy.Deny("ana", "read", "timeline/secret"),
        },
        Sets: []tarski.RuleSet{policy.RBAC()},
    })
    if err != nil {
        panic(err) // a bad policy never serves
    }
    ctx := context.Background()

    d, _ := p.Decide(ctx, policy.Request{Principal: "ana", Action: "read", Resource: "timeline/bp"})
    fmt.Println(d.Allow, d.RuleSets)
    fmt.Println(d.Proof)

    d, _ = p.Decide(ctx, policy.Request{Principal: "ana", Action: "read", Resource: "timeline/secret"})
    fmt.Println(d.Allow)
    fmt.Println(d.Why)
}
```

It prints the following (hash abbreviated; it changes whenever `rbac.dl` does):

```
true [rbac/1 sha256:0bf5a121…]
allow("ana", "read", "timeline/bp")  rbac/1:19  allow(P, A, R) :- permit(P, A, Pat), resource(R), prefix(Pat, R), not deny(P, A, R).
  permit("ana", "read", "timeline/")  rbac/1:16  permit(P, A, Pat) :- principal(P), action(A), holds(P, Role), grants(Role, A, Pat).
    principal("ana")  fact
    action("read")  fact
    holds("ana", "reader")  rbac/1:13  holds(P, R) :- holds(P, Q), inherits(Q, R).
      holds("ana", "operator")  rbac/1:12  holds(P, R) :- principal(P), role(P, R).
        principal("ana")  (shown above)
        role("ana", "operator")  fact
      inherits("operator", "reader")  fact
    grants("reader", "read", "timeline/")  fact
  resource("timeline/bp")  fact

false
allow("ana", "read", "timeline/secret") is not derived
  rbac/1:19  allow(P, A, R) :- permit(P, A, Pat), resource(R), prefix(Pat, R), not deny(P, A, R).
    stopped at not deny(P, A, R)  with A="read", P="ana", Pat="timeline/", R="timeline/secret"
    blocked by
      deny("ana", "read", "timeline/secret")  rbac/1:8  deny(P, A, R) :- principal(P), action(A), resource(R), denied(P, A, R).
        principal("ana")  fact
        action("read")  fact
        resource("timeline/secret")  fact
        denied("ana", "read", "timeline/secret")  fact
```

## Using it

- **Build the policy once, decide many times.** `Load` compiles the rules and holds the standing
  facts; a `Policy` is a value, safe to share between goroutines.
- **Every error is a deny.** `Decide` never returns `Allow=true` with an error. A malformed request
  (empty field, undeclared context key, wrong kind) wraps `policy.ErrRequest`; any other error is a
  broken policy or engine.
- **Context keys are declared or refused.** List every key a request may carry in
  `Config.Context` with its kind; a request carrying anything else is refused. The rules see them
  as `context(Key, Value)` facts.
- **Grants are string prefixes.** `Grant("reader", "read", "timeline/")` covers every resource
  starting with `timeline/`. There are no path semantics: end a prefix with your separator, and
  make resource names canonical before you call. `Deny` is exact; a subtree denial is a rule of
  your own using `prefix/2`.
- **Extend with your own rules.** Add a rule set after `RBAC()` in `Config.Sets` to define
  denials, add roles, or read context. Rule sets are policy, never user input.
- **List what a principal holds.** `Permitted(ctx, principal, action)` returns the grant prefixes
  from the same rules that gate requests. It does not apply `deny`, which is per concrete resource,
  and it carries no context, so it is discovery, not a gate: `Decide` every resource you then act on.
  To list who may do something, loop over your principals and call `Decide`.
- **Render a receipt.** `Proof` and `Why` print as indented text and marshal to compact JSON.
- **See it embedded.** `examples/logsvc` is a worked example: a log service whose roles and
  tokens live in two YAML files, with the one function that turns them into facts, a policy
  swap for run-time changes, and the matrix pinned as a test. Its comments are the tutorial.

## Reference

Every exported name is documented in the source and on
[pkg.go.dev](https://pkg.go.dev/github.com/mattjoyce/shearline), or locally:

```
go doc github.com/mattjoyce/shearline/policy
go doc github.com/mattjoyce/tarski
```

[`tarski`](https://github.com/mattjoyce/tarski) is the engine: parse, compile, evaluate, proofs,
why-not, hashed rule sets; its contract is its own `docs/DESIGN.md`. `policy` is the PARC layer
over it with the bundled `rbac.dl`.

## Why it is built this way

The theory, in four sentences. A policy is a value and a decision is a pure function of (policy,
request), so the same inputs always give the same answer, the same proof and the same hash.
Everything fails closed: a bad rule set is refused at `Load`, a bad request at `Decide`, and a
bound crossed during evaluation is an error, never a partial answer. The engine knows no
vocabulary, so the consumer owns what names mean and Shearline owns only that the derivation is
right. Every refusal explains itself from the decision, not from a log, because a policy you cannot
interrogate is one you will stop trusting.

The long form is in `docs/DESIGN.md`: the public API, the evaluation invariants, every deviation
from the original spec, and the six design decisions with their reasons. `SPEC.md` is the original
draft, kept as history. `docs/reviews/` holds the three reviews the module went through, a
design-stage panel, a whole-module panel and an adversarial edge audit with fuzzing and oracles,
each finding tied to the commit that resolved it.

Engineering standards: one dependency (tarski), nothing else, `gofmt` and `goimports`, `golangci-lint`, `gosec`,
table-driven tests with real components, `go test -race -shuffle=on`, coverage above 80%, functions
under about 60 lines and files under about 400. CI runs all of it on every push.

## Changes

The current release is v0.2.0. For a v0.1 user the visible changes are those named under Status
above: the engine is now tarski, its errors start `tarski:`, and large and small numbers print in
full. Bundled rule-set hashes are unchanged. [CHANGELOG.md](CHANGELOG.md) lists each release.

## Licence

Apache License 2.0. See `LICENSE`.
