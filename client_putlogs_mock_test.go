package sls_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/aliyun/aliyun-log-go-sdk/internal/testutil"
	"github.com/aliyun/aliyun-log-go-sdk/internal/testutil/clienthelper"
)

// TestPutLogsMock proves Client.PutLogs goes through the supplied
// http.RoundTripper and signs the request as expected. Lives in the
// external test package so it can use clienthelper without creating
// an import cycle.
func TestPutLogsMock(t *testing.T) {
	const (
		project  = "my-project"
		logstore = "my-store"
	)

	transport := testutil.NewMockTransport()
	client := clienthelper.NewMockedClient(transport)

	var captured *http.Request
	transport.RegisterResponder("POST",
		"=~^http://"+project+"\\."+clienthelper.MockEndpoint+"/logstores/"+logstore+"$",
		func(req *http.Request) (*http.Response, error) {
			captured = req
			return &http.Response{
				Status:     "200 OK",
				StatusCode: 200,
				Body:       http.NoBody,
				Header:     make(http.Header),
				Request:    req,
			}, nil
		},
	)

	lg := &sls.LogGroup{
		Topic:  proto.String("topic"),
		Source: proto.String("127.0.0.1"),
		Logs: []*sls.Log{
			{
				Time: proto.Uint32(1700000000),
				Contents: []*sls.LogContent{
					{Key: proto.String("k"), Value: proto.String("v")},
				},
			},
		},
	}

	require.NoError(t, client.PutLogs(project, logstore, lg))
	require.NotNil(t, captured, "PutLogs did not invoke the transport")

	require.Equal(t, "POST", captured.Method)
	require.Equal(t, project+"."+clienthelper.MockEndpoint, captured.URL.Host)
	require.Equal(t, "/logstores/"+logstore, captured.URL.Path)

	// PutLogs lz4-compresses the body and advertises the compression.
	require.Equal(t, "lz4", captured.Header.Get("x-log-compresstype"))
	require.Equal(t, "application/x-protobuf", captured.Header.Get("Content-Type"))
	require.NotEmpty(t, captured.Header.Get("x-log-bodyrawsize"))

	// Signed and stamped with the mock AK.
	auth := captured.Header.Get("Authorization")
	require.True(t, strings.HasPrefix(auth, "LOG "+clienthelper.MockAccessKeyID+":"),
		"unexpected Authorization header: %q", auth)
	require.NotEmpty(t, captured.Header.Get("Date"))
	require.Equal(t, "0.6.0", captured.Header.Get("x-log-apiversion"))
}

// TestPutLogsMockServerError shows how RegisterError surfaces an SLS
// error envelope back to the caller.
func TestPutLogsMockServerError(t *testing.T) {
	const (
		project  = "my-project"
		logstore = "my-store"
	)

	transport := testutil.NewMockTransport()
	client := clienthelper.NewMockedClient(transport)

	testutil.RegisterError(t, transport, "POST",
		"=~^http://"+project+"\\."+clienthelper.MockEndpoint+"/logstores/"+logstore+"$",
		404, "LogStoreNotExist", "logstore not found")

	lg := &sls.LogGroup{
		Logs: []*sls.Log{
			{
				Time: proto.Uint32(1700000000),
				Contents: []*sls.LogContent{
					{Key: proto.String("k"), Value: proto.String("v")},
				},
			},
		},
	}

	err := client.PutLogs(project, logstore, lg)
	require.Error(t, err)
	slsErr, ok := err.(*sls.Error)
	require.Truef(t, ok, "expected *sls.Error, got %T: %v", err, err)
	require.Equal(t, "LogStoreNotExist", slsErr.Code)
	require.Equal(t, int32(404), slsErr.HTTPCode)
}
