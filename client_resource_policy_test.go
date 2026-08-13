package sls_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/aliyun/aliyun-log-go-sdk/internal/testutil"
	"github.com/aliyun/aliyun-log-go-sdk/internal/testutil/clienthelper"
)

const (
	resourcePolicyProject  = "resource-policy-project"
	resourcePolicyLogstore = "resource-policy-logstore"
	resourcePolicyDocument = `{"Version":"1","Statement":[]}`
)

func TestPutResourcePolicy(t *testing.T) {
	transport := testutil.NewMockTransport()
	client := clienthelper.NewMockedClient(transport)
	url := "http://" + resourcePolicyProject + "." + clienthelper.MockEndpoint + "/resource-policies"

	var bodies []map[string]interface{}
	transport.RegisterResponder(http.MethodPut, url, func(req *http.Request) (*http.Response, error) {
		rawBody, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		var body map[string]interface{}
		require.NoError(t, json.Unmarshal(rawBody, &body))
		bodies = append(bodies, body)
		require.Equal(t, "application/json", req.Header.Get("Content-Type"))
		require.Equal(t, strconv.Itoa(len(rawBody)), req.Header.Get("x-log-bodyrawsize"))
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
	})

	require.NoError(t, client.PutResourcePolicy(resourcePolicyProject, &sls.PutResourcePolicyRequest{
		ResourceType:   sls.ResourcePolicyProject,
		PolicyDocument: resourcePolicyDocument,
	}))
	require.NoError(t, client.PutResourcePolicy(resourcePolicyProject, &sls.PutResourcePolicyRequest{
		ResourceType:   sls.ResourcePolicyLogstore,
		ResourceName:   resourcePolicyLogstore,
		PolicyDocument: resourcePolicyDocument,
		DryRun:         true,
	}))

	require.Equal(t, []map[string]interface{}{
		{
			"resourceType":   "project",
			"policyDocument": resourcePolicyDocument,
			"dryRun":         false,
		},
		{
			"resourceType":   "logstore",
			"resourceName":   resourcePolicyLogstore,
			"policyDocument": resourcePolicyDocument,
			"dryRun":         true,
		},
	}, bodies)
}

func TestGetResourcePolicy(t *testing.T) {
	transport := testutil.NewMockTransport()
	client := clienthelper.NewMockedClient(transport)
	url := "http://" + resourcePolicyProject + "." + clienthelper.MockEndpoint +
		"/resource-policies?resourceName=" + resourcePolicyLogstore + "&resourceType=logstore"

	transport.RegisterResponder(http.MethodGet, url, func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "0", req.Header.Get("x-log-bodyrawsize"))
		return httpmock.NewJsonResponse(http.StatusOK, map[string]interface{}{
			"resourceType":   "logstore",
			"resourceName":   resourcePolicyLogstore,
			"policyDocument": resourcePolicyDocument,
			"revision":       3,
			"createTime":     10,
			"updateTime":     20,
		})
	})

	resp, err := client.GetResourcePolicy(
		resourcePolicyProject, sls.ResourcePolicyLogstore, resourcePolicyLogstore,
	)
	require.NoError(t, err)
	require.Equal(t, sls.ResourcePolicyLogstore, resp.ResourceType)
	require.Equal(t, resourcePolicyLogstore, resp.ResourceName)
	require.Equal(t, resourcePolicyDocument, resp.PolicyDocument)
	require.EqualValues(t, 3, resp.Revision)
	require.EqualValues(t, 10, resp.CreateTime)
	require.EqualValues(t, 20, resp.UpdateTime)
}

func TestGetProjectResourcePolicyOmitsResourceNameAndUsesResponseTarget(t *testing.T) {
	transport := testutil.NewMockTransport()
	client := clienthelper.NewMockedClient(transport)
	url := "http://" + resourcePolicyProject + "." + clienthelper.MockEndpoint +
		"/resource-policies?resourceType=project"

	testutil.RegisterJSON(t, transport, http.MethodGet, url, http.StatusOK, map[string]interface{}{
		"resourceType":   "logstore",
		"resourceName":   "unexpected",
		"policyDocument": resourcePolicyDocument,
		"revision":       1,
		"createTime":     2,
		"updateTime":     3,
	})

	resp, err := client.GetResourcePolicy(resourcePolicyProject, sls.ResourcePolicyProject, "")
	require.NoError(t, err)
	require.Equal(t, sls.ResourcePolicyLogstore, resp.ResourceType)
	require.Equal(t, "unexpected", resp.ResourceName)
}

func TestDeleteResourcePolicy(t *testing.T) {
	transport := testutil.NewMockTransport()
	client := clienthelper.NewMockedClient(transport)
	url := "http://" + resourcePolicyProject + "." + clienthelper.MockEndpoint +
		"/resource-policies?resourceType=project"

	transport.RegisterResponder(http.MethodDelete, url, func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "0", req.Header.Get("x-log-bodyrawsize"))
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
	})

	require.NoError(t, client.DeleteResourcePolicy(resourcePolicyProject, sls.ResourcePolicyProject, ""))
}

func TestResourcePolicyValidation(t *testing.T) {
	client := clienthelper.NewMockedClient(testutil.NewMockTransport())

	require.Error(t, client.PutResourcePolicy(resourcePolicyProject, nil))
	require.Error(t, client.PutResourcePolicy("", &sls.PutResourcePolicyRequest{
		ResourceType:   sls.ResourcePolicyProject,
		PolicyDocument: resourcePolicyDocument,
	}))
	require.Error(t, client.PutResourcePolicy(resourcePolicyProject, &sls.PutResourcePolicyRequest{
		ResourceType: sls.ResourcePolicyProject,
	}))
	_, err := client.GetResourcePolicy(resourcePolicyProject, "", "")
	require.Error(t, err)
	_, err = client.GetResourcePolicy(resourcePolicyProject, "dashboard", "")
	require.Error(t, err)
	require.Error(t, client.DeleteResourcePolicy(resourcePolicyProject, "", ""))
}

func TestResourcePolicyTargetValidationIsDelegatedToServer(t *testing.T) {
	transport := testutil.NewMockTransport()
	client := clienthelper.NewMockedClient(transport)
	putURL := "http://" + resourcePolicyProject + "." + clienthelper.MockEndpoint + "/resource-policies"
	deleteURL := putURL + "?resourceName=" + resourcePolicyLogstore + "&resourceType=project"

	transport.RegisterResponder(http.MethodPut, putURL, func(req *http.Request) (*http.Response, error) {
		var body map[string]interface{}
		require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
		require.Equal(t, resourcePolicyLogstore, body["resourceName"])
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
	})
	transport.RegisterResponder(http.MethodDelete, deleteURL,
		httpmock.NewStringResponder(http.StatusOK, ""))

	require.NoError(t, client.PutResourcePolicy(resourcePolicyProject, &sls.PutResourcePolicyRequest{
		ResourceType:   sls.ResourcePolicyProject,
		ResourceName:   resourcePolicyLogstore,
		PolicyDocument: resourcePolicyDocument,
	}))
	require.NoError(t, client.DeleteResourcePolicy(
		resourcePolicyProject, sls.ResourcePolicyProject, resourcePolicyLogstore,
	))
}
