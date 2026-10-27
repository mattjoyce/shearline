package hrpolicy

import "github.com/mattjoyce/tarski"

// The entitlement data: which actions each path carries. This is policy, reviewed like
// code; the rules in hrchat.dl say how the paths combine, and this says what each one
// holds. An action absent from every list is one nobody can take.

// selfMay is what anyone may do to their own record.
var selfMay = []string{
	"read.directory", "read.profile", "read.history", "read.leave", "read.nok", "read.emergency",
	"read.address", "read.bank", "read.tfn", "read.salary", "read.payslip", "read.performance",
	"read.medical",
	"write.nok", "write.emergency", "write.address", "write.leave", "write.bank", "write.tfn",
	"write.medical",
}

// managerMay is what a direct manager may do to a direct report. Nobody above the
// direct manager holds any of it; wider visibility is a role, or a question to HR.
var managerMay = []string{
	"read.profile", "read.history", "read.leave", "read.emergency", "read.performance", "read.salary",
	"approve.leave", "write.performance", "request.salary",
}

// hrMay is what the HR business partner may do to a member of their department. Bank
// and tax file number are payroll's, not HR's.
var hrMay = []string{
	"read.profile", "read.history", "read.leave", "read.nok", "read.emergency", "read.address",
	"read.salary", "read.payslip", "read.performance", "read.medical",
	"write.profile", "write.address", "write.leave", "approve.leave", "write.performance",
	"write.medical", "approve.salary", "write.salary", "write.status",
}

// roleMay is what a standing role may do to every employee record.
var roleMay = map[string][]string{
	"everyone": {"read.directory"},
	"payroll":  {"read.bank", "write.bank", "read.tfn", "write.tfn", "read.payslip", "read.salary"},
}

// alsoNeeds is an attestation an action needs beyond what its level implies.
var alsoNeeds = [][2]string{{"approve.salary", "four_eyes"}}

// channels is where a request may come from. The chatbot is "chat"; the HR web app is
// "web". The same policy serves both; the chat channel carries less.
var channels = []string{"chat", "web"}

// channelForbids is what a channel never carries, whoever asks. A salary set, a
// termination and a tax file number change are not things a conversation does.
var channelForbids = [][2]string{
	{"chat", "write.salary"}, {"chat", "write.status"}, {"chat", "write.tfn"},
}

// entitlementFacts is the data above as base relations.
func entitlementFacts() []tarski.Fact {
	var fs []tarski.Fact
	add := func(rel string, args ...string) {
		t := make(tarski.Tuple, len(args))
		for i, a := range args {
			t[i] = tarski.String(a)
		}
		fs = append(fs, tarski.Fact{Relation: rel, Tuple: t})
	}
	for _, a := range selfMay {
		add("self_may", a)
	}
	for _, a := range managerMay {
		add("manager_may", a)
	}
	for _, a := range hrMay {
		add("hr_may", a)
	}
	for _, role := range []string{"everyone", "payroll"} {
		for _, a := range roleMay[role] {
			add("role_may", role, a)
		}
	}
	for _, n := range alsoNeeds {
		add("also_needs", n[0], n[1])
	}
	for _, c := range channels {
		add("channel", c)
	}
	for _, f := range channelForbids {
		add("channel_forbids", f[0], f[1])
	}
	return fs
}
