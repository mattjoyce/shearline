module github.com/mattjoyce/shearline/examples/logsvc

go 1.25

require (
	github.com/mattjoyce/shearline v0.1.0
	gopkg.in/yaml.v3 v3.0.1
)

// The example tracks the repository it lives in, not the published release.
replace github.com/mattjoyce/shearline => ../..
