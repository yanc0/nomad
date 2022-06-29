package structs

import (
	"fmt"
)

// The CheckMode of a check is either Healthiness or Readiness.
type CheckMode byte

const (
	// A Healthiness check is asking a service, "are you healthy?". A service that
	// is healthy is thought to be _capable_ of serving traffic, but might not
	// want it yet.
	Healthiness CheckMode = iota

	// A Readiness check is asking a service, "do you want traffic?". A service
	// that is not ready is thought to not want traffic, even if it is passing
	// other healthiness checks.
	Readiness
)

// GetCheckMode determines whether the check is readiness or healthiness.
func GetCheckMode(c *ServiceCheck) CheckMode {
	if c != nil && c.OnUpdate == "ignore" {
		return Readiness
	}
	return Healthiness
}

// An CheckID is unique to a check.
type CheckID string

func (c CheckMode) String() string {
	switch c {
	case Readiness:
		return "readiness"
	default:
		return "healthiness"
	}
}

// A CheckQueryResult represents the outcome of a single execution of a Nomad service
// check. It records the result, the output, and when the execution took place.
//
// Knowledge of the context of the check (i.e. alloc / task) is left to the caller.
// Any check math (e.g. success_before_passing) is left to the caller.
type CheckQueryResult struct {
	ID        CheckID
	Kind      CheckMode
	Status    CheckStatus
	Output    string
	Timestamp int64
}

func (r *CheckQueryResult) String() string {
	return fmt.Sprintf("(%s %s %s %v)", r.ID, r.Kind, r.Status, r.Timestamp)
}

// A CheckStatus is resultant detected status of a check upon executing it. The
// status of a query is ternary - success, failure, or pending (not yet executed).
type CheckStatus byte

const (
	CheckSuccess CheckStatus = iota
	CheckFailure
	CheckPending
)

func (s CheckStatus) String() string {
	switch s {
	case CheckSuccess:
		return "success"
	case CheckFailure:
		return "failure"
	default:
		return "pending"
	}
}

// MakeCheckID returns an ID unique to the check.
//
// Checks of group-level services have no task.
func MakeCheckID(allocID, group, task, name string) CheckID {
	id := allocID[0:8]
	if task == "" {
		return CheckID(fmt.Sprintf("chk_%s_%s_%s", group, name, id))
	}
	return CheckID(fmt.Sprintf("chk_%s_%s_%s_%s", group, task, name, id))
}

// server only, to proxy to client
//// CheckResultsByAllocationRequest is the request object to retrieve the latest
//// nomad service check information specific to an allocation.
//type CheckResultsByAllocationRequest struct {
//	QueryOptions
//}
//
//// CheckResultsByAllocationResponse is the response object for retrieving the latest
//// nomad service check information specific to an allocation.
//type CheckResultsByAllocationResponse struct {
//	QueryMeta
//	CheckResults []*CheckQueryResult
//}
