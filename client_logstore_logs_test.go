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
				from, to := int64(0), int64(1700000000)
				expected := map[string]interface{}{"from": float64(from), "to": float64(to)}
				var query, rowID string
				if selector == "query" {
					query = "status:error"
					expected["query"] = query
				} else {
					rowID = "row-123"
					expected["rowId"] = rowID
				}
				data := `{"status":"已处理"}`
				if operation == "update" {
					expected["data"], expected["updateMode"] = data, "partial"
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
					resp, err := client.UpdateLogStoreLogs("project", "store", &sls.UpdateLogStoreLogsRequest{From: from, To: to, Query: query, RowID: rowID, UpdateMode: "partial", Data: data})
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
					resp, e := client.UpdateLogStoreLogs("project", "store", &sls.UpdateLogStoreLogsRequest{From: 1700000000, To: 1700000100, RowID: "row", Data: `{}`})
					err = e
					if tc.wantError {
						require.Nil(t, resp)
					} else {
						require.Equal(t, int64(0), resp.AffectedRows)
					}
				} else {
					resp, e := client.DeleteLogStoreLogs("project", "store", &sls.DeleteLogStoreLogsRequest{From: 1700000000, To: 1700000100, RowID: "row"})
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

// Range validation belongs to the service, and even zero bounds must be sent.
func TestLogStoreLogsTimeRangePassthrough(t *testing.T) {
	for _, bounds := range [][2]int64{{0, 0}, {10, 5}} {
		transport := testutil.NewMockTransport()
		client := clienthelper.NewMockedClient(transport)
		calls := 0
		for _, operation := range []string{"updatelogs", "deletelogs"} {
			transport.RegisterResponder(http.MethodPost, "http://project."+clienthelper.MockEndpoint+"/logstores/store/"+operation, func(req *http.Request) (*http.Response, error) {
				calls++
				var body map[string]interface{}
				require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
				require.Equal(t, float64(bounds[0]), body["from"])
				require.Equal(t, float64(bounds[1]), body["to"])
				return &http.Response{StatusCode: 400, Body: io.NopCloser(strings.NewReader(`{"errorCode":"InvalidParameter","errorMessage":"invalid time range"}`)), Header: make(http.Header)}, nil
			})
		}
		_, err := client.UpdateLogStoreLogs("project", "store", &sls.UpdateLogStoreLogsRequest{From: bounds[0], To: bounds[1], RowID: "row", Data: `{}`})
		var serviceError *sls.Error
		require.ErrorAs(t, err, &serviceError)
		require.Equal(t, "invalid time range", serviceError.Message)
		_, err = client.DeleteLogStoreLogs("project", "store", &sls.DeleteLogStoreLogsRequest{From: bounds[0], To: bounds[1], RowID: "row"})
		require.ErrorAs(t, err, &serviceError)
		require.Equal(t, "invalid time range", serviceError.Message)
		require.Equal(t, 2, calls)
	}
}

func TestUpdateLogStoreLogsOptionalFields(t *testing.T) {
	for _, mode := range []string{"", "full", "partial"} {
		t.Run("mode="+mode, func(t *testing.T) {
			transport := testutil.NewMockTransport()
			client := clienthelper.NewMockedClient(transport)
			calls := 0
			transport.RegisterResponder(http.MethodPost, "http://project."+clienthelper.MockEndpoint+"/logstores/store/updatelogs", func(req *http.Request) (*http.Response, error) {
				calls++
				var body map[string]interface{}
				require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
				require.NotContains(t, body, "data")
				if mode == "" {
					require.NotContains(t, body, "updateMode")
				} else {
					require.Equal(t, mode, body["updateMode"])
				}
				require.Equal(t, "id:query", body["query"])
				require.Equal(t, "row", body["rowId"])
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"affectedRows":0}`)), Header: make(http.Header)}, nil
			})
			_, err := client.UpdateLogStoreLogs("project", "store", &sls.UpdateLogStoreLogsRequest{From: 1, To: 2, Query: "id:query", RowID: "row", UpdateMode: mode})
			require.NoError(t, err)
			require.Equal(t, 1, calls)
		})
	}
}
