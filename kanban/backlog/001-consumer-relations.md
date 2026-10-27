# 001 Let a policy declare its own base relations

Asked for by: hrchat (`hrchat/kanban/backlog/002-shearline-consumer-relations.md`).
Owner: shearline-lead. Built by: opencode, on a branch. Review: shearline-lead, before merge.
Release: v0.3.0 (a new field, additive). Written in ASD-STE100.

## Job story

When my policy must read facts about my domain, I want to declare those relations in
`policy.Config`, so that I can use `policy.Load` and `policy.Decide` and not keep a copy
of them.

## What happens now

1. `Load` compiles over `schema(declared)` in `policy/facts.go`.
2. That schema names eight base relations. A request supplies `principal/1`, `action/1`,
   `resource/1` and `context/2`. Standing facts supply `role/2`, `inherits/2`, `grants/3`
   and `denied/3`. `Config` cannot add a relation.
3. tarski refuses a fact or a rule that names a relation the schema does not name.
4. Result: hrchat compiles `RBAC()` over a wider schema and copies `Load` and `Decide`
   (about 120 lines). A correction to Shearline does not reach the copy.

## Change

Add one field to `Config`. A nil map declares no relation and keeps the v0.2.2 behaviour.

```go
type Config struct {
    Standing  []tarski.Fact
    Sets      []tarski.RuleSet
    Context   map[string]tarski.Kind
    Relations map[string]Relation // consumer base relations; nil declares none
}

// Relation is one consumer base relation: one kind for each column, in order.
type Relation struct{ Kinds []tarski.Kind }
```

`Load` must do these steps. Each refusal wraps `tarski.ErrInvalid` and starts `policy: load:`,
as the current errors do.

1. Refuse a declared name that is one of the eight PARC relations above. All eight, not
   only the four request relations: `schema()` is a map, so a declared `role` would
   silently replace Shearline's arity and its non-empty check.
2. Refuse a relation with no columns (`len(Kinds) == 0`), an empty name, and a kind that
   is not `KindString`, `KindNumber` or `KindBool`.
3. Add each declared relation to the schema: arity `len(Kinds)`, and one check that,
   for each constant argument, requires the declared kind and refuses an empty string in
   a string column. A variable passes. This check sees rule atoms and standing facts
   (tarski `NewBase` and `Compile` both run `Schema.Checks`).
4. Copy the map into the Policy, as `checkContext` does for `Context`, so a caller that
   changes its map after `Load` changes nothing.
