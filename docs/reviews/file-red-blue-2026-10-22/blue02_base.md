# Independent defensive review: datalog/base.go

Reviewed snapshot 5fe3a78, scoped to base.go; consulted the public contract, term constructors, schema validator and existing base tests. No production edits.

## Confirmed gap: copy occurs after callback (medium)

`group` checks caller-owned `f.Tuple` for UTF-8, builds a separate Atom for the callback, calls arbitrary schema code, then copies the original tuple. A callback holding the caller's tuple can synchronously change it between validation and ownership transfer. The committed tuple then differs from the tuple approved by both UTF-8 validation and the schema callback. No race, unsafe code or private access is needed.

`datalog/blue02_base_test.go:TestBlue02BaseCallbackChangesValidatedInput` demonstrates this: the check sees `safe`, changes the captured input tuple to byte FF, returns nil, and NewBase successfully stores the invalid string. Command `go test ./datalog -run '^TestBlue02BaseCallbackChangesValidatedInput$' -count=1` fails with `base stored "\xff" after validating safe`. This violates the explicit string-domain promise in term.go/DESIGN.md and the checked-fact promise in base.go. With has the same path.

Recommended ownership order: copy the tuple before validating or invoking user code, then validate, check and store that owned copy. This protects the current fact. If the intended construction semantics are a snapshot of the entire fact batch, copy all Fact records and tuple backing arrays before the first callback as well: a callback can otherwise mutate subsequent elements of the variadic input slice, including Relation and Tuple. Full-batch snapshot semantics are currently unspecified, so that second point is a contract decision rather than a proven violation.

Regression: run the reproducer for both NewBase and With; require the stored tuple to remain `safe` even when the callback changes the original tuple to invalid UTF-8 or a schema-forbidden value. Include a callback that mutates its Atom.Args and retains it for mutation after return; this must have no effect on stored facts (already defended).

## Existing defenses

- Schema validation runs before construction: built-in names, nonpositive arity and orphan checks are refused. Both schema maps are copied, so later ordinary map edits cannot change a constructed Base's relation shape or callback selection.
- Each fact must name a declared relation, have exact arity and contain valid UTF-8 strings. Const fields are private and constructors restrict numeric inputs; base.go need not duplicate their finite-number validation for ordinary external callers.
- Callback arguments are fresh Term slices containing value constants, so changing or retaining Atom.Args cannot alias stored tuples. Callback errors preserve both ErrInvalid and the caller's sentinel via wrapping.
- Input tuples are copied before storage, preventing mutation after a successful constructor returns. This defense needs the ordering fix above for callback reentrancy.
- Failed grouping returns before any standing relation changes. With builds a new relation map, shares sealed untouched relations and rebuilds touched ones; Base.Len only reads. Nil maps/zero Base do not panic: Len is zero, empty With succeeds, nonempty With rejects undeclared facts.
- Duplicate tuples collapse in the relation builder. Every declared relation is created even with no facts. Existing tests cover ordinary tuple copying, duplicates, bad inputs, atomic extension, concurrent evaluation/extension and randomized With/union equivalence.

## Callback and concurrency contract gaps

Copying a function value does not freeze its closure. A callback can retain mutable external configuration, change its decision after construction, panic, block indefinitely, or race when With calls it concurrently. NewBase and With provide no recovery, deadline or serialization. No contract promises recovery from arbitrary user callbacks, so these are integration constraints/documentation omissions, not independently confirmed engine defects. The claimed safety of concurrent Evaluate remains valid with respect to callbacks because Evaluate does not invoke these retained checks. Concurrent With requires concurrency-safe checks, which deserves an explicit comment.

Changing closure state also makes the documented With/NewBase-over-union equivalence conditional: With does not recheck prior tuples; rebuilding the entire union does. Specify pure/stable checks for that postcondition rather than silently implying callback state is frozen. Test a stable callback under concurrent With using -race; document that concurrent caller mutation of schema maps or fact slices during constructor calls is outside the ordinary Go ownership contract.

## Resource and naming boundaries

Construction has no context, fact/byte/arity bound or allocation budget. It copies all fact occurrences before deduplicating, and With rebuilds an entire touched relation even when the added fact is already present. Very wide relations create indexes at every position. Evaluation Options explicitly bound derived data/work; they do not claim to bound construction. Thus unbounded construction is an API choice, with availability risk if exposed directly to untrusted bulk input. A meaningful resource regression is a benchmark for duplicate-heavy construction and repeated duplicate-only With on a large relation; require evidence before choosing quotas or optimization.

Schema names are not checked against parser identifier syntax or UTF-8; empty/invalid names can exist in a Base. The documented schema validator only promises built-in/arity/orphan-check checks, and public ASTs can name such relations, so this is a naming-contract question, not a demonstrated base.go bug. Add a documented-name-policy test if that contract is tightened.

## Additional precise tests worth adding

1. Mutate and delete original schema map entries after NewBase; With must continue using original arities and check function selection.
2. Reject a batch containing an accepted fact followed by invalid UTF-8 or callback failure; standing Base contents and answer hashes must remain unchanged.
3. Mutate tuples passed to With after return; both standing and extended answers must remain stable, including a relation shared through a later empty With.
4. Assert errors.Is finds both ErrInvalid and a callback sentinel for NewBase and With, and cover nil Checks/map entries.
5. Explicit zero-Base/empty-schema With behavior and empty declared relations, so future validation changes do not accidentally turn harmless zero values into panics.

No other concrete base.go correctness defect was established in this review. The one added test intentionally fails on the reviewed implementation and captures the confirmed ownership-order defect.
