# Luminary review — Hickey × Armstrong (structure × runtime), After pass: code audit

Artifact: `github.com/mattjoyce/shearline` @ `009a13a` — packages `datalog/`, `policy/` (all `.go`, `rbac.dl`).
Contract: `docs/DESIGN.md`. Intent: `SPEC.md`. `docs/reviews/` deliberately not read.
Reviewer: Max (Anthropic top rung, read-only), 2026-10-22. Transcribed by the lead; nothing in the repo was modified by the reviewer.

## Evidence run

```
go test -race -shuffle=on -cover ./...
ok  github.com/mattjoyce/shearline/datalog  2.275s  coverage: 96.2% of statements
ok  github.com/mattjoyce/shearline/policy   1.571s  coverage: 92.9% of statements

go test -run xxx -bench . -benchmem ./datalog/     (Apple M1, go1.27.2)
BenchmarkEvaluate/proofs=false-8   134   8355162 ns/op   6369234 B/op   116532 allocs/op
BenchmarkEvaluate/proofs=true-8    100  11110047 ns/op   8789903 B/op   162022 allocs/op

go vet ./... && gofmt -l .   → clean
```

Severity: 🔴 must-fix · 🟠 should-fix · 🟡 consider. Counts: 🔴 1 · 🟠 4 · 🟡 11.

---

## Eye 1 — Hickey (structure at rest)

✅ The cleave is real: `datalog` knows no principal, `policy` knows no evaluation strategy (`policy/policy.go:105-133` is a `With` + `Evaluate` + one tuple lookup); `Base`, `Plan`, `Policy` are values; `With` shares untouched relations (`datalog/base.go:50-57`); request facts and standing facts are two inputs, never one mutable base.

### H1 🟠 `Answer` publishes the Base's places as if they were values
`datalog/answer.go:29-31` copies the outer `[]Tuple` and sorts it, but each inner `Tuple` is the same backing array the frozen relation holds — for a base relation, the array inside the shared `Base` every concurrent `Evaluate` reads. `Answer.Bytes()` (`answer.go:73`) returns the internal slice too. `NewBase` copies on the way in (`base.go:77`, and DESIGN:24 promises it) but nothing copies on the way out. A consumer writing `ans.Tuples[0][0] = …` rewrites the standing base for every request in flight. Unreachable through `policy` (neither `Decide` nor `Permitted` exposes an `Answer`), reachable through `datalog`'s public face, which Anam will call directly. Fix: `slices.Clone` each tuple in `Relation` (cost is bounded by MaxBytes) and return a copy from `Bytes()`, or document that `Answer` aliases.

### H2 🟡 `Proof.Rule` is a pointer into the plan, and it does not say which set it came from
`datalog/eval.go:281` `d := &derivation{rule: &r.Rule}`; `datalog/proof.go:43` hands that pointer out. `Rule.Body[i].Atom` is itself `*Atom` (`parse.go:40`). `d.Proof.Rule.Head.Name = "x"` rewrites the live plan under every later decision. DESIGN:76-77 says "never written through" — a promise, not a property. Separately, `Rule` carries `Line` but no rule-set identity; with two sets loaded (the documented extension path, `policy.go:69-73`) "line 4" is ambiguous in a receipt. Consider a `Proof.Rule` that is a value plus `RuleSet string`, or an index pair `(set, rule)` the consumer resolves.

### H3 🟡 `policy.Schema` and `rbac` are places published as values
`policy/facts.go:12-22` is a package-level `var` holding two maps; `policy/policy.go:29,41` returns the package `RuleSet` whose `Program.Rules[i].Body[j].Atom` are pointers. Any code in the process can `delete(policy.Schema.Checks, "grants")` or mutate an `RBAC()` atom and change what every later `Load` compiles. `Compile`/`NewBase` copy the maps at use (`plan.go:65-68`, `base.go:29-36`), so an already-loaded `Policy` is safe. Consider `func Schema() datalog.Schema` building a fresh value, and `Compile` deep-copying rules.

