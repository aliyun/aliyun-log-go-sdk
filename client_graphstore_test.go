package sls

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aliyun/aliyun-log-go-sdk/internal/testutil"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

func TestGraphStoreManagementRequests(t *testing.T) {
	for _, mode := range []string{"v1", "v4", "token"} {
		t.Run(mode, func(t *testing.T) {
			var client ClientInterface = CreateNormalInterfaceV2("example.com",
				NewUpdateFuncProviderAdapter(func() (string, string, string, time.Time, error) {
					return "key", "secret", "token", time.Now().Add(time.Hour), nil
				}))
			if mode == "v4" {
				client.SetAuthVersion(AuthV4)
				client.SetRegion("cn-test")
			}
			if mode == "token" {
				client = &TokenAutoUpdateClient{logClient: client, maxTryTimes: 3}
			}
			transport := testutil.NewMockTransport()
			client.SetHTTPClient(&http.Client{Transport: transport})
			var captured *http.Request
			var requestBody []byte
			responseBody := ""
			transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
				captured = req
				var err error
				requestBody, err = io.ReadAll(req.Body)
				require.NoError(t, err)
				response := httpmock.NewStringResponse(200, responseBody)
				response.Header.Set("x-log-requestid", "test-request")
				return response, nil
			})
			check := func(method, path string) {
				t.Helper()
				require.Equal(t, method, captured.Method)
				require.Equal(t, path, captured.URL.Path)
				require.Equal(t, "project.example.com", captured.Host)
				require.Equal(t, "0.6.0", captured.Header.Get("x-log-apiversion"))
				require.Equal(t, "application/json", captured.Header.Get("Content-Type"))
				require.Equal(t, strconv.Itoa(len(requestBody)), captured.Header.Get("x-log-bodyrawsize"))
				require.Equal(t, "token", captured.Header.Get("x-acs-security-token"))
				if mode == "v4" {
					require.True(t, strings.HasPrefix(captured.Header.Get("Authorization"), "SLS4-HMAC-SHA256"))
				} else {
					require.True(t, strings.HasPrefix(captured.Header.Get("Authorization"), "LOG key:"))
				}
			}
			require.NoError(t, client.CreateGraphStore("project", &CreateGraphStoreRequest{
				StoreName: "store", ShardCount: 2,
				BaseSchema: graphStoreTestBase(),
			}))
			check(http.MethodPost, "/graphstore")
			var body map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(requestBody, &body))
			require.Len(t, body, 3)
			require.Contains(t, string(body["baseSchema"]), "9223372036854775807")
			require.Equal(t, byte('{'), body["baseSchema"][0])
			require.Empty(t, captured.Header.Get("x-graphstore-cms-mode"))
			require.Empty(t, captured.Header.Get("x-graphstore-time-state-enable"))
			if mode == "v4" {
				require.NotEmpty(t, captured.Header.Get("x-log-content-sha256"))
			} else {
				require.NotEmpty(t, captured.Header.Get("Content-MD5"))
			}
			cms, timeState := true, false
			cmsBase := graphStoreTestBase()
			cmsBase.CMSMode, cmsBase.TimeStateEnable, cmsBase.Version = true, true, 2
			cmsBase.VertexLayout.LabelComponents = []string{"domain", "type"}
			cmsBase.EdgeLayout.Source.LabelComponents = []string{"src_domain", "src_type"}
			cmsBase.EdgeLayout.Target.LabelComponents = []string{"dst_domain", "dst_type"}
			require.NoError(t, client.CreateGraphStore("project", &CreateGraphStoreRequest{
				StoreName: "cms", ShardCount: 2, BaseSchema: cmsBase, CMSMode: &cms, TimeStateEnable: &timeState,
			}))
			check(http.MethodPost, "/graphstore")
			require.Equal(t, "true", captured.Header.Get("x-graphstore-cms-mode"))
			require.Equal(t, "false", captured.Header.Get("x-graphstore-time-state-enable"))
			require.NotContains(t, string(requestBody), `"cmsMode"`)
			require.NotContains(t, string(requestBody), `"timeStateEnable"`)
			require.NotContains(t, string(requestBody), `"version"`)
			require.Contains(t, string(requestBody), `"label":["domain","type"]`)
			require.Contains(t, string(requestBody), `"label":["src_domain","src_type"]`)
			require.Contains(t, string(requestBody), `"label":["dst_domain","dst_type"]`)
			maxHash := uint64(math.MaxUint64)
			for _, hash := range []*uint64{nil, new(uint64), &maxHash} {
				require.NoError(t, client.SplitGraphStoreShard("project", "store", 3, hash))
				check(http.MethodPost, "/graphstore/store/shards/3/split")
				if hash == nil {
					require.Empty(t, requestBody)
				} else {
					require.Equal(t, fmt.Sprintf(`{"splitHash":%d}`, *hash), string(requestBody))
				}
			}
			require.NoError(t, client.UpdateGraphStoreBaseSchema("project", "store", &GraphStoreBaseSchemaUpdate{
				TTLSeconds: math.MaxInt64, IndexEnable: false,
				GraphLayouts: []GraphStoreGraphLayoutUpdate{
					{Name: GraphStoreLayoutVertex, KeyIndexConfigs: map[string]GraphStoreIndexConfig{"id": {Enable: false}}},
					{Name: GraphStoreLayoutEdge, KeyIndexConfigs: map[string]GraphStoreIndexConfig{}},
				},
			}))
			check(http.MethodPut, "/graphstore/store/baseschema")
			require.JSONEq(t, `{"value":{"ttlSeconds":9223372036854775807,"indexEnable":false,"graphLayouts":[{"name":"vertex","keyIndexConfigs":{"id":{"enable":false}}},{"name":"edge","keyIndexConfigs":{}}]}}`, string(requestBody))
			createCollection := graphStoreTestCollection("雪")
			createCollection.Graph = &GraphStoreCollectionGraph{Label: []string{"domain", "type"}}
			require.NoError(t, client.CreateGraphStoreCollectionSchema("project", "store", createCollection))
			check(http.MethodPost, "/graphstore/store/collectionschemas")
			require.JSONEq(t, `{"value":{"name":"雪","description":"schema","graphLayout":"vertex","collectionKeyValues":[],"fields":[]}}`, string(requestBody))
			require.NoError(t, client.UpdateGraphStoreCollectionSchema("project", "store", "9223372036854775807", graphStoreTestUpdate("9223372036854775807")))
			check(http.MethodPut, "/graphstore/store/collectionschemas/9223372036854775807")
			require.JSONEq(t, `{"value":{"name":"9223372036854775807","description":"schema","fields":[]}}`, string(requestBody))
			require.NoError(t, client.DeleteGraphStoreCollectionSchema("project", "store", "9223372036854775807"))
			check(http.MethodDelete, "/graphstore/store/collectionschemas/9223372036854775807")
			require.NoError(t, client.DeleteGraphStore("project", "store"))
			check(http.MethodDelete, "/graphstore/store")

			responseBody = `{"projectId":2147483647,"storeName":"store","state":"READY","currentShardCount":2,"shards":[{"shardId":2},{"shardId":7}]}`
			store, err := client.GetGraphStore("project", "store")
			require.NoError(t, err)
			check(http.MethodGet, "/graphstore/store")
			require.Equal(t, int32(math.MaxInt32), store.ProjectID)
			require.Equal(t, int32(2), store.CurrentShardCount)
			require.Equal(t, "READY", store.State)
			require.Equal(t, []GraphStoreShardInfo{{ShardID: 2}, {ShardID: 7}}, store.Shards)
			responseBody = `{"projectId":2147483647,"storeName":"store","state":"READY","currentShardCount":2}`
			responseBody = `{"stores":[` + responseBody + `],"count":1,"total":9223372036854775807}`
			offset, size := 0, 1
			stores, err := client.ListGraphStores("project", &ListGraphStoresRequest{Offset: &offset, Size: &size})
			require.NoError(t, err)
			check(http.MethodGet, "/graphstore")
			require.Equal(t, url.Values{"offset": {"0"}, "size": {"1"}}, captured.URL.Query())
			require.Len(t, stores.Stores, 1)
			require.Equal(t, uint64(1), stores.Count)
			require.Equal(t, int64(math.MaxInt64), stores.Total)
			_, err = client.ListGraphStores("project", nil)
			require.NoError(t, err)
			require.Empty(t, captured.URL.RawQuery)
			responseBody = `{"projectId":1,"storeName":"store","storeId":9223372036854775807,"baseSchema":` + graphStoreTestJSON(t, graphStoreTestBase()) + `}`
			base, err := client.GetGraphStoreBaseSchema("project", "store")
			require.NoError(t, err)
			check(http.MethodGet, "/graphstore/store/baseschema")
			require.Equal(t, int64(math.MaxInt64), base.StoreID)
			require.Equal(t, int64(math.MaxInt64), base.BaseSchema.TTLSeconds)
			expected := graphStoreTestCollection("9223372036854775807")
			expected.Graph = &GraphStoreCollectionGraph{Label: []string{"infra", "host"}}
			schema := graphStoreTestJSON(t, expected)
			responseBody = `{"projectId":1,"storeName":"store","collectionSchema":` + schema + "}"
			collection, err := client.GetGraphStoreCollectionSchema("project", "store", "9223372036854775807")
			require.NoError(t, err)
			check(http.MethodGet, "/graphstore/store/collectionschemas/9223372036854775807")
			require.Equal(t, *expected, collection.CollectionSchema)
			responseBody = `{"projectId":1,"storeName":"store","collectionSchemas":[` + schema + `],"nextPageToken":"a+b/=&"}`
			page, err := client.ListGraphStoreCollectionSchemas("project", "store", 1, "a+b/=&")
			require.NoError(t, err)
			check(http.MethodGet, "/graphstore/store/collectionschemas")
			require.Equal(t, url.Values{"pageSize": {"1"}, "pageToken": {"a+b/=&"}}, captured.URL.Query())
			require.Equal(t, "a+b/=&", page.NextPageToken)
			require.Equal(t, collection.CollectionSchema, page.CollectionSchemas[0])
			responseBody = `{"projectId":1,"storeName":"store","collectionSchemas":[]}`
			page, err = client.ListGraphStoreCollectionSchemas("project", "store", 0, "")
			require.NoError(t, err)
			require.Empty(t, captured.URL.RawQuery)
			require.Empty(t, page.NextPageToken)
		})
	}
}

