package sls

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
)

type apiKeyRoundTripper func(*http.Request) (*http.Response, error)

func (f apiKeyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type apiKeyTestProvider struct{ calls int }

func (p *apiKeyTestProvider) GetCredentials() (Credentials, error) {
	p.calls++
	return Credentials{}, errors.New("AK provider must not be used")
}

func apiKeyTestLogGroup() *LogGroup {
	return &LogGroup{
		Topic: proto.String("topic"), Source: proto.String("source"),
		Logs: []*Log{{Time: proto.Uint32(1700000000), Contents: []*LogContent{
			{Key: proto.String("content"), Value: proto.String(strings.Repeat("log data ", 100))},
		}}},
	}
}

func apiKeyTestResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}
}

func TestClientAPIKeyWrites(t *testing.T) {
	lg := apiKeyTestLogGroup()
	raw, err := proto.Marshal(lg)
	require.NoError(t, err)
	for _, compress := range []int{Compress_LZ4, Compress_ZSTD, Compress_None} {
		for _, path := range []string{"logs", "hash", "raw", "raw-hash", "metrics"} {
			t.Run(fmt.Sprintf("%s/compress=%d", path, compress), func(t *testing.T) {
				provider := &apiKeyTestProvider{}
				client := &Client{
					Endpoint: "https://endpoint.example:8443", ApiKey: "plain-api-key",
					AccessKeyID: "ak", AccessKeySecret: "secret", SecurityToken: "sts",
					AuthVersion:   AuthV4, // No Region: API key must bypass V4 validation.
					UserAgent:     "api-key-test",
					CommonHeaders: map[string]string{"x-custom": "keep"},
				}
				client.WithCredentialsProvider(provider)
				calls := 0
				client.HTTPClient = &http.Client{Transport: apiKeyRoundTripper(func(req *http.Request) (*http.Response, error) {
					calls++
					body, err := io.ReadAll(req.Body)
					require.NoError(t, err)
					require.Equal(t, "https", req.URL.Scheme)
					require.Equal(t, "project.endpoint.example:8443", req.URL.Host)
					require.Equal(t, http.MethodPost, req.Method)
					require.Equal(t, []string{"Bearer plain-api-key"}, req.Header.Values(HTTPHeaderAuthorization))
					require.Equal(t, fmt.Sprintf("%X", md5.Sum(body)), req.Header.Get(HTTPHeaderContentMD5))
					require.Equal(t, int64(len(body)), req.ContentLength)
					require.Equal(t, strconv.Itoa(len(raw)), req.Header.Get(HTTPHeaderBodyRawSize))
					require.Equal(t, "application/x-protobuf", req.Header.Get(HTTPHeaderContentType))
					require.Equal(t, version, req.Header.Get(HTTPHeaderAPIVersion))
					require.Equal(t, "api-key-test", req.Header.Get(HTTPHeaderUserAgent))
					require.Equal(t, "keep", req.Header.Get("x-custom"))
					_, err = http.ParseTime(req.Header.Get(HTTPHeaderDate))
					require.NoError(t, err)
					for _, h := range []string{HTTPHeaderAcsSecurityToken, HTTPHeaderSignatureMethod, HTTPHeaderLogDate, HTTPHeaderLogContentSha256} {
						require.Empty(t, req.Header.Values(h))
					}
					wantCompress := map[int]string{Compress_LZ4: "lz4", Compress_ZSTD: "zstd", Compress_None: ""}[compress]
					if path == "metrics" {
						wantCompress = "lz4"
					}
					require.Equal(t, wantCompress, req.Header.Get("x-log-compresstype"))
					if wantCompress == "" {
						require.Equal(t, raw, body)
					}
					switch path {
					case "metrics":
						require.Equal(t, "/prometheus/project/store/api/v1/write", req.URL.Path)
					case "hash", "raw-hash":
						require.Equal(t, "/logstores/store/shards/route", req.URL.Path)
						require.Equal(t, "abcdef", req.URL.Query().Get("key"))
					default:
						require.Equal(t, "/logstores/store", req.URL.Path)
					}
					if path == "logs" || path == "hash" {
						require.Equal(t, "processor-1", req.URL.Query().Get("processor"))
					}
					return apiKeyTestResponse(req, http.StatusOK, ""), nil
				})}
				hash := "abcdef"
				switch path {
				case "logs", "hash":
					req := &PostLogStoreLogsRequest{LogGroup: lg, CompressType: compress, Processor: "processor-1"}
					if path == "hash" {
						req.HashKey = &hash
					}
					err = client.PostLogStoreLogsV2("project", "store", req)
				case "raw":
					err = client.PutRawLogWithCompressType("project", "store", raw, compress)
				case "raw-hash":
					err = client.PostRawLogWithCompressType("project", "store", raw, compress, &hash)
				case "metrics":
					err = client.PutLogsWithMetricStoreURL("project", "store", lg)
				}
				require.NoError(t, err)
				require.Equal(t, 1, calls)
				require.Zero(t, provider.calls)
			})
		}
	}
}