### H4 🟡 `RuleSet.Hash` and `RuleSet.Program.Text` are two sources of truth, and `Load` trusts both
`datalog/bundled.go:13-18` is a plain exported struct; `policy/policy.go:88-89` records `s.ID()` into every `Decision.RuleSets` without recomputing anything. A `RuleSet{Name:"rbac", Version:1, Hash:"sha256:…", Program: p}` built by hand (or edited after `NewRuleSet`) puts a hash in the receipt that matches neither the text nor the rules that ran. The SPEC job story (SPEC.md:30-31) is exactly that the hash in the decision is trustworthy. Consider `Load` recomputing `sha256(s.Program.Text)` and refusing a mismatch, or unexporting the fields.

### H5 🟡 `relation` braids two modes behind a bit
`datalog/eval.go:44-51`: one struct is both the frozen, fully-indexed base relation and the growing, lazily-indexed derived relation, switched by `frozen`; `add` panics on the wrong mode (`eval.go:80-82`) and `candidates` branches on it (`eval.go:97-110`). Two types (`frozen` with only `candidates`/`has`, `growing` with `add`) would make the panic unrepresentable rather than guarded. Not a bug; the shape that produced the guard.

### H6 🟡 The sticky `ev.over` is error-as-place, and it leaks one tuple (see A3)
`datalog/eval.go:178` holds the MaxBytes refusal as evaluator state and three different sites check it (`eval.go:188`, `:329`, `:359`). A3 below is the hole that pattern left.

### H7 🟡 Numeric identity is split: `-0` vs `0`
`term.go:43` canonicalises the lexeme with `FormatFloat(n,'g',-1,64)`, which renders `-0` as `"-0"` and `0` as `"0"`; `term.go:89-91` keys on the lexeme; `term.go:101-108` compares on the float, where they are equal. So `context("x", -0)` is not found by an index probe for `0` (`eval.go:394`) but `X = 0` holds. One equality for comparison, another for identity. Normalise `-0` to `0` in `Number`.

### H8 🟡 DESIGN claims evidence the tests do not carry
DESIGN.md:42-44: "the property a test holds over random fact sets: `b.With(f…)` is observationally equal to `NewBase(schema, standing…, f…)`". `datalog/base_test.go:183-210` (`TestBase_With`) is one fixed example with three facts; there is no randomised property test in the module. Also DESIGN.md:133-136 says declared-and-undefined "is how a bundled rule set exposes a hook", but `rbac.dl` declares nothing and defines `deny/3` from `denied/3` (`rbac.dl:4`); the hook mechanism is unused by the one bundled set. Doc drift, both.

---

## Eye 2 — Armstrong (behaviour under fault)

✅ Fail-closed everywhere I probed: zero `Policy` (`policy.go:110` → `base.go:65` nil-map miss → `ErrInvalid`), `Load` with no sets (`Relation("allow")` → error), cancelled ctx before work (`policy.go:107`, `eval.go:141`), empty request fields (`facts.go:40-41`), foreign base (`plan.go:244-252`); race-clean under 8×50 concurrent `Decide`+`Permitted` (`policy_test.go:295-321`); deadline at entry and every 1024 steps (`eval.go:363`); the bundled asset fails at init, not at first request (`policy.go:29-37`).

### A1 🔴 The bundled rules derive the whole world per request; the bounds were sized for one principal
`policy/rbac.dl:6-7`:
```
holds(P, R) :- role(P, R).
holds(P, R) :- holds(P, Q), inherits(Q, R).
```
Neither rule mentions `principal(P)`. `permit/3` (`rbac.dl:10`) is seeded by the request, but `holds/2` is not, so every `Decide` and `Permitted` recomputes the transitive role closure for **every principal in the standing base**, then throws it away. Same for `rbac.dl:4` `deny(P,A,R) :- denied(P,A,R)` — every denial copied per request.