func TestGraphStoreInputAndResponseValidation(t *testing.T) {
	for _, shards := range []string{"null", "{}", "[null]", "[{}]", "[{\"shardId\":2147483648}]"} {
		var result GetGraphStoreResponse
		require.Error(t, json.Unmarshal([]byte(`{"projectId":1,"storeName":"store","state":"READY","currentShardCount":2,"shards":`+shards+`}`), &result))
	}
	client := CreateNormalInterface("example.com", "key", "secret", "")
	transport := testutil.NewMockTransport()
	client.SetHTTPClient(&http.Client{Transport: transport})
	for _, name := range []string{"", ".", "..", "a\\b", "a b", "../other", "a/b", "a%2fb", "a?x=y", "a#b", "雪", "a\x00b"} {
		_, err := client.GetGraphStoreCollectionSchema("project", "store", name)
		require.Error(t, err)
		require.Error(t, client.UpdateGraphStoreCollectionSchema("project", "store", name, graphStoreTestUpdate("aa")))
		require.Error(t, client.DeleteGraphStoreCollectionSchema("project", "store", name))
	}
	require.Error(t, client.UpdateGraphStoreBaseSchema("project", "store", nil))
	require.Error(t, client.UpdateGraphStoreBaseSchema("project", "../store", &GraphStoreBaseSchemaUpdate{}))
	require.Error(t, client.CreateGraphStoreCollectionSchema("project", "store", nil))
	require.Error(t, client.UpdateGraphStoreCollectionSchema("project", "store", "aa", nil))
	require.Error(t, client.UpdateGraphStoreCollectionSchema("project", "store", "aa", graphStoreTestUpdate("bb")))
	require.Error(t, client.CreateGraphStore("project", nil))
	require.Error(t, client.DeleteGraphStore("project", "../logstores/store"))
	_, err := client.GetGraphStore("", "store")
	require.Error(t, err)
	_, err = client.GetGraphStore("project", "")
	require.Error(t, err)
	require.Equal(t, 0, transport.GetTotalCallCount())
	for _, response := range []interface{}{
		&GraphStore{}, &GetGraphStoreResponse{}, &ListGraphStoresResponse{}, &GetGraphStoreBaseSchemaResponse{},
		&GetGraphStoreCollectionSchemaResponse{}, &ListGraphStoreCollectionSchemasResponse{},
	} {
		for _, body := range []string{"{}", "null", "[]", "{} {}", "{}\x00tail"} {
			require.Error(t, json.Unmarshal([]byte(body), response))
		}
	}
	for _, id := range []string{"null", "1.5", `"42"`, "9223372036854775808"} {
		require.Error(t, json.Unmarshal([]byte(`{"projectId":1,"storeName":"s","storeId":`+id+`,"baseSchema":`+graphStoreTestJSON(t, graphStoreTestBase())+`}`), &GetGraphStoreBaseSchemaResponse{}))
	}
	for _, name := range []string{"1.5", "42", "[]", "{}"} {
		schema := `{"description":"schema","graphLayout":"vertex","collectionKeyValues":[],"fields":[],"name":` + name + "}"
		require.Error(t, json.Unmarshal([]byte(`{"projectId":1,"storeName":"s","collectionSchema":`+schema+"}"), &GetGraphStoreCollectionSchemaResponse{}))
		require.Error(t, json.Unmarshal([]byte(`{"projectId":1,"storeName":"s","collectionSchemas":[`+schema+"]}"), &ListGraphStoreCollectionSchemasResponse{}))
	}
	require.Error(t, json.Unmarshal([]byte(`{"stores":[{}],"count":1,"total":1}`), &ListGraphStoresResponse{}))
	require.Error(t, json.Unmarshal([]byte(`{"stores":[],"count":-1,"total":0}`), &ListGraphStoresResponse{}))
	for _, schema := range []string{"[]", `"{}"`, "1", "true", "{bad}"} {
		require.Error(t, json.Unmarshal([]byte(`{"projectId":1,"storeName":"s","storeId":1,"baseSchema":`+schema+`}`), &GetGraphStoreBaseSchemaResponse{}))
	}
	var base GetGraphStoreBaseSchemaResponse
	require.NoError(t, json.Unmarshal([]byte(`{"projectId":1,"storeName":"s","storeId":1,"baseSchema":`+graphStoreTestJSON(t, graphStoreTestBase())+`}`), &base))
	require.Equal(t, int64(math.MaxInt64), base.BaseSchema.TTLSeconds)
	require.NoError(t, json.Unmarshal([]byte(`{"projectId":1,"storeName":"s","collectionSchemas":[],"nextPageToken":null}`), &ListGraphStoreCollectionSchemasResponse{}))
	for _, shards := range []string{`[]`, `[{"shardId":2},{"shardId":1}]`, `[{"shardId":1},{"shardId":1}]`} {
		var result GetGraphStoreResponse
		require.NoError(t, json.Unmarshal([]byte(`{"projectId":1,"storeName":"s","state":"READY","currentShardCount":42,"shards":`+shards+`,"diagnostic":true}`), &result))
	}
	require.Error(t, json.Unmarshal([]byte(`{"projectId":1.0,"storeName":"s","state":"READY","currentShardCount":1}`), &GraphStore{}))
	require.Error(t, json.Unmarshal([]byte(`{"projectId":2147483648,"storeName":"s","state":"READY","currentShardCount":1}`), &GraphStore{}))
}

