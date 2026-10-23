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

This directory is its own Go module so that the YAML library stays out of the engine,
which is standard library only. Its `go.mod` points at the repository it lives in.
