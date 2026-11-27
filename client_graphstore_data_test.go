package sls

import (
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aliyun/aliyun-log-go-sdk/internal/testutil"
	"github.com/gogo/protobuf/proto"
	"github.com/jarcoal/httpmock"
	"github.com/pierrec/lz4/v4"
	"github.com/stretchr/testify/require"
)

const graphStoreRows = `{"data":{"fields":["a","b","node"],"values":[[null,"","{\"id\":\"雪\"}"],["x\u0000y","雪","s"]]},"status":"incomplete","stats":{"rowCount":2,"elapsedMs":-9223372036854775808,"peakMemoryBytes":18446744073709551615},"warnings":["","雪"]}`

func TestGraphStoreQueryPreservesTypedJSON(t *testing.T) {
	body := []byte(`{"status":"success","data":{"fields":["v"],"values":[[{"$type":"Integer","_value":"9223372036854775807"}],[{"$type":"Integer","_value":"-9223372036854775808"}],[{"$type":"Float","_value":"1.0"}],[{"$type":"String","_value":"1.0"}],[{"$type":"Float","_value":"NaN"}],[{"$type":"Boolean","_value":true}],[{"$type":"Null","_value":null}],[{"$type":"String","_value":""}],[{"$type":"String","_value":"雪\u0000tail"}],[{"$type":"List","_value":[{"$type":"Integer","_value":"9007199254740993"},{"$type":"Null","_value":null}]}],[{"$type":"Map","_value":{"$type":{"$type":"String","_value":"business"},"_value":{"$type":"String","_value":"[1,2]"}}}],[{"$type":"Node","_value":{"_element_id":"a","_labels":["N"],"_properties":{"n":{"$type":"Integer","_value":"9223372036854775807"}}}}],[{"$type":"Relationship","_value":{"_element_id":"r","_start_node_element_id":"a","_end_node_element_id":"b","_type":"R","_properties":{}}}],[{"$type":"Path","_value":[{"$type":"Node","_value":{"_element_id":"a","_labels":["N"],"_properties":{"n":{"$type":"Integer","_value":"9223372036854775807"}}}},{"$type":"Relationship","_value":{"_element_id":"r","_start_node_element_id":"a","_end_node_element_id":"b","_type":"R","_properties":{}}},{"$type":"Node","_value":{"_element_id":"b","_labels":[],"_properties":{}}}]}],[{"$type":"Date","_value":"2026-10-21"}],[{"$type":"Point","_value":"SRID=7203;POINT (1 2)"}],[{"$type":"FutureType","_value":{"opaque":[1,null]}}]]}}`)
	for _, version := range []AuthVersionType{AuthV1, AuthV4} {
		client := CreateNormalInterfaceV2("http://example.com", NewStaticCredentialsProvider("key", "secret", ""))
		client.SetAuthVersion(version)
		client.SetRegion("cn-test")
		transport := testutil.NewMockTransport()
		client.SetHTTPClient(&http.Client{Transport: transport})
		transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
			return httpmock.NewStringResponse(200, string(body)), nil
		})
		request := NewGetGraphStoreDataRequest("RETURN n")
		request.ResultFormat = "typed_json"
		result, err := client.GetGraphStoreData("project", "store", request)
		require.NoError(t, err)
		encoded, err := json.Marshal(result)
		require.NoError(t, err)
		require.JSONEq(t, string(body), string(encoded))
		require.Equal(t, "9223372036854775807", result.Data.Values[0][0].(map[string]interface{})["_value"])
		require.Equal(t, "雪\x00tail", result.Data.Values[8][0].(map[string]interface{})["_value"])
	}
}

func TestGraphStoreQueryResultFormatWire(t *testing.T) {
	for _, version := range []AuthVersionType{AuthV1, AuthV4} {
		client := CreateNormalInterfaceV2("http://example.com", NewStaticCredentialsProvider("key", "secret", ""))
		client.SetAuthVersion(version)
		client.SetRegion("cn-test")
		transport := testutil.NewMockTransport()
		client.SetHTTPClient(&http.Client{Transport: transport})
		for _, format := range []string{"", "json", "typed_json", "future-format"} {
			transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
				var body map[string]interface{}
				require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
				if format == "" {
					require.NotContains(t, body, "resultFormat")
				} else {
					require.Equal(t, format, body["resultFormat"])
				}
				cell := `42`
				if format == "typed_json" {
					cell = `{"$type":"Integer","_value":"42"}`
				}
				return httpmock.NewStringResponse(200, `{"status":"success","data":{"fields":["n"],"values":[[`+cell+`]]}}`), nil
			})
			request := NewGetGraphStoreDataRequest("RETURN 42 AS n")
			request.ResultFormat = format
			response, err := client.GetGraphStoreData("project", "store", request)
			require.NoError(t, err)
			if format == "typed_json" {
				require.Equal(t, map[string]interface{}{"$type": "Integer", "_value": "42"}, response.Data.Values[0][0])
			} else {
				require.Equal(t, json.Number("42"), response.Data.Values[0][0])
			}
		}
	}
}

