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

type Checker interface {
	Do(*QueryContext, *Query) *structs.CheckQueryResult
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
func (c *checker) Do(qc *QueryContext, q *Query) *structs.CheckQueryResult {
	var qr *structs.CheckQueryResult

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

func (c *checker) checkTCP(qc *QueryContext, q *Query) *structs.CheckQueryResult {
	qr := &structs.CheckQueryResult{
		Kind:      q.Kind,
		Timestamp: c.now(),
		Status:    structs.CheckSuccess,
	}

	addr, err := address(qc, q)
	if err != nil {
		qr.Output = err.Error()
		qr.Status = structs.CheckFailure
		return qr
	}

	if _, err = net.Dial("tcp", addr); err != nil {
		qr.Output = err.Error()
		qr.Status = structs.CheckFailure
		return qr
	}

	qr.Output = "nomad: ok"
	return qr
}

func (c *checker) checkHTTP(qc *QueryContext, q *Query) *structs.CheckQueryResult {
	qr := &structs.CheckQueryResult{
		Kind:      q.Kind,
		Timestamp: c.now(),
		Status:    structs.CheckPending,
	}

	addr, err := address(qc, q)
	if err != nil {
		qr.Output = err.Error()
		qr.Status = structs.CheckFailure
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
		qr.Status = structs.CheckFailure
		return qr
	}

	result, err := c.httpClient.Do(request)
	if err != nil {
		qr.Output = fmt.Sprintf("nomad: %s", err.Error())
		qr.Status = structs.CheckFailure
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
		qr.Status = structs.CheckSuccess
	} else {
		qr.Status = structs.CheckFailure
	}

	return qr
}
