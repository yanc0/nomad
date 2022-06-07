package checks

import (
	"crypto/md5"
	"fmt"
)

// An ID is unique to a check.
type ID string

// MakeID returns an ID unique to the check.
//
// Checks of group-level services have no task.
func MakeID(allocID, group, task, name string) ID {
	sum := md5.New()
	_, _ = sum.Write([]byte(allocID))
	_, _ = sum.Write([]byte(group))
	_, _ = sum.Write([]byte(task))
	_, _ = sum.Write([]byte(name))
	h := sum.Sum(nil)
	return ID(fmt.Sprintf("%x", h))
}

// The Kind of a check is either Healthiness or Readiness.
type Kind byte

const (
	// A Healthiness check is asking a service, "are you healthy?". A service that
	// is healthy is thought to be _capable_ of serving traffic, but might not
	// want it yet.
	Healthiness Kind = iota

	// A Readiness check is asking a service, "do you want traffic?". A service
	// that is not ready is thought to not want traffic, even if it is passing
	// other healthiness checks.
	Readiness
)

func (k Kind) String() string {
	switch k {
	case Readiness:
		return "readiness"
	default:
		return "healthiness"
	}
}

// A Result is the immediate detected state of a check after executing it. A result
// of a query is ternary - success, failure, or pending (not yet executed).
type Result byte

const (
	Success Result = iota
	Failure
	Pending
)

func (r Result) String() string {
	switch r {
	case Success:
		return "success"
	case Failure:
		return "failure"
	default:
		return "pending"
	}
}

// A QueryResult represents the outcome of a single execution of a Nomad service
// check. It records the result, the output, and when the execution took place.
//
// Knowledge of the context of the check (i.e. alloc / task) is left to the caller.
// Any check math (e.g. success_before_passing) is left to the caller.
type QueryResult struct {
	ID        ID
	Kind      Kind
	Result    Result
	Output    string
	Timestamp int64
}

// Stub creates a temporary QueryResult for the check of ID in the Pending state
// so we can represent the status of not being checked yet.
func Stub(id ID, kind Kind, now int64) *QueryResult {
	return &QueryResult{
		ID:        id,
		Kind:      kind,
		Result:    Pending,
		Output:    "waiting on nomad",
		Timestamp: now,
	}
}

func (qr *QueryResult) String() string {
	return fmt.Sprintf("(%s %s %s %v)", qr.ID, qr.Kind, qr.Result, qr.Timestamp)
}