func TestGraphStoreQueryStructuredValues(t *testing.T) {
	body := []byte(`{"data":{"fields":["v"],"values":[[9223372036854775807],[true],[[1,{"name":"雪","text":"[1,2]"}]],[{"labels":["person"],"properties":{"age":30}}],["123"],["true"],["[]"],[null],[""]]},"status":"success"}`)
	result, err := decodeGraphStoreQuery(body)
	require.NoError(t, err)
	require.Equal(t, json.Number("9223372036854775807"), result.Data.Values[0][0])
	require.Equal(t, true, result.Data.Values[1][0])
	require.Equal(t, []interface{}{json.Number("1"), map[string]interface{}{"name": "雪", "text": "[1,2]"}}, result.Data.Values[2][0])
	require.Equal(t, map[string]interface{}{"labels": []interface{}{"person"}, "properties": map[string]interface{}{"age": json.Number("30")}}, result.Data.Values[3][0])
	for i, value := range []interface{}{"123", "true", "[]", nil, ""} {
		require.Equal(t, value, result.Data.Values[i+4][0])
	}
	for _, version := range []AuthVersionType{AuthV1, AuthV4} {
		client := CreateNormalInterfaceV2("http://example.com", NewStaticCredentialsProvider("key", "secret", ""))
		client.SetAuthVersion(version)
		client.SetRegion("cn-test")
		transport := testutil.NewMockTransport()
		client.SetHTTPClient(&http.Client{Transport: transport})
		transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
			response := httpmock.NewStringResponse(200, string(body))
			response.Header.Set(RequestIDHeader, "structured-request")
			return response, nil
		})
		decoded, err := client.GetGraphStoreData("project", "store", NewGetGraphStoreDataRequest("RETURN 1"))
		require.NoError(t, err)
		require.Equal(t, result.Data.Values, decoded.Data.Values)
		require.Equal(t, "structured-request", decoded.RequestID)
	}
	for _, suffix := range []string{" null", " trailing", "\x00"} {
		_, err := decodeGraphStoreQuery(append(append([]byte(nil), body...), suffix...))
		require.Error(t, err)
	}
}

func TestGraphStoreDataWire(t *testing.T) {
	for _, version := range []AuthVersionType{AuthV1, AuthV4} {
		for _, token := range []string{"", "token"} {
			t.Run(string(version)+"/"+token, func(t *testing.T) {
				client := CreateNormalInterfaceV2("http://example.com", NewStaticCredentialsProvider("key", "secret", token))
				client.SetAuthVersion(version)
				client.SetRegion("cn-test")
				transport := testutil.NewMockTransport()
				client.SetHTTPClient(&http.Client{Transport: transport})
				var captured *http.Request
				var sent []byte
				transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
					captured = req
					var err error
					sent, err = io.ReadAll(req.Body)
					require.NoError(t, err)
					response := httpmock.NewStringResponse(200, graphStoreRows)
					response.Header.Set(RequestIDHeader, "data-request")
					return response, nil
				})
				check := func(path, contentType string, rawSize int) {
					t.Helper()
					require.Equal(t, http.MethodPost, captured.Method)
					require.Equal(t, "/graphstore/store/"+path, captured.URL.Path)
					require.Empty(t, captured.URL.RawQuery)
					require.Equal(t, "project.example.com", captured.URL.Host)
					require.Equal(t, contentType, captured.Header.Get("Content-Type"))
					require.Equal(t, strconv.Itoa(rawSize), captured.Header.Get(HTTPHeaderBodyRawSize))
					require.Equal(t, token, captured.Header.Get(HTTPHeaderAcsSecurityToken))
					if version == AuthV4 {
						hash := sha256.Sum256(sent)
						require.Equal(t, hex.EncodeToString(hash[:]), captured.Header.Get("x-log-content-sha256"))
						require.True(t, strings.HasPrefix(captured.Header.Get("Authorization"), "SLS4-HMAC-SHA256"))
					} else {
						hash := md5.Sum(sent)
						require.Equal(t, strings.ToUpper(hex.EncodeToString(hash[:])), captured.Header.Get("Content-MD5"))
						require.True(t, strings.HasPrefix(captured.Header.Get("Authorization"), "LOG key:"))
					}
				}
				groups := []*LogGroup{
					graphStoreTestGroup("vertex"), // Incompressible LZ4 block.
					{Logs: []*Log{{Time: proto.Uint32(1), Contents: []*LogContent{
						graphStoreTestField("__layout__", "edge"),
						{Key: proto.String("\x00\xffkey"), Value: proto.String("\x00\xff" + strings.Repeat("雪", 256))},
					}}}},
				}
				for _, group := range groups {
					raw, err := proto.Marshal(group)
					require.NoError(t, err)
					for _, compression := range []int{Compress_None, Compress_LZ4, Compress_ZSTD, Compress_GZIP} {
						require.NoError(t, client.PostGraphStoreData("project", "store", &PostGraphStoreDataRequest{LogGroup: group, CompressType: compression}))
						check("write", "application/x-protobuf", len(raw))
						decoded := sent
						switch compression {
						case Compress_LZ4:
							require.Equal(t, "lz4", captured.Header.Get("x-log-compresstype"))
							decoded = make([]byte, len(raw))
							n, err := lz4.UncompressBlock(sent, decoded)
							require.NoError(t, err)
							require.Equal(t, len(raw), n)
						case Compress_ZSTD:
							require.Equal(t, "zstd", captured.Header.Get("x-log-compresstype"))
							decoded, err = slsZstdCompressor.Decompress(sent, nil)
							require.NoError(t, err)
						case Compress_GZIP:
							require.Equal(t, "deflate", captured.Header.Get("x-log-compresstype"))
							decoded = decodeGraphStoreDeflate(t, sent)
						default:
							require.Empty(t, captured.Header.Get("x-log-compresstype"))
						}
						require.Equal(t, raw, decoded)
					}
				}
				query := &GetGraphStoreDataRequest{Query: "RETURN '雪\\\"\n\x00'", Start: 0, End: 4294967295, Session: "opaque\x00session"}
				result, err := client.GetGraphStoreData("project", "store", query)
				require.NoError(t, err)
				check("query", "application/json", len(sent))
				require.Equal(t, "lz4", captured.Header.Get("Accept-Encoding"))
				var actual GetGraphStoreDataRequest
				require.NoError(t, json.Unmarshal(sent, &actual))
				require.Equal(t, *query, actual)
				require.Equal(t, "data-request", result.RequestID)
				require.Nil(t, result.Data.Values[0][0])
				require.Equal(t, "", result.Data.Values[0][1])
				require.Equal(t, `{"id":"雪"}`, result.Data.Values[0][2])
				require.Equal(t, "x\x00y", result.Data.Values[1][0])
				require.Equal(t, int64(-9223372036854775808), result.Stats.ElapsedMs)
				require.Equal(t, []string{"", "雪"}, result.Warnings)
				require.Equal(t, "incomplete", result.Status)
				require.Equal(t, uint64(18446744073709551615), result.Stats.PeakMemoryBytes)
			})
		}
	}
}

