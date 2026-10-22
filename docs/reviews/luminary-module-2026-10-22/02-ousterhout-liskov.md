# Ousterhout × Liskov — After pass (code audit) — Shearline @ 009a13a

Pair: surface × substitution. Reviewer: Max (Anthropic top rung), 2026-10-22. Independent of
`docs/reviews/luminary-design-*` (not read).

**Artifact.** `datalog/*.go`, `policy/*.go`, `policy/rbac.dl`; contract `docs/DESIGN.md`; intent
`SPEC.md`. **Evidence.** `go test -race -shuffle=on -cover ./...` → datalog 96.2 %, policy 92.9 %,
`go vet` clean. Three throwaway hostile-consumer test files (14 probes) in my scratchpad:
`/private/tmp/claude-501/-Users-mattjoyce-Projects-shearline/cd983570-d926-4d82-862f-d6f19ae622ba/scratchpad/probe/{datalog_probe_test.go,policy_probe_test.go,order_probe_test.go}`.
Every finding below quotes `file:line` and, where a probe proved it, the probe's output.

**Line numbers are at 009a13a.** The working tree moved under this audit (other agents in the same
session were editing `datalog/` and `policy/` while it ran), so I extracted the commit with
`git archive 009a13a` into the scratchpad and re-ran every probe against that clean copy: the
output was byte-identical to the first run, and the content I cite (no `Verify`, no `Plan.Arity`,
`var Schema`, the unseeded `rbac.dl`) is the commit's. The appendix at the end records which
findings the working tree had already closed when I finished, with the same probes as evidence.

Severity: 🔴 must-fix · 🟠 should-fix · 🟡 consider. Counts: **1 🔴 · 7 🟠 · 12 🟡**.

---

## Eye 1 — Ousterhout: the shape of the boundary

*Is the module deep? What must a caller know? Which parameter and error shapes leak the
implementation? Which exceptions could be defined out of existence?*

✅ `Compile`/`Evaluate` and `Load`/`Decide` are genuinely deep: five calls buy stratified semi-naive
evaluation, frozen shared indexes, proofs and two bounds, and the comments carry intent rather than
syntax (`eval.go:371-374`, `builtins.go:26-28`, `policy.go:13-22`).

### O-1 🟠 `RuleSet` is an open struct, so "named, versioned, hashed" is advisory
`bundled.go:13-18` exports `Name`, `Version`, `Hash`, `Program`; the only validation lives in the
constructor `bundled.go:26-31`. `policy.Load` (`policy.go:86-89`) copies `s.ID()` into every
decision without looking. Probe P7: `Load(standing, RBAC(), datalog.RuleSet{Name:"forged set/with
spaces", Version:-1, Hash:"sha256:0000", Program: p})` → `Decision.RuleSets = [... "forged set/with
spaces/-1 sha256:0000"]`; a zero `RuleSet{}` loads and yields the ID `"/0 "`. `Compile` also accepts
a `Program` whose `Text` disagrees with its `Rules` (probe F). The SPEC job story "I want its hash in
every decision … so I can say which rule set stood behind an answer" (`SPEC.md:30-31`) is only true
for sets built by `NewRuleSet`. Caller must know: never build a `RuleSet` by hand. Define it out of
existence: unexport the fields (`Name()`, `Version()`, `Hash()`, `ID()`, `Program()`), or at minimum
have `Load` recompute `sha256(Program.Text)` and refuse a mismatch or an invalid name.

### O-2 🟠 `Decision.Proof.Rule` is a live pointer into the shared `Plan`
`proof.go:13` says "shared not copied, and is never written through" — true of the engine, not of
the receipt holder. `proof.go:43` sets `p.Rule = d.rule`, `eval.go:281` takes `&r.Rule` into
`plan.strata`. Probe G: set `p.Rule.Head.Name = "pwned"` on a returned proof, call `Evaluate` again
on the same plan → **nil-pointer panic inside the engine** (`eval.go:242`, `ev.db["pwned"]` is nil).
In `policy`, `Decision` (`policy.go:65`) hands this pointer to every consumer; a receipt pipeline
that "tidies" or canonicalises its proof corrupts the `Policy` every goroutine shares. The cheap
design (one pointer per derivation) exports the engine's internal representation as a mutable
handle. Copy `Rule` by value into `Proof` (Rule is a head plus a small body slice; cost is linear in
the proof, which is already built per call), or make the field unexported with a copying accessor.

