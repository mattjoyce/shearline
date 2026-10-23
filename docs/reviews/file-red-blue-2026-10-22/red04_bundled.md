# Independent review: datalog/bundled.go

Snapshot: `5fe3a78`. Scope: all of `datalog/bundled.go`; adjacent `Program`, `Parse`, `Compile`, and `policy.Load` definitions inspected only to establish seams. No production changes. Bounded reproducer: `datalog/red04_bundled_test.go`; `go test ./datalog -run '^TestRed04BundledBoundaryProbes$' -v` passes and logs each observed acceptance.

## Confirmed findings

### P2 — Verify does not enforce rule-set identity invariants

Location: `datalog/bundled.go:55-70`, specifically line 66; relevant constructor checks: lines 29-34.

Trigger: construct a valid set, change its exported `Name` to an empty string or a name containing slash, semicolon, or newline, change `Version` to -1, and synchronize `Program.Name` with `fmt.Sprintf("%s/%d", Name, Version)`. `Verify()` succeeds because it computes the expected AST name from these unvalidated values. No hash recomputation or rule-content change is needed.

Evidence: the reproducer confirms all four malformed identities pass `Verify`; it also changes `policy.RBAC()` to the name `"injected;\nname"`, version -1, synchronizes `Program.Name`, and confirms `policy.Load` accepts it. `Load` invokes `Verify` and then stores `ID()` without independent identity validation. IDs may therefore contain the very separators and line breaks the constructor excludes to keep receipts unambiguous. Empty names and negative versions likewise pass the consumer verification boundary.

Caller precondition versus guarantee: a caller that exclusively uses unmodified constructor results gets the constructor's ASCII identity checks. The public mutable struct and `Verify` do not enforce that precondition; `policy.Load` accepts supplied structs. This is receipt/identity integrity, not a demonstrated authorization bypass: the policy API explicitly treats sets as trusted policy. Recommend sharing name/version validation between constructor and verifier.

### P3 — Name validation misses whitespace the contract excludes

Location: `datalog/bundled.go:23-30`, especially line 29.

Trigger: invoke `NewRuleSet` with a name containing vertical tab, form feed, NBSP, or Unicode line separator. `strings.ContainsAny(name, " \\t\\r\\n;/")` checks only four whitespace characters.

Evidence: constructor and subsequent `Verify` both accept `"bad\vname"`, `"bad\fname"`, `"bad\u00a0name"`, and `"bad\u2028name"`. This contradicts the documented whitespace prohibition and permits whitespace-sensitive readers to split a name or display it across lines. No actual downstream parser collision was established; byte-for-byte ID collision is not claimed. Recommend `unicode.IsSpace` plus the explicit slash/semicolon checks if the documented invariant is intended.

### P3 — Successful constructor can return a set that cannot pass Verify

Location: `datalog/bundled.go:35-40` versus lines 56-57.

Trigger: `NewRuleSet("empty", 0, "")` succeeds because `Parse("")` accepts an empty program, but the returned set immediately fails `Verify` with `ErrInvalid` because its text is empty.

Evidence: reproduced directly. Thus the documented fail-instead-of-partial constructor does not guarantee a usable/verifiable result. An empty extension cannot traverse `policy.Load` even though construction succeeded; whitespace-only text is a different, nonempty representation and passes this guard. Recommend rejecting empty text at construction or deliberately allowing its SHA-256 verification; choose one consistent policy.

## Defenses and nonfindings

- `textHash` uses SHA-256 over exact input bytes and lowercase hex with a fixed `sha256:` prefix. Verify uses exact string comparison, so truncated, malformed, uppercase, or incorrect digests cannot pass as the canonical digest for the supplied text. The hash identifies content, not a publisher or approval authority; no authentication guarantee is inferred.
- Verify reparses authoritative text and compares the whole `Program` using `reflect.DeepEqual`. This covers rules, declarations, terms, bodies, rule text, source positions, program name, and set tags; an AST head mutation was explicitly rejected. Malformed user-built AST pointers are compared rather than executed or dereferenced by custom verification code.
- Changing only `Name` without synchronizing `Program.Name` is rejected. Coordinated replacement with another valid name is not cryptographically prevented because identity is outside the text digest; the confirmed finding concerns acceptance of identities forbidden by the constructor, not an assumed signed-identity promise.
- Constructor errors preserve parser classification through `%w`; validation and Verify failures wrap `ErrInvalid`. Verify's parse failure deliberately retains `ErrInvalid` and includes parser diagnostics through `%v` rather than retaining `ErrSyntax` as a second error identity. No contradictory public error contract was found.
- `ID()` is formatting only and cannot reject invalid mutable fields; consumers must establish validity first. It makes no standalone validation promise.
- `RuleSet` contains mutable slices/maps through `Program`; copying the value does not create ownership isolation, and successful Verify does not freeze it. The inspected Compile seam clones programs before compiling. No sequential post-compile mutation defect was established here, and simultaneous caller mutation would need a caller synchronization contract; no race claim is made.
- Parsing/verification do not impose text-size limits and reparse on every Verify. No resource cap is promised and no bounded-input performance failure was established, so this is not reported as a separate defect.

Review complete after concrete boundary and AST-tamper probes; no speculative high-severity finding.
