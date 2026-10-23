# Blue review: datalog/relation.go

Scope: snapshot 5fe3a78; relation.go only, with builder/base/plan/eval/term contracts read for reachability. No review or red files inspected. No production changes.

## Assessment

No confirmed public-input correctness or panic defect found in this file. Its safety is intentionally conditional on validated arity, immutable owned tuples, non-nil source selection, and evaluator-owned environments. The clearest remaining concern is resource accounting before candidate iteration.

## Defenses already present

- Candidate selection is a sound superset: choosing the first bound argument cannot exclude any fully matching tuple; bind checks every remaining argument. Repeated variables are checked against their first binding, and anonymous variables are skipped independently.
- Failed bind rolls back only names introduced by that attempt. Pre-existing environment bindings survive; repeated names are appended once because the first insertion makes subsequent occurrences bound.
- Indexed lookup and scanned lookup share constant identity: key includes kind and length-delimited text; same compares kind and canonical text. Numeric canonicalization is upstream. Thus string/number/bool identity and embedded delimiters do not diverge between paths.
- Public fact arity is checked and tuple slices copied in Base.group. Compile checks atom arity; Plan.accepts checks runtime base arity. These are the actual defenses against t[pos] and t[i] panics.
- relBuilder.seal clears the builder and transfers its storage. Base relations have eager indexes and no evaluator writes; derived builders remain evaluation-local. Returning borrowed tuple/candidate slices is safe while callers only read them.

## Boundaries to retain explicitly

relation.go:27-37 and 87-104 do not validate positions or tuple/argument arity. Invalid positions return an empty result on an indexed relation but panic on a nonempty scanned relation; short tuples can panic in bind, while fewer args can match a tuple prefix. These are internal precondition violations, not demonstrated public exploits. Adding local checks could make internal failures clearer, but must not silently turn broken invariants into ordinary misses.

source with neither pointer set panics and both pointers set prefer val. Keep construction internal and retain the exactly-one-source invariant. A nil environment can be read but cannot accept a fresh bind. value silently returns the zero constant for an unbound variable; callers must maintain the compile-time safety guarantee.

Returned candidate slices alias index storage and tuples() exposes owned storage internally. Any future sorting/filtering-in-place or mutation by a caller would corrupt subsequent lookups or relation identity. Preserve read-only usage or introduce an iterator/copy at that boundary.

## Resource concern

relation.go:77-80 allocates and fills an O(N) integer slice before the evaluator charges its first join step. With a large base and MaxSteps=1 this still costs roughly 8*N bytes on a 64-bit platform. An early successful exists call likewise prepares every index before visiting one tuple. Repeated broad joins multiply these transient allocations. Scanned candidates (31-36) also do an entire pass before a caller can check cancellation or charge a step; a miss has no returned candidates to charge. Builder candidate-index construction has the analogous upstream cost. This means MaxSteps bounds tuple attempts, not total CPU or allocation; MaxBytes only counts derived output. This is a concrete limitation, not a claim that limits are wholly ineffective.

Suggested direction: iterate a numeric range for unbound scans, and provide cancellation/work accounting during candidate construction or scanning where those operations are in a bounded public path. Avoid copying existing indexed candidate lists merely for defensive ownership without evidence of a mutable caller.

## Meaningful future tests

1. Differential property test: indexed, sealed-unindexed and growing-builder sources must yield exactly the same successfully bound tuple set as a simple full scan, across constants at every position, empty relations, repeated variables, wildcards, existing bindings, and mixed scalar kinds. Compare final matches rather than requiring identical candidate lists.
2. Transactional bind test: mismatch after several new bindings (including a repeated variable) restores an environment containing unrelated and pre-existing bound names byte-for-byte. Successful binding reports each newly introduced name exactly once; undo restores the original environment.
3. Identity corpus: delimiter/NUL-containing strings, empty strings, string "1" versus number 1, bool true versus string "true", equivalent number lexemes, and negative zero. Compare index and scan results.
4. Public boundary tests: malformed program atom arities, wrong fact arities, and runtime bases with incompatible arities must return ErrInvalid before matching/bind. Include caller mutation of input tuple slices after NewBase to verify ownership.
5. Allocation benchmark: large all-unbound matching/early exists with a tiny MaxSteps budget, varying N. Record allocated bytes and cancellation latency; this should motivate an iterator and then serve as a regression check. Avoid flaky wall-clock assertions.
6. Concurrent evaluations sharing one immutable base under the race detector, combined with Base.With snapshots, to protect borrowed-slice ownership.

No tests added or run; this is a contract and implementation review.
