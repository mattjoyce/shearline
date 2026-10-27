package gate

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mattjoyce/shearline/examples/hrchat/catalog"
	"github.com/mattjoyce/shearline/examples/hrchat/org"
)

// Store is the fake payroll system: the org's records, read and written by field. It
// trusts its caller completely, which is the point: the gate is the only caller, and
// the policy has already decided.
type Store struct {
	org    *org.Org
	Events []string // writes, approvals and requests, in order
}

// NewStore holds the org's records.
func NewStore(o *org.Org) *Store { return &Store{org: o} }

// Exec runs a tool against the subject's record and returns the reply.
func (s *Store) Exec(t catalog.Tool, subject string, args map[string]string) string {
	e, _ := s.org.Get(subject)
	a := catalog.Actions[t.Action]
	r := &e.Record
	switch a.Verb {
	case "read":
		return s.read(e, a.Field)
	case "write":
		return s.write(e, a.Field, args["value"], t.Name)
	case "approve", "request":
		ev := fmt.Sprintf("%s %s on %s (%s)", t.Name, a.Field, subject, orDash(args["request_id"], args["value"]))
		s.Events = append(s.Events, ev)
		r.History = append(r.History, ev)
		return "recorded: " + ev
	}
	return "no handler"
}

func (s *Store) read(e *org.Employee, field string) string {
	r := e.Record
	switch field {
	case "directory":
		return fmt.Sprintf("%s, %s, %s, reports to %s, %s", e.Name, e.Title, e.Dept, orDash(e.ManagerID, ""), e.Location)
	case "profile":
		return fmt.Sprintf("%s: %s in %s, %s, since %s", e.Name, e.Title, e.Dept, e.Location, e.Started)
	case "history":
		return strings.Join(r.History, "; ")
	case "leave":
		return fmt.Sprintf("%d days available", r.LeaveDays)
	case "nok":
		return r.NextOfKin
	case "emergency":
		return r.Emergency
	case "address":
		return r.Address
	case "bank":
		return r.Bank
	case "tfn":
		return r.TFN
	case "salary":
		return fmt.Sprintf("AUD %d", r.Salary)
	case "payslip":
		return r.Payslip
	case "performance":
		return r.Performance
	case "medical":
		return r.Medical
	}
	return "no such field"
}

func (s *Store) write(e *org.Employee, field, value, tool string) string {
	r := &e.Record
	switch field {
	case "nok":
		r.NextOfKin = value
	case "emergency":
		r.Emergency = value
	case "address":
		r.Address = value
	case "leave":
		r.History = append(r.History, tool+": "+value)
	case "profile":
		e.Title = value
	case "bank":
		r.Bank = value
	case "tfn":
		r.TFN = value
	case "performance":
		r.Performance = value
	case "salary":
		n, err := strconv.Atoi(value)
		if err != nil {
			return "refused by the store: salary must be a whole number"
		}
		r.Salary = n
	case "medical":
		r.Medical = value
	case "status":
		r.Status = value
	}
	ev := fmt.Sprintf("%s on %s = %s", tool, e.ID, value)
	s.Events = append(s.Events, ev)
	return "updated: " + ev
}

func orDash(a, b string) string {
	if a != "" {
		return a
	}
	if b != "" {
		return b
	}
	return "-"
}
