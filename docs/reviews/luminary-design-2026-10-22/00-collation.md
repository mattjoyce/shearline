# Collation — Luminary panel on docs/DESIGN.md, 2026-10-22

Pairs: Hickey × Armstrong (structure × runtime), Ousterhout × Liskov (surface × substitution).
Run before any teammate coded against the contract; resolutions are folded into `docs/DESIGN.md`.

## Converged (both pairs, independently)

| # | Finding | Severity | Resolution in DESIGN.md | Lands in |
|---|---|---|---|---|
| 1 | Proofs must be deterministic; map-ordered `merge` makes them not | 🔴 | sorted relation order on the derivation path; determinism stated; twice-evaluate test | task #1 (merge order), task #3 (test) |
| 2 | Proof over a DAG: share, don't re-expand | 🟠 | memoised per (relation, tuple); shared `Proof` values | task #3 |
| 3 | `Decision.RuleSet string` hides a list | 🟠 | `RuleSets []string` | task #4 |

## One pair, accepted

| # | Finding | Pair | Severity | Resolution | Lands in |
|---|---|---|---|---|---|
| 4 | Bounded bytes, unbounded work | Armstrong | 🟠 | `Options.MaxSteps`; `maxDecisionSteps` constant | task #1, #4 |
| 5 | Frozen must be structural | Armstrong | 🟠 | frozen relation refuses `add` | task #1 |
| 6 | `Schema.Checks` must run on facts too | Liskov | 🟠 | checks run in `NewBase`/`With` as well | task #1 |
| 7 | `With` needs a stated, tested postcondition | Liskov | 🟠 | `With ≡ NewBase(all)` property test | task #1 |
| 8 | `policy` fact constructors | Ousterhout | 🟠 | `Role`, `Inherits`, `Grant`, `Deny` | task #4 |
| 9 | `RBAC()` parse at init, test holds it | Armstrong | 🟡 | package var, `TestRBAC_Parses` | task #4 |
| 10 | Rule sets are policy, not user input | Liskov | 🟡 | sentence in `Load` doc | task #4 |
| 11 | `Permitted` doc leads with "prefixes" | Liskov | 🟡 | doc comment order | task #4 |
| 12 | `*TooLarge` names the remedy for a decision | Armstrong | 🟡 | wrap with remedy in `Decide` | task #4 |

## Correction after implementation

Finding #1 was overstated. The cutter showed that each derived relation appends only from its
own delta, so map order in `merge` never interleaved tuples within one relation and the
derivation order was already a function of the inputs. The sorted walk stays as a guard for the
proof recorder, not as a bug fix; the determinism test and the twice-evaluate proof comparison
remain the standing evidence.

## Held tension (decided, not averaged)

`Schema.Checks` makes `Schema` a non-value. Hickey objects; the spec and Ousterhout want the
small surface. Decision: keep the checks in `Schema`, widen their promise to facts (Liskov), and
accept that a schema is not hashable. Revisit only if a consumer needs to hash one.
