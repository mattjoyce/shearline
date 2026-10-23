# RED10: datalog/answer.go

Scope: snapshot 5fe3a78, assigned file only; term.go and existing answer tests read for interface seams. No previous reports inspected. No production edits. Bounded probe: `go test ./datalog -run '^TestRed10AnswerBoundaries$' -v` passes; assertions reproduce the observations below in `datalog/red10_answer_test.go`.

## Confirmed findings

### Medium: public mutation separates transmitted answer from its pinned hash

Lines 15–20, 45–54, 75–81. Answer exposes Relation and nested Tuples for mutation but caches bytes/hash once. It has no MarshalJSON method, so ordinary encoding/json serialization reads current exported fields, while Bytes()/Hash() continue describing the original fields. Len() also reads current fields. The existing ownership comment explicitly permits mutation.

Concrete trigger: decode `{"relation":"r","tuples":[[1]]}`, assign `a.Tuples[0][0] = datalog.Int(2)`, then marshal a. Bytes() still yields `{"relation":"r","tuples":[[1]]}` but json.Marshal(a) yields `{"relation":"r","tuples":[[2]]}`. Decoding the transmitted result produces a different hash. Changing Relation or appending/removing tuples has the same effect. Copying Answer also shares its tuple slices, permitting one copy to alter another copy's observable fields while retaining their old hashes.

Impact: a caller using Hash() to pin an answer and json.Marshal to send/store it can associate the old commitment with different answer contents. This requires local caller mutation; I found no external receipt consumer in the inspected production seams, so this is an API consistency defect, not a demonstrated remote signature bypass. A consistent immutable representation, or explicit snapshot-only interface/serialization behavior, would resolve the ambiguity.

### Low: decoded answers bypass ordering and relation shape invariants

Lines 61–71 versus 32–36. UnmarshalJSON only checks nonempty relation and non-nil outer tuples. It accepts reversed row order, duplicates, null rows, zero-width and mixed-width rows, and arbitrary nonempty relation names. seal() neither sorts nor validates. For example `{"relation":"r","tuples":[[2],[1]]}` and `{"relation":"r","tuples":[[1],[2]]}` decode successfully and hash differently. `{"relation":"r","tuples":[null,[],[1],[1,2],[1]]}` also succeeds, although it is not an ordinary fixed-arity set relation produced by the engine.

Impact: the decoder cannot establish that a received Answer has the sorted/set relation semantics promised by the type description. A caller comparing hashes as semantic relation identities gets false mismatches for row permutations; downstream callers assuming fixed arity can receive malformed shapes. There is no demonstrated engine ingestion of decoded answers, so no engine corruption or direct panic claimed. If the decoder is deliberately a trusted exact-wire reader, the sorting issue is a documented trust-boundary limitation rather than a higher-severity bug. Reordering/deduplicating during decode would change incoming bytes; alternatively reject noncanonical structure.

## Other boundary observations

- Standard JSON behavior accepts unknown keys and duplicate keys (last value wins); `{"relation":"r","relation":"s","tuples":[],"ignored":true}` seals as relation s. This is confirmed permissiveness, not independently a security finding: no raw-input commitment or stricter adjacent parser was demonstrated.
- Re-encoding canonicalizes whitespace/key order and scalar numeric spelling through Const.UnmarshalJSON. Hash describes canonical re-encoded content, not exact received JSON bytes. Canonical server output round-trips; arbitrary incoming wire bytes need not hash as received.
- Zero-value / externally constructed Answer has no seal: public fields and Len may be populated while Bytes() is nil and Hash() empty. This is another manifestation of the construction/mutation API mismatch, not a separate finding.

## Defended guarantees and error behavior

- Result.Relation rejects unknown names with ErrInvalid, clones the outer slice and each row, and sorts engine-produced rows before sealing. Const contains only scalar value fields, so row cloning is sufficient to isolate the frozen base.
- Bytes returns a clone. Its mutation cannot alter pinned bytes; private hash string and bytes cannot be directly modified outside the package.
- Hash is SHA-256 over exactly the cached Bytes data, prefixed `sha256:`. No cryptographic weakness identified.
- Missing/empty relation, missing/null tuples, null constants, overflowing numbers, malformed JSON, and trailing JSON values are refused. Probe confirms ordinary failed decodes leave a previously valid receiver unchanged, because decode/validation use a temporary.
- Empty non-nil tuple collections decode successfully, matching an empty engine relation. JSON scalar decoding rejects objects/arrays/null as constants through the neighboring Const decoder.
- A nil *Answer receiver on a valid input panics at assignment, as many custom unmarshaler APIs do; no realistic reachable external nil receiver path found, so not reported as a vulnerability.
- No size bound exists at this API boundary beyond encoding/json limits and available memory. Resource policy belongs to callers; no additional unbounded algorithm beyond input storage/serialization and engine-result sorting found.

## Residual uncertainty

Focused review did not inspect all downstream consumers or prescribe whether decoded relation shape should be validated against a schema. Invalid UTF-8 repair happens in the neighboring JSON/constant boundary; no new independent claim about that seam is made here.
