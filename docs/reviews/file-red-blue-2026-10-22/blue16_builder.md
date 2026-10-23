# Blue review: datalog/builder.go

Snapshot: 5fe3a78. Scope: builder.go, with immediate ownership, identity, join and output callers inspected. No production changes. No other review documents or red files inspected.

## Verdict

No confirmed correctness or isolation defect reachable through the supported public API was found in this file. The implementation relies on package-private ownership and arity invariants; the current callers uphold them.

## Safeguards checked

- Identity: add and has use Tuple.key; that format concatenates kind-tagged, length-delimited constants. Embedded delimiters, mixed scalar kinds and canonical numeric spellings do not cause duplicate identity collisions. Duplicate add leaves tuple length unchanged, so retaining an existing index is correct.
- Aliasing: add deliberately takes the tuple slice without copying. Base.group copies caller tuples before rebuild; evaluator.apply allocates each derived tuple. merge and rebuild share previously internal tuples but never mutate them. Result.Relation and Result.Proof clone tuples before returning public values. Const contains immutable strings and scalar fields.
- Cached indexes: candidates discards every cached position when tuple length changes. add is append-only, and duplicates do not alter stored rows. No caller changes tuples in place. A returned candidate slice is consumed only by iteration; callers do not mutate it. Reads do not overlap database growth: evaluation writes round output separately and merges between joins.
- Sealing: seal builds indexes from the final tuples, transfers the tuple slice and membership map, and clears the complete builder including lazy indexes. Base retains only the relation; evaluator seals after all joins and returns relation values. No supported caller reuses or concurrently accesses a sealed builder.
- Arity: indexAt assumes a valid argument position and consistent tuple arity. Base.group validates supplied row lengths; compiler/evaluator generate heads with compiled arity and matching passes positions from validated atoms.

## Private misuse versus reachable gaps

A direct package-internal caller can corrupt membership/index consistency by modifying a tuple passed to add, mutate candidates' returned slice, panic by adding to the cleared builder after seal, or panic through invalid positions/ragged tuples. These are real private preconditions, not evidence of a public exploit: relBuilder and indexAt are unexported and the inspected production call paths preserve those conditions. A second seal on a cleared builder yields an empty relation rather than preserving its prior content; again no caller does this.

## Resource considerations

Each lazy index costs a complete pass and per-row position storage; any subsequent growth invalidates all positions. Repeated growth interleaved with lookups can therefore rebuild indexes repeatedly. seal(true) allocates indexes for every argument position; seal(false) avoids that work. NewBase has no work/memory budget, and evaluator index construction itself does not call the step/deadline checks. Thus MaxSteps counts visited join candidates, not index-building work, and MaxBytes is explicitly approximate tuple text rather than heap usage. These are observable cost characteristics, but this file alone does not establish a violated complexity guarantee or a distinct corrective defect. They merit stress measurement when resource bounds are central.

## Validation

Existing TestRelBuilder_IndexesLazilyAndSeals explicitly covers lazy construction, growth invalidation, duplicate rejection and both sealing modes. Package test result recorded separately by the reviewing agent. No additional tests or production edits were needed.

Targeted validation passed: `go test ./datalog -run '^TestRelBuilder_IndexesLazilyAndSeals$' -count=1` with a writable temporary GOCACHE. The package-wide run failed on existing parallel-review tests outside builder scope; no such test files were opened.
