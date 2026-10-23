# Blue 08: datalog/builtins.go

Snapshot: `5fe3a78`. Independent review confined to `datalog/builtins.go`; read SPEC.md, README.md and existing builtins tests for contract and validation. No review reports or red-team artifacts read. No production changes.

## Assessment

No confirmed externally reachable correctness or authorization bug found in this file. The three current entries implement exactly the documented finite string guards. Several safety properties deliberately depend on compilation and scheduling elsewhere; this review does not certify those implementations.

## Existing safeguards

- Lines 20–22 register only deterministic, side-effect-free standard-library string predicates. They produce no values and cannot expand the term domain.
- Lines 31–34 check both kinds before accessing text, preventing accidental numeric/boolean string coercion. The pattern/subject reversal correctly adapts the public pattern-first convention to Go string functions.
- Lines 25–28 explicitly document non-string false and negated non-string true. Line 51 implements boolean complement for every predicate. An attacker exploiting a permissive negated guard on a non-string value is exercising the specified semantics, not bypassing an internal type check; policies requiring strings must obtain that restriction from schema or positive conditions.
- Empty patterns match all strings, including empty subjects; case is significant. These follow Go string semantics and existing tests cover them. There is no regex parser, escaping interpretation, path cleanup or locale dependence here.
- Registry iteration does not affect evaluation: dispatch is by exact name. Current predicates contain no random, clock, environmental or mutable state.
- Existing tests exercise fake registration through compiler restrictions, arity, unbound/anonymous terms, ordering, constants, negation, mixed kinds and recursion. `go test ./datalog -run TestBuiltins -count=1` passed.

## Real boundaries and residual gaps

1. **Unchecked internal preconditions (lines 29–34, 46–51).** The evaluator trusts name existence, arity and term binding. A missing registry entry causes a nil-function panic; fewer than two arguments panic in stringTest; surplus arguments are silently ignored by its callback. These are concrete local behaviors, but not demonstrated public-input defects: existing compiler tests explicitly reject bad arity and unsafe terms. Retain compilation as the enforced boundary, and test exported construction paths if future changes bypass Compile. Adding runtime checks alone requires deciding how invalid internal state should propagate, since this function currently returns only bool.

2. **Predicate work is not interruptible or metered locally (line 34).** Prefix/suffix inspect matching bytes, and contains searches the subject. One invocation's CPU work depends on string length; it cannot observe context cancellation or charge byte work because its signature has neither context nor budget. This is a local limitation, not a demonstrated global budget bypass: base/request byte limits and caller budget checkpoints may adequately bound it. Recommend an integration stress test measuring cancellation latency and step/byte limits for large admissible repeated strings before making any stronger claim. Keep future callbacks bounded; regex, parsing, external calls, or generated values would invalidate today's simple resource reasoning.

3. **Registry immutability/purity is a convention (lines 8–22).** The private map prevents consumer registration through the public API, and its current entries are safe. Package-internal code can replace/delete entries after compilation, race with Evaluate, change arity between compile and execution, or install a stateful/nil callback. Existing tests intentionally mutate it sequentially. This is a maintenance hazard rather than a present attacker-controlled capability. If a public extension mechanism is ever introduced, freeze a validated registry per plan (or equivalent immutable snapshot), validate name/arity/callback, and explicitly require deterministic, total, finite predicates. Do not imply arbitrary callbacks inherit current guarantees.

## Recommended regression additions

- Table-driven differential test against strings.HasPrefix/HasSuffix/Contains over empty strings, long near-matches, multibyte Unicode, combining characters, embedded NUL, and invalid UTF-8 if public Const construction admits it. Assert both positive and negative guards and pattern-first order. Byte equality, not Unicode normalization or path boundaries, should be the expected semantics.
- Extend mixed-kind coverage to every publicly supported Const kind and both argument positions, including zero/invalid values only where public constructors permit them; assert exact complement under negation.
- Repeated and concurrent evaluation of the same compiled plan/base with all three guards should yield identical answers/hashes and pass the race detector. Keep registry-mutating hook tests nonparallel.
- Add a registry-wide invariant test ensuring each current entry has expected arity and a nonnil callback. Preserve the existing fake predicate test so additions cannot bypass compiler/scheduler integration.
- Budget/cancellation stress test above, using legal admitted inputs and recording observed limits; avoid a timing-only fragile unit assertion.
