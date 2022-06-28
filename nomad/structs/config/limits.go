package config

import (
	"math"

	"github.com/hashicorp/nomad/helper"
)

const (
	// LimitsNonStreamingConnsPerClient is the number of connections per
	// peer to reserve for non-streaming RPC connections. Since streaming
	// RPCs require their own TCP connection, they have their own limit
	// this amount lower than the overall limit. This reserves a number of
	// connections for Raft and other RPCs.
	//
	// TODO Remove limit once MultiplexV2 is used.
	LimitsNonStreamingConnsPerClient = 20
)

// Limits configures timeout limits similar to Consul's limits configuration
// parameters. Limits is the internal version with the fields parsed.
type Limits struct {
	// HTTPSHandshakeTimeout is the deadline by which HTTPS TLS handshakes
	// must complete.
	//
	// 0 means no timeout.
	HTTPSHandshakeTimeout string `hcl:"https_handshake_timeout"`

	// HTTPMaxConnsPerClient is the maximum number of concurrent HTTP
	// connections from a single IP address. nil/0 means no limit.
	HTTPMaxConnsPerClient *int `hcl:"http_max_conns_per_client"`

	// RPCHandshakeTimeout is the deadline by which RPC handshakes must
	// complete. The RPC handshake includes the first byte read as well as
	// the TLS handshake and subsequent byte read if TLS is enabled.
	//
	// The deadline is reset after the first byte is read so when TLS is
	// enabled RPC connections may take (timeout * 2) to complete.
	//
	// The RPC handshake timeout only applies to servers. 0 means no
	// timeout.
	RPCHandshakeTimeout string `hcl:"rpc_handshake_timeout"`

	// RPCMaxConnsPerClient is the maximum number of concurrent RPC
	// connections from a single IP address. nil/0 means no limit.
	RPCMaxConnsPerClient *int `hcl:"rpc_max_conns_per_client"`

	// RPCDefaultWriteRate is the default maximum write RPC requests
	// per endpoint per user per minute. nil/0 means no limit.
	RPCDefaultWriteRate *int `hcl:"rpc_default_write_rate"`

	// RPCDefaultReadRate is the default maximum read RPC requests per
	// endpoint per user per minute. nil/0 means no limit.
	RPCDefaultReadRate *int `hcl:"rpc_default_read_rate"`

	// RPCDefaultListRate is the default maximum list RPC requests per
	// endpoint per user per minute. nil/0 means no limit.
	RPCDefaultListRate *int `hcl:"rpc_default_list_rate"`

	// These are the RPC limits for individual RPC endpoints
	Namespace *RPCEndpointLimits `hcl:"namespace"`
	Job       *RPCEndpointLimits `hcl:"job"`
	// TODO, etc...
}

// DefaultLimits returns the default limits values. User settings should be
// merged into these defaults.
func DefaultLimits() Limits {
	return Limits{
		HTTPSHandshakeTimeout: "5s",
		HTTPMaxConnsPerClient: helper.IntToPtr(100),
		RPCHandshakeTimeout:   "5s",
		RPCMaxConnsPerClient:  helper.IntToPtr(100),
		RPCDefaultWriteRate:   helper.IntToPtr(math.MaxInt),
		RPCDefaultReadRate:    helper.IntToPtr(math.MaxInt),
		RPCDefaultListRate:    helper.IntToPtr(math.MaxInt),
	}
}

// Merge returns a new Limits where non-empty/nil fields in the argument have
// precedence.
func (l *Limits) Merge(o Limits) Limits {
	m := *l

	if o.HTTPSHandshakeTimeout != "" {
		m.HTTPSHandshakeTimeout = o.HTTPSHandshakeTimeout
	}
	if o.HTTPMaxConnsPerClient != nil {
		m.HTTPMaxConnsPerClient = helper.IntToPtr(*o.HTTPMaxConnsPerClient)
	}
	if o.RPCHandshakeTimeout != "" {
		m.RPCHandshakeTimeout = o.RPCHandshakeTimeout
	}
	if o.RPCMaxConnsPerClient != nil {
		m.RPCMaxConnsPerClient = helper.IntToPtr(*o.RPCMaxConnsPerClient)
	}
	if o.RPCDefaultWriteRate != nil {
		m.RPCDefaultWriteRate = helper.IntToPtr(*o.RPCDefaultWriteRate)
	}
	if o.RPCDefaultReadRate != nil {
		m.RPCDefaultReadRate = helper.IntToPtr(*o.RPCDefaultReadRate)
	}
	if o.RPCDefaultListRate != nil {
		m.RPCDefaultListRate = helper.IntToPtr(*o.RPCDefaultListRate)
	}

	if o.Namespace != nil {
		m.Namespace = m.Namespace.Merge(*o.Namespace)
	}

	return m
}

// Copy returns a new deep copy of a Limits struct.
func (l *Limits) Copy() Limits {
	c := *l
	if l.HTTPMaxConnsPerClient != nil {
		c.HTTPMaxConnsPerClient = helper.IntToPtr(*l.HTTPMaxConnsPerClient)
	}
	if l.RPCMaxConnsPerClient != nil {
		c.RPCMaxConnsPerClient = helper.IntToPtr(*l.RPCMaxConnsPerClient)
	}
	if l.RPCDefaultWriteRate != nil {
		c.RPCDefaultWriteRate = helper.IntToPtr(*l.RPCDefaultWriteRate)
	}
	if l.RPCDefaultReadRate != nil {
		c.RPCDefaultReadRate = helper.IntToPtr(*l.RPCDefaultReadRate)
	}
	if l.RPCDefaultListRate != nil {
		c.RPCDefaultListRate = helper.IntToPtr(*l.RPCDefaultListRate)
	}
	c.Namespace = l.Namespace.Copy()

	return c
}

type RPCEndpointLimits struct {
	RPCWriteRate *int `hcl:"rpc_write_rate"`
	RPCReadRate  *int `hcl:"rpc_read_rate"`
	RPCListRate  *int `hcl:"rpc_list_rate"`
}

func (l *RPCEndpointLimits) Merge(o RPCEndpointLimits) *RPCEndpointLimits {
	if l == nil {
		m := o
		return &m
	}
	m := l
	if o.RPCWriteRate != nil {
		m.RPCWriteRate = helper.IntToPtr(*o.RPCWriteRate)
	}
	if o.RPCReadRate != nil {
		m.RPCReadRate = helper.IntToPtr(*o.RPCReadRate)
	}
	if o.RPCListRate != nil {
		m.RPCListRate = helper.IntToPtr(*o.RPCListRate)
	}
	return m
}

func (l *RPCEndpointLimits) Copy() *RPCEndpointLimits {
	c := l
	if l.RPCWriteRate != nil {
		c.RPCWriteRate = helper.IntToPtr(*l.RPCWriteRate)
	}
	if l.RPCReadRate != nil {
		c.RPCReadRate = helper.IntToPtr(*l.RPCReadRate)
	}
	if l.RPCListRate != nil {
		c.RPCListRate = helper.IntToPtr(*l.RPCListRate)
	}
	return c
}
