package roundtrip

import (
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTemplateTripper_RoundTrip(t *testing.T) {
	testCases := []struct {
		inputTemplateTripper *TemplateTripper
		inputReq             *http.Request
		expectedResp         *http.Response
		expectedError        error
		name                 string
	}{
		{
			inputTemplateTripper: &TemplateTripper{},
			inputReq:             &http.Request{URL: &url.URL{Path: "/v1/nodes"}},
			expectedResp:         nil,
			expectedError:        errors.New("attempted to call blocked API endpoint: /v1/nodes"),
			name:                 "blocked request path",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResp, actualError := tc.inputTemplateTripper.RoundTrip(tc.inputReq)
			require.Equal(t, tc.expectedResp, actualResp)
			require.Equal(t, tc.expectedError, actualError)
		})
	}
}