func TestGraphStoreDataInput(t *testing.T) {
	client := CreateNormalInterface("example.com", "key", "secret", "")
	transport := testutil.NewMockTransport()
	client.SetHTTPClient(&http.Client{Transport: transport})
	for _, group := range []*LogGroup{nil, {}, {Logs: []*Log{}}} {
		for _, compression := range []int{Compress_LZ4, Compress_None, Compress_ZSTD} {
			require.Error(t, client.PostGraphStoreData("project", "store", &PostGraphStoreDataRequest{LogGroup: group, CompressType: compression}))
		}
		for _, compression := range []int{-1, Compress_Max} {
			require.Equal(t, InvalidCompressError, client.PostGraphStoreData("project", "store", &PostGraphStoreDataRequest{LogGroup: group, CompressType: compression}))
		}
	}
	write := &PostGraphStoreDataRequest{LogGroup: graphStoreTestGroup("vertex")}
	require.Error(t, client.PostGraphStoreData("project", "store", nil))
	for _, names := range [][2]string{{"", "store"}, {"project", ""}, {"project", "a/b"}, {"project", "a?b"}} {
		require.Error(t, client.PostGraphStoreData(names[0], names[1], write))
		result, err := client.GetGraphStoreData(names[0], names[1], &GetGraphStoreDataRequest{Query: "RETURN 1", Start: 10, End: 20})
		require.Error(t, err)
		require.Nil(t, result)
	}
	for _, query := range []*GetGraphStoreDataRequest{nil, {}, {Query: "q", Start: -1, End: 1}, {Query: "q", Start: 2, End: 1}} {
		result, err := client.GetGraphStoreData("project", "store", query)
		require.Error(t, err)
		require.Nil(t, result)
	}
	write.LogGroup.Logs[0].Time = nil
	require.Error(t, client.PostGraphStoreData("project", "store", write))
	require.Zero(t, transport.GetTotalCallCount())
}

func TestGraphStoreClosedInt64TimeRange(t *testing.T) {
	client := CreateNormalInterface("example.com", "key", "secret", "")
	transport := testutil.NewMockTransport()
	client.SetHTTPClient(&http.Client{Transport: transport})
	for _, query := range []*GetGraphStoreDataRequest{
		NewGetGraphStoreDataRequest("RETURN 1"),
		{Query: "RETURN 1", Start: 0, End: 0},
		{Query: "RETURN 1", Start: 4294967296, End: 9223372036854775807},
	} {
		transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			var actual map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(body, &actual))
			require.NotContains(t, actual, "from")
			require.NotContains(t, actual, "to")
			require.Equal(t, strconv.FormatInt(query.Start, 10), string(actual["start"]))
			require.Equal(t, strconv.FormatInt(query.End, 10), string(actual["end"]))
			require.NotContains(t, actual, "session")
			return httpmock.NewStringResponse(200, graphStoreRows), nil
		})
		_, err := client.GetGraphStoreData("project", "store", query)
		require.NoError(t, err)
	}
}

