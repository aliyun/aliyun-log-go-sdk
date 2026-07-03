package sls

import (
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

func TestListAllProjects(t *testing.T) {
	const endpoint = "mock-test-endpoint.aliyuncs.com"
	client := CreateNormalInterface(endpoint, "testAccessKeyId", "testAccessKeySecret", "").(*Client)
	transport := httpmock.NewMockTransport()
	client.SetHTTPClient(&http.Client{Transport: transport})

	transport.RegisterResponder("GET", "http://"+endpoint+"/?offset=5&projectName=demo&regionId=cn-shanghai&resourceGroupId=rg-a&searchText=prod&size=10&type=all",
		func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "0", req.Header.Get("x-log-bodyrawsize"))
			require.Equal(t, endpoint, req.Host)
			return httpmock.NewStringResponse(200, `{
				"count": 1,
				"total": 1,
				"projects": [
					{
						"projectName": "demo-project",
						"description": "demo",
						"createTime": 1710000000,
						"updateTime": 1710000100,
						"region": "cn-hangzhou",
						"resourceGroupId": "rg-a"
					}
				]
			}`), nil
		})

	resp, err := client.ListAllProjects(&ListAllProjectsRequest{
		Offset:          5,
		Size:            10,
		RegionId:        "cn-shanghai",
		ProjectName:     "demo",
		ResourceGroupId: "rg-a",
		SearchText:      "prod",
	})
	require.NoError(t, err)
	require.Equal(t, 1, resp.Count)
	require.Equal(t, 1, resp.Total)
	require.Len(t, resp.Projects, 1)
	require.Equal(t, "demo-project", resp.Projects[0].ProjectName)
	require.Equal(t, "cn-hangzhou", resp.Projects[0].Region)
	require.Equal(t, "rg-a", resp.Projects[0].ResourceGroupId)
}

func TestListAllProjectsWithNilRequest(t *testing.T) {
	const endpoint = "mock-test-endpoint.aliyuncs.com"
	client := CreateNormalInterface(endpoint, "testAccessKeyId", "testAccessKeySecret", "").(*Client)
	transport := httpmock.NewMockTransport()
	client.SetHTTPClient(&http.Client{Transport: transport})

	transport.RegisterResponder("GET", "http://"+endpoint+"/?type=all",
		httpmock.NewStringResponder(200, `{"count":0,"total":0,"projects":[]}`))

	resp, err := client.ListAllProjects(nil)
	require.NoError(t, err)
	require.Equal(t, 0, resp.Count)
	require.Equal(t, 0, resp.Total)
	require.Empty(t, resp.Projects)
}
