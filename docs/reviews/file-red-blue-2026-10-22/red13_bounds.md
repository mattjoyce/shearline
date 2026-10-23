# RED TEAM: datalog/bounds.go

Reviewed assigned file at commit `5fe3a78`, with `eval.go` constructors and existing bound tests read only as API seams. No production changes. No confirmed actionable finding.

## Coverage and evidence

- `bounds.go:8–12`: `ErrTooLarge` is a stable sentinel. The exported pointer error's `Is` implementation at line 43 correctly recognizes it, including through standard `%w` wrapping. Lack of `Unwrap` is appropriate: this is a classified error, not a wrapper containing an underlying error.
- `bounds.go:14–18`: both bound labels match the documented option names and the evaluator's emitted values (`eval.go:220`, `eval.go:284`).
- `bounds.go:20–32`: reviewed every exported field. `Relation` identifies the rule head being derived; `Tuples` and `Bytes` are aggregate derived counts; `Steps` is aggregate work. Byte refusals populate `MaxBytes`, step refusals populate `MaxSteps`; the unused limit being zero is consistent with `Bound` being the explicit discriminator. Neither documentation nor construction promises that both configured limits are copied onto every error.
- `bounds.go:34–40`: step messages use `Steps`/`MaxSteps` and byte messages use `Tuples`/`MaxBytes`, with relation and remediation in both. `Bytes` is structured diagnostic data even though the byte message does not print its precise approximate value. Approximation is documented here and in `Options.MaxBytes` and the charging function.
- `bounds.go:43`: ordinary `errors.As(err, &target)` with `var target *TooLarge` works by standard assignability; a custom `As` method is unnecessary. `errors.Is` does not incorrectly equate separate `*TooLarge` instances, and direct identity checks retain Go's normal behavior.

## Malformed and zero values; conventional Go preconditions

- `(&TooLarge{}).Error()` formats a byte refusal with zero counters and an empty relation; it does not panic. Caller-supplied negative counters, contradictory counts, or an unknown `Bound` likewise format without panic. An unknown label takes the byte branch (`bounds.go:35`), but such a value is not emitted by either evaluator constructor. There is no documented public validation or round-trip parsing contract for manually assembled error values, so this is not a confirmed defect.
- Calling `Error()` on a nil `*TooLarge` dereferences the receiver. Nil-receiver safety is not promised and evaluators return a nonnil instance when refusing work; Go error implementations conventionally need not support manually created typed-nil errors. `Is` itself does not dereference the receiver. No finding from this asymmetry.
- Passing a non-pointer/invalid destination to `errors.As` has the usual standard-library preconditions; it is not this type's responsibility to accept those calls.
- Public mutable fields and the exported mutable sentinel follow ordinary Go API conventions. Caller mutation or concurrent unsynchronized mutation is not an independently supported input contract.

## Verification

`go test ./datalog -run 'TestEvaluate_(MaxBytes|MaxSteps)' -count=1` passed. Existing tests exercise sentinel classification, typed extraction, bound labels, relation and limit fields, diagnostic text, final-tuple byte crossing, and negation-probe step charging.

Severity: none. Confirmed bugs: none. The malformed-value observations above are explicitly nonfindings, not speculative production vulnerabilities.