func TestGraphStoreQueryResponses(t *testing.T) {
	client := CreateNormalInterface("example.com", "key", "secret", "")
	transport := testutil.NewMockTransport()
	client.SetHTTPClient(&http.Client{Transport: transport})
	body := graphStoreRows
	compression, rawSize := "", ""
	transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
		response := httpmock.NewStringResponse(200, body)
		response.Header.Set("x-log-compresstype", compression)
		response.Header.Set(HTTPHeaderBodyRawSize, rawSize)
		response.Header.Set(RequestIDHeader, "query-request")
		return response, nil
	})
	query := func(valid bool) {
		t.Helper()
		before := transport.GetTotalCallCount()
		result, err := client.GetGraphStoreData("project", "store", &GetGraphStoreDataRequest{Query: "RETURN 1", Start: 10, End: 20})
		if valid {
			require.NoError(t, err)
			require.NotNil(t, result)
		} else {
			var badResponse *BadResponseError
			require.ErrorAs(t, err, &badResponse, body)
			require.Equal(t, 200, badResponse.HTTPCode)
			require.Equal(t, "query-request", http.Header(badResponse.RespHeader).Get(RequestIDHeader))
			require.Nil(t, result)
		}
		require.Equal(t, before+1, transport.GetTotalCallCount())
	}
	query(true)
	for _, body = range []string{"", "[]", graphStoreRows + "{}", graphStoreRows + "\x00"} {
		query(false)
	}
	for _, replacement := range [][2]string{
		{`"fields":["a","b","node"]`, `"fields":[1,"b","node"]`},
		{`"status":"incomplete"`, `"status":1`},
		{`"rowCount":2`, `"rowCount":-1`},
		{`"peakMemoryBytes":18446744073709551615`, `"peakMemoryBytes":18446744073709551616`},
		{`"elapsedMs":-9223372036854775808`, `"elapsedMs":-9223372036854775809`},
		{`"elapsedMs":-9223372036854775808`, `"elapsedMs":1.5`},
		{`"elapsedMs":-9223372036854775808`, `"elapsedMs":"1"`},
		{`"warnings":["","雪"]`, `"warnings":[false]`},
	} {
		body = strings.Replace(graphStoreRows, replacement[0], replacement[1], 1)
		require.NotEqual(t, graphStoreRows, body)
		query(false)
	}
	for _, replacement := range [][2]string{
		{`"fields":["a","b","node"]`, `"fields":[null,"b","node"]`},
		{`[null,"","{\"id\":\"雪\"}"]`, `null`},
		{`[null,"","{\"id\":\"雪\"}"]`, `[]`},
		{`"status":"incomplete"`, `"diagnostic":true,"status":"incomplete"`},
		{`"warnings":["","雪"]`, `"warnings":[null]`},
		{`"data":{`, `"diagnostic":{},"data":{"extra":1,`},
	} {
		body = strings.Replace(graphStoreRows, replacement[0], replacement[1], 1)
		require.NotEqual(t, graphStoreRows, body)
		query(true)
	}
	body = strings.Replace(graphStoreRows, "雪", "\xff", 1)
	query(true) // encoding/json replaces invalid UTF-8, as it does for LogStore.
	for _, scope := range []string{"", "stats", "data"} {
		var outer map[string]json.RawMessage
		require.NoError(t, json.Unmarshal([]byte(graphStoreRows), &outer))
		fields := outer
		if scope != "" {
			fields = nil
			require.NoError(t, json.Unmarshal(outer[scope], &fields))
		}
		for key, original := range fields {
			for _, null := range []bool{true, false} {
				if null {
					fields[key] = json.RawMessage("null")
				} else {
					delete(fields, key)
				}
				if scope != "" {
					outer[scope], _ = json.Marshal(fields)
				}
				encoded, err := json.Marshal(outer)
				require.NoError(t, err)
				body = string(encoded)
				query(true)
			}
			fields[key] = original
		}
	}
	body = `{"data":{"fields":[],"values":[]},"status":"success"}`
	query(true)
	body = strings.Replace(body, `"values":[]`, `"values":[null]`, 1)
	query(true)
	compressed := make([]byte, lz4.CompressBlockBound(len(graphStoreRows)))
	n, err := lz4.CompressBlock([]byte(graphStoreRows), compressed, nil)
	require.NoError(t, err)
	if n == 0 {
		n, _ = copyIncompressible([]byte(graphStoreRows), compressed)
	}
	body, compression, rawSize = string(compressed[:n]), "lz4", strconv.Itoa(len(graphStoreRows))
	query(true)
	for _, body = range []string{string(compressed[:n-1]), string(compressed[:n]) + "\x00"} {
		query(false)
	}
	body = string(compressed[:n])
	for _, rawSize = range []string{"", "-1", "x", "9223372036854775808", "0", strconv.Itoa(len(graphStoreRows) + 1)} {
		query(false)
	}
	body, rawSize = "not-lz4", strconv.Itoa(len(graphStoreRows))
	query(false)
	for _, compression = range []string{"none", "zstd", "deflate", "unknown"} {
		query(false)
	}
}

