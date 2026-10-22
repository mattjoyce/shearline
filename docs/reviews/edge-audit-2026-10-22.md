# Adversarial module audit — 2026-10-22

Base commit: b7a6717. Branch: audit/edges-20261022.
Worktree: /private/tmp/shearline-edge-audit.

Scope: public API integrity, authorization isolation, malformed program values,
serialization identity, schema enforcement, recursive evaluation, negation, and
existing concurrency/limit regressions. Production code is unchanged. Tests are
intentionally red where they demonstrate the findings below.

## Findings

### 1. High: loaded policy aliases caller-owned program data

`TestAuditPolicyDetachedFromInputProgram` loads a rule allowing Alice to read
Alice's private resource, then changes the original Program's head argument to
Bob's resource. The already-loaded policy now allows Bob's resource.

`Compile` copies Rule structs but retains nested slices and pointers
(`datalog/plan.go:55`, `datalog/stratify.go:129`). This violates Policy's stated
immutability. Concurrent caller mutation could also race with evaluation; this
audit proves the sequential authorization change, not a concurrent race.

Suggested repair: deep-copy programs at the compilation ownership boundary,
including heads, body atoms, comparisons and declarations.

### 2. High: hash verification does not verify the executable program

`TestAuditRuleSetASTMustMatchHash` edits the parsed head before loading, retaining
the original Program.Text and hash. Load succeeds and authorizes Bob's resource,
while the proof's rule text still says Alice's resource.

`RuleSet.Verify` hashes only Program.Text (`datalog/bundled.go:51`). Evaluation
uses the independently mutable AST. This breaks the decision receipt's connection
to the executed rules. It requires access to the Go program object; this is not a
demonstrated prompt-only attack.

Suggested repair: establish one authoritative representation, such as recompiling
verified text, or rejecting AST/text disagreement. Check set identity metadata at
the same boundary. Cloning alone does not repair this mismatch.

### 3. Medium: malformed public ASTs panic at Compile

`TestAuditMalformedPublicASTRejected` supplies a Rule containing an empty Literal,
with and without Negated. Both panic on a nil dereference rather than returning
ErrInvalid. The parser does not generate these shapes, but Program, Rule and
Literal are exported and constructible.

Suggested repair: structural validation before traversing or ordering literals.

### 4. Medium: unknown comparison operator silently means >=

`TestAuditUnknownComparisonRejected` changes a parsed comparison's Op to `bogus`.
Compile accepts it and evaluation derives the output fact. The default arm of
`holds` implements >= (`datalog/eval.go:334`). Again, this requires a hand-built or
modified AST, not ordinary parsed rule text.

Suggested repair: validate operators at Compile; make >= explicit and unknown
operators fail closed.

### 5. Medium: distinct string values can share answer bytes and hashes

`TestAuditInvalidUTF8DoesNotCollideInAnswerHash` compares a string containing byte
0xff with the Unicode replacement character. The engine distinguishes the facts,
but JSON serialization replaces invalid UTF-8, producing identical answers and
hashes. This is a serialization collision, not a SHA-256 collision.

Suggested repair: reject invalid UTF-8 at a validated input boundary, or define a
lossless encoding. Document the accepted string domain explicitly.

### 6. Contract gap: plan schema checks do not constrain foreign bases

`TestAuditPlanCannotBypassSchemaChecks` compiles under a schema rejecting a value,
then evaluates a base made under a same-arity schema with no checks. The rejected
value is accepted. Plan.accepts checks relation names/arities, not validation
semantics (`datalog/plan.go:263`).

This affects generic datalog callers composing bases and plans. Policy.Load builds
both from its own schema, so this is not a demonstrated bypass through normal
policy.Request inputs. Decide whether schema compatibility means shape only, or
whether check provenance must be enforced; the current wording overpromises.

## Passing evidence

- Existing tests, plus fuzz seeds: `go test -race -shuffle=on -count=2 -cover ./... -skip TestAudit`.
  Coverage: datalog 95.7%, policy 92.3%.
- `go vet ./...` passed.
- Parser → compiler → bounded evaluator fuzz: 773,297 executions in 30 seconds,
  no reported failure. Inputs capped at 4 KiB; 1,000 join steps, 4 KiB derived
  bytes and 100 ms context per evaluation. This does not bound parsing/compilation.
- Transitive closure versus an independent Floyd–Warshall oracle: 188,602
  executions in 30 seconds. Includes cycles, self-edges, duplicates, proof
  membership and absence queries on graphs of up to eight nodes.
- Repeated-variable joins and stratified negation versus a Boolean-matrix oracle:
  344,021 executions in 30 seconds, up to eight nodes.
- Corporate-chat delegate/ownership cases: self allowed; another employee,
  near-prefix employee ID, missing delegate and wrong delegate denied.
- Zero-valued Policy fails closed without panicking for a well-formed request.
- All six findings reproduce with `-race`; no concurrent mutation was introduced.

Fuzz runs are finite exploration, not a proof of correctness. Existing bounds tests
were rerun; this audit did not establish a total memory or wall-clock ceiling for
parsing, compilation, proof construction or rendering.

## Integration edges already in the contract

- Principal and delegate identity must come from trusted caller context.
- Prefix grants have no path normalization or exact-match semantics. A trusted
  integration must canonicalize resources before evaluation and execute that same
  target. An apparent leaf prefix also matches longer strings.
- Permitted returns prefixes and does not apply concrete-resource denials.
- Numeric constants use float64; integers beyond 2^53 are not exact identifiers.
- Consumer rules can add allow directly and omit the bundled deny guard. Rule
  authors must preserve delegate/ownership constraints in every allow path.

## Reproduce

From the audit worktree:

```sh
go test ./... -run TestAudit -count=1
go test ./... -race -run TestAudit -count=1
go test -race -shuffle=on -count=2 -cover ./... -skip TestAudit
go test ./datalog -run '^$' -fuzz '^FuzzAuditParseCompileEvaluate$' -fuzztime=30s -parallel=2
go test ./datalog -run '^$' -fuzz '^FuzzAuditTransitiveClosure$' -fuzztime=30s -parallel=2
go test ./datalog -run '^$' -fuzz '^FuzzAuditNegationAndRepeatedVariables$' -fuzztime=30s -parallel=2
```

Recommended repair order: executable rule identity and ownership (1–2), AST
validation (3–4), string serialization domain (5), schema compatibility contract (6).

---

## Resolution (lead, 2026-10-22)

All six accepted. Findings 1 to 5 are fixed on master with the audit's tests adopted as
regressions (each red on b7a6717, green after), plus its three fuzz suites and oracles under
`datalog/fuzz_test.go` and `datalog/oracle_test.go`; finding 6 is a contract correction in
`docs/DESIGN.md` (plan/base compatibility is by shape only; a caller that wants one set of checks
everywhere builds plan and base from one Schema, as `policy.Load` does). The string domain and the
float64 identifier limit are now stated in the contract. None of the six was reachable through
`policy.Request`; all need the Go program object.
