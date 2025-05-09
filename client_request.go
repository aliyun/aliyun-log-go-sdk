package sls

// request sends a request to SLS.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/go-kit/kit/log/level"
)

// response must be nil or a pointer to a struct which is json unmarshalable
// @param reqBody if nil, it will be ignored, if is a []byte, use it, otherwise, it will be json marshaled
func (c *Client) doRequest(project, method, path string, queryParams, headers map[string]string, reqBody any, response any) error {
	buf, respHeader, statusCode, err := c.doRequestInner(project, method, path, queryParams, headers, reqBody)
	if err != nil {
		return err
	}
	if response == nil {
		return nil
	}
	if err := json.Unmarshal(buf, response); err != nil {
		return invalidJsonRespError(string(buf), respHeader, statusCode)
	}
	return nil
}

// get raw bytes of http response body
func (c *Client) doRequestRaw(project, method, path string, queryParams, headers map[string]string, reqBody any) (respBody []byte, err error) {
	buf, _, _, err := c.doRequestInner(project, method, path, queryParams, headers, reqBody)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// do not use this directly outside this file
func (c *Client) doRequestInner(project, method, path string, queryParams, headers map[string]string, reqBody any) (respBody []byte, respHeader http.Header, statusCode int, err error) {
	// body
	body, isJson, err := getRequestBody(reqBody)
	if err != nil {
		return nil, nil, 0, err
	}
	// headers
	if headers == nil {
		headers = make(map[string]string)
	}
	if isJson {
		headers[HTTPHeaderContentType] = "application/json"
	}
	if _, ok := headers[HTTPHeaderBodyRawSize]; !ok {
		headers[HTTPHeaderBodyRawSize] = strconv.Itoa(len(body))
	}

	r, err := c.request(project, method, getRequestUrl(path, queryParams), headers, body)
	if err != nil {
		return nil, nil, 0, err
	}

	// response
	defer r.Body.Close()
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, nil, 0, readResponseError(err)
	}
	if r.StatusCode != http.StatusOK {
		// should not reach here, but we keep checking it
		return nil, nil, 0, httpStatusNotOkError(buf, r.Header, r.StatusCode)
	}
	return buf, r.Header, r.StatusCode, nil
}

// request sends a request to alibaba cloud Log Service.
// @note if error is nil, you must call http.Response.Body.Close() to finalize reader
func (c *Client) request(project, method, uri string, headers map[string]string, body []byte) (*http.Response, error) {
	var endpoint string
	var usingHTTPS bool
	if strings.HasPrefix(c.Endpoint, "https://") {
		endpoint = c.Endpoint[8:]
		usingHTTPS = true
	} else if strings.HasPrefix(c.Endpoint, "http://") {
		endpoint = c.Endpoint[7:]
	} else {
		endpoint = c.Endpoint
	}

	// SLS public request headers
	var hostStr string
	if len(project) == 0 {
		hostStr = endpoint
	} else {
		hostStr = project + "." + endpoint
	}
	headers[HTTPHeaderHost] = hostStr
	headers[HTTPHeaderAPIVersion] = version

	if len(c.UserAgent) > 0 {
		headers[HTTPHeaderUserAgent] = c.UserAgent
	} else {
		headers[HTTPHeaderUserAgent] = DefaultLogUserAgent
	}

	c.accessKeyLock.RLock()
	stsToken := c.SecurityToken
	accessKeyID := c.AccessKeyID
	accessKeySecret := c.AccessKeySecret
	region := c.Region
	authVersion := c.AuthVersion
	c.accessKeyLock.RUnlock()

	if c.credentialsProvider != nil {
		res, err := c.credentialsProvider.GetCredentials()
		if err != nil {
			return nil, fmt.Errorf("fail to fetch credentials: %w", err)
		}
		accessKeyID = res.AccessKeyID
		accessKeySecret = res.AccessKeySecret
		stsToken = res.SecurityToken
	}

	// Access with token
	if stsToken != "" {
		headers[HTTPHeaderAcsSecurityToken] = stsToken
	}

	if body != nil {
		if _, ok := headers[HTTPHeaderContentType]; !ok {
			return nil, fmt.Errorf("Can't find 'Content-Type' header")
		}
	}
	for k, v := range c.InnerHeaders {
		headers[k] = v
	}
	var signer Signer
	if authVersion == AuthV4 {
		headers[HTTPHeaderLogDate] = dateTimeISO8601()
		signer = NewSignerV4(accessKeyID, accessKeySecret, region)
	} else if authVersion == AuthV0 {
		signer = NewSignerV0()
	} else {
		headers[HTTPHeaderDate] = nowRFC1123()
		signer = NewSignerV1(accessKeyID, accessKeySecret)
	}
	if err := signer.Sign(method, uri, headers, body); err != nil {
		return nil, err
	}

	addHeadersAfterSign(c.CommonHeaders, headers)
	// Initialize http request
	reader := bytes.NewReader(body)
	var urlStr string
	// using http as default
	if !GlobalForceUsingHTTP && usingHTTPS {
		urlStr = "https://"
	} else {
		urlStr = "http://"
	}
	urlStr += hostStr + uri
	req, err := http.NewRequest(method, urlStr, reader)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Add(k, v)
	}
	if IsDebugLevelMatched(5) {
		dump, e := httputil.DumpRequest(req, true)
		if e != nil {
			level.Info(Logger).Log("msg", e)
		}
		level.Info(Logger).Log("msg", "HTTP Request:\n%v", string(dump))
	}

	// Get ready to do request
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = defaultHttpClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Parse the sls error from body.
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		buf, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, readResponseError(err)
		}
		return nil, httpStatusNotOkError(buf, resp.Header, resp.StatusCode)
	}
	if IsDebugLevelMatched(5) {
		dump, e := httputil.DumpResponse(resp, true)
		if e != nil {
			level.Info(Logger).Log("msg", e)
		}
		level.Info(Logger).Log("msg", "HTTP Response:\n%v", string(dump))
	}

	return resp, nil
}

func getRequestBody(reqBody any) (body []byte, isJson bool, err error) {
	if reqBody == nil {
		return nil, false, nil
	}
	if b, ok := reqBody.([]byte); ok {
		return b, false, nil
	}
	body, err = json.Marshal(reqBody)
	return body, true, err
}

func getRequestUrl(path string, queryParams map[string]string) string {
	if queryParams == nil {
		return path
	}
	urlVal := url.Values{}
	for k, v := range queryParams {
		urlVal.Add(k, v)
	}
	return path + "?" + urlVal.Encode()
}
