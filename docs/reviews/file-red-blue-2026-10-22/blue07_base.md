# Blue 07: datalog/base.go

Scope: independent review of base.go against public DESIGN and the Schema/Const contracts. No production edits. Existing unrelated review artifacts and adversarial tests were not opened. A broad symbol search inadvertently returned snippets from other review/test files; findings below were independently derived from base.go and demonstrated with a new test.

## Confirmed gap: callback can replace validated input before ownership transfer

`group` validates UTF-8, calls the user check, and only then clones `f.Tuple` (lines 75–81). Although `check` allocates detached Atom arguments, a callback can independently capture the caller's original tuple. In a synchronous callback, replace an element with `String("\xff")` and return nil. The cloned tuple then contains invalid UTF-8 even though the documented ingress validation succeeded. No race, unsafe operation, or access to private state is necessary. The same applies to `With`. A callback can alternatively replace an accepted policy value with a forbidden, valid UTF-8 value: the check examined a different tuple from the one retained.

Evidence: `go test ./datalog -run '^TestBlue07' -count=1` fails with `base retained callback replacement instead of the validated input snapshot`. New external-package test: `datalog/blue07_base_test.go`. It permits rejection or preservation of the original validated tuple; it rejects acceptance of the substituted tuple.

Suggested repair: clone each fact tuple before its UTF-8 validation and callback, then retain that same owned snapshot. Because callbacks can also capture the variadic facts slice and mutate later facts, consider snapshotting the complete batch before invoking any callback if batch-at-entry semantics are desired. The latter is a contract clarification; the single-fact bypass is already concrete.

## Existing safeguards

- NewBase validates schema before construction and clones both maps. Subsequent caller edits to schema maps do not change the base's schema or selected check functions.
- Unknown relation, incorrect arity, and invalid UTF-8 fail before callbacks. Check errors wrap both ErrInvalid and the original error using `%w`.
- Check Atom.Args is allocated separately, containing value constants. Mutating or retaining those arguments cannot directly modify the tuple later stored by the base.
- Input tuples are cloned before storage in the normal callback-free case. Relations are rebuilt only after all added facts pass validation, so a later rejected fact does not partly publish earlier additions or mutate the standing base.
- Untouched relations are intentionally shared; touched relations are rebuilt and sealed. Empty declared relations are constructed. Deduplication is delegated to the relation builder.
- A zero Base tolerates Len and With() and rejects any provided unknown relation.

## Boundaries and useful regressions

Callback side effects cannot be rolled back, and a callback panic currently propagates. Neither panic recovery nor purity is promised, so these are boundaries rather than independent defects. Copying a function does not freeze its closure: changing externally captured state can change future With validation. Concurrent With calls invoke the same closure; callback synchronization remains the caller's responsibility. The documented concurrent Evaluate guarantee does not imply concurrent arbitrary callbacks are safe.

Useful additional regression cases: callback mutates/retains Atom.Args (must not alter facts); original schema maps mutated after NewBase (must not affect With); a valid addition followed by check rejection (standing base and earlier derived base remain unchanged); With tuple reuse after return; and check rejection preserves both error identities. Existing ordinary tests already cover several nearby cases, so prioritize callback ownership and explicit failed-batch preservation.
