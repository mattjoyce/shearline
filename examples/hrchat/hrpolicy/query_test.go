package hrpolicy

import (
	"context"
	"math/rand/v2"
	"os"
	"testing"
	"time"

	"github.com/mattjoyce/shearline/examples/hrchat/org"
	"github.com/mattjoyce/shearline/policy"
	"github.com/mattjoyce/tarski"
)

type matrixKey struct {
	principal, action, resource string
	bag                         int
}

func matrixKeys(m policy.Matrix) map[matrixKey]bool {
	keys := make(map[matrixKey]bool, len(m.Allows))
	for _, a := range m.Allows {
		keys[matrixKey{a.Principal, a.Action, a.Resource, a.Bag}] = true
	}
	return keys
}

func TestQuery_HRChatAgreement(t *testing.T) {
	o := org.Generate(1)
	p := loadShearline(t, o, func(_ tarski.Fact) bool { return true })
	spaces := propertySpaces(t, o)
	// #nosec G404 -- the sample is synthetic and must repeat.
	rng := rand.New(rand.NewPCG(1, 1))
	type sample struct{ resource, action, variant int }
	groups := make([][]sample, len(spaces))
	for range 2000 {
		i := rng.IntN(len(spaces))
		groups[i] = append(groups[i], sample{rng.IntN(10), rng.IntN(27), rng.IntN(48)})
	}
	for i, samples := range groups {
		s := spaces[i]
		// Keep one copy of each sampled bag in its original index order.
		indices := map[int]int{}
		var bags []map[string]tarski.Const
		for _, sample := range samples {
			_, bag := requestAt(s, sample.resource, sample.action, sample.variant)
			if _, ok := indices[bag]; !ok {
				indices[bag] = len(bags)
				bags = append(bags, s.Contexts[bag])
			}
		}
		group := s
		group.Contexts = bags
		m, err := p.Query(context.Background(), group)
		if err != nil {
			t.Fatal(err)
		}
		keys := matrixKeys(m)
		for _, sample := range samples {
			req, bag := requestAt(s, sample.resource, sample.action, sample.variant)
			assertQueryRequest(t, p, req, keys[matrixKey{req.Principal, req.Action, req.Resource, indices[bag]}])
		}
	}
	t.Log("2000 seeded requests agree across 30 Query calls")
}

func assertQueryRequest(t *testing.T, p policy.Policy, req policy.Request, allowed bool) {
	t.Helper()
	d, err := p.Decide(context.Background(), req)
	if err != nil || d.Allow != allowed {
		t.Fatalf("%s %s %s requester=%s: Query=%v Decide=%v err=%v", req.Principal, req.Action, req.Resource, req.Context["requester"].Text(), allowed, d.Allow, err)
	}
}

func TestQuery_HRChatPrincipalKey(t *testing.T) {
	o := org.Generate(1)
	p := loadShearline(t, o, func(_ tarski.Fact) bool { return true })
	for _, hr := range o.Employees {
		for _, dept := range hr.PartnerOf {
			for _, subject := range o.Employees {
				if subject.Dept != dept || subject.ID == hr.ID {
					continue
				}
				who, resource := org.Resource(hr.ID), org.Resource(subject.ID)
				bag := (Context{Channel: "web", Guardrail: "clean", MFA: true, Confirmed: true, Requester: who}).consts()
				s := policy.Space{Principals: []string{who, resource}, Actions: []string{"approve.salary", "read.salary", "read.directory"}, Resources: []string{who, resource}, Contexts: []map[string]tarski.Const{bag}}
				m, err := p.Query(context.Background(), s)
				if err != nil {
					t.Fatal(err)
				}
				keys := matrixKeys(m)
				for _, principal := range s.Principals {
					for _, action := range s.Actions {
						for _, r := range s.Resources {
							req := policy.Request{Principal: principal, Action: action, Resource: r, Context: bag}
							assertQueryRequest(t, p, req, keys[matrixKey{principal, action, r, 0}])
						}
					}
				}
				// One subject per department suffices to expose the old met/1 leak.
				break
			}
		}
	}
}

func TestQuery_HRChatMeasurement(t *testing.T) {
	if os.Getenv("SHEARLINE_QUERY_MEASURE") != "1" {
		t.Skip("set SHEARLINE_QUERY_MEASURE=1 for the full timing check")
	}
	o := org.Generate(1)
	p := loadShearline(t, o, func(_ tarski.Fact) bool { return true })
	spaces := propertySpaces(t, o)
	start := time.Now()
	var matrices []map[matrixKey]bool
	var maxSteps, maxBytes int
	for _, s := range spaces {
		m, err := p.Query(context.Background(), s)
		if err != nil {
			t.Fatal(err)
		}
		maxSteps, maxBytes = max(maxSteps, m.Steps), max(maxBytes, m.Bytes)
		matrices = append(matrices, matrixKeys(m))
	}
	queryTime := time.Since(start)
	start = time.Now()
	count, loopSteps, loopBytes := 0, 0, 0
	for i, s := range spaces {
		for r := range s.Resources {
			for a := range s.Actions {
				for variant := range 48 {
					req, bag := requestAt(s, r, a, variant)
					d, err := p.Decide(context.Background(), req)
					if err != nil || d.Allow != matrices[i][matrixKey{req.Principal, req.Action, req.Resource, bag}] {
						t.Fatalf("full-space disagreement: %+v: %v", req, err)
					}
					loopSteps, loopBytes = max(loopSteps, d.Steps), max(loopBytes, d.Bytes)
					count++
				}
			}
		}
	}
	loopTime := time.Since(start)
	ratio := float64(queryTime) / float64(loopTime)
	t.Logf("seed=1 calls=%d evaluations=%d decisions=%d Query=%s loop=%s ratio=%.6f Query Steps=%d Bytes=%d loop Steps=%d Bytes=%d", len(spaces), len(spaces)*192, count, queryTime, loopTime, ratio, maxSteps, maxBytes, loopSteps, loopBytes)
	bounds := p.Bounds()
	if count != 388800 || ratio > 0.1 || maxSteps > bounds.MaxSteps/4 || maxBytes > bounds.MaxBytes/4 {
		t.Fatal("measurement misses the card's bar; stop and record the numbers")
	}
}
