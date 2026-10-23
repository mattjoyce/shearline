---
type: Concept
title: Proof
description: "A proof is the first derivation of a tuple, shared as a DAG of values, deterministic, and naming its rule by set, line and text."
tags: [proof, determinism, receipt]
timestamp: 2026-10-22T00:00:00Z
---

# What a proof is

`datalog.Proof` is how one tuple came to be: its relation, the tuple, the rule that first produced it, and the proofs of the positive body atoms in the rule's written order.

| Case | `Rule` | `Body` |
|---|---|---|
| Base fact | nil | empty |
| Derived tuple | the first rule that produced it | one `Proof` per positive body atom |

Built-ins, comparisons and negations contribute no body entry. A proof is recorded only when `Options.Proofs` is on; it touches the hot loop.

# First derivation

A tuple may have many derivations. The proof keeps the first one, in evaluation order. That choice is stable only because evaluation is deterministic, which is the next property.

# A DAG of values

Tuples recur in bodies: `principal("ana")` appears under both `permit` and `holds`. `Result.Proof` builds each `(relation, tuple)` proof once and reuses that value wherever it appears, so the size is linear in the derivation records and not in the unfolded tree. The structure is finite and acyclic, since a body tuple was always derived strictly before its head. `Proof.String()` prints a shared tuple once and says "(shown above)" the second time.

# Determinism

For one `(plan, base, options)` every evaluation yields the same proof for every tuple. Three things hold that up.

- Walks over maps on the derivation path iterate in sorted relation order.
- `policy.Load` sorts the standing facts (by relation, then by each position's kind and text), because a base is its fact sequence. A proof is therefore a function of the policy's content, not of the order the caller listed facts.
- A test evaluates twice and compares proofs deeply.

The design panel flagged map-ordered `merge` as the cause of non-determinism. After implementation the cutter showed this was overstated: each derived relation appends only from its own delta, so the order was already a function of the inputs. The sorted walk stays as a guard for the proof recorder.

# Naming its rule: Set, Line, Text

The rule inside a proof is a deep copy. Holding it reaches nothing in the shared plan. It carries:

| Field | Value |
|---|---|
| `Set` | `"name/version"` of the rule set Compile took it from; empty for a bare `Parse` |
| `Line` | line in that set's text |
| `Text` | the rule as written, final period included, whitespace trimmed |

So a receipt reads `rbac/1:19  allow(P, A, R) :- ...`. The hash is not in the proof; it stays in `Decision.RuleSets`, and a stored receipt keeps both. See [decision 2](/decisions/2-proof-names-its-rule-set.md) and [rule-set identity](/concepts/rule-set-identity.md).

# Rendering

`String()` is an indented tree, one tuple per line, `set:line text` on the right or `fact`. `MarshalJSON` writes `{relation, tuple, rule: {set, line, text} or null, body}` and never the parser's AST. A tuple proved earlier in the same document is not written out again: it is `{relation, tuple, rule: null, shown: true}`, and its proof is the first in the document with that relation and tuple, so the JSON is linear in the derivation however much of it is shared, as the text is.

# Related

- [Why-not provenance](/concepts/why-not-provenance.md) is the counterpart for a tuple that was not derived.
- [Fail-closed decision](/concepts/fail-closed-decision.md): `Decision.Proof` is zero when denied.
- API: [Result.Proof and WhyNot](/api/datalog-result-proof-whynot.md).

# Citations

[1] [DESIGN.md, Proof contract](../../docs/DESIGN.md)
[2] [datalog/proof.go](../../datalog/proof.go)
[3] [datalog/render.go](../../datalog/render.go)
[4] [Design panel collation, finding 1 and correction](../../docs/reviews/luminary-design-2026-10-22/00-collation.md)
