# Independent blue review: datalog/eval.go

Snapshot: 5fe3a78. Scope: evaluator only; supporting relation/builder, plan, proof contracts and existing evaluator tests inspected. No production edits. No review/ or red* file contents inspected.

## Defenses that hold

- Entry cancellation precedes plan/base validation and allocation. Errors wrap the context error, preserving errors.Is.
- Every positive candidate and negation candidate reaches step before binding. MaxSteps rejects candidate limit+1, including failed unifications and duplicate-producing joins. The counter spans all rules, strata and rounds.
- MaxBytes counts each newly derived relation/tuple once, including intermediate relations, deduplicated against both the existing database and current round. The post-stratum/final ev.over checks cover the last-candidate crossing that might otherwise disappear when the next delta has no match.
- Each round reads only the previous database/delta, collecting new tuples separately. Merging precedes the following round, allowing nonlinear recursive rules to use the delta at each recursive position. Stratified negation reads complete lower strata.
- New evaluators own their builders, environments, counters, proof maps and stacks. Shared plan and base are read-only here. Sealing relinquishes builders; public proof extraction copies rule structures and tuples, so internal rule/body tuple sharing does not expose plan/base mutation.
- Proof records are installed only after successful first insertion. Stack storage is copied for each derivation. Proof antecedents precede the head's round, preventing proof cycles even for recursive programs. Sorted merge order and stable rule/candidate order make first derivations deterministic for the same plan/base sequence.

## Explicit units and choices, not defects

- MaxBytes is approximate unescaped tuple JSON, not heap, serialized Result size, proof memory, base size, keys/indexes, relation names, or execution work. A large proof body for a tiny head can therefore consume much more memory than this byte budget. This follows the documented unit; an actual heap budget would be a separate feature.
- MaxSteps counts candidates returned to join/probe loops, not every CPU operation or every source tuple examined while preparing an index. It is exact in that unit. Zero is unbounded; negative values are also effectively unbounded in this implementation, although only zero is documented.
- Cancellation polling every 1024 candidates explicitly trades latency for overhead. Cancellation after the entry check can go unobserved when fewer than 1024 candidates are visited. There is no final ctx.Err check. That follows the documented polling model, rather than establishing a strict cancellation-at-return guarantee.
- Bounds can overshoot by the crossing tuple/candidate before returning an error; equality is allowed. No partial Result is returned on errors.
- Proof body records contain positive atoms only. Guards and negations are present in the referenced rule but are not individual proof children, consistent with the public proof contract.

## Remaining gaps / useful hardening

1. **Uninterruptible candidate preparation weakens the broad claim that deadlines bound evaluation.** join and exists call matching before either step or a cancellation check. matching can allocate O(N) integer candidates, and a derived builder can build an O(N) index before returning zero candidates. merge, stratum/rule iteration, sealing, large tuple construction/key generation, and proof recording also do not poll. Consequently an already-running evaluation can spend substantial time after cancellation without reaching the next candidate poll; a plan with many ground rules can perform arbitrarily many rule applications with zero steps. These are omissions in cancellation coverage, while the MaxSteps candidate definition itself is honest. Recommend checking context at rule/round boundaries and making expensive preparation cooperatively cancellable if strict prompt cancellation matters.

2. **Byte refusal is latched, not immediately propagated from emit.** A crossing in a recursive round can proceed through merge and further rule setup before another positive candidate or final Evaluate check returns it. No successful over-budget result is apparent, but avoidable indexing/allocation can follow the refusal. Returning an error directly through emit/apply, or checking ev.over immediately after every recursive apply and before merge, would tighten fail-fast behavior.

3. **Counter overflow is unchecked.** bytes, tuples and steps are ints. Impractically large evaluations could wrap counters and defeat comparisons; long-running tiny-output duplicate workloads reach this earlier on 32-bit than 64-bit systems. This is defensive hardening, not an observed practical bypass on the reviewed host.

## Specific regression tests

- Exact boundary: two unique numeric unary tuples charge 6 bytes; MaxBytes 6 succeeds, 5 fails. Include duplicate rules and same tuple in different relations to verify deduplication scope.
- MaxSteps: one rejected repeated-variable binding still charges a candidate; negation charges the first matching candidate and every failed candidate; limit equality succeeds and next candidate fails.
- Cancel during evaluation with deterministic context instrumentation, across a 1024-step boundary; verify errors.Is and no partial Result. Separately demonstrate/document the sub-1024 success window and a ground-fact-heavy program's zero-step cancellation gap.
- Lazy derived index miss with many tuples: instrument preparation and demonstrate that zero returned candidates incurs no join steps. Use a benchmark or explicit hook, not a flaky wall-clock assertion.
- Byte crossing on the last recursive candidate followed by another stratum with empty candidate sets: must return BoundBytes. With proofs on, verify identical bounds and failure metadata.
- Nonlinear recursion and mutual recursion against a simple naive oracle; permutation of rules must preserve tuples, while first proof follows the fixed supplied plan order.
- Multiple derivations and cyclic graph: assert first proof, written positive-body order, no cycles, and no overwritten records. Concurrent Evaluate with Proofs on/off over one plan/base under -race; mutate returned proof values and re-evaluate to confirm ownership.

Existing eval_test.go covers recursion, stratified negation, comparisons, pre-cancelled contexts, byte and step refusal, final recursive tuple byte crossing, negation probe charges, and answer ownership. None of the remaining observations establishes wrong derived tuples or a successful over-limit candidate-count result.

Validation: ran the existing TestEvaluate_ tests. A broad package run also encountered failures in other reviewers' added tests outside this file's scope; those test files were not inspected.
