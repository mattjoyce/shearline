# RED16: datalog/plan.go

Reviewed snapshot 5fe3a78, only `datalog/plan.go`; inspected neighboring AST copy, shape, stratification, base construction, evaluation and result APIs solely to establish seams. No production edits. Targeted evidence: `go test ./datalog -run '^TestRed16' -count=1` passes. These tests demonstrate current defective behavior rather than assert a fix.

## Confirmed finding: P2 — schema callbacks receive writable aliases into the compiled plan

Location: `datalog/plan.go:250-251` (also ownership promise at 44-46).

`checkAtom` passes an Atom by value, but its Args slice still points at the owned AST's backing array. A user callback can retain that slice while performing validation or collecting diagnostics. Mutating the retained slice after Compile returns changes the plan's subsequent answers. `stratify` keeps the same rule atoms, so it does not remove this alias. There is no documented callback requirement prohibiting retention or mutation.

Concrete trigger in `TestRed16CheckRetainsPlanArguments`: compile `out("yes") :- node("allowed").` with a node check retaining `a.Args`; evaluate against only `node("other")` and get zero out tuples. Set `retained[0]` to the constant `"other"`; reevaluate the identical Plan and Base and get one tuple. Compile succeeded and neither the original Program nor Base was changed. Thus the externally held callback value defeats Plan ownership/immutability and deterministic reuse; concurrent mutation would additionally race (not race-tested here).

Immediate mutation during the callback can also happen after `checkRule` computes the bound-variable set and after arity checking, so callback effects are not necessarily held to the same validation path. Only the retained-slice answer change was dynamically tested. Give callbacks a deep copied Atom (Args copied), rather than the plan-owned atom.

## Confirmed contract gap: P2 — schema compatibility ignores relation checks

Location: `datalog/plan.go:79-83,273-285`.

Plan retains only base names/arities. A Base constructed with identical arities and different or absent Checks is accepted, even when the plan's schema would reject all the supplied facts. This contradicts the public Evaluate statement that a base built under another schema is ErrInvalid if Checks are part of schema identity, as their inclusion in exported Schema suggests.

Concrete evidence in `TestRed16ForeignChecksAccepted`: compile `out(X) :- node(X).` using a node check accepting variables but rejecting every constant; create Base using `node/1` with no check and one constant fact. Evaluate succeeds and derives that fact. Building the same fact under the compile schema would fail. The unchecked Base can therefore supply values outside the plan schema's domain assumptions.

Precondition/interpretation: caller supplies a Base from a different schema; this is not an invalid tuple injected into a correctly built Base. If compatibility intentionally means arities alone and the caller must enforce validator compatibility, classify this as documentation/API-contract clarification rather than a runtime defect. Functions cannot simply be compared for semantic equality; use explicit schema identity, suitable fact revalidation, or clearly document the narrower guarantee.

## Nonfindings and limits

- Program cloning is deep for rule/head/body argument slices and comparisons; ordinary caller edits to Program do not alias the plan. The callback is a separate escape path.
- Shape checks run before rule traversal and reject missing/both literal alternatives and negated comparisons; no reachable nil dereference was found for those inputs.
- Built-ins do not bind variables; ordinary positive atoms bind head/negation/comparison variables; `_` restrictions are explicit and consistent across these paths.
- Checks run only for schema/base relations by design; derived relations cannot be assigned Checks because checkSchema rejects unknown check names.
- Missing base relations, wrong base arities, and a Base holding any derived relation are explicitly refused. Extra unrelated base relations are tolerated; no standalone bug is asserted because the accepts comment describes the narrower safety cases.
- Empty schema relation names are not refused by checkSchema although empty AST names are refused by shape validation. This leaves an unusable schema entry but no independently demonstrated material consequence, so not promoted to a finding.
- No attempt to enforce purity of caller functions, recover arbitrary callback panics, or permit concurrent caller mutation of input maps is assumed. No whole-suite test run was performed, as other review tests may intentionally fail.
