package sls

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

// PutObject put an object to the specified logstore
func (s *LogStore) PutObject(objectName string, content []byte, headers map[string]string) (*PutObjectResponse, error) {
	if objectName == "" {
		return nil, fmt.Errorf("object name cannot be empty")
	}

	encodedObjectName := objectNameEncode(objectName)
	uri := fmt.Sprintf("/logstores/%s/objects/%s", s.Name, encodedObjectName)

	// Prepare headers
	h := make(map[string]string)
	for k, v := range headers {
		h[k] = v
	}
	h["x-log-bodyrawsize"] = fmt.Sprintf("%d", len(content))
	if _, ok := h["Content-Type"]; !ok {
		h["Content-Type"] = "application/octet-stream"
	}

	// Send request
	r, err := request(s.project, "PUT", uri, h, content)
	if err != nil {
		return nil, NewClientError(err)
	}
	defer r.Body.Close()

	// Extract response headers
	respHeaders := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			canonicalKey := http.CanonicalHeaderKey(k)
			respHeaders[canonicalKey] = v[0]
		}
	}

	return &PutObjectResponse{
		Headers: respHeaders,
	}, nil
}

// GetObject get an object from the specified logstore
func (s *LogStore) GetObject(objectName string) (*GetObjectResponse, error) {
	if objectName == "" {
		return nil, fmt.Errorf("object name cannot be empty")
	}

	encodedObjectName := objectNameEncode(objectName)
	uri := fmt.Sprintf("/logstores/%s/objects/%s", s.Name, encodedObjectName)

	// Prepare headers
	h := map[string]string{
		"x-log-bodyrawsize": "0",
	}

	// Send request
	r, err := request(s.project, "GET", uri, h, nil)
	if err != nil {
		return nil, NewClientError(err)
	}
	defer r.Body.Close()

	// Read response body
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, readResponseError(err)
	}

	respHeaders := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			canonicalKey := http.CanonicalHeaderKey(k)
			respHeaders[canonicalKey] = v[0]
		}
	}

	return &GetObjectResponse{
		Body:    body,
		Headers: respHeaders,
	}, nil
}

// objectNameEncode encodes object name for use in URLs
func objectNameEncode(objectName string) string {
	return url.PathEscape(objectName)
}
