# Blue 13: datalog/answer.go

Scope: snapshot 5fe3a78, production review limited to `datalog/answer.go`; contract `docs/DESIGN.md` and `SPEC.md`. No production edits. Existing ordinary answer tests consulted; no root review files, red tests, or history inspected. Probes ran as an external temporary Go program with a temporary build cache.

## Existing protections

- `Result.Relation` clones both the tuple-list and each tuple before sorting (32–36). Mutating a returned answer cannot mutate a base/result through these slices. Existing `TestAnswer_DoesNotAliasTheBase` exercises tuple replacement, inner-element mutation, and Bytes mutation.
- `Bytes` returns a fresh copy (78). Hash is SHA-256 of the bytes produced by `seal` (46–54), with a stable two-field JSON shape.
- Result answers sort tuples before sealing. Empty relations are encoded with `[]`, avoiding nil-vs-empty variation. Unknown relations wrap ErrInvalid.
- Decode uses a temporary value; JSON parsing failures and missing relation/tuples leave an existing receiver unchanged. Null constants were rejected in the probe by delegated constant decoding.
- The standard round-trip test verifies canonical server bytes survive decode with the same hash.

## Gaps and recommended tests

### 1. Mutable public fields disagree with pinned serialization (medium, concrete)

After decoding `{"relation":"r","tuples":[[1]]}`, assigning `a.Relation = "s"` makes `json.Marshal(a)` emit relation s while `a.Bytes()` still emits relation r and `a.Hash()` still authenticates r. Tuple mutation and shallow copying Answer have the same issue. The file explicitly permits holder mutation (30–31), so this is not a base-ownership breach; it is an ambiguity about which representation is the receipt. There is no MarshalJSON to make ordinary JSON transport use pinned bytes.

Test: modify Relation, an inner tuple, and the outer slice after sealing; compare MarshalJSON output, Bytes, Hash, and Len. Also copy Answer by assignment and mutate a shared tuple. Decide explicitly whether the answer is an immutable snapshot with accessors, or a mutable view whose cached receipt stays pinned; test the chosen transport contract.

### 2. Decoder does not enforce the sorted, set-shaped relation contract (medium, concrete)

At 67 only relation nonemptiness and non-nil outer tuples are checked. Probes successfully sealed each of these:

- `{"relation":"r","tuples":[[2],[1]]}` (unsorted)
- `{"relation":"r","tuples":[[1],[1]]}` (duplicates)
- `{"relation":"r","tuples":[null,[],[1,2]]}` (null and mixed-arity rows)

The first two allow different hashes for the same logical relation. The third accepts row shapes that one relation cannot produce. The decoder has no schema, so it cannot verify declared arity, but it can reject null rows and inconsistent widths (while preserving legitimate zero-arity `[[]]`). Either reject noncanonical semantic input or normalize sorting/deduplication before sealing. This is a decode-boundary gap, not evidence that evaluator-produced answers are noncanonical.

Tests: permutations must reject or converge to one hash; duplicate tuples; mixed widths; null row; valid zero-arity relation; empty relation; typed mixed constants and their ordering. Preserve the existing exact canonical round trip.

### 3. Invalid Unicode is silently repaired at decode (low/medium, concrete)

A raw invalid UTF-8 byte in relation was accepted and reserialized as U+FFFD. An unpaired surrogate string constant `"\ud800"` was also accepted and became U+FFFD. Thus malformed transport and a real replacement character can converge to one answer hash. DESIGN's string-domain protection describes lexer/base boundaries, but this public decoding boundary also admits data.

Tests: raw invalid bytes in relation and tuple strings; isolated high/low surrogates; valid surrogate pairs; real U+FFFD. Decide whether decoding permissively normalizes malformed transport or rejects it. If rejection is desired, raw UTF-8 validation alone will not detect escaped unpaired surrogates.

### 4. Error classification and malformed-object policy need explicit coverage (low)

Missing fields wrap ErrSyntax, while JSON structural errors return directly (64–68), so errors.Is(err, ErrSyntax) is not consistent across malformed answers. Unknown fields, case-insensitive field matching, and duplicate object keys inherit encoding/json behavior. These are not demonstrated contract violations because the answer decode grammar is unspecified.

Tests: invalid syntax, wrong outer type, missing/empty/null fields, duplicate keys, unknown fields, and wrong tuple types. Start each failure case with a sealed receiver and assert no state changes. Require error sentinels only after documenting the desired classification. Nil receiver behavior can follow normal Go receiver conventions.

### 5. Resource limits are absent at this API (document/test scope)

Decode builds the full tuple graph then serializes it again; Relation clones all tuples, sorts, serializes, and hashes. Bytes allocates a full copy per call. There is no context, length, tuple-count or byte-bound parameter. DESIGN explicitly bounds derived evaluation data, not incoming answer transport or all output allocation; this is not a MaxBytes bypass claim. Large base answers and untrusted decoded answers need caller-side limits or a separate bounded decoding API.

Tests/benchmarks: large empty-row arrays, long strings requiring JSON escaping, and large valid canonical inputs; measure allocations. Do not impose arbitrary library limits without specifying the API contract.

## Priorities

First specify/test the relationship among mutable fields, pinned Bytes/Hash, and ordinary json.Marshal. Next close or document decoder semantic normalization (ordering, set semantics, row shape). Then choose malformed Unicode and JSON error policies. Existing result/base isolation protections are appropriate and already covered.
