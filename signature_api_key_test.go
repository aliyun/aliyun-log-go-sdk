package sls

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignerAPIKey(t *testing.T) {
	for _, body := range [][]byte{nil, {}, []byte("compressed log body")} {
		t.Run(fmt.Sprintf("nil=%t/length=%d", body == nil, len(body)), func(t *testing.T) {
			headers := map[string]string{
				"Authorization":      "LOG old:signature",
				"Date":               "old date",
				"Content-Type":       "application/x-protobuf",
				"x-log-bodyrawsize":  "100",
				"x-log-compresstype": "lz4",
			}
			require.NoError(t, newSignerAPIKey("key+/=plain").Sign("POST", "/logstores/store", headers, body))
			require.Equal(t, "Bearer key+/=plain", headers[HTTPHeaderAuthorization])
			_, err := http.ParseTime(headers[HTTPHeaderDate])
			require.NoError(t, err)
			require.True(t, strings.HasSuffix(headers[HTTPHeaderDate], " GMT"))
			if body == nil {
				require.NotContains(t, headers, HTTPHeaderContentMD5)
			} else {
				require.Equal(t, fmt.Sprintf("%X", md5.Sum(body)), headers[HTTPHeaderContentMD5])
			}
			// Aside from Bearer and the omitted signature method, keep V1 headers.
			v1Headers := map[string]string{
				HTTPHeaderDate:       headers[HTTPHeaderDate],
				"Content-Type":       "application/x-protobuf",
				"x-log-bodyrawsize":  "100",
				"x-log-compresstype": "lz4",
			}
			require.NoError(t, NewSignerV1("ak", "secret").Sign("POST", "/logstores/store", v1Headers, body))
			delete(v1Headers, HTTPHeaderSignatureMethod)
			v1Headers[HTTPHeaderAuthorization] = "Bearer key+/=plain"
			require.Equal(t, v1Headers, headers)
		})
	}
}