func TestGraphStoreDataRetriesAndToken(t *testing.T) {
	oldRetry := RetryOnServerErrorEnabled
	defer func() { RetryOnServerErrorEnabled = oldRetry }()
	for _, operation := range []string{"write", "batch", "query"} {
		t.Run(operation, func(t *testing.T) {
			client := CreateNormalInterface("example.com", "key", "secret", "old-token")
			client.SetRetryTimeout(2 * time.Second)
			transport := testutil.NewMockTransport()
			client.SetHTTPClient(&http.Client{Transport: transport})
			invoke := func(c ClientInterface) error {
				switch operation {
				case "write":
					return c.PostGraphStoreData("project", "store", &PostGraphStoreDataRequest{LogGroup: graphStoreTestGroup("vertex")})
				case "batch":
					return c.BatchPostGraphStoreData("project", "store", &BatchPostGraphStoreDataRequest{LogGroups: []*LogGroup{graphStoreTestGroup("vertex")}})
				}
				_, err := c.GetGraphStoreData("project", "store", &GetGraphStoreDataRequest{Query: "RETURN 1", Start: 10, End: 20})
				return err
			}
			calls, status, failCount := 0, 500, 1
			transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
				calls++
				if calls > failCount {
					return httpmock.NewStringResponse(200, graphStoreRows), nil
				}
				errorBody := `{"errorCode":"ServiceError","errorMessage":"failed"}`
				if operation == "query" {
					errorBody = `{"status":"error","error_code":"ServiceError","error":"failed"}`
				}
				response := httpmock.NewStringResponse(status, errorBody)
				response.Header.Set(RequestIDHeader, "last-request")
				return response, nil
			})
			RetryOnServerErrorEnabled = true
			for _, status = range []int{500, 502, 503} {
				calls = 0
				require.NoError(t, invoke(client))
				require.Equal(t, 2, calls)
			}
			for _, status = range []int{400, 401, 403, 404, 409, 413, 501, 504} {
				calls = 0
				var serviceError *Error
				require.ErrorAs(t, invoke(client), &serviceError)
				require.Equal(t, int32(status), serviceError.HTTPCode)
				require.Equal(t, "last-request", serviceError.RequestID)
				require.Equal(t, "ServiceError", serviceError.Code)
				require.Equal(t, "failed", serviceError.Message)
				require.Equal(t, 1, calls)
			}
			RetryOnServerErrorEnabled = false
			calls, status = 0, 503
			require.Error(t, invoke(client))
			require.Equal(t, 1, calls)
			RetryOnServerErrorEnabled = true
			client.SetRetryTimeout(10 * time.Millisecond)
			calls, failCount = 0, 100
			require.ErrorContains(t, invoke(client), "stopped retrying")
			require.Equal(t, 1, calls)
			refreshes := 0
			token := &TokenAutoUpdateClient{logClient: client, maxTryTimes: 3, tokenUpdateFunc: func() (string, string, string, time.Time, error) {
				refreshes++
				return "key", "secret", "new-token", time.Now().Add(time.Hour), nil
			}}
			transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
				if req.Header.Get(HTTPHeaderAcsSecurityToken) == "old-token" {
					return httpmock.NewStringResponse(401, `{"errorCode":"Unauthorized","errorMessage":"expired"}`), nil
				}
				require.Equal(t, "new-token", req.Header.Get(HTTPHeaderAcsSecurityToken))
				return httpmock.NewStringResponse(200, graphStoreRows), nil
			})
			require.NoError(t, invoke(token))
			require.Equal(t, 1, refreshes)
			calls = 0
			transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) { calls++; return nil, errors.New("connection lost") })
			require.Error(t, invoke(client))
			require.Equal(t, 1, calls)
			transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
				time.Sleep(20 * time.Millisecond)
				return httpmock.NewStringResponse(200, graphStoreRows), nil
			})
			require.NoError(t, invoke(client)) // Native retryTimeout does not cancel an in-flight HTTP request.
		})
	}
}

