# Independent review: datalog/lex.go

Scope: assigned file only, at 5fe3a78. Parser and constant conversion inspected only to establish reachable behavior. Production code unchanged.

## Confirmed finding

### LOW — claimed Go/JSON string escapes do not include valid JSON escapes

- Location: `datalog/lex.go:141-156`, especially `strconv.Unquote(raw)` at line 154.
- Trigger: parse the program `a("\/").` or a literal containing the JSON surrogate pair `\uD83D\uDE00`.
- Evidence: both return `ErrSyntax: ... bad string ...`, confirmed by `TestRed01JSONEscapeCompatibility` in `datalog/red01_lex_test.go`. Both are valid JSON spellings of valid UTF-8 string values (`/` and `😀`). Go's unquoter rejects the slash escape and individual surrogate escapes.
- Impact: callers generating literals using a JSON encoder can encounter rejection, despite the local "Go/JSON escapes" contract. A caller using literal Unicode or Go-style `\U0001F600` succeeds. This is a compatibility/documentation issue, not an invalid-UTF-8 bypass or privilege escalation.
- Suggested resolution: define the language explicitly as Go quoted strings and correct the comment, or implement the promised JSON escape support. The intended external language contract is not established by this file, so severity is low.

## Tested nonfindings and defenses

- `TestRed01LexerBoundaries` covers lone minus, incomplete exponent/sign, exponent overflow, trailing backslash, raw newline, invalid raw/escaped UTF-8, non-ASCII name, embedded NUL, empty input, EOF comment, CRLF, quoted backslash/quote, Unicode strings, and signed decimal exponent. Errors remain `ErrSyntax`; no panic observed.
- `number` always advances when reached from `next`, including malformed `-`; incomplete numeric tokens are rejected by the parser's `Number` conversion. The scanner's acceptance of `01` and `-.5` was confirmed, but is not reported separately: only the local "JSON number" comment promises strict JSON syntax; no unsafe result or explicit public strict-number grammar was established.
- The string scanner may advance one byte beyond EOF for a final backslash (line 149), but exits the loop and reports unterminated string without slicing past the source.
- `utf8.ValidString` checks both raw spelling and decoded string (lines 160-161), preventing raw invalid bytes and byte escapes producing invalid UTF-8.
- Ordered punctuation recognizes multi-character operators before prefixes. Statement periods remain outside numbers unless followed by a digit. Whitespace/comments advance monotonically; percent signs inside strings do not start comments.
- `all` snapshots the position after whitespace and sets every token's byte position, including EOF. Duplicate whitespace skipping in `next` is harmless. Token substrings share immutable Go string storage; no caller-mutable byte alias was found.
- No user-controlled lexer state is exported. Invalid negative positions or calling `number` at EOF would require violating private-method preconditions and were not treated as reachable defects.

Validation: `go test ./datalog -run 'TestRed01' -count=1` passed.
