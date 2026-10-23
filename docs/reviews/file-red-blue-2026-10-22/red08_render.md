# RED08 — datalog/render.go

Scope: snapshot 5fe3a78; only render.go reviewed, with proof.go, whynot.go and term.go inspected for input contracts and graph/encoding seams. No production edits.

## Confirmed findings

### R08-1 — High: proof JSON expands a compact valid DAG exponentially

Lines 184–194, also reached by blocked absence proofs at 227–229. `Proof.json` recursively copies every child without memoization or a size/depth budget. Engine-produced proofs share subproofs; a legal chain `p0(X) :- node(X). p1(X) :- p0(X), p0(X). ...` needs linear evaluation work and stored provenance but doubles rendered descendants at every level. `json.Marshal` cannot apply its usual cycle/depth protection until this eager conversion has finished. Evaluation MaxSteps/MaxBytes do not bound this conversion.

Evidence: `TestRed08RenderSharedProofGrowth` succeeds with MaxSteps=100 and MaxBytes=4096. At depths 8, 12, 16 JSON is respectively 61,595; 987,070; 15,794,710 bytes, while text is 717; 1,161; 1,677 bytes. Greater depth predictably exhausts memory/CPU; that destructive endpoint was not run. The full-tree wire shape is explicitly documented, so expansion is not a schema violation; the finding is the missing bound when serializing otherwise bounded engine output. Consider references or a bounded/error-returning traversal before allocating an unfolded tree.

### R08-2 — Medium: whitespace normalization changes the displayed producing rule

Line 73 (`oneLine`), consumed at 97 and 135. `strings.Fields` normalizes whitespace inside quoted constants as well as between tokens. A valid rule `p("a  b") :- node("a  b").` is shown as `p("a b") :- node("a b").`, a different rule about different constants. This compromises operator-facing provenance even with unmodified engine-produced inputs. JSON preserves original rule text and is unaffected.

Evidence: `TestRed08RenderRuleLiteralWhitespace` prints `p("a  b")  line 1  p("a b") :- node("a b").` followed by `node("a  b")  fact`. Preserve literal contents when flattening source text, e.g. through token-aware normalization.

### R08-3 — Low: public Absence value with nil Rule panics in String

Line 135 dereferences `rf.Rule.Text`; `ruleRef` also dereferences at 64. `Absence{Rules: []RuleFailure{{}}}.String()` panics. JSON already accepts that shape via `ruleJSON(nil)`. Confirmed by `TestRed08RenderNilFailureRule`. Engine-produced failures have rules, so this is limited to publicly constructed/mutated explanation values. Handle absent rules or explicitly document the invariant.

## Additional boundary observations

Public slices permit cycles: `p.Body=make([]Proof,1); p.Body[0]=p` and `a.Rules=make([]RuleFailure,1); a.Rules[0].Because=[]Absence{a}`. The former terminates in String through tuple identity but recurses indefinitely in JSON; the latter recurses indefinitely in both renderings. This follows directly from the traversal, but fatal stack exhaustion was not executed. Engine-produced values are documented acyclic and the absence walker is bounded, so this is a public-input robustness extension of R08-1, not a separately claimed engine cycle bug.

## Nonfindings / limits

- String constants use strconv.Quote; JSON fields use encoding/json. Embedded newlines/quotes in valid constant values do not create raw extra text lines.
- Relation/tuple keys include kind and length encoded constants. No collision found for engine-valid relation identifiers, mixed constant kinds, or embedded NUL string values.
- Text proof deduplication intentionally identifies the same relation/tuple; distinct public proof alternatives can therefore be suppressed, but this agrees with documented engine identity and is not separately reported.
- Bound variable names are sorted for deterministic text; JSON map encoding is deterministic. Open absence goal positions become null as documented; constants retain type.
- Rule source Set, relation names, variable names and comparison operators are trusted strings in public constructed values; no separate injection claim is made without a supported engine ingress for malformed values.
- Serialization does not include rule-set content hashes or AST. This is the documented receipt shape, not evidence of a missing promised integrity check.

Validation: `go test ./datalog -run TestRed08Render -v` passes. Reproducers: datalog/red08_render_test.go. Fatal recursion and resource exhaustion endpoints were intentionally not run.
