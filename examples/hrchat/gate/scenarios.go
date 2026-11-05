package gate

import (
	"strings"

	"github.com/mattjoyce/shearline/examples/hrchat/org"
)

// Cast is the people the scenarios need, found in a generated organisation.
type Cast struct {
	CEO, CFO, CTO string
	Director      string // a director in engineering
	Manager       string // a manager under Director
	IC            string // a report of Manager
	Peer          string // another report of Manager
	Payroll       string // a payroll officer, under CFO
	HRBPEng       string // the HR partner for engineering
	HRBPPeople    string // the HR partner for people
}

// FindCast picks the cast from an organisation.
func FindCast(o *org.Org) Cast {
	var c Cast
	c.CEO = o.Employees[0].ID
	for _, e := range o.Employees {
		switch {
		case e.Dept == "finance" && strings.HasPrefix(e.Title, "Chief"):
			c.CFO = e.ID
		case e.Dept == "engineering" && strings.HasPrefix(e.Title, "Chief"):
			c.CTO = e.ID
		case c.Payroll == "" && len(e.Roles) > 0 && e.Roles[0] == "payroll":
			c.Payroll = e.ID
		case len(e.PartnerOf) > 0 && e.PartnerOf[0] == "engineering":
			c.HRBPEng = e.ID
		case len(e.PartnerOf) > 0 && e.PartnerOf[0] == "people":
			c.HRBPPeople = e.ID
		}
	}
	for _, e := range o.Employees {
		if e.Dept != "engineering" || e.Title == "Manager" || strings.HasPrefix(e.Title, "Director") || strings.HasPrefix(e.Title, "Chief") {
			continue
		}
		mgr, _ := o.Get(e.ManagerID)
		if reports := o.Reports(mgr.ID); len(reports) >= 2 {
			c.IC, c.Peer = reports[0].ID, reports[1].ID
			c.Manager = mgr.ID
			c.Director = mgr.ManagerID
			break
		}
	}
	return c
}

// Scenario is one call the gate should allow or refuse, and why.
type Scenario struct {
	Name    string
	Session Session
	Call    Call
	Attest  Attestation
	Allow   bool
	Reason  string // a substring the refusal must contain, when refused
}

func clean() Attestation { return Attestation{Guardrail: "clean"} }

func confirmed() Attestation { return Attestation{Guardrail: "clean", Confirmed: true} }

func call(tool, subject string, kv ...string) Call {
	args := map[string]string{"employee_id": subject}
	for i := 0; i+1 < len(kv); i += 2 {
		args[kv[i]] = kv[i+1]
	}
	return Call{Tool: tool, Args: args}
}

func chat(id string, mfa bool) Session { return Session{EmployeeID: id, Channel: "chat", MFA: mfa} }

func web(id string, mfa bool) Session { return Session{EmployeeID: id, Channel: "web", MFA: mfa} }

