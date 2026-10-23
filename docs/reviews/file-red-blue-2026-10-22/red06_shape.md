# RED06: datalog/shape.go

Reviewed snapshot 5fe3a78, assigned file only; neighboring code inspected only to establish Compile and evaluation seams. No production edits.

## Confirmed findings

### P1: programmatic constants bypass the valid UTF-8 invariant

Location: `datalog/shape.go:38-47`, `50-63` (no term validation in head, body atoms, or comparisons).

A manually constructed fact rule with head `p` and one term `{Const: String(string([]byte{0xff}))}` supplies an invalid UTF-8 string through the exported AST. Compile accepts it, Evaluate derives it, and Result.Relation serializes it as U+FFFD. A second program with the actual valid U+FFFD constant produces a different engine constant but the identical Answer bytes and hash. This defeats the documented answer identity/hash invariant. The valid UTF-8 requirement and this exact collision rationale are explicit in `term.go`'s String documentation; base facts reject invalid UTF-8 via `checkUTF8`, but derived constants bypass that entry point. The lexer cannot produce this input.

Concrete evidence: `TestRed06ShapeInvalidUTF8HashCollision` checks different internal constants and equal hashes; both answers serialize as `{"relation":"p","tuples":[["�"]]}`. Caller preconditions: directly construct/edit exported Program and use String with invalid bytes; no private fields or unsafe operations needed. Applies to head constants immediately; body/comparison constants also bypass validation, though their downstream effects were not separately asserted. Severity P1 for answer/receipt integrity; ordinary parsed programs are unaffected.

### P2: zero-argument rule heads bypass positive-arity validation

Location: `datalog/shape.go:38-41`.

`Compile(Schema{}, Program{Rules: []Rule{{Head: Atom{Name: "p"}}}})` succeeds and `Plan.Arity("p")` returns `(0, true)`. Declarations at lines 25-26 and schemas reject arity below one, and the parser's atom loop requires at least one term. However, shape validates only the head name; relations registers the head's length as its arity and subsequent arity checks compare zero against that same zero. This admits a relation explicitly disallowed at the other construction paths and breaks the invariant callers can infer from all normal compilation paths. Confirmed in `TestRed06ShapeAcceptsZeroArity`. Caller precondition: manually built/exported AST, no declaration of the same relation. No crash or further security impact claimed; severity P2 for validation inconsistency.

## Tested nonfindings and scope

`go test ./datalog -run TestRed06 -v` passes all three reproducer tests. The tests deliberately assert current defective acceptance rather than desired rejection. `TestRed06ShapeRejectsMalformedBranches` confirms ErrInvalid (without panic) for empty declaration names, zero/negative declaration arity, empty head names, empty literals (including Negated=true), simultaneous atom/comparison, negated comparison, empty body atom names, and unknown comparison operators. A zero Term remains a valid empty string constant as documented.

Branch inspection: empty/nil program collections and nil rule/declaration collections are safe; program-name fallback affects diagnostics only; sorted declarations give deterministic first rejection. Exclusive atom/comparison validation protects downstream guard dereferences. Clone before shape tolerates nil pointer members through its copy helpers. Names and variable spellings are not lexically validated; arbitrary nonempty names are treated consistently as opaque identifiers downstream, so no independent material defect is claimed for that omission. Const fields are private and number constructors reject nonfinite numbers; no unsupported private-state forgery was used.

Artifacts: `datalog/red06_shape_test.go`. No other review artifacts or review history consulted.