### O-3 🟡 `Request.Context` makes the consumer learn the engine's vocabulary
`DESIGN.md:188-189`: "constructors are how a consumer builds standing facts without learning the
engine's vocabulary". `policy.go:51` then types `Context` as `map[string]datalog.Const`, and the only
numeric constructor is `datalog.Number(lexeme string) (Const, error)` (`term.go:38`) — a parser-shaped
signature (a JSON lexeme in, an error out) for a consumer holding an `int` hour. Also the ergonomics
hide O-7 below. Consider `datalog.Float(float64)`/`Int(int64)` or policy-level `Str/Num/Bool`.

### O-4 🟡 One sentinel for everything a caller might want to tell apart
`plan.go:10-14` lists eight conditions behind `ErrInvalid`; `answer.go:27` adds "no relation in the
result"; `policy.go:93,97,112,122` wrap all of them as `policy: load:`/`policy: decide:`. A consumer
serving HTTP cannot distinguish *this request was malformed* (`Decide` → `With` refuses an empty
principal, a 4xx) from *this policy is broken* (`Decide` → `Relation("allow")` missing, a 5xx)
without matching message text. Not a correctness bug — but it is the error shape leaking the
implementation's single bucket. A `policy.ErrRequest` wrapping the former would cost one line.

### O-5 🟡 `Schema.Checks func(Atom) error` leaks the AST and loses the caller's error
A check receives the parser's `Atom` with `Term`s and must know `Term.IsVar()` and that `"_"` is a
variable (`facts.go:34`), even at `NewBase` where no variable can occur (`base.go:114-118` dresses
constants up as terms). The check's error is flattened with `%v` (`plan.go:157`, `base.go:119`):
probe J shows `errors.Is(err, mySentinel) == false` at both sites. Wrap with `%w` (two characters)
and consider a constants-only shape (`func(Tuple) error`) for the base path.

### O-6 🟡 `Decide` pays for an `Answer` it throws away
`policy.go:120-131` calls `res.Relation("allow")` — which copies, sorts, JSON-serialises and sha256s
the whole relation (`answer.go:29-36`) — then linearly scans `ans.Tuples` for the request. The result
already answers membership: `res.Proof("allow", want)` (`proof.go:23-31`) returns `ok` iff the tuple
is in the result. `Allow, Proof = res.Proof(...)` is both deeper and cheaper. (`Permitted` legitimately
needs the relation.)

### O-7 🟠 `Number` accepts lexemes JSON has no value for, and the failure surfaces far away
`term.go:11-12`: "the three scalar kinds JSON has". `term.go:38-44` delegates to
`strconv.ParseFloat`, which accepts `NaN`, `Inf`, `+Inf`, `-inf`, `infinity`, `1_000`, `.5`, `5.`
(probe A, all `err=nil`). `MarshalJSON` (`term.go:53-59`) then writes the raw lexeme, and
`Answer.seal` (`answer.go:40-50`) fails with `json: error calling MarshalJSON … invalid character
'N'`. So a `Const` that is legal to construct makes any relation that holds it impossible to read
(`Result.Relation` errors; a `Decide` whose consumer rule carries a context value into `allow` denies
with an opaque engine error). Also `compare` (`term.go:96-112`) makes `NaN = NaN` true. One check
(`math.IsNaN || math.IsInf → ErrSyntax`) defines the error out of existence at the only door.

### O-8 🟡 Two conventions for "unknown relation", and a dangling message
`Base.Len` returns 0 for a name it lacks (`base.go:97-104`); `Result.Relation` makes it an error
(`answer.go:22-27`). Both are documented; a caller still carries two rules. And the zero `Base`
(probe F) / zero `Policy` (probe P6) refuse with `"…the schema does not have; the relations are "`
— `known(nil)` (`plan.go:232-239`) renders the empty list as nothing. Say "the schema has no
relations".

### O-9 🟡 `*TooLarge`: which bound, and no package prefix
`eval.go:16-19` encodes *which* bound by "the one whose Max field is set and exceeded"; a caller
inspects two field pairs to learn one bit. `TooLarge.Error()` (`eval.go:29-36`) reads `derivation
too large: …` with no `datalog:` prefix, against `DESIGN.md:10-11`; probe H shows the exact text.

