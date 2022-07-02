package structs

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
)

// The CheckMode of a check is either Healthiness or Readiness.
type CheckMode string

const (
	// A Healthiness check is asking a service, "are you healthy?". A service that
	// is healthy is thought to be _capable_ of serving traffic, but might not
	// want it yet.
	Healthiness CheckMode = "healthiness"

	// A Readiness check is asking a service, "do you want traffic?". A service
	// that is not ready is thought to not want traffic, even if it is passing
	// other healthiness checks.
	Readiness CheckMode = "readiness"
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

// A CheckQueryResult represents the outcome of a single execution of a Nomad service
// check. It records the result, the output, and when the execution took place.
//
// Knowledge of the context of the check (i.e. alloc / task) is left to the caller.
// Any check math (e.g. success_before_passing) is left to the caller.
type CheckQueryResult struct {
	ID        CheckID
	Mode      CheckMode
	Status    CheckStatus
	Output    string
	Timestamp int64

	// check coordinates
	Group   string
	Task    string
	Service string
	Check   string
}

func (r *CheckQueryResult) String() string {
	return fmt.Sprintf("(%s %s %s %v)", r.ID, r.Mode, r.Status, r.Timestamp)
}

// A CheckStatus is the result of executing a check. The status of a query is
// ternary - success, failure, or pending (not yet executed). Deployments treat
// pending and failure as the same - a deployment cannot continue until a check
// is passing.
type CheckStatus string

const (
	CheckSuccess CheckStatus = "success"
	CheckFailure CheckStatus = "failure"
	CheckPending CheckStatus = "pending"
)

// NomadCheckID returns an ID unique to the nomad service check.
//
// Checks of group-level services have no task.
func NomadCheckID(allocID, group string, c *ServiceCheck) CheckID {
	sum := md5.New()
	write := func(s string) {
		if s != "" {
			_, _ = sum.Write([]byte(s))
		}
	}
	write(allocID)
	write(group)
	write(c.TaskName)
	write(c.Name)
	write(c.Type)
	write(c.PortLabel)
	write(c.OnUpdate)
	write(c.AddressMode)
	_ = binary.Write(sum, binary.LittleEndian, c.Interval)
	_ = binary.Write(sum, binary.LittleEndian, c.Timeout)
	write(c.Protocol)
	write(c.Path)
	write(c.Method)
	h := sum.Sum(nil)
	return CheckID(fmt.Sprintf("%x", h))
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
