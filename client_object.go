package sls

import "net/http"

// PutObjectResponse defines response from PutObject call
type PutObjectResponse struct {
	Headers map[string]string
}

func (resp *PutObjectResponse) GetETag() string {
	if resp.Headers == nil {
		return ""
	}
	return resp.Headers[http.CanonicalHeaderKey("Etag")]
}

// GetObjectResponse defines response from GetObject call
type GetObjectResponse struct {
	Body    []byte
	Headers map[string]string
}

func (resp *GetObjectResponse) GetETag() string {
	if resp.Headers == nil {
		return ""
	}
	return resp.Headers[http.CanonicalHeaderKey("Etag")]
}

func (resp *GetObjectResponse) GetContentType() string {
	if resp.Headers == nil {
		return ""
	}
	return resp.Headers[http.CanonicalHeaderKey("Content-Type")]
}

func (c *Client) PutObject(project, logstore, objectName string, content []byte, headers map[string]string) (*PutObjectResponse, error) {
	ls := convertLogstore(c, project, logstore)
	return ls.PutObject(objectName, content, headers)
}

func (c *Client) GetObject(project, logstore, objectName string) (*GetObjectResponse, error) {
	ls := convertLogstore(c, project, logstore)
	return ls.GetObject(objectName)
}
