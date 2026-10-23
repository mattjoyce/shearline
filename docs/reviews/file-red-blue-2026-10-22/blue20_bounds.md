# Blue 20: datalog/bounds.go

Scope: independent inspection of `datalog/bounds.go` at snapshot `5fe3a78`, using its comments and `docs/DESIGN.md` as contract context. No other production file or review was inspected. No production edits.

## Defensible guarantees

- `*TooLarge` implements `error`; both byte and step branches start with `datalog:` and include the applicable option name, relation, and corrective suggestion.
- Every non-nil `*TooLarge`, including the zero value and inconsistent public literals, matches the exported `ErrTooLarge` sentinel through `errors.Is`. Wrapping with `%w` preserves this classification.
- Standard `errors.As` extracts the original `*TooLarge`, including all seven exported fields, without needing an `As` method. The pointer receiver is conventional for a structured error; a `TooLarge` value itself does not implement `error`.
- `errors.Is(e, e)` preserves pointer identity despite `e.Is(e)` being false: the standard helper checks identity before invoking custom `Is`. A separately created same-text sentinel does not match. This is normal Go sentinel behavior.
- Error rendering is deterministic for a fixed value. No reliance on formatting text is needed for classification or typed details.

## Actual limits and severity

1. **Unknown Bound renders as MaxBytes (low, defensive API concern).** Only exact `BoundSteps` selects the steps branch; empty, misspelled, or future bound names all render a byte refusal. Exported fields permit inconsistent or negative counts and limits, and there is no constructor or validation seam in this file. Thus `TooLarge{Bound: "unknown", Steps: 10, MaxSteps: 1}` classifies as too large but renders a zero-byte refusal. This is a real observable behavior, but not evidence that an evaluator-produced refusal violates the documented contract: production construction sites were outside scope. A zero-value-usefulness guarantee is not documented. An explicit switch with an unknown-bound fallback would improve diagnostics if arbitrary caller construction is supported.

2. **Nil receiver Error panics (informational, Go convention).** A typed nil `*TooLarge` stored in an `error` interface is non-nil; calling its `Error()` dereferences it and panics. Its `Is(ErrTooLarge)` succeeds because that method does not inspect the receiver. Go does not require error methods to be nil-safe. This is not a contract violation or an independent engine defect without a production path returning a typed nil.

3. **Truth of refusal fields is a producer obligation.** The type cannot establish that a bound was crossed, that Relation identifies the active relation, or that the counts cover all relations. Bytes is exposed for typed inspection but deliberately absent from the byte message, which instead states that the configured limit was exceeded. The documented approximate byte accounting also means the message should not be read as a guarantee of exact JSON size. Establishing construction-site correctness requires a separate evaluator review; bounds.go alone neither proves nor disproves it.

No high- or medium-severity defect is established in this file. Do not promote malformed caller literals, direct `Is` identity expectations, or typed-nil behavior into engine findings without a supported construction path or stronger contract.

## Verification

Added `datalog/blue20_bounds_test.go`. `go test datalog/bounds.go datalog/blue20_bounds_test.go` passed. Tests cover zero/malformed values, both formatting branches, sentinel and identity behavior, wrapped Is/As, and nil-receiver behavior. The file-list invocation isolates this review from unrelated concurrent test files.
