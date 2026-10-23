# Blue review: datalog/proof.go

Scope: every function in proof.go at snapshot 5fe3a78; API types, derivation recording and existing proof tests consulted solely to establish contracts. No production edits.

## Defensive posture and assumptions

`Result.Proof` fails closed when recording was disabled, the relation is absent, or tuple membership fails. Zero Result is safe. Membership and typed, length-prefixed tuple identity prevent wrong-arity and scalar-type confusion. Per-call builders isolate concurrent independent readers and returned values from subsequent queries.

`builder.build` clones tuple slices, memoizes completed subproofs, and preserves recorded premise order. Its lack of cycle detection is justified by private Result fields and Evaluate's strictly-earlier derivation invariant; public callers cannot insert cycles into these internal records. Positive premises only, first derivation only, and base leaves with no rule are explicit contracts, not missing evidence checks.

`builder.rule`, `copyLiteral`, and `copyAtom` copy the current Rule's complete fields and all mutable nested slices/pointers. Const and Term contain scalar values, so shallow element copying is sufficient. Shared rule copies and shared subproof slices within a single answer are intentional DAG ownership; callers must expect mutation of one shared occurrence to affect another occurrence of that same answer. Returned mutation does not reach the plan or Result. Existing tests cover recursive termination, premise order, first-proof determinism, disabled/missing queries, program facts, and deep rule-copy isolation.

## Concrete gap: composite memo identity is ambiguous for accepted programmatic names

`build` concatenates relation name, NUL, and tuple key. Tuple strings may contain NUL; programmatic schema and atom names can also contain NUL (current checks accept them). For relation `a`, tuple `["x\\u0000", "y"]`, the memo key equals that for relation `a\\u000002:x`, tuple `["y"]`: both encode `a NUL 02:x NUL 01:y`. A parent referencing both returns the first premise again for the second. The explanation names the wrong relation and wrong-arity tuple even though evaluation correctly matched both facts.

Evidence: `go test ./datalog -run '^TestBlue01ProofMemoBoundary$' -count=1` fails with `second premise replaced by first: relation "a", tuple [x\x00 y]`. The uniquely named test is retained in datalog/blue01_proof_test.go. This uses only public constructors and Compile/Evaluate; no internal corruption. Parser-produced relation names avoid the problem, so exposure requires programmatic names. Recommend a map key struct containing separate relation and tuple-key strings; this avoids tightening existing name contracts.

## Resource and maintenance recommendations

Memoization bounds construction to reachable nodes and premise edges rather than unfolded paths, but recursive build still consumes stack proportional to derivation depth, and Proof has no cancellation or extraction budget. Evaluate's documented limits apply to evaluation, not this extraction. This is a robustness consideration for unusually deep accepted inputs, not a demonstrated crash or violation of a promised extraction limit. Consider iterative traversal if untrusted workloads require a hard depth guarantee; add a long linear derivation benchmark/test without transitive-closure quadratic setup.

Add regression tests for tuple and Body mutations followed by another query on the same Result; actual alias sharing in a diamond (current shared-subproof tests primarily compare equal values); concurrent independent Proof calls under the race detector; and composite identities with NUL-bearing names/strings. Keep the deep-copy test synchronized with any future reference-bearing Rule/Term fields: manual Rule construction currently copies every field but offers no automatic protection when the type grows.