### O-10 🟡 Doc drift a reader will trip on
`bundled.go:22-23` ("free of spaces and semicolons") omits the slash the code refuses at
`bundled.go:27`. `DESIGN.md:24` says the check runs on "every atom (head or body)", but a head can
never name a base relation (`plan.go:122-124`), so the head half is dead text. `DESIGN.md:42-44`
promises "the property a test holds over random fact sets" for `With ≡ NewBase`; `base_test.go:182-206`
is one fixed example and there is no `rand`/`quick` anywhere in the suite — a claim of done with no
evidence attached.

---

## Eye 2 — Liskov: the promise the boundary keeps

*Is every contract in DESIGN.md honoured off the happy path? Which pre/post-conditions and invariants
hold only when the caller is polite? What breaks if a different rule set or base slots in?*

✅ `Const`'s representation is fully hidden — no odd `Kind` is constructible from outside — and
`Decide` stayed total under every hostile request I built (zero `Policy`, nil context, empty key,
boolean and zero-value context values): `Allow=false` with an error, never a panic, never a silent
allow (probes P4-P6). `-race` with concurrent `Decide` and `Evaluate` is clean.

### L-1 🔴 `Load` accepts a policy that fails — or panics — at request time
`SPEC.md:176-178`: "`Compile` refuses at load what would fail at request time, so a bad policy never
serves." `DESIGN.md:165-166`: "Any error is the consumer's to fix before serving." `policy.Load`
(`policy.go:78-100`) never checks that the compiled plan defines `allow/3` and `permit/3`, the two
relations `Decide` (`policy.go:120`) and `Permitted` (`policy.go:150,157,160`) read. Probe P3, three
symptoms of the one omission:
- set without `allow`: `Load` ok; every `Decide` → `policy: decide: … no relation allow in the result`;
- set defining `allow/2`: `Load` ok; `Decide` → `Allow=false, err=nil` forever (`sameTuple` length
  mismatch at `policy.go:174-176`) — a silent, total deny with no signal;
- set defining `permit/2`: `Load` ok; **`Permitted` panics** `index out of range [2] with length 2`
  at `policy.go:160` (`t[2]`).
Rule sets are trusted, but a trusted author forgetting `RBAC()` is exactly the misconfiguration the
load-time promise exists for, and a policy engine must not panic on its read path. Fix in `Load`:
after `Compile`, require `allow/3` and `permit/3` in the plan's relations (expose a small
`Plan.Arity(name) (int, bool)` or check `res.Relation` once on an empty base) and refuse otherwise.

### L-2 🟠 `Options.MaxBytes`: "over it, `*TooLarge`" is false in one reachable pattern
`eval.go:121-123` and `DESIGN.md:54`. `count` (`eval.go:299-301`) only *sets* `ev.over`; it is
returned by the next `join` iteration (`eval.go:329`) or by the first-pass check (`eval.go:188-190`).
In the semi-naive loop (`eval.go:192-208`) and after the strata loop in `Evaluate` (`eval.go:157-162`)
nothing reads `ev.over`. Probe C: `reach(X) :- start(X). reach(Z) :- reach("a"), edge("a", Z).` with
`start("a")`, `edge("a","b")`, `MaxBytes: 7` — the second tuple crosses (10 > 7), the next round's
delta atom `reach("a")` has zero candidates for `reach("b")`, the stratum ends, and `Evaluate` returns
`err=nil` with both tuples. Overrun is bounded (any later join iteration would catch it) so this is a
misreport, not a runaway — but `policy` calls `maxDecisionBytes` "an invariant, not a knob"
(`policy.go:13-17`) and here the invariant is silently waived. One line after the strata loop:
`if ev.over != nil { return Result{}, ev.over }`.

