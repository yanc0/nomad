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
}

// Stub creates a temporary QueryResult for the check of ID in the Pending state
// so we can represent the status of not being checked yet.
func Stub(id structs.CheckID, kind structs.CheckMode, now int64) *structs.CheckQueryResult {
	return &structs.CheckQueryResult{
		ID:        id,
		Kind:      kind,
		Status:    structs.CheckPending,
		Output:    "waiting on nomad",
		Timestamp: now,
	}
}
