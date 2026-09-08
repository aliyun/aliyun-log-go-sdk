package sls_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/aliyun/aliyun-log-go-sdk/internal/testutil"
	"github.com/aliyun/aliyun-log-go-sdk/internal/testutil/clienthelper"
)

func TestLogStoreEnableModifyJSON(t *testing.T) {
	data, err := json.Marshal(&sls.LogStore{Name: "store-1", EnableModify: true})
	require.NoError(t, err)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &body))
	require.Equal(t, true, body["enableModify"])

	var store sls.LogStore
	require.NoError(t, json.Unmarshal([]byte(`{"enableModify":true}`), &store))
	require.True(t, store.EnableModify)
}

func TestEnableLogStoreModify(t *testing.T) {
	const (
		project  = "my-project"
		logstore = "my-store"
	)

	transport := testutil.NewMockTransport()
	client := clienthelper.NewMockedClient(transport)
	url := "http://" + project + "." + clienthelper.MockEndpoint + "/logstores/" + logstore + "/modification"

	transport.RegisterResponder(http.MethodPut, url, func(req *http.Request) (*http.Response, error) {
		var body map[string]bool
		require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
		require.Equal(t, map[string]bool{"enabled": true}, body)
		require.Equal(t, "application/json", req.Header.Get("Content-Type"))
		require.Equal(t, "16", req.Header.Get("x-log-bodyrawsize"))
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
	})

	require.NoError(t, client.EnableLogStoreModify(project, logstore))
}

func TestCreateLogStoreWithEnableModify(t *testing.T) {
	transport := testutil.NewMockTransport()
	client := clienthelper.NewMockedClient(transport)
	calls := 0
	transport.RegisterResponder(http.MethodPost, "http://my-project."+clienthelper.MockEndpoint+"/logstores", func(req *http.Request) (*http.Response, error) {
		calls++
		var body map[string]interface{}
		require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
		require.Equal(t, true, body["enableModify"])
		require.Equal(t, "my-store", body["logstoreName"])
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
	})
	require.NoError(t, client.CreateLogStoreV2("my-project", &sls.LogStore{Name: "my-store", TTL: 1, ShardCount: 1, EnableModify: true}))
	require.Equal(t, 1, calls)
}
