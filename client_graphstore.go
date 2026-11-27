package sls

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// CreateGraphStoreRequest supplies a concrete logical schema; the service owns business compilation.
type CreateGraphStoreRequest struct {
	StoreName  string               `json:"storeName"`
	ShardCount int32                `json:"shardCount"`
	BaseSchema GraphStoreBaseSchema `json:"baseSchema"`
	// Nil leaves the respective creation mode to the service default.
	CMSMode         *bool `json:"-"`
	TimeStateEnable *bool `json:"-"`
}

func (r CreateGraphStoreRequest) MarshalJSON() ([]byte, error) {
	type plain CreateGraphStoreRequest
	r.BaseSchema.Version = 0
	r.BaseSchema.CMSMode = false
	base := struct {
		graphStoreBaseSchemaWire
		TimeStateEnable *bool `json:"timeStateEnable,omitempty"`
	}{graphStoreBaseSchemaWire: r.BaseSchema.wire()}
	return json.Marshal(struct {
		plain
		BaseSchema interface{} `json:"baseSchema"`
	}{plain(r), base})
}

type ListGraphStoresRequest struct {
	Offset *int
	Size   *int
}

type GraphStore struct {
	ProjectID         int32  `json:"projectId"`
	StoreName         string `json:"storeName"`
	State             string `json:"state"`
	CurrentShardCount int32  `json:"currentShardCount"`
}

type GraphStoreShardInfo struct {
	ShardID int32 `json:"shardId"`
}

type GetGraphStoreResponse struct {
	GraphStore
	Shards []GraphStoreShardInfo `json:"shards"`
}

type ListGraphStoresResponse struct {
	Stores []GraphStore `json:"stores"`
	Count  uint64       `json:"count"`
	Total  int64        `json:"total"`
}

type GetGraphStoreBaseSchemaResponse struct {
	ProjectID  int32                `json:"projectId"`
	StoreName  string               `json:"storeName"`
	StoreID    int64                `json:"storeId"`
	BaseSchema GraphStoreBaseSchema `json:"baseSchema"`
}

type GetGraphStoreCollectionSchemaResponse struct {
	ProjectID        int32                      `json:"projectId"`
	StoreName        string                     `json:"storeName"`
	CollectionSchema GraphStoreCollectionSchema `json:"collectionSchema"`
}

type ListGraphStoreCollectionSchemasResponse struct {
	ProjectID         int32                        `json:"projectId"`
	StoreName         string                       `json:"storeName"`
	CollectionSchemas []GraphStoreCollectionSchema `json:"collectionSchemas"`
	NextPageToken     string                       `json:"nextPageToken,omitempty"`
}

func graphStorePathSegment(name string) error {
	if name == "" || strings.IndexFunc(name, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-')
	}) >= 0 {
		return fmt.Errorf("invalid GraphStore path segment")
	}
	return nil
}

func graphStoreCollectionPathSegment(name string) error {
	if name == "" || name == "." || name == ".." || strings.IndexFunc(name, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			r == '_' || r == '-' || r == '.' || r == '@')
	}) >= 0 {
		return fmt.Errorf("invalid GraphStore collection path segment")
	}
	return nil
}

func graphStorePath(name string) (string, error) {
	if err := graphStorePathSegment(name); err != nil {
		return "", err
	}
	return "/graphstore/" + name, nil
}

// Request assembly only: authentication, HTTP errors and timeout remain owned by c.request.
func (c *Client) graphStoreRequest(project, method, uri string, query url.Values, body interface{}, result interface{}, requestHeaders ...map[string]string) error {
	if project == "" {
		return fmt.Errorf("project is required")
	}
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	if len(query) != 0 {
		uri += "?" + query.Encode()
	}
	headers := map[string]string{
		HTTPHeaderBodyRawSize: strconv.Itoa(len(data)),
		HTTPHeaderContentType: "application/json",
	}
	for _, extra := range requestHeaders {
		for name, value := range extra {
			headers[name] = value
		}
	}
	response, err := c.request(project, method, uri, headers, data)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if result == nil {
		_, err = io.Copy(io.Discard, response.Body)
		return err
	}
	data, err = io.ReadAll(response.Body)
	if err != nil {
		return readResponseError(err)
	}
	if err := json.Unmarshal(data, result); err != nil {
		return newBadResponseError(string(data), response.Header, response.StatusCode, err)
	}
	return nil
}

