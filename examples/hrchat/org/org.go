// Package org generates a synthetic organisation of about a thousand people and turns it
// into the standing facts the policy reads: who reports to whom, who is in which
// department, who partners which department for HR, and who holds a standing role.
//
// The generator is deterministic: one seed gives one organisation, byte for byte, so a
// decision and its proof can be reproduced. Nothing here is real; the payroll record on
// each employee is filler for the fake store the gate executes against.
package org

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"strconv"
	"strings"

	"github.com/mattjoyce/shearline/policy"
	"github.com/mattjoyce/tarski"
)

// Employee is one person: their place in the tree, their standing roles, and the record
// the HR tools read and write.
type Employee struct {
	ID        string // E0001 ...
	Name      string
	Title     string
	Dept      string
	ManagerID string // "" for the CEO
	Location  string
	Started   string   // YYYY-MM
	Roles     []string // standing roles: payroll
	PartnerOf []string // departments this person is the HR business partner for
	Record    Record
}

// Record is the fake payroll record, one value per field the tool catalogue names.
type Record struct {
	Salary      int
	Bank        string
	TFN         string
	NextOfKin   string
	Emergency   string
	Address     string
	LeaveDays   int
	Performance string
	Medical     string
	Payslip     string
	Status      string
	History     []string
}

// Org is the generated organisation. Employees is in ID order.
type Org struct {
	Seed      uint64
	Employees []*Employee
	byID      map[string]*Employee
}

// Resource is the name an employee has in the policy, as principal and as subject.
func Resource(id string) string { return "emp/" + id }

// ID is the inverse of Resource; ok is false for a name outside the employee namespace.
func ID(resource string) (id string, ok bool) {
	if !strings.HasPrefix(resource, "emp/") {
		return "", false
	}
	return strings.TrimPrefix(resource, "emp/"), true
}

// division is the shape of one top-level unit: a chief, directors under them, managers
// under those, and teams under the managers, until Size people are placed.
type division struct {
	Dept, Chief string
	Size        int
	Titles      []string
}

var divisions = []division{
	{"engineering", "Chief Technology Officer", 350, []string{"Software Engineer", "Senior Software Engineer", "Staff Engineer", "Site Reliability Engineer", "QA Engineer", "Data Engineer"}},
	{"operations", "Chief Operating Officer", 235, []string{"Operations Analyst", "Logistics Coordinator", "Support Specialist", "Facilities Officer", "Procurement Officer"}},
	{"sales", "Chief Revenue Officer", 150, []string{"Account Executive", "Sales Development Rep", "Solutions Engineer", "Account Manager"}},
	{"finance", "Chief Financial Officer", 80, []string{"Financial Analyst", "Accountant", "Treasury Analyst", "Payroll Officer"}},
	{"product", "Chief Product Officer", 70, []string{"Product Manager", "Product Designer", "UX Researcher", "Product Analyst"}},
	{"marketing", "Chief Marketing Officer", 60, []string{"Marketing Manager", "Content Specialist", "Growth Analyst", "Brand Designer"}},
	{"people", "Chief People Officer", 55, []string{"HR Business Partner", "Recruiter", "People Operations Specialist", "Learning Specialist"}},
}

// Generate builds the organisation for seed. The same seed always gives the same org.
func Generate(seed uint64) *Org {
	// #nosec G404 -- a seeded generator is the point: the data is synthetic, not secret, and must repeat.
	g := &gen{rng: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)), org: &Org{Seed: seed, byID: map[string]*Employee{}}}
	ceo := g.add("Chief Executive Officer", "executive", "")
	for _, d := range divisions {
		g.division(d, ceo.ID)
	}
	g.specialise()
	return g.org
}

type gen struct {
	rng *rand.Rand
	org *Org
}