### L-3 🟠 `Base` immutability holds only if nobody writes through an `Answer`
`base.go:11-13`: "immutable once built and safe for concurrent Evaluate calls." `Result.Relation`
(`answer.go:29-31`) copies the slice of tuple *headers*; each `Tuple` still aliases the frozen
relation's backing array, including base relations the `Result` shares with the `Base`. Probe B:
`ans.Tuples[0][0] = String("mallory")` on the `role` answer, re-evaluate the same `Base` → the base
now holds `role(mallory, admin)` and derives `has(mallory)`. In `policy` the `Answer` never escapes, so
the standing base is safe today; in `datalog` the hashed "value" (`answer.go:11-13`) is a window onto
shared mutable storage. `Relation` already pays O(n log n) to sort and serialise; cloning each tuple
is noise on top. (`Proof` gets this right: `proof.go:41` clones.)

### L-4 🟠 "Duplicate facts count once" is implemented over a key that is not injective
`base.go:23-24`. `Tuple.key()` (`term.go:127-134`) joins `kind+text` segments with `\x00`; a string
may contain `\x00`. Probe D: `role("x\x00""0y", "z")` and `role("x", "y\x00""0z")` are distinct facts and
`NewBase` holds **one** (`Len(role) = 1`); `policy.Load` accepts them (probe P10, "nul in role").
Joins, `has`, `Proof` lookup and the per-position index all share this identity, so two distinct
tuples are one tuple everywhere. Consumers whose role/resource strings come from outside can be made
to collide. Length-prefix each segment (or refuse NUL at `NewBase`); either closes it.

### L-5 🟠 "A base built under a schema other than the plan's is `ErrInvalid`" — only if smaller
`eval.go:138`. `Plan.accepts` (`plan.go:244-252`) checks that every plan relation exists in the base
at the right arity, nothing more. A base built under a *superset* schema passes, and `Evaluate`
(`eval.go:151-156`) copies the base's relations into `ev.db` and then overwrites any name the plan
derives with a fresh empty relation. Probe E: plan over `{a/1}` deriving `d/1`; base over `{a/1, d/1}`
holding `d("from-base")` → `err=nil`, result `d = []`. The documented precondition is weaker than
the one the code enforces, and the gap drops data silently. `accepts` should also refuse a base
relation that the plan derives (`p.idb[name]`), or require exact schema equality.

### L-6 🟠 `RuleSet.Hash` "over Text" is an invariant the type cannot keep
Same locus as O-1 (`bundled.go:13-18`, `policy.go:86-89`, probe P7). From this eye: the receipt's
postcondition — `Decision.RuleSets` identifies the text that stood behind the answer — holds for
`NewRuleSet` values and for nothing else that satisfies the type. Any consumer that round-trips a
`RuleSet` through JSON or builds one from a store can present a hash the text does not have.
Verify at `Load` or unexport.

### L-7 🟠 `Plan` immutability has the same hole, through `Proof.Rule`
Same locus as O-2 (`proof.go:13,43`; probe G panic). From this eye: "immutable once built and safe
for concurrent Evaluate" (`DESIGN.md:121-122`) is a promise about an object whose address every
`Decision` carries out. A promise the holder of a receipt can break for every other goroutine is not
a promise.

### L-8 🟡 Proof determinism is over the fact *sequence*, and `Load` does not normalise it
`DESIGN.md:75-76`: "for one (plan, base, options) every evaluation yields the same proof". True — and
`base` means the ordered sequence of facts, since `rebuild` (`base.go:83-95`) and the per-position
index preserve insertion order and "first derivation" follows it. `Request.facts()` sorts context keys
precisely "so … any proof drawn from it does not depend on map order" (`facts.go:75-76`), but `Load`
(`policy.go:95`) passes `standing` in caller order. Probe (`order_probe_test.go`): the same four
standing facts in two orders → `allow` proved via `role(alice,reader)` in one and `role(alice,admin)`
in the other; `reflect.DeepEqual(proofs) == false`. A consumer loading standing facts from a map or
an unordered store gets a different receipt for the same request on each restart. Either sort facts
at `Load` (cheap, closes it) or state in `Decision`'s doc that proofs are stable per `Policy`, not per
policy *content*.

