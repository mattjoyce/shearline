# Blue 11: datalog/bundled.go

Independent review of snapshot 5fe3a78. Production scope: only datalog/bundled.go. Contract consulted: SPEC.md and docs/DESIGN.md. No review/ or red-team materials read. No production edits.

## Protected boundaries

- NewRuleSet refuses empty names, common ASCII separators, slash, semicolon and negative versions; parse errors return a zero RuleSet and preserve their error chain.
- Hash is exactly lowercase sha256 over original text bytes, including formatting/comments. It is intentionally a text hash, not a semantic equivalence hash or signature of author identity.
- Verify recomputes the exact hash, reparses text, and compares the complete Program with reflect.DeepEqual after setting its expected name. Hash alteration, text alteration without corresponding hash, AST alteration without corresponding text, and unsynchronized name/version changes are refused.
- Verify wraps every failure in ErrInvalid, including a parser failure whose hash was updated to match the invalid text. Empty Program.Text is explicitly refused.
- A coordinated change to valid name/version plus Program.Name is not itself an integrity defect: the contract hashes text only, so this API cannot authenticate publisher identity or preserve historical version provenance.

## Remaining defense gaps

### P2: Verify does not enforce the identity constraints of NewRuleSet (lines 55–70)

RuleSet has exported fields and Verify is explicitly the boundary for hand-built or edited sets. Start with NewRuleSet("valid", 1, "hook/1."), then set Name to "bad/name" (or "", or "bad;name"), Version to -1, and Program.Name to fmt.Sprintf("%s/%d", Name, Version). Verify returns nil. Its reconstructed name comparison confirms internal consistency but never validates name/version. This admits the exact ambiguous receipt/proof labels and negative versions the constructor contract prohibits. Reuse one identity validator from both construction and verification. ID itself may remain a formatter if verification is the loading boundary.

### P2: Constructor's whitespace filter is incomplete (line 29)

The documented contract says names are free of whitespace. strings.ContainsAny only excludes space, tab, CR and LF. Names containing vertical tab, form feed, NBSP (U+00A0), or Unicode line separator (U+2028) all construct and verify successfully. Such names can split visually or under ordinary whitespace readers, violating the stated readable identity constraint. Use unicode.IsSpace (and define any separate desired control-character policy explicitly).

### P3: Empty text succeeds at construction but immediately fails verification (lines 35–40 versus 56–58)

NewRuleSet("empty", 0, "") returns success and a valid SHA-256 of the empty string; its Verify immediately returns ErrInvalid. This contradicts the constructor's promise of a complete usable set and produces a late loading error. Align the two methods: under the current explicit Verify contract, reject empty text at construction. This is specifically zero-byte text; a parsed comment-only or whitespace-only program is not necessarily invalid under the documented empty-relation semantics.

## Validation

Added datalog/blue11_bundled_test.go with a passing observational test reproducing all three gaps. Command: `go test ./datalog -run TestBlue11BundledBoundaryObservations -count=1` → PASS. The tests deliberately record current behavior; they must be inverted when defenses are implemented. No claim is made about other production files or downstream loading implementation.