func TestGraphStoreQueryOutcome(t *testing.T) {
	for _, status := range []string{"success", "incomplete", "error", "unknown", ""} {
		result, err := decodeGraphStoreQuery([]byte(`{"status":"` + status + `","error_code":"InvalidQuery","error":"detail"}`))
		require.NoError(t, err)
		require.Equal(t, status, result.Status)
		require.Nil(t, result.Stats)
		require.Nil(t, result.Warnings)
		require.Nil(t, result.Data.Fields)
		require.Nil(t, result.Data.Values)
	}
	for _, body := range []string{
		`null`, `{}`, `{"data":null,"status":null,"stats":null,"warnings":null}`,
		`{"data":{}}`, `{"data":{"fields":null,"values":null}}`,
	} {
		result, err := decodeGraphStoreQuery([]byte(body))
		require.NoError(t, err, body)
		require.Equal(t, &GetGraphStoreDataResponse{}, result, body)
	}
	for _, body := range []string{`{"stats":{}}`, `{"stats":{"rowCount":null,"elapsedMs":null,"peakMemoryBytes":null}}`} {
		result, err := decodeGraphStoreQuery([]byte(body))
		require.NoError(t, err)
		require.Equal(t, &GraphStoreQueryStats{}, result.Stats)
	}
	result, err := decodeGraphStoreQuery([]byte(`{"stats":{"rowCount":7},"warnings":[],"data":{"fields":[],"values":[]}}`))
	require.NoError(t, err)
	require.Equal(t, &GraphStoreQueryStats{RowCount: 7}, result.Stats)
	require.Equal(t, []string{}, result.Warnings)
	require.Equal(t, []string{}, result.Data.Fields)
	require.Equal(t, [][]interface{}{}, result.Data.Values)
}

func TestGraphStoreQueryMeta(t *testing.T) {
	oldRetry := RetryOnServerErrorEnabled
	RetryOnServerErrorEnabled = false
	defer func() { RetryOnServerErrorEnabled = oldRetry }()
	client := CreateNormalInterface("example.com", "key", "secret", "")
	transport := testutil.NewMockTransport()
	client.SetHTTPClient(&http.Client{Transport: transport})
	for _, meta := range []string{"", "null", "{}", `{"requestId":"worker","durationMs":0,"extra":{"text":"雪\u0000tail","n":18446744073709551615,"nested":[null,true,{"error":"original"}]}}`} {
		for _, status := range []int{200, 400, 500} {
			for _, format := range []string{"json", "typed_json"} {
				for _, compressed := range []bool{false, true} {
					if status != 200 && compressed {
						continue
					}
					for _, outcome := range []string{"success", "incomplete"} {
						body := `{"status":"` + outcome + `","data":{"fields":[],"values":[]}`
						if status != 200 {
							body = `{"status":"error","error_code":"InternalServerError","error":"Public message unchanged"`
						}
						if meta != "" {
							body += `,"meta":` + meta
						}
						body += "}"
						rawSize := len(body)
						if compressed {
							out := make([]byte, lz4.CompressBlockBound(len(body)))
							n, err := lz4.CompressBlock([]byte(body), out, nil)
							require.NoError(t, err)
							if n == 0 {
								n, _ = copyIncompressible([]byte(body), out)
							}
							body = string(out[:n])
						}
						transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
							r := httpmock.NewStringResponse(status, body)
							r.Header.Set(RequestIDHeader, "http-request")
							if compressed {
								r.Header.Set("x-log-compresstype", "lz4")
								r.Header.Set(HTTPHeaderBodyRawSize, strconv.Itoa(rawSize))
							}
							return r, nil
						})
						req := NewGetGraphStoreDataRequest("RETURN 1")
						req.ResultFormat = format
						result, err := client.GetGraphStoreData("project", "store", req)
						if status == 200 {
							require.NoError(t, err)
							if meta == "" || meta == "null" {
								require.Nil(t, result.Meta)
							} else {
								require.Equal(t, meta, string(result.Meta))
							}
							encoded, encodeErr := json.Marshal(result)
							require.NoError(t, encodeErr)
							var fields map[string]json.RawMessage
							require.NoError(t, json.Unmarshal(encoded, &fields))
							_, present := fields["meta"]
							require.Equal(t, meta != "" && meta != "null", present)
							require.Equal(t, outcome, result.Status)
						} else {
							var serviceError *Error
							require.ErrorAs(t, err, &serviceError)
							if meta == "" || meta == "null" {
								require.Nil(t, serviceError.Meta)
							} else {
								require.Equal(t, meta, string(serviceError.Meta))
							}
							require.Equal(t, "Public message unchanged", serviceError.Message)
							require.Equal(t, int32(status), serviceError.HTTPCode)
							require.Equal(t, "http-request", serviceError.RequestID)
						}
					}
				}
			}
		}
	}
	transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
		return httpmock.NewStringResponse(400, `{"errorCode":"ParameterInvalid","errorMessage":"logstore error","meta":{"ignored":true}}`), nil
	})
	_, err := client.GetGraphStoreData("project", "store", NewGetGraphStoreDataRequest("RETURN 1"))
	var serviceError *Error
	require.ErrorAs(t, err, &serviceError)
	require.Empty(t, serviceError.Meta)
	require.Equal(t, "logstore error", serviceError.Message)
}

