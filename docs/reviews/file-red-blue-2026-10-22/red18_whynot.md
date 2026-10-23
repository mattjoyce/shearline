# RED TEAM: datalog/whynot.go

Reviewed assigned file at baseline 5fe3a78. Neighbor definitions inspected only for matching, constant keys, plan arities, and proof ownership seams. No production edits; no other review reports or history read.

## Confirmed finding: partial goals lose repeated-variable constraints (medium)

Locations: `datalog/whynot.go:289-307` (`unifyHead`), with related memo-key loss at `327-340` (`goalKey`).

Public trigger:

```datalog
p("a", "b").
q("missing") :- p(X, X).
```

Evaluate with proofs enabled, then call `WhyNot("q", Tuple{String("missing")}, 0)`. The goal is correctly absent. Its failed atom is `p(X,X)`. However, the nested Absence includes the fact rule `p("a","b")` as a unifying rule, even though its unequal arguments cannot satisfy X=X. The resulting nested RuleFailure has zero Failed, no Because, and Truncated=false. This violates the documented invariant that only unifying heads are included and that a full body match cannot occur when explaining absence.

Cause: an open goal term binds nothing; goal-variable identities and equality constraints never enter unification. This also affects variable heads: for goal p(X,X), head p(Y,"a") must bind Y="a", but the walker instead explores all Y values. It can therefore select irrelevant failures or produce another spurious full match.

Related confirmed memo defect: `goalKey("p", [X,X]) == goalKey("p", [X,Y])`. Renaming variables is safe, but erasing equality classes conflates different existential queries. A previous independent-open-goal explanation can suppress a later equality-constrained one (and conversely). The key collision itself is directly tested; a complete public call exhibiting suppression was not separately constructed. Treat this as part of the same constraint-loss finding, not a separate severity claim.

Evidence: `datalog/red18_whynot_test.go`; `go test ./datalog -run '^TestRed18WhyNot' -v` passes two characterization tests confirming the public incorrect explanation and memo collision. These tests assert observed defects, not desired future behavior.

## Nonfindings and limits of review

- Public name/tuple arity checks and proof-disabled/present-tuple rejection precede the walk. Zero Result safely rejects through absent relation/arity. Nonpositive maxDepth consistently selects the documented default (including negative input).
- Concrete top-level heads correctly reject conflicting constants and repeated head variables receiving unequal constants. The defect requires recursively produced partial goals.
- Positive candidate binding rolls back newly introduced variables; failed binds undo partial writes. Negated probes use the same binder and remove temporary bindings before returning the blocking tuple. Ordinary repeated variables in body matching remain enforced.
- Farthest-failure selection retains the first path at each index as documented. Bound maps, literal arguments, goals, copied rules, and blocking proofs do not expose plan/result-owned mutable slices/maps through the inspected seams.
- Depth decrements only across Because; a limit sets truncation and propagates upward. Seen goals prevent repeated recursive descent. Step exhaustion propagates to enclosing Absence even when a prior failure was retained. The candidate budget permits 65,536 charged candidates and stops before using candidate 65,537.
- The step budget bounds charged candidates, not total CPU/allocation: `matching` can scan/materialize an entire relation before `spend` runs, and rule lookup scans the plan. This is a real limitation of the documented candidate budget, but no separate demonstrated exploit/performance regression is claimed here.
- Caller-selected large maxDepth and long ordered bodies can permit deep stacks; no stack exhaustion was demonstrated. No additional confirmed security or evaluation-result corruption found in this file.
