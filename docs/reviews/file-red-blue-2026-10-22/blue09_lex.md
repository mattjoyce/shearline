# Blue 09 — datalog/lex.go

Snapshot: 5fe3a78. Scope: lexer only; consulted Parse's syntax comment and numeric-token handoff, existing UTF-8 tests and parser fuzz harness. No production edits.

## Verdict

No demonstrated panic, invalid-string acceptance, indexing failure, or unbounded superlinear scan. The material ambiguity is the comments' JSON terminology versus the actual Go-compatible lexical grammar. Treat that as a contract clarification unless strict JSON compatibility is a requirement.

## Existing defenses

- ASCII-only names and variables avoid classifying UTF-8 lead bytes as Latin-1 letters. Unknown characters are decoded as complete runes for diagnostics. Invalid UTF-8 outside strings produces ErrSyntax; actual U+FFFD remains valid inside strings.
- Strings undergo both strconv.Unquote validation and UTF-8 validation of original and decoded bytes. This catches raw malformed encodings and valid-looking Go byte escapes that produce malformed text. Existing audit tests and fuzz harness cover those invariants.
- Escaped quotes/backslashes are skipped before searching for the closing quote. Trailing backslash advances pos one past EOF, but the loop exits and returns an error without slicing or indexing at that position. No externally successful path uses that overrun.
- Every successful non-EOF token advances pos. All scanner indexes are bounded on their reachable internal call paths. Number and word helpers are unexported and called only after a nonempty matching prefix.
- Minus-only and unfinished exponents may be emitted as number tokens, but Parse passes every numeric term to Number and propagates ErrSyntax. The lexer is intentionally shape-oriented rather than a complete numeric validator.
- Fractional scanning requires a following digit, preserving the terminating period in `a(1).`. Two-character punctuation is checked before its one-character prefixes.
- skipSpace counts LF once, handles CRLF, and leaves the first token after comments with a correct byte offset. Byte offsets intentionally differ from rune columns. Both all and next call skipSpace, but the second call sees no whitespace and does not duplicate line increments.
- Scans are linear in input size, with a fixed small punctuation list. Unquote/UTF-8 checking add constant passes over each string. Token buffering and decoded string storage are O(input size). No lexer-specific input limit is promised.

## Contract edges, not proven security failures

1. lex.go's `number scans a JSON number` is inaccurate: Parse accepts `a(01).` and `a(-.5).`. These normalize to finite numeric constants downstream. Leading plus and leading dot alone are rejected by token dispatch. If strict JSON numeric syntax is intended, enforce it; otherwise change the comment to describe the accepted grammar.
2. `Go/JSON escapes` does not provide all JSON string escapes: `a("\/").` and `a("\uD83D\uDE00").` are rejected by Go's Unquote, although escaped slash and paired UTF-16 surrogates are JSON-compatible. Go escapes such as `\x41` and `\101` are accepted. This is a compatibility/documentation gap, not invalid UTF-8 admission. Choose explicitly whether Go literals or JSON literals are the language.
3. Bare CR does not increment line numbers; only LF does. This matches the scanner's explicit convention and normal CRLF behavior. Supporting historic CR-only source requires a deliberate line-ending contract.
4. Malformed bytes in comments are discarded. The stated invariant protects string values, not the encoding of the entire program text; do not report this as a bypass of the string invariant.

## Verification and useful tests

Added datalog/blue09_lex_test.go. `go test ./datalog -run '^TestBlue09' -count=1` passes. Tests pin truncated escapes/numbers, surrogate rejection, malformed bytes, token offsets/lines/EOF, and current numeric/string grammar differences. Further useful coverage: escape parity before quotes, all supported Go escapes, CRLF comments, complete punctuation pairs, multi-byte error diagnostics, and lexer-only fuzzing asserting progress and monotonically increasing byte positions for every successfully tokenized input. Existing pipeline fuzzing already checks no panic and valid UTF-8 accepted constants.
