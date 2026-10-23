# Naur × Procida authoring pass on `examples/logsvc`, 2026-10-23

Artifact: the tutorial-by-comments example at `examples/logsvc/` before its first commit.
Pair: Naur × Procida, authoring pass (docs → reader), run blind by a read-only Max agent
against the live code, the policy package docs and `rbac.dl`, with the documented commands
run and three extra probes. Verdict as delivered: concerns. Every finding below was
resolved in the same commit that landed the example.

| # | Severity | Finding | Resolution |
|---|---|---|---|
| 1 | 🔴 | Comments called prefix matching "an exact match" when no endpoint prefixes another; `go run . tok-e210 /stopall` returned allow | Comments corrected. `decide` resolves the path against the route table first, so an unnamed path is an unknown endpoint before Shearline is asked. `Facts` refuses a matrix in which one endpoint is a prefix of another. Tests for both |
| 2 | 🟠 | "Six endpoints" in three places; there are eight | Corrected in README, package comment, rbac.yaml, OKF |
| 3 | 🟠 | Proof said to reach "down to the YAML lines"; it prints facts, and `rbac/1:19` named a file never mentioned | Reworded: a fact prints as `fact`; `rbac/1:19` is line 19 of `policy/rbac.dl` |
| 4 | 🟠 | `rbac.dl`, `denied/3` and `policy.Deny` invisible; add-a-deny and per-method had no path | Facts comment names `rbac.dl` and `Deny`; three extension recipes close the package comment |
| 5 | 🟡 | Test comment understated RW's permits | Reworded |
| 6 | 🟠 | Datalog and `name/2` arrive unglossed | README sends the reader to the module README gloss and defines arity notation |
| 7 | 🟠 | `Guard` never runs on the guided path | Labelled as off the path, with the test that exercises it named |
| 8 | 🟡 | tokens.yaml described a writer that does not exist | "In the real service …", pointing at `Guard.Reload` |
| 9 | 🟡 | Design apologetics mid-tutorial in `printMatrix` | Removed; the token list now lives in `Snapshot` |

Tension held: Naur wanted more theory in the comments, Procida wanted the tutorial to
withhold. Resolved as the pair suggested: pointers inside `Facts`, recipes at the close of
the package comment, `Guard` kept and labelled.
