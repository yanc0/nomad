package checks

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

// GetCheckQuery extracts the needed info from c to actually execute the check.
func GetCheckQuery(c *structs.ServiceCheck) *Query {
	protocol := "http"
	if c.Protocol != "" {
		protocol = c.Protocol
	}
	return &Query{
		Kind:        structs.GetCheckMode(c),
		Type:        c.Type,
		AddressMode: c.AddressMode,
		PortLabel:   c.PortLabel,
		Path:        c.Path,
		Method:      c.Method,
		Protocol:    protocol,
	}
}

// A Query is derived from a ServiceCheck and contains the minimal
// amount of information needed to actually execute that check.
type Query struct {
	Kind structs.CheckMode // readiness or healthiness
	Type string            // tcp or http

	AddressMode string // host, driver, or alloc
	PortLabel   string // label or value

	Protocol string // http checks only (http or https)
	Path     string // http checks only
	Method   string // http checks only
}

// A QueryContext contains allocation and service parameters necessary for
// address resolution.
type QueryContext struct {
	ID               structs.CheckID
	CustomAddress    string
	ServicePortLabel string
	Networks         structs.Networks
	NetworkStatus    structs.NetworkStatus
	Ports            structs.AllocatedPorts

	Group   string
	Task    string
	Service string
	Check   string
}

// Stub creates a temporary QueryResult for the check of ID in the Pending state
// so we can represent the status of not being checked yet.
func Stub(
	id structs.CheckID, kind structs.CheckMode, now int64,
	group, task, service, check string,
) *structs.CheckQueryResult {
	return &structs.CheckQueryResult{
		ID:        id,
		Mode:      kind,
		Status:    structs.CheckPending,
		Output:    "nomad: waiting to run",
		Timestamp: now,
		Group:     group,
		Task:      task,
		Service:   service,
		Check:     check,
	}
}

// AllocationResults is a view of the check_id -> latest result for group and task
// checks in an allocation.
type AllocationResults map[structs.CheckID]*structs.CheckQueryResult

// diff returns the set of IDs in ids that are not in m.
func (m AllocationResults) diff(ids []structs.CheckID) []structs.CheckID {
	var missing []structs.CheckID
	for _, id := range ids {
		if _, exists := m[id]; !exists {
			missing = append(missing, id)
		}
	}
	return missing
}

// ClientResults is a holistic view of alloc_id -> check_id -> latest result
// group and task checks across all allocations on a client.
type ClientResults map[string]AllocationResults

func (cr ClientResults) Insert(allocID string, result *structs.CheckQueryResult) {
	if _, exists := cr[allocID]; !exists {
		cr[allocID] = make(AllocationResults)
	}
	cr[allocID][result.ID] = result
}