func TestGraphStoreCollectionNamesStayOpaque(t *testing.T) {
	client := CreateNormalInterface("example.com", "key", "secret", "")
	transport := testutil.NewMockTransport()
	client.SetHTTPClient(&http.Client{Transport: transport})
	for _, name := range []string{"a.b", "a@b", "a.@b", "12", "1a", "aa", strings.Repeat("a", 63), "a", strings.Repeat("a", 64)} {
		// Configurable length/name rules belong to the service, not the SDK.
		transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "/graphstore/store/collectionschemas/"+name, req.URL.Path)
			body := ""
			if req.Method == http.MethodGet {
				body = `{"projectId":1,"storeName":"store","collectionSchema":` + graphStoreTestJSON(t, graphStoreTestCollection(name)) + `}`
			}
			return httpmock.NewStringResponse(200, body), nil
		})
		result, err := client.GetGraphStoreCollectionSchema("project", "store", name)
		require.NoError(t, err)
		require.Equal(t, name, result.CollectionSchema.Name)
		require.NoError(t, client.UpdateGraphStoreCollectionSchema("project", "store", name,
			graphStoreTestUpdate(name)))
		require.NoError(t, client.DeleteGraphStoreCollectionSchema("project", "store", name))
	}
}

func TestGraphStoreUpdateNameIsValidatedLocally(t *testing.T) {
	client := CreateNormalInterface("example.com", "key", "secret", "")
	transport := testutil.NewMockTransport()
	client.SetHTTPClient(&http.Client{Transport: transport})
	for _, name := range []string{"", "renamed"} {
		err := client.UpdateGraphStoreCollectionSchema("project", "store", "a.@b", graphStoreTestUpdate(name))
		require.Error(t, err)
	}
	require.Equal(t, 0, transport.GetTotalCallCount())
}

