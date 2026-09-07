package sls_test

import (
	"encoding/json"
	"net/http"
	"testing"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/aliyun/aliyun-log-go-sdk/internal/testutil"
	"github.com/aliyun/aliyun-log-go-sdk/internal/testutil/clienthelper"
	"github.com/stretchr/testify/require"
)

func TestCreateMetricStoreMock(t *testing.T) {
	transport := testutil.NewMockTransport()
	client := clienthelper.NewMockedClient(transport)
	var paths []string
	var logstore sls.LogStore
	var substore sls.SubStore
	for _, path := range []string{"/logstores", "/logstores/my-metrics/substores"} {
		transport.RegisterResponder("POST", "http://my-project."+clienthelper.MockEndpoint+path,
			func(req *http.Request) (*http.Response, error) {
				paths = append(paths, req.URL.Path)
				if req.URL.Path == "/logstores" {
					require.NoError(t, json.NewDecoder(req.Body).Decode(&logstore))
				} else {
					require.NoError(t, json.NewDecoder(req.Body).Decode(&substore))
				}
				return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
			})
	}

	require.NoError(t, client.CreateMetricStore("my-project", &sls.LogStore{Name: "my-metrics", TTL: 18, ShardCount: 2}))
	require.Equal(t, []string{"/logstores", "/logstores/my-metrics/substores"}, paths)
	require.Equal(t, "Metrics", logstore.TelemetryType)
	require.Equal(t, "my-metrics", logstore.Name)
	require.Equal(t, 18, logstore.TTL)
	require.Equal(t, sls.SubStore{
		Name: "prom", TTL: 18, SortedKeyCount: 2, TimeIndex: 2,
		Keys: []sls.SubStoreKey{
			{Name: "__name__", Type: "text"},
			{Name: "__labels__", Type: "labels"},
			{Name: "__time_nano__", Type: "long"},
			{Name: "__value__", Type: "double"},
		},
	}, substore)
}
