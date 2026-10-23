# Independent blue review: datalog/term.go

Scope: snapshot 5fe3a78, term.go and its public contract/tests. No production edits. A standalone probe run with a temporary Go cache confirmed the examples below.

## Findings and defenses

- **Low: direct UnmarshalJSON accepts a valid prefix and ignores trailing input** (term.go:98–120). `c.UnmarshalJSON([]byte("1 true"))`, `true!`, and `"ok" junk` all succeed. The receiver changes even though the supplied bytes are not one JSON value. `json.Unmarshal` validates its entire input before invoking this method, so normal wrapper callers are protected; the exported method itself is not. Require EOF after the first decode, before assigning the receiver. Test trailing scalar, punctuation, whitespace-only suffix, and preservation of the old value on rejection.

- **Low: JSON string decoding silently repairs malformed Unicode** (term.go:103–109). Direct decoding of a quoted 0xff produces U+FFFD; `"\ud800"` does too. Standard encoding/json intentionally behaves this way, and every resulting Const is valid UTF-8, so this does not introduce an invalid engine constant. However, consumers expecting invalid wire identities to be refused can instead get the identity of an existing replacement-character string. Consider documenting JSON's replacement semantics, or reject malformed UTF-8 before decoding and validate escaped surrogate pairs if strict wire identity is required. Tests should contrast malformed bytes, lone surrogates, legitimate U+FFFD, and a valid surrogate pair.

- **Low / contract clarification: Number accepts Go numeric syntax beyond JSON** (term.go:43–55). `0x1p2`, `1_000`, `+1`, and `01` succeed and canonicalize to valid JSON numbers. This contradicts the narrow wording “from a JSON lexeme,” but does not damage identity: all accepted finite values pass through number(). JSON decoding itself uses encoding/json's stricter grammar. Either document the constructor's permissive syntax or validate JSON number grammar before ParseFloat. Add accepted/rejected spelling tables for the intended policy.

## Existing defenses and deliberate choices

- Private Const fields prevent external construction of malformed kinds, inconsistent numeric text/value pairs, or fabricated booleans. The zero value is the valid empty string. A nil *Const receiver is ordinary caller misuse, not an input-driven reachable invalid kind.
- Number and Float refuse NaN/infinities and numeric overflow; shared number() canonicalizes equivalent finite values and negative zero. Underflow such as `1e-9999` becomes zero, consistently with the documented float64 domain. Integer rounding beyond 2^53 is explicitly documented; exact identifiers belong in strings.
- Kind-prefixed, byte-length-prefixed keys distinguish types and tuple boundaries, including NUL and long strings. same(), compare(), and keys agree for values made through supported constructors.
- String intentionally accepts arbitrary bytes and leaves rejection to lexer/Base boundaries under the contract. Direct MarshalJSON of `String("\xff")` replaces the byte with U+FFFD, so standalone constructor/JSON round-tripping is lossy outside the valid domain. Rejecting invalid UTF-8 in MarshalJSON would be a reasonable additional defense, but boundary validation already covers the documented engine ingress.
- UnmarshalJSON assigns only after successful scalar decode/construction; null, containers, malformed numbers, and overflow leave the previous receiver intact. Errors from encoding/json are not ErrSyntax-wrapped; decide/document whether callers should rely on the sentinel for this API.

## Useful regression properties

For finite floats (including subnormals, extrema, and signed zero), Float → JSON → Const should preserve Go equality and canonical text. For valid UTF-8 strings and booleans, the same round-trip should preserve equality. Internal tests can assert `same(a,b)` iff keys match, numeric comparison equality iff same(), and tuple key uniqueness for adversarial boundaries; differing numeric kinds remain unordered by compare as designed.

No high-severity identity/comparison defect found in supported valid constants. All proposed behavior changes above concern tightening/documenting external decoding and constructor contracts.
