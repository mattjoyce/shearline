# RED TEAM: datalog/base.go

Reviewed revision `5fe3a78`; scope is `datalog/base.go`. No confirmed actionable defect found under ordinary public API use. No production changes or additional tests.

## Confirmed defenses / nonfindings

- Lines 28–40: schema validation precedes construction; arities and check maps are copied. Later edits to the caller's schema maps cannot alter the base. Built-in names, nonpositive arities, and orphan checks are rejected by the delegated schema validator.
- Lines 65–85: each fact's relation membership and exact tuple length are checked before indexing or callbacks. Nil tuples fail positive arity. Unknown/empty relation names are rejected unless explicitly supplied in the schema; the file does not promise identifier grammar validation.
- Lines 77–83 and 131–137: invalid UTF-8 string constants fail with ErrInvalid before insertion. Const fields are private; normal public constructors provide valid numeric/boolean representations, including a legitimate zero-value empty string.
- Lines 83 and 119–123: tuple storage and callback arguments have independent slices. Mutating input tuples after return, or retaining/mutating the Atom.Args given to a check, cannot change stored facts. Const values contain scalar fields only.
- Lines 47–61: validation completes before any relation rebuilding; errors leave the standing base unchanged. Untouched relations are shared safely; touched relations rebuild private set/index structures. The builder deduplicates exact tuple identities and never mutates shared prior tuple slices.
- Lines 104–108: Len handles nil maps, unknown names, and zero Base without indexing or panic. Zero Base.With() also returns an empty base; adding any fact to a zero base produces ErrInvalid.
- Lines 115–125: absent/nil callbacks are accepted as documented. Callback failures wrap both ErrInvalid and the original error, preserving errors.Is matching. Callback panics are not recovered; no panic-containment contract is advertised.

## Hardening observations, not findings

- Lines 77–83: a schema callback can close over the caller's original tuple and mutate it after UTF-8 validation but before copying. A deliberately reentrant callback could consequently insert an invalid UTF-8 value. This requires the trusted validator to mutate a separate alias, rather than its supplied Atom; it is not an untrusted-fact validation bypass under normal callback use. Copying before all validation would harden this unusual ownership seam. Not reproduced and not promoted to a defect.
- Lines 59/99: every schema position receives an index, even for empty relations. Very large positive schema arities therefore consume memory/time proportional to arity, while repeated small With calls rebuild the complete touched relation. These costs follow the documented indexing/rebuild design, and no input or memory limit is promised. External applications accepting untrusted schemas would need their own bounds.
- Shared function values can close over mutable state; copying a function value does not snapshot its closure. Concurrent With calls require user callbacks to be concurrency-safe. The documented concurrency promise concerns Evaluate, which does not invoke these callbacks.

## Verification

Ran `go test ./datalog`: PASS (`ok`, 0.386s). Existing tests cover tuple ownership, validation, extension, equivalence to union, and concurrent evaluation.
