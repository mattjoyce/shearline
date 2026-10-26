# logsvc: embedding Shearline in a service

A log store with eight endpoints, four roles and a handful of tokens. The permission matrix
lives in two YAML files. One function turns them into Shearline facts. The rest is reading
files and printing proofs.

If Datalog is new to you, read the two-sentence gloss under "What it is" in the module README
first. One notation to know: `inherits/2` names a relation with two columns.

Start with the package comment in `matrix.go`; it gives the reading order. Then:

```
go run .                    # every token and what it may call
go run . tok-0b44 /stop     # one allow, with its proof
go run . tok-9c1e /stop     # one deny, with the reason
go test ./...
```

This directory is its own Go module so that the YAML library stays out of the root module,
whose only dependency is the [tarski](https://github.com/mattjoyce/tarski) engine.

## Which Shearline it builds against

Its `go.mod` requires the released `github.com/mattjoyce/shearline v0.2.0`, the first tag
that depends on tarski, so the example builds the same way inside and outside this repository.
To test an unreleased change to the root module against the example, add a temporary
`replace github.com/mattjoyce/shearline => ../..` to its `go.mod`, and take it out before
committing.

Outside this repository, copy the `go.mod` as it stands: `go get github.com/mattjoyce/shearline@v0.2.0`.
