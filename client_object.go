package sls

type GetObjectResponse struct {
	Body    []byte
	Headers map[string]string
}

func (resp *GetObjectResponse) GetETag() string {
	if resp.Headers == nil {
		return ""
	}
	return resp.Headers["Etag"]
}

func (resp *GetObjectResponse) GetContentType() string {
	if resp.Headers == nil {
		return ""
	}
	return resp.Headers["Content-Type"]
}

func (c *Client) PutObject(project, logstore, objectName string, content []byte, headers map[string]string) error {
	ls := convertLogstore(c, project, logstore)
	return ls.PutObject(objectName, content, headers)
}

func (c *Client) GetObject(project, logstore, objectName string) (*GetObjectResponse, error) {
	ls := convertLogstore(c, project, logstore)
	return ls.GetObject(objectName)
}