### L-9 🟡 `MaxBytes` is "over every derived tuple's JSON text" — it is an estimate of it
`eval.go:121`, `eval.go:288-296`: `1 + len(t) + Σ(len(s) + 2 if string)`. JSON escaping is not
counted. Probe H: a tuple whose one string is `"\` × 10 is charged 24 bytes; its JSON array is 44
bytes; `MaxBytes: 30` is not triggered. Harmless as a sanity bound, wrong as a stated postcondition.
Say "approximately" or count escapes.

### L-10 🟡 `Permitted`'s postcondition — "the grant patterns" — is only as true as the sets loaded
`policy.go:135-137,160`. `permit/3` is derived, so `Schema.Checks` never sees it; `nonEmptyString`'s
claim that "an empty prefix would match every resource" (`facts.go:24-28`) guards `grants` facts only.
Probe P2: a consumer rule `permit(P, A, 7) :- …` → `Permitted = ["7", "docs/"]` while `Decide` can
never match a number; `permit(P, A, "") :- …` → `Permitted = [""]` and `Decide` allows `delete
secrets/x`. Both are consumer-authored, as `DESIGN.md:163-166` warns, and `SPEC.md:28-29` ("the listing
and the gate cannot disagree") is already narrowed by `DESIGN.md:178-179` (deny not applied). The
remaining gap: say that `permit/3`'s third column is a contract (non-empty string) the consumer's set
must keep, and have `Permitted` skip or refuse a non-string there rather than `Text()` it.

### L-11 🟡 `policy.Schema` is a mutable package variable
`facts.go:12`. Probe P8: `policy.Schema.Arities["grants"] = 2` → the next `Load(RBAC())` fails
`grants has 3 arguments, but grants is grants/2`. The PARC contract is a global any package can
rewrite at runtime. Make it a function returning a fresh value, or copy on `Load` and document it.

### L-12 🟡 `Decide`'s error-vs-deny split is total, but the two are not separable (see O-4)
A different implementation that returned `ErrInvalid` for a missing `allow` relation at `Load`
instead of per request would be *more* conservative and break no caller — which is the tell that
the current behaviour is the weaker contract (and L-1 is the fix).

---

## Convergence — where both eyes land on the same spot (highest confidence)

1. **L-1 🔴** `Load` must validate `allow/3` and `permit/3`. Ousterhout: a caller must know to pass
   `RBAC()` first and gets a per-request error or a panic if they forget — define it out of
   existence at the one door. Liskov: the load-time promise is written in both SPEC and DESIGN and
   not kept.
2. **O-2 / L-7 🟠** `Proof.Rule` as a pointer into the shared `Plan`. Shape leak and broken invariant
   are the same line; the probe's panic is the proof.
3. **O-1 / L-6 🟠** `RuleSet` open struct. The receipt's integrity is the module's whole reason to carry
   a hash, and today it is a convention.
4. **L-5 🟠** `accepts` enforces less than `Evaluate`'s doc promises, and the gap drops data.
5. **L-2 🟠** `MaxBytes` overrun unreported in the constant-filtered-delta pattern; one-line fix.
6. **O-7 🟠** `Number(NaN)`: an error that should be impossible to construct is instead reported by an
   unrelated function three calls later.

## Tension — where the lenses pull apart (decide, don't average)

- **Open data vs enforceable invariant (RuleSet, Proof.Rule).** Ousterhout prefers plain exported
  structs: smaller surface, no getters, Go-idiomatic. Liskov says a type *is* its contract and an
  invariant the type cannot keep is not one. The honest options are (a) unexport and add accessors
  (bigger surface, real invariant) or (b) keep the struct and verify at `Load`/copy at `Proof`
  (cheap, closes the practical hole, leaves the type's own promise advisory). Either is defensible;
  the current state — documented invariant, no enforcement — is the one that is not.
- **"Rule sets are policy, never user input" vs the `permit/3` contract.** Ousterhout likes one
  trust level: no validation surface for derived relations keeps `Load` small. Liskov points out
  that `Permitted` and the empty-prefix guard then only hold for well-behaved sets, and the
  behaviour under a careless set is a panic (L-1) or a wildcard (L-10). Validate the two contract
  relations at `Load`; leave the rest to trust.
- **Estimate vs statement (MaxBytes).** Ousterhout is right that the hot loop should not JSON-encode
  to count bytes. Liskov is right that the doc should then not say "JSON text". Fix the sentence,
  not the loop.
- **Total `Decide` vs distinguishable failures.** "Error is deny" is the right simplification for the
  gate; it is the wrong one for the operator reading logs. Keep totality; add one sentinel for
  request-side refusals.
- **Receipts stable per Policy vs per content (L-8).** Sorting standing facts at `Load` makes proofs a
  function of content and costs O(n log n) once; leaving it makes them a function of the caller's
  order and must be said. Shearline's "deterministic, hashable" pitch (`SPEC.md:9-11`) argues for
  sorting.

## Verdict

**concerns.** Nothing allows what should be denied under the bundled set; the engine's core promises
(fail-closed, stratified, terminating, concurrent-safe, proofs deterministic per sequence) hold under
every hostile input I tried. What does not hold is the load-time gate (L-1, with a panic on the read
path) and four stated invariants that are true only for polite callers (L-2, L-3, L-5, L-7/O-2) plus a
receipt-integrity promise the type cannot keep (O-1/L-6). All have one- to ten-line fixes.

---

## Appendix — status against the working tree at the time of writing (not part of the 009a13a verdict)

Same probes, `replace` pointed at the live repo (`git diff HEAD --stat`: 16 files, +549/−59, uncommitted).
"Closed" means the probe that failed at 009a13a now passes or is refused at the door; nothing here was
taken from the diff text alone.

| Finding | Working tree | Evidence |
|---|---|---|
| L-1 🔴 Load does not validate `allow/3`, `permit/3` | **closed** | `Load` → `no rule set defines allow/3; load RBAC() first or define it`; `allow/2` → `allow is allow/2, but a decision reads allow/3` |
| O-1 / L-6 🟠 hand-built `RuleSet` hash | **closed** | `Load` → `hash "sha256:0000" is not the hash of its text (…)`; zero set → `has no program text to verify its hash against` |
| L-2 🟠 MaxBytes overrun unreported | **closed** | probe C at `MaxBytes: 7` now returns `*TooLarge` (`2 derived tuples … over 7 bytes`) |
| L-3 🟠 `Answer.Tuples` alias the Base | **closed** | probe B: base still `role(alice, admin)` after the write |
| L-4 🟠 NUL key collision | **closed** | probe D: `Len(role) = 2` |
| O-7 🟠 `Number(NaN/Inf)` | **closed** | `"NaN" is not a finite number` |
| L-11 🟡 `policy.Schema` mutable | **closed** | `Schema()` returns a fresh value; mutating it leaves `Load(RBAC())` ok |
| L-9 🟡 MaxBytes "JSON text" | closed as a doc fix | `Options.MaxBytes` doc now says "approximately … (lengths unescaped)"; probe H still shows 24 charged vs 44 actual, which is now what the doc promises |
| O-2 / L-7 🟠 `Proof.Rule` pointer into the shared `Plan` | **open** | probe G: writing `Proof.Rule.Head.Name` still panics the next `Evaluate` (`nil pointer dereference`); `DESIGN.md` "Open" now lists it as item 2, unresolved |
| L-5 🟠 superset-schema base accepted, derived name shadows base | **open** | probe E: `err=nil`, `d = []` |
| L-8 🟡 proof depends on standing-fact order | **open** | `order A: role(alice,reader); order B: role(alice,admin); DeepEqual=false` (walk adapted to the seeded `holds`) |
| L-10 🟡 `Permitted` lists non-string / empty "prefixes" | **open** | `["7" "docs/"]`; `permit(P,A,"")` → `Decide` allows `delete secrets/x`, `Permitted = [""]` |
| O-5 🟡 check error identity lost | **open** | `errors.Is(err, sentinel) = false` at both `Compile` and `NewBase` |
| O-8 🟡 dangling "the relations are " | **open** | zero `Policy`/`Base` messages unchanged |
| O-9 🟡 `TooLarge.Error()` has no `datalog:` prefix | **open** | `"derivation too large: …"` |
| O-3, O-4, O-6, O-10 🟡 | **open / unverified** | not probed against the working tree; O-10's property-test claim at `DESIGN.md:42-44` is untouched by the diff |

Still-open count at the working tree: **0 🔴 · 2 🟠 (O-2/L-7, L-5) · 8 🟡.** The two open 🟠 are the
ones both lenses converged on that need a design decision rather than a guard: whether `Proof.Rule`
is a pointer or a value, and whether `accepts` means "at least the plan's schema" or "exactly it".
