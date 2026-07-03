package sls

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
)

type DescribeRegionsRequest struct {
	// Language specifies the localized region name language, such as zh, en, or jp.
	Language string `json:"language,omitempty"`
}

type RegionInfo struct {
	Region              string   `json:"region"`
	LocalName           string   `json:"localName"`
	IntranetEndpoint    string   `json:"intranetEndpoint"`
	InternetEndpoint    string   `json:"internetEndpoint"`
	InternalEndpoint    string   `json:"internalEndpoint"`
	DataRedundancyTypes []string `json:"dataRedundancyType"`
}

type DescribeRegionsResponse struct {
	Regions []RegionInfo `json:"regions"`
}

func (c *Client) DescribeRegions(req *DescribeRegionsRequest) (*DescribeRegionsResponse, error) {
	h := map[string]string{
		"x-log-bodyrawsize": "0",
		"Content-Type":      "application/json",
	}

	urlVal := url.Values{}
	if req != nil && req.Language != "" {
		urlVal.Add("language", req.Language)
	}

	uri := "/regions"
	if len(urlVal) > 0 {
		uri += "?" + urlVal.Encode()
	}

	r, err := c.request("", "GET", uri, h, nil)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		return nil, httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}

	resp := &DescribeRegionsResponse{}
	if err := json.Unmarshal(buf, resp); err != nil {
		return nil, NewClientError(err)
	}
	return resp, nil
}
