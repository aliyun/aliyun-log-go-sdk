package sls_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/aliyun/aliyun-log-go-sdk/internal/testutil"
	"github.com/aliyun/aliyun-log-go-sdk/internal/testutil/clienthelper"
	"github.com/stretchr/testify/require"
)

func TestLogStoreLogs(t *testing.T) {
	for _, operation := range []string{"update", "delete"} {
		for _, selector := range []string{"query", "rowID"} {
			t.Run(operation+"/"+selector, func(t *testing.T) {
				transport := testutil.NewMockTransport()
				client := clienthelper.NewMockedClient(transport)
				expected := map[string]interface{}{}
				var from, to *int64
				var query, rowID string
				if selector == "query" {
					start, end := int64(0), int64(1700000000)
					from, to, query = &start, &end, "status:error"
					expected["from"], expected["to"], expected["query"] = float64(start), float64(end), query
				} else {
					rowID = "row-123"
					expected["rowId"] = rowID
				}
				data := `{"status":"已处理"}`
				if operation == "update" {
					expected["data"], expected["updateMode"] = data, "replace"
				}
				calls := 0
				transport.RegisterResponder(http.MethodPost, "http://project."+clienthelper.MockEndpoint+"/logstores/store/"+operation+"logs", func(req *http.Request) (*http.Response, error) {
					calls++
					raw, err := io.ReadAll(req.Body)
					require.NoError(t, err)
					var body map[string]interface{}
					require.NoError(t, json.Unmarshal(raw, &body))
					require.Equal(t, expected, body)
					require.Equal(t, "application/json", req.Header.Get("Content-Type"))
					require.Equal(t, strconv.Itoa(len(raw)), req.Header.Get("x-log-bodyrawsize"))
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"affectedRows":4294967296}`)), Header: make(http.Header)}, nil
				})
				if operation == "update" {
					resp, err := client.UpdateLogStoreLogs("project", "store", &sls.UpdateLogStoreLogsRequest{From: from, To: to, Query: query, RowID: rowID, UpdateMode: "replace", Data: data})
					require.NoError(t, err)
					require.Equal(t, int64(4294967296), resp.AffectedRows)
				} else {
					resp, err := client.DeleteLogStoreLogs("project", "store", &sls.DeleteLogStoreLogsRequest{From: from, To: to, Query: query, RowID: rowID})
					require.NoError(t, err)
					require.Equal(t, int64(4294967296), resp.AffectedRows)
				}
				require.Equal(t, 1, calls)
			})
		}
	}
}

func TestLogStoreLogsResponses(t *testing.T) {
	for _, operation := range []string{"update", "delete"} {
		for _, tc := range []struct {
			name, body string
			status     int
			wantError  bool
		}{
			{"zero", `{"affectedRows":0}`, 200, false},
			{"invalidJSON", `{`, 200, true},
			{"invalidCount", `{"affectedRows":"bad"}`, 200, true},
			{"serviceError", `{"errorCode":"InvalidParameter","errorMessage":"bad selector"}`, 400, true},
		} {
			t.Run(operation+"/"+tc.name, func(t *testing.T) {
				transport := testutil.NewMockTransport()
				client := clienthelper.NewMockedClient(transport)
				transport.RegisterResponder(http.MethodPost, "http://project."+clienthelper.MockEndpoint+"/logstores/store/"+operation+"logs", func(req *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
				})
				var err error
				if operation == "update" {
					resp, e := client.UpdateLogStoreLogs("project", "store", &sls.UpdateLogStoreLogsRequest{RowID: "row", Data: `{}`})
					err = e
					if tc.wantError {
						require.Nil(t, resp)
					} else {
						require.Equal(t, int64(0), resp.AffectedRows)
					}
				} else {
					resp, e := client.DeleteLogStoreLogs("project", "store", &sls.DeleteLogStoreLogsRequest{RowID: "row"})
					err = e
					if tc.wantError {
						require.Nil(t, resp)
					} else {
						require.Equal(t, int64(0), resp.AffectedRows)
					}
				}
				if tc.wantError {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
				if tc.status == 400 {
					var serviceError *sls.Error
					require.ErrorAs(t, err, &serviceError)
					require.Equal(t, "InvalidParameter", serviceError.Code)
				}
			})
		}
	}
}

func TestLogStoreLogsValidation(t *testing.T) {
	client := clienthelper.NewMockedClient(testutil.NewMockTransport())
	_, err := client.UpdateLogStoreLogs("project", "store", nil)
	require.Error(t, err)
	_, err = client.DeleteLogStoreLogs("project", "store", nil)
	require.Error(t, err)
	for _, target := range [][2]string{{"", "store"}, {"project", ""}} {
		_, err = client.UpdateLogStoreLogs(target[0], target[1], &sls.UpdateLogStoreLogsRequest{})
		require.Error(t, err)
		_, err = client.DeleteLogStoreLogs(target[0], target[1], &sls.DeleteLogStoreLogsRequest{})
		require.Error(t, err)
	}
}