`policy/policy.go:14-15` states the premise the constants were sized from: "A decision is a few joins over one principal's roles, so a megabyte of derived tuples means the grants reachable from the principal are wrong." That premise is false for the rules as written. Arithmetic from `eval.go:289-296`: a `holds` tuple of two 10-byte strings is charged 1+2+(10+2)+(10+2) = 27 bytes; `1<<20 / 27 ≈ 38,800` tuples. A deployment with more `(principal, role)` pairs than that cannot decide *anything*: every request returns `*TooLarge` wrapped as "narrow the grants reachable from this principal" (`policy.go:169`), which points the operator at the wrong thing. Below the cliff, every decision still costs O(all roles) and the `MaxSteps` budget (`1<<22`) is spent on strangers. This is the one bundled rule set and the module calls itself general-purpose (SPEC.md:11-13).

Fail-closed, so not a wrong allow — an availability cliff with a misleading diagnostic, baked into the asset the design calls "invariants not knobs" (DESIGN.md:233).

Fix (two lines, `rbac.dl`):
```
holds(P, R) :- principal(P), role(P, R).
deny(P, A, R) :- principal(P), action(A), resource(R), denied(P, A, R).
```
`Permitted` already supplies `principal` and `action` (`policy.go:142`), so it keeps working; the per-resource `deny` seed is harmless there because `resource` is empty. Prove it: a test with 50,000 `Role(fmt.Sprint("p",i), "reader")` standing facts and one `Decide` for `p1` — today it must return `ErrTooLarge`; after the fix it must allow. Add it as the standing scale test.

### A2 🟠 `Permitted` can panic, and `Load` does not refuse the policy that makes it panic
`policy/policy.go:157` `sameTuple(t[:2], …)` and `:160` `t[2].Text()` assume `permit/3`. `Load` (`policy.go:78-100`) compiles whatever sets it is given — "The first set is normally RBAC()" — and verifies nothing about `allow` or `permit`. A consumer set that defines `permit(P) :- principal(P).` without `RBAC()` loads cleanly and the first `Permitted` call dies with a slice-bounds panic (the tuple is `make(Tuple, 1)`, `eval.go:238`). The sibling failure in `Decide` is quieter: with `allow/2` or no `allow` at all, `policy.go:124-131` never matches (or `Relation` errors) and every request is denied — correct, silent, and undiagnosed until someone asks why. Both contradict `policy.go:72-73` "Any error is the consumer's to fix before serving" and SPEC.md:32-33 (refused at load, with a message). Fix: give `Plan` an accessor (`func (p Plan) Arity(name string) (int, bool)`) and have `Load` require `allow/3` and `permit/3`, returning `ErrInvalid` naming the missing one.

### A3 🟡 `MaxBytes` can be crossed with `err == nil`, by one tuple
`datalog/eval.go:299-301` sets `ev.over`; it is returned by `stratum`'s first loop (`:188-190`) and by `join` on the *next* candidate (`:329-331`). If the crossing tuple is the last candidate of the last join of a round, `join` returns `nil`, and the following round only reaches `:329` if some rule's first atom has candidates (`:328`). When the next round's `matching` is empty for every recursive rule (e.g. a constant first argument the new tuple does not match), the loop exits, `stratum` returns `nil`, and — if this was the last stratum — `Evaluate` (`:157-162`) returns `(Result, nil)` over the bound. A lower stratum is caught by the next stratum's `:188`. Overshoot is ≤ one tuple; the contract "over it, *TooLarge" (DESIGN.md:54) is what slips. Fix: `if ev.over != nil { return Result{}, ev.over }` before `eval.go:162`.

### A4 🟡 Work that no bound counts
- `datalog/eval.go:377-387` `exists` (the negation probe) iterates `matching(...)` and never calls `step`: no `MaxSteps` charge, no deadline check. A guard like `not r(_, _)` scans all of `r` on every probe; `eval.go:125` says MaxSteps "bounds the work" — not this work. Under `rbac.dl` the probe is indexed on `P` so it is bounded by one principal's `denied` facts, but a consumer rule set is not so constrained.
- `datalog/base.go:44-59` `With` is uncharged and unchecked: `Request.Context` is an unbounded map (`policy.go:51`), so a request with 10⁶ keys builds 10⁶ facts, runs 10⁶ checks and builds the index before any bound or `ctx` look. Consumer-bounded today; name it in the doc or cap it in `Decide`.