func (g *gen) add(title, dept, manager string) *Employee {
	n := len(g.org.Employees) + 1
	e := &Employee{
		ID:        fmt.Sprintf("E%04d", n),
		Name:      firstNames[g.rng.IntN(len(firstNames))] + " " + lastNames[g.rng.IntN(len(lastNames))],
		Title:     title,
		Dept:      dept,
		ManagerID: manager,
		Location:  locations[g.rng.IntN(len(locations))],
		Started:   fmt.Sprintf("%d-%02d", 2008+g.rng.IntN(18), 1+g.rng.IntN(12)),
	}
	e.Record = g.record(e)
	g.org.Employees = append(g.org.Employees, e)
	g.org.byID[e.ID] = e
	return e
}

// division places Size people: the chief, then directors of 40 to 55 each, then managers
// of 5 to 9 each, then the individual contributors.
func (g *gen) division(d division, ceo string) {
	chief := g.add(d.Chief, d.Dept, ceo)
	remaining := d.Size - 1
	for remaining > 0 {
		dir := g.add("Director, "+strings.ToUpper(d.Dept[:1])+d.Dept[1:], d.Dept, chief.ID)
		remaining--
		budget := min(remaining, 40+g.rng.IntN(16))
		remaining -= budget
		for budget > 0 {
			mgr := g.add("Manager", d.Dept, dir.ID)
			budget--
			team := min(budget, 5+g.rng.IntN(5))
			budget -= team
			for range team {
				g.add(d.Titles[g.rng.IntN(len(d.Titles))], d.Dept, mgr.ID)
			}
		}
	}
}

// specialise hands out the standing roles and the HR partnerships: the first seven
// individual contributors in people partner one department each, and the first three
// in finance are payroll officers.
func (g *gen) specialise() {
	depts := make([]string, 0, len(divisions))
	for _, d := range divisions {
		depts = append(depts, d.Dept)
	}
	partners, payroll := 0, 0
	for _, e := range g.org.Employees {
		if !strings.HasPrefix(e.Title, "Manager") && !strings.HasPrefix(e.Title, "Director") && !strings.HasPrefix(e.Title, "Chief") {
			switch {
			case e.Dept == "people" && partners < len(depts):
				e.Title = "HR Business Partner"
				e.PartnerOf = []string{depts[partners]}
				partners++
			case e.Dept == "finance" && payroll < 3:
				e.Title = "Payroll Officer"
				e.Roles = []string{"payroll"}
				payroll++
			}
		}
	}
}

func (g *gen) record(e *Employee) Record {
	band := 70000
	switch {
	case strings.HasPrefix(e.Title, "Chief"):
		band = 320000
	case strings.HasPrefix(e.Title, "Director"):
		band = 190000
	case e.Title == "Manager":
		band = 135000
	}
	digits := func(n int) string {
		var b strings.Builder
		for range n {
			b.WriteString(strconv.Itoa(g.rng.IntN(10)))
		}
		return b.String()
	}
	medical := "none"
	if g.rng.IntN(12) == 0 {
		medical = medicals[g.rng.IntN(len(medicals))]
	}
	return Record{
		Salary:      band + g.rng.IntN(band/4),
		Bank:        "BSB " + digits(3) + "-" + digits(3) + " ACC " + digits(8),
		TFN:         digits(3) + " " + digits(3) + " " + digits(3),
		NextOfKin:   firstNames[g.rng.IntN(len(firstNames))] + " " + lastNames[g.rng.IntN(len(lastNames))] + " (spouse)",
		Emergency:   "+61 4" + digits(8),
		Address:     fmt.Sprintf("%d %s Street, %s", 1+g.rng.IntN(200), lastNames[g.rng.IntN(len(lastNames))], e.Location),
		LeaveDays:   g.rng.IntN(30),
		Performance: ratings[g.rng.IntN(len(ratings))],
		Medical:     medical,
		Payslip:     "2026-10: net " + digits(4) + ".00",
		Status:      "active",
		History:     []string{e.Started + " joined as " + e.Title},
	}
}

// Get is the employee with this ID.
func (o *Org) Get(id string) (*Employee, bool) {
	e, ok := o.byID[id]
	return e, ok
}

