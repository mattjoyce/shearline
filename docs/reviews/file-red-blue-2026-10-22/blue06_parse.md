# Blue team: datalog/parse.go

Snapshot: 5fe3a789d230a28d59b16a03f3c6ac7753ac1a8c. Independent review of parse.go; no red reports/tests or review contents read. Consulted SPEC.md, docs/DESIGN.md, parser/source-text/clone tests and immediate lexer/constant/copy/compiler dependencies to establish contracts. No production changes.

## Assumptions and contract

DESIGN wins over SPEC. Parse accepts syntax and Compile owns semantic checks, including one relation name having one arity, safety and stratification. Comparisons are between Terms, which include boolean constants. Program is a mutable caller-owned AST; Compile must establish its own ownership. Evaluation bounds are expressly evaluation bounds, not a promise of bounded policy loading. Duplicate rules/facts are not inherently invalid Datalog.

## Confirmed gaps

### P2: Conflicting declarations disappear before semantic validation

At parse.go:155 a later declaration overwrites the earlier entry without checking its arity. Reproduced:

```go
p, err := datalog.Parse("hook/1.\nhook/2.") // nil error; Declared["hook"] == 2
plan, err := datalog.Compile(datalog.Schema{}, p) // nil error; hook/2
```

Reversing the statements successfully compiles hook/1 instead. The compiler explicitly enforces “One name, one arity,” but cannot validate syntax already discarded by the parser. This is an actual contract violation rather than a request to reject harmless repeated identical declarations. A mistaken or concatenated policy can silently change an open relation's shape.

Protection: compare a new declaration with the existing map entry before assignment and reject conflicting arities with the relation and current source line. Keep same-arity repeats idempotent. Alternatively retain all declarations for Compile, but that requires a larger AST change. Document which sentinel owns conflicting declarations; syntax rejection is the smallest compatible approach.

Regression cases: conflicting declarations in both orders; conflicts separated by rules/comments; three declarations where the last restores the first arity; identical repeats accepted; declarations split across Programs still rejected by Compile when inconsistent.

### P2: Boolean constants cannot start a comparison

parse.go:195 routes every tokName into atom(), before term() can recognize true/false. These documented Term values work on the right but not the left:

```text
p(1) :- true = true.      -> expected "(", got "="
p(1) :- false != true.    -> expected "(", got "!="
p(1) :- 1 = 1.            -> accepted
p(1) :- "x" = "x".        -> accepted
p(X) :- q(X), X = true.   -> accepted
```

Comparison's public comment says it is a guard between two terms; Term expressly includes constants and Parse lists true/false as constants. No documented restriction excludes boolean left operands. The grammar currently makes equality asymmetric at parse time.

Protection: distinguish a boolean term from a relation call by lookahead (e.g. true/false followed by a comparison operator), retaining existing relation names true(...) and false(...) if they remain legal. Alternatively explicitly reserve boolean names, but that is a language change and should be documented.

Regression cases: both booleans as left operands for all comparison operators; true/false atoms; true/false in argument positions; incomplete boolean comparisons report ErrSyntax rather than panic. Ordered boolean comparisons already have a defined constant ordering, so should not be rejected merely by the parser.

## Existing defenses worth retaining

- Lexer appends EOF. statement's i+1 lookahead is safe because it executes only at a non-EOF name; take never advances beyond EOF. atom/body loops consume tokens or return an error, with no recursive descent over unbounded nesting. Trailing commas, missing operands/periods, empty atoms and empty bodies fail instead of producing partial valid ASTs.
- Parse returns Program{} on any failure. Declaration-map mutation before a missing period is therefore not observable by callers.
- Rule.Text slices from the first token byte offset through the confirmed final period; comments within a statement survive, comments outside it do not. Head/rule/comparison/atom lines preserve their starting token positions. Existing TestParse_RuleText covers multiline comments and adjacent statements; a manual multiline probe agreed.
- Finite numeric validation delegates to Number and retains errors.Is(ErrSyntax). Arity uses Atoi plus positive check, rejecting fractional/exponent/negative/zero/out-of-range arities instead of coercing them.
- Program.Clone duplicates declaration map, rules, argument/body slices, literal atoms and comparisons, while strings and Const values are safe immutable values. Existing TestProgram_Clone exercises mutation through each of these paths and preserves nil state. No present AST ownership defect found in this file.
- Parse does not prematurely reject unsafe facts/rules or unknown relations: those belong to Compile. Parser restriction alone must not substitute for validation of manually constructed Programs.

## Language choices and lower-priority hardening

- `not` is contextual syntax in a body: `not(1).` is a legal fact, but `p(X) :- not(X).` fails because not introduces negation. This is not an authorization bypass. Document reserved/contextual names or explicitly decide a lookahead grammar before changing it. `not not(X)` can express negation of a relation named not; positive use currently cannot.
- Zero-arity atoms/declarations are intentionally unsupported (existing parser tests require rejection). Nested terms, bare string identifiers, chained comparisons and extra punctuation need no permissive recovery.
- Duplicate facts/rules should stay legal absent an explicit uniqueness contract. Retaining rule order is useful because first derivation selects proof provenance. Do not deduplicate source statements as a parser security fix.
- All tokens are materialized before parsing (parse.go:83), followed by AST allocation. Large hostile policy text can allocate many times its byte length; Parse has no context or input/token/rule/arity cap. This is a boundary concern, not a breach of DESIGN's Evaluate MaxBytes/MaxSteps guarantee. Consumers loading untrusted text should enforce a source-byte limit before Parse; if hostile policy loading becomes supported, specify independent load limits and test them. Streaming tokens could reduce peak memory but is not required to fix either confirmed gap.
- Program.Text intentionally retains the source. Substrings in rule names/text/variables can retain the whole source even if a caller later clears Program.Text and keeps one Rule. Clone duplicates mutable AST storage, not immutable string backing. Its “shares nothing” wording should be understood as mutation isolation; cloning every string would be a new memory-retention guarantee, not a current correctness requirement.
- Invalid arity diagnostics print the failed conversion's integer result rather than the original lexeme: `a/1.0.` reports `a/0`, and overflow can report a saturated machine integer. Retain the original token for accurate diagnostics. Numeric term failures also begin `line N: datalog: syntax:` rather than the other parser paths' `datalog: syntax: line N:`; sentinel classification remains correct. Normalize only if uniform prefix formatting is intended as an API guarantee.
- Declaration positions are discarded by the public map representation; conflicts with schema or another Program can later report line 0. Carry declaration metadata if precise declaration diagnostics become part of the source-position contract. Current proof requirements concern Rule.Line/Text, which are retained.

## Verification and suggested regression strategy

Ran `go test ./datalog -run '^TestParse' -count=1` successfully. Ran separate temporary Go probes for conflicting declarations, boolean comparisons, invalid arity, overflowing number and multiline source attribution. No production edits or permanent tests added.

Add a Parse fuzz invariant for arbitrary bytes: never panic; either return ErrSyntax and a zero Program, or a Program whose rules have nonempty Text ending in a period and positive source lines. Seed EOF after every grammar component, very long flat bodies/argument lists, comments containing punctuation, multibyte UTF-8 literals, escaped quotes and malformed numbers. Keep fuzz resource claims separate from termination: finite input is scanned without parser recursion, but finite does not mean cheap. Existing clone/source-text tests already cover important ownership and provenance contracts; extend only for newly found cases.