func (c *Client) CreateGraphStore(project string, req *CreateGraphStoreRequest) error {
	if req == nil {
		return fmt.Errorf("CreateGraphStore request is required")
	}
	headers := make(map[string]string)
	if req.CMSMode != nil {
		headers["x-graphstore-cms-mode"] = strconv.FormatBool(*req.CMSMode)
	}
	if req.TimeStateEnable != nil {
		headers["x-graphstore-time-state-enable"] = strconv.FormatBool(*req.TimeStateEnable)
	}
	return c.graphStoreRequest(project, http.MethodPost, "/graphstore", nil, req, nil, headers)
}

func (c *Client) GetGraphStore(project, graphStore string) (*GetGraphStoreResponse, error) {
	uri, err := graphStorePath(graphStore)
	if err != nil {
		return nil, err
	}
	result := &GetGraphStoreResponse{}
	if err := c.graphStoreRequest(project, http.MethodGet, uri, nil, nil, result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListGraphStores returns one page; nil offsets and sizes are omitted.
func (c *Client) ListGraphStores(project string, req *ListGraphStoresRequest) (*ListGraphStoresResponse, error) {
	query := url.Values{}
	if req != nil {
		if req.Offset != nil {
			query.Set("offset", strconv.Itoa(*req.Offset))
		}
		if req.Size != nil {
			query.Set("size", strconv.Itoa(*req.Size))
		}
	}
	result := &ListGraphStoresResponse{}
	if err := c.graphStoreRequest(project, http.MethodGet, "/graphstore", query, nil, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) DeleteGraphStore(project, graphStore string) error {
	uri, err := graphStorePath(graphStore)
	if err != nil {
		return err
	}
	return c.graphStoreRequest(project, http.MethodDelete, uri, nil, nil, nil)
}

// SplitGraphStoreShard distinguishes nil (server-selected split) from an explicit zero hash.
func (c *Client) SplitGraphStoreShard(project, graphStore string, shardID int32, splitHash *uint64) error {
	uri, err := graphStorePath(graphStore)
	if err != nil {
		return err
	}
	var body interface{}
	if splitHash != nil {
		body = struct {
			SplitHash uint64 `json:"splitHash"`
		}{*splitHash}
	}
	return c.graphStoreRequest(project, http.MethodPost,
		uri+"/shards/"+strconv.FormatInt(int64(shardID), 10)+"/split", nil, body, nil)
}

func (c *Client) GetGraphStoreBaseSchema(project, graphStore string) (*GetGraphStoreBaseSchemaResponse, error) {
	uri, err := graphStorePath(graphStore)
	if err != nil {
		return nil, err
	}
	result := &GetGraphStoreBaseSchemaResponse{}
	if err := c.graphStoreRequest(project, http.MethodGet, uri+"/baseschema", nil, nil, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) UpdateGraphStoreBaseSchema(project, graphStore string, schema *GraphStoreBaseSchemaUpdate) error {
	uri, err := graphStorePath(graphStore)
	if err != nil {
		return err
	}
	if schema == nil {
		return fmt.Errorf("baseSchema is required")
	}
	body := struct {
		Value *GraphStoreBaseSchemaUpdate `json:"value"`
	}{schema}
	return c.graphStoreRequest(project, http.MethodPut, uri+"/baseschema", nil, body, nil)
}

func (c *Client) CreateGraphStoreCollectionSchema(project, graphStore string, schema *GraphStoreCollectionSchema) error {
	uri, err := graphStorePath(graphStore)
	if err != nil {
		return err
	}
	if schema == nil {
		return fmt.Errorf("collectionSchema is required")
	}
	value := *schema
	value.Graph = nil
	body := struct {
		Value *GraphStoreCollectionSchema `json:"value"`
	}{&value}
	return c.graphStoreRequest(project, http.MethodPost, uri+"/collectionschemas", nil, body, nil)
}

func (c *Client) GetGraphStoreCollectionSchema(project, graphStore string, collectionSchemaName string) (*GetGraphStoreCollectionSchemaResponse, error) {
	if err := graphStoreCollectionPathSegment(collectionSchemaName); err != nil {
		return nil, err
	}
	uri, err := graphStorePath(graphStore)
	if err != nil {
		return nil, err
	}
	result := &GetGraphStoreCollectionSchemaResponse{}
	if err := c.graphStoreRequest(project, http.MethodGet,
		uri+"/collectionschemas/"+collectionSchemaName, nil, nil, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) ListGraphStoreCollectionSchemas(project, graphStore string, pageSize int, pageToken string) (*ListGraphStoreCollectionSchemasResponse, error) {
	uri, err := graphStorePath(graphStore)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	if pageSize != 0 {
		query.Set("pageSize", strconv.Itoa(pageSize))
	}
	if pageToken != "" {
		query.Set("pageToken", pageToken)
	}
	result := &ListGraphStoreCollectionSchemasResponse{}
	if err := c.graphStoreRequest(project, http.MethodGet, uri+"/collectionschemas", query, nil, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) UpdateGraphStoreCollectionSchema(project, graphStore string, collectionSchemaName string, schema *GraphStoreCollectionSchemaUpdate) error {
	if err := graphStoreCollectionPathSegment(collectionSchemaName); err != nil {
		return err
	}
	uri, err := graphStorePath(graphStore)
	if err != nil {
		return err
	}
	if schema == nil {
		return fmt.Errorf("collectionSchema is required")
	}
	if schema.Name != collectionSchemaName {
		return fmt.Errorf("GraphStore collection name must match the path")
	}
	body := struct {
		Value *GraphStoreCollectionSchemaUpdate `json:"value"`
	}{schema}
	return c.graphStoreRequest(project, http.MethodPut,
		uri+"/collectionschemas/"+collectionSchemaName, nil, body, nil)
}

func (c *Client) DeleteGraphStoreCollectionSchema(project, graphStore string, collectionSchemaName string) error {
	if err := graphStoreCollectionPathSegment(collectionSchemaName); err != nil {
		return err
	}
	uri, err := graphStorePath(graphStore)
	if err != nil {
		return err
	}
	return c.graphStoreRequest(project, http.MethodDelete,
		uri+"/collectionschemas/"+collectionSchemaName, nil, nil, nil)
}

func (r *GraphStoreShardInfo) UnmarshalJSON(data []byte) error {
	type plain GraphStoreShardInfo
	var candidate GraphStoreShardInfo
	value := struct {
		*plain
		ShardID *int32 `json:"shardId"`
	}{plain: (*plain)(&candidate)}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value.ShardID == nil {
		return fmt.Errorf("missing or null GraphStoreShardInfo field")
	}
	candidate.ShardID = *value.ShardID
	*r = candidate
	return nil
}

func (r *GraphStore) UnmarshalJSON(data []byte) error {
	type plain GraphStore
	var candidate GraphStore
	value := struct {
		*plain
		ProjectID         *int32  `json:"projectId"`
		StoreName         *string `json:"storeName"`
		State             *string `json:"state"`
		CurrentShardCount *int32  `json:"currentShardCount"`
	}{plain: (*plain)(&candidate)}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value.ProjectID == nil || value.StoreName == nil || value.State == nil || value.CurrentShardCount == nil {
		return fmt.Errorf("missing or null GraphStore field")
	}
	candidate.ProjectID = *value.ProjectID
	candidate.StoreName = *value.StoreName
	candidate.State = *value.State
	candidate.CurrentShardCount = *value.CurrentShardCount
	*r = candidate
	return nil
}

func (r *ListGraphStoresResponse) UnmarshalJSON(data []byte) error {
	type plain ListGraphStoresResponse
	var candidate ListGraphStoresResponse
	value := struct {
		*plain
		Stores *[]GraphStore `json:"stores"`
		Count  *uint64       `json:"count"`
		Total  *int64        `json:"total"`
	}{plain: (*plain)(&candidate)}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value.Stores == nil || value.Count == nil || value.Total == nil {
		return fmt.Errorf("missing or null ListGraphStoresResponse field")
	}
	candidate.Stores = *value.Stores
	candidate.Count = *value.Count
	candidate.Total = *value.Total
	*r = candidate
	return nil
}

func (r *GetGraphStoreBaseSchemaResponse) UnmarshalJSON(data []byte) error {
	type plain GetGraphStoreBaseSchemaResponse
	var candidate GetGraphStoreBaseSchemaResponse
	value := struct {
		*plain
		ProjectID *int32  `json:"projectId"`
		StoreName *string `json:"storeName"`
		StoreID   *int64  `json:"storeId"`
	}{plain: (*plain)(&candidate)}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value.ProjectID == nil || value.StoreName == nil || value.StoreID == nil {
		return fmt.Errorf("missing or null GetGraphStoreBaseSchemaResponse field")
	}
	candidate.ProjectID = *value.ProjectID
	candidate.StoreName = *value.StoreName
	candidate.StoreID = *value.StoreID
	*r = candidate
	return nil
}

func (r *GetGraphStoreCollectionSchemaResponse) UnmarshalJSON(data []byte) error {
	type plain GetGraphStoreCollectionSchemaResponse
	var candidate GetGraphStoreCollectionSchemaResponse
	value := struct {
		*plain
		ProjectID *int32  `json:"projectId"`
		StoreName *string `json:"storeName"`
	}{plain: (*plain)(&candidate)}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value.ProjectID == nil || value.StoreName == nil {
		return fmt.Errorf("missing or null GetGraphStoreCollectionSchemaResponse field")
	}
	candidate.ProjectID = *value.ProjectID
	candidate.StoreName = *value.StoreName
	*r = candidate
	return nil
}

func (r *ListGraphStoreCollectionSchemasResponse) UnmarshalJSON(data []byte) error {
	type plain ListGraphStoreCollectionSchemasResponse
	var candidate ListGraphStoreCollectionSchemasResponse
	value := struct {
		*plain
		ProjectID *int32  `json:"projectId"`
		StoreName *string `json:"storeName"`
	}{plain: (*plain)(&candidate)}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value.ProjectID == nil || value.StoreName == nil {
		return fmt.Errorf("missing or null ListGraphStoreCollectionSchemasResponse field")
	}
	candidate.ProjectID = *value.ProjectID
	candidate.StoreName = *value.StoreName
	*r = candidate
	return nil
}

func (r *GetGraphStoreResponse) UnmarshalJSON(data []byte) error {
	var value struct {
		ProjectID         *int32                 `json:"projectId"`
		StoreName         *string                `json:"storeName"`
		State             *string                `json:"state"`
		CurrentShardCount *int32                 `json:"currentShardCount"`
		Shards            *[]GraphStoreShardInfo `json:"shards"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value.ProjectID == nil || value.StoreName == nil || value.State == nil || value.CurrentShardCount == nil || value.Shards == nil {
		return fmt.Errorf("missing or null GetGraphStoreResponse field")
	}
	*r = GetGraphStoreResponse{GraphStore{*value.ProjectID, *value.StoreName, *value.State, *value.CurrentShardCount}, *value.Shards}
	return nil
}
