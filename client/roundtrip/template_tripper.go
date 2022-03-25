package roundtrip

import (
	"fmt"
	"net/http"
	"strings"
)

// SetupTemplateRoundTripper builds our TemplateTransport which implements the
// http.RoundTripper interface. This is used by the template runner when
// interacting with Nomad endpoints and template functions.
func SetupTemplateRoundTripper(nodeSecret string) *TemplateTripper {
	return &TemplateTripper{
		nodeSecret: nodeSecret,
		transport:  nil,
	}
}

// Ensure TemplateTransport satisfies the http.RoundTripper interface.
var _ http.RoundTripper = &TemplateTripper{}

// TemplateTripper is the clients custom round tripper for requests made by
// consul-template to nomad related APIs.
type TemplateTripper struct {

	// nodeSecret stores the node secret ID used for HTTP request
	// authorization.
	nodeSecret string

	// transport is our custom transport used to round trip HTTP requests.
	transport *http.Transport
}

func (t *TemplateTripper) RoundTrip(req *http.Request) (*http.Response, error) {

	// Protect and only serve if they request is calling an endpoint we expect
	// and allow.
	if !strings.HasPrefix(req.URL.Path, "/v1/service") {
		return nil, fmt.Errorf("attempted to call blocked API endpoint: %v", req.URL.Path)
	}

	// The request must be cloned as we want to update fields without modifying
	// the original request.
	clonedReq := req.Clone(req.Context())

	// Add the authorization header.
	clonedReq.Header.Add("X-Nomad-Token", t.nodeSecret)

	// Call our custom round-tripper, returning the response and error from
	// this.
	return t.transport.RoundTrip(clonedReq)
}

func (t *TemplateTripper) Transport() *http.Transport { return t.transport }
