package testutil

import (
	"testing"

	"github.com/jarcoal/httpmock"
)

// NewMockTransport returns a fresh httpmock-backed transport. Plug it
// into an *http.Client and pass that client to the SDK Client (via
// SetHTTPClient or via the clienthelper helper).
//
// URL pattern conventions used by httpmock.RegisterResponder, mirrored
// by the helpers below:
//
//   - A plain string is matched as the EXACT URL (scheme + host + path
//     + raw query). Example:
//
//	"http://my-project.cn-mock.example.com/logstores/my-store"
//
//   - Prefix the URL with "=~" for a Go regexp match. The SDK puts the
//     project name in the host (https://<project>.<endpoint>/...), so
//     this is usually the most ergonomic choice. Example matching any
//     project on the mock endpoint:
//
//	`=~^http://[^/]+\.cn-mock\.example\.com/logstores/my-store$`
//
// See https://pkg.go.dev/github.com/jarcoal/httpmock for the full
// matcher reference.
func NewMockTransport() *httpmock.MockTransport {
	return httpmock.NewMockTransport()
}

// RegisterJSON registers a responder that returns `body` marshalled to
// JSON with HTTP status `status`. `body` may be any value
// json.Marshal-able; pass a struct, a map, or a raw json.RawMessage.
//
// urlPattern follows the same conventions documented on
// NewMockTransport.
func RegisterJSON(t *testing.T, transport *httpmock.MockTransport, method, urlPattern string, status int, body interface{}) {
	t.Helper()
	transport.RegisterResponder(
		method,
		urlPattern,
		httpmock.NewJsonResponderOrPanic(status, body),
	)
}

// RegisterError registers a responder that mirrors the SLS server's
// JSON error envelope: { "errorCode": code, "errorMessage": message }.
// Use it to simulate things like "ProjectNotExist" / 404,
// "Unauthorized" / 401 etc. without involving the real backend.
func RegisterError(t *testing.T, transport *httpmock.MockTransport, method, urlPattern string, status int, code, message string) {
	t.Helper()
	body := map[string]string{
		"errorCode":    code,
		"errorMessage": message,
	}
	transport.RegisterResponder(
		method,
		urlPattern,
		httpmock.NewJsonResponderOrPanic(status, body),
	)
}
