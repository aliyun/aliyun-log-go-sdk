package sls

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

func TestPaginationSizeIsSentOnlyWhenPositive(t *testing.T) {
	tests := []struct {
		name       string
		sizeHeader bool
		call       func(*Client, int) error
	}{
		{
			name:       "ListDashboard",
			sizeHeader: true,
			call: func(client *Client, size int) error {
				_, _, _, err := client.ListDashboard("project", "", 0, size)
				return err
			},
		},
		{
			name:       "ListDashboardV2",
			sizeHeader: true,
			call: func(client *Client, size int) error {
				_, _, _, _, err := client.ListDashboardV2("project", "", 0, size)
				return err
			},
		},
		{
			name:       "ListSavedSearch",
			sizeHeader: true,
			call: func(client *Client, size int) error {
				_, _, _, err := client.ListSavedSearch("project", "", 0, size)
				return err
			},
		},
		{
			name:       "ListSavedSearchV2",
			sizeHeader: true,
			call: func(client *Client, size int) error {
				_, _, _, _, err := client.ListSavedSearchV2("project", "", 0, size)
				return err
			},
		},
		{
			name: "ListAlert",
			call: func(client *Client, size int) error {
				_, _, _, err := client.ListAlert("project", "", "", 0, size)
				return err
			},
		},
		{
			name: "ListScheduledSQL",
			call: func(client *Client, size int) error {
				_, _, _, err := client.ListScheduledSQL("project", "", "", 0, size)
				return err
			},
		},
		{
			name: "ListScheduledSQLJobInstances",
			call: func(client *Client, size int) error {
				_, _, _, err := client.ListScheduledSQLJobInstances("project", "job", &InstanceStatus{Size: int64(size)})
				return err
			},
		},
		{
			name: "ListIngestion",
			call: func(client *Client, size int) error {
				_, _, _, err := client.ListIngestion("project", "", "", "", 0, size)
				return err
			},
		},
		{
			name: "ListExport",
			call: func(client *Client, size int) error {
				_, _, _, err := client.ListExport("project", "", "", "", 0, size)
				return err
			},
		},
		{
			name: "ListETL",
			call: func(client *Client, size int) error {
				_, err := client.ListETL("project", 0, size)
				return err
			},
		},
		{
			name: "ListResource",
			call: func(client *Client, size int) error {
				_, _, _, err := client.ListResource("type", "", 0, size)
				return err
			},
		},
		{
			name: "ListResourceRecord",
			call: func(client *Client, size int) error {
				_, _, _, err := client.ListResourceRecord("resource", 0, size)
				return err
			},
		},
		{
			name: "ListLogStoreV2",
			call: func(client *Client, size int) error {
				_, err := client.ListLogStoreV2("project", 0, size, "")
				return err
			},
		},
		{
			name: "ListMachinesV2",
			call: func(client *Client, size int) error {
				_, _, err := client.ListMachinesV2("project", "group", 0, size)
				return err
			},
		},
		{
			name: "ListEtlMeta",
			call: func(client *Client, size int) error {
				_, _, _, err := client.ListEtlMeta("project", "meta", 0, size)
				return err
			},
		},
		{
			name: "ListEtlMetaName",
			call: func(client *Client, size int) error {
				_, _, _, err := client.ListEtlMetaName("project", 0, size)
				return err
			},
		},
	}

	for _, test := range tests {
		for _, size := range []int{0, -1, 10} {
			t.Run(fmt.Sprintf("%s/size=%d", test.name, size), func(t *testing.T) {
				const endpoint = "mock-test-endpoint.aliyuncs.com"
				client := CreateNormalInterface(endpoint, "xxxxx", "xxxxxx", "").(*Client)
				transport := httpmock.NewMockTransport()
				client.SetHTTPClient(&http.Client{Transport: transport})

				called := false
				transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
					called = true
					var values []string
					var present bool
					if test.sizeHeader {
						values, present = req.Header[http.CanonicalHeaderKey("size")]
					} else {
						values, present = req.URL.Query()["size"]
					}

					if size > 0 {
						require.True(t, present)
						require.Equal(t, []string{strconv.Itoa(size)}, values)
					} else {
						require.False(t, present)
					}
					return httpmock.NewStringResponse(http.StatusOK, `{}`), nil
				})

				require.NoError(t, test.call(client, size))
				require.True(t, called)
			})
		}
	}
}

func TestGetLogRequestOmitsZeroLines(t *testing.T) {
	for _, lines := range []int64{0, 10} {
		t.Run(fmt.Sprintf("lines=%d", lines), func(t *testing.T) {
			req := &GetLogRequest{Lines: lines}

			body, err := json.Marshal(req)
			require.NoError(t, err)
			var payload map[string]interface{}
			require.NoError(t, json.Unmarshal(body, &payload))

			line, inBody := payload["line"]
			urlLines, inURL := req.ToURLParams()["line"]
			if lines > 0 {
				require.True(t, inBody)
				require.Equal(t, float64(lines), line)
				require.True(t, inURL)
				require.Equal(t, []string{strconv.FormatInt(lines, 10)}, urlLines)
			} else {
				require.False(t, inBody)
				require.False(t, inURL)
			}
		})
	}
}
