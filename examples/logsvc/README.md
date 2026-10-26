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

Its `go.mod` has `replace github.com/mattjoyce/shearline => ../..`, so the example always
builds against the checkout it sits in, never a downloaded copy. That is deliberate for now.
The only Shearline tag, v0.1.0, predates the move to tarski and does not have the API this
example uses, so there is no release it could pin instead. The `require` line still says
`v0.1.0`; Go needs a version there, and the `replace` overrides it. Once a Shearline release
that depends on tarski is tagged, drop the `replace` and require that tag.

Outside this repository, copy the pattern, not the `go.mod`: `go get
github.com/mattjoyce/shearline` once that release exists.