func TestGraphStoreQueryRejectsNonObjectMeta(t *testing.T) {
	oldRetry := RetryOnServerErrorEnabled
	RetryOnServerErrorEnabled = false
	defer func() { RetryOnServerErrorEnabled = oldRetry }()
	client := CreateNormalInterface("example.com", "key", "secret", "")
	transport := testutil.NewMockTransport()
	client.SetHTTPClient(&http.Client{Transport: transport})
	for _, meta := range []string{"[]", "42", "true", `"text"`} {
		for _, status := range []int{200, 400, 500} {
			body := `{"status":"success","data":{"fields":[],"values":[]},"meta":` + meta + `}`
			if status != 200 {
				body = `{"status":"error","error_code":"InternalServerError","error":"public","meta":` + meta + `}`
			}
			transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
				response := httpmock.NewStringResponse(status, body)
				response.Header.Set(RequestIDHeader, "invalid-meta")
				return response, nil
			})
			result, err := client.GetGraphStoreData("project", "store", NewGetGraphStoreDataRequest("RETURN 1"))
			require.Nil(t, result)
			var badResponse *BadResponseError
			if status == 200 {
				require.ErrorAs(t, err, &badResponse)
			} else {
				// The data API retains its existing ClientError wrapping of request decode errors.
				var clientError *Error
				require.ErrorAs(t, err, &clientError)
				require.Equal(t, "ClientError", clientError.Code)
				require.Nil(t, clientError.Meta)
				require.NoError(t, json.Unmarshal([]byte(clientError.Message), &badResponse))
			}
			require.Equal(t, status, badResponse.HTTPCode)
			require.Equal(t, "invalid-meta", http.Header(badResponse.RespHeader).Get(RequestIDHeader))
			require.Equal(t, body, badResponse.RespBody)
			require.Contains(t, badResponse.ErrorMessage, "meta must be an object")
		}
	}
}

func TestGraphStoreQueryErrorStringKeepsLegacyFields(t *testing.T) {
	expected := `{
    "httpCode": 400,
    "errorCode": "InvalidQuery",
    "errorMessage": "public",
    "requestID": "request"
}`
	for _, meta := range []json.RawMessage{nil, json.RawMessage("null"), json.RawMessage(`{"extra":"diagnostic"}`)} {
		serviceError := Error{HTTPCode: 400, Code: "InvalidQuery", Message: "public", RequestID: "request", Meta: meta}
		require.Equal(t, expected, serviceError.Error())
		require.Equal(t, expected, serviceError.String())
		encoded, err := json.Marshal(serviceError)
		require.NoError(t, err)
		require.JSONEq(t, expected, string(encoded))
		require.Equal(t, meta, serviceError.Meta)
	}
}

func graphStoreTestField(key, value string) *LogContent {
	return &LogContent{Key: proto.String(key), Value: proto.String(value)}
}

func graphStoreTestGroup(layout string) *LogGroup {
	return &LogGroup{Logs: []*Log{{Time: proto.Uint32(1), Contents: []*LogContent{graphStoreTestField("__layout__", layout)}}}}
}

func decodeGraphStoreDeflate(t *testing.T, data []byte) []byte {
	t.Helper()
	reader, err := zlib.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	decoded, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	return decoded
}

func TestGraphStoreBatchWriteWire(t *testing.T) {
	client := CreateNormalInterface("example.com", "key", "secret", "")
	transport := testutil.NewMockTransport()
	client.SetHTTPClient(&http.Client{Transport: transport})
	var batch SlsLogPackageList
	transport.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, req.Method)
		require.Equal(t, "/graphstore/store/write", req.URL.Path)
		require.Equal(t, "application/x-protobuf", req.Header.Get("Content-Type"))
		require.Equal(t, "batch_group", req.Header.Get("x-log-mode"))
		require.Equal(t, "mixed", req.Header.Get("x-log-compresstype"))
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		require.Equal(t, strconv.Itoa(len(body)), req.Header.Get(HTTPHeaderBodyRawSize))
		require.NoError(t, proto.Unmarshal(body, &batch))
		return httpmock.NewStringResponse(200, ""), nil
	})
	vertex, edge := graphStoreTestGroup("vertex"), graphStoreTestGroup("edge")
	edge.Logs[0].Contents = append(edge.Logs[0].Contents,
		graphStoreTestField("__active_from__", "0"), graphStoreTestField("__active_to__", "4294967295"))
	groups := []*LogGroup{vertex, edge}
	for _, tc := range []struct {
		compression int
		wire        int32
	}{
		{Compress_LZ4, 2}, {Compress_None, 0}, {Compress_ZSTD, 3}, {Compress_GZIP, 1},
	} {
		require.NoError(t, client.BatchPostGraphStoreData("project", "store", &BatchPostGraphStoreDataRequest{
			LogGroups: groups, CompressType: tc.compression,
		}))
		require.Len(t, batch.Packages, len(groups))
		for i, pack := range batch.Packages {
			require.NotNil(t, pack.CompressType)
			require.Equal(t, tc.wire, int32(pack.GetCompressType()))
			require.Equal(t, int32(groups[i].Size()), pack.GetUncompressSize())
			expected, err := groups[i].Marshal()
			require.NoError(t, err)
			decoded := pack.Data
			switch tc.compression {
			case Compress_LZ4:
				decoded = make([]byte, len(expected))
				n, err := lz4.UncompressBlock(pack.Data, decoded)
				require.NoError(t, err)
				require.Equal(t, len(expected), n)
			case Compress_ZSTD:
				decoded, err = slsZstdCompressor.Decompress(pack.Data, nil)
				require.NoError(t, err)
			case Compress_GZIP:
				decoded = decodeGraphStoreDeflate(t, pack.Data)
			}
			require.Equal(t, expected, decoded)
		}
	}
	require.Equal(t, 4, transport.GetTotalCallCount())
}

