# RED07 — datalog/proof.go

Reviewed snapshot 5fe3a78, restricted to this file; neighboring types, validation, derivation recording and existing proof tests read only to establish its seams.

## Confirmed finding

**Medium — ambiguous composite memo key returns a different relation's proof (line 46).** `name + "\x00" + t.key()` is not injective when relation names contain NUL. Public `Schema`, `Program`, `Atom`, and `Fact` constructors accept these names (shape validation only rejects empty names). A proof containing both `a("x\x00", "y")` and relation `"a\x0002:x"` with tuple `("y")` gives both the identical memo key `"a\x0002:x\x0001:y"`. The second body entry is consequently returned as relation `a`, with two columns, rather than its actual relation and one column. This produces false provenance from an otherwise successful evaluation.

Evidence: `datalog/red07_proof_test.go`, `TestRed07ProofMemoRelationCollision`, builds the entire case through exported APIs. `go test ./datalog -run 'TestRed07ProofMemoRelationCollision|TestProof_' -count=1` fails at the explicit assertion: second body is `"a"` with `["x\x00", "y"]`, expected `"a\x0002:x"` with `["y"]`. Use a struct key containing separate relation and tuple-key fields (or an unambiguous length prefix).

Caller preconditions: a program/schema assembled through the exported structs, containing a NUL relation name, plus a proof whose reachable body contains the colliding tuples. Ordinary parsed identifiers cannot supply this trigger; no unsafe access or concurrent mutation is needed. This is a confirmed correctness issue, not a demonstrated confidentiality or code-execution issue.

## Bounded probes and nonfindings

- `go test ./datalog -run '^TestProof_' -count=1` passes. Probes completed; no production edits.
- `Result.Proof` checks proof enablement, relation existence and tuple membership before descending. Zero Result, unknown names, wrong arity and absent tuples return false; attacker-supplied tuple constants do not bypass set membership. The tuple key itself is length-delimited; the defect is the outer relation/tuple composition.
- Tuple slices are cloned. `Const` and `Term` contain scalar/string fields, so no nested mutable storage escapes through them. Rule head arguments, literal slices, atom pointers and comparison pointers are copied; shared rule copies and shared subproof slices within one call are explicitly documented. Separate calls allocate separate copies. No plan/base alias found.
- Derivation records and source rules are private; successful evaluation records only previously available body tuples. Thus nil rule pointers, cycles and missing derived records cannot be manufactured through the public `Proof` parameters. Manually corrupting private Result internals is outside caller preconditions.
- Recursion depth is proportional to derivation depth and has no independent context/size limit. A sufficiently enormous valid proof could exhaust resources, but no bounded practical crash was established; not reported as a confirmed vulnerability. Memoization bounds repeated DAG construction by reachable records and body references rather than unfolded paths.
- Internal `builder.build`, `builder.rule`, `copyLiteral`, and `copyAtom` take no interface values or callbacks. Their pointer/slice assumptions are provided by validated evaluation state. No additional confirmed issue.
