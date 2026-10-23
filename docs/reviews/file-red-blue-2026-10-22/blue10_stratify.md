# Independent blue review: datalog/stratify.go

Scope: snapshot identified as 5fe3a78; reviewed only stratify.go as implementation, with plan.go, shape.go, builtins.go and existing plan_test.go as contracts. No prior review material or red tests read. No production edits or additional tests.

## Assessment

No semantic stratification defect found. Tarjan's lowlink updates distinguish active ancestors from completed SCCs correctly. Dependencies point from consumers to producers, so emitted SCCs are dependencies-first; taking the maximum predecessor level plus one for negative edges implements stratification. Any negative edge inside an SCC is correctly rejected, including a negative self-loop. A declared but undefined relation is visited when reached, gets level zero, and remains empty as documented.

Determinism is adequate for the same ordered Program input: root names and component members are sorted, adjacency order follows the supplied rule/body order, and grouping preserves supplied rule order. Reordering logically equivalent source programs need not yield identical error witnesses or rule plans; that stronger property is not promised.

Guard movement is safe under Compile's preceding shape and variable-safety checks. Constants and wildcard-only negations may execute first; built-ins bind nothing; positive atoms retain source order; every named guard variable must eventually be bound. `recursive` indexes refer to the reordered body and intentionally include all same-stratum positive derived relations, rather than only relations in the head's SCC.

## Concrete resource weakness: quadratic guard scheduling

Lines 152–174 rescan and copy every pending guard after every positive atom, including atoms that bind no new variables. A valid body containing N copies of `node("a")`, then `node(X)`, plus N copies of `X = "a"` requires approximately N² readiness checks and cumulative copied literals despite O(N) source size. The guards can occur anywhere in the original body because the initial collection includes them all. `still` starts empty on every flush, so allocation traffic is also quadratic. This is a demonstrable complexity property, not a demonstrated process crash. Exploit significance depends on whether callers accept untrusted, large programs; evaluation MaxSteps/MaxBytes do not bound Compile.

Suggested defense: compute each variable's earliest binding position once, assign each guard to the maximum position of its named variables (position zero for ground guards), then emit stable buckets after each positive atom. Preserve original guard order within a bucket. At minimum avoid flushing when no new binding was added, although that alone does not eliminate the general quadratic case with many distinct early variables and guards awaiting a final variable.

Suggested test: compile valid rules with N and 2N late-bound guards in a benchmark with allocation reporting. Prefer a structural/instrumented work invariant over a timing-based unit test.

## Theoretical limits, not confirmed defects

- Recursive DFS at lines 84–90 consumes O(depth) Go stack. An arbitrarily long valid dependency chain can require large stack growth; sufficiently large input can eventually hit Go's fatal stack limit. No crash size was established. An explicit DFS frame stack or explicit compile input/depth limit would make this bounded and recoverable.
- Duplicate dependency edges increase memory and traversal work, but remain linear in source size. Integer stratum overflow would require more graph levels than can realistically fit in memory; no actionable integer-overflow defect found.

## Missing invariants worth testing

1. Generate small signed graphs and compare rejection with an independent reachability oracle: reject exactly when a negative edge's endpoints are mutually reachable.
2. For every accepted graph, verify positive edges have head level >= dependency level and negative edges have head level > dependency level, including undefined declared hooks and disconnected nodes.
3. Guard scheduling property: output preserves literal multiplicity, positive subsequence, and stable guard ordering at each earliest binding boundary; each named variable is bound before its guard. Cover repeated variables, anonymous negation positions, ground false guards, and negated built-ins.
4. Repeat identical compilations to assert plan/error determinism; distinguish this from source-permutation invariance.
5. Exercise a long chain and a large positive SCC under a documented compile resource policy. Keep fatal stack-overflow probes out of the ordinary in-process test suite.

These proposed checks strengthen assurances; their absence does not establish semantic failure.
