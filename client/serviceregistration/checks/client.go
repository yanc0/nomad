package checks

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/serviceregistration"
	"github.com/hashicorp/nomad/nomad/structs"
	"gophers.dev/pkgs/netlog"
	"oss.indeed.com/go/libtime"
)

// A Query is derived from a structs.ServiceCheck and contains the minimal
// amount of information needed to actually execute that check.
type Query struct {
	Kind Kind   // readiness or healthiness
	Type string // tcp or http

	AddressMode string // host, driver, or alloc
	PortLabel   string // label or value

	Protocol string // http checks only (http or https)
	Path     string // http checks only
	Method   string // http checks only
}

// A QueryContext contains allocation and service parameters necessary for
// address resolution.
type QueryContext struct {
	ID               ID
	CustomAddress    string
	ServicePortLabel string
	Networks         structs.Networks
	NetworkStatus    structs.NetworkStatus
	Ports            structs.AllocatedPorts
}

// GetKind determines whether the check is readiness or healthiness.
func GetKind(c *structs.ServiceCheck) Kind {
	if c != nil && c.OnUpdate == "ignore" {
		return Readiness
	}
	return Healthiness
}

// GetQuery extracts the needed info from c to actually execute the check.
func GetQuery(c *structs.ServiceCheck) *Query {
	protocol := "http"
	if c.Protocol != "" {
		protocol = c.Protocol
	}
	return &Query{
		Kind:        GetKind(c),
		Type:        c.Type,
		AddressMode: c.AddressMode,
		PortLabel:   c.PortLabel,
		Path:        c.Path,
		Method:      c.Method,
		Protocol:    protocol,
	}
}

type Checker interface {
	Do(*QueryContext, *Query) *QueryResult
}

func New(log hclog.Logger, alloc *structs.Allocation) Checker {
	httpClient := cleanhttp.DefaultPooledClient()
	httpClient.Timeout = 1 * time.Minute
	return &checker{
		log:        log.Named("checks"),
		httpClient: httpClient,
		clock:      libtime.SystemClock(),
	}
}

type checker struct {
	log        hclog.Logger
	clock      libtime.Clock
	httpClient *http.Client
}

func (c *checker) now() int64 {
	return c.clock.Now().UTC().Unix()
}

// Do will execute the Query given the QueryContext and produce a QueryResult
func (c *checker) Do(qc *QueryContext, q *Query) *QueryResult {
	var qr *QueryResult

	switch q.Type {
	case "http":
		qr = c.checkHTTP(qc, q)
	default:
		qr = c.checkTCP(qc, q)
	}

	qr.ID = qc.ID
	return qr
}

// resolve the address to use when executing Query given a QueryContext
func address(qc *QueryContext, q *Query) (string, error) {
	mode := q.AddressMode
	if mode == "" { // determine resolution for check address
		if qc.CustomAddress != "" {
			// if the service is using a custom address, enable the check to
			// inherit that custom address
			mode = structs.AddressModeAuto
		} else {
			// otherwise a check defaults to the host address
			mode = structs.AddressModeHost
		}
	}

	label := q.PortLabel
	if label == "" {
		label = qc.ServicePortLabel
	}
	netlog.Cyan("ADDRESS q.PortLabel: %s, qc.ServicePortLabel: %s, label: %s", q.PortLabel, qc.ServicePortLabel, label)

	status := qc.NetworkStatus.NetworkStatus()
	addr, port, err := serviceregistration.GetAddress(
		qc.CustomAddress, // custom address
		mode,             // check address mode
		label,            // port label
		qc.Networks,      // allocation networks
		nil,              // driver network (not supported)
		qc.Ports,         // ports
		status,           // allocation network status
	)
	if err != nil {
		netlog.Cyan("ADDRESS error: %s", err.Error())
		return "", err
	}
	if port > 0 {
		addr = net.JoinHostPort(addr, strconv.Itoa(port))
	}
	netlog.Cyan("ADDRESS: %s, %d", addr, port)
	return addr, nil
}

func (c *checker) checkTCP(qc *QueryContext, q *Query) *QueryResult {
	qr := &QueryResult{
		Kind:      q.Kind,
		Timestamp: c.now(),
		Result:    Success,
	}

	addr, err := address(qc, q)
	if err != nil {
		qr.Output = err.Error()
		qr.Result = Failure
		return qr
	}

	if _, err = net.Dial("tcp", addr); err != nil {
		qr.Output = err.Error()
		qr.Result = Failure
		return qr
	}

	qr.Output = "nomad: ok"
	return qr
}

func (c *checker) checkHTTP(qc *QueryContext, q *Query) *QueryResult {
	qr := &QueryResult{
		Kind:      q.Kind,
		Timestamp: c.now(),
		Result:    Pending,
	}

	addr, err := address(qc, q)
	if err != nil {
		qr.Output = err.Error()
		qr.Result = Failure
		return qr
	}

	u := (&url.URL{
		Scheme: q.Protocol,
		Host:   addr,
		Path:   q.Path,
	}).String()

	request, err := http.NewRequest(q.Method, u, nil)
	if err != nil {
		qr.Output = fmt.Sprintf("nomad: %s", err.Error())
		qr.Result = Failure
		return qr
	}

	result, err := c.httpClient.Do(request)
	if err != nil {
		qr.Output = fmt.Sprintf("nomad: %s", err.Error())
		qr.Result = Failure
		return qr
	}

	b, err := ioutil.ReadAll(result.Body)
	if err != nil {
		qr.Output = fmt.Sprintf("nomad: %s", err.Error())
		// let the status code dictate query result
	} else {
		qr.Output = string(b)
	}

	if result.StatusCode < 400 {
		qr.Result = Success
	} else {
		qr.Result = Failure
	}

	return qr
}
