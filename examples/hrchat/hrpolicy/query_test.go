package hrpolicy

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mattjoyce/shearline/examples/hrchat/catalog"
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
	t.Log("2000 seeded requests agree across 30 Query calls; single-principal Spaces cannot see a principal-axis leak")
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

// measurementShape keeps one copy of each bag and of each request 007 needs.
// Query still answers the product; the comparator is only the needed requests.
func measurementShape(t *testing.T, original policy.Space) (policy.Space, []matrixKey) {
	t.Helper()
	s := original
	s.Contexts = nil
	indices := map[string]int{}
	bagKey := func(bag map[string]tarski.Const) string {
		b, err := json.Marshal(bag)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	for _, bag := range original.Contexts {
		key := bagKey(bag)
		if _, ok := indices[key]; !ok {
			indices[key] = len(s.Contexts)
			s.Contexts = append(s.Contexts, bag)
		}
	}
	seen := map[matrixKey]bool{}
	var needed []matrixKey
	for r := range original.Resources {
		for a := range original.Actions {
			for variant := range 48 {
				req, _ := requestAt(original, r, a, variant)
				key := matrixKey{req.Principal, req.Action, req.Resource, indices[bagKey(req.Context)]}
				if !seen[key] {
					seen[key] = true
					needed = append(needed, key)
				}
			}
		}
	}
	return s, needed
}

func TestQuery_HRChatMeasurement(t *testing.T) {
	if os.Getenv("SHEARLINE_QUERY_MEASURE") != "1" {
		t.Skip("set SHEARLINE_QUERY_MEASURE=1 for the full timing check")
	}
	seed := uint64(1)
	if value := os.Getenv("SHEARLINE_QUERY_SEED"); value != "" {
		var err error
		seed, err = strconv.ParseUint(value, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
	}
	host, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	commit, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("environment Go=%s host=%s platform=%s/%s commit=%s (working tree under test)", runtime.Version(), host, runtime.GOOS, runtime.GOARCH, strings.TrimSpace(string(commit)))
	o := org.Generate(seed)
	p := loadShearline(t, o, func(_ tarski.Fact) bool { return true })
	spaces := propertySpaces(t, o)
	var elapsed [4]time.Duration // Query, needed loop, A/A loop 1, A/A loop 2
	var maxSteps, maxBytes, loopSteps, loopBytes, count, evaluations int
	for i, original := range spaces {
		s, needed := measurementShape(t, original)
		evaluations += len(s.Contexts)
		count += len(needed)
		var keys map[matrixKey]bool
		var loops [3][]bool
		order := []int{0, 1, 2, 3}
		if i%2 == 1 {
			order = []int{1, 0, 3, 2}
		}
		for _, side := range order {
			start := time.Now()
			if side == 0 {
				m, err := p.Query(context.Background(), s)
				if err != nil {
					t.Fatal(err)
				}
				keys = matrixKeys(m)
				maxSteps, maxBytes = max(maxSteps, m.MaxSteps), max(maxBytes, m.MaxBytes)
			} else {
				answers := make([]bool, len(needed))
				for j, key := range needed {
					req := policy.Request{Principal: key.principal, Action: key.action, Resource: key.resource, Context: s.Contexts[key.bag]}
					d, err := p.Decide(context.Background(), req)
					if err != nil {
						t.Fatal(err)
					}
					answers[j] = d.Allow
					loopSteps, loopBytes = max(loopSteps, d.Steps), max(loopBytes, d.Bytes)
				}
				loops[side-1] = answers
			}
			elapsed[side] += time.Since(start)
		}
		for j, key := range needed {
			for side := range loops {
				if loops[side][j] != keys[key] {
					t.Fatalf("seed=%d full-space disagreement: %+v loop=%d", seed, key, side)
				}
			}
		}
	}
	ratio := float64(elapsed[0]) / float64(elapsed[1])
	aaRatio := float64(elapsed[2]) / float64(elapsed[3])
	noise := float64(elapsed[2]-elapsed[3]) / float64((elapsed[2]+elapsed[3])/2)
	if noise < 0 {
		noise = -noise
	}
	bounds := p.Bounds()
	t.Logf("seed=%d calls=%d evaluations=%d decisions=%d Query=%s loop=%s ratio=%.6f AA1=%s AA2=%s AA_ratio=%.6f AA_relative_delta=%.6f Query MaxSteps=%d MaxBytes=%d loop Steps=%d Bytes=%d step_bound_pct=%.4f byte_bound_pct=%.4f", seed, len(spaces), evaluations, count, elapsed[0], elapsed[1], ratio, elapsed[2], elapsed[3], aaRatio, noise, maxSteps, maxBytes, loopSteps, loopBytes, 100*float64(maxSteps)/float64(bounds.MaxSteps), 100*float64(maxBytes)/float64(bounds.MaxBytes))
	if count != 375840 || evaluations != 5280 || ratio > 1 || maxSteps > bounds.MaxSteps/4 || maxBytes > bounds.MaxBytes/4 {
		t.Fatal("post hoc acceptance 12b misses the needed-work or 25% bound bar")
	}
}

func TestQuery_HRChatMultiAxisAgreement(t *testing.T) {
	o := org.Generate(1)
	p := loadShearline(t, o, func(_ tarski.Fact) bool { return true })
	// #nosec G404 -- deterministic synthetic subsets, not security randomness.
	rng := rand.New(rand.NewPCG(10, 13))
	actions := catalog.ActionNames()
	for sample := range 12 {
		// Include a manager, their report and an unrelated principal in each random
		// subset. This makes the M3 direct(_, S) mutation observable on every run.
		managers := []*org.Employee{}
		for _, e := range o.Employees {
			if e.Title == "Manager" && len(o.Reports(e.ID)) > 0 {
				managers = append(managers, e)
			}
		}
		manager := managers[rng.IntN(len(managers))]
		reports := o.Reports(manager.ID)
		report := reports[rng.IntN(len(reports))]
		other := o.Employees[rng.IntN(len(o.Employees))]
		for other.ID == manager.ID || other.ID == report.ID {
			other = o.Employees[rng.IntN(len(o.Employees))]
		}
		principals := []string{org.Resource(manager.ID), org.Resource(report.ID), org.Resource(other.ID)}
		resources := []string{org.Resource(report.ID), org.Resource(other.ID), org.Resource(manager.ID)}
		subset := []string{"approve.leave", "read.directory", actions[rng.IntN(len(actions))], actions[rng.IntN(len(actions))]}
		bags := []map[string]tarski.Const{
			(Context{Channel: "web", Guardrail: "clean", MFA: true, Confirmed: true, Requester: org.Resource(other.ID)}).consts(),
			(Context{Channel: []string{"web", "chat"}[rng.IntN(2)], Guardrail: "clean", MFA: rng.IntN(2) == 1, Confirmed: rng.IntN(2) == 1, Requester: org.Resource(report.ID)}).consts(),
		}
		s := policy.Space{Principals: principals, Actions: subset, Resources: resources, Contexts: bags}
		m, err := p.Query(context.Background(), s)
		if err != nil {
			t.Fatalf("sample=%d: %v", sample, err)
		}
		keys := matrixKeys(m)
		for bag, c := range bags {
			for _, who := range principals {
				for _, action := range subset {
					for _, resource := range resources {
						assertQueryRequest(t, p, policy.Request{Principal: who, Action: action, Resource: resource, Context: c}, keys[matrixKey{who, action, resource, bag}])
					}
				}
			}
		}
	}
	t.Log("12 deterministic multi-member principal/action/resource/bag subsets agree with Decide")
}

func TestQuery_HRChatManagerKey(t *testing.T) {
	o := org.Generate(1)
	p := loadShearline(t, o, func(_ tarski.Fact) bool { return true })
	bag := (Context{Channel: "web", Guardrail: "clean", MFA: true, Confirmed: true, Requester: "emp/E0001"}).consts()
	s := policy.Space{Principals: []string{"emp/E0004", "emp/E0001"}, Actions: catalog.ActionNames(), Resources: []string{"emp/E0005"}, Contexts: []map[string]tarski.Const{bag}}
	m, err := p.Query(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	keys := matrixKeys(m)
	for _, who := range s.Principals {
		for _, action := range s.Actions {
			assertQueryRequest(t, p, policy.Request{Principal: who, Action: action, Resource: s.Resources[0], Context: bag}, keys[matrixKey{who, action, s.Resources[0], 0}])
		}
	}
	t.Logf("M3 probe: %d allows agree with Decide", len(m.Allows))
}

func TestQuery_HRChatProofControl(t *testing.T) {
	o := org.Generate(1)
	p := loadShearline(t, o, func(_ tarski.Fact) bool { return true })
	total := 0
	for _, s := range propertySpaces(t, o)[:8] {
		m, err := p.Query(context.Background(), s)
		if err != nil {
			t.Fatal(err)
		}
		total += len(m.Allows)
	}
	if total != 23760 {
		t.Fatalf("allows=%d want 23760", total)
	}
	t.Logf("hrchat/2 first 8 property Spaces: allows=%d flagged=0", total)
}
