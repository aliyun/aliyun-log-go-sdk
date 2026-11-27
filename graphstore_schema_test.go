package sls

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGraphStoreTypedSchemaWireAndCompatibility(t *testing.T) {
	base := graphStoreTestBase()
	base.VertexLayout.CK = []string{}
	base.VertexLayout.KeyIndexConfigs = map[string]GraphStoreIndexConfig{}
	base.EdgeLayout.RouteKey = []string{"sl"}
	base.FullTextIndex = &GraphStoreFullTextIndex{Token: " ,", CaseSensitive: true}
	encoded := graphStoreTestJSON(t, base)
	require.Contains(t, encoded, `"graphLayouts"`)
	require.NotContains(t, encoded, `"VertexLayout"`)
	require.Contains(t, encoded, `"ck":[]`)
	require.Contains(t, encoded, `"keyIndexConfigs":{}`)
	require.NotContains(t, encoded, `"pk"`)
	var read GraphStoreBaseSchema
	require.NoError(t, json.Unmarshal([]byte(encoded), &read))
	require.Equal(t, base, read)
	for _, replacement := range [][2]string{
		{`"ttlSeconds":9223372036854775807`, `"ttlSeconds":9223372036854775808`},
		{`"ttlSeconds":9223372036854775807`, `"ttlSeconds":1.5`},
		{`"timeStateEnable":false`, `"timeStateEnable":"false"`},
	} {
		bad := strings.Replace(encoded, replacement[0], replacement[1], 1)
		require.NotEqual(t, encoded, bad)
		require.Error(t, json.Unmarshal([]byte(bad), &GraphStoreBaseSchema{}), bad)
	}
	ttl := int64(math.MaxInt64)
	collection := graphStoreTestCollection("雪\x00")
	collection.TTLSeconds = &ttl
	collection.CollectionKeyValues = []GraphStoreCollectionKeyValue{{Name: "vlabel", Value: ""}}
	collection.Fields = []GraphStoreFieldSchema{
		{Name: "exact", Type: GraphStoreFieldInt, IndexConfig: GraphStoreIndexConfig{Enable: true}},
		{Name: "full", Type: GraphStoreFieldString, IndexConfig: GraphStoreIndexConfig{Enable: true, FullTextIndex: base.FullTextIndex}},
		{Name: "range", Type: GraphStoreFieldDouble, IndexConfig: GraphStoreIndexConfig{Enable: true, RangeIndex: &GraphStoreRangeIndex{}}},
		{Name: "bool", Type: GraphStoreFieldBool},
	}
	encoded = graphStoreTestJSON(t, collection)
	var decoded GraphStoreCollectionSchema
	require.NoError(t, json.Unmarshal([]byte(encoded), &decoded))
	require.Equal(t, *collection, decoded)
	for _, replacement := range [][2]string{
		{`"enable":true`, `"enable":"true"`},
		{`"fields":`, `"fields":"wrong","ignored":`},
	} {
		bad := strings.Replace(encoded, replacement[0], replacement[1], 1)
		require.NotEqual(t, encoded, bad)
		require.Error(t, json.Unmarshal([]byte(bad), &GraphStoreCollectionSchema{}), bad)
	}
	for _, ttl := range []*int64{nil, new(int64), &ttl} {
		update := graphStoreTestUpdate("collection")
		update.TTLSeconds = ttl
		body := graphStoreTestJSON(t, update)
		require.NotContains(t, body, "graphLayout")
		require.NotContains(t, body, "collectionKeyValues")
		var decoded GraphStoreCollectionSchemaUpdate
		require.NoError(t, json.Unmarshal([]byte(body), &decoded))
		require.Equal(t, *update, decoded)
	}
	var update GraphStoreCollectionSchemaUpdate
	require.NoError(t, json.Unmarshal([]byte(`{"name":"c","description":"schema","ttlSeconds":null,"fields":[]}`), &update))
	require.Nil(t, update.TTLSeconds)
	negative := int64(-1)
	collection.TTLSeconds = &negative
	require.NoError(t, json.Unmarshal([]byte(graphStoreTestJSON(t, collection)), &decoded))
	require.Equal(t, &negative, decoded.TTLSeconds)
}