5. Do not add code for these; tarski refuses them already. Each still needs a test:
   - a declared name that a loaded rule set derives with a rule (`plan.go`: "is a base
     relation; a rule cannot add to it");
   - a declared name that is a built-in, for example `prefix` (`plan.go`: "is a built-in
     and cannot be a base relation");
   - a standing fact of a relation that is declared nowhere (current behaviour).

Consumer relations are standing only. `Request` does not change, and `requestRelations`
does not change.

`Decide`, `Permitted`, the bounds, the answer bytes, the proof bytes and the bundled
rule-set hash (`rbac/1 sha256:0bf5a121...`) do not change.

## Acceptance

- [ ] A policy with `Relations` loads. A rule set that reads a declared relation compiles.
- [ ] A recursive rule over a declared relation works, for example a manager path
      `manages(M, E) :- reports_to(E, M).` and `manages(M, E) :- reports_to(E, X), manages(M, X).`
- [ ] A proof shows a consumer fact as `fact`, the same as a PARC fact.
- [ ] Each refusal in steps 1, 2, 3 and 5 has a test that checks `errors.Is(err, tarski.ErrInvalid)`
      and the message. Step 3 has a wrong-kind test and an empty-string test, for a fact
      and for a rule constant.
- [ ] A test changes the caller's `Relations` map after `Load` and shows no effect.
- [ ] Bounds: a test with a 2,000-person reporting chain at depth 12 decides inside
      `maxDecisionSteps`. Put the measured `Result.Cost` steps in the test comment. If it
      does not fit, stop and report to shearline-lead; do not change the bounds.
- [ ] A nil `Relations` gives the same answers, proofs and hashes as v0.2.2. Every existing
      test passes without an edit. Add a test that pins `RBAC().ID()` to
      `rbac/1 sha256:0bf5a1217fbaad382dd0f5044fa0e27c0fd978dec0c616576a066c862c5dbd36`.
- [ ] `go test -race -shuffle=on ./...` passes in the root module and in `examples/logsvc`.
- [ ] `go vet ./...` and `gosec ./...` pass (the CI gate).
- [ ] Docs: the `Config` doc comment, `docs/DESIGN.md`, `okf/packages/policy.md`, a new
      okf concept or API entry for `Relations`, `okf/log.md`, and an Unreleased entry in
      `CHANGELOG.md`. The okf validator reports CONFORMANT.
- [ ] Do not tag. Do not merge. Open a PR and leave it for review.

## After merge (hrchat, not this branch)

hrchat deletes `hrpolicy/schema.go` and its `Load` and `Decide` copies, and all 39 scenarios
still pass.

## Out of scope

- A check function for each relation. Kinds and the non-empty rule are sufficient today.
- Declarations read from the rule set text (`reports_to/2.`). A typo there becomes an
  empty relation with no error.
- Request-supplied consumer facts.
- Demand evaluation (`Plan.Demand`). It is the natural next step for recursive consumer
  relations, because it derives only the chain above the requesting principal. Separate card.

## Review 1 (shearline-lead, 2026-10-27): changes requested

The gates pass: race tests in both modules, vet, gosec, gofmt, okf CONFORMANT, 93.4% cover.
The design of `checkRelations` and `consumerRelationCheck` is right. Fix these, then
commit, push and open the PR.

### Must fix

1. `TestConsumerRelations_BoundsDepth12` proves nothing. Its second `manages` rule has the
   arguments swapped, `allow` holds from one base edge, the data is 2,000 disjoint chains
   (26,000 people, not 2,000), and the "measured" comment is not a measurement. Rewrite it:
   a 2,000-person tree (manager of `p<i>` is `p<(i-1)/2>`), the card's two `manages` rules,
   `allow(P, A, R) :- principal(P), action(A), resource(R), about(R, E), manages(P, E).`
   with `about/2` a second declared relation, request `p0` reading `review/p1999`. Call
   `tarski.Evaluate` with the decision bounds, read `res.Cost()`, and assert both numbers
   are under half their bound. My measurement: steps 41,980 (1.0% of `maxDecisionSteps`),
   bytes 259,225 (25% of `maxDecisionBytes`). Put the measured numbers in the comment.
   Bytes, not steps, is the bound that binds: a 2,000-person single chain exceeds
   `maxDecisionBytes` at `manages`. Say so in the concept doc (see 6).
2. `okf/log.md` has two `## Unreleased` blocks. The first lost its backticks to the shell.
   Delete it. Match the existing entries: `* **Change**: ...` under the dated list, no new heading.
3. `CHANGELOG.md`: the Unreleased section is at the bottom, after v0.1.0. Move it to the
   top, above v0.2.2, in the style of the other entries.
4. The `Config` doc comment in `policy/policy.go` does not mention `Relations`, and the `Load`
   comment still says it compiles "over Schema()". Update both. The card asked for this.
5. Tests the card asked for and that are missing:
   - a wrong-kind standing fact (only a rule constant is tested);
   - an empty string as a rule constant (only a standing fact is tested);
   - the error message, not only `errors.Is` and the prefix, for every refusal in
     `checkRelations` and `consumerRelationCheck`.
6. `okf/concepts/consumer-relations.md`: add the bounds note from 1 (a recursive relation is
   computed in full for each decision, and derived bytes bind before steps), and a
   Citations section like the other concepts (`policy/policy.go`, `policy/facts.go`).

### Clean up

7. `Policy.relations` and `copyRelations` are dead: nothing reads the field, and
   `checkRelations` already returns a private copy. Delete both.
8. `checkRelations` tests `requestRelations[name]` after `baseRelations[name]`. The request
   relations are a subset; delete the second test.
9. The new `if in == nil` in `checkContext` changes nothing (a nil map ranges as empty).
   Revert it. The same for `if declaredRelations != nil` in `schemaWithRelations`.
10. Typo `aritities`. Give `schemaWithRelations` and `consumerRelationCheck` doc comments,
    as the other functions in `facts.go` have. Then `schema()` can call
    `schemaWithRelations(declared, nil)` only if something still uses `schema()`; if not, fold them.
11. `TestConsumerRelations_Basic` loads twice, discards `p`, and never decides. Make it one
    Load and one Decide, or delete it (Recursive and ProofShowsFact cover it).
12. `TestConsumerRelations_MapCopied` also changes a `Kinds` slice element after Load, not
    only the map entry, because slice aliasing is the real risk.

## Review 2 (shearline-lead, 2026-10-27): changes requested again

Commit 59925c3. The gates pass. Of the 12 items in Review 1:

- Done: 3.
- Part done: 2 (the duplicate block is gone, but a new `## Unreleased` heading remains; use a
  `* **Change**:` line in the list), 4 (the `Load` comment still says "over Schema()"),
  5 (message tests were added for `checkRelations` only; a wrong-kind standing fact, an
  empty-string rule constant and the `consumerRelationCheck` messages are still missing).
- Not done: 1, 6, 7, 8, 9, 10, 11, 12. Item 1 is the main one: `TestConsumerRelations_BoundsDepth12`
  is unchanged and still measures nothing.

The branch is not pushed and has no PR. Work each item in Review 1 by its number, then push
and open the PR.
