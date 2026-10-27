// Package catalog is the HR chatbot's tool surface: every tool the MCP server exposes,
// the action each one is to the policy, and the sensitivity of that action.
//
// A tool is what the model calls. An action is what the policy decides. Several tools
// may be one action (get_leave_balance and list_leave_requests are both read.leave),
// and the policy never sees a tool name: it sees "<verb>.<field>" and a level.
package catalog

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mattjoyce/tarski"
)

// Level is how sensitive an action is. Low needs no attestation beyond a clean turn on a
// known channel; High needs MFA, and for a write-class verb a confirmation too.
type Level string

// The two levels. Reading employment history is Low; reading bank details is High.
// Writing next of kin is Low; writing bank details is High.
const (
	Low  Level = "low"
	High Level = "high"
)

// Action is "<verb>.<field>": what is done, to which part of the record.
type Action struct {
	Name  string
	Verb  string // read, write, approve, request
	Field string
	Level Level
}

// Tool is one MCP tool. Every tool takes employee_id, the subject of the call; writes
// take a value as well, and approvals a request id.
type Tool struct {
	Name        string
	Description string
	Action      string
	Args        []Arg
}

// Arg is one tool input.
type Arg struct {
	Name, Type, Doc string
	Required        bool
}

var subject = Arg{"employee_id", "string", "the employee the call is about, e.g. E0042", true}
var value = Arg{"value", "string", "the new value", true}
var request = Arg{"request_id", "string", "the leave or pay-change request", true}

// writeVerbs are the verbs that change state. approve and request are writes with a
// narrower meaning, so a high action with one of them needs a confirmation like write.
var writeVerbs = []string{"write", "approve", "request"}

// Actions is every action the tools map to, keyed by name.
var Actions = index([]Action{
	{"read.directory", "read", "directory", Low},
	{"read.profile", "read", "profile", Low},
	{"read.history", "read", "history", Low},
	{"read.leave", "read", "leave", Low},
	{"read.nok", "read", "nok", Low},
	{"read.emergency", "read", "emergency", Low},
	{"read.address", "read", "address", Low},
	{"read.bank", "read", "bank", High},
	{"read.tfn", "read", "tfn", High},
	{"read.salary", "read", "salary", High},
	{"read.payslip", "read", "payslip", High},
	{"read.performance", "read", "performance", High},
	{"read.medical", "read", "medical", High},
	{"write.nok", "write", "nok", Low},
	{"write.emergency", "write", "emergency", Low},
	{"write.address", "write", "address", Low},
	{"write.leave", "write", "leave", Low},
	{"approve.leave", "approve", "leave", Low},
	{"write.profile", "write", "profile", Low},
	{"write.bank", "write", "bank", High},
	{"write.tfn", "write", "tfn", High},
	{"write.performance", "write", "performance", High},
	{"request.salary", "request", "salary", High},
	{"approve.salary", "approve", "salary", High},
	{"write.salary", "write", "salary", High},
	{"write.medical", "write", "medical", High},
	{"write.status", "write", "status", High},
})