func TestGraphStoreSchemaNativeDefaults(t *testing.T) {
	var collection GraphStoreCollectionSchema
	require.NoError(t, json.Unmarshal([]byte(`{"graphLayout":"future","fields":[null,{"type":"UNKNOWN","newOption":true}],"ttlSeconds":-1}`), &collection))
	require.Empty(t, collection.Name)
	require.Nil(t, collection.CollectionKeyValues)
	require.Equal(t, GraphStoreLayoutType("future"), collection.GraphLayout)
	require.Equal(t, GraphStoreFieldType("UNKNOWN"), collection.Fields[1].Type)
	require.Equal(t, int64(-1), *collection.TTLSeconds)
	var base GraphStoreBaseSchema
	require.NoError(t, json.Unmarshal([]byte(`{"newOption":true,"graphLayouts":[null,{"name":"vertex","id":"v"},{"name":"other"}]}`), &base))
	require.Equal(t, "v", base.VertexLayout.ID)
	require.Empty(t, base.VertexLayout.Label)
	require.Empty(t, base.EdgeLayout.Label)
	require.Nil(t, base.CommonFieldNames)
	for _, body := range []string{`{"projectId":1,"storeName":"s","storeId":1}`, `{"projectId":1,"storeName":"s","storeId":1,"baseSchema":null,"collectionSchema":{},"collectionSchemas":[null,{}]}`} {
		var base GetGraphStoreBaseSchemaResponse
		var schema GetGraphStoreCollectionSchemaResponse
		var list ListGraphStoreCollectionSchemasResponse
		require.NoError(t, json.Unmarshal([]byte(body), &base))
		require.NoError(t, json.Unmarshal([]byte(body), &schema))
		require.NoError(t, json.Unmarshal([]byte(body), &list))
		require.Empty(t, base.BaseSchema.VertexLayout.ID)
		require.Empty(t, schema.CollectionSchema.Name)
	}
}

func TestGraphStoreVersionIsReadOnly(t *testing.T) {
	var base GraphStoreBaseSchema
	require.NoError(t, json.Unmarshal([]byte(`{"version":1}`), &base))
	require.Equal(t, uint32(1), base.Version)
	require.Contains(t, graphStoreTestJSON(t, base), `"version":1`)
	request := CreateGraphStoreRequest{StoreName: "s", ShardCount: 2, BaseSchema: base}
	require.NotContains(t, graphStoreTestJSON(t, request), `"version"`)
	require.Equal(t, uint32(1), request.BaseSchema.Version)
	for _, value := range []string{`-1`, `4294967296`, `1.5`, `"1"`} {
		require.Error(t, json.Unmarshal([]byte(`{"version":`+value+`}`), &GraphStoreBaseSchema{}))
	}
}

func TestGraphStoreOrderedLabels(t *testing.T) {
	var base GraphStoreBaseSchema
	require.NoError(t, json.Unmarshal([]byte(`{"cmsMode":true,"timeStateEnable":true,"graphLayouts":[{"name":"vertex","label":["domain","type"],"id":"id"},{"name":"edge","label":"relation","source":{"label":"old_src","id":"sid"},"target":{"label":["dst_domain","dst_type"],"id":"tid"}}]}`), &base))
	require.True(t, base.CMSMode)
	require.True(t, base.TimeStateEnable)
	require.Equal(t, []string{"domain", "type"}, base.VertexLayout.LabelFields())
	require.Equal(t, []string{"old_src"}, base.EdgeLayout.Source.LabelFields())
	require.Equal(t, []string{"dst_domain", "dst_type"}, base.EdgeLayout.Target.LabelFields())
	require.Equal(t, "relation", base.EdgeLayout.Label)
	require.Contains(t, graphStoreTestJSON(t, base), `"label":["old_src"]`)
	require.Error(t, json.Unmarshal([]byte(`{"label":["type",1]}`), &GraphStoreVertexLayout{}))

	var collection GraphStoreCollectionSchema
	require.NoError(t, json.Unmarshal([]byte(`{"graphLayout":"edge","graph":{"label":"contains","sourceLabel":["infra","host"],"targetLabel":["app","pod"]}}`), &collection))
	require.Equal(t, &GraphStoreCollectionGraph{Label: "contains", SourceLabel: []string{"infra", "host"}, TargetLabel: []string{"app", "pod"}}, collection.Graph)
}