### A5 🟠 `Number` accepts `NaN`/`Inf`, and `compare` makes `NaN` equal to everything
`datalog/term.go:38-44` is `strconv.ParseFloat` with no guard; `go doc strconv.ParseFloat`: "ParseFloat recognizes the string "NaN", and the (possibly signed) strings "Inf" and "Infinity"… It ignores case." `term.go:101-108` then returns `(0, true)` for `NaN` against anything (`<` false, `>` false → `0`), so `eval.go:448-449` makes `NaN = 14` **true**, `!=` false, `<=` and `>=` true. The lexer cannot produce it (`parse.go:135-137` starts only on a digit or `-`), but the exported constructor can, and `Request.Context` is `map[string]datalog.Const` — a consumer converting an inbound `"hour: NaN"` header with `datalog.Number` hands the engine a constant that satisfies every equality. With a consumer allow rule over context (`allow(P,A,R) :- …, context("hour", H), H = 14.`), that is `Allow=true` wrongly. The bundled rules do not read context, so RBAC alone is not exposed. `MarshalJSON` (`term.go:54-59`) would also emit bare `NaN`, which `encoding/json` rejects on `seal` — a late, confusing error. Fix: in `Number`, `math.IsNaN(n) || math.IsInf(n, 0)` → `ErrSyntax`; normalise `-0` (H7).

### A6 🟠 `Tuple.key()` is ambiguous on NUL, so distinct tuples collapse
`datalog/term.go:127-134` joins `c.key()` values with `\x00`; `term.go:89-91` makes `c.key()` one kind byte plus the raw string, and Go strings may contain NUL. Hand-traced (not executed):
```
["a\x000", "b"]   → '0' 'a' NUL '0' NUL '0' 'b' NUL
["a", "\x000b"]   → '0' 'a' NUL '0' NUL '0' 'b' NUL
```
Same key. `relation.add` (`eval.go:83-88`) then drops the second as a duplicate, `has` (`eval.go:92`) answers true for a tuple that is not there, and the proofs map (`eval.go:285`) and memo (`proof.go:38`) collide the same way. Which tuple survives depends on derivation order. In `policy`, `Decide` matches by `sameTuple` on real constants (`policy.go:174-184`), so this cannot produce a wrong allow; it can make a derived tuple vanish (a wrong deny, or a wrong `Permitted` listing) when resource or principal strings carry NUL — which attacker-shaped input routinely does. Fix: length-prefix each component in `key()` (`strconv.Itoa(len(s)) + ":" + s`), which also removes the kind-byte trick's reliance on `\x00`. Prove it with a 6-line test that adds both tuples to a relation and expects `Len == 2`.

### A7 🟡 An empty context value is a wildcard prefix
`policy/facts.go:31-33` exempts `context`'s value from `nonEmptyString`, by design (`facts.go:25-26`), and `TestDecide_EmptyContextValueIsAllowed` pins it. The hazard the check exists for — "an empty prefix would match every resource" (`facts.go:27`) — reopens the moment a consumer rule writes `context("scope", S), prefix(S, R)`: a request with `scope: ""` matches every resource. Not a bug in the bundled set; a sharp edge the schema half-guards. Say so in `Load`'s doc comment next to the kind warning (`policy.go:75-77`), or let `nonEmptyString` refuse the empty *string* value and keep numbers/bools free.

