// Package clienthelper builds an *sls.Client wired to a caller-provided
// http.RoundTripper. It is split from internal/testutil because that
// parent package must NOT import the root sls package (it would create
// an import cycle for in-package `package sls` test files that need
// testutil.RequireE2E).
//
// Tests that want a fully constructed mocked client should be written
// as external test packages (`package sls_test`) and import this
// sub-package directly.
package clienthelper

import (
	"net/http"

	sls "github.com/aliyun/aliyun-log-go-sdk"
)

// Mock identity used by NewMockedClient. Exposed so tests can
// pre-compute expected signatures / authorization headers.
const (
	MockEndpoint        = "cn-mock.example.com"
	MockAccessKeyID     = "mock-id"
	MockAccessKeySecret = "mock-key"
)

// NewMockedClient returns a *sls.Client whose underlying *http.Client
// uses the supplied transport. The client is otherwise initialised
// with deterministic mock credentials (see MockEndpoint, MockAccessKey*
// constants) so tests don't depend on environment configuration.
//
// Pair it with testutil.NewMockTransport():
//
//	transport := testutil.NewMockTransport()
//	client := clienthelper.NewMockedClient(transport)
//	testutil.RegisterJSON(t, transport, "POST",
//	    `=~^http://[^/]+\.cn-mock\.example\.com/logstores/my-store$`,
//	    200, map[string]string{"ok": "true"})
//	err := client.PutLogs("my-project", "my-store", lg)
func NewMockedClient(transport http.RoundTripper) *sls.Client {
	httpClient := &http.Client{Transport: transport}
	c := sls.CreateNormalInterface(
		MockEndpoint,
		MockAccessKeyID,
		MockAccessKeySecret,
		"",
	).(*sls.Client)
	c.SetHTTPClient(httpClient)
	return c
}
