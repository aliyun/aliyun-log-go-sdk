package sls

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

// ResourcePolicyResourceType identifies a resource supported by the Resource
// Policy API.
type ResourcePolicyResourceType string

const (
	ResourcePolicyProject  ResourcePolicyResourceType = "project"
	ResourcePolicyLogstore ResourcePolicyResourceType = "logstore"
)

// PutResourcePolicyRequest defines a resource policy to create or update.
type PutResourcePolicyRequest struct {
	ResourceType   ResourcePolicyResourceType `json:"resourceType"`
	ResourceName   string                     `json:"resourceName,omitempty"`
	PolicyDocument string                     `json:"policyDocument"`
	DryRun         bool                       `json:"dryRun"`
}

// GetResourcePolicyResponse describes a resource policy returned by SLS.
type GetResourcePolicyResponse struct {
	ResourceType   ResourcePolicyResourceType `json:"resourceType"`
	ResourceName   string                     `json:"resourceName,omitempty"`
	PolicyDocument string                     `json:"policyDocument"`
	Revision       int64                      `json:"revision"`
	CreateTime     int64                      `json:"createTime"`
	UpdateTime     int64                      `json:"updateTime"`
}

// PutResourcePolicy creates or updates a resource policy.
func (c *Client) PutResourcePolicy(project string, req *PutResourcePolicyRequest) error {
	if req == nil {
		return NewClientError(errors.New("resource policy request must not be nil"))
	}
	if err := validateResourcePolicyTarget(project, req.ResourceType); err != nil {
		return err
	}
	if req.PolicyDocument == "" {
		return NewClientError(errors.New("policy document must not be empty"))
	}

	body, err := json.Marshal(req)
	if err != nil {
		return NewClientError(err)
	}
	h := map[string]string{
		"x-log-bodyrawsize": fmt.Sprintf("%d", len(body)),
		"Content-Type":      "application/json",
	}

	r, err := c.request(project, http.MethodPut, "/resource-policies", h, body)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	return nil
}

// GetResourcePolicy gets the resource policy for a project or logstore.
func (c *Client) GetResourcePolicy(project string, resourceType ResourcePolicyResourceType, resourceName string) (*GetResourcePolicyResponse, error) {
	if err := validateResourcePolicyTarget(project, resourceType); err != nil {
		return nil, err
	}

	h := map[string]string{
		"x-log-bodyrawsize": "0",
		"Content-Type":      "application/json",
	}
	uri := "/resource-policies?" + resourcePolicyTargetQuery(resourceType, resourceName)
	r, err := c.request(project, http.MethodGet, uri, h, nil)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, readResponseError(err)
	}
	resp := &GetResourcePolicyResponse{}
	if err = json.Unmarshal(body, resp); err != nil {
		return nil, NewClientError(err)
	}
	return resp, nil
}

// DeleteResourcePolicy deletes the resource policy for a project or logstore.
func (c *Client) DeleteResourcePolicy(project string, resourceType ResourcePolicyResourceType, resourceName string) error {
	if err := validateResourcePolicyTarget(project, resourceType); err != nil {
		return err
	}

	h := map[string]string{
		"x-log-bodyrawsize": "0",
		"Content-Type":      "application/json",
	}
	uri := "/resource-policies?" + resourcePolicyTargetQuery(resourceType, resourceName)
	r, err := c.request(project, http.MethodDelete, uri, h, nil)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	return nil
}

func validateResourcePolicyTarget(project string, resourceType ResourcePolicyResourceType) error {
	if project == "" {
		return NewClientError(errors.New("project must not be empty"))
	}
	if resourceType != ResourcePolicyProject && resourceType != ResourcePolicyLogstore {
		return NewClientError(errors.New("resource type must be project or logstore"))
	}
	return nil
}

func resourcePolicyTargetQuery(resourceType ResourcePolicyResourceType, resourceName string) string {
	v := url.Values{}
	v.Add("resourceType", string(resourceType))
	if resourceName != "" {
		v.Add("resourceName", resourceName)
	}
	return v.Encode()
}