### A8 🟡 `count` is not the JSON text
`eval.go:289-296` charges `len(s)+2` per string; `encoding/json` escapes `"`, `\`, control bytes and `<`, `>`, `&` (as `<` etc.), so the real `Answer` text can be larger than what was charged. "over every derived tuple's JSON text" (DESIGN.md:54, `eval.go:121`) is an estimate; say "approximately" or charge the escaped length.

### A9 🟡 Lexer reads bytes as runes
`parse.go:127` `r := rune(lx.src[lx.pos])`; `:131-134` classify it with `unicode.IsLower/IsUpper`, so UTF-8 lead bytes are judged as Latin-1 (0xC3 → `Ã` → "uppercase" → variable). Only the error path is affected (every such input is refused), but the message prints a lone byte. `utf8.DecodeRuneInString` is the one-line fix.

---

## Where the two eyes converge (highest confidence)

1. **A1 + A4 — the bounds and the rules disagree about what a request is.** Hickey: `rbac.dl` complects "the relation `holds` in general" with "what this request needs"; the engine has no incremental state across requests (by design), so a per-request rule must be per-request. Armstrong: the error kernel's two constants (`policy.go:17,22`) were derived from a workload the bundled rules do not produce, and the remedy string sends the operator the wrong way. One fix, two lines of Datalog, one scale test.
2. **A2 — refuse at `Load` what `Permitted` will crash on.** Both lenses read `policy.go:72-73` as a contract and `policy.go:157` as its breach.
3. **H1 / H2 / H3 / H4 — "value" by convention.** Hickey calls `Answer.Tuples`, `Proof.Rule`, `policy.Schema` and `RuleSet.Hash` places published as values; Armstrong calls them shared state that one consumer bug propagates into every concurrent decision. Same finding, two vocabularies: the module's immutability holds for the paths the tests run and is not enforced at the edges.
4. **A5 / A6 / H7 — constants are not yet values.** NaN breaks equality, NUL breaks identity, `-0` splits them. All three are in `term.go`, the smallest file, and the one everything above rests on.

## Where they pull apart (decide, don't average)

- **Let-it-crash vs the total function.** Armstrong would let `Permitted` panic on a malformed policy — it is a programming error, a supervisor restarts the process, and defensive checks are waste. Hickey and the DESIGN contract (`policy.go:102-104`, DESIGN.md:170-174) say `Decide`/`Permitted` are total over their inputs. The contract wins because the consumer was promised it — but that puts the check at `Load`, not in `Permitted`: refuse early, never guard late.
- **`Proof.Rule` shared vs copied.** DESIGN.md:71-77 chooses the pointer for linear size; Hickey wants a value so a receipt cannot be a handle on the plan. Decide which the receipt is *for*: if proofs are serialised and stored (SPEC.md:26-27), a value with a rule-set ID is what survives serialisation anyway and the pointer buys nothing.
- **Unseeded `holds` is the truer relation.** A Datalog purist (Hickey, on a good day) would say `holds(P,R) :- role(P,R)` is what `holds` *means* and seeding it by `principal` smuggles the query into the rule. Armstrong answers that this engine evaluates bottom-up, per request, with a byte budget, and a relation nobody reads is cost with no meaning. Here the runtime wins: the policy layer's bounds are stated per request, so the bundled rules must be per request.
- **Denied requests get no proof.** SPEC.md:26-27 wants the derivation behind a refusal; DESIGN.md:184 returns a zero `Proof` on deny because negation has no derivation in this engine. A deny-by-`deny/3` could still surface the proof of the `deny` tuple that blocked it. Intent vs contract — Matt's call.

## Unverified

- A6 (NUL collision) and A1's 38k figure are traced from the code and the byte-count in `count`, not executed; each needs a scratch test the reviewer did not write (read-only pass).
- A5's "consumer header → `Number(NaN)`" path assumes a consumer uses `datalog.Number` on untrusted text; the acceptance of `NaN` by `ParseFloat` itself is verified from its doc.
- `TestEvaluate_MaxBytes` and `TestEvaluate_MaxSteps` were not read line by line; A3 is from the evaluator's control flow alone.

**Verdict: concerns.** No wrong allow on the bundled rules; one availability cliff with a misleading remedy in the one bundled asset (A1), one panic path `Load` should have refused (A2), and three identity/equality holes in `term.go` (A5, A6, H7) that any consumer rule over context inherits.