func TestGraphStoreErrorsAndTokenRefresh(t *testing.T) {
	normal := CreateNormalInterface("example.com", "key", "secret", "old-token")
	refreshes := 0
	token := &TokenAutoUpdateClient{
		logClient: normal, maxTryTimes: 3,
		tokenUpdateFunc: func() (string, string, string, time.Time, error) {
			refreshes++
			return "key", "secret", "new-token", time.Now().Add(time.Hour), nil
		},
	}
	transport := testutil.NewMockTransport()
	normal.SetHTTPClient(&http.Client{Transport: transport})
	calls := 0
	status := 400
	transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
		calls++
		response := httpmock.NewStringResponse(status, `{"errorCode":"ServiceError","errorMessage":"failed"}`)
		response.Header.Set("x-log-requestid", "error-request")
		return response, nil
	})
	for _, client := range []ClientInterface{normal, token} {
		for _, status = range []int{400, 403, 404, 409, 413, 500, 503} {
			before := calls
			err := client.DeleteGraphStore("project", "store")
			var serviceError *Error
			require.ErrorAs(t, err, &serviceError)
			require.Equal(t, int32(status), serviceError.HTTPCode)
			require.Equal(t, "error-request", serviceError.RequestID)
			require.Equal(t, before+1, calls)
		}
	}
	require.Zero(t, refreshes)
	transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("x-acs-security-token") == "old-token" {
			return httpmock.NewStringResponse(401, `{"errorCode":"Unauthorized","errorMessage":"expired"}`), nil
		}
		require.Equal(t, "new-token", req.Header.Get("x-acs-security-token"))
		return httpmock.NewStringResponse(200, ""), nil
	})
	require.NoError(t, token.DeleteGraphStore("project", "store"))
	require.Equal(t, 1, refreshes)
	transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("connection lost")
	})
	before := calls
	require.Error(t, token.CreateGraphStore("project", &CreateGraphStoreRequest{StoreName: "store", BaseSchema: graphStoreTestBase()}))
	require.Equal(t, before+1, calls)
}

