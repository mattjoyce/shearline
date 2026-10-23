# RED15 — datalog/builder.go

Reviewed commit `5fe3a789d230a28d59b16a03f3c6ac7753ac1a8c`. Scope: builder.go, with immediate callers and tuple identity inspected for ownership, validation and index-consumer preconditions. No production changes.

## Findings

No confirmed actionable defect in this file under current production callers. No speculative issue is promoted to a finding.

## Field and method audit / nonfindings

- Lines 9–12 (`tuples`, `set`, `idx`, `idxLen`): insertion order and deduplication agree while tuples are immutable. `idxLen` records the tuple count when cached indexes were created; every successful add increases that count, forcing a complete cache replacement on the next candidates call. Duplicate adds preserve the count and correctly preserve indexes. No removal or tuple replacement exists in current callers.
- Lines 15–27 (`newRelBuilder`, `add(t)`): constructor initializes the required map. `add` retains the supplied tuple slice, so mutating `t[0]` after adding would make the set key and indexes disagree with the row. This is a confirmed internal precondition, not a reachable caller defect: Base.group copies fact slices before rebuild; rebuild shares only already-immutable prior rows; evaluator.apply allocates a fresh head tuple each emission and never mutates it; merge shares those immutable tuples. Const has only value fields. Existing public Result.Relation clones each row before returning it.
- Line 30 (`has(t)`): lookup uses the same self-delimiting, type-prefixed Tuple.key as add, preserving duplicate identity across string/number/bool and embedded delimiters. Nil and empty tuples have the same key, consistent with being the same zero-column row; valid schema relations have positive arity.
- Lines 34–43 (`candidates(pos, c)`): cache covers each requested position separately and handles absent constants by returning nil. Returned index slices alias the cache; mutating an element would corrupt later lookups, but production consumers matching/join/exists only range over them. A retained old candidate slice remains valid when growth causes a cache replacement. The evaluator merges into the database between complete join applications; it does not require a candidate iteration to incorporate concurrent additions.
- Lines 50–59 (`seal(arity, withIndexes)`): transfers rows and set into the relation, optionally rebuilds indexes for every valid column, then clears all builder fields. It does not reuse stale lazy indexes. Clearing the builder does not clear the transferred map or backing slice. A zero-value or already-sealed builder cannot add (nil-map panic), and a second seal yields an empty relation; these violate the explicit one-use lifecycle. All production builders come from newRelBuilder and are sealed once, after their final reads/writes.
- Lines 53–55 and 63–68 (`arity`, `pos`, `tuples`): negative arity with indexes panics; pos outside an existing row panics; arity exceeding row width panics while indexing; too-small arity omits indexes. These are real behavior under invalid internal inputs, not confirmed API defects. Base.group checks each fact against schema width, schema validation checks positive arities, and compiled evaluator heads/body calls use plan arities. Empty input indexing is safe and returns an empty map. `c` needs no separate validation: it is used only as a key.
- Sealed base relations may be shared by concurrent Evaluate calls; their builder has already relinquished storage and no exposed tuple or cache slice gives public callers a write path. Builders themselves are local to one construction/evaluation and are not promised concurrent use.

## Verification

`go test ./datalog -run '^TestRelBuilder_IndexesLazilyAndSeals$' -count=1` passed. This exercises initial lazy indexing, cache invalidation after growth, duplicate rejection, and both indexed and unindexed sealing. No additional tests or production edits were necessary.
