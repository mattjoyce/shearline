# Hickey × Armstrong — structure × runtime — Before pass on docs/DESIGN.md

Artifact: `docs/DESIGN.md` at ae982d9, with `SPEC.md` as context. Date 2026-10-22.

## Hickey — structure at rest

- 🟠 **`Decision.RuleSet string`, IDs joined by `"; "`.** A receipt field that a consumer will one
  day split on a separator is data braided with presentation. The decision stands on a list of
  rule sets; carry the list. → `RuleSets []string`, each a `RuleSet.ID()`.
- 🟠 **`Proof` as a tree over a DAG.** A derivation is a DAG (one `holds` tuple feeds many
  `permit` tuples). Expanding it as a tree of values re-derives shared nodes, and for a long
  recursive chain the tree can be far larger than the record it came from. Values can share:
  build each `(relation, tuple)` proof once and reuse the same `Proof` value wherever it appears
  in a body. Linear in the records; still an immutable value.
- 🟡 **`Proof.Rule *Rule` points into the plan.** A pointer into another value's slice is an
  identity leaking out of a value. Acceptable because `Plan` is immutable, but say so: the rule is
  shared, not copied, and must not be written through.
- 🟡 **`Schema.Checks map[string]func(Atom) error`.** A function inside what was a plain value:
  `Schema` can no longer be compared or hashed. It is the spec's ask and the cost is small, but
  the braid is real. Keep it, and keep every other field of `Schema` plain data.
- ✅ `Policy` as a value (plan + frozen base + IDs), `With` producing a new base per request, and
  standing facts kept apart from request facts, are exactly the epochal shape the thesis wants.

## Armstrong — behaviour under fault

- 🟠 **Evaluation is bounded in bytes but not in work.** `MaxBytes` charges only *new* tuples. A
  rule that joins a wide cross product and derives nothing new (every head tuple a duplicate) runs
  unbounded on a `context.Background()`. `Decide` sits on a request path; its termination must not
  depend on the caller remembering a deadline. → `Options.MaxSteps` (0 = none), and a named
  `maxDecisionSteps` in `policy`, so a decision does bounded work whatever ctx carries.
- 🟠 **"Frozen" must be structural, not hoped.** The concurrency invariant rests on
  `relation.candidates` never mutating a base relation. Make a frozen relation refuse `add`
  (panic: a programming error, let it crash) and build the indexes at freeze time, so a future
  edit cannot quietly reintroduce the race. The cutter's `-race` test over concurrent `Evaluate`
  is the standing proof.
- 🟡 **`RBAC()` with no error return.** The embedded text is a build asset; a parse failure is a
  defect, not a runtime condition. Parse once at package init (`var rbac = mustRuleSet(...)`) so a
  broken asset fails at process start, never at the first request, and a test holds it parsing.
- 🟡 **`maxDecisionBytes` as an invariant.** Right call: a decision that derives a megabyte is a
  wrong policy, not a large one. Say the remedy in the `*TooLarge` message (narrow the grants
  reachable from this principal), since the consumer cannot raise the bound.
- ✅ Error kernel is `Compile` + `Evaluate`, both pure, both fail closed; every `policy` error is a
  deny; the library owns no state a crash could corrupt. No supervisor needed because nothing
  runs.

## Convergence

Both eyes land on the same two places: the proof structure (a DAG, shared, bounded) and the
runtime bound (work, not just bytes). Those are the must-dos before anyone codes `eval.go`.

## Tension

Hickey wants `Schema` to be plain data; the spec wants per-relation checks in it. Held, not
averaged: the checks stay in `Schema` because they are knowledge about a base relation, and the
price is that `Schema` is not a hashable value. Revisit if a consumer ever needs to hash a schema.
