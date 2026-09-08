package sls

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

// UpdateLogStoreLogsRequest selects logs by time range and query, or by row ID.
// From and To are required Unix timestamps in seconds defining [From, To),
// including when selecting by row ID. The service validates the time range.
// Specify Query or RowID; if both are set, the service gives RowID precedence.
// Query accepts search expressions, not SQL or SPL.
type UpdateLogStoreLogsRequest struct {
	From  int64  `json:"from"`
	To    int64  `json:"to"`
	Query string `json:"query,omitempty"`
	RowID string `json:"rowId,omitempty"`
	// UpdateMode is full or partial; when omitted, the service defaults to partial.
	UpdateMode string `json:"updateMode,omitempty"`
	// Data is a JSON-encoded string containing the fields to update.
	Data string `json:"data,omitempty"`
}

// UpdateLogStoreLogsResponse reports the number of logs affected synchronously.
type UpdateLogStoreLogsResponse struct {
	AffectedRows int64 `json:"affectedRows"`
}

// UpdateLogStoreLogs synchronously updates logs and returns the affected row count.
// Log modification must be enabled on the logstore (LogStore.EnableModify).
// This API does not create an asynchronous task.
func (c *Client) UpdateLogStoreLogs(project, logstore string, req *UpdateLogStoreLogsRequest) (*UpdateLogStoreLogsResponse, error) {
	if req == nil {
		return nil, NewClientError(errors.New("update logs request must not be nil"))
	}
	resp := &UpdateLogStoreLogsResponse{}
	if err := c.modifyLogStoreLogs(project, logstore, "updatelogs", req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// DeleteLogStoreLogsRequest selects logs by time range and query, or by row ID.
// From and To are required Unix timestamps in seconds defining [From, To),
// including when selecting by row ID. The service validates the time range.
// Specify Query or RowID; if both are set, the service gives RowID precedence.
// Query accepts search expressions, not SQL or SPL.
type DeleteLogStoreLogsRequest struct {
	From  int64  `json:"from"`
	To    int64  `json:"to"`
	Query string `json:"query,omitempty"`
	RowID string `json:"rowId,omitempty"`
}

// DeleteLogStoreLogsResponse reports the number of logs affected synchronously.
type DeleteLogStoreLogsResponse struct {
	AffectedRows int64 `json:"affectedRows"`
}

// DeleteLogStoreLogs synchronously deletes logs and returns the affected row count.
// Log modification must be enabled on the logstore (LogStore.EnableModify).
// This API does not create an asynchronous task.
func (c *Client) DeleteLogStoreLogs(project, logstore string, req *DeleteLogStoreLogsRequest) (*DeleteLogStoreLogsResponse, error) {
	if req == nil {
		return nil, NewClientError(errors.New("delete logs request must not be nil"))
	}
	resp := &DeleteLogStoreLogsResponse{}
	if err := c.modifyLogStoreLogs(project, logstore, "deletelogs", req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) modifyLogStoreLogs(project, logstore, operation string, req, result interface{}) error {
	if project == "" || logstore == "" {
		return NewClientError(errors.New("project and logstore must not be empty"))
	}
	body, err := json.Marshal(req)
	if err != nil {
		return NewClientError(err)
	}
	headers := map[string]string{
		"Content-Type":      "application/json",
		"x-log-bodyrawsize": fmt.Sprintf("%d", len(body)),
	}
	r, err := c.request(project, http.MethodPost, "/logstores/"+logstore+"/"+operation, headers, body)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	body, err = ioutil.ReadAll(r.Body)
	if err != nil {
		return readResponseError(err)
	}
	if err = json.Unmarshal(body, result); err != nil {
		return NewClientError(err)
	}
	return nil
}
