package producer

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/require"
)

type producerAPIKeyTransport func(*http.Request) (*http.Response, error)

func (f producerAPIKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type producerAPIKeyProvider struct{ calls int32 }

func (p *producerAPIKeyProvider) GetCredentials() (sls.Credentials, error) {
	atomic.AddInt32(&p.calls, 1)
	return sls.Credentials{}, errors.New("AK provider must not be called")
}

func TestProducerAPIKey(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, path := range []string{"logs", "hash", "metric-url-ignored", "hash-metric-url-ignored"} {
			name := "NewProducer/" + path
			if legacy {
				name = "InitProducer/" + path
			}
			t.Run(name, func(t *testing.T) {
				provider := &producerAPIKeyProvider{}
				var stsCalls int32
				requests := make(chan *http.Request, 1)
				config := GetDefaultProducerConfig()
				config.Endpoint = "https://endpoint.example"
				config.ApiKey = "producer-key"
				config.CredentialsProvider = provider
				config.AccessKeyID, config.AccessKeySecret = "ak", "secret"
				config.AuthVersion = sls.AuthV4
				config.UpdateStsToken = func() (string, string, string, time.Time, error) {
					atomic.AddInt32(&stsCalls, 1)
					return "", "", "", time.Time{}, errors.New("STS refresh must not be started")
				}
				config.StsTokenShutDown = make(chan struct{})
				config.UseMetricStoreURL = strings.Contains(path, "metric-url")
				config.AdjustShargHash = false
				config.UserAgent = "producer-api-key-test"
				config.Logger = log.NewNopLogger()
				config.DisableRuntimeMetrics = true
				config.MaxIoWorkerCount = 1
				config.LingerMs = 100
				config.Retries = 0
				config.HTTPClient = &http.Client{Timeout: time.Second, Transport: producerAPIKeyTransport(func(req *http.Request) (*http.Response, error) {
					requests <- req.Clone(req.Context())
					return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
				})}
				var p *Producer
				if legacy {
					p = InitProducer(config)
				} else {
					var err error
					p, err = NewProducer(config)
					require.NoError(t, err)
				}
				require.False(t, p.producerConfig.UseMetricStoreURL)
				p.Start()
				entry := GenerateLog(1700000000, map[string]string{"content": "api key log"})
				var err error
				if strings.HasPrefix(path, "hash") {
					err = p.HashSendLog("project", "store", "abcdef", "topic", "source", entry)
				} else {
					err = p.SendLog("project", "store", "topic", "source", entry)
				}
				p.SafeClose()
				require.NoError(t, err)
				select {
				case req := <-requests:
					require.Equal(t, "https", req.URL.Scheme)
					require.Equal(t, "Bearer producer-key", req.Header.Get("Authorization"))
					require.Equal(t, "producer-api-key-test", req.Header.Get("User-Agent"))
					require.NotEmpty(t, req.Header.Get("Date"))
					require.NotEmpty(t, req.Header.Get("Content-MD5"))
					require.Empty(t, req.Header.Get("x-acs-security-token"))
					require.Empty(t, req.Header.Get("x-log-signaturemethod"))
					if strings.HasPrefix(path, "hash") {
						require.Equal(t, "/logstores/store/shards/route", req.URL.Path)
						require.Equal(t, "abcdef", req.URL.Query().Get("key"))
					} else {
						require.Equal(t, "/logstores/store", req.URL.Path)
					}
				default:
					t.Fatal("producer did not send the log")
				}
				require.Zero(t, atomic.LoadInt32(&provider.calls))
				require.Zero(t, atomic.LoadInt32(&stsCalls))
				select {
				case <-config.StsTokenShutDown:
					t.Fatal("unused STS shutdown channel must not be closed")
				default:
				}
			})
		}
	}
}
