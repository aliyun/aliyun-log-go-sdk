package sls

import (
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

func TestDescribeRegions(t *testing.T) {
	const endpoint = "mock-test-endpoint.aliyuncs.com"
	client := CreateNormalInterface(endpoint, "testAccessKeyId", "testAccessKeySecret", "").(*Client)
	transport := httpmock.NewMockTransport()
	client.SetHTTPClient(&http.Client{Transport: transport})

	transport.RegisterResponder("GET", "http://"+endpoint+"/regions?language=zh",
		func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "0", req.Header.Get("x-log-bodyrawsize"))
			require.Equal(t, endpoint, req.Host)
			return httpmock.NewStringResponse(200, `{
				"regions": [
					{
						"region": "cn-hangzhou",
						"localName": "Hangzhou",
						"intranetEndpoint": "cn-hangzhou-intranet.log.aliyuncs.com",
						"internetEndpoint": "cn-hangzhou.log.aliyuncs.com",
						"internalEndpoint": "cn-hangzhou-internal.log.aliyuncs.com",
						"dataRedundancyType": ["LRS", "ZRS"]
					}
				]
			}`), nil
		})

	resp, err := client.DescribeRegions(&DescribeRegionsRequest{Language: "zh"})
	require.NoError(t, err)
	require.Len(t, resp.Regions, 1)
	require.Equal(t, "cn-hangzhou", resp.Regions[0].Region)
	require.Equal(t, "Hangzhou", resp.Regions[0].LocalName)
	require.Equal(t, "cn-hangzhou-intranet.log.aliyuncs.com", resp.Regions[0].IntranetEndpoint)
	require.Equal(t, "cn-hangzhou.log.aliyuncs.com", resp.Regions[0].InternetEndpoint)
	require.Equal(t, "cn-hangzhou-internal.log.aliyuncs.com", resp.Regions[0].InternalEndpoint)
	require.Equal(t, []string{"LRS", "ZRS"}, resp.Regions[0].DataRedundancyTypes)
}

func TestDescribeRegionsWithoutLanguage(t *testing.T) {
	const endpoint = "mock-test-endpoint.aliyuncs.com"
	client := CreateNormalInterface(endpoint, "testAccessKeyId", "testAccessKeySecret", "").(*Client)
	transport := httpmock.NewMockTransport()
	client.SetHTTPClient(&http.Client{Transport: transport})

	transport.RegisterResponder("GET", "http://"+endpoint+"/regions",
		httpmock.NewStringResponder(200, `{"regions":[]}`))

	resp, err := client.DescribeRegions(nil)
	require.NoError(t, err)
	require.Empty(t, resp.Regions)
}