func graphStoreTestBase() GraphStoreBaseSchema {
	return GraphStoreBaseSchema{TTLSeconds: math.MaxInt64,
		VertexLayout: GraphStoreVertexLayout{Label: "vlabel", ID: "vid"},
		EdgeLayout:   GraphStoreEdgeLayout{Label: "elabel", Source: GraphStoreEndpoint{Label: "sl", ID: "sid"}, Target: GraphStoreEndpoint{Label: "tl", ID: "tid"}}}
}

func graphStoreTestCollection(name string) *GraphStoreCollectionSchema {
	return &GraphStoreCollectionSchema{Name: name, Description: "schema", GraphLayout: GraphStoreLayoutVertex,
		CollectionKeyValues: []GraphStoreCollectionKeyValue{}, Fields: []GraphStoreFieldSchema{}}
}

func graphStoreTestUpdate(name string) *GraphStoreCollectionSchemaUpdate {
	return &GraphStoreCollectionSchemaUpdate{Name: name, Description: "schema", Fields: []GraphStoreFieldSchema{}}
}

func graphStoreTestJSON(t *testing.T, value interface{}) string {
	t.Helper()
	body, err := json.Marshal(value)
	require.NoError(t, err)
	return string(body)
}

func TestGraphStoreManagementBadResponseContext(t *testing.T) {
	client := CreateNormalInterface("example.com", "key", "secret", "")
	transport := testutil.NewMockTransport()
	client.SetHTTPClient(&http.Client{Transport: transport})
	transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
		response := httpmock.NewStringResponse(200, `{"projectId":2147483648}`)
		response.Header.Set(RequestIDHeader, "management-request")
		return response, nil
	})
	_, err := client.GetGraphStore("project", "store")
	var bad *BadResponseError
	require.ErrorAs(t, err, &bad)
	require.Equal(t, 200, bad.HTTPCode)
	require.Equal(t, "management-request", http.Header(bad.RespHeader).Get(RequestIDHeader))
	require.Equal(t, 1, transport.GetTotalCallCount())
}

func TestGraphStoreSchemaBusinessRulesReachService(t *testing.T) {
	client := CreateNormalInterface("example.com", "key", "secret", "")
	transport := testutil.NewMockTransport()
	client.SetHTTPClient(&http.Client{Transport: transport})
	transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		require.True(t, json.Valid(body))
		return httpmock.NewStringResponse(400, `{"errorCode":"InvalidParameter","errorMessage":"invalid schema"}`), nil
	})
	negative := int64(-1)
	for _, err := range []error{
		client.CreateGraphStore("project", &CreateGraphStoreRequest{StoreName: "store", BaseSchema: GraphStoreBaseSchema{TTLSeconds: -1}}),
		client.CreateGraphStoreCollectionSchema("project", "store", &GraphStoreCollectionSchema{TTLSeconds: &negative}),
		client.UpdateGraphStoreCollectionSchema("project", "store", "one", &GraphStoreCollectionSchemaUpdate{Name: "one", TTLSeconds: &negative}),
	} {
		var serviceError *Error
		require.ErrorAs(t, err, &serviceError)
		require.Equal(t, int32(400), serviceError.HTTPCode)
	}
	require.Equal(t, 3, transport.GetTotalCallCount())
}