func TestClientAPIKeyRespectsHTTPSConfiguration(t *testing.T) {
	oldForce := GlobalForceUsingHTTP
	t.Cleanup(func() { GlobalForceUsingHTTP = oldForce })
	tests := []struct {
		endpoint                    string
		forceHTTP, usingHTTP        bool
		clientScheme, projectScheme string
	}{
		{"endpoint.example", false, false, "http", "http"},
		{"http://endpoint.example", false, false, "http", "http"},
		{"https://endpoint.example", false, false, "https", "https"},
		{"endpoint.example", true, false, "http", "http"},
		{"http://endpoint.example", true, false, "http", "http"},
		{"https://endpoint.example", true, false, "http", "http"},
		{"https://endpoint.example", false, true, "https", "http"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/forceHTTP=%t/usingHTTP=%t", tt.endpoint, tt.forceHTTP, tt.usingHTTP), func(t *testing.T) {
			GlobalForceUsingHTTP = tt.forceHTTP
			client := &Client{Endpoint: tt.endpoint, ApiKey: "key"}
			wantScheme := tt.clientScheme
			calls := 0
			client.HTTPClient = &http.Client{Transport: apiKeyRoundTripper(func(req *http.Request) (*http.Response, error) {
				calls++
				require.Equal(t, wantScheme+"://project.endpoint.example/logstores/store", req.URL.String())
				require.Equal(t, "Bearer key", req.Header.Get(HTTPHeaderAuthorization))
				return apiKeyTestResponse(req, http.StatusOK, ""), nil
			})}
			resp, err := client.request("project", http.MethodPost, "/logstores/store", map[string]string{HTTPHeaderBodyRawSize: "0"}, nil)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			wantScheme = tt.projectScheme
			ls := convertLogstore(client, "project", "store")
			ls.project.UsingHTTP = tt.usingHTTP
			ls.project.parseEndpoint()
			require.NoError(t, ls.PutLogs(apiKeyTestLogGroup()))
			require.Equal(t, 2, calls)
		})
	}
}

func TestClientAPIKeyLeavesOperationSupportToServer(t *testing.T) {
	provider := &apiKeyTestProvider{}
	client := (&Client{Endpoint: "https://endpoint.example", ApiKey: "key", AuthVersion: AuthV4}).WithCredentialsProvider(provider)
	calls := 0
	client.HTTPClient = &http.Client{Transport: apiKeyRoundTripper(func(req *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, "https", req.URL.Scheme)
		require.Equal(t, "Bearer key", req.Header.Get(HTTPHeaderAuthorization))
		resp := apiKeyTestResponse(req, http.StatusForbidden, `{"errorCode":"Unauthorized","errorMessage":"rejected by service"}`)
		resp.Header.Set(RequestIDHeader, "server-request-id")
		return resp, nil
	})}
	operations := []struct {
		name string
		call func() error
	}{
		{"list shards", func() error { _, err := client.ListShards("project", "store"); return err }},
		{"query logs", func() error {
			_, err := client.GetLogsV3("project", "store", &GetLogRequest{From: 1, To: 2, Query: "*"})
			return err
		}},
		{"create index", func() error { return convertLogstore(client, "project", "store").CreateIndexString(`{}`) }},
		{"delete logs", func() error {
			_, err := client.DeleteLogStoreLogs("project", "store", &DeleteLogStoreLogsRequest{From: 1, To: 2})
			return err
		}},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			before := calls
			err := operation.call()
			var serviceErr *Error
			require.ErrorAs(t, err, &serviceErr)
			require.Equal(t, int32(http.StatusForbidden), serviceErr.HTTPCode)
			require.Equal(t, "Unauthorized", serviceErr.Code)
			require.Equal(t, "rejected by service", serviceErr.Message)
			require.Equal(t, "server-request-id", serviceErr.RequestID)
			require.Equal(t, before+1, calls)
		})
	}
	require.Zero(t, provider.calls)
}