// Scenarios is the matrix the gate is held to, as a test and as a demonstration. Each
// row is a thing a chatbot user, or a model acting for one, might try.
func Scenarios(c Cast) []Scenario {
	return []Scenario{
		// The self path.
		{"employee reads own leave balance", chat(c.IC, false), call("get_leave_balance", c.IC), clean(), true, ""},
		{"employee reads own bank details without MFA", chat(c.IC, false), call("get_bank_details", c.IC), clean(), false, `"mfa")`},
		{"employee reads own bank details with MFA", chat(c.IC, true), call("get_bank_details", c.IC), clean(), true, ""},
		{"employee updates own next of kin", chat(c.IC, false), call("update_next_of_kin", c.IC, "value", "Sam Lee (partner)"), clean(), true, ""},
		{"employee changes own bank details, MFA but no confirmation", chat(c.IC, true), call("update_bank_details", c.IC, "value", "BSB 000-000 ACC 1"), clean(), false, `"confirmed")`},
		{"employee changes own bank details, MFA and confirmed", chat(c.IC, true), call("update_bank_details", c.IC, "value", "BSB 000-000 ACC 1"), confirmed(), true, ""},
		{"employee changes own tax file number over chat", chat(c.IC, true), call("update_tax_file_number", c.IC, "value", "000 000 000"), confirmed(), false, "channel_forbids"},
		{"employee approves own leave", chat(c.IC, false), call("approve_leave_request", c.IC, "request_id", "L1"), clean(), false, `no path: stopped at self_may("approve.leave")`},
		// The jailbreak guardrail as an attestation.
		{"flagged turn: employee reads own leave balance", chat(c.IC, false), call("get_leave_balance", c.IC), Attestation{Guardrail: "flagged"}, false, `context("guardrail", "clean")`},
		{"guardrail verdict missing: nothing is cleared", chat(c.IC, true), call("get_leave_balance", c.IC), Attestation{}, false, `context("guardrail", "clean")`},
		// Peers and strangers.
		{"employee reads a peer's leave balance", chat(c.IC, false), call("get_leave_balance", c.Peer), clean(), false, "stopped at"},
		{"employee reads the CEO's directory entry", chat(c.IC, false), call("get_directory_entry", c.CEO), clean(), true, ""},
		{"employee reads the CEO's salary with MFA", chat(c.IC, true), call("get_salary", c.CEO), clean(), false, "stopped at"},
		// Managers, direct and skip-level.
		{"manager approves a direct report's leave", chat(c.Manager, false), call("approve_leave_request", c.IC, "request_id", "L1"), clean(), true, ""},
		{"manager reads a direct report's salary with MFA", chat(c.Manager, true), call("get_salary", c.IC), clean(), true, ""},
		{"manager reads a direct report's bank details with MFA", chat(c.Manager, true), call("get_bank_details", c.IC), clean(), false, "stopped at"},
		{"director reads a skip-level report's history", chat(c.Director, false), call("get_employment_history", c.IC), clean(), false, "no path"},
		{"director reads a skip-level report's salary with MFA", chat(c.Director, true), call("get_salary", c.IC), clean(), false, "stopped at"},
		{"manager raises a pay change for a direct report", chat(c.Manager, true), call("request_pay_change", c.IC, "value", "+5%"), confirmed(), true, ""},
		{"manager sets a direct report's salary over chat", chat(c.Manager, true), call("set_salary", c.IC, "value", "99000"), confirmed(), false, "stopped at"},
		{"manager's 30th high read this hour", chat(c.Manager, true), call("get_salary", c.IC), Attestation{Guardrail: "clean", HighReads: 30}, false, `"velocity")`},
		{"manager reads their own manager's performance review", chat(c.Manager, true), call("get_performance_review", c.Director), clean(), false, "no path"},
		// HR partners and four eyes.
		{"HR partner reads an engineer's medical accommodations with MFA", chat(c.HRBPEng, true), call("get_medical_accommodations", c.IC), clean(), true, ""},
		{"HR partner reads an engineer's bank details with MFA", chat(c.HRBPEng, true), call("get_bank_details", c.IC), clean(), false, "stopped at"},
		{"HR partner approves a pay change another manager raised", chat(c.HRBPEng, true), call("approve_pay_change", c.IC, "request_id", "P1"), Attestation{Guardrail: "clean", Confirmed: true, Requester: c.Manager}, true, ""},
		{"HR partner approves a pay change they raised themselves", chat(c.HRBPEng, true), call("approve_pay_change", c.IC, "request_id", "P1"), Attestation{Guardrail: "clean", Confirmed: true, Requester: c.HRBPEng}, false, `"four_eyes")`},
		{"HR partner approves a pay change with no requester recorded", chat(c.HRBPEng, true), call("approve_pay_change", c.IC, "request_id", "P1"), confirmed(), false, `"four_eyes")`},
		{"HR partner terminates an engineer over chat", chat(c.HRBPEng, true), call("terminate_employment", c.IC, "value", "resigned"), confirmed(), false, "channel_forbids"},
		{"HR partner terminates an engineer in the web app", web(c.HRBPEng, true), call("terminate_employment", c.IC, "value", "resigned"), confirmed(), true, ""},
		{"HR partner for people updates their own job title", chat(c.HRBPPeople, false), call("update_job_title", c.HRBPPeople, "value", "Head of Everything"), clean(), false, "not self_may"},
		// Payroll.
		{"payroll officer reads an engineer's bank details with MFA", chat(c.Payroll, true), call("get_bank_details", c.IC), clean(), true, ""},
		{"payroll officer changes an engineer's bank details, confirmed", chat(c.Payroll, true), call("update_bank_details", c.IC, "value", "BSB 111-111 ACC 2"), confirmed(), true, ""},
		{"payroll officer reads a second bank account", chat(c.Payroll, true), call("get_bank_details", c.Peer), clean(), true, ""},
		{"payroll officer reads a third bank account (the audit sees the fan-out)", chat(c.Payroll, true), call("get_bank_details", c.Manager), clean(), true, ""},
		{"payroll officer reads the CFO's bank details (CFO is above them)", chat(c.Payroll, true), call("get_bank_details", c.CFO), clean(), false, "up(P, M)"},
		{"payroll officer reads an engineer's medical accommodations", chat(c.Payroll, true), call("get_medical_accommodations", c.IC), clean(), false, "stopped at"},
		// The model's own mistakes.
		{"model invents a tool", chat(c.IC, false), Call{Tool: "export_all_records", Args: map[string]string{"employee_id": c.IC}}, clean(), false, "unknown tool"},
		{"model invents an employee", chat(c.IC, false), call("get_leave_balance", "E9999"), clean(), false, "unknown employee"},
		{"model omits the employee", chat(c.IC, false), Call{Tool: "get_leave_balance", Args: map[string]string{}}, clean(), false, "unknown employee"},
	}
}
