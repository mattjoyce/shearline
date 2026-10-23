# RED 12 — datalog/term.go

Reviewed snapshot 5fe3a78; scope: all of term.go, with base UTF-8 checks and term tests consulted only to establish contracts. No production edits.

## Confirmed findings

### Low: direct UnmarshalJSON silently discards trailing input

Lines 98–104 decode exactly one value and never check EOF. Calling `c.UnmarshalJSON([]byte("1 2"))` succeeds and sets number 1; `true garbage` succeeds with true; `01` succeeds with number 0. A public JSON decoding entry point consequently reports successful decoding for invalid JSON and can silently change the supplied numeric value. This is directly reachable by callers of the exported method (existing tests themselves use this calling convention). `encoding/json.Unmarshal` validates its complete input first and rejects all three cases, so this finding does **not** establish malformed network answers passing a normal outer JSON decoder. Fix would require checking that a second decode reaches EOF before assigning the receiver. Confirmed by TestRed12TermProbes.

### Low: Number accepts lexemes outside its stated JSON grammar

Lines 43–47 promise a JSON lexeme but use Go's broader ParseFloat grammar. `Number("+1")`, `Number("01")`, `Number("0x1p0")`, `Number("1_000")`, `Number(".5")`, and `Number("1.")` all succeed; every input is rejected by `json.Valid`. Values are subsequently canonicalized to valid JSON, so this is an input contract/validation gap, not an invalid-output or equality bug. In particular, direct JSON decoding of `01` produces 0 while Number("01") produces 1. Confirmed by TestRed12TermProbes. A specification decision to permit Go float syntax would remove the constructor finding but should change the JSON-lexeme documentation.

## Documented choices and nonfindings

- Float64 precision loss, including integers above 2^53 and decimal rounding/underflow, is an explicit domain choice (lines 24–26, 57–59). No arbitrary-precision requirement inferred. Finite float64 maximum and smallest subnormal round-trip through JSON with unchanged identity/key/order in probes.
- All accessible numeric construction paths canonicalize through number; signed zero becomes positive zero. NaN/infinities and overflowing text are rejected. For reachable finite numbers, comparison equality, canonical text, same, and keys agree.
- Kind discriminators plus decimal byte lengths and colons keep keys unambiguous, including strings containing NUL/colon/digits and tuple boundaries. keyOverhead is explicitly only an allocation hint; long strings exceeding three length digits do not affect correctness.
- Const's zero value is a valid empty string. Private fields prevent ordinary external construction of malformed number/bool kinds. Nil receiver decoding of a valid scalar would panic, consistent with an invalid receiver rather than a confirmed supported-use bug.
- String intentionally accepts invalid UTF-8 and explicitly delegates validation to Base entry points; MarshalJSON's replacement behavior is therefore documented and not reported separately. Go's JSON decoder replaces invalid UTF-8 and lone surrogates with U+FFFD, yielding a valid domain string; no strict raw-JSON Unicode rejection promise found here.
- Decoder errors preserve an existing receiver: null, array, object, number overflow, incomplete exponent, and unterminated string were probed. Valid values replace the receiver completely; no stale numeric-field state survives constructors.
- Mixed kinds are intentionally unordered by compare and ordered by kind by less; bool text orders false before true. tupleLess handles empty tuples and unequal lengths lexicographically.

## Verification

Added only `datalog/red12_term_test.go`, with bounded focused reproduction/round-trip probes. `go test ./datalog -run TestRed12TermProbes -v` passed. An initial probe incorrectly expected incomplete exponent `1e` to be accepted; observed unexpected EOF, corrected the probe, and included it in the receiver-preservation cases. No speculative findings promoted to defects; no wider suite run for this read-only review.
