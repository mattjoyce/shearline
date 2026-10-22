// Package policy asks one question of a rule set: may this principal take this action
// on this resource, in this context? It owns the PARC vocabulary (principal, action,
// resource, context) and nothing about how rules are evaluated; that is package
// datalog's.
//
// A Policy is built once from standing facts and rule sets, then answers any number
// of requests. It holds no state a request can change, so Decide and Permitted are
// safe to call from many goroutines. Every failure is a deny: a non-nil error always
// comes with Allow false, and an absent allow/3 tuple is a deny with a nil error.
//
// RBAC is the one bundled rule set. A consumer extends it with its own set, most
// often to define deny/3, and says so at Load.
package policy
