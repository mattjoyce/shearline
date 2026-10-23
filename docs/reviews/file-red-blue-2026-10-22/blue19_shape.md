# Independent review: datalog/shape.go

Snapshot: 5fe3a78. Scope: shape.go; exported AST, parser and Compile contracts consulted. No review documents or red test contents inspected.

## Finding: [P2] Reject zero-argument heads in checkRuleShape

Location: datalog/shape.go:39–43 (the head-name check).

`checkRuleShape` accepts `Rule{Head: Atom{Name: "flag"}}`: there is no minimum head arity check. `Compile` then infers `flag/0` from that head, and its subsequent atom arity validation accepts it because the inferred arity agrees. This lets a manually constructed AST bypass the language's positive-arity invariant: declarations explicitly reject arity below one in this file, schemas do likewise, and the parser cannot construct an empty argument list. A zero-argument body atom referring to this head also passes the later arity check.

Reproduction: `Compile(Schema{}, Program{Rules: []Rule{{Head: Atom{Name: "flag"}}}})` returns no error and `plan.Arity("flag")` returns `(0, true)`. Confirmed by `go test ./datalog -run '^TestBlue19ZeroArity$' -v`; retained probe is datalog/blue19_shape_test.go.

Recommendation: require at least one argument on every head and body atom during shape validation, returning ErrInvalid through the existing wrappers. Add rejection tests for a zero-argument fact and a rule using a zero-argument derived relation.

## Other checks

The literal pointer union checks reject missing/both alternatives before downstream traversal; negated comparisons and unknown comparison operators are covered. The term contract explicitly uses nonempty Var as the discriminant and permits the zero term as an empty string constant, so an otherwise populated Const on a variable is not independently a demonstrated violation. Names and variable strings outside parser syntax (including invalid UTF-8) are accepted, but the consulted exported AST contract does not establish that parser token spelling is mandatory for hand-built programs; no separate actionable finding is asserted for that.
