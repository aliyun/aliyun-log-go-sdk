package signer

import (
	"crypto/md5"
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type SignerSuite struct {
	suite.Suite
	mockAKID           string
	mockAKSec          string
	uri                string
	method             string
	region             string
	body               string
	dateTime           string
	urlParams, headers map[string]string
	v4                 Signer
}

func toUriWithQuery(uri string, urlParams map[string]string) string {
	vals := url.Values{}
	for k, v := range urlParams {
		vals.Add(k, v)
	}
	return fmt.Sprintf("%s?%s", uri, vals.Encode())
}

func (s *SignerSuite) SetupTest() {
	s.mockAKID = "acsddda21dsd"
	s.mockAKSec = "zxasdasdasw2"
	s.uri = "/logstores"
	s.method = "POST"
	s.region = "cn-hangzhou"
	s.body = "adasd= -asd zcas"
	s.headers = map[string]string{
		"hello":      "world",
		"hello-Text": "a12X- ",
		" Ko ":       "",
		"":           "AA",
		"x-log-test": "het123",
		"x-acs-ppp":  "dds",
	}
	s.urlParams = map[string]string{
		" abc":  "efg",
		" agc ": "",
		"":      "efg",
		"A-bc":  "eFg",
	}
	s.dateTime = "20220808T032330Z"
	// Set date
	s.headers["x-log-date"] = s.dateTime
	s.v4, _ = GetSigner(s.mockAKID, s.mockAKSec, SignVersionV4, s.region)
}

func (s *SignerSuite) TestSignV4Case1() {
	assert.Nil(s.T(), s.v4.Sign(s.method, toUriWithQuery(s.uri, s.urlParams), s.headers, []byte(s.body)))
	auth := s.headers["Authorization"]
	exp := "SLS4-HMAC-SHA256 " +
		"Credential=acsddda21dsd/20220808/cn-hangzhou/sls/aliyun_v4_request," +
		"SignedHeaders=x-acs-ppp;x-log-content-sha256;x-log-date;x-log-test," +
		"Signature=348d28cb4aa259a5302105b52d7d0ecde7ab415b3c0eb3a452f2a2fd38468991"
	assert.Equal(s.T(), exp, auth)
}

// Empty urlParams, empty headers, region cn-shanghai
func (s *SignerSuite) TestSignV4Case2() {
	s.region = "cn-shanghai"
	s.v4, _ = GetSigner(s.mockAKID, s.mockAKSec, SignVersionV4, s.region)
	s.headers = make(map[string]string)
	s.headers["x-log-date"] = s.dateTime

	assert.Nil(s.T(), s.v4.Sign(s.method, s.uri, s.headers, []byte(s.body)))
	auth := s.headers["Authorization"]
	exp := "SLS4-HMAC-SHA256 " +
		"Credential=acsddda21dsd/20220808/cn-shanghai/sls/aliyun_v4_request," +
		"SignedHeaders=x-log-content-sha256;x-log-date," +
		"Signature=8a10a5e723cb2e75964816de660b2c16a58af8bc0261f7f0722d832468c76ce8"
	assert.Equal(s.T(), exp, auth)
}

// Empty body
func (s *SignerSuite) TestSignV4Case3() {
	s.body = ""
	assert.Nil(s.T(), s.v4.Sign(s.method, toUriWithQuery(s.uri, s.urlParams), s.headers, []byte(s.body)))
	auth := s.headers["Authorization"]
	exp := "SLS4-HMAC-SHA256 " +
		"Credential=acsddda21dsd/20220808/cn-hangzhou/sls/aliyun_v4_request," +
		"SignedHeaders=x-acs-ppp;x-log-content-sha256;x-log-date;x-log-test," +
		"Signature=17277e433834a91c193f2dd6f237fc9b33c653f13f4c87e9e73a5f7fcabc6631"
	assert.Equal(s.T(), exp, auth)
}

// Empty body and method get
func (s *SignerSuite) TestSignV4Case4() {
	s.body = ""
	s.method = "GET"
	assert.Nil(s.T(), s.v4.Sign(s.method, toUriWithQuery(s.uri, s.urlParams), s.headers, []byte(s.body)))
	auth := s.headers["Authorization"]
	exp := "SLS4-HMAC-SHA256 " +
		"Credential=acsddda21dsd/20220808/cn-hangzhou/sls/aliyun_v4_request," +
		"SignedHeaders=x-acs-ppp;x-log-content-sha256;x-log-date;x-log-test," +
		"Signature=ef98c6596c88b80d12366ec42f4fab6d82037d961d84f2e8c52ab10908406470"
	assert.Equal(s.T(), exp, auth)
}

// Complex uri and urlParams
func (s *SignerSuite) TestSignV4Case5() {
	s.uri = "/logstores/hello/a+*~bb/cc"
	s.urlParams["abs-ij*asd/vc"] = "a~js+d ada"
	s.urlParams["a abAas123/vc"] = "a~jdad a2ADFs+d ada"
	assert.Nil(s.T(), s.v4.Sign(s.method, toUriWithQuery(s.uri, s.urlParams), s.headers, []byte(s.body)))
	auth := s.headers["Authorization"]
	exp := "SLS4-HMAC-SHA256 " +
		"Credential=acsddda21dsd/20220808/cn-hangzhou/sls/aliyun_v4_request," +
		"SignedHeaders=x-acs-ppp;x-log-content-sha256;x-log-date;x-log-test," +
		"Signature=d79c9358725537e03e3e0ff6d375853f36e2a7f853a2960053a498eefbbb42f5"
	assert.Equal(s.T(), exp, auth)
}

func (s *SignerSuite) TestSignV1Case1() {
	headers := map[string]string{
		"x-log-apiversion":      "0.6.0",
		"x-log-signaturemethod": "hmac-sha1",
		"x-log-bodyrawsize":     "0",
		"Date":                  "Mon, 3 Jan 2010 08:33:47 GMT",
	}
	mockAKID := "mockAccessKeyID"
	mockAKSec := "mockAccessKeySecret"
	expSign := "Rwm6cTKzoti4HWoe+GKcb6Kv07E="
	expAuth := fmt.Sprintf("SLS %s:%s", mockAKID, expSign)

	v1, _ := GetSigner(mockAKID, mockAKSec, SignVerionV1, "")
	v1.Sign("GET", "/logstores", headers, nil)
	auth := headers["Authorization"]
	assert.Equal(s.T(), expAuth, auth)
}

// Protobuf content
func (s *SignerSuite) TestSignV1Case2() {
	body := []byte{10, 50, 10, 30, 8, 248, 178, 147,
		158, 5, 18, 22, 10, 7, 84, 101, 115, 116, 75,
		101, 121, 18, 11, 84, 101, 115, 116, 67, 111,
		110, 116, 101, 110, 116, 26, 0, 34, 14, 49,
		48, 46, 50, 51, 48, 46, 50, 48, 49, 46, 49, 49, 55}
	md5Sum := fmt.Sprintf("%X", md5.Sum(body))
	headers := map[string]string{
		"x-log-apiversion":      "0.6.0",
		"x-log-signaturemethod": "hmac-sha1",
		"x-log-bodyrawsize":     "50",
		"Content-MD5":           md5Sum,
		"Content-Type":          "application/x-protobuf",
		"Content-Length":        "50",
		"Date":                  "Mon, 3 Jan 2010 08:33:47 GMT",
	}
	mockAKID := "mockAccessKeyID"
	mockAKSec := "mockAccessKeySecret"
	expSign := "87xQWqFaOSewqRIma8kPjGYlXHc="
	expAuth := fmt.Sprintf("SLS %s:%s", mockAKID, expSign)
	v1, _ := GetSigner(mockAKID, mockAKSec, SignVerionV1, "")
	v1.Sign("GET", "/logstores/app_log", headers, body)
	auth := headers["Authorization"]
	assert.Equal(s.T(), expAuth, auth)
}

// Test suite entry point
func TestSignerSuite(t *testing.T) {
	suite.Run(t, new(SignerSuite))
}