func TestClientAPIKeyDirectRequest(t *testing.T) {
	oldForce := GlobalForceUsingHTTP
	GlobalForceUsingHTTP = true
	t.Cleanup(func() { GlobalForceUsingHTTP = oldForce })
	provider := &apiKeyTestProvider{}
	client := (&Client{
		Endpoint: "https://endpoint.example", ApiKey: "key", AuthVersion: AuthV4,
		SecurityToken: "sts",
		CommonHeaders: map[string]string{"x-custom": "keep"},
	}).WithCredentialsProvider(provider)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			var body []byte
			if method == http.MethodPost {
				body = []byte(`{"value":"test"}`)
			}
			calls := 0
			client.HTTPClient = &http.Client{Transport: apiKeyRoundTripper(func(req *http.Request) (*http.Response, error) {
				calls++
				require.Equal(t, "http://project.endpoint.example/test-api", req.URL.String())
				require.Equal(t, []string{"Bearer key"}, req.Header.Values(HTTPHeaderAuthorization))
				require.Equal(t, "keep", req.Header.Get("x-custom"))
				require.Empty(t, req.Header.Values(HTTPHeaderAcsSecurityToken))
				require.Empty(t, req.Header.Values(HTTPHeaderSignatureMethod))
				_, err := http.ParseTime(req.Header.Get(HTTPHeaderDate))
				require.NoError(t, err)
				if body != nil {
					require.Equal(t, fmt.Sprintf("%X", md5.Sum(body)), req.Header.Get(HTTPHeaderContentMD5))
				} else {
					require.Empty(t, req.Header.Get(HTTPHeaderContentMD5))
				}
				return apiKeyTestResponse(req, http.StatusOK, "{}"), nil
			})}
			resp, err := client.request("project", method, "/test-api", map[string]string{
				HTTPHeaderBodyRawSize: strconv.Itoa(len(body)), HTTPHeaderContentType: "application/json",
			}, body)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			require.Equal(t, 1, calls)
		})
	}
	require.Zero(t, provider.calls)
}

func TestClientAPIKeyUsesHTTPClientPolicyWithoutAKFallback(t *testing.T) {
	for _, status := range []int{301, 302, 307, 308, 401, 403} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			provider := &apiKeyTestProvider{}
			calls := 0
			redirects := 0
			httpClient := &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
				redirects++
				return http.ErrUseLastResponse
			}, Transport: apiKeyRoundTripper(func(req *http.Request) (*http.Response, error) {
				calls++
				require.Equal(t, "https", req.URL.Scheme)
				resp := apiKeyTestResponse(req, status, `{"errorCode":"Rejected","errorMessage":"rejected"}`)
				resp.Header.Set("Location", "http://project.endpoint.example/redirect")
				return resp, nil
			})}
			client := (&Client{Endpoint: "https://endpoint.example", ApiKey: "key", HTTPClient: httpClient}).WithCredentialsProvider(provider)
			err := client.PutLogs("project", "store", apiKeyTestLogGroup())
			require.Error(t, err)
			require.Equal(t, 1, calls)
			if status < 400 {
				require.Equal(t, 1, redirects)
			} else {
				require.Zero(t, redirects)
			}
			require.Zero(t, provider.calls)
			require.NotNil(t, httpClient.CheckRedirect)
			require.Equal(t, time.Second, httpClient.Timeout)
		})
	}
}

func TestClientAPIKeyTLSAndDebug(t *testing.T) {
	lg := apiKeyTestLogGroup()
	raw, err := proto.Marshal(lg)
	require.NoError(t, err)
	type capturedRequest struct {
		header http.Header
		body   []byte
	}
	captured := make(chan capturedRequest, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured <- capturedRequest{header: r.Header.Clone(), body: body}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	var logs bytes.Buffer
	oldLogger, oldDebug := Logger, GlobalDebugLevel
	Logger, GlobalDebugLevel = log.NewLogfmtLogger(&logs), 5
	t.Cleanup(func() { Logger, GlobalDebugLevel = oldLogger, oldDebug })
	client := &Client{Endpoint: server.URL, ApiKey: "plaintext-secret-key", HTTPClient: server.Client()}
	require.NoError(t, client.PutLogsWithCompressType("", "store", lg, Compress_None))
	received := <-captured
	require.Equal(t, "Bearer plaintext-secret-key", received.header.Get(HTTPHeaderAuthorization))
	require.Equal(t, raw, received.body)
	require.Contains(t, logs.String(), "plaintext-secret-key")
}

func TestClientEmptyAPIKeyKeepsAKAuthentication(t *testing.T) {
	for _, auth := range []AuthVersionType{AuthV1, AuthV4} {
		t.Run(string(auth), func(t *testing.T) {
			client := (&Client{Endpoint: "http://endpoint.example", Region: "cn-test", AuthVersion: auth}).WithCredentialsProvider(NewStaticCredentialsProvider("ak", "secret", "sts"))
			client.HTTPClient = &http.Client{Transport: apiKeyRoundTripper(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, "http", req.URL.Scheme)
				require.Equal(t, "sts", req.Header.Get(HTTPHeaderAcsSecurityToken))
				prefix := "LOG ak:"
				if auth == AuthV4 {
					prefix = "SLS4-HMAC-SHA256 "
				}
				require.True(t, strings.HasPrefix(req.Header.Get(HTTPHeaderAuthorization), prefix))
				return apiKeyTestResponse(req, http.StatusOK, ""), nil
			})}
			require.NoError(t, client.PutLogs("project", "store", apiKeyTestLogGroup()))
		})
	}
}
