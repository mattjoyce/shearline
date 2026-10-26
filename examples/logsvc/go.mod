module github.com/mattjoyce/shearline/examples/logsvc

go 1.25

require (
	github.com/mattjoyce/shearline v0.1.0
	github.com/mattjoyce/tarski v0.2.2
	gopkg.in/yaml.v3 v3.0.1
)

// The example tracks the repository it lives in, not a published release: the only tag,
// v0.1.0, predates the move to tarski. Drop this replace once a tarski-based release is tagged.
replace github.com/mattjoyce/shearline => ../..
