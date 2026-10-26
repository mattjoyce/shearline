# The engine, as its consumers see it

> **Status, 2026-10-26: the cut is done.** The engine is
> [github.com/mattjoyce/tarski](https://github.com/mattjoyce/tarski), tagged v0.1.0 and v0.2.0
> (aggregates), then v0.2.1 and v0.2.2 (fixes), and Shearline now requires v0.2.2 with its
> `datalog/` package deleted. Its design and changelog live there (`docs/DESIGN.md`, `CHANGELOG.md`). Items 1 and 2 below are shipped;
> what Shearline did on the tag is recorded at the end. The rest of this file is kept as the
> consumer's steer it was.

Direction of travel for the Datalog engine that was cut out of `shearline/datalog` into a
module of its own. Written 2026-10-25 by the Shearline lead, from design conversations with
Matt, for the agent doing the cut and for anyone deciding what the engine should grow next.
Shearline is a consumer of the engine, not its owner. This is a consumer's steer.

## What the engine is

A function. It takes a plan (compiled rules) and a base (a set of facts) and returns a set of
derived facts, with, on request, a proof for any member and a why-not for any non-member.
Nothing else. It reads no file, holds no state, knows no vocabulary: `role`, `entry`,
`grants` are strings to it.

Everything about it follows from treating relations as sets. A rule is four set operations:
join (intersection on shared columns), projection (drop columns), union (two rules, one
head), difference (`not`, under stratification). Recursion is the least fixed point: apply
the rules until the set stops growing. The sets are finite because no rule can make a new
value, so evaluation terminates. Sets have no order, so facts can arrive in any order and the
answer, the proof and the hash are the same. Sets have no duplicates, so a fact stated twice
changes nothing.

## The admission rule

A feature belongs in the engine only if, with it, evaluation is still

1. a pure function from (plan, base) to a set,
2. terminating,
3. deterministic, including the bytes of the canonical answer and its hash,
4. boundable by `Options`, in derived bytes and in work.

Arithmetic in rule heads (makes new values: fails 2), regular expressions (fails nothing in
theory and everything in practice: cost is unbounded by the input), floating sums in
arbitrary order (fails 3), and any state across evaluations (fails 1) stay out. Every string
built-in so far, `prefix`, `suffix`, `contains`, passes all four, and so would `under(Sep,
Pat, R)`, the segment-aware prefix decision 1 in `DESIGN.md` reserved.

## The two consumers, and what each asks of the engine

**Shearline, policy.** One decision per request. The base is a few hundred standing facts
plus three or four request facts. The rules are nineteen lines. What matters: the engine
must be cheap to call once per request; the bounds must be per request, so the bundled rules
are seeded by the request (the 38,000-principal cliff, `docs/reviews/luminary-module-…`);
proofs must name their rule set, line and text, because a decision is a receipt; why-not
must say how far the losing rule got, because a refusal that cannot explain itself is one
nobody will trust. Policy will never need aggregates, and `policy.Load` refuses a consumer rule set that uses
them (done, with tarski v0.2).

**Anam, query.** One evaluation per question over a ledger of thousands of entries. The base
is `entry/7`, `attr/3`, `closes/1`, materialised from the fold at a watermark. The rules are
Anam's own `anam.dl`, parameterised by `instant/1`. What matters: the whole ledger is in the
base and only a sliver is relevant to the question; answers are hashed and compared with the
fold's own snapshot as an oracle; a query over payload attributes is attribute-based access
and stays in Anam. Anam will need aggregates: the first counting question cannot be written
today. Anam's unmodified `rules/anam.dl` compiles and evaluates correctly on the engine as
of Shearline v0.1.0, with proofs, which it never had before; a test that keeps this true is
cheap insurance.

Anam will import both: the engine for `anam query`, Shearline for who may run which read.
That makes a diamond in `go.mod`, resolved by Go to one engine version, so the `Fact` and
`RuleSet` types Anam builds are the types Shearline accepts. The engine's exported types are
therefore shared currency between two consumers and should change rarely and deliberately.

## Shapes the engine already serves, worth keeping in mind

- **For-all as negation.** "Every requested unit is covered" is `missing(P) :- asked(U), not
  covered(P, U).` and `… not missing(P)`. Anam's spans-as-filters design rests on it.
- **Hierarchy as prefix.** `facet/health.` covers every health type; `2026-06` covers every
  day in June, when units are ISO strings. One built-in, no path semantics.
- **Declared and undefined is empty.** A rule set can leave a hook relation open for a
  consumer set to fill. Shearline's `denied/3` and Anam's `instant/1` both use it.
- **Request facts.** `Base.With(facts)` adds a request's facts to standing ones without
  copying. Policy's PARC facts and Anam's `instant/1` are both this.

## Direction of travel

In order, each gated on a consumer actually needing it.

1. **The cut.** History carried by `git filter-repo --path datalog/`. API frozen at today's
   surface for the first tag, so Shearline's bring-over is a path rewrite and every test and
   hash still passes. Stdlib only. The admission rule in the README.

2. **Aggregates.** `count`, `sum`, `min`, `max` in a rule head over a grouped body.
   Stratified like negation: an aggregate over a relation that reaches itself is refused at
   compile. Deterministic: sum in canonical order, or restrict `sum` to exactly representable
   values. Charged to the bounds as any derived relation is. Proof of an aggregate tuple is
   the group it was taken over. This is Anam's first ask.

3. **Demand-driven evaluation.** Given a goal relation, derive only tuples that can reach it
   (the magic-sets transformation, or a simpler sideways-information-passing restriction).
   Semantics unchanged, every existing test and hash still holds. It generalises what
   Shearline did by hand in `rbac.dl` and what Anam does with `instant/1`, and it makes
   consumer rule sets safe by construction rather than by discipline. Both consumers benefit.

4. **`under/3`**, the segment-aware prefix, if and when a consumer shows segmented names where
   `prefix/2` is wrong. Decision 1 in `DESIGN.md`.

Not planned: arithmetic, regex, function symbols, mutable state, a file format, a network
face, any vocabulary.

## What Shearline did when the module tagged

Done in PR #1 (merged to master): imports rewritten to `github.com/mattjoyce/tarski`, `datalog/`
deleted, `examples/logsvc/go.mod` requires tarski as well, README says "one dependency, our own
engine", `DESIGN.md` § Package `datalog` is a pointer to tarski's contract, `okf/packages/datalog.md`
points to the module, and `policy.Load` refuses aggregates. The original plan, for the record:

Rewrite imports; replace `datalog/` with the dependency; update `DESIGN.md` § datalog API and
§ evaluation invariants to point at the module's own contract; rewrite `okf/packages/datalog.md`
and the engine concepts to reference the module; change `examples/logsvc/go.mod`; change the
README's "standard library only" to "one dependency, our own engine"; tag Shearline v0.2.0. All of it is done: v0.2.0 is tagged at the merge of PR #3, and `examples/logsvc` requires that release rather than a `replace`.
The policy tests, the example, and every hash in the README are the acceptance check.