// Tools is the manifest, in the order the server lists them.
var Tools = []Tool{
	// Read, low.
	{"get_directory_entry", "Name, title, department, manager and location of an employee.", "read.directory", []Arg{subject}},
	{"get_profile", "An employee's profile: title, department, location, start date.", "read.profile", []Arg{subject}},
	{"get_employment_history", "An employee's positions and moves, oldest first.", "read.history", []Arg{subject}},
	{"get_leave_balance", "Days of leave an employee has available.", "read.leave", []Arg{subject}},
	{"list_leave_requests", "An employee's pending and past leave requests.", "read.leave", []Arg{subject}},
	{"get_next_of_kin", "An employee's recorded next of kin.", "read.nok", []Arg{subject}},
	{"get_emergency_contact", "An employee's emergency contact number.", "read.emergency", []Arg{subject}},
	{"get_postal_address", "An employee's postal address.", "read.address", []Arg{subject}},
	// Read, high.
	{"get_bank_details", "The account an employee's pay goes to.", "read.bank", []Arg{subject}},
	{"get_tax_file_number", "An employee's tax file number.", "read.tfn", []Arg{subject}},
	{"get_salary", "An employee's current base salary.", "read.salary", []Arg{subject}},
	{"get_payslip", "An employee's most recent payslip.", "read.payslip", []Arg{subject}},
	{"get_performance_review", "An employee's latest performance rating and notes.", "read.performance", []Arg{subject}},
	{"get_medical_accommodations", "Workplace accommodations recorded for an employee.", "read.medical", []Arg{subject}},
	// Write, low.
	{"update_next_of_kin", "Set an employee's next of kin.", "write.nok", []Arg{subject, value}},
	{"update_emergency_contact", "Set an employee's emergency contact number.", "write.emergency", []Arg{subject, value}},
	{"update_postal_address", "Set an employee's postal address.", "write.address", []Arg{subject, value}},
	{"submit_leave_request", "Lodge a leave request for an employee (value: dates).", "write.leave", []Arg{subject, value}},
	{"cancel_leave_request", "Withdraw a pending leave request.", "write.leave", []Arg{subject, request}},
	{"approve_leave_request", "Approve a pending leave request.", "approve.leave", []Arg{subject, request}},
	{"update_job_title", "Set an employee's job title.", "write.profile", []Arg{subject, value}},
	// Write, high.
	{"update_bank_details", "Change the account an employee's pay goes to.", "write.bank", []Arg{subject, value}},
	{"update_tax_file_number", "Change an employee's tax file number.", "write.tfn", []Arg{subject, value}},
	{"record_performance_rating", "Record a performance rating for an employee.", "write.performance", []Arg{subject, value}},
	{"request_pay_change", "Raise a pay change for an employee, for HR approval.", "request.salary", []Arg{subject, value}},
	{"approve_pay_change", "Approve a raised pay change.", "approve.salary", []Arg{subject, request}},
	{"set_salary", "Set an employee's base salary directly.", "write.salary", []Arg{subject, value}},
	{"update_medical_accommodations", "Record or change workplace accommodations.", "write.medical", []Arg{subject, value}},
	{"terminate_employment", "End an employee's employment.", "write.status", []Arg{subject, value}},
}

func index(as []Action) map[string]Action {
	m := make(map[string]Action, len(as))
	for _, a := range as {
		m[a.Name] = a
	}
	return m
}

// Lookup is the tool by name.
func Lookup(name string) (Tool, bool) {
	for _, t := range Tools {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}

// ActionOf is the action a tool is to the policy.
func ActionOf(tool string) (Action, bool) {
	t, ok := Lookup(tool)
	if !ok {
		return Action{}, false
	}
	a, ok := Actions[t.Action]
	return a, ok
}

// Check refuses a catalogue whose tools name an action that does not exist.
func Check() error {
	for _, t := range Tools {
		if _, ok := Actions[t.Action]; !ok {
			return fmt.Errorf("catalog: tool %s names unknown action %s", t.Name, t.Action)
		}
	}
	return nil
}

// Facts is the catalogue as base relations: verb/2, level/2 and writes/1.
func Facts() []tarski.Fact {
	var fs []tarski.Fact
	for _, v := range writeVerbs {
		fs = append(fs, fact("writes", v))
	}
	for _, name := range ActionNames() {
		a := Actions[name]
		fs = append(fs, fact("verb", a.Name, a.Verb), fact("level", a.Name, string(a.Level)))
	}
	return fs
}

// ActionNames is every action name, sorted.
func ActionNames() []string {
	out := make([]string, 0, len(Actions))
	for n := range Actions {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Manifest is the tool list in the shape an MCP tools/list reply carries, with the
// action and level beside each so a reader can see what the policy will be asked.
func Manifest() ([]byte, error) {
	type prop struct {
		Type        string `json:"type"`
		Description string `json:"description"`
	}
	type schema struct {
		Type       string          `json:"type"`
		Properties map[string]prop `json:"properties"`
		Required   []string        `json:"required"`
	}
	type entry struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Action      string `json:"x-action"`
		Level       Level  `json:"x-level"`
		InputSchema schema `json:"inputSchema"`
	}
	var out []entry
	for _, t := range Tools {
		s := schema{Type: "object", Properties: map[string]prop{}}
		for _, a := range t.Args {
			s.Properties[a.Name] = prop{a.Type, a.Doc}
			if a.Required {
				s.Required = append(s.Required, a.Name)
			}
		}
		out = append(out, entry{t.Name, t.Description, t.Action, Actions[t.Action].Level, s})
	}
	return json.MarshalIndent(map[string]any{"tools": out}, "", "  ")
}

func fact(relation string, args ...string) tarski.Fact {
	t := make(tarski.Tuple, len(args))
	for i, a := range args {
		t[i] = tarski.String(a)
	}
	return tarski.Fact{Relation: relation, Tuple: t}
}
