# Red/blue file review matrix

Snapshot: `5fe3a789d230a28d59b16a03f3c6ac7753ac1a8c`.

Red and blue reviewers worked independently on one file at a time. `Intersection`
lists concrete findings independently identified in both passes. `—` means no
confirmed file-specific finding in that pass, not that the file has no risks.
Severity and detail live in each linked report.

| File | Red team | Blue team | Intersection |
|---|---|---|---|
| `datalog/answer.go` | [Mutation splits JSON from cached digest; decoder accepts noncanonical rows](red10_answer.md) | [Same mutation and decoder gaps; invalid Unicode normalization](blue13_answer.md) | Mutable fields can disagree with cached Bytes/Hash; decoder accepts unsorted, duplicate or ragged tuples |
| `datalog/base.go` | No confirmed defect; callback closure and bulk bounds are caller constraints ([report](red02_base.md)) | Callback can mutate original tuple after validation but before storage ([report](blue02_base.md)) | — |
| `datalog/bounds.go` | No confirmed defect ([report](red13_bounds.md)) | No actionable defect; unknown Bound formatting is a malformed-value edge ([report](blue20_bounds.md)) | — |
| `datalog/builder.go` | No confirmed defect under internal callers ([report](red15_builder.md)) | No confirmed public-path defect ([report](blue16_builder.md)) | — |
| `datalog/builtins.go` | No confirmed defect ([report](red17_builtins.md)) | No confirmed defect; input string cost noted ([report](blue08_builtins.md)) | — |
| `datalog/bundled.go` | Verify accepts synchronized invalid identities; whitespace filter incomplete; empty text constructs but fails Verify ([report](red04_bundled.md)) | Same identity, whitespace and empty-text inconsistencies ([report](blue11_bundled.md)) | Verify accepts invalid identity fields; constructor/Verify disagree on whitespace and empty text |
| `datalog/eval.go` | Context cancellation can be missed through many rule applications with no candidate visits ([report](red03_eval.md)) | Cancellation gaps in preparation and delayed bound propagation ([report](blue05_eval.md)) | Cancellation can be missed on work paths without periodic candidate-step checks |
| `datalog/lex.go` | Rejects valid JSON slash/surrogate escapes despite “Go/JSON” comment ([report](red01_lex.md)) | Grammar comments imply JSON, implementation is mixed/Go-oriented ([report](blue09_lex.md)) | String escape and number grammar do not match the stated JSON-compatible contract |
| `datalog/parse.go` | Conflicting declarations silently overwrite earlier arity ([report](red19_parse.md)) | Same declaration overwrite; boolean constants cannot begin comparisons ([report](blue06_parse.md)) | Conflicting declaration arities are discarded instead of rejected |
| `datalog/plan.go` | Check callback can retain/mutate plan-owned Args; base compatibility ignores Checks ([report](red16_plan.md)) | Same callback alias; schema callback mutation can also change later checks ([report](blue18_plan.md)) | Callback receives mutable alias into the compiled plan, so retained mutation changes evaluations |
| `datalog/proof.go` | Composite memo key collides with NUL-bearing relation names ([report](red07_proof.md)) | Same collision via accepted programmatic names ([report](blue01_proof.md)) | NUL-bearing relation names collide in proof memo keys and return wrong premise provenance |
| `datalog/relation.go` | Why-not scans/materializes candidates before charging its budget ([report](red14_relation.md)) | Candidate preparation likewise precedes work accounting ([report](blue12_relation.md)) | Candidate scans and allocation are outside the claimed step budget |
| `datalog/render.go` | JSON unfolds shared proof DAG exponentially; text alters whitespace inside constants; nil Rule can panic ([report](red08_render.md)) | Confirms exponential expansion and whitespace alteration; cyclic public graph and indentation risks ([report](blue17_render.md)) | Shared-proof JSON expansion grows exponentially; quoted source text is normalized incorrectly |
| `datalog/shape.go` | Hand-built AST admits invalid UTF-8 constants (answer hash collision) and zero-arity heads ([report](red06_shape.md)) | Zero-arity heads accepted despite positive-arity invariant ([report](blue19_shape.md)) | Zero-argument derived heads bypass positive-arity validation |
| `datalog/stratify.go` | No semantic defect; exhaustive signed-graph check passed ([report](red11_stratify.md)) | No semantic defect; quadratic guard scheduling noted ([report](blue10_stratify.md)) | — |
| `datalog/term.go` | Direct unmarshal ignores trailing bytes; Number accepts non-JSON syntax ([report](red12_term.md)) | Same, plus Unicode replacement can normalize distinct inputs ([report](blue04_term.md)) | Direct `Const.UnmarshalJSON` accepts trailing input; `Number` accepts non-JSON lexemes |
| `datalog/whynot.go` | Nested open goals lose repeated-variable equality; memo key collapses alias patterns ([report](red18_whynot.md)) | Same repeated-variable explanation bug and memo-key weakness ([report](blue03_whynot.md)) | Why-not explanations can report a rule as matching when repeated variables do not unify |
| `policy/facts.go` | No confirmed defect ([report](red09_facts.md)) | No confirmed defect ([report](blue14_facts.md)) | — |
| `policy/policy.go` | Required relations can be declarations only; context errors omit documented ErrInvalid wrapping ([report](red05_policy.md)) | Listing docs overstate equivalence; trust/context and full receipt identity need consumer care ([report](blue15_policy.md)) | — |

## Pass order

Red: lex, base, eval, bundled, policy, shape, proof, render, facts, answer,
stratify, term, bounds, relation, builder, plan, builtins, whynot, parse.

Blue randomized order: proof, base, whynot, term, eval, parse, base (second
independent review), builtins, lex, stratify, bundled, relation, answer, facts,
policy, builder, render, plan, shape, bounds. Each reviewer received one file only
and no prior conversation history. The duplicate base review is retained as
independent corroboration, so 20 blue reviews cover all 19 implementation files.
