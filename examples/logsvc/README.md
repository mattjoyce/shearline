# logsvc: embedding Shearline in a service

A log store with eight endpoints, four roles and four tokens. The permission matrix lives in
two YAML files, as a product owner would write it. One function turns them into Shearline
facts. The rest is reading files, failing closed, and printing proofs.

## Why this example is here

It is the smallest complete embedding, and the one to read first. It answers the questions
every service asks before the interesting ones:

- **Where does the policy come from?** From files the service owns. Shearline reads no file
  and knows no YAML; the service turns its own shapes into `Role`, `Inherits` and `Grant`
  facts in one function (`Facts`, in `matrix.go`).
- **What if the files are wrong?** Nothing is served. The YAML decoder refuses an unknown
  key, `Facts` refuses a role one file uses and the other does not define, and
  `policy.Load` refuses a malformed fact. A typo is an error at start, not a role that
  silently grants nothing.
- **What if they change while running?** `Guard.Reload` (`load.go`) swaps a new policy in
  atomically, and a bad reload keeps the old one.
- **What does a caller see?** An allow comes with its proof, naming the inheritance that
  carried the grant. A deny comes with the reason.

If Datalog is new to you, read the two-sentence gloss under "What it is" in the module README
first. One notation to know: `inherits/2` names a relation with two columns.

## Run it

From this directory:

```
go run .                    # every token and what it may call
go run . tok-0b44 /stop     # one allow, with its proof
go run . tok-9c1e /stop     # one deny, with the reason
go test ./...
```

## The shape

The package comment in `matrix.go` gives the reading order. In short:

| File | What it is |
|---|---|
| `rbac.yaml` | the permission matrix: roles, what each inherits, which endpoints each grants |
| `tokens.yaml` | which token holds which roles |
| `matrix.go` | the YAML shapes, and `Facts`, the one function that turns them into Shearline facts |
| `load.go` | reading the files, the three layers that fail closed, and `Guard` for run-time swaps |
| `main.go` | the command: the whole matrix, or one decision |
| `logsvc_test.go` | the matrix pinned as a table, plus proofs, refusals, bad files and a bad reload |

An endpoint is a resource and a grant is a prefix, so each endpoint is written once, on the
lowest role that has it, and inheritance carries it upward. The service asks about the route
its router matched, never the raw path, so an unknown path never becomes a question of
permission.

Three extensions show whether the shape holds. A new role is a YAML entry and no Go. A denial
for one token on one endpoint is a `policy.Deny` fact, and a refusal names it. Per-method
permissions make each grant a method and endpoint pair.

## Which Shearline it builds against

This directory is its own Go module so that the YAML library stays out of the root module,
whose only dependency is the [tarski](https://github.com/mattjoyce/tarski) engine. Its
`go.mod` requires the released `github.com/mattjoyce/shearline v0.2.0`, the first tag that
depends on tarski, so the example builds the same way inside and outside this repository.
To test an unreleased change to the root module against the example, add a temporary
`replace github.com/mattjoyce/shearline => ../..` to its `go.mod`, and take it out before
committing.

Outside this repository, copy the `go.mod` as it stands: `go get github.com/mattjoyce/shearline@v0.2.0`.
