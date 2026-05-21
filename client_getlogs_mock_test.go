package sls_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/aliyun/aliyun-log-go-sdk/internal/testutil"
	"github.com/aliyun/aliyun-log-go-sdk/internal/testutil/clienthelper"
)

// TestGetLogsMock proves Client.GetLogs sends a properly signed POST
// to /logstores/<store>/logs and decodes the JSON response shape we
// register through the mock transport.
func TestGetLogsMock(t *testing.T) {
	const (
		project  = "my-project"
		logstore = "my-store"
	)

	transport := testutil.NewMockTransport()
	client := clienthelper.NewMockedClient(transport)

	body := map[string]interface{}{
		"meta": map[string]interface{}{
			"progress": "Complete",
			"count":    2,
			"hasSQL":   false,
			"keys":     []string{},
		},
		"data": []map[string]string{
			{"k": "v1"},
			{"k": "v2"},
		},
	}

	var captured *http.Request
	jsonResponder := httpmock.NewJsonResponderOrPanic(200, body)
	transport.RegisterResponder("POST",
		"=~^http://"+project+"\\."+clienthelper.MockEndpoint+"/logstores/"+logstore+"/logs$",
		func(req *http.Request) (*http.Response, error) {
			captured = req
			return jsonResponder(req)
		},
	)

	resp, err := client.GetLogs(project, logstore, "topic",
		1700000000, 1700000060, "*", 100, 0, false)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, int64(2), resp.Count)
	require.Equal(t, "Complete", resp.Progress)

	require.NotNil(t, captured, "GetLogs did not invoke the transport")
	require.Equal(t, "POST", captured.Method)
	require.Equal(t, project+"."+clienthelper.MockEndpoint, captured.URL.Host)
	require.Equal(t, "/logstores/"+logstore+"/logs", captured.URL.Path)
	require.Equal(t, "application/json", captured.Header.Get("Content-Type"))

	auth := captured.Header.Get("Authorization")
	require.True(t, strings.HasPrefix(auth, "LOG "+clienthelper.MockAccessKeyID+":"),
		"unexpected Authorization header: %q", auth)
}

// TestGetLogsMockProjectNotExist demonstrates RegisterError driving
// the SLS error envelope path on read APIs.
func TestGetLogsMockProjectNotExist(t *testing.T) {
	const (
		project  = "no-such-project"
		logstore = "my-store"
	)

	transport := testutil.NewMockTransport()
	client := clienthelper.NewMockedClient(transport)

	testutil.RegisterError(t, transport, "POST",
		"=~^http://"+project+"\\."+clienthelper.MockEndpoint+"/logstores/"+logstore+"/logs$",
		404, "ProjectNotExist", "The Project does not exist: "+project)

	_, err := client.GetLogs(project, logstore, "", 0, 1, "*", 10, 0, false)
	require.Error(t, err)
	slsErr, ok := err.(*sls.Error)
	require.Truef(t, ok, "expected *sls.Error, got %T: %v", err, err)
	require.Equal(t, "ProjectNotExist", slsErr.Code)
	require.Equal(t, int32(404), slsErr.HTTPCode)
}