// Reports is the direct reports of id, in ID order.
func (o *Org) Reports(id string) []*Employee {
	var out []*Employee
	for _, e := range o.Employees {
		if e.ManagerID == id {
			out = append(out, e)
		}
	}
	return out
}

// Chain is the managers above id, nearest first, up to the CEO.
func (o *Org) Chain(id string) []string {
	var out []string
	for e, ok := o.byID[id]; ok && e.ManagerID != ""; e, ok = o.byID[e.ManagerID] {
		out = append(out, e.ManagerID)
	}
	return out
}

// Depts is every department name, sorted.
func (o *Org) Depts() []string {
	seen := map[string]bool{}
	for _, e := range o.Employees {
		seen[e.Dept] = true
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// Facts is the organisation as the policy's base relations: reports_to/2, dept/2,
// hr_partner/2, and role/2 for each standing role. The policy sorts them at Load, so
// their order here is immaterial.
func (o *Org) Facts() []tarski.Fact {
	var fs []tarski.Fact
	for _, e := range o.Employees {
		r := Resource(e.ID)
		fs = append(fs, fact("dept", r, e.Dept))
		if e.ManagerID != "" {
			fs = append(fs, fact("reports_to", r, Resource(e.ManagerID)))
		}
		for _, d := range e.PartnerOf {
			fs = append(fs, fact("hr_partner", r, d))
		}
		for _, role := range e.Roles {
			fs = append(fs, policy.Role(r, role))
		}
	}
	return fs
}

func fact(relation string, args ...string) tarski.Fact {
	t := make(tarski.Tuple, len(args))
	for i, a := range args {
		t[i] = tarski.String(a)
	}
	return tarski.Fact{Relation: relation, Tuple: t}
}

var firstNames = []string{"Aisha", "Ben", "Chloe", "Dev", "Elena", "Farid", "Grace", "Hugo", "Imogen", "Jack", "Keiko", "Liam", "Mei", "Noah", "Olu", "Priya", "Quinn", "Rosa", "Sam", "Tariq", "Uma", "Viktor", "Wren", "Xavier", "Yara", "Zane", "Amara", "Bruno", "Cara", "Dmitri", "Esme", "Felix", "Gita", "Hana", "Ivan", "Jade", "Kai", "Lena", "Marco", "Nadia", "Oscar", "Pia", "Rafael", "Sofia", "Theo", "Una", "Vera", "Will", "Xiu", "Yusuf", "Zara", "Arlo", "Bea", "Cyrus", "Dana", "Eli", "Freya", "Gus", "Hiro", "Isla", "Jonah", "Kira", "Luca", "Maya"}

var lastNames = []string{"Abbott", "Bautista", "Chen", "Dubois", "Ekwueme", "Fischer", "Garcia", "Hassan", "Ito", "Jensen", "Khan", "Larsen", "Moreau", "Nakamura", "Okafor", "Patel", "Quayle", "Rossi", "Singh", "Tanaka", "Ueda", "Varga", "Walsh", "Xu", "Yilmaz", "Zhang", "Andersen", "Bergstrom", "Costa", "Dlamini", "Evans", "Ferreira", "Gupta", "Haddad", "Ibrahim", "Jovanovic", "Kowalski", "Lindqvist", "Mendes", "Nguyen", "Osei", "Park", "Rahman", "Silva", "Torres", "Ulrich", "Vance", "Weber", "Yamamoto", "Zuniga", "Ahmed", "Brennan", "Castillo", "Doyle", "Eriksson", "Fontaine", "Grant", "Hughes", "Iyer", "Joshi", "Kim", "Lopez", "Murphy", "Novak"}

var locations = []string{"Sydney", "Melbourne", "Brisbane", "Perth", "Adelaide", "Auckland", "Singapore"}

var ratings = []string{"exceeds", "meets", "meets", "meets", "developing"}

var medicals = []string{"ergonomic workstation", "reduced hours (medical)", "hearing accommodation", "remote work (medical)"}
