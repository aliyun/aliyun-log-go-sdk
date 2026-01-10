package sls

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type MultimodalStatus string

const (
	MultimodalEnabled  MultimodalStatus = "Enabled"
	MultimodalDisabled MultimodalStatus = "Disabled"
)

type GetLogStoreMultimodalConfigurationResponse struct {
	Status         MultimodalStatus `json:"status"`
	AnonymousWrite MultimodalStatus `json:"anonymousWrite,omitempty"`
}

type PutLogStoreMultimodalConfigurationRequest struct {
	Status         MultimodalStatus  `json:"status"`
	AnonymousWrite *MultimodalStatus `json:"anonymousWrite,omitempty"`
}

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

func (c *Client) PutLogStoreMultimodalConfiguration(project, logstore string, req *PutLogStoreMultimodalConfigurationRequest) error {
	body, err := json.Marshal(req)
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
