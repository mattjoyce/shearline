# RED TEAM: datalog/relation.go

Scope: `datalog/relation.go` at 5fe3a78. Neighbor files inspected only for construction, validation, ownership and call-site seams. No other reviews/history consulted. No production edits.

## Confirmed finding

### Low: candidate generation does unbudgeted linear work before explanation limits apply

Location: `relation.go:31-37`, `relation.go:77-80`.

Sealed derived relations deliberately have no indexes. A lookup for even one bound argument scans every tuple before returning its candidate slice. With no bound argument, matching allocates and populates an integer slice for the entire relation. Callers cannot stop either operation early. In particular, `walker.positive` and `walker.probe` in whynot.go charge `spend()` only after matching returns, and charge nothing at all for a missing bound value. Consequently, the why-not candidate budget does not bound lookup CPU or temporary memory. Repeated missing derived lookups can each scan a large relation without exhausting that budget.

Concrete bounded evidence: `TestRed14MissingDerivedProbeSpendsNoBudget` constructs an unindexed 100,000-row relation, probes for an absent bound value through the actual why-not `walker.probe`, and confirms `steps == 0`, `held == false`, `stop == false`. The complete scan follows directly from the loop at lines 32-35. This is a confirmed resource-accounting limitation, not an incorrect answer or a violation of the narrowly worded “body-literal candidates” count. Severity is low because substantial relation size must already exist; no claim of an externally exploitable denial of service is established here.

Suggested direction: account for scanned rows during candidate generation, use an iterator that can consult the caller's budget, or provide bounded/lazy explanation indexes. All-unbound matching could iterate row positions directly without constructing a full integer slice.

## Nonfindings and preconditions checked

- `has` uses the same tuple identity as builder insertion. Constant keys include kind and self-delimiting text length; `same` compares the same kind/text pair. String/number separation and embedded NULs do not produce inconsistent scan/index matches. `TestRed14CandidateParity` confirms indexed/scanned candidates and insertion order for both argument positions, typed values, embedded NULs and misses.
- `candidates(pos,c)` assumes `0 <= pos < arity` and consistent tuple arity. Invalid positions can panic in scanning or silently miss in indexed relations, but these are unexported helpers; Compile checks atom arity and Base checks fact arity. No reachable malformed-position path was found through the inspected callers. Missing index positions are not valid for a sealed indexed relation: seal constructs all positions.
- Candidate selection on only the first bound argument is conservative: later constants, previously bound variables and repeated variables are checked by `bind`. It can select more candidates than a selective-index strategy, but does not omit correct rows under validated inputs.
- `bind` checks repeated variables against their first assignment, records a variable once, and deletes only its own assignments on mismatch. Existing bindings remain intact. Anonymous `_` positions are independent and never added to env. `TestRed14BindRollbackAndWildcard` verifies mismatch rollback, success and caller unbinding with an existing binding.
- `bind` requires tuple/argument arity agreement and a nonnil writable environment when binding new variables. Callers supply environments and validated tuples. The lack of a defensive arity check is not a confirmed public-path defect.
- `constant` returns `(zero,false)` for unbound variables. `value` discards that bool, but compiled rule safety ensures head/guard inputs are bound, and anonymous head/guard variables are refused. No reachable unbound-value error was identified.
- `source` requires exactly one of `val`/`bld`; inspected construction sites obey this. A zero source would panic and a doubly populated source prefers val, but no public-path source-invariant breach was established.
- Tuple and candidate slices returned internally alias immutable storage. Base input tuples are copied; seal transfers builder ownership and zeroes the builder; public Answer tuples are cloned. Inspected callers read these slices and only mutate their own env. No exposed mutation of relation storage was found.
- Complexity: indexed lookup builds one constant key and returns an existing candidate slice; unindexed lookup is O(rows) plus matching-index output; all-unbound matching is O(rows) time and memory; bind is O(arity), plus newly bound variable storage. Large strings additionally affect key construction/equality. These costs are expected apart from the budget limitation above.

## Verification

`go test ./datalog -run '^TestRed14' -count=1` passed. Added only `datalog/red14_relation_test.go` and this review. No additional speculative correctness findings.
