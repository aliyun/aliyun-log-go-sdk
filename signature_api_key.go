package sls

import (
	"crypto/md5"
	"fmt"
)

type signerAPIKey struct {
	apiKey string
}

var _ Signer = (*signerAPIKey)(nil)

// newSignerAPIKey authenticates requests with a plaintext API key instead of an AK signature.
// Currently, only log write requests support API keys.
// Configure the client Endpoint with an https:// prefix to avoid exposing the key
// over HTTP. Supported operations are checked by the service.
func newSignerAPIKey(apiKey string) Signer {
	return &signerAPIKey{apiKey: apiKey}
}

// Sign keeps the V1 Date and body checksum format without calculating an AK signature.
func (s *signerAPIKey) Sign(method, uri string, headers map[string]string, body []byte) error {
	headers[HTTPHeaderDate] = nowRFC1123()
	if body != nil {
		headers[HTTPHeaderContentMD5] = fmt.Sprintf("%X", md5.Sum(body))
	}
	headers[HTTPHeaderAuthorization] = "Bearer " + s.apiKey
	return nil
}
