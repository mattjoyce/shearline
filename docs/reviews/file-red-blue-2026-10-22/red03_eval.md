# RED03: datalog/eval.go

Reviewed at `5fe3a78`; production unchanged. Neighbor reads were limited to plan validation, relation/builder ownership and candidate lookup, base construction, term representations, and evaluator tests.

## Confirmed finding

### P2 — Cancellation is never revisited for arbitrarily many rule applications with no candidate visits

Locations: `datalog/eval.go:99-125`, `:230-245`, `:288-294`, and successful return at `:65`.

`Evaluate` observes `ctx.Err()` once at entry; subsequent observations occur exclusively every 1,024 candidate visits in `step`. Valid ground facts, guards, and negations whose candidate sets are empty can execute without any candidate visits. There is no cancellation check between rules/rounds or before successful return. Consequently, cancellation after entry can be ignored throughout an arbitrarily large plan, and the evaluator returns success with a context that has long been canceled. Duplicate ground rules keep derived bytes constant, so setting MaxBytes does not make this path interruptible.

Evidence: `datalog/red03_eval_test.go`, `TestRed03CancellationWithoutCandidateVisits`, compiles 20,000 copies of `answer("ok").`, then cancels the ordinary backing context immediately after the initial active-context observation. Evaluation processes the entire plan successfully with `MaxSteps: 1, MaxBytes: 100`; backing context is demonstrably `context.Canceled`. A context wrapper places the cancellation deterministically at this scheduling boundary; it does not modify evaluator internals. The behavior is confirmed, while worst-case real-world delay depends on plan size and machine speed.

MaxSteps itself is **not** incorrectly enforced by this example: its documented unit is visited tuples, and no tuple is visited. The defect is the unbounded interval between cancellation observations. Check context during rule traversal and before successful return; retain candidate polling for large joins.

## Tested defenses and non-findings

Ran `go test ./datalog -run 'TestRed03|TestEvaluate_' -count=1 -v`; all selected tests passed, including existing recursion/fixpoint, stratified negation, comparisons, duplicate-work MaxSteps, negation-probe MaxSteps, last-tuple MaxBytes, pre-canceled contexts, foreign-base rejection, derived/base collisions, deterministic derivation order, and concurrent evaluations sharing a base.

Ownership inspection found no evaluator mutation of caller-owned bases/plans: derived builders are evaluation-local; sealing transfers only those builders; proof rule references point into the immutable compiled plan; body tuple references point into immutable relation data. Compile's defensive copying and Base's tuple copying support these assumptions. Nil context is outside the standard context API caller contract and is not reported as a bug. Negative option values currently behave as unlimited; Options does not promise rejection of them, so this is not a finding.

MaxBytes explicitly approximates unescaped derived tuple text, excludes base data, and is not a heap-memory cap. Proof stacks, tuple keys, indexes, and candidate arrays are therefore not reported as byte-accounting defects. Candidate generation at `:252`/`:311` happens before `step`, and merge at `:133-145` performs uncharged work; those can delay cancellation/allocate before the next step check. These are inspection-only related limitations, not independently reproduced findings here.

No additional confirmed correctness or malformed-input defect was found in the assigned file. Validated Plan/Base are opaque public values; join's atom dereferences and tuple indexing rely on their compile/construction invariants.
