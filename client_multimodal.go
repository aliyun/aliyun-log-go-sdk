package sls

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

// MultimodalConfiguration represents the multimodal configuration of a logstore
type MultimodalConfiguration struct {
	Status         string `json:"status"`
	AnonymousWrite string `json:"anonymousWrite,omitempty"`
}

// GetLogStoreMultimodalConfigurationResponse defines the response from GetLogStoreMultimodalConfiguration call
type GetLogStoreMultimodalConfigurationResponse struct {
	Status         string `json:"status"`
	AnonymousWrite string `json:"anonymousWrite,omitempty"`
}

// PutLogStoreMultimodalConfigurationResponse defines the response from PutLogStoreMultimodalConfiguration call
type PutLogStoreMultimodalConfigurationResponse struct {
	// Standard response with no additional fields
}

// GetLogStoreMultimodalConfiguration gets the multimodal configuration of the logstore
func (c *Client) GetLogStoreMultimodalConfiguration(project, logstore string) (*GetLogStoreMultimodalConfigurationResponse, error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
		"Content-Type":      "application/json",
	}

	uri := fmt.Sprintf("/logstores/%s/multimodalconfiguration", logstore)
	r, err := c.request(project, "GET", uri, h, nil)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, readResponseError(err)
	}

	resp := &GetLogStoreMultimodalConfigurationResponse{}
	if err = json.Unmarshal(buf, resp); err != nil {
		return nil, NewClientError(err)
	}

	return resp, nil
}

// PutLogStoreMultimodalConfiguration sets the multimodal configuration of the logstore
func (c *Client) PutLogStoreMultimodalConfiguration(project, logstore, status string, anonymousWrite ...string) error {
	config := &MultimodalConfiguration{
		Status: status,
	}

	// Handle optional anonymousWrite parameter
	if len(anonymousWrite) > 0 && anonymousWrite[0] != "" {
		config.AnonymousWrite = anonymousWrite[0]
	}

	body, err := json.Marshal(config)
	if err != nil {
		return NewClientError(err)
	}

	h := map[string]string{
		"x-log-bodyrawsize": fmt.Sprintf("%d", len(body)),
		"Content-Type":      "application/json",
	}

	uri := fmt.Sprintf("/logstores/%s/multimodalconfiguration", logstore)
	r, err := c.request(project, "PUT", uri, h, body)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return nil
}
