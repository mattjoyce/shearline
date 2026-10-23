# Resolution (author, 2026-10-23)

Snapshot audited: `5fe3a78`. Fixed on master in three commits after `863a294`; every
finding below names the one it landed in. The audit's reproducers were not adopted as
written, since they assert the defects; each is replaced by a test that asserts the
behaviour wanted, under the finding's name, in `datalog/audit2_test.go` or beside the
code it holds. Nothing here was reachable through `policy.Request`; the authorization
decision was sound throughout. What was loose was the explanation, the receipts, and
the contracts around hand-built values.

## Commit 1: why-not unification

| Finding | Resolution |
|---|---|
| red18, blue03: a nested goal `p(X, X)` lost its equality; a constant head was a candidate, head variables bound apart, a head constant did not bind the goal | `unifyHead` unifies head and goal over a union-find and returns what the goal fixes and which head variables it equates; the walk renames the equated variables to one and reports the literal as written. `goalKey` tells `p(X, X)` from `p(X, Y)`. |

## Commit 2: rendering, cancellation, parse, policy contracts

| Finding | Resolution |
|---|---|
| red08 R08-1, blue17: `Proof.MarshalJSON` unfolded a shared DAG exponentially | A proof is written once per (relation, tuple); a later occurrence is `{relation, tuple, rule: null, shown: true}`, as the text says "(shown above)". Documented in DESIGN.md and the OKF proof concept. |
| red08 R08-2: `oneLine` collapsed whitespace inside quoted constants | Token-aware: a string is kept as written, a comment is dropped. |
| red08 R08-3: nil `Rule` in a hand-built `Absence` panicked in `String` | Renders as "(no rule)". A cyclic hand-built `Absence` ends in "(explanation truncated)" at depth 1024, text and JSON. |
| red03, blue05: cancellation seen only every 1024 join steps; ground rules visit no tuple | The deadline is asked before each rule and before the result is returned, as well as per 1024 steps. |
| red19, blue06: `p/1. p/2.` was a silent overwrite; `true = B` did not parse; `true(1)` was a relation | A conflicting redeclaration is ErrSyntax; `true`/`false` are constants everywhere, so a comparison may start with one and a relation cannot be named one. |
| red05 (low): context errors omitted `ErrInvalid` | Both sentinels wrap. A request with more context keys than are declared is refused before its keys are sorted (blue15 B15-4). |
| red05 (low): `allow/3. permit/3.` loaded and served a silent deny-all | `Plan.Defines` is new; `Load` requires `allow` and `permit` to be defined, not only declared. |
| blue15 B15-1: README read as if `Permitted` were a gate | README: `Permitted` carries no deny and no context; it is discovery, `Decide` every resource acted on. DESIGN decision 5's "listing" is the `Decide` loop, which is the gate; no change there. |
| blue15 B15-2: a declared context key is optional | Stated on `Config`: a declaration is a schema, not attestation; a key a rule relies on is the consumer's to require, or to require positively in the rule. |
| blue15 B15-3: rule-set IDs do not cover standing facts | Consumer responsibility, already stated; a receipt that must identify the whole policy carries the consumer's own hash of its standing facts. No change. |

## Commit 3: values, names, grammar

| Finding | Resolution |
|---|---|
| red10, blue13: `Answer`'s exported fields could disagree with its cached bytes and hash; the decoder accepted unsorted, repeated, ragged and null rows | `Answer` is immutable: `Relation()`, `Tuples()` (copy), `Len()`, `Bytes()` (copy), `Hash()`; it marshals as `Bytes`, a zero `Answer` marshals to an error, and `UnmarshalJSON` admits only an answer as written. API change, pre-1.0. |
| red07, blue01: a NUL in a relation name collided in the proof memo key | A name is held to the lexer's grammar wherever it comes from: `checkShape` (declarations, heads, body atoms), `checkSchema`, and `Answer.UnmarshalJSON`. The key's separator is then unambiguous. |
| red06, blue19: a hand-built AST admitted an invalid UTF-8 constant (answer-hash collision) and a zero-arity head | `checkShape` holds every term: a string is UTF-8, a variable is a variable name, an atom has an argument. |
| red16, blue18: a schema check received a writable alias into the compiled plan | The check sees a copy of the atom. Plan/base compatibility by shape only stands as the contract (edge audit, finding 6). |
| blue02: a check could change the caller's tuple between validation and storage | The tuple is copied before anything looks at it; what is checked is what is stored. |
| red12, blue04: `Number` read Go's float grammar; `Const.UnmarshalJSON` ignored trailing input | `Number` holds to JSON's number grammar; a constant is one JSON value. |
| red01, blue09: "Go/JSON escapes" but `\/` and surrogate pairs were refused | A literal reads as JSON or as Go: `unquote` tries JSON's escapes, then Go's; a lone surrogate reads as neither. |
| red04, blue11: `Verify` did not hold a set's identity; the name check missed Unicode whitespace; a set with empty text constructed and could not verify | `checkIdentity` is shared by `NewRuleSet` and `Verify`: no whitespace or control character, slash or semicolon, version not negative. Empty text is refused at construction. |
| red14, blue12: a why-not walk scanned a derived relation before charging its budget | The scan is charged: every tuple when the relation has no index or no argument is bound. |

## Not changed

- blue20 (bounds), red13, red11/blue10 (stratify), red17/blue08 (builtins), red09/blue14 (facts), red15/blue16 (builder): no defect.
- blue02's wider points (callback purity, concurrency of checks, construction bounds) are integration constraints and stay as stated contracts.
- red16's second point, plan/base compatibility by shape only, was decided at the edge audit.
