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

func TestAutoIndexDefaults(t *testing.T) {
	for _, line := range []*sls.IndexLine{{}, {AutoTextKeys: []string{}}, sls.CreateDefaultIndex().Line} {
		require.False(t, line.AutoKeyDetect)
		require.Empty(t, line.AutoTextKeys)
		body, err := json.Marshal(line)
		require.NoError(t, err)
		require.NotContains(t, string(body), "auto_key_detect")
		require.NotContains(t, string(body), "auto_text_keys")
	}
	var index sls.Index
	require.NoError(t, json.Unmarshal([]byte(`{"line":{"token":[","," "],"caseSensitive":false}}`), &index))
	require.False(t, index.Line.AutoKeyDetect)
	require.Empty(t, index.Line.AutoTextKeys)
}

func TestAutoIndexCreateGetUpdate(t *testing.T) {
	transport := testutil.NewMockTransport()
	client := clienthelper.NewMockedClient(transport)
	url := "http://my-project." + clienthelper.MockEndpoint + "/logstores/my-logs/index"
	fields := []string{"host", "request_id", "latency"}
	testutil.RegisterJSON(t, transport, "GET", url, http.StatusOK, map[string]interface{}{
		"line": map[string]interface{}{
			"token": []string{",", " "}, "caseSensitive": false,
			"auto_key_detect": true, "auto_text_keys": fields,
		},
	})
	var methods []string
	var bodies []map[string]json.RawMessage
	for _, method := range []string{"POST", "PUT"} {
		transport.RegisterResponder(method, url, func(req *http.Request) (*http.Response, error) {
			var body struct {
				Line map[string]json.RawMessage `json:"line"`
			}
			require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
			methods = append(methods, req.Method)
			bodies = append(bodies, body.Line)
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
		})
	}
	index := sls.CreateDefaultIndex()
	index.Line.AutoKeyDetect = true
	index.Line.AutoTextKeys = fields
	require.NoError(t, client.CreateIndex("my-project", "my-logs", *index))
	index, err := client.GetIndex("my-project", "my-logs")
	require.NoError(t, err)
	require.True(t, index.Line.AutoKeyDetect)
	require.Equal(t, fields, index.Line.AutoTextKeys)
	index.Line.CaseSensitive = true
	require.NoError(t, client.UpdateIndex("my-project", "my-logs", *index))
	index.Line.AutoTextKeys = []string{"host"}
	require.NoError(t, client.UpdateIndex("my-project", "my-logs", *index))
	index.Line.AutoTextKeys = []string{}
	require.NoError(t, client.UpdateIndex("my-project", "my-logs", *index))
	index.Line.AutoKeyDetect = false
	require.NoError(t, client.UpdateIndex("my-project", "my-logs", *index))

	require.Equal(t, []string{"POST", "PUT", "PUT", "PUT", "PUT"}, methods)
	for _, body := range bodies[:2] {
		require.JSONEq(t, `["host","request_id","latency"]`, string(body["auto_text_keys"]))
		require.Equal(t, "true", string(body["auto_key_detect"]))
	}
	require.Equal(t, "true", string(bodies[1]["caseSensitive"]))
	require.JSONEq(t, `["host"]`, string(bodies[2]["auto_text_keys"]))
	require.NotContains(t, bodies[3], "auto_text_keys")
	require.Equal(t, "true", string(bodies[3]["auto_key_detect"]))
	require.NotContains(t, bodies[4], "auto_key_detect")
}
