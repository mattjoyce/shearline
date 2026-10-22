# Ousterhout × Liskov — surface × substitution — Before pass on docs/DESIGN.md

Artifact: `docs/DESIGN.md` at ae982d9, with `SPEC.md` as context. Date 2026-10-22.

## Liskov — the promise

- 🔴 **Proofs are not deterministic as designed.** "The first rule and body tuples that produced
  it" depends on the order tuples enter the derived relations. In Anam's evaluator `merge` ranges
  over `delta`, a Go map, so the insertion order of a round's tuples varies run to run, the index
  candidates vary, and so does which derivation is first. `Answer` survives this because it sorts;
  `Proof` does not. A proof that goes into a receipt must be a function of (plan, base, options).
  → `merge` and every other map walk on the derivation path iterate in sorted relation order, the
  contract states determinism outright, and a test evaluates twice and compares proofs deeply.
- 🟠 **`Schema.Checks` runs on atoms, not on facts.** The contract reads "every atom naming the
  relation"; a caller will assume it means every *tuple* of the relation. A fact loaded through
  `NewBase`/`With` is a tuple of constants and is exactly what a check wants to see. → Run each
  relation's check on its facts too (build `Atom{Name, Args: constants}`), so the schema's word
  holds everywhere a tuple enters.
- 🟠 **`With` has no stated postcondition.** "Shares untouched relations" describes the
  implementation; the promise a caller can rely on is `b.With(f...)` ≡ `NewBase(schema, standing…,
  f…)` for every observable (`Len`, every `Evaluate`, every `Answer` hash). State it, and test it
  as a property over random fact sets.
- 🟡 **`Load` trusts a consumer's rule set.** A later set may add rules to `allow/3` itself,
  widening the gate arbitrarily. That is extension by design, but say in the contract that rule
  sets are policy, never user input, and that deny-by-absence is the only promise that survives an
  arbitrary set.
- 🟡 **`Permitted` returns prefixes.** A caller reading the name expects resources. The doc says
  prefixes; make the sentence the first line of the doc comment, and consider whether the name
  should say it.

## Ousterhout — the surface

- 🟠 **`policy` is shallow for the consumer it is meant to hide Datalog from.** To load three
  roles a consumer writes `datalog.Fact{Relation: "grants", Tuple: datalog.Tuple{datalog.String(...)…}}`.
  The PARC layer exists so a consumer need not know the engine's vocabulary. → Four constructors,
  `policy.Role(p, r)`, `policy.Inherits(r, q)`, `policy.Grant(role, action, prefix)`, and
  `policy.Deny(p, a, r)`, each returning a `datalog.Fact`. Cheap, and the only way most callers
  should ever build a fact.
- 🟡 **`Decision.RuleSet` as one string** is the smaller surface, but it hides a list and invites
  parsing. A `[]string` is as small and honest.
- 🟡 **`Request.Context map[string]datalog.Const`** forces the consumer into `datalog.String` etc.
  Acceptable: typed constants are the point, and a `map[string]any` would push the kind error to
  request time. Keep.
- ✅ The engine's surface is deep: seven functions over the whole of parse, compile, evaluate,
  proof, hash. `Compile` and `Evaluate` taking values and returning values define a whole class of
  caller errors out of existence (nothing to close, nothing to reset).

## Convergence

Both eyes: the policy constructors (what the caller must know) and the proof contract (what the
caller may rely on) are the two gaps. Determinism is the one that bites silently.

## Tension

Ousterhout would keep `Schema.Checks` for the small surface; Liskov wants the contract to hold on
facts as well as atoms. Resolved by widening the promise, not the surface: the same check, run in
both places.
