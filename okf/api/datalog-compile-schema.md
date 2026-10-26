---
type: Go API
title: tarski.Compile and Schema
description: "Compile validates programs against a Schema and groups rules into strata; Schema names base relations and optional per-relation checks."
resource: github.com/mattjoyce/tarski.Compile
tags: [go, api, compile, schema]
timestamp: 2026-10-22T00:00:00Z
---

> This describes the engine, which now lives in [tarski](https://github.com/mattjoyce/tarski) (required at v0.3.2); Shearline cut it out at c4f75ab and the code is no longer in this module. The contract is tarski's `docs/DESIGN.md`. Kept here because the policy layer relies on it.

# Signatures

```go
type Schema struct {
    Arities map[string]int
    Checks  map[string]func(Atom) error // nil map or nil entry: no check
}
func Compile(schema Schema, programs ...Program) (Plan, error)
func (p Plan) Arity(name string) (int, bool)
```

# Schema

- `Arities` names every base relation a base may hold.
- `Checks` runs on every atom that names the relation at `Compile` (a head cannot name a base relation, so in practice body atoms), and on every fact of the relation at `NewBase` and `With`, read as an atom of constants. The schema's word therefore holds everywhere a tuple enters.
- A check sees one atom, so it can hold a constant to a shape; reachability is not its business. Its error is wrapped, so `errors.Is` on the caller's sentinel works.
- A schema that carries checks is not comparable and not hashable. This was a held tension: Hickey objected, the spec and Ousterhout wanted the small surface, and the checks stayed in `Schema` with their promise widened to facts. Keep every other field plain data.
- A schema that names a built-in, a non-positive arity, or a check for a relation it lacks is `ErrInvalid`.

# Compile

- Programs are read as one: a later program defines what an earlier one declared.
- Programs are deep-copied before use and a malformed one (an empty `Literal`, an unknown comparison operator) is `ErrInvalid`, never a panic. See [rule-set identity](/concepts/rule-set-identity.md).
- Validates arities, safety and stratification; see [stratified negation](/concepts/stratified-negation.md).
- A relation declared and defined nowhere is empty, not an error.
- Each rule gets `Set` from its program's `Name`, so a [proof](/concepts/proof.md) can name its set.

# Plan.Arity

Reports a relation's arity, base or derived, and whether it exists. A consumer uses it to check that a rule set defines the relations it will read, at the arity it will read them; `policy.Load` uses it for `allow/3` and `permit/3`.

# Plan and base compatibility is by shape only

`Evaluate` checks names and arities. A base built under a schema with different `Checks` passes, because its facts were checked by its own schema. A caller who wants one set of checks everywhere compiles and builds from one `Schema` value, as `policy.Load` does. This was edge audit finding 6, resolved as a contract correction.

# Related

[Evaluate and Options](/api/datalog-evaluate-options.md), [Base.With](/api/datalog-base-with.md), [policy.Schema](/api/policy-fact-constructors.md).

# Citations

[1] [tarski/plan.go](https://github.com/mattjoyce/tarski/blob/v0.3.2/plan.go)
[2] [DESIGN.md, Schema and Compile](../../docs/DESIGN.md)
[3] [Edge audit, finding 6](../../docs/reviews/edge-audit-2026-10-22.md)