func TestGraphStoreWriteValidation(t *testing.T) {
	client := CreateNormalInterface("example.com", "key", "secret", "")
	transport := testutil.NewMockTransport()
	client.SetHTTPClient(&http.Client{Transport: transport})
	vertex := graphStoreTestGroup("vertex")
	for _, tc := range []struct {
		name string
		edit func(*LogGroup)
		want string
	}{
		{"missing layout with tag", func(g *LogGroup) {
			g.Logs[0].Contents = nil
			g.LogTags = []*LogTag{{Key: proto.String("__layout__"), Value: proto.String("vertex")}}
		}, "__layout__"},
		{"duplicate layout", func(g *LogGroup) {
			g.Logs[0].Contents = append(g.Logs[0].Contents, graphStoreTestField("__layout__", "vertex"))
		}, "__layout__"},
		{"unknown layout", func(g *LogGroup) { g.Logs[0].Contents[0].Value = proto.String("VERTEX") }, "__layout__"},
		{"zero time", func(g *LogGroup) { g.Logs[0].Time = proto.Uint32(0) }, "Time"},
		{"partial time", func(g *LogGroup) {
			g.Logs[0].Contents = append(g.Logs[0].Contents, graphStoreTestField("__active_from__", "1"))
		}, "__active_to__"},
		{"bad time", func(g *LogGroup) {
			g.Logs[0].Contents = append(g.Logs[0].Contents,
				graphStoreTestField("__active_from__", "+1"), graphStoreTestField("__active_to__", "4294967295"))
		}, "__active_from__"},
		{"overflow time", func(g *LogGroup) {
			g.Logs[0].Contents = append(g.Logs[0].Contents,
				graphStoreTestField("__active_from__", "1"), graphStoreTestField("__active_to__", "4294967296"))
		}, "__active_to__"},
		{"nil content", func(g *LogGroup) { g.Logs[0].Contents = append(g.Logs[0].Contents, nil) }, "contents"},
		{"nil tag", func(g *LogGroup) { g.LogTags = append(g.LogTags, nil) }, "logTags"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := graphStoreTestGroup("edge")
			tc.edit(bad)
			require.ErrorContains(t, client.PostGraphStoreData("project", "store", &PostGraphStoreDataRequest{LogGroup: bad}), tc.want)
			require.ErrorContains(t, client.BatchPostGraphStoreData("project", "store", &BatchPostGraphStoreDataRequest{
				LogGroups: []*LogGroup{vertex, bad},
			}), "logGroups[1]")
		})
	}
	require.Error(t, client.BatchPostGraphStoreData("project", "store", nil))
	require.Error(t, client.BatchPostGraphStoreData("project", "store", &BatchPostGraphStoreDataRequest{}))
	require.Error(t, client.BatchPostGraphStoreData("project", "store", &BatchPostGraphStoreDataRequest{LogGroups: []*LogGroup{vertex, nil}}))
	crowded := &LogGroup{Logs: make([]*Log, graphStoreMaxWriteLogs)}
	for i := range crowded.Logs {
		crowded.Logs[i] = vertex.Logs[0]
	}
	require.ErrorContains(t, client.BatchPostGraphStoreData("project", "store", &BatchPostGraphStoreDataRequest{LogGroups: []*LogGroup{crowded, vertex}}), "exceeds 100000 records")

	large := graphStoreTestGroup("vertex")
	large.Logs[0].Contents = append(large.Logs[0].Contents, graphStoreTestField("payload", strings.Repeat("x", GraphStoreMaxWriteBytes/2)))
	require.ErrorContains(t, client.BatchPostGraphStoreData("project", "store", &BatchPostGraphStoreDataRequest{LogGroups: []*LogGroup{large, large}}), "PostBodyTooLarge")

	field := large.Logs[0].Contents[1]
	field.Value = proto.String(strings.Repeat("x", GraphStoreMaxWriteBytes-large.Size()+len(field.GetValue())))
	require.Equal(t, GraphStoreMaxWriteBytes, large.Size())
	require.ErrorContains(t, client.BatchPostGraphStoreData("project", "store", &BatchPostGraphStoreDataRequest{
		LogGroups: []*LogGroup{large}, CompressType: Compress_None,
	}), "PostBodyTooLarge")
	require.Zero(t, transport.GetTotalCallCount())
}
