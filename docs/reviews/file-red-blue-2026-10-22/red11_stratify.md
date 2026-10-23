# RED TEAM: datalog/stratify.go

Commit: `5fe3a78`. Scope: only `datalog/stratify.go`; validation, built-ins, and evaluation inspected solely as caller/consumer context. No production edits.

## Findings

No confirmed correctness or security defect found in the assigned file. No severity-bearing finding.

## Evidence and checked invariants

- Lines 12–44, 54–66, 72–117: independently checked all 19,683 three-node dependency graphs, where each possible edge is absent, positive, or negative. Added `datalog/red11_stratify_test.go`. Its independent transitive-closure oracle rejects exactly graphs containing a negative edge with a return path (including negative self-edges). All accepted graphs satisfy head-stratum >= positive-dependency-stratum and head-stratum > negative-dependency-stratum. `go test ./datalog -run TestRed11 -count=1` passes.
- Lines 84–110: Tarjan assigns distinct discovery indices, propagates child lowlinks, uses discovery indices for stack back-edges, and emits completed dependency components before dependents. Positive SCCs share one level; any internal negative edge is rejected at lines 24–27.
- Lines 54–66 and 88–90: a declared but undefined IDB relation can be an edge target without an initial graph key. DFS still visits it with no outgoing edges, assigning stratum zero. Negating that empty hook correctly raises the dependent to stratum one. Base relations and built-ins do not add IDB dependencies.
- Lines 121–137: rules preserve source order inside each stratum. Every positive IDB read at the same stratum is marked for delta reevaluation, including reads between distinct positive SCCs sharing the same level. This conservative marking is necessary because the evaluator initializes the whole stratum before merging initial deltas. Lower-stratum reads need no delta slot. Negated reads cannot share the head's stratum after successful stratification.
- Lines 143–176, 181–198: positive atoms preserve written order. Guards are queued once; constant guards may run first, and variable guards become eligible immediately after the last binding positive atom. Wildcards in negated atoms are deliberately ignored by readiness. Comparisons/built-ins do not bind variables. No guard drops on the fallback path.
- Determinism: graph keys and SCC members are sorted; adjacency and rule order come from input slices. Map iteration in `group` computes only a maximum. Fixed program input therefore yields stable levels, body order, rule order, and cycle diagnostic selection. Permuting source rules can change diagnostic choice and first-proof ordering; source ordering is preserved intentionally, rather than canonicalized.
- Empty/nil rules and IDB maps terminate normally; an empty program yields one empty stratum. Duplicate dependencies do not alter semantics. Multiple rules for one head accumulate all dependencies.

## Caller assumptions

`stratify`, `group`, `order`, and `ready` are internal helpers, not independent validation entry points. `Compile` checks shape before walking literals, builds the complete IDB map from declarations and heads, rejects base/built-in heads, validates relation arities, and requires every named guard variable to occur in a positive ordinary atom. Consequently, nil atom/comparison dereferences on malformed direct helper calls, missing IDB entries, and unsafe pending guards are not externally reachable through the checked caller. `Compile` also clones programs, so body pointers retained in ordered rules are plan-owned. The built-in registry is assumed immutable after initialization as documented by its owner.

## Scale observations, not promoted findings

- Lines 84–90 recurse to dependency-chain depth. Go grows goroutine stacks, but sufficiently huge adversarial programs could exhaust stack/memory. No practical failure threshold or exposed hostile-input contract was demonstrated; stack exhaustion is a suspicion, not a confirmed defect.
- Lines 152–174 rescan and reallocate the pending guard slice after every positive atom. Exact worst case: P positive atoms followed by G guards all requiring a variable bound only by the final positive atom gives Θ(P×G) readiness work and pending-slice copy allocations (in addition to term scanning). This is confirmed complexity from the implementation, but no material application regression or resource boundary was demonstrated, so it is recorded as a scale limitation rather than a severity-bearing bug.
- Graph traversal stores duplicate dependency edges. Space is proportional to body references; ordering adds sorting costs for relation names and SCC members. Maximum numeric stratum is bounded by dependency path length, so overflow cannot arise before physically impossible input sizes.

## Verification

The additional exhaustive test passes. `go test ./datalog` fails in concurrently present `TestRed07ProofMemoRelationCollision`; that failure is outside this file's scope and was not investigated. Existing compilation/evaluation tests cover unsafe negation, negative cycles, positive recursion, lower-stratum negation, and undefined declared hooks. No crash-size stress run was performed. No unrelated review reports/history consulted.
