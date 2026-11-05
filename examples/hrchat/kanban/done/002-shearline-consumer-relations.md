# 002 Shearline: let a policy declare its own base relations

Owner: hrchat (consumer). Ask for: shearline-lead. Written in ASD-STE100.

## Job story

When my policy must read facts about my domain, I want to declare those relations in
`policy.Config`, so that I can use `policy.Load` and `policy.Decide` and not keep a copy
of them.

## User story

As the developer of an HR chatbot gate, I want Shearline to accept `reports_to/2`,
`dept/2` and `hr_partner/2` as standing facts, so that the rule set can derive a manager
path from them and each decision keeps Shearline's bounds, proof and hash.

## Case

### What happens now

1. `policy.Load` builds its fact base from `schema()` in `policy/facts.go`.
2. That schema names eight relations: principal, action, resource, context, role,
   inherits, grants and denied. `Config` cannot add a relation.
3. tarski refuses a fact of a relation the schema does not name. It also refuses a rule
   that names such a relation.
4. Result: a policy that needs a relation of its own cannot use `Load` or `Decide`.

### What hrchat did

hrchat compiles `policy.RBAC()` over a wider schema and copies `Load` and `Decide`
(`hrpolicy/policy.go`, about 120 lines). The copy works. The copy is a risk: a correction
to Shearline does not reach it.

### Proposed change

Add one field to `Config`. A nil map keeps the current behaviour.

```go
type Config struct {
    Standing  []tarski.Fact
    Sets      []tarski.RuleSet
    Context   map[string]tarski.Kind
    Relations map[string]Relation   // consumer base relations; nil declares none
}

// Relation is one consumer relation: one kind for each column.
type Relation struct{ Kinds []tarski.Kind }
```

`Load` must:

1. Add each declared relation to the schema, with its arity and a check for each column.
2. Refuse an empty string in a string column. This is the rule for every PARC column.
3. Refuse a declared name that is a PARC relation.
4. Refuse a declared name that a loaded rule set defines with a rule.
5. Refuse a standing fact of a relation that is declared nowhere. This is the current rule.

`Decide`, `Permitted`, the bounds, the answer bytes and the bundled rule-set hashes do not
change.

### Acceptance

- [x] A policy with `Relations` loads, and a rule set that names a declared relation compiles.
- [x] A proof shows a consumer fact as `fact`, the same as a PARC fact.
- [x] Each of the five refusals above has a test with the error it returns.
- [x] A nil `Relations` gives the same answers, proofs and hashes as v0.2.1. The golden tests hold.
- [x] `go test -race -shuffle=on ./...` and the go-quality gate pass.
- [x] hrchat deletes `hrpolicy/schema.go` and the `Load` and `Decide` copies, and all 39 scenarios still pass.

### Out of scope

- A check function for each relation. Kinds and the non-empty rule are sufficient for
  each consumer today.
- Declarations read from the rule set text (`reports_to/2.`). A typo there becomes an
  empty relation with no error.

## Done

Shearline v0.3.0 shipped `Config.Relations` (PR #8). hrchat requires v0.4.0, which adds
`Decision.Steps`, `Decision.Bytes` and `Policy.Bounds()`. `hrpolicy/schema.go` and the
copies of `Load` and `Decide` are deleted, and all 39 scenarios pass.
